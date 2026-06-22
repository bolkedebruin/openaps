package gridprofile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// asyncMaxConcurrentUIDs bounds the number of overlay-apply goroutines that
// may run in parallel across all uids. Excess overlay applies queue.
const asyncMaxConcurrentUIDs = 3

// errSuperseded is the cancellation cause attached when a newer overlay-apply
// for the same uid replaces an older one. Distinguished from a nil cause
// (shutdown via the apply-parent context) so the running goroutine can drop
// silently on supersession (the new job emits its own events) but still emit
// a terminal overlay_apply_complete row on shutdown.
var errSuperseded = errors.New("overlay apply superseded by newer apply for same uid")

// EventSink receives audit-log events emitted by the async overlay applier.
// All events use by="inv-driver". The sink implementation routes them to the
// store.events table (see store.AppendEvent). A nil sink disables emission.
type EventSink interface {
	// AppendEvent records one event. uid is the inverter uid (may be empty for
	// fleet events but is always set by the overlay applier), kind is the
	// stable event-kind string, severity is "info" or "warn", detail is a
	// short human-readable summary.
	AppendEvent(ctx context.Context, tsMs int64, uid, kind, severity, detail string)
}

// reconcileRunner is the minimal interface the async applier needs from the
// reconciler; the production type is *Reconciler.  Defining it as an interface
// lets tests substitute a stub that simulates blocking / failing applies.
type reconcileRunner interface {
	ReconcileUID(ctx context.Context, uid string) (ReconcileReport, error)
}

// applier serialises overlay-apply work per uid and bounds the number of
// concurrent uid applies. It is safe for concurrent use.
//
// The applier is owned by Manager; it is constructed lazily on first
// SetOverlay call.
type applier struct {
	rec   reconcileRunner
	sink  EventSink
	sema  chan struct{} // bounded concurrency over all uids
	mu    sync.Mutex
	jobs  map[string]*applyJob // uid -> in-flight job
	clock func() time.Time     // for tests
}

// applyJob tracks one in-flight per-uid apply so a newer apply for the same
// uid can supersede it.
type applyJob struct {
	overlayID string
	// cancel is supersession-only: it cancels jobCtx with cause errSuperseded
	// so the running goroutine can tell supersession apart from shutdown
	// (which cancels via the parent context with a nil cause).
	cancel context.CancelCauseFunc
	// done is closed when the goroutine returns.
	done chan struct{}
}

// overlayCodeSet builds the lookup used by run() to filter the reconciler's
// PointResults down to overlay-owned codes — base-profile points the overlay
// never touched MUST NOT produce overlay_param_* events.
func overlayCodeSet(codes []string) map[string]struct{} {
	m := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		m[c] = struct{}{}
	}
	return m
}

// newApplier constructs an applier.
func newApplier(rec reconcileRunner, sink EventSink) *applier {
	return &applier{
		rec:   rec,
		sink:  sink,
		sema:  make(chan struct{}, asyncMaxConcurrentUIDs),
		jobs:  map[string]*applyJob{},
		clock: time.Now,
	}
}

// enqueueWithPoints is the production entry point. codes lists the overlay's
// own APS codes (e.g. ["BP","BQ"]); the apply goroutine emits overlay_param_*
// rows ONLY for those codes — the reconcile report's base-profile points are
// ignored even if they appear in PointResults. pointCount reflects the
// overlay's point count for the started/complete summaries; codes scopes the
// per-param loop.
func (a *applier) enqueueWithPoints(parent context.Context, uid, overlayID string, codes []string) <-chan struct{} {
	return a.enqueueInternal(parent, uid, overlayID, len(codes), codes)
}

// enqueueInternal is the shared implementation. scopedCodes==nil disables the
// filter (all PointResults are emitted).
func (a *applier) enqueueInternal(parent context.Context, uid, overlayID string, pointCount int, scopedCodes []string) <-chan struct{} {
	// WithCancelCause lets us label why a job was cancelled: supersession sets
	// cause=errSuperseded (silent drop in run()); shutdown cancels the parent
	// with cause=nil so context.Cause(jobCtx) == parent's cause (typically nil
	// or context.Canceled) — run() then emits a terminal "aborted: shutdown"
	// overlay_apply_complete row.
	jobCtx, cancel := context.WithCancelCause(parent)

	a.mu.Lock()
	if prev, ok := a.jobs[uid]; ok {
		// Supersede the older apply; its remaining work and events are dropped.
		slog.Debug("gridprofile overlay_apply_superseded",
			"uid", uid, "old_id", prev.overlayID, "new_id", overlayID)
		prev.cancel(errSuperseded)
	}
	job := &applyJob{overlayID: overlayID, cancel: cancel, done: make(chan struct{})}
	a.jobs[uid] = job
	a.mu.Unlock()

	go a.run(jobCtx, job, uid, overlayID, pointCount, scopedCodes)
	return job.done
}

// run executes the apply on a single goroutine, gated by the bounded
// semaphore. It emits started → per-point → complete events.
//
// On exit, cancellation cause distinguishes two flavours: supersession
// (cause=errSuperseded) → silent drop, the newer apply emits its own events;
// shutdown / parent-ctx done → emit a terminal overlay_apply_complete with
// severity=warn and detail prefixed "aborted: shutdown" so the audit log
// never has a started row without a matching terminal row.
func (a *applier) run(ctx context.Context, job *applyJob, uid, overlayID string, pointCount int, scopedCodes []string) {
	defer close(job.done)
	defer a.clearJob(uid, job)

	// scopedCodes (when non-nil) filters the reconcile report's PointResults
	// down to overlay-owned codes; a nil map means "emit all results" (legacy
	// callers / tests). Production overlay applies always pass a non-nil
	// scope so base-profile points in the merged report never produce
	// overlay_param_* events.
	var scope map[string]struct{}
	if scopedCodes != nil {
		scope = overlayCodeSet(scopedCodes)
	}
	inScope := func(code string) bool {
		if scope == nil {
			return true
		}
		_, ok := scope[code]
		return ok
	}

	// Bounded concurrency across all uids.
	select {
	case a.sema <- struct{}{}:
	case <-ctx.Done():
		// Cancelled before we ever started. Supersession is silent; a
		// non-supersession cancellation (shutdown) emits a terminal row so
		// the started event for *this* uid isn't dangling — but if we never
		// emitted started either (which is the case here), there's nothing
		// to balance, so still emit a single complete row to record that the
		// queued apply never ran.
		if errors.Is(context.Cause(ctx), errSuperseded) {
			return
		}
		a.emit(context.Background(), uid, "overlay_apply_complete", "warn",
			fmt.Sprintf("id=%q uid=%s aborted: shutdown — applied 0 of %d", overlayID, uid, pointCount))
		return
	}
	defer func() { <-a.sema }()

	// If a newer apply has superseded us between semaphore-grab and the next
	// statement (rare), drop silently.
	if ctx.Err() != nil {
		if errors.Is(context.Cause(ctx), errSuperseded) {
			return
		}
		a.emit(context.Background(), uid, "overlay_apply_complete", "warn",
			fmt.Sprintf("id=%q uid=%s aborted: shutdown — applied 0 of %d", overlayID, uid, pointCount))
		return
	}

	a.emit(ctx, uid, "overlay_apply_started", "info",
		fmt.Sprintf("id=%q uid=%s points=%d", overlayID, uid, pointCount))

	rpt, err := a.rec.ReconcileUID(ctx, uid)

	// Cancellation handling: supersession silent; shutdown emits a terminal
	// row with the partial report.
	if ctx.Err() != nil {
		if errors.Is(context.Cause(ctx), errSuperseded) {
			return
		}
		// Shutdown summary reports only overlay-scoped points so the
		// numerator/denominator match the started event's points=N.
		ok, fail := countResultsScoped(rpt.Points, inScope)
		a.emit(context.Background(), uid, "overlay_apply_complete", "warn",
			fmt.Sprintf("id=%q uid=%s aborted: shutdown — applied %d of %d", overlayID, uid, ok, ok+fail))
		return
	}

	var okCount, failedCount int
	if err != nil {
		// Top-level error — count as one failure spanning the apply.
		a.emit(ctx, uid, "overlay_param_failed", "warn",
			fmt.Sprintf("id=%q uid=%s err=%q", overlayID, uid, err.Error()))
		failedCount++
	} else {
		// Scope per-point emission to overlay-owned codes. The reconciler's
		// report merges base + overlay (the full effective profile), but
		// overlay_param_* events are about overlay-owned points only —
		// base-profile reconcile is VerifyStartup's concern.
		scopedLen := 0
		for _, pr := range rpt.Points {
			if inScope(pr.ApsCode) {
				scopedLen++
			}
		}
		for _, pr := range rpt.Points {
			if !inScope(pr.ApsCode) {
				continue
			}
			if ctx.Err() != nil {
				if errors.Is(context.Cause(ctx), errSuperseded) {
					return
				}
				a.emit(context.Background(), uid, "overlay_apply_complete", "warn",
					fmt.Sprintf("id=%q uid=%s aborted: shutdown — applied %d of %d", overlayID, uid, okCount, scopedLen))
				return
			}
			switch pr.Result {
			case "in_sync", "applied":
				a.emit(ctx, uid, "overlay_param_written", "info",
					fmt.Sprintf("id=%q uid=%s code=%s value=%g", overlayID, uid, pr.ApsCode, pr.Intended))
				okCount++
			case "unknown":
				// QS1A page-A code: not readable so confirmation is impossible.
				// Treat as ok (not a wire failure) but do not emit a per-param
				// row — the overlay_apply_complete summary still reports it.
				okCount++
			default:
				a.emit(ctx, uid, "overlay_param_failed", "warn",
					fmt.Sprintf("id=%q uid=%s code=%s err=%q", overlayID, uid, pr.ApsCode, pr.Result))
				failedCount++
			}
		}
	}

	// Severity reflects per-param failure: warn if any param failed (the
	// summary surfaces a problem the operator should investigate), info on a
	// clean apply.
	sev := "info"
	if failedCount > 0 {
		sev = "warn"
	}
	a.emit(ctx, uid, "overlay_apply_complete", sev,
		fmt.Sprintf("id=%q uid=%s ok=%d failed=%d", overlayID, uid, okCount, failedCount))
}

// countResultsScoped tallies (ok, failed) the same way run() classifies
// points, restricted to codes the inScope predicate accepts — used by run() so
// an aborted shutdown summary reports only the overlay's own points, not the
// merged base+overlay set the reconciler returned.
func countResultsScoped(points []PointResult, inScope func(string) bool) (ok, failed int) {
	for _, pr := range points {
		if !inScope(pr.ApsCode) {
			continue
		}
		switch pr.Result {
		case "in_sync", "applied", "unknown":
			ok++
		default:
			failed++
		}
	}
	return
}

// clearJob removes job from the in-flight map if it is still the current one
// for uid. A newer job will already have overwritten the entry.
func (a *applier) clearJob(uid string, job *applyJob) {
	a.mu.Lock()
	if cur, ok := a.jobs[uid]; ok && cur == job {
		delete(a.jobs, uid)
	}
	a.mu.Unlock()
}

// emit forwards one event to the sink with the current timestamp.  Nil sink
// is a no-op (e.g. tests that don't care about events).
func (a *applier) emit(ctx context.Context, uid, kind, severity, detail string) {
	if a.sink == nil {
		return
	}
	a.sink.AppendEvent(ctx, a.clock().UnixMilli(), uid, kind, severity, detail)
}

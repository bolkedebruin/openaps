package gridprofile

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordedEvent is one event captured by recordingSink.
type recordedEvent struct {
	tsMs     int64
	uid      string
	kind     string
	severity string
	detail   string
}

// recordingSink captures every AppendEvent call for assertion. Safe for
// concurrent use.
type recordingSink struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *recordingSink) AppendEvent(_ context.Context, tsMs int64, uid, kind, severity, detail string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{tsMs, uid, kind, severity, detail})
}

func (r *recordingSink) snapshot() []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedEvent, len(r.events))
	copy(out, r.events)
	return out
}

func (r *recordingSink) kinds() []string {
	out := []string{}
	for _, e := range r.snapshot() {
		out = append(out, e.kind)
	}
	return out
}

// stubRunner simulates a reconciler. ReconcileUID blocks for delay, then
// returns the configured report or error. Cancelling ctx aborts immediately.
// fleetUIDs (when set) is what FleetUIDs returns, so the stub also satisfies
// fleetRunner for the base-select async path.
type stubRunner struct {
	calls      atomic.Int64
	bcastCalls atomic.Int64
	delay      time.Duration
	report     ReconcileReport
	err        error
	fleetUIDs  []string
	fleetErr   error
	bcastErr   error
}

func (s *stubRunner) FleetUIDs(context.Context) ([]string, error) {
	return s.fleetUIDs, s.fleetErr
}

// ApplyBaseBroadcast lets the stub satisfy broadcastRunner for the
// base-select-via-broadcast async path. It records the call and returns
// bcastErr (nil by default = success).
func (s *stubRunner) ApplyBaseBroadcast(context.Context, Broadcaster, ...uint8) (BroadcastReport, error) {
	s.bcastCalls.Add(1)
	return BroadcastReport{}, s.bcastErr
}

func (s *stubRunner) ReconcileUID(ctx context.Context, uid string) (ReconcileReport, error) {
	s.calls.Add(1)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return ReconcileReport{UID: uid}, ctx.Err()
		}
	}
	rpt := s.report
	rpt.UID = uid
	return rpt, s.err
}

// newTestApplier wires a fresh recordingSink into a fresh applier around the
// caller-supplied runner. The runner stays caller-owned so each test can
// configure delay/report/err inline; everything else is boilerplate.
func newTestApplier(_ *testing.T, runner reconcileRunner) (*applier, *recordingSink) {
	sink := &recordingSink{}
	return newApplier(runner, sink), sink
}

// TestOverlayApply_ReturnsQueuedImmediately confirms SetOverlay's response
// path completes well under the 5 s ecu-web ceiling even when the underlying
// reconcile would block for several seconds.
func TestOverlayApply_ReturnsQueuedImmediately(t *testing.T) {
	runner := &stubRunner{delay: 5 * time.Second}
	a, _ := newTestApplier(t, runner)

	uid := "999900000001"
	t0 := time.Now()
	done := a.enqueueInternal(context.Background(), uid, "x", 2, nil)
	elapsed := time.Since(t0)

	if elapsed > 50*time.Millisecond {
		t.Errorf("enqueue blocked for %v; want < 50 ms", elapsed)
	}
	// The goroutine is still running; cancel via supersede so the test
	// doesn't leak a 5 s sleep.
	a.enqueueInternal(context.Background(), uid, "x2", 1, nil)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("superseded goroutine did not exit")
	}
}

// TestOverlayApply_EmitsStartedThenCompleted exercises the happy path: a
// 2-point report with one applied + one in-sync produces started, two
// param_written rows, and a complete row with ok=2 failed=0.
func TestOverlayApply_EmitsStartedThenCompleted(t *testing.T) {
	runner := &stubRunner{
		report: ReconcileReport{
			Points: []PointResult{
				{ApsCode: "BP", Intended: 49.3, Result: "applied"},
				{ApsCode: "BQ", Intended: 50.5, Result: "in_sync"},
			},
		},
	}
	a, sink := newTestApplier(t, runner)

	uid := "999900000001"
	done := a.enqueueInternal(context.Background(), uid, "victron-shift", 2, nil)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("apply did not complete")
	}

	got := sink.snapshot()
	if len(got) != 4 {
		t.Fatalf("want 4 events, got %d: %+v", len(got), got)
	}

	wantKinds := []string{
		"overlay_apply_started",
		"overlay_param_written",
		"overlay_param_written",
		"overlay_apply_complete",
	}
	for i, k := range wantKinds {
		if got[i].kind != k {
			t.Errorf("event[%d].kind = %q, want %q", i, got[i].kind, k)
		}
		if got[i].uid != uid {
			t.Errorf("event[%d].uid = %q, want %q", i, got[i].uid, uid)
		}
	}

	// Severities: info/info/info/info (no failures).
	for i, ev := range got {
		if ev.severity != "info" {
			t.Errorf("event[%d].severity = %q, want info", i, ev.severity)
		}
	}

	// Started detail carries id + uid + points.
	if !strings.Contains(got[0].detail, `id="victron-shift"`) || !strings.Contains(got[0].detail, "points=2") {
		t.Errorf("started detail wrong: %q", got[0].detail)
	}
	// Complete detail carries ok=2 failed=0.
	if !strings.Contains(got[3].detail, "ok=2") || !strings.Contains(got[3].detail, "failed=0") {
		t.Errorf("complete detail wrong: %q", got[3].detail)
	}
	// Param-written rows carry the aps code.
	codes := []string{got[1].detail, got[2].detail}
	sort.Strings(codes)
	if !strings.Contains(codes[0], "code=BP") || !strings.Contains(codes[1], "code=BQ") {
		t.Errorf("param_written details missing codes: %v", codes)
	}
}

// TestOverlayApply_EmitsParamFailedOnWireError exercises the failure path:
// two points where one returns a send_error result; we expect
// overlay_param_failed with severity warn and a final overlay_apply_complete
// with failed>0.
func TestOverlayApply_EmitsParamFailedOnWireError(t *testing.T) {
	runner := &stubRunner{
		report: ReconcileReport{
			Points: []PointResult{
				{ApsCode: "BP", Intended: 49.3, Result: "applied"},
				{ApsCode: "DD", Intended: 16.7, Result: "send_error: network down"},
			},
		},
	}
	a, sink := newTestApplier(t, runner)

	uid := "999900000003"
	done := a.enqueueInternal(context.Background(), uid, "p1", 2, nil)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("apply did not complete")
	}

	got := sink.snapshot()
	var failedRow *recordedEvent
	var completeRow *recordedEvent
	for i := range got {
		if got[i].kind == "overlay_param_failed" {
			failedRow = &got[i]
		}
		if got[i].kind == "overlay_apply_complete" {
			completeRow = &got[i]
		}
	}
	if failedRow == nil {
		t.Fatalf("expected overlay_param_failed; got kinds=%v", sink.kinds())
	}
	if failedRow.severity != "warn" {
		t.Errorf("failed severity = %q, want warn", failedRow.severity)
	}
	if !strings.Contains(failedRow.detail, "code=DD") || !strings.Contains(failedRow.detail, "send_error") {
		t.Errorf("failed detail wrong: %q", failedRow.detail)
	}
	if completeRow == nil {
		t.Fatal("expected overlay_apply_complete")
	}
	if !strings.Contains(completeRow.detail, "failed=1") || !strings.Contains(completeRow.detail, "ok=1") {
		t.Errorf("complete detail wrong: %q", completeRow.detail)
	}
	// failed>0 must surface as severity=warn on the summary row so operators
	// can filter the events table by severity and not miss partial-failure
	// applies (the per-param warn row carries the detail; the summary row
	// signals the rollup).
	if completeRow.severity != "warn" {
		t.Errorf("complete severity = %q, want warn (failed>0)", completeRow.severity)
	}
}

// TestOverlayApply_EmitsParamFailedOnTopLevelError checks that a reconcile
// returning a non-nil error (model unknown, store failure) lands a single
// overlay_param_failed plus a complete row with failed=1.
func TestOverlayApply_EmitsParamFailedOnTopLevelError(t *testing.T) {
	runner := &stubRunner{err: errors.New("model unknown")}
	a, sink := newTestApplier(t, runner)
	done := a.enqueueInternal(context.Background(), "999900000001", "p1", 1, nil)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("apply did not complete")
	}
	kinds := sink.kinds()
	if len(kinds) != 3 || kinds[0] != "overlay_apply_started" ||
		kinds[1] != "overlay_param_failed" || kinds[2] != "overlay_apply_complete" {
		t.Fatalf("unexpected event sequence: %v", kinds)
	}
}

// TestOverlayApply_Supersedes launches two overlapping applies for the same
// uid; the second cancels the first mid-way, the first emits nothing after
// the supersession, and the second runs to completion.
func TestOverlayApply_Supersedes(t *testing.T) {
	// One runner with a short delay long enough that we can launch a second
	// apply before the first completes. Both applies use the same runner so
	// there is no mid-flight write to applier.rec (which would race with the
	// first goroutine's read).
	runner := &stubRunner{
		delay: 250 * time.Millisecond,
		report: ReconcileReport{
			Points: []PointResult{{ApsCode: "BP", Intended: 49.3, Result: "applied"}},
		},
	}
	a, sink := newTestApplier(t, runner)

	uid := "999900000001"
	d1 := a.enqueueInternal(context.Background(), uid, "old", 1, nil)

	// Wait a beat so the started event lands and the first goroutine is
	// inside ReconcileUID. Then enqueue the supersede.
	time.Sleep(50 * time.Millisecond)
	d2 := a.enqueueInternal(context.Background(), uid, "new", 1, nil)

	// First goroutine should exit promptly via its cancelled context.
	select {
	case <-d1:
	case <-time.After(2 * time.Second):
		t.Fatal("superseded goroutine did not exit")
	}
	select {
	case <-d2:
	case <-time.After(2 * time.Second):
		t.Fatal("replacement apply did not complete")
	}

	// The "old" apply should have emitted only the started event (and nothing
	// after the supersede), and the "new" apply should have emitted started +
	// param_written + complete.
	var oldStarted, newCompleted bool
	for _, e := range sink.snapshot() {
		if e.kind == "overlay_apply_started" && strings.Contains(e.detail, `id="old"`) {
			oldStarted = true
		}
		if e.kind == "overlay_apply_complete" && strings.Contains(e.detail, `id="old"`) {
			t.Errorf("superseded apply must NOT emit complete: %q", e.detail)
		}
		if e.kind == "overlay_param_written" && strings.Contains(e.detail, `id="old"`) {
			t.Errorf("superseded apply must NOT emit param_written after cancel: %q", e.detail)
		}
		if e.kind == "overlay_apply_complete" && strings.Contains(e.detail, `id="new"`) {
			newCompleted = true
		}
	}
	if !oldStarted {
		t.Error("expected the original apply to emit started")
	}
	if !newCompleted {
		t.Errorf("expected the replacement apply to complete; events=%v", sink.kinds())
	}
}

// TestOverlayApply_EmitsOnlyOverlayPoints verifies the scoped-emission fix.
// The reconciler's report carries 5 PointResults (3 base + 2 overlay), but
// the applier is given an overlay with only 2 points (BP/BQ) — exactly 2
// overlay_param_* events fire and the _started / _complete summaries report
// points=2 / ok=2 / failed=0. Base-profile points MUST NOT produce events,
// even if they appear in the reconciler's merged report.
func TestOverlayApply_EmitsOnlyOverlayPoints(t *testing.T) {
	runner := &stubRunner{
		report: ReconcileReport{
			Points: []PointResult{
				// 3 base-profile points (the merged effective report carries the
				// full set even though the overlay only touches 2)
				{ApsCode: "AB", Intended: 264, Result: "in_sync"},
				{ApsCode: "AC", Intended: 253, Result: "in_sync"},
				{ApsCode: "AD", Intended: 207, Result: "in_sync"},
				// 2 overlay-owned points
				{ApsCode: "BP", Intended: 49.3, Result: "in_sync"},
				{ApsCode: "BQ", Intended: 50.5, Result: "applied"},
			},
		},
	}
	a, sink := newTestApplier(t, runner)

	uid := "999900000001"
	done := a.enqueueWithPoints(context.Background(), uid, "ds3-reconnect-widen", []string{"BP", "BQ"})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("apply did not complete")
	}

	got := sink.snapshot()
	// Expect exactly: started + 2 param_written + complete = 4 events.
	if len(got) != 4 {
		t.Fatalf("want 4 events, got %d: %+v", len(got), got)
	}

	// No overlay_param_* events for base-profile codes.
	for _, e := range got {
		if e.kind == "overlay_param_written" || e.kind == "overlay_param_failed" {
			for _, baseCode := range []string{"AB", "AC", "AD"} {
				if strings.Contains(e.detail, "code="+baseCode) {
					t.Errorf("base-profile code %q must not produce overlay_param_* event: %q",
						baseCode, e.detail)
				}
			}
		}
	}

	// Per-point events present for both overlay codes.
	codes := map[string]bool{}
	for _, e := range got {
		if e.kind == "overlay_param_written" {
			for _, c := range []string{"BP", "BQ"} {
				if strings.Contains(e.detail, "code="+c) {
					codes[c] = true
				}
			}
		}
	}
	if !codes["BP"] || !codes["BQ"] {
		t.Errorf("expected param_written for BP and BQ; got %v", codes)
	}

	// _started agrees: points=2.
	if got[0].kind != "overlay_apply_started" || !strings.Contains(got[0].detail, "points=2") {
		t.Errorf("started should report points=2; got %q", got[0].detail)
	}
	// _complete agrees: ok=2 failed=0.
	if got[3].kind != "overlay_apply_complete" ||
		!strings.Contains(got[3].detail, "ok=2") ||
		!strings.Contains(got[3].detail, "failed=0") {
		t.Errorf("complete should report ok=2 failed=0; got %q", got[3].detail)
	}
}

// TestOverlayApply_AllOverlayParamsInReport: when every overlay code is in
// the report AND nothing else is, the scoped counts and emission match the
// pre-scope behaviour exactly. This is the regression-protector for the
// happy-path equivalence: scoping a report that ALREADY only contains
// overlay codes must produce the same event sequence as before.
func TestOverlayApply_AllOverlayParamsInReport(t *testing.T) {
	runner := &stubRunner{
		report: ReconcileReport{
			Points: []PointResult{
				{ApsCode: "BP", Intended: 49.3, Result: "applied"},
				{ApsCode: "BQ", Intended: 50.5, Result: "in_sync"},
			},
		},
	}
	a, sink := newTestApplier(t, runner)

	done := a.enqueueWithPoints(context.Background(), "999900000001", "ov", []string{"BP", "BQ"})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("apply did not complete")
	}
	kinds := sink.kinds()
	wantKinds := []string{
		"overlay_apply_started",
		"overlay_param_written",
		"overlay_param_written",
		"overlay_apply_complete",
	}
	if len(kinds) != len(wantKinds) {
		t.Fatalf("want %d events, got %d: %v", len(wantKinds), len(kinds), kinds)
	}
	for i, k := range wantKinds {
		if kinds[i] != k {
			t.Errorf("kinds[%d] = %q, want %q", i, kinds[i], k)
		}
	}
	last := sink.snapshot()[len(kinds)-1]
	if !strings.Contains(last.detail, "ok=2") || !strings.Contains(last.detail, "failed=0") {
		t.Errorf("complete detail wrong: %q", last.detail)
	}
}

// TestOverlayApply_EmitsCompleteOnShutdown verifies that cancelling the
// daemon's parent context mid-apply emits a terminal overlay_apply_complete
// row with severity=warn and "aborted: shutdown" in the detail — so the
// audit log never has a started row without a matching terminal row even
// across daemon restarts. Distinct from supersession, which is intentionally
// silent (the newer apply emits its own events).
func TestOverlayApply_EmitsCompleteOnShutdown(t *testing.T) {
	runner := &stubRunner{
		delay: 2 * time.Second, // longer than the parent-cancel wait below
		report: ReconcileReport{
			Points: []PointResult{{ApsCode: "BP", Intended: 49.3, Result: "applied"}},
		},
	}
	a, sink := newTestApplier(t, runner)

	parent, cancel := context.WithCancel(context.Background())
	uid := "999900000001"
	done := a.enqueueInternal(parent, uid, "ov-shut", 1, nil)

	// Let the started event land + the goroutine enter ReconcileUID.
	time.Sleep(100 * time.Millisecond)
	cancel() // simulate daemon shutdown

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown-aborted goroutine did not exit")
	}

	var complete *recordedEvent
	for _, e := range sink.snapshot() {
		if e.kind == "overlay_apply_complete" {
			ev := e
			complete = &ev
		}
	}
	if complete == nil {
		t.Fatalf("expected overlay_apply_complete on shutdown; got kinds=%v", sink.kinds())
	}
	if complete.severity != "warn" {
		t.Errorf("shutdown complete severity = %q, want warn", complete.severity)
	}
	if !strings.Contains(complete.detail, "aborted: shutdown") {
		t.Errorf("shutdown complete detail = %q, want substring 'aborted: shutdown'", complete.detail)
	}
}

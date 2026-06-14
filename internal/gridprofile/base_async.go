package gridprofile

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/bolkedebruin/openaps/internal/buslock"
)

// broadcastBusOwner is the owner label the broadcast path registers with the
// shared bus guard. A const so a typo is a compile error, not a silent
// owner-string mismatch in diagnostics.
const broadcastBusOwner = "gridprofile-broadcast"

// fleetRunner is the slice of the reconciler the fleet applier needs: the
// per-uid reconcile plus the fleet enumeration. The production type is
// *Reconciler; tests substitute a stub.
type fleetRunner interface {
	reconcileRunner
	FleetUIDs(ctx context.Context) ([]string, error)
}

// broadcastRunner extends fleetRunner with the whole-profile broadcast apply.
// The production type is *Reconciler; tests substitute a stub.
type broadcastRunner interface {
	fleetRunner
	ApplyBaseBroadcast(ctx context.Context, b Broadcaster, modelCodes ...uint8) (BroadcastReport, error)
}

// broadcastJob carries the parameters of one async base-select-via-broadcast.
// The reconcile work runs through the applier's own runner (a.rec), which the
// manager wires to the production *Reconciler and tests substitute with a stub.
type broadcastJob struct {
	sender     Broadcaster
	modelCodes []uint8
	// lock, when non-nil, serialises the broadcast bus disruption against the
	// pairing state machine. Acquired INSIDE the goroutine so the IPC roundtrip
	// returns promptly; a "bus busy" rejection is surfaced as an event, not a
	// blocking error.
	lock  *buslock.Lock
	label string
}

// enqueueFleet starts a background fleet-wide reconcile (a base-profile select
// or an overlay clear that touches the whole effective profile) and returns
// immediately. Progress is reported via the audit/events stream:
//   - one fleet-level profile_apply_started row,
//   - per-uid overlay_apply_started / overlay_param_* / overlay_apply_complete
//     rows (the same rows a per-uid overlay apply emits — the events stream is
//     uniform across base and overlay applies),
//   - one terminal profile_apply_complete row.
//
// Each uid is reconciled through the SAME per-uid serialization the overlay
// applier uses (enqueue keyed by uid), so a base-select reconcile never races
// an overlapping overlay apply for the same inverter — the newer job supersedes
// the older one, per-uid. label scopes the fleet-level summary rows ("base
// select" / "overlay clear: <uid>").
//
// run() of the per-uid jobs uses scopedCodes=nil so it emits results for every
// reconciled point (a base apply has no overlay-owned subset to filter to).
//
// A rapid burst of base-selects spawns one runFleet goroutine each; they are
// not deduplicated, but they are self-limiting: per-uid jobs supersede each
// other (the older job's done channel closes, unblocking the older runFleet so
// it exits) and per-uid concurrency stays capped at the semaphore. The only
// caller is the auth-gated UI, so the transient churn is acceptable.
func (a *applier) enqueueFleet(parent context.Context, run fleetRunner, label string) {
	go a.runFleet(parent, run, label)
}

// runFleet enumerates the fleet, enqueues a per-uid reconcile for each member,
// waits for them all, and emits the fleet-level summary rows.
func (a *applier) runFleet(parent context.Context, run fleetRunner, label string) {
	a.emit(parent, "", "profile_apply_started", "info", label)

	uids, err := run.FleetUIDs(parent)
	if err != nil {
		log.Printf("gridprofile: fleet apply %q: enumerate fleet: %v", label, err)
		a.emit(context.Background(), "", "profile_apply_complete", "warn",
			fmt.Sprintf("%s — enumerate fleet failed: %v", label, err))
		return
	}

	dones := make([]<-chan struct{}, 0, len(uids))
	for _, uid := range uids {
		// pointCount==0 / scopedCodes==nil: a base reconcile has no overlay-owned
		// point subset, so the per-uid run emits every reconciled point's result.
		dones = append(dones, a.enqueueInternal(parent, uid, label, 0, nil))
	}

	// Wait for every per-uid apply to finish (or the parent to cancel) so the
	// terminal profile_apply_complete marks the true end of the fleet apply.
	for _, d := range dones {
		select {
		case <-d:
		case <-parent.Done():
			if errors.Is(context.Cause(parent), errSuperseded) {
				return
			}
			a.emit(context.Background(), "", "profile_apply_complete", "warn",
				fmt.Sprintf("%s — aborted: shutdown", label))
			return
		}
	}

	a.emit(parent, "", "profile_apply_complete", "info",
		fmt.Sprintf("%s — reconciled %d inverter(s)", label, len(uids)))
}

// enqueueBroadcastFleet starts a background base-select-via-broadcast and
// returns immediately. The goroutine acquires the BusLock (if any) for its
// lifetime, pushes the active base over broadcast, then runs the per-uid
// unicast follow-up reconcile through the same per-uid serialization the
// overlay/fleet appliers use. Progress is reported via the events stream.
//
// The BusLock is acquired INSIDE the goroutine — the IPC caller never blocks
// on it. A "bus busy" rejection surfaces as a profile_apply_complete warn row
// rather than a synchronous error, keeping the IPC roundtrip immediate.
func (a *applier) enqueueBroadcastFleet(parent context.Context, job broadcastJob) {
	go a.runBroadcastFleet(parent, job)
}

// runBroadcastFleet executes the broadcast push followed by the unicast
// follow-up reconcile, holding the BusLock across the broadcast disruption.
func (a *applier) runBroadcastFleet(parent context.Context, job broadcastJob) {
	a.emit(parent, "", "profile_apply_started", "info", job.label)

	run, ok := a.rec.(broadcastRunner)
	if !ok {
		a.emit(context.Background(), "", "profile_apply_complete", "warn",
			fmt.Sprintf("%s — runner does not support broadcast", job.label))
		return
	}

	if job.lock != nil {
		// Fail-fast (TryAcquire, not the priority AcquirePriority the pairing
		// path uses): broadcast jobs are not serialised upstream, so a priority
		// acquire here would let a burst of concurrent broadcasts hold the pause
		// request and starve telemetry. A busy broadcast just reports and retries.
		if ok, owner := job.lock.TryAcquire(broadcastBusOwner); !ok {
			a.emit(context.Background(), "", "profile_apply_complete", "warn",
				fmt.Sprintf("%s — bus busy with %s; not applied", job.label, owner))
			return
		}
		defer job.lock.Release()
	}

	if _, err := run.ApplyBaseBroadcast(parent, job.sender, job.modelCodes...); err != nil {
		log.Printf("gridprofile: fleet broadcast %q: apply: %v", job.label, err)
		a.emit(context.Background(), "", "profile_apply_complete", "warn",
			fmt.Sprintf("%s — broadcast failed: %v", job.label, err))
		return
	}

	// Unicast follow-up: cover unsupported-broadcast points and confirm the
	// full effective state per inverter, through the same per-uid serialization.
	uids, err := run.FleetUIDs(parent)
	if err != nil {
		log.Printf("gridprofile: fleet broadcast %q: enumerate fleet: %v", job.label, err)
		a.emit(context.Background(), "", "profile_apply_complete", "warn",
			fmt.Sprintf("%s — enumerate fleet failed: %v", job.label, err))
		return
	}

	dones := make([]<-chan struct{}, 0, len(uids))
	for _, uid := range uids {
		dones = append(dones, a.enqueueInternal(parent, uid, job.label, 0, nil))
	}
	for _, d := range dones {
		select {
		case <-d:
		case <-parent.Done():
			if errors.Is(context.Cause(parent), errSuperseded) {
				return
			}
			a.emit(context.Background(), "", "profile_apply_complete", "warn",
				fmt.Sprintf("%s — aborted: shutdown", job.label))
			return
		}
	}

	a.emit(parent, "", "profile_apply_complete", "info",
		fmt.Sprintf("%s — broadcast + reconciled %d inverter(s)", job.label, len(uids)))
}

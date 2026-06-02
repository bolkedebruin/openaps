package gridprofile

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// stubSender / stubReadback satisfy the Reconciler's collaborator interfaces
// without doing anything — these tests don't exercise the wire write path,
// they only verify the manager-level fleet-membership gate and the
// boot-time reconcile-pass enqueue.
type stubSender struct{}

func (stubSender) Send(context.Context, string, []byte) error { return nil }

type stubReadback struct{}

func (stubReadback) ReadbackNative(string) (map[string]float64, bool) { return nil, false }
func (stubReadback) TriggerRead(string)                               {}

// newManagerForTest assembles a Manager with a real Store + a Reconciler
// whose modelOf is supplied by the caller. The applier stays uninitialised
// until enqueue/setOverlay is invoked.
func newManagerForTest(t *testing.T, modelOf func(uid string) (uint8, bool)) (*Manager, func()) {
	t.Helper()
	db, cleanup := openTestDB(t)
	st := NewStore(db)
	rec := NewReconciler(st, stubSender{}, stubReadback{}, modelOf, Options{AutoReassert: false})
	m := &Manager{
		Store:      st,
		Reconciler: rec,
		Events:     nil, // tests don't inspect events here
		// ApplyParent left nil — these tests don't exercise long-lived applies.
	}
	return m, cleanup
}

// validOverlayJSON builds a one-point overlay body that satisfies the v1
// schema, with the given uids[] list.
func validOverlayJSON(t *testing.T, id string, uids []string) []byte {
	t.Helper()
	pe, err := BuildOverlayPoint("CB", 50.3)
	if err != nil {
		t.Fatalf("BuildOverlayPoint: %v", err)
	}
	ov := Overlay{
		Schema: SchemaVersion,
		ID:     id,
		UIDs:   uids,
		Points: []PointEntry{pe},
	}
	raw, err := json.Marshal(ov)
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	return raw
}

// TestSetOverlay_RejectsUidNotInFleet checks the fleet-membership gate: any
// uid in the request that modelOf doesn't know is rejected up-front; the
// overlay is NOT persisted and no async apply is queued. This closes the
// bogus-uid growth vector for gp_overlays (operator typo → dead row).
func TestSetOverlay_RejectsUidNotInFleet(t *testing.T) {
	known := "999900000001"
	unknown := "806000000000"

	// modelOf returns true only for the known uid; the unknown uid returns
	// (0, false), simulating a typo or a never-seen device.
	modelOf := func(uid string) (uint8, bool) {
		if uid == known {
			return 0x20, true
		}
		return 0, false
	}
	m, cleanup := newManagerForTest(t, modelOf)
	defer cleanup()

	ctx := context.Background()
	body := validOverlayJSON(t, "victron-shift", []string{known, unknown})

	resp := m.setOverlay(ctx, known, body)
	if resp.GetOk() {
		t.Fatalf("setOverlay should have failed on unknown uid, got Ok=true: %+v", resp)
	}
	if !strings.Contains(resp.GetError(), unknown) || !strings.Contains(resp.GetError(), "not in fleet") {
		t.Errorf("error should name the rejected uid + reason, got %q", resp.GetError())
	}

	// No row should have been inserted (Get must fail with ErrNoRows or
	// equivalent).
	if _, err := m.Store.GetOverlay(ctx, known); err == nil {
		t.Error("overlay must NOT be persisted when fleet-membership check fails")
	}
	if _, err := m.Store.GetOverlay(ctx, unknown); err == nil {
		t.Error("overlay must NOT be persisted for unknown uid either")
	}

	// And no job was enqueued: the applier should never have been created.
	if m.applierInst != nil && len(m.applierInst.jobs) != 0 {
		t.Errorf("expected no in-flight jobs; got %d", len(m.applierInst.jobs))
	}
}

// TestSetOverlay_AcceptsAllUidsInFleet is the positive control: when every
// uid is known, setOverlay persists + enqueues normally.
func TestSetOverlay_AcceptsAllUidsInFleet(t *testing.T) {
	a := "999900000001"
	b := "999900000003"
	modelOf := func(uid string) (uint8, bool) {
		if uid == a || uid == b {
			return 0x20, true
		}
		return 0, false
	}
	m, cleanup := newManagerForTest(t, modelOf)
	defer cleanup()

	ctx := context.Background()
	body := validOverlayJSON(t, "victron-shift", []string{a, b})

	resp := m.setOverlay(ctx, a, body)
	if !resp.GetOk() {
		t.Fatalf("setOverlay should have succeeded: %q", resp.GetError())
	}
	// Persisted.
	if _, err := m.Store.GetOverlay(ctx, a); err != nil {
		t.Errorf("overlay should be persisted: %v", err)
	}
}

// TestReconcileAllOverlays_EnqueuesEachPersistedOverlay verifies that on
// boot, every (uid, overlay) pair persisted in gp_overlays is enqueued for
// async apply, but uids absent from the fleet (modelOf=false) are skipped.
//
// Seeds two overlays — one targeting two uids both in fleet, one targeting a
// mix — then asserts the enqueue calls match the in-fleet uids only.
func TestReconcileAllOverlays_EnqueuesEachPersistedOverlay(t *testing.T) {
	a := "999900000001"
	b := "999900000003"
	gone := "ffffffffffff" // persisted overlay row but not in fleet anymore

	modelOf := func(uid string) (uint8, bool) {
		if uid == a || uid == b {
			return 0x20, true
		}
		return 0, false
	}
	m, cleanup := newManagerForTest(t, modelOf)
	defer cleanup()

	ctx := context.Background()
	// Seed two distinct overlays directly via the store (bypassing the
	// fleet-membership gate so we can also exercise the in-fleet-only skip
	// for "gone").
	ov1 := mustOverlay(t, "ov1", []string{a, b})
	ov2 := mustOverlay(t, "ov2", []string{a, gone})
	if err := m.Store.UpsertOverlay(ctx, a, ov1); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	if err := m.Store.UpsertOverlay(ctx, b, ov1); err != nil {
		t.Fatalf("seed b: %v", err)
	}
	// One overlay row persisted under "gone" too (typical post-decommission
	// state). The pass should not enqueue against gone but SHOULD against a.
	if err := m.Store.UpsertOverlay(ctx, gone, ov2); err != nil {
		t.Fatalf("seed gone: %v", err)
	}

	// Pre-construct the applier with a fast no-op runner + recording sink so
	// ReconcileAllOverlays uses it directly (asyncApplier()'s sync.Once is
	// satisfied by the matching applierInst assignment). This lets us count
	// real enqueue → ReconcileUID hops without running any real wire I/O.
	noopRunner := &stubRunner{report: ReconcileReport{}}
	m.applierInst = newApplier(noopRunner, &recordingSink{})
	m.applierOnce.Do(func() {}) // mark Once as fired so asyncApplier returns applierInst as-is

	if err := m.ReconcileAllOverlays(ctx); err != nil {
		t.Fatalf("ReconcileAllOverlays: %v", err)
	}
	// Give the goroutines a moment to drain (each just runs the noop runner).
	time.Sleep(150 * time.Millisecond)

	// noopRunner.calls.Load() counts each ReconcileUID invocation, which equals one
	// enqueue per (uid, overlay) pair that passed the fleet gate.
	// Expected in-fleet enqueues:
	//   - (a, ov1) [ov1 targets a,b → a in-fleet]
	//   - (b, ov1) [b in-fleet]
	//   - (a, ov2) [ov2 targets a,gone → a in-fleet]
	// Excluded:
	//   - (gone, *) — not in fleet, skipped.
	// Note: ListOverlays groups by id (DISTINCT id), so each overlay
	// contributes len(in-fleet uids).
	got := noopRunner.calls.Load()
	// ListOverlays + GROUP BY id returns each overlay once with its body
	// (which includes the full uids list). For ov1: uids=[a,b], both
	// in-fleet → 2 enqueues. For ov2: uids=[a,gone], one in-fleet → 1.
	// Total expected = 3.
	if got != 3 {
		t.Errorf("expected 3 enqueues (in-fleet uids across both overlays); got %d", got)
	}
}

// TestReconcileOverlaysForUID_NoOverlay verifies the per-uid hook is a no-op
// when no overlay is persisted for the uid (the common case at boot for
// uids without a Local Site profile). No applier instance is created and no
// jobs are enqueued.
func TestReconcileOverlaysForUID_NoOverlay(t *testing.T) {
	uid := "999900000001"
	modelOf := func(u string) (uint8, bool) {
		if u == uid {
			return 0x20, true
		}
		return 0, false
	}
	m, cleanup := newManagerForTest(t, modelOf)
	defer cleanup()

	// Pre-construct the applier with a counting runner so we can assert no
	// enqueue happened (calls would be incremented by ReconcileUID).
	noopRunner := &stubRunner{report: ReconcileReport{}}
	m.applierInst = newApplier(noopRunner, &recordingSink{})
	m.applierOnce.Do(func() {})

	if err := m.ReconcileOverlaysForUID(context.Background(), uid); err != nil {
		t.Fatalf("ReconcileOverlaysForUID: %v", err)
	}
	// Give any (incorrect) goroutine a chance to fire.
	time.Sleep(50 * time.Millisecond)
	if noopRunner.calls.Load() != 0 {
		t.Errorf("expected 0 enqueues for uid with no overlay; got %d", noopRunner.calls.Load())
	}
}

// TestReconcileOverlaysForUID_EnqueuesOverlay seeds an overlay for a uid in
// the fleet and verifies the per-uid hook enqueues exactly one apply, with
// the overlay's id (which the runner sees indirectly via call count + sink
// events).
func TestReconcileOverlaysForUID_EnqueuesOverlay(t *testing.T) {
	uid := "999900000001"
	modelOf := func(u string) (uint8, bool) {
		if u == uid {
			return 0x20, true
		}
		return 0, false
	}
	m, cleanup := newManagerForTest(t, modelOf)
	defer cleanup()

	ctx := context.Background()
	ov := mustOverlay(t, "victron-shift", []string{uid})
	if err := m.Store.UpsertOverlay(ctx, uid, ov); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Pre-construct the applier with a counting runner.
	noopRunner := &stubRunner{report: ReconcileReport{}}
	sink := &recordingSink{}
	m.applierInst = newApplier(noopRunner, sink)
	m.applierOnce.Do(func() {})

	if err := m.ReconcileOverlaysForUID(ctx, uid); err != nil {
		t.Fatalf("ReconcileOverlaysForUID: %v", err)
	}
	// Drain the goroutine.
	time.Sleep(150 * time.Millisecond)

	if noopRunner.calls.Load() != 1 {
		t.Errorf("expected 1 enqueue; got %d", noopRunner.calls.Load())
	}

	// Verify the started event has the overlay's id.
	var started *recordedEvent
	for _, e := range sink.snapshot() {
		if e.kind == "overlay_apply_started" {
			ev := e
			started = &ev
		}
	}
	if started == nil {
		t.Fatalf("expected overlay_apply_started; got kinds=%v", sink.kinds())
	}
	if !strings.Contains(started.detail, `id="victron-shift"`) {
		t.Errorf("started event should carry overlay id; got %q", started.detail)
	}
	// points=1 — the overlay has one point (CB).
	if !strings.Contains(started.detail, "points=1") {
		t.Errorf("started event should report points=1 (the overlay's point count); got %q", started.detail)
	}
}

// TestReconcileOverlaysForUID_SkipsUidNotInFleet: when modelOf returns false
// the hook is a no-op. Mirrors the fleet-membership policy in setOverlay /
// ReconcileAllOverlays.
func TestReconcileOverlaysForUID_SkipsUidNotInFleet(t *testing.T) {
	// modelOf returns false for every uid.
	modelOf := func(string) (uint8, bool) { return 0, false }
	m, cleanup := newManagerForTest(t, modelOf)
	defer cleanup()

	// Even seed an overlay row (simulating a stale row from a prior fleet)
	// — the hook still must skip it.
	uid := "ffffffffffff"
	ov := mustOverlay(t, "ghost", []string{uid})
	if err := m.Store.UpsertOverlay(context.Background(), uid, ov); err != nil {
		t.Fatalf("seed: %v", err)
	}

	noopRunner := &stubRunner{}
	m.applierInst = newApplier(noopRunner, &recordingSink{})
	m.applierOnce.Do(func() {})

	if err := m.ReconcileOverlaysForUID(context.Background(), uid); err != nil {
		t.Fatalf("ReconcileOverlaysForUID: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if noopRunner.calls.Load() != 0 {
		t.Errorf("expected 0 enqueues when uid is not in fleet; got %d", noopRunner.calls.Load())
	}
}

// mustOverlay builds an overlay with one valid point or fails the test.
func mustOverlay(t *testing.T, id string, uids []string) Overlay {
	t.Helper()
	pe, err := BuildOverlayPoint("CB", 50.3)
	if err != nil {
		t.Fatalf("BuildOverlayPoint: %v", err)
	}
	return Overlay{
		Schema: SchemaVersion,
		ID:     id,
		UIDs:   uids,
		Points: []PointEntry{pe},
	}
}

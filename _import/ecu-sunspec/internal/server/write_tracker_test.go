package server

import (
	"testing"
	"time"

	"github.com/bolke/ecu-sunspec/internal/source"
)

func TestWriteTracker_DefaultIsCompleted(t *testing.T) {
	wt := NewWriteTracker()
	rslt, req := wt.Get(2, 711)
	if rslt != WriteRsltCompleted {
		t.Errorf("default rslt=%d want COMPLETED(%d)", rslt, WriteRsltCompleted)
	}
	if req != 0 {
		t.Errorf("default reqCounter=%d want 0", req)
	}
}

func TestWriteTracker_TrackThenIncomplete(t *testing.T) {
	wt := NewWriteTracker()
	wt.Track(2, 711, map[string]float64{"DC": 50.5})
	rslt, req := wt.Get(2, 711)
	if rslt != WriteRsltInProgress {
		t.Errorf("rslt=%d want IN_PROGRESS(%d)", rslt, WriteRsltInProgress)
	}
	if req != 1 {
		t.Errorf("reqCounter=%d want 1", req)
	}
}

func TestWriteTracker_ReconcileMatch(t *testing.T) {
	wt := NewWriteTracker()
	wt.Track(2, 711, map[string]float64{"DC": 50.5})

	snap := source.Snapshot{
		Inverters: []source.Inverter{{UID: "INV-A"}},
		Protection: map[string]source.ProtectionParams{
			"INV-A": {OFDroopStart: 50.5, Has: map[string]bool{"DC": true}},
		},
	}
	wt.Reconcile(snap)

	rslt, _ := wt.Get(2, 711)
	if rslt != WriteRsltCompleted {
		t.Errorf("after reconcile rslt=%d want COMPLETED", rslt)
	}
}

func TestWriteTracker_ReconcileMismatchKeepsInProgress(t *testing.T) {
	wt := NewWriteTracker()
	wt.Track(2, 711, map[string]float64{"DC": 50.5})

	// Inverter still reports the old value.
	snap := source.Snapshot{
		Inverters: []source.Inverter{{UID: "INV-A"}},
		Protection: map[string]source.ProtectionParams{
			"INV-A": {OFDroopStart: 50.2, Has: map[string]bool{"DC": true}},
		},
	}
	wt.Reconcile(snap)

	rslt, _ := wt.Get(2, 711)
	if rslt != WriteRsltInProgress {
		t.Errorf("rslt=%d want IN_PROGRESS (mismatch within deadline)", rslt)
	}
}

func TestWriteTracker_ReconcileFailsAfterDeadline(t *testing.T) {
	wt := NewWriteTracker()
	wt.Track(2, 711, map[string]float64{"DC": 50.5})

	// Force an expired deadline.
	wt.mu.Lock()
	wt.entries[trackerKey{uid: 2, model: 711}].deadline = time.Now().Add(-1 * time.Second)
	wt.mu.Unlock()

	snap := source.Snapshot{
		Inverters: []source.Inverter{{UID: "INV-A"}},
		Protection: map[string]source.ProtectionParams{
			"INV-A": {OFDroopStart: 50.2, Has: map[string]bool{"DC": true}},
		},
	}
	wt.Reconcile(snap)

	rslt, _ := wt.Get(2, 711)
	if rslt != WriteRsltFailed {
		t.Errorf("rslt=%d want FAILED (mismatch + past deadline)", rslt)
	}
}

func TestWriteTracker_ReconcileTolerance(t *testing.T) {
	wt := NewWriteTracker()
	wt.Track(2, 711, map[string]float64{"DC": 50.5})

	// Inverter rounds to 50.50001 — should still match within tolerance.
	snap := source.Snapshot{
		Inverters: []source.Inverter{{UID: "INV-A"}},
		Protection: map[string]source.ProtectionParams{
			"INV-A": {OFDroopStart: 50.50001, Has: map[string]bool{"DC": true}},
		},
	}
	wt.Reconcile(snap)

	rslt, _ := wt.Get(2, 711)
	if rslt != WriteRsltCompleted {
		t.Errorf("rslt=%d want COMPLETED (within tolerance)", rslt)
	}
}

func TestWriteTracker_ReconcileMultipleFields(t *testing.T) {
	wt := NewWriteTracker()
	wt.Track(2, 711, map[string]float64{
		"DC": 50.5, "DD": 40.0, "CV": 14,
	})

	snap := source.Snapshot{
		Inverters: []source.Inverter{{UID: "INV-A"}},
		Protection: map[string]source.ProtectionParams{
			"INV-A": {
				OFDroopStart: 50.5,
				OFDroopSlope: 40.0,
				OFDroopMode:  14,
				Has:          map[string]bool{"DC": true, "DD": true, "CV": true},
			},
		},
	}
	wt.Reconcile(snap)
	if rslt, _ := wt.Get(2, 711); rslt != WriteRsltCompleted {
		t.Errorf("all-match: rslt=%d want COMPLETED", rslt)
	}

	// Now mutate one field — should drop back to mismatch (no new Track).
	wt2 := NewWriteTracker()
	wt2.Track(2, 711, map[string]float64{"DC": 50.5, "DD": 40.0, "CV": 14})
	snap.Protection["INV-A"] = source.ProtectionParams{
		OFDroopStart: 50.5, OFDroopSlope: 40.0, OFDroopMode: 15, // wrong
		Has: map[string]bool{"DC": true, "DD": true, "CV": true},
	}
	wt2.Reconcile(snap)
	if rslt, _ := wt2.Get(2, 711); rslt != WriteRsltInProgress {
		t.Errorf("one-field-mismatch: rslt=%d want IN_PROGRESS", rslt)
	}
}

func TestWriteTracker_PruneSettled(t *testing.T) {
	wt := NewWriteTracker()
	wt.Track(2, 711, map[string]float64{"DC": 50.5})

	// Force completion.
	snap := source.Snapshot{
		Inverters: []source.Inverter{{UID: "INV-A"}},
		Protection: map[string]source.ProtectionParams{
			"INV-A": {OFDroopStart: 50.5, Has: map[string]bool{"DC": true}},
		},
	}
	wt.Reconcile(snap)

	// Backdate the deadline so PruneSettled removes it.
	wt.mu.Lock()
	wt.entries[trackerKey{uid: 2, model: 711}].deadline = time.Now().Add(-time.Hour)
	wt.mu.Unlock()

	wt.PruneSettled(time.Minute)
	if len(wt.entries) != 0 {
		t.Errorf("PruneSettled left %d entries; want 0", len(wt.entries))
	}
	// Get on a pruned entry returns the default (COMPLETED, 0).
	if rslt, req := wt.Get(2, 711); rslt != WriteRsltCompleted || req != 0 {
		t.Errorf("post-prune Get = (%d, %d) want (COMPLETED, 0)", rslt, req)
	}
}

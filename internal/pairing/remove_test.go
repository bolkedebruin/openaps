package pairing

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/buslock"
	"github.com/bolkedebruin/openaps/wire"
)

// seedInverter inserts a fleet row so DeleteInverter has something to remove
// and the evict has a serial that exists in inventory.
func seedInverter(t *testing.T, m *Manager, uid string, sa uint16) {
	t.Helper()
	if err := m.Store.SetInverterShortAddr(context.Background(), uid, sa); err != nil {
		t.Fatalf("seed inverter %s: %v", uid, err)
	}
}

// assertDeleted fails if the uid still has an inventory row.
func assertDeleted(t *testing.T, m *Manager, uid string) {
	t.Helper()
	_, err := m.Store.GetInverterPairing(context.Background(), uid)
	if err != sql.ErrNoRows {
		t.Fatalf("inverter %s still present (err=%v); expected deleted", uid, err)
	}
}

func newRemoveManager(t *testing.T, tr *Transport) *Manager {
	t.Helper()
	return &Manager{
		Store: newTestStore(t), Transport: tr, Lock: buslock.New(),
		Events:           &recordingEvents{},
		Settings:         &stubSettings{pan: "0DCE"},
		CurrentChannel:   func() uint32 { return 16 },
		CommitSettle:     10 * time.Millisecond,
		VerifyRetrySleep: 5 * time.Millisecond,
	}
}

// TestRemove_EvictSuccess: force=false, the directed set_inv_pan succeeds, so
// the unit is evicted to 0xFFFF and then deleted.
func TestRemove_EvictSuccess(t *testing.T) {
	tr, mock := newMockTransport()
	mock.opPAN = 0x0DCE
	// get_module_pan must report a non-zero channel so the evict targets it.
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		return &wire.PairingCmdResult{Ok: true}
	}
	m := newRemoveManager(t, tr)
	const uid = "999900000003"
	seedInverter(t, m, uid, 0x1234)

	ev := m.Events.(*recordingEvents)
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_RemoveById{RemoveById: &wire.RemoveById{Serial: uid}}})
	if !resp.GetOk() {
		t.Fatalf("start remove: %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}
	if !final.Evicted {
		t.Fatalf("expected Evicted=true; status message=%q", final.Message)
	}
	// A directed set_inv_pan (the evict) must have been issued.
	sawEvict := false
	for _, c := range mock.cmds {
		if sip := c.GetSetInvPan(); sip != nil {
			sawEvict = true
			if sip.GetPan() != 0xFFFF {
				t.Fatalf("evict pan=0x%X want 0xFFFF", sip.GetPan())
			}
			if sip.GetSerial() != uid {
				t.Fatalf("evict serial=%q want %q", sip.GetSerial(), uid)
			}
		}
	}
	if !sawEvict {
		t.Fatalf("expected a set_inv_pan evict command; got %v", mock.opNames())
	}
	assertDeleted(t, m, uid)
	if !ev.has("inverter_removed") {
		t.Fatalf("expected inverter_removed milestone; got %v", ev.kinds)
	}
}

// TestRemove_EvictFailStillDeletes: force=false but the directed set_inv_pan
// fails. The DB rows MUST still be deleted, Evicted stays false, and the
// terminal message warns the unit may reappear.
func TestRemove_EvictFailStillDeletes(t *testing.T) {
	tr, mock := newMockTransport()
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		if c.GetSetInvPan() != nil {
			return &wire.PairingCmdResult{Ok: false, Error: "no ack from inverter"}
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	m := newRemoveManager(t, tr)
	const uid = "999900000007"
	seedInverter(t, m, uid, 0x4321)

	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_RemoveById{RemoveById: &wire.RemoveById{Serial: uid}}})
	if !resp.GetOk() {
		t.Fatalf("start remove: %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q (a failed evict must NOT abort the removal)", final.Stage, final.Error)
	}
	if final.Evicted {
		t.Fatalf("expected Evicted=false on failed evict")
	}
	if final.Message == "" {
		t.Fatalf("expected a warning message on failed evict")
	}
	// The row is gone despite the evict failure.
	assertDeleted(t, m, uid)
	if !m.Events.(*recordingEvents).has("inverter_evict_failed") {
		t.Fatalf("expected inverter_evict_failed milestone; got %v", m.Events.(*recordingEvents).kinds)
	}
}

// TestRemove_Force: force=true deletes the entry with NO radio op at all.
func TestRemove_Force(t *testing.T) {
	tr, mock := newMockTransport()
	m := newRemoveManager(t, tr)
	const uid = "999900000009"
	seedInverter(t, m, uid, 0x0001)

	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_RemoveById{RemoveById: &wire.RemoveById{Serial: uid, Force: true}}})
	if !resp.GetOk() {
		t.Fatalf("start remove: %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}
	if final.Evicted {
		t.Fatalf("force delete must not report Evicted=true")
	}
	// No radio command of any kind should have been sent on the force path.
	if names := mock.opNames(); len(names) != 0 {
		t.Fatalf("force delete issued radio ops %v; want none", names)
	}
	assertDeleted(t, m, uid)
}

// TestRemove_RejectsBadSerial: a non-12-digit serial is rejected synchronously.
func TestRemove_RejectsBadSerial(t *testing.T) {
	tr, _ := newMockTransport()
	m := newRemoveManager(t, tr)
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_RemoveById{RemoveById: &wire.RemoveById{Serial: "abc"}}})
	if resp.GetOk() {
		t.Fatalf("expected rejection of bad serial")
	}
}

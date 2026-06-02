package pairing

import (
	"context"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/buslock"
	"github.com/bolkedebruin/openaps/wire"
)

// newChannelManager builds a Manager seeded with the given inverter serials
// and a current channel, wired to a fresh mock transport.
func newChannelManager(t *testing.T, mock *mockTransport, tr *Transport, settings *stubSettings, currentChannel uint32, serials ...string) *Manager {
	t.Helper()
	st := newTestStore(t)
	ctx := context.Background()
	for _, uid := range serials {
		if err := st.SetInverterShortAddr(ctx, uid, 0x0001); err != nil {
			t.Fatal(err)
		}
	}
	return &Manager{
		Store: st, Transport: tr, Lock: buslock.New(), Events: &recordingEvents{},
		Settings: settings, CurrentChannel: func() uint32 { return currentChannel },
		CommitSettle: 10 * time.Millisecond, VerifyRetrySleep: 5 * time.Millisecond,
	}
}

func TestChangeChannel_MoveAndVerify(t *testing.T) {
	tr, mock := newMockTransport()
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		if c.GetGetShortAddr() != nil {
			return &wire.PairingCmdResult{Ok: true, ShortAddr: 0x0202}
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	settings := &stubSettings{pan: "0DCE", channel: 16}
	ev := &recordingEvents{}
	m := newChannelManager(t, mock, tr, settings, 16, "999900000003", "999900000001")
	m.Events = ev

	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_ChangeChannel{ChangeChannel: &wire.FleetChangeChannel{Channel: 20}}})
	if !resp.GetOk() {
		t.Fatalf("start change_channel: %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}
	if got := settings.Channel(); got != 20 {
		t.Fatalf("persisted channel = %d want 20", got)
	}
	if !ev.has("channel_change_committed") {
		t.Fatalf("expected channel_change_committed milestone; got %v", ev.kinds)
	}

	// Each inverter must be hopped via a directed set_inv_pan onto the new
	// channel while the PAN stays unchanged (0x0DCE).
	mock.mu.Lock()
	hops := 0
	var moduleMoved bool
	for _, c := range mock.cmds {
		if sip := c.GetSetInvPan(); sip != nil {
			if sip.GetChannel() != 20 || sip.GetPan() != 0x0DCE {
				t.Errorf("set_inv_pan ch=%d pan=0x%X want ch=20 pan=0x0DCE", sip.GetChannel(), sip.GetPan())
			}
			hops++
		}
		if sm := c.GetSetModulePan(); sm != nil && sm.GetChannel() == 20 && sm.GetPan() == 0x0DCE {
			moduleMoved = true
		}
	}
	mock.mu.Unlock()
	if hops != 2 {
		t.Fatalf("set_inv_pan hops = %d want 2", hops)
	}
	if !moduleMoved {
		t.Fatalf("expected a set_module_pan to channel 20 on PAN 0x0DCE")
	}
	// A channel migration must NOT use prime/commit (those are rekey-only).
	for _, op := range mock.opNames() {
		if op == "prime_inv" || op == "commit_pan" {
			t.Fatalf("unexpected rekey primitive %q in channel migration: %v", op, mock.opNames())
		}
	}
}

func TestChangeChannel_RejectSameChannel(t *testing.T) {
	tr, mock := newMockTransport()
	_ = mock
	settings := &stubSettings{pan: "0DCE", channel: 16}
	m := newChannelManager(t, mock, tr, settings, 16, "999900000003")
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_ChangeChannel{ChangeChannel: &wire.FleetChangeChannel{Channel: 16}}})
	if resp.GetOk() {
		t.Fatalf("expected rejection when new channel == current channel")
	}
}

func TestChangeChannel_RejectZeroChannel(t *testing.T) {
	tr, mock := newMockTransport()
	settings := &stubSettings{pan: "0DCE", channel: 16}
	m := newChannelManager(t, mock, tr, settings, 16, "999900000003")
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_ChangeChannel{ChangeChannel: &wire.FleetChangeChannel{Channel: 0}}})
	if resp.GetOk() {
		t.Fatalf("expected rejection for unset (0) target channel")
	}
}

// TestChangeChannel_OutOfRangePassesThrough proves inv-driver no longer
// validates the channel range: an out-of-range channel reaches the wire and
// the failure surfaces from the (simulated) ecu-zb reject, not from a
// driver-side bounds check.
func TestChangeChannel_OutOfRangePassesThrough(t *testing.T) {
	tr, mock := newMockTransport()
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		if sip := c.GetSetInvPan(); sip != nil && sip.GetChannel() > 26 {
			// ecu-zb's checkChannel rejects the out-of-range channel.
			return &wire.PairingCmdResult{Ok: false, Error: "channel 99 out of range"}
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	settings := &stubSettings{pan: "0DCE", channel: 16}
	m := newChannelManager(t, mock, tr, settings, 16, "999900000003")
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_ChangeChannel{ChangeChannel: &wire.FleetChangeChannel{Channel: 99}}})
	// The op STARTS (no driver-side range check) and fails at the wire.
	if !resp.GetOk() {
		t.Fatalf("change_channel should start (range owned by ecu-zb): %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageError {
		t.Fatalf("stage=%q want error from ecu-zb reject", final.Stage)
	}
	// Channel must NOT have been persisted.
	if got := settings.Channel(); got != 16 {
		t.Fatalf("persisted channel = %d want unchanged 16", got)
	}
	// A directed hop with the out-of-range channel must have been attempted.
	mock.mu.Lock()
	sawOOR := false
	for _, c := range mock.cmds {
		if sip := c.GetSetInvPan(); sip != nil && sip.GetChannel() == 99 {
			sawOOR = true
		}
	}
	mock.mu.Unlock()
	if !sawOOR {
		t.Fatalf("expected an out-of-range set_inv_pan to reach the wire")
	}
}

func TestChangeChannel_RollbackOnHopFailure(t *testing.T) {
	tr, mock := newMockTransport()
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		if c.GetSetInvPan() != nil {
			return &wire.PairingCmdResult{Ok: false, Error: "inverter unreachable"}
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	settings := &stubSettings{pan: "0DCE", channel: 16}
	m := newChannelManager(t, mock, tr, settings, 16, "999900000003")
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_ChangeChannel{ChangeChannel: &wire.FleetChangeChannel{Channel: 20}}})
	if !resp.GetOk() {
		t.Fatalf("start: %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageError {
		t.Fatalf("stage=%q want error", final.Stage)
	}
	// Channel must NOT have been persisted (failure before the module move).
	if got := settings.Channel(); got != 16 {
		t.Fatalf("persisted channel = %d want unchanged 16 (rollback)", got)
	}
	// A rollback set_module_pan back to the OLD channel (16) on the SAME PAN
	// must be present.
	mock.mu.Lock()
	sawRollback := false
	for _, c := range mock.cmds {
		if sm := c.GetSetModulePan(); sm != nil && sm.GetChannel() == 16 && sm.GetPan() == 0x0DCE {
			sawRollback = true
		}
	}
	mock.mu.Unlock()
	if !sawRollback {
		t.Fatalf("expected a rollback set_module_pan to channel 16")
	}
}

func TestChangeChannel_PartialVerifyReportsError(t *testing.T) {
	tr, mock := newMockTransport()
	// Permanent verify failure for one specific inverter — rebindVerify
	// retries stragglers (live test showed some QS1As need a few seconds
	// to answer the directed 0x0E after rearm), so a transient single-call
	// failure now succeeds on retry. A permanently-unresponsive inverter is
	// what yields the partial-verify report.
	const failSerial = "999900000003"
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		if g := c.GetGetShortAddr(); g != nil {
			if g.GetSerial() == failSerial {
				return &wire.PairingCmdResult{Ok: false, Error: "no reply"}
			}
			return &wire.PairingCmdResult{Ok: true, ShortAddr: 0x0303}
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	settings := &stubSettings{pan: "0DCE", channel: 16}
	m := newChannelManager(t, mock, tr, settings, 16, "999900000003", "999900000001")
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_ChangeChannel{ChangeChannel: &wire.FleetChangeChannel{Channel: 20}}})
	if !resp.GetOk() {
		t.Fatalf("start: %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageError {
		t.Fatalf("stage=%q want error (partial verify)", final.Stage)
	}
	// The move committed before verify, so the channel IS persisted (no
	// rollback once the fleet has moved).
	if got := settings.Channel(); got != 20 {
		t.Fatalf("persisted channel = %d want 20 (committed before partial verify)", got)
	}
}

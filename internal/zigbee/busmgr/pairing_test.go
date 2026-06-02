package busmgr

import (
	"errors"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/zigbee/modem"
	"github.com/bolkedebruin/openaps/wire"
)

// fakeExecutor records calls and returns canned results.
type fakeExecutor struct {
	setModulePan struct {
		pan uint16
		ch  byte
	}
	commit struct {
		pan uint16
		ch  byte
	}
	primeSerial string
	bindSA      uint16
	scanWindow  time.Duration

	opPAN     uint16
	opChannel byte
	shortAddr uint16
	found     []modem.FoundUnit
	err       error
}

func (f *fakeExecutor) SetModulePan(pan uint16, ch byte) error {
	f.setModulePan.pan, f.setModulePan.ch = pan, ch
	return f.err
}
func (f *fakeExecutor) GetShortAddr(string) (uint16, error)  { return f.shortAddr, f.err }
func (f *fakeExecutor) SetInvPan(string, uint16, byte) error { return f.err }
func (f *fakeExecutor) PrimeInv(serial string, _ uint16, _ byte) error {
	f.primeSerial = serial
	return f.err
}
func (f *fakeExecutor) CommitPan(pan uint16, ch byte) error {
	f.commit.pan, f.commit.ch = pan, ch
	return f.err
}
func (f *fakeExecutor) BindQuiet(sa uint16) error { f.bindSA = sa; return f.err }
func (f *fakeExecutor) GetOperatingPAN() uint16   { return f.opPAN }
func (f *fakeExecutor) GetOperatingChannel() byte { return f.opChannel }
func (f *fakeExecutor) ReportScan(w time.Duration) ([]modem.FoundUnit, error) {
	f.scanWindow = w
	return f.found, f.err
}

// drainResult pulls the next PairingCmdResult the handler enqueued.
func drainResult(t *testing.T, c *Client) *wire.PairingCmdResult {
	t.Helper()
	select {
	case env := <-c.out:
		pr, ok := env.GetBody().(*wire.Envelope_PairingResult)
		if !ok {
			t.Fatalf("enqueued body is %T, want *Envelope_PairingResult", env.GetBody())
		}
		return pr.PairingResult
	case <-time.After(time.Second):
		t.Fatal("no PairingCmdResult enqueued")
		return nil
	}
}

func newTestClient() *Client {
	return &Client{out: make(chan *wire.Envelope, outboundCap)}
}

func TestHandlePairingCmd_NoExecutor(t *testing.T) {
	c := newTestClient()
	c.handlePairingCmd(&wire.PairingCmd{ReqId: 7})
	res := drainResult(t, c)
	if res.ReqId != 7 || res.Ok {
		t.Fatalf("want req_id=7 ok=false, got req_id=%d ok=%v", res.ReqId, res.Ok)
	}
	if res.Error == "" {
		t.Errorf("expected an error message when executor is nil")
	}
}

func TestHandlePairingCmd_SetModulePan(t *testing.T) {
	fe := &fakeExecutor{}
	c := newTestClient()
	c.Pairing = fe
	c.handlePairingCmd(&wire.PairingCmd{
		ReqId: 1,
		Op:    &wire.PairingCmd_SetModulePan{SetModulePan: &wire.SetModulePan{Pan: 0xFFFF, Channel: 0x10}},
	})
	res := drainResult(t, c)
	if !res.Ok || res.ReqId != 1 {
		t.Fatalf("got ok=%v req_id=%d err=%q", res.Ok, res.ReqId, res.Error)
	}
	if fe.setModulePan.pan != 0xFFFF || fe.setModulePan.ch != 0x10 {
		t.Errorf("executor got pan=0x%04X ch=0x%02X", fe.setModulePan.pan, fe.setModulePan.ch)
	}
}

func TestHandlePairingCmd_ReportScan(t *testing.T) {
	fe := &fakeExecutor{found: []modem.FoundUnit{
		{Serial: "999900000003", Encrypted: true},
		{Serial: "123456789012", Encrypted: false},
	}}
	c := newTestClient()
	c.Pairing = fe
	c.handlePairingCmd(&wire.PairingCmd{
		ReqId: 2,
		Op:    &wire.PairingCmd_ReportScan{ReportScan: &wire.ReportIdScan{TimeoutMs: 3000}},
	})
	res := drainResult(t, c)
	if !res.Ok {
		t.Fatalf("scan failed: %q", res.Error)
	}
	if fe.scanWindow != 3*time.Second {
		t.Errorf("window = %v, want 3s", fe.scanWindow)
	}
	if len(res.Found) != 2 {
		t.Fatalf("found %d, want 2", len(res.Found))
	}
	if res.Found[0].GetSerial() != "999900000003" || !res.Found[0].GetEncrypted() {
		t.Errorf("found[0] = %+v", res.Found[0])
	}
	if res.Found[1].GetEncrypted() {
		t.Errorf("found[1] should be plaintext")
	}
}

func TestHandlePairingCmd_ReportScanDefaultWindow(t *testing.T) {
	fe := &fakeExecutor{}
	c := newTestClient()
	c.Pairing = fe
	c.handlePairingCmd(&wire.PairingCmd{
		ReqId: 3,
		Op:    &wire.PairingCmd_ReportScan{ReportScan: &wire.ReportIdScan{}},
	})
	_ = drainResult(t, c)
	if fe.scanWindow != defaultScanWindow {
		t.Errorf("window = %v, want default %v", fe.scanWindow, defaultScanWindow)
	}
}

func TestHandlePairingCmd_GetShortAddr(t *testing.T) {
	fe := &fakeExecutor{shortAddr: 0x0042}
	c := newTestClient()
	c.Pairing = fe
	c.handlePairingCmd(&wire.PairingCmd{
		ReqId: 4,
		Op:    &wire.PairingCmd_GetShortAddr{GetShortAddr: &wire.GetShortAddr{Serial: "999900000003"}},
	})
	res := drainResult(t, c)
	if !res.Ok || res.ShortAddr != 0x0042 {
		t.Fatalf("got ok=%v sa=0x%04X err=%q", res.Ok, res.ShortAddr, res.Error)
	}
}

func TestHandlePairingCmd_CommitAndBind(t *testing.T) {
	fe := &fakeExecutor{}
	c := newTestClient()
	c.Pairing = fe
	c.handlePairingCmd(&wire.PairingCmd{
		ReqId: 5,
		Op:    &wire.PairingCmd_CommitPan{CommitPan: &wire.CommitPanNow{Pan: 0x0DCE, Channel: 0x10}},
	})
	if res := drainResult(t, c); !res.Ok || fe.commit.pan != 0x0DCE {
		t.Fatalf("commit ok=%v pan=0x%04X", res.Ok, fe.commit.pan)
	}
	c.handlePairingCmd(&wire.PairingCmd{
		ReqId: 6,
		Op:    &wire.PairingCmd_BindQuiet{BindQuiet: &wire.BindQuiet{ShortAddr: 0x0042}},
	})
	if res := drainResult(t, c); !res.Ok || fe.bindSA != 0x0042 {
		t.Fatalf("bind ok=%v sa=0x%04X", res.Ok, fe.bindSA)
	}
}

func TestHandlePairingCmd_ExecutorError(t *testing.T) {
	fe := &fakeExecutor{err: errors.New("modem timeout")}
	c := newTestClient()
	c.Pairing = fe
	c.handlePairingCmd(&wire.PairingCmd{
		ReqId: 8,
		Op:    &wire.PairingCmd_PrimeInv{PrimeInv: &wire.PrimeInverterPan{Serial: "999900000003", Pan: 0x0DCE, Channel: 0x10}},
	})
	res := drainResult(t, c)
	if res.Ok {
		t.Fatalf("expected failure")
	}
	if res.Error != "modem timeout" {
		t.Errorf("error = %q, want 'modem timeout'", res.Error)
	}
	if fe.primeSerial != "999900000003" {
		t.Errorf("prime serial = %q", fe.primeSerial)
	}
}

// TestCheckChannel asserts the validation contract directly: 0 (unset/current)
// is allowed, 11..26 are accepted, and anything else nonzero is rejected.
func TestCheckChannel(t *testing.T) {
	for _, ch := range []uint32{0, 11, 16, 26} {
		if err := checkChannel(ch); err != nil {
			t.Errorf("checkChannel(%d) = %v, want nil", ch, err)
		}
	}
	for _, ch := range []uint32{1, 10, 27, 99, 256, 0xFFFFFFFF} {
		if err := checkChannel(ch); err == nil {
			t.Errorf("checkChannel(%d) = nil, want rejection", ch)
		}
	}
}

// TestHandlePairingCmd_SetInvPanChannelGuard confirms set_inv_pan rejects an
// out-of-range channel before reaching the executor. inv-driver no longer
// validates the range; ecu-zb is the authority.
func TestHandlePairingCmd_SetInvPanChannelGuard(t *testing.T) {
	for _, ch := range []uint32{10, 27} {
		fe := &fakeExecutor{}
		c := newTestClient()
		c.Pairing = fe
		c.handlePairingCmd(&wire.PairingCmd{
			ReqId: 20,
			Op: &wire.PairingCmd_SetInvPan{SetInvPan: &wire.SetInverterPanChannel{
				Serial: "999900000003", Pan: 0x0DCE, Channel: ch,
			}},
		})
		res := drainResult(t, c)
		if res.Ok || res.Error == "" {
			t.Fatalf("set_inv_pan channel %d: want rejection, got ok=%v err=%q", ch, res.Ok, res.Error)
		}
	}
	// In-range channel reaches the executor and succeeds.
	fe := &fakeExecutor{}
	c := newTestClient()
	c.Pairing = fe
	c.handlePairingCmd(&wire.PairingCmd{
		ReqId: 21,
		Op: &wire.PairingCmd_SetInvPan{SetInvPan: &wire.SetInverterPanChannel{
			Serial: "999900000003", Pan: 0x0DCE, Channel: 16,
		}},
	})
	if res := drainResult(t, c); !res.Ok {
		t.Fatalf("set_inv_pan channel 16: want ok, got err=%q", res.Error)
	}
}

// TestHandlePairingCmd_SetModulePanChannelGuard confirms set_module_pan rejects
// an out-of-range channel before reaching the executor.
func TestHandlePairingCmd_SetModulePanChannelGuard(t *testing.T) {
	for _, ch := range []uint32{10, 27} {
		fe := &fakeExecutor{}
		c := newTestClient()
		c.Pairing = fe
		c.handlePairingCmd(&wire.PairingCmd{
			ReqId: 22,
			Op:    &wire.PairingCmd_SetModulePan{SetModulePan: &wire.SetModulePan{Pan: 0x0DCE, Channel: ch}},
		})
		res := drainResult(t, c)
		if res.Ok || res.Error == "" {
			t.Fatalf("set_module_pan channel %d: want rejection, got ok=%v err=%q", ch, res.Ok, res.Error)
		}
	}
}

func TestHandlePairingCmd_EmptyOp(t *testing.T) {
	c := newTestClient()
	c.Pairing = &fakeExecutor{}
	c.handlePairingCmd(&wire.PairingCmd{ReqId: 9})
	res := drainResult(t, c)
	if res.Ok || res.Error == "" {
		t.Fatalf("expected error for empty op, got ok=%v err=%q", res.Ok, res.Error)
	}
}

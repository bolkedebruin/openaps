package pairing

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// CmdSender delivers one PairingCmd envelope to the bus backend (ecu-zb)
// and reports whether the enqueue succeeded. It is the only outbound edge
// of the pairing package; in production it wraps ipc.Server.SendToBackend.
type CmdSender interface {
	SendToBackend(backend string, env *wire.Envelope) bool
}

// defaultCmdTimeout bounds a single radio primitive. The slow primitives
// (report_scan, commit_pan ×3 + settle) carry their own longer per-op
// timeout via Transport.do's ctx deadline; this is the floor for the quick
// directed ops (get_short_addr, set_inv_pan, prime_inv, bind_quiet).
const defaultCmdTimeout = 15 * time.Second

// Transport correlates asynchronous PairingCmdResult replies to the
// PairingCmd that produced them. inv-driver assigns a monotonic req_id to
// every command, registers a one-shot result channel under that id, emits
// the command onto the ecu-zb publisher's downstream out channel, and parks
// on the channel until the matching result is routed back by the ingest
// loop (or the per-primitive timeout fires).
//
// Exactly one result is delivered per req_id; the registry entry is removed
// on delivery or timeout so a late/duplicate reply is dropped.
type Transport struct {
	// Sender routes the PairingCmd to the bus backend.
	Sender CmdSender
	// Backend is the publisher backend name (the active ecu-zb).
	Backend string
	// Timeout bounds a single primitive when the caller's context has no
	// earlier deadline. Zero falls back to defaultCmdTimeout.
	Timeout time.Duration

	reqSeq atomic.Uint64

	mu      sync.Mutex
	waiters map[uint64]chan *wire.PairingCmdResult
}

// NewTransport builds a Transport bound to a sender + backend.
func NewTransport(sender CmdSender, backend string) *Transport {
	return &Transport{
		Sender:  sender,
		Backend: backend,
		waiters: make(map[uint64]chan *wire.PairingCmdResult),
	}
}

// Deliver routes an inbound PairingCmdResult to the goroutine waiting on its
// req_id. Returns true if a waiter was found (and the result delivered),
// false if no waiter is registered (late/duplicate/unknown id). Called from
// the ingest loop when it sees an Envelope_PairingResult.
func (t *Transport) Deliver(res *wire.PairingCmdResult) bool {
	if res == nil {
		return false
	}
	t.mu.Lock()
	ch, ok := t.waiters[res.GetReqId()]
	if ok {
		delete(t.waiters, res.GetReqId())
	}
	t.mu.Unlock()
	if !ok {
		return false
	}
	// Buffered channel (cap 1) — never blocks; the waiter may already be
	// gone on a race with timeout, in which case the send is a no-op via
	// the buffer slot that is then GC'd.
	ch <- res
	return true
}

// do assigns a req_id, fills it into cmd, registers a waiter, sends the
// command, and blocks for the result (or ctx/timeout). The returned result
// is never nil on a nil error.
func (t *Transport) do(ctx context.Context, cmd *wire.PairingCmd) (*wire.PairingCmdResult, error) {
	if t.Sender == nil || t.Backend == "" {
		return nil, fmt.Errorf("pairing transport: no sender/backend configured")
	}
	reqID := t.reqSeq.Add(1)
	cmd.ReqId = reqID

	ch := make(chan *wire.PairingCmdResult, 1)
	t.mu.Lock()
	if t.waiters == nil {
		t.waiters = make(map[uint64]chan *wire.PairingCmdResult)
	}
	t.waiters[reqID] = ch
	t.mu.Unlock()

	// Ensure the waiter is always removed even on the error paths below.
	cleanup := func() {
		t.mu.Lock()
		delete(t.waiters, reqID)
		t.mu.Unlock()
	}

	env := &wire.Envelope{Body: &wire.Envelope_PairingCmd{PairingCmd: cmd}}
	if !t.Sender.SendToBackend(t.Backend, env) {
		cleanup()
		return nil, fmt.Errorf("pairing transport: send to backend %q failed (absent or queue full)", t.Backend)
	}

	timeout := t.Timeout
	if timeout <= 0 {
		timeout = defaultCmdTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		cleanup()
		return nil, ctx.Err()
	case <-timer.C:
		cleanup()
		return nil, fmt.Errorf("pairing transport: req_id=%d timed out after %s", reqID, timeout)
	case res := <-ch:
		// Waiter already removed by Deliver.
		if !res.GetOk() {
			return res, fmt.Errorf("pairing primitive failed: %s", res.GetError())
		}
		return res, nil
	}
}

// The primitive wrappers below build the typed PairingCmd oneof and run it
// through do. Each maps 1:1 to an ecu-zb radio op.

// setModulePan parks the local module on pan/channel (0x05). pan may be
// 0xFFFF for rendezvous.
func (t *Transport) setModulePan(ctx context.Context, pan, channel uint32) error {
	_, err := t.do(ctx, &wire.PairingCmd{Op: &wire.PairingCmd_SetModulePan{
		SetModulePan: &wire.SetModulePan{Pan: pan, Channel: channel}}})
	return err
}

// reportScan runs the 0xD1-on / collect-0x1D / 0xD2-off discovery window and
// returns the announcing inverters.
func (t *Transport) reportScan(ctx context.Context, timeoutMs uint32) ([]*wire.FoundInverter, error) {
	res, err := t.do(ctx, &wire.PairingCmd{Op: &wire.PairingCmd_ReportScan{
		ReportScan: &wire.ReportIdScan{TimeoutMs: timeoutMs}}})
	if err != nil {
		return nil, err
	}
	return res.GetFound(), nil
}

// getShortAddr queries one inverter (by serial) for its assigned short
// address (0x0E).
func (t *Transport) getShortAddr(ctx context.Context, serial string) (uint16, error) {
	res, err := t.do(ctx, &wire.PairingCmd{Op: &wire.PairingCmd_GetShortAddr{
		GetShortAddr: &wire.GetShortAddr{Serial: serial}}})
	if err != nil {
		return 0, err
	}
	sa := res.GetShortAddr()
	if sa > 0xFFFF {
		return 0, fmt.Errorf("get_short_addr %s: short_addr 0x%X exceeds uint16", serial, sa)
	}
	return uint16(sa), nil
}

// setInvPan forces one inverter onto pan/channel directed (0x0F).
func (t *Transport) setInvPan(ctx context.Context, serial string, pan, channel uint32) error {
	_, err := t.do(ctx, &wire.PairingCmd{Op: &wire.PairingCmd_SetInvPan{
		SetInvPan: &wire.SetInverterPanChannel{Serial: serial, Pan: pan, Channel: channel}}})
	return err
}

// primeInv primes one inverter with the target pan/channel (0x11).
func (t *Transport) primeInv(ctx context.Context, serial string, pan, channel uint32) error {
	_, err := t.do(ctx, &wire.PairingCmd{Op: &wire.PairingCmd_PrimeInv{
		PrimeInv: &wire.PrimeInverterPan{Serial: serial, Pan: pan, Channel: channel}}})
	return err
}

// commitPan broadcasts the PAN-change commit (0x22; ecu-zb sends it ×3).
func (t *Transport) commitPan(ctx context.Context, pan, channel uint32) error {
	_, err := t.do(ctx, &wire.PairingCmd{Op: &wire.PairingCmd_CommitPan{
		CommitPan: &wire.CommitPanNow{Pan: pan, Channel: channel}}})
	return err
}

// bindQuiet turns report-id off + binds one inverter by short address
// (0x08).
func (t *Transport) bindQuiet(ctx context.Context, shortAddr uint16) error {
	_, err := t.do(ctx, &wire.PairingCmd{Op: &wire.PairingCmd_BindQuiet{
		BindQuiet: &wire.BindQuiet{ShortAddr: uint32(shortAddr)}}})
	return err
}

// getModulePan reads the bus backend's (ecu-zb's) operating PAN — the value
// it configured at bring-up or adopted on the last real set. inv-driver uses
// it as the prime/migrate target and the rekey old-PAN, so the radio owner is
// the single source of truth and the driver never re-derives the PAN from
// settings.
func (t *Transport) getModulePan(ctx context.Context) (uint32, error) {
	pan, _, err := t.getModuleConfig(ctx)
	return pan, err
}

// getModuleConfig reads the backend's operating PAN and channel. The channel
// is 0 if the backend doesn't report one (older backend); callers that need
// it should fall back. The PAN is validated as above.
func (t *Transport) getModuleConfig(ctx context.Context) (pan uint32, channel uint32, err error) {
	res, err := t.do(ctx, &wire.PairingCmd{Op: &wire.PairingCmd_GetModulePan{
		GetModulePan: &wire.Empty{}}})
	if err != nil {
		return 0, 0, err
	}
	pan = res.GetPan()
	if pan == 0 || pan > 0xFFFF {
		return 0, 0, fmt.Errorf("get_module_pan: backend reported no/invalid operating PAN (0x%X)", pan)
	}
	return pan, res.GetChannel(), nil
}

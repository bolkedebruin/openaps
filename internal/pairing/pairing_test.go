package pairing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/buslock"
	"github.com/bolkedebruin/openaps/internal/store"
	"github.com/bolkedebruin/openaps/wire"
)

// mockTransport is a CmdSender that records every PairingCmd and auto-replies
// via the bound Transport.Deliver, so the state machine drives end-to-end
// without a real bus. A responder hook lets a test shape the reply (e.g. fail
// the first get_short_addr to exercise the migrate branch).
type mockTransport struct {
	mu   sync.Mutex
	cmds []*wire.PairingCmd
	tr   *Transport
	// opPAN is the operating PAN the simulated ecu-zb reports for
	// get_module_pan (the radio-owned PAN inv-driver queries instead of
	// deriving). Defaults to 0x0DCE.
	opPAN uint32
	// responder returns the result for a given cmd. If nil, every cmd
	// succeeds with an empty result. get_module_pan is answered globally
	// (before the responder) so tests don't each have to handle it.
	responder func(cmd *wire.PairingCmd) *wire.PairingCmdResult
}

func newMockTransport() (*Transport, *mockTransport) {
	m := &mockTransport{opPAN: 0x0DCE}
	tr := NewTransport(m, "ecu-zb")
	tr.Timeout = 2 * time.Second
	m.tr = tr
	return tr, m
}

func (m *mockTransport) SendToBackend(_ string, env *wire.Envelope) bool {
	cmd := env.GetPairingCmd()
	if cmd == nil {
		return false
	}
	m.mu.Lock()
	m.cmds = append(m.cmds, cmd)
	resp := m.responder
	opPAN := m.opPAN
	m.mu.Unlock()

	var res *wire.PairingCmdResult
	// get_module_pan is answered globally so each test responder need not.
	if _, ok := cmd.GetOp().(*wire.PairingCmd_GetModulePan); ok {
		res = &wire.PairingCmdResult{Ok: true, Pan: opPAN}
	} else if resp != nil {
		res = resp(cmd)
	}
	if res == nil {
		res = &wire.PairingCmdResult{Ok: true}
	}
	res.ReqId = cmd.GetReqId()
	// Deliver asynchronously so do() is parked on its channel first.
	go m.tr.Deliver(res)
	return true
}

func (m *mockTransport) opNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.cmds))
	for _, c := range m.cmds {
		out = append(out, opName(c))
	}
	return out
}

func opName(c *wire.PairingCmd) string {
	switch c.GetOp().(type) {
	case *wire.PairingCmd_SetModulePan:
		return "set_module_pan"
	case *wire.PairingCmd_ReportScan:
		return "report_scan"
	case *wire.PairingCmd_GetShortAddr:
		return "get_short_addr"
	case *wire.PairingCmd_SetInvPan:
		return "set_inv_pan"
	case *wire.PairingCmd_PrimeInv:
		return "prime_inv"
	case *wire.PairingCmd_CommitPan:
		return "commit_pan"
	case *wire.PairingCmd_BindQuiet:
		return "bind_quiet"
	case *wire.PairingCmd_GetModulePan:
		return "get_module_pan"
	}
	return "?"
}

// stubSettings is a pairing.Settings with an in-memory PAN + channel.
type stubSettings struct {
	mu      sync.Mutex
	pan     string
	channel uint32
}

func (s *stubSettings) PANOverride() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pan
}

func (s *stubSettings) SetPANOverride(_ context.Context, panHex string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pan = panHex
	return nil
}

func (s *stubSettings) SetChannel(_ context.Context, channel uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channel = channel
	return nil
}

func (s *stubSettings) Channel() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.channel
}

// recordingEvents captures milestone kinds.
type recordingEvents struct {
	mu    sync.Mutex
	kinds []string
}

func (r *recordingEvents) AppendEvent(_ context.Context, _ int64, _, kind, _, _, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kinds = append(r.kinds, kind)
	return nil
}

func (r *recordingEvents) has(kind string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, k := range r.kinds {
		if k == kind {
			return true
		}
	}
	return false
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(context.Background(), dir+"/state.db")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// waitDone polls the status until the stage is terminal or the deadline hits.
func waitDone(t *testing.T, m *Manager) PairingStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s := m.status.snapshot()
		switch s.Stage {
		case StageDone, StageAborted, StageError:
			// Also wait for the goroutine to release the lock.
			m.runMu.Lock()
			done := m.done
			m.runMu.Unlock()
			if done != nil {
				select {
				case <-done:
				case <-time.After(time.Second):
				}
			}
			return s
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("op did not finish; last stage=%q", m.status.snapshot().Stage)
	return PairingStatus{}
}

func TestAdd_BindOnPAN(t *testing.T) {
	tr, mock := newMockTransport()
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		if c.GetGetShortAddr() != nil {
			return &wire.PairingCmdResult{Ok: true, ShortAddr: 0x1234}
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	st := newTestStore(t)
	ev := &recordingEvents{}
	m := &Manager{
		Store: st, Transport: tr, Lock: buslock.New(), Events: ev,
		Settings: &stubSettings{pan: "0DCE"}, CurrentChannel: func() uint32 { return 16 },
		CommitSettle: 10 * time.Millisecond, VerifyRetrySleep: 5 * time.Millisecond,
	}

	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_AddById{AddById: &wire.AddById{Serial: "999900000003"}}})
	if !resp.GetOk() {
		t.Fatalf("start add: %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageDone {
		t.Fatalf("stage = %q want done; err=%q", final.Stage, final.Error)
	}
	// On-PAN: no migrate primitives, just get_short_addr + bind_quiet.
	ops := mock.opNames()
	for _, op := range ops {
		if op == "prime_inv" || op == "commit_pan" {
			t.Fatalf("unexpected migrate op %q for on-PAN bind: %v", op, ops)
		}
	}
	if !ev.has("bind_ok") {
		t.Fatalf("expected bind_ok milestone; got %v", ev.kinds)
	}
	// short_addr persisted.
	row, err := st.GetInverterPairing(context.Background(), "999900000003")
	if err != nil {
		t.Fatalf("GetInverterPairing: %v", err)
	}
	if row.ShortAddr != 0x1234 {
		t.Fatalf("short_addr = 0x%X want 0x1234", row.ShortAddr)
	}
}

// offPANResponder fails the first get_short_addr, so the add takes the
// migrate path. It answers a report_scan with serial only while the module
// sits parked on answersOn. It succeeds on every later query. It tracks the
// park channel the way the real backend does: from the set_module_pan it was
// told to run.
type offPANResponder struct {
	mu         sync.Mutex
	serial     string
	answersOn  uint32 // 0 → never answers a scan
	parkedOn   uint32
	queries    int
	scanChans  []uint32
	primeChan  uint32
	primePan   uint32
	commitChan uint32
	// primeAfterScans is the number of listens before the prime went out.
	// A test uses it to tell which channel the prime targeted.
	primeAfterScans int
	// scanDelay slows each listen, so a test can interleave an abort with
	// a sweep. Against the mock, the sweep would otherwise finish at once.
	scanDelay time.Duration
}

// scanCount returns the number of listens so far.
func (r *offPANResponder) scanCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.scanChans)
}

// sawScanOn reports whether the module ever listened on ch.
func (r *offPANResponder) sawScanOn(ch uint32) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.scanChans {
		if c == ch {
			return true
		}
	}
	return false
}

func (r *offPANResponder) respond(c *wire.PairingCmd) *wire.PairingCmdResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case c.GetSetModulePan() != nil:
		r.parkedOn = c.GetSetModulePan().GetChannel()
	case c.GetReportScan() != nil:
		if r.scanDelay > 0 {
			time.Sleep(r.scanDelay)
		}
		r.scanChans = append(r.scanChans, r.parkedOn)
		if r.answersOn != 0 && r.parkedOn == r.answersOn {
			return &wire.PairingCmdResult{Ok: true, Found: []*wire.FoundInverter{{Serial: r.serial}}}
		}
	case c.GetPrimeInv() != nil:
		r.primeChan = c.GetPrimeInv().GetChannel()
		r.primePan = c.GetPrimeInv().GetPan()
		r.primeAfterScans = len(r.scanChans)
	case c.GetCommitPan() != nil:
		r.commitChan = c.GetCommitPan().GetChannel()
	case c.GetGetShortAddr() != nil:
		r.queries++
		if r.queries == 1 {
			// Off-PAN: the unit is not on our channel, so it does not answer.
			return &wire.PairingCmdResult{Ok: false, Error: "no reply from inverter"}
		}
		return &wire.PairingCmdResult{Ok: true, ShortAddr: 0x55AA}
	}
	return &wire.PairingCmdResult{Ok: true}
}

// parkChannels returns the channels on which the module parked for a
// rendezvous (pan 0xFFFF), in order.
func (m *mockTransport) parkChannels() []uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []uint32
	for _, c := range m.cmds {
		if p := c.GetSetModulePan(); p != nil && p.GetPan() == 0xFFFF {
			out = append(out, p.GetChannel())
		}
	}
	return out
}

// newTestManager is the common Manager wiring: a real bus lock, an in-memory
// PAN/channel, and short settles. A test then does not wait on radio latency
// that it does not test.
func newTestManager(t *testing.T, tr *Transport, st *store.Store, settings Settings, channel uint32) *Manager {
	t.Helper()
	return &Manager{
		Store: st, Transport: tr, Lock: buslock.New(), Events: &recordingEvents{},
		Settings: settings, CurrentChannel: func() uint32 { return channel },
		CommitSettle: 10 * time.Millisecond, VerifyRetrySleep: 5 * time.Millisecond,
	}
}

func newAddManager(t *testing.T, tr *Transport, st *store.Store, ev *recordingEvents) *Manager {
	t.Helper()
	m := newTestManager(t, tr, st, &stubSettings{pan: "0DCE"}, 16)
	m.Events = ev
	return m
}

func runAdd(t *testing.T, m *Manager, serial string) PairingStatus {
	t.Helper()
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_AddById{AddById: &wire.AddById{Serial: serial}}})
	if !resp.GetOk() {
		t.Fatalf("start add: %s", resp.GetError())
	}
	return waitDone(t, m)
}

// An earlier scan heard the unit on another channel. The add migrates it
// from THAT channel. The rendezvous runs where the unit listens, while the
// prime and the commit carry the ECU's own PAN and channel as the target.
func TestAdd_MigratesFromTheRecordedChannel(t *testing.T) {
	const serial = "999900000001"
	tr, mock := newMockTransport()
	r := &offPANResponder{serial: serial, answersOn: 21}
	mock.responder = r.respond

	st := newTestStore(t)
	if err := st.SetInverterFoundChannel(context.Background(), serial, 21); err != nil {
		t.Fatalf("SetInverterFoundChannel: %v", err)
	}
	m := newAddManager(t, tr, st, &recordingEvents{})

	if final := runAdd(t, m, serial); final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}

	// The add confirms the recorded channel and uses it directly: one
	// listen, on 21, with no sweep from the bottom of the range.
	if len(r.scanChans) != 1 || r.scanChans[0] != 21 {
		t.Errorf("listened on %v, want exactly channel 21 — a recorded channel must not trigger a sweep", r.scanChans)
	}
	if parks := mock.parkChannels(); len(parks) != 1 || parks[0] != 21 {
		t.Errorf("rendezvous parked on %v, want channel 21 (where the unit was heard)", parks)
	}
	if r.primeChan != 16 || r.commitChan != 16 {
		t.Errorf("prime ch=%d commit ch=%d, want the target channel 16 in both", r.primeChan, r.commitChan)
	}
	if r.primePan != 0x0DCE {
		t.Errorf("prime pan=0x%04X, want the operating PAN 0x0DCE", r.primePan)
	}
}

// With no recorded channel, the add sweeps to find the unit. It does not
// prime blindly on the ECU's own channel. That is the one channel on which a
// unit bound to another ECU is least likely to listen.
func TestAdd_LocatesTheChannelWhenNoneRecorded(t *testing.T) {
	const serial = "999900000002"
	tr, mock := newMockTransport()
	r := &offPANResponder{serial: serial, answersOn: 21}
	mock.responder = r.respond

	st := newTestStore(t)
	ev := &recordingEvents{}
	m := newAddManager(t, tr, st, ev)

	if final := runAdd(t, m, serial); final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}

	if len(r.scanChans) == 0 || r.scanChans[0] != defaultChanLo {
		t.Errorf("sweep started at %v, want channel %d", r.scanChans, defaultChanLo)
	}
	if last := r.scanChans[len(r.scanChans)-1]; last != 21 {
		t.Errorf("sweep ended on channel %d, want it to stop at 21 where the unit answered", last)
	}
	for _, ch := range r.scanChans {
		if ch > 21 {
			t.Errorf("swept channel %d past the one the unit answered on: %v", ch, r.scanChans)
		}
	}
	if r.primeChan != 16 || r.commitChan != 16 {
		t.Errorf("prime ch=%d commit ch=%d, want the target channel 16", r.primeChan, r.commitChan)
	}
	// The add records the located channel, so a later add skips the sweep.
	row, err := st.GetInverterPairing(context.Background(), serial)
	if err != nil {
		t.Fatalf("GetInverterPairing: %v", err)
	}
	if row.FoundChannel != 21 {
		t.Errorf("found_channel = %d, want 21 persisted for next time", row.FoundChannel)
	}
	if !ev.has("channel_located") {
		t.Errorf("expected channel_located milestone; got %v", ev.kinds)
	}
}

// The add reports a unit that answers on no channel as unreachable. A prime
// and a commit anyway would broadcast a PAN change on a channel where nothing
// listens. They would then report a migration that never happened.
func TestAdd_FailsWhenTheUnitAnswersNowhere(t *testing.T) {
	const serial = "999900000003"
	tr, mock := newMockTransport()
	r := &offPANResponder{serial: serial} // answersOn 0 → never answers
	mock.responder = r.respond
	m := newAddManager(t, tr, newTestStore(t), &recordingEvents{})

	final := runAdd(t, m, serial)
	if final.Stage != StageError {
		t.Fatalf("stage=%q, want error", final.Stage)
	}
	if !strings.Contains(final.Error, "did not answer on any channel") {
		t.Errorf("error = %q, want it to name the exhausted sweep", final.Error)
	}
	for _, op := range mock.opNames() {
		if op == "prime_inv" || op == "commit_pan" {
			t.Fatalf("sent %q for a unit that answered nowhere: %v", op, mock.opNames())
		}
	}
	if got := len(r.scanChans); got != int(defaultChanHi-defaultChanLo+1) {
		t.Errorf("swept %d channels, want the full %d-%d range", got, defaultChanLo, defaultChanHi)
	}
	assertRadioRestored(t, mock)
}

// assertRadioRestored confirms that the last radio command was the pan=0
// sentinel that returns the radio to the operating PAN. A sweep that ends
// without it leaves the whole fleet dark on PAN 0xFFFF.
func assertRadioRestored(t *testing.T, mock *mockTransport) {
	t.Helper()
	mock.mu.Lock()
	defer mock.mu.Unlock()
	for i := len(mock.cmds) - 1; i >= 0; i-- {
		if p := mock.cmds[i].GetSetModulePan(); p != nil {
			if p.GetPan() != 0 {
				t.Fatalf("radio left parked on PAN 0x%04X ch=%d; want the pan=0 restore", p.GetPan(), p.GetChannel())
			}
			return
		}
	}
	t.Fatal("no set_module_pan issued at all; radio state unknown")
}

// A recorded channel that is our own is no hint at all. The direct query on
// it already failed. A rendezvous on it would use the very channel that this
// whole path exists to move away from.
func TestAdd_SweepsWhenTheRecordedChannelIsOurOwn(t *testing.T) {
	const serial = "999900000004"
	tr, mock := newMockTransport()
	r := &offPANResponder{serial: serial, answersOn: 19}
	mock.responder = r.respond

	st := newTestStore(t)
	if err := st.SetInverterFoundChannel(context.Background(), serial, 16); err != nil {
		t.Fatalf("SetInverterFoundChannel: %v", err)
	}
	m := newAddManager(t, tr, st, &recordingEvents{})

	if final := runAdd(t, m, serial); final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}
	if len(r.scanChans) == 0 || r.scanChans[0] != defaultChanLo {
		t.Fatalf("listened on %v, want a sweep from channel %d", r.scanChans, defaultChanLo)
	}
	// The sweep passes over 16 like any other channel. What matters is that
	// the prime went out only after the sweep reached the channel on which
	// the unit answers.
	if last := r.scanChans[len(r.scanChans)-1]; last != 19 {
		t.Errorf("last listen was channel %d, want 19: %v", last, r.scanChans)
	}
	if r.primeAfterScans != len(r.scanChans) {
		t.Errorf("prime sent after %d listens, want it only after the last (%d)", r.primeAfterScans, len(r.scanChans))
	}
	row, err := st.GetInverterPairing(context.Background(), serial)
	if err != nil {
		t.Fatalf("GetInverterPairing: %v", err)
	}
	if row.FoundChannel != 19 {
		t.Errorf("found_channel = %d, want the located channel 19 to replace the stale 16", row.FoundChannel)
	}
}

// A stale record costs one listen and then falls back to the sweep. The
// record is stale because the unit moved to another channel after the scan
// that wrote it. The add primes and commits nothing at the wrong channel.
func TestAdd_FallsBackToSweepWhenTheRecordIsStale(t *testing.T) {
	const serial = "999900000005"
	tr, mock := newMockTransport()
	r := &offPANResponder{serial: serial, answersOn: 24}
	mock.responder = r.respond

	st := newTestStore(t)
	if err := st.SetInverterFoundChannel(context.Background(), serial, 13); err != nil {
		t.Fatalf("SetInverterFoundChannel: %v", err)
	}
	ev := &recordingEvents{}
	m := newAddManager(t, tr, st, ev)

	if final := runAdd(t, m, serial); final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}
	if len(r.scanChans) == 0 || r.scanChans[0] != 13 {
		t.Fatalf("listened on %v, want the recorded channel 13 tried first", r.scanChans)
	}
	// The add sent nothing at channel 13. The prime came after the sweep
	// reached the channel on which the unit is.
	if r.primeAfterScans != len(r.scanChans) {
		t.Errorf("prime sent after %d listens, want it only after the last (%d)", r.primeAfterScans, len(r.scanChans))
	}
	if !r.sawScanOn(24) {
		t.Errorf("never listened on 24 where the unit answers: %v", r.scanChans)
	}
	row, err := st.GetInverterPairing(context.Background(), serial)
	if err != nil {
		t.Fatalf("GetInverterPairing: %v", err)
	}
	if row.FoundChannel != 24 {
		t.Errorf("found_channel = %d, want the stale 13 replaced by 24", row.FoundChannel)
	}
}

func TestRekey_CommitAndVerify(t *testing.T) {
	tr, mock := newMockTransport()
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		if c.GetGetShortAddr() != nil {
			return &wire.PairingCmdResult{Ok: true, ShortAddr: 0x0101}
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	st := newTestStore(t)
	// Seed two inverters in the inventory.
	ctx := context.Background()
	for _, uid := range []string{"999900000003", "999900000001"} {
		if err := st.SetInverterShortAddr(ctx, uid, 0x0001); err != nil {
			t.Fatal(err)
		}
	}
	settings := &stubSettings{pan: "0DCE"}
	ev := &recordingEvents{}
	m := &Manager{
		Store: st, Transport: tr, Lock: buslock.New(), Events: ev,
		Settings: settings, CurrentChannel: func() uint32 { return 16 },
		CommitSettle: 10 * time.Millisecond, VerifyRetrySleep: 5 * time.Millisecond,
	}
	resp := m.Handle(ctx, "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_FleetRekey{FleetRekey: &wire.FleetRekey{NewPan: "1A2B", Channel: 20}}})
	if !resp.GetOk() {
		t.Fatalf("start rekey: %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}
	if got := settings.PANOverride(); got != "1A2B" {
		t.Fatalf("pan_override = %q want 1A2B", got)
	}
	if !ev.has("rekey_committed") {
		t.Fatalf("expected rekey_committed milestone; got %v", ev.kinds)
	}
	// 3 commit rounds expected.
	commits := 0
	for _, op := range mock.opNames() {
		if op == "commit_pan" {
			commits++
		}
	}
	if commits != commitRounds {
		t.Fatalf("commit rounds = %d want %d", commits, commitRounds)
	}
}

func TestRekey_RollbackOnPrimeFailure(t *testing.T) {
	tr, mock := newMockTransport()
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		if c.GetPrimeInv() != nil {
			return &wire.PairingCmdResult{Ok: false, Error: "prime rejected"}
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.SetInverterShortAddr(ctx, "999900000003", 0x0001); err != nil {
		t.Fatal(err)
	}
	settings := &stubSettings{pan: "0DCE"}
	m := &Manager{
		Store: st, Transport: tr, Lock: buslock.New(), Events: &recordingEvents{},
		Settings: settings, CurrentChannel: func() uint32 { return 16 },
		CommitSettle: 10 * time.Millisecond, VerifyRetrySleep: 5 * time.Millisecond,
	}
	resp := m.Handle(ctx, "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_FleetRekey{FleetRekey: &wire.FleetRekey{NewPan: "1A2B"}}})
	if !resp.GetOk() {
		t.Fatalf("start: %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageError {
		t.Fatalf("stage=%q want error", final.Stage)
	}
	// PAN must NOT have moved (rollback keeps old PAN).
	if got := settings.PANOverride(); got != "0DCE" {
		t.Fatalf("pan_override = %q want unchanged 0DCE (rollback)", got)
	}
	// A rollback set_module_pan back to the old PAN must be present.
	sawRollback := false
	mock.mu.Lock()
	for _, c := range mock.cmds {
		if sm := c.GetSetModulePan(); sm != nil && sm.GetPan() == 0x0DCE {
			sawRollback = true
		}
	}
	mock.mu.Unlock()
	if !sawRollback {
		t.Fatalf("expected a rollback set_module_pan to 0x0DCE")
	}
}

func TestLock_MutualExclusion(t *testing.T) {
	lock := buslock.New()
	tr, mock := newMockTransport()
	// Block the first op inside report_scan so the lock stays held.
	release := make(chan struct{})
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		if c.GetReportScan() != nil {
			<-release
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	m := &Manager{
		Store: newTestStore(t), Transport: tr, Lock: lock, Events: &recordingEvents{},
		Settings: &stubSettings{pan: "0DCE"}, CurrentChannel: func() uint32 { return 16 },
		CommitSettle: 10 * time.Millisecond, VerifyRetrySleep: 5 * time.Millisecond,
	}
	// Simulate the grid-profile broadcast holding the lock.
	if ok, _ := lock.TryAcquire("gridprofile-broadcast"); !ok {
		t.Fatal("could not pre-acquire lock")
	}
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_Scan{Scan: &wire.ScanStart{}}})
	if resp.GetOk() {
		t.Fatalf("expected scan to be refused while lock held")
	}
	lock.Release()
	close(release)
}

// TestConcurrentStatusAndAbort exercises the milestone attribution path
// against concurrent get_status/abort callers. Run under -race to prove the
// "by" attribution is no longer a data race (it is captured at begin and read
// under the status lock, not via a shared mutable field).
func TestConcurrentStatusAndAbort(t *testing.T) {
	tr, mock := newMockTransport()
	// Slow the per-cmd reply so the op stays in-flight while we poke it.
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		time.Sleep(2 * time.Millisecond)
		if c.GetGetShortAddr() != nil {
			return &wire.PairingCmdResult{Ok: true, ShortAddr: 0x1234}
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	m := &Manager{
		Store: newTestStore(t), Transport: tr, Lock: buslock.New(),
		Events:   &recordingEvents{},
		Settings: &stubSettings{pan: "0DCE"}, CurrentChannel: func() uint32 { return 16 },
		CommitSettle: time.Millisecond, VerifyRetrySleep: time.Millisecond,
	}
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_AddById{AddById: &wire.AddById{Serial: "999900000003"}}})
	if !resp.GetOk() {
		t.Fatalf("start add: %s", resp.GetError())
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = m.getStatus()
			}
		}()
	}
	wg.Add(1)
	go func() { defer wg.Done(); _ = m.abort() }()
	wg.Wait()
	waitDone(t, m)
}

func TestStatus_JSONShape(t *testing.T) {
	m := &Manager{}
	m.status.begin(OpScan, StageScan, 3, "ecu-web")
	m.status.upsertInverter(PerInverter{Serial: "999900000003", State: "found", Encrypted: true})
	resp := m.getStatus()
	if !resp.GetOk() {
		t.Fatalf("get_status: %s", resp.GetError())
	}
	var s PairingStatus
	if err := json.Unmarshal(resp.GetStatusJson(), &s); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if s.Op != OpScan || s.Stage != StageScan || s.Total != 3 {
		t.Fatalf("status fields wrong: %+v", s)
	}
	if len(s.PerInverter) != 1 || !s.PerInverter[0].Encrypted {
		t.Fatalf("per_inverter wrong: %+v", s.PerInverter)
	}
}

func TestParsePAN(t *testing.T) {
	cases := []struct {
		in      string
		want    uint32
		wantErr bool
	}{
		{"0DCE", 0x0DCE, false},
		{"0x1a2b", 0x1A2B, false},
		{"FFFF", 0xFFFF, false},
		{"1", 0x0001, false},
		{"", 0, true},
		{"12345", 0, true},
		{"GHIJ", 0, true},
	}
	for _, c := range cases {
		got, err := parsePAN(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parsePAN(%q) = 0x%X, want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePAN(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parsePAN(%q) = 0x%X want 0x%X", c.in, got, c.want)
		}
	}
}

func TestReplace_RejectsBadInput(t *testing.T) {
	m := &Manager{Transport: nil}
	cases := []*wire.ReplaceInverter{
		{OldUid: "bad", NewSerial: "999900000003"},
		{OldUid: "999900000003", NewSerial: ""},
		{OldUid: "999900000003", NewSerial: "12ab"},
	}
	for i, c := range cases {
		resp := m.startReplace("ecu-web", c)
		if resp.GetOk() {
			t.Errorf("case %d: expected rejection for %+v", i, c)
		}
	}
}

func TestTransport_Timeout(t *testing.T) {
	// A sender that never delivers a result → do() must time out.
	silent := senderFunc(func(string, *wire.Envelope) bool { return true })
	tr := NewTransport(silent, "ecu-zb")
	tr.Timeout = 50 * time.Millisecond
	_, err := tr.getShortAddr(context.Background(), "999900000003")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

type senderFunc func(string, *wire.Envelope) bool

func (f senderFunc) SendToBackend(b string, e *wire.Envelope) bool { return f(b, e) }

func TestTransport_DeliverNoWaiter(t *testing.T) {
	tr := NewTransport(senderFunc(func(string, *wire.Envelope) bool { return true }), "ecu-zb")
	if tr.Deliver(&wire.PairingCmdResult{ReqId: 999}) {
		t.Fatal("Deliver with no waiter should return false")
	}
	if tr.Deliver(nil) {
		t.Fatal("Deliver(nil) should return false")
	}
}

var _ = fmt.Sprintf

// A scan is what teaches the ECU where a unit lives. The scan must persist
// the channel on which the unit answered, so a later add reaches the unit
// without another sweep.
func TestScan_RecordsTheChannelEachUnitAnsweredOn(t *testing.T) {
	const serial = "999900000006"
	tr, mock := newMockTransport()
	r := &offPANResponder{serial: serial, answersOn: 23}
	mock.responder = r.respond

	st := newTestStore(t)
	m := newAddManager(t, tr, st, &recordingEvents{})

	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_Scan{Scan: &wire.ScanStart{Slow: true}}})
	if !resp.GetOk() {
		t.Fatalf("start scan: %s", resp.GetError())
	}
	if final := waitDone(t, m); final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}

	row, err := st.GetInverterPairing(context.Background(), serial)
	if err != nil {
		t.Fatalf("GetInverterPairing: %v", err)
	}
	if row.FoundChannel != 23 {
		t.Fatalf("found_channel = %d, want 23 recorded by the scan", row.FoundChannel)
	}
	// A junk serial in the scan results must not create a row.
	if _, err := st.GetInverterPairing(context.Background(), "not-a-serial"); err == nil {
		t.Error("a malformed serial from the radio created an inverter row")
	}
	assertRadioRestored(t, mock)
}

// A sweep is the longest window an operator can interrupt: up to 16 channels
// of dwell. An abort must therefore end the op as aborted, send nothing at
// the fleet, and hand the radio back to the operating PAN.
func TestAdd_AbortDuringSweepRestoresTheRadio(t *testing.T) {
	const serial = "999900000007"
	tr, mock := newMockTransport()
	// The unit answers nowhere, so the sweep runs the full range. The
	// per-listen delay gives the abort a window, the way a real dwell would.
	r := &offPANResponder{serial: serial, scanDelay: 20 * time.Millisecond}
	mock.responder = r.respond
	ev := &recordingEvents{}
	m := newAddManager(t, tr, newTestStore(t), ev)

	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_AddById{AddById: &wire.AddById{Serial: serial}}})
	if !resp.GetOk() {
		t.Fatalf("start add: %s", resp.GetError())
	}
	// Abort once the sweep is under way.
	deadline := time.Now().Add(2 * time.Second)
	for r.scanCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_Abort{Abort: &wire.Empty{}}})

	final := waitDone(t, m)
	if final.Stage != StageAborted {
		t.Fatalf("stage=%q err=%q, want aborted", final.Stage, final.Error)
	}
	if !ev.has("pairing_aborted") || ev.has("pairing_error") {
		t.Errorf("milestones = %v, want pairing_aborted and no pairing_error", ev.kinds)
	}
	for _, op := range mock.opNames() {
		if op == "prime_inv" || op == "commit_pan" {
			t.Fatalf("transmitted %q during an aborted locate: %v", op, mock.opNames())
		}
	}
	assertRadioRestored(t, mock)
}

// Replace shares bindAndMigrate, so the channel resolution has to work there
// too. It then hands over to the inheritance tail.
func TestReplace_MigratesTheNewUnitFromItsOwnChannel(t *testing.T) {
	const oldUID, newSerial = "999900000008", "999900000009"
	tr, mock := newMockTransport()
	r := &offPANResponder{serial: newSerial, answersOn: 18}
	mock.responder = r.respond

	st := newTestStore(t)
	ctx := context.Background()
	if err := st.SetInverterShortAddr(ctx, oldUID, 0x0101); err != nil {
		t.Fatalf("seed old unit: %v", err)
	}
	if err := st.SetInverterFoundChannel(ctx, newSerial, 18); err != nil {
		t.Fatalf("SetInverterFoundChannel: %v", err)
	}
	m := newAddManager(t, tr, st, &recordingEvents{})

	resp := m.Handle(ctx, "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_Replace{Replace: &wire.ReplaceInverter{
			OldUid: oldUID, NewSerial: newSerial}}})
	if !resp.GetOk() {
		t.Fatalf("start replace: %s", resp.GetError())
	}
	if final := waitDone(t, m); final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}

	if len(r.scanChans) != 1 || r.scanChans[0] != 18 {
		t.Errorf("listened on %v, want exactly the recorded channel 18", r.scanChans)
	}
	if r.primeChan != 16 || r.commitChan != 16 {
		t.Errorf("prime ch=%d commit ch=%d, want the target channel 16", r.primeChan, r.commitChan)
	}
	// The same op retires the dead unit.
	if _, err := st.GetInverterPairing(ctx, oldUID); err == nil {
		t.Error("old unit still present after replace")
	}
	assertRadioRestored(t, mock)
}

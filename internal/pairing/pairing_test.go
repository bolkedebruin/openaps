package pairing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bolke/inv-driver/internal/buslock"
	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
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
		Op: &wire.PairingRequest_AddById{AddById: &wire.AddById{Serial: "806000042582"}}})
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
	row, err := st.GetInverterPairing(context.Background(), "806000042582")
	if err != nil {
		t.Fatalf("GetInverterPairing: %v", err)
	}
	if row.ShortAddr != 0x1234 {
		t.Fatalf("short_addr = 0x%X want 0x1234", row.ShortAddr)
	}
}

func TestAdd_MigrateWhenOffPAN(t *testing.T) {
	tr, mock := newMockTransport()
	var firstQuery sync.Once
	mock.responder = func(c *wire.PairingCmd) *wire.PairingCmdResult {
		if c.GetGetShortAddr() != nil {
			fail := false
			firstQuery.Do(func() { fail = true })
			if fail {
				// Off-PAN: first query fails → triggers migrate.
				return &wire.PairingCmdResult{Ok: false, Error: "no reply"}
			}
			return &wire.PairingCmdResult{Ok: true, ShortAddr: 0x55AA}
		}
		return &wire.PairingCmdResult{Ok: true}
	}
	m := &Manager{
		Store: newTestStore(t), Transport: tr, Lock: buslock.New(),
		Events:   &recordingEvents{},
		Settings: &stubSettings{pan: "0DCE"}, CurrentChannel: func() uint32 { return 16 },
		CommitSettle: 10 * time.Millisecond, VerifyRetrySleep: 5 * time.Millisecond,
	}
	// Shorten the post-commit settle for the test.
	resp := m.Handle(context.Background(), "ecu-web", &wire.PairingRequest{
		Op: &wire.PairingRequest_AddById{AddById: &wire.AddById{Serial: "704000006835"}}})
	if !resp.GetOk() {
		t.Fatalf("start: %s", resp.GetError())
	}
	final := waitDone(t, m)
	if final.Stage != StageDone {
		t.Fatalf("stage=%q err=%q", final.Stage, final.Error)
	}
	ops := mock.opNames()
	want := map[string]bool{"set_module_pan": false, "prime_inv": false, "commit_pan": false}
	for _, op := range ops {
		if _, ok := want[op]; ok {
			want[op] = true
		}
	}
	for op, seen := range want {
		if !seen {
			t.Fatalf("expected migrate op %q in %v", op, ops)
		}
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
	for _, uid := range []string{"806000042582", "704000006835"} {
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
	if err := st.SetInverterShortAddr(ctx, "806000042582", 0x0001); err != nil {
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
		Op: &wire.PairingRequest_AddById{AddById: &wire.AddById{Serial: "806000042582"}}})
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
	m.status.upsertInverter(PerInverter{Serial: "806000042582", State: "found", Encrypted: true})
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
		{OldUid: "bad", NewSerial: "806000042582"},
		{OldUid: "806000042582", NewSerial: ""},
		{OldUid: "806000042582", NewSerial: "12ab"},
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
	_, err := tr.getShortAddr(context.Background(), "806000042582")
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

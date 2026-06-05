package probe

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/internal/events"
	"github.com/bolkedebruin/openaps/internal/ingest"
	"github.com/bolkedebruin/openaps/internal/ipc"
	"github.com/bolkedebruin/openaps/internal/store"
	"github.com/bolkedebruin/openaps/wire"
)

type fakeSender struct {
	mu     sync.Mutex
	calls  []*wire.Envelope
	refuse bool
}

func (f *fakeSender) SendToBackend(backend string, env *wire.Envelope) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refuse {
		return false
	}
	f.calls = append(f.calls, env)
	return true
}

func (f *fakeSender) snap() []*wire.Envelope {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*wire.Envelope, len(f.calls))
	copy(out, f.calls)
	return out
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestProbe_IteratesNullModelCode_Only(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openStore(t)
	fs := &fakeSender{}

	// uidA: model_code NULL — should be probed.
	if err := s.UpsertInverterFromTelemetry(ctx, "uidA", 0x1111, "", "", 1); err != nil {
		t.Fatalf("seed A: %v", err)
	}
	// uidB: model_code + software_version populated — should be skipped.
	mc := uint32(0x24)
	sw := uint32(3067)
	if _, err := s.UpsertInverterInfo(ctx, store.InverterInfoUpdate{
		UID: "uidB", TsMs: 1, ShortAddr: 0x2222, ModelCode: &mc, SoftwareVer: &sw,
	}); err != nil {
		t.Fatalf("seed B: %v", err)
	}
	// uidC: model_code NULL — should also be probed.
	if err := s.UpsertInverterFromTelemetry(ctx, "uidC", 0x3333, "", "", 1); err != nil {
		t.Fatalf("seed C: %v", err)
	}

	p := &Probe{Store: s, Server: fs, Backend: "test", SendInterval: 1 * time.Millisecond}
	sent, err := p.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sent != 2 {
		t.Fatalf("sent: got %d want 2", sent)
	}
	got := fs.snap()
	if len(got) != 2 {
		t.Fatalf("calls: got %d want 2", len(got))
	}
	uids := []string{got[0].GetSend().GetPeerUid(), got[1].GetSend().GetPeerUid()}
	if uids[0] != "uidA" || uids[1] != "uidC" {
		t.Fatalf("uids: got %v want [uidA uidC]", uids)
	}
	// Frame is the 13-byte info-query L2 body (outer envelope is
	// wrapped by the backend, not the probe).
	if !codec.MatchOutboundInfoQuery(got[0].GetSend().GetFrame()) {
		t.Fatalf("frame is not the info-query L2: % X", got[0].GetSend().GetFrame())
	}
}

func TestProbe_NoTargets_NoCall(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openStore(t)
	fs := &fakeSender{}
	p := &Probe{Store: s, Server: fs, Backend: "test"}
	sent, err := p.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sent != 0 || len(fs.snap()) != 0 {
		t.Fatalf("expected no sends, got sent=%d calls=%d", sent, len(fs.snap()))
	}
}

func TestProbe_SenderRefusal_AbortsPass(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openStore(t)
	if err := s.UpsertInverterFromTelemetry(ctx, "u1", 0x1111, "", "", 1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.UpsertInverterFromTelemetry(ctx, "u2", 0x2222, "", "", 1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	fs := &fakeSender{refuse: true}
	p := &Probe{Store: s, Server: fs, Backend: "test"}
	sent, err := p.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sent != 0 {
		t.Fatalf("sent: got %d want 0", sent)
	}
}

func TestProbe_SkipsRowsWithNullShortAddr(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openStore(t)
	// Raw insert with short_addr NULL.
	if _, err := s.DB().ExecContext(ctx, `
INSERT INTO inverters (uid, short_addr, family, model, paired_at_ms, last_seen_ms)
VALUES ('nullsa', NULL, NULL, NULL, 1, 1)`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.UpsertInverterFromTelemetry(ctx, "u1", 0x1111, "", "", 1); err != nil {
		t.Fatalf("seed u1: %v", err)
	}
	fs := &fakeSender{}
	p := &Probe{Store: s, Server: fs, Backend: "test"}
	sent, err := p.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sent != 1 {
		t.Fatalf("sent: got %d want 1", sent)
	}
	if got := fs.snap()[0].GetSend().GetPeerUid(); got != "u1" {
		t.Fatalf("uid: %q", got)
	}
}

// TestProbe_ColdStart_EndToEnd wires the probe to a real ipc.Server
// with a fake publisher attached, seeds the store with an inverter
// lacking model_code, and asserts the publisher receives the
// downstream Send envelope after Hello.
func TestProbe_ColdStart_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Server + store + fake publisher.
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "p.sock")
	if len(sockPath) >= 100 {
		sockPath = filepath.Join(os.TempDir(), "ipc-probe-"+strconv.FormatInt(time.Now().UnixNano(), 36)+".sock")
	}
	st, err := store.Open(ctx, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	// Seed two inverters: one with NULL model_code, one with a known
	// model_code (must NOT be probed).
	if err := st.UpsertInverterFromTelemetry(ctx, "uidProbe1", 0x5011, "", "", 1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	mc := uint32(0x24)
	sw := uint32(3067)
	if _, err := st.UpsertInverterInfo(ctx, store.InverterInfoUpdate{
		UID: "uidKnown", TsMs: 1, ShortAddr: 0x2222, ModelCode: &mc, SoftwareVer: &sw,
	}); err != nil {
		t.Fatalf("seed known: %v", err)
	}

	pub := events.New()
	srv := &ipc.Server{
		SocketPath: sockPath,
		Ingestor:   &ingest.Ingestor{S: st, Pub: pub},
		Publisher:  pub,
	}

	probeRan := make(chan struct{}, 1)
	srv.OnPublisherAttach = func(backend string) {
		if backend != "apsystems-stock-zb" {
			return
		}
		p := &Probe{Store: st, Server: srv, Backend: backend, SendInterval: 1 * time.Millisecond}
		if _, err := p.Run(ctx); err != nil {
			t.Errorf("probe.Run: %v", err)
		}
		probeRan <- struct{}{}
	}

	doneCh := make(chan error, 1)
	go func() { doneCh <- srv.Serve(ctx) }()
	t.Cleanup(func() { cancel(); <-doneCh })

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	// Fake publisher connects and says Hello with the matching
	// backend name, then reads one downstream frame.
	c, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend: "apsystems-stock-zb", Version: "0.0.0", StartedAtMs: 1,
	}}}
	if err := wire.WriteFrame(c, hello); err != nil {
		t.Fatalf("hello: %v", err)
	}

	// Wait for the probe pass to complete on the server side.
	select {
	case <-probeRan:
	case <-time.After(3 * time.Second):
		t.Fatal("probe did not run within 3s of publisher attach")
	}

	// The fake publisher must receive a downstream Send envelope
	// with peer_uid=uidProbe1.
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got wire.Envelope
	if err := wire.ReadFrame(c, &got); err != nil {
		t.Fatalf("read downstream: %v", err)
	}
	send := got.GetSend()
	if send == nil {
		t.Fatalf("expected Send envelope, got %T", got.GetBody())
	}
	if send.GetPeerUid() != "uidProbe1" {
		t.Fatalf("peer_uid: %q want uidProbe1", send.GetPeerUid())
	}
	if !codec.MatchOutboundInfoQuery(send.GetFrame()) {
		t.Fatalf("frame is not info-query L2: % X", send.GetFrame())
	}

	// uidKnown must NOT be probed; setting a short read deadline and
	// asserting EOF / timeout.
	_ = c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var extra wire.Envelope
	if err := wire.ReadFrame(c, &extra); err == nil {
		t.Fatalf("unexpected second downstream frame: %T", extra.GetBody())
	}
}

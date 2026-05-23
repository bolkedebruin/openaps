package telemetrypoll

import (
	"bytes"
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
)

// fakeRouter captures every SendToBackend call.
type fakeRouter struct {
	mu    sync.Mutex
	calls []*wire.Envelope
}

func (f *fakeRouter) SendToBackend(_ string, env *wire.Envelope) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, env)
	return true
}

func (f *fakeRouter) snap() []*wire.Envelope {
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

// TestPoller_OneBBSendPerInverterPerRound verifies that poll() emits
// exactly one BB query per inverter, sets both PeerUid and ShortAddr,
// and uses the canonical OutboundBBQueryL2 frame body.
func TestPoller_OneBBSendPerInverterPerRound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openStore(t)

	// Seed two inverters with known short_addr.
	if err := s.UpsertInverterFromTelemetry(ctx, "aaa111", 0x1234, "", "", 1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.UpsertInverterFromTelemetry(ctx, "bbb222", 0x5678, "", "", 1); err != nil {
		t.Fatalf("seed: %v", err)
	}

	fr := &fakeRouter{}
	p := &Poller{
		Store:        NewStoreAdapter(s),
		Server:       fr,
		Backend:      "test-backend",
		SendInterval: 1 * time.Millisecond,
	}

	if err := p.poll(ctx); err != nil {
		t.Fatalf("poll: %v", err)
	}

	got := fr.snap()
	if len(got) != 2 {
		t.Fatalf("expected 2 sends, got %d", len(got))
	}

	want := codec.OutboundBBQueryL2()

	// Collect sends indexed by uid so the test is order-independent.
	byUID := make(map[string]*wire.Send, 2)
	for _, env := range got {
		s := env.GetSend()
		if s == nil {
			t.Fatalf("envelope is not a Send: %T", env.GetBody())
		}
		byUID[s.GetPeerUid()] = s
	}

	for uid, expectSA := range map[string]uint32{
		"aaa111": 0x1234,
		"bbb222": 0x5678,
	} {
		snd, ok := byUID[uid]
		if !ok {
			t.Errorf("missing send for uid=%s", uid)
			continue
		}
		if snd.GetShortAddr() != expectSA {
			t.Errorf("uid=%s: ShortAddr=%#x want %#x", uid, snd.GetShortAddr(), expectSA)
		}
		if snd.GetPeerUid() != uid {
			t.Errorf("PeerUid=%q want %q", snd.GetPeerUid(), uid)
		}
		if !bytes.Equal(snd.GetFrame(), want) {
			t.Errorf("uid=%s: frame mismatch\ngot  % X\nwant % X", uid, snd.GetFrame(), want)
		}
	}
}

// TestPoller_EmptyInventory sends nothing and does not error.
func TestPoller_EmptyInventory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openStore(t)
	fr := &fakeRouter{}
	p := &Poller{
		Store:   NewStoreAdapter(s),
		Server:  fr,
		Backend: "test-backend",
	}
	if err := p.poll(ctx); err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(fr.snap()) != 0 {
		t.Fatalf("expected 0 sends for empty inventory, got %d", len(fr.snap()))
	}
}

// TestPoller_NullShortAddrSkipped verifies that inverters with no
// short_addr are not dispatched.
func TestPoller_NullShortAddrSkipped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openStore(t)

	// Insert one inverter with NULL short_addr via raw SQL.
	if _, err := s.DB().ExecContext(ctx, `
INSERT INTO inverters (uid, short_addr, family, model, paired_at_ms, last_seen_ms)
VALUES ('noaddr', NULL, NULL, NULL, 1, 1)`); err != nil {
		t.Fatalf("seed noaddr: %v", err)
	}
	// Insert one with a valid short_addr.
	if err := s.UpsertInverterFromTelemetry(ctx, "hasaddr", 0xABCD, "", "", 1); err != nil {
		t.Fatalf("seed hasaddr: %v", err)
	}

	fr := &fakeRouter{}
	p := &Poller{
		Store:        NewStoreAdapter(s),
		Server:       fr,
		Backend:      "test-backend",
		SendInterval: 1 * time.Millisecond,
	}

	if err := p.poll(ctx); err != nil {
		t.Fatalf("poll: %v", err)
	}
	got := fr.snap()
	if len(got) != 1 {
		t.Fatalf("expected 1 send, got %d", len(got))
	}
	if uid := got[0].GetSend().GetPeerUid(); uid != "hasaddr" {
		t.Fatalf("unexpected uid=%q", uid)
	}
}

// TestPoller_ZeroShortAddrSkipped verifies that inverters with short_addr=0
// (broadcast/coordinator sentinel) are excluded from the poll round.
func TestPoller_ZeroShortAddrSkipped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openStore(t)

	// Insert one inverter with short_addr = 0 via raw SQL.
	if _, err := s.DB().ExecContext(ctx, `
INSERT INTO inverters (uid, short_addr, family, model, paired_at_ms, last_seen_ms)
VALUES ('zeroaddr', 0, NULL, NULL, 1, 1)`); err != nil {
		t.Fatalf("seed zeroaddr: %v", err)
	}
	// Insert one with a valid non-zero short_addr.
	if err := s.UpsertInverterFromTelemetry(ctx, "realaddr", 0x1234, "", "", 1); err != nil {
		t.Fatalf("seed realaddr: %v", err)
	}

	fr := &fakeRouter{}
	p := &Poller{
		Store:        NewStoreAdapter(s),
		Server:       fr,
		Backend:      "test-backend",
		SendInterval: 1 * time.Millisecond,
	}

	if err := p.poll(ctx); err != nil {
		t.Fatalf("poll: %v", err)
	}
	got := fr.snap()
	if len(got) != 1 {
		t.Fatalf("expected 1 send (only realaddr), got %d", len(got))
	}
	if uid := got[0].GetSend().GetPeerUid(); uid != "realaddr" {
		t.Fatalf("unexpected uid=%q (zeroaddr should be skipped)", uid)
	}
}

// TestPoller_BackendAbsent_SkipsRound verifies that when SendToBackend
// returns false the poller logs and returns without error.
type refusingRouter struct{}

func (r *refusingRouter) SendToBackend(_ string, _ *wire.Envelope) bool { return false }

func TestPoller_BackendAbsent_SkipsRound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openStore(t)
	if err := s.UpsertInverterFromTelemetry(ctx, "u1", 0x0001, "", "", 1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	p := &Poller{
		Store:   NewStoreAdapter(s),
		Server:  &refusingRouter{},
		Backend: "test-backend",
	}
	if err := p.poll(ctx); err != nil {
		t.Fatalf("poll should return nil on backend refusal, got: %v", err)
	}
}

// TestPoller_Run_TicksAndCancels exercises the Run goroutine: seeds one
// inverter, starts the poller with a fast ticker, waits for at least one
// send, then cancels and verifies clean shutdown.
func TestPoller_Run_TicksAndCancels(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	s := openStore(t)

	if err := s.UpsertInverterFromTelemetry(context.Background(), "tick1", 0x0011, "", "", 1); err != nil {
		t.Fatalf("seed: %v", err)
	}

	fr := &fakeRouter{}
	p := &Poller{
		Store:        NewStoreAdapter(s),
		Server:       fr,
		Backend:      "test-backend",
		Interval:     10 * time.Millisecond,
		SendInterval: 1 * time.Millisecond,
	}

	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(fr.snap()) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
	if len(fr.snap()) == 0 {
		t.Fatal("expected at least one send before cancel")
	}
}

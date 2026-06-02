package ingest

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/store"
)

// kindsByUID returns the per-uid list of online/offline/sunrise/sundown
// events recorded since tSince (newest-first not required — preserves the
// insert order from QueryEvents ASC).
func kindsByUID(t *testing.T, s *store.Store, since int64) []string {
	t.Helper()
	evs, err := s.QueryEvents(context.Background(), store.EventFilter{
		SinceMs: since,
		Limit:   100,
	})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	out := make([]string, 0, len(evs))
	for _, e := range evs {
		out = append(out, e.Kind+":"+e.InverterUID+":"+e.By)
	}
	return out
}

// TestOnlineSweep_DebounceAndSunrise covers the operator-facing contract:
//   - the first online sighting emits immediately (sunrise) so dawn lands fast,
//   - a brief flap (offline < debounce) is suppressed,
//   - a real offline (>= debounce) emits exactly once,
//   - fleet sunrise/sundown track the any-inverter-online debounced state.
func TestOnlineSweep_DebounceAndSunrise(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "ev.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	in := &Ingestor{S: s}

	const min = int64(time.Minute / time.Millisecond)
	now := int64(1_000_000)

	// t=0: inverter A first telemetry → online + fleet sunrise.
	in.markSeen("A", now)
	in.onlineSweep(ctx, now)
	got := kindsByUID(t, s, 0)
	if len(got) != 2 || got[0] != "online:A:inv-driver" || got[1] != "sunrise::inv-driver" {
		t.Fatalf("after first sighting got %v, want [online:A, sunrise]", got)
	}

	// t=2m: A flaps (no fresh markSeen). Raw is offline (60s past lastSeen),
	// but debounce hasn't elapsed → no event yet.
	in.onlineSweep(ctx, now+2*min)
	if got := kindsByUID(t, s, 0); len(got) != 2 {
		t.Fatalf("flap should be suppressed; got %v", got)
	}

	// t=2m30s: A reappears within the debounce window → still online, no events.
	in.markSeen("A", now+2*min+30_000)
	in.onlineSweep(ctx, now+2*min+30_000)
	if got := kindsByUID(t, s, 0); len(got) != 2 {
		t.Fatalf("flap-then-return should not emit; got %v", got)
	}

	// t=2m30s + 6m: A goes truly offline (>5min after lastSeen) → offline +
	// fleet sundown (A was the only online inverter).
	t2 := now + 2*min + 30_000 + 6*min
	in.onlineSweep(ctx, t2)
	got = kindsByUID(t, s, 0)
	if len(got) != 4 || got[2] != "offline:A:inv-driver" || got[3] != "sundown::inv-driver" {
		t.Fatalf("after sustained offline got %v, want [...,offline:A, sundown]", got)
	}

	// t2+1m: nothing changes — should not re-emit.
	in.onlineSweep(ctx, t2+min)
	if got := kindsByUID(t, s, 0); len(got) != 4 {
		t.Errorf("steady-state offline should not re-emit; got %v", got)
	}

	// A comes back later → online + sunrise.
	t3 := t2 + 10*min
	in.markSeen("A", t3)
	in.onlineSweep(ctx, t3)
	got = kindsByUID(t, s, 0)
	if len(got) != 6 || got[4] != "online:A:inv-driver" || got[5] != "sunrise::inv-driver" {
		t.Fatalf("after rejoin got %v, want [...,online:A, sunrise]", got)
	}
}

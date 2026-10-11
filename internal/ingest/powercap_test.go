package ingest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/internal/store"
	"github.com/bolkedebruin/openaps/wire"
)

// syncRouter is a goroutine-safe Router: the re-assert path schedules a
// delayed read-back that sends from a background goroutine.
type syncRouter struct {
	mu    sync.Mutex
	sends []*wire.Envelope
}

func (r *syncRouter) SendToBackend(_ string, env *wire.Envelope) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sends = append(r.sends, env)
	return true
}

// setPowerSends returns the routed frames that are set-power writes.
func (r *syncRouter) setPowerSends() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out [][]byte
	for _, e := range r.sends {
		if f := e.GetSend().GetFrame(); codec.IsSetPower(f) {
			out = append(out, f)
		}
	}
	return out
}

func newCapIngestor(t *testing.T) (*Ingestor, *syncRouter) {
	t.Helper()
	in := newIngestor(t)
	r := &syncRouter{}
	in.Router = r
	in.RouteBackend = "ecu-zb"
	in.ControllerBackends = []string{"ecu-web"}
	return in, r
}

func capIntent(t *testing.T, in *Ingestor, uid string) (int, bool) {
	t.Helper()
	w, ok, err := in.S.PowerCapIntent(context.Background(), uid)
	if err != nil {
		t.Fatalf("PowerCapIntent: %v", err)
	}
	return w, ok
}

func TestCapDrift_AdoptsFirstReadback(t *testing.T) {
	t.Parallel()
	in, r := newCapIngestor(t)
	in.checkCapDrift(context.Background(), "806000042582", codec.ModelQS1A, map[string]float64{"DA": 375.02})
	if w, ok := capIntent(t, in, "806000042582"); !ok || w != 375 {
		t.Fatalf("intent = %d,%v; want 375,true", w, ok)
	}
	if n := len(r.setPowerSends()); n != 0 {
		t.Fatalf("adopt sent %d set-power frames, want 0", n)
	}
}

func TestCapDrift_DoesNotAdoptOutOfRange(t *testing.T) {
	t.Parallel()
	in, _ := newCapIngestor(t)
	in.checkCapDrift(context.Background(), "806000042582", codec.ModelQS1A, map[string]float64{"DA": 0.1385})
	if _, ok := capIntent(t, in, "806000042582"); ok {
		t.Fatal("adopted an out-of-range read-back as intent")
	}
}

func TestCapDrift_ReassertsOnceWithinInterval(t *testing.T) {
	t.Parallel()
	in, r := newCapIngestor(t)
	ctx := context.Background()
	const uid = "806000042582"
	if err := in.S.SetPowerCapIntent(ctx, uid, 500, 1, "ecu-web"); err != nil {
		t.Fatal(err)
	}
	in.checkCapDrift(ctx, uid, codec.ModelQS1A, map[string]float64{"DA": 0.1385})
	in.checkCapDrift(ctx, uid, codec.ModelQS1A, map[string]float64{"DA": 0.1385})

	sends := r.setPowerSends()
	if len(sends) != 1 {
		t.Fatalf("set-power sends = %d, want 1 (second is rate-limited)", len(sends))
	}
	want, _ := codec.EncodeSetPower(codec.ModelQS1A, 500, false)
	if !equalBytes(sends[0], want) {
		t.Errorf("re-assert frame % X, want % X", sends[0], want)
	}
	evs, err := in.S.QueryEvents(ctx, store.EventFilter{Kind: "power_cap_reasserted", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].InverterUID != uid {
		t.Fatalf("power_cap_reasserted events = %+v, want one for %s", evs, uid)
	}
}

func TestCapDrift_WithinToleranceIsQuiet(t *testing.T) {
	t.Parallel()
	in, r := newCapIngestor(t)
	ctx := context.Background()
	if err := in.S.SetPowerCapIntent(ctx, "704000006835", 375, 1, "ecu-web"); err != nil {
		t.Fatal(err)
	}
	in.checkCapDrift(ctx, "704000006835", codec.ModelDS3, map[string]float64{"DA": 374.9})
	in.checkCapDrift(ctx, "704000006835", codec.ModelDS3, map[string]float64{"AC": 196})
	if n := len(r.setPowerSends()); n != 0 {
		t.Fatalf("set-power sends = %d, want 0", n)
	}
}

func TestRecordCapIntent_UnicastAndBroadcast(t *testing.T) {
	t.Parallel()
	in, _ := newCapIngestor(t)
	ctx := context.Background()
	for uid, code := range map[string]uint32{
		"806000041211": uint32(codec.ModelQS1A),
		"806000042582": uint32(codec.ModelQS1A),
		"704000006835": uint32(codec.ModelDS3),
	} {
		c := code
		if _, err := in.S.UpsertInverterInfo(ctx, store.InverterInfoUpdate{UID: uid, TsMs: time.Now().UnixMilli(), ShortAddr: 1, ModelCode: &c}); err != nil {
			t.Fatal(err)
		}
	}

	uni, _ := codec.EncodeSetPower(codec.ModelDS3, 300, false)
	if err := in.Handle(ctx, "ecu-web", &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{PeerUid: "704000006835", Frame: uni}}}); err != nil {
		t.Fatal(err)
	}
	bc, _ := codec.EncodeSetPower(codec.ModelQS1A, 250, true)
	if err := in.Handle(ctx, "ecu-web", &wire.Envelope{Body: &wire.Envelope_Broadcast{Broadcast: &wire.Broadcast{Frame: bc}}}); err != nil {
		t.Fatal(err)
	}

	for uid, want := range map[string]int{"704000006835": 300, "806000041211": 250, "806000042582": 250} {
		if w, ok := capIntent(t, in, uid); !ok || w != want {
			t.Errorf("%s intent = %d,%v; want %d", uid, w, ok, want)
		}
	}
}

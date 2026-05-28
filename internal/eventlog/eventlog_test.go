package eventlog

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestHandle_MapsAndDefaultsLimit(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	_ = st.AppendEvent(ctx, 100, "uidA", "paired", "info", "", "")
	_ = st.AppendDecodeFailed(ctx, 200, 9, "crc", "AA")

	h := NewHandler(st)
	resp := h.Handle(ctx, &wire.EventsRequest{}) // limit 0 -> default
	if !resp.GetOk() {
		t.Fatalf("not ok: %s", resp.GetError())
	}
	if len(resp.GetEvents()) != 2 {
		t.Fatalf("got %d events, want 2", len(resp.GetEvents()))
	}
	// newest-first.
	top := resp.GetEvents()[0]
	if top.GetKind() != "decode_failed" || top.GetShortAddr() != 9 || top.GetDetail() != "crc" {
		t.Errorf("top event wrong: %+v", top)
	}
}

func TestHandle_ExcludesFirehoseByDefault(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	_ = st.AppendEvent(ctx, 100, "u", "paired", "info", "", "")
	for i := int64(0); i < 10; i++ {
		_ = st.AppendEvent(ctx, 200+i, "u", "telemetry", "info", "", "")
		_ = st.AppendEvent(ctx, 300+i, "u", "inverter_info", "info", "", "")
	}
	h := NewHandler(st)

	// Default (no kind): firehose kinds excluded -> only the paired event.
	resp := h.Handle(ctx, &wire.EventsRequest{})
	if len(resp.GetEvents()) != 1 || resp.GetEvents()[0].GetKind() != "paired" {
		t.Fatalf("default should exclude telemetry/inverter_info, got %d events", len(resp.GetEvents()))
	}
	// Explicit kind=telemetry still returns them.
	resp = h.Handle(ctx, &wire.EventsRequest{Kind: "telemetry"})
	if len(resp.GetEvents()) != 10 {
		t.Errorf("explicit kind=telemetry should return 10, got %d", len(resp.GetEvents()))
	}
}

func TestHandle_LimitClamped(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	for i := int64(0); i < 5; i++ {
		_ = st.AppendEvent(ctx, i+1, "u", "k", "info", "", "")
	}
	h := &Handler{Store: st, DefaultLimit: 200, MaxLimit: 3}
	resp := h.Handle(ctx, &wire.EventsRequest{Limit: 100}) // clamps to MaxLimit=3
	if len(resp.GetEvents()) != 3 {
		t.Errorf("got %d, want 3 (clamped)", len(resp.GetEvents()))
	}
}

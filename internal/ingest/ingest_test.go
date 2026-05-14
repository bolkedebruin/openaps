package ingest

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
)

func newIngestor(t *testing.T) *Ingestor {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return &Ingestor{S: s}
}

func TestHandle_Hello(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend:     "apsystems-stock-zb",
		Version:     "0.1.0",
		StartedAtMs: 1234,
	}}}
	if err := in.Handle(context.Background(), "apsystems-stock-zb", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	var kind, severity string
	var uid sql.NullString
	if err := in.S.DB().QueryRow(`SELECT kind, severity, inverter_uid FROM events`).Scan(&kind, &severity, &uid); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if kind != "backend_hello" {
		t.Fatalf("kind: got %q want backend_hello", kind)
	}
	if severity != "info" {
		t.Fatalf("severity: got %q want info", severity)
	}
	if uid.Valid {
		t.Fatalf("inverter_uid should be NULL, got %q", uid.String)
	}
}

func TestHandle_Telemetry_QS1A(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
		TsMs:         9999,
		ShortAddr:    0x5011,
		PeerUid:      "806000042582",
		Cmd:          0xB1,
		Model:        "QS1A",
		GridV:        232.0,
		BusV:         400.0,
		FreqHz:       50.0,
		ActivePowerW: 800.0,
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var family sql.NullString
	if err := in.S.DB().QueryRow(`SELECT family FROM inverters WHERE uid='806000042582'`).Scan(&family); err != nil {
		t.Fatalf("inverter: %v", err)
	}
	if !family.Valid || family.String != "qs1a" {
		t.Fatalf("family: got %v want qs1a", family)
	}

	var acW, acV, freq, busV float64
	if err := in.S.DB().QueryRow(`SELECT ac_w, ac_v, ac_freq, bus_v FROM telemetry_live WHERE inverter_uid='806000042582'`).Scan(&acW, &acV, &freq, &busV); err != nil {
		t.Fatalf("live: %v", err)
	}
	if acW != 800.0 || acV != 232.0 || freq != 50.0 || busV != 400.0 {
		t.Fatalf("live values: ac_w=%v ac_v=%v freq=%v bus=%v", acW, acV, freq, busV)
	}

	var kind string
	if err := in.S.DB().QueryRow(`SELECT kind FROM events WHERE inverter_uid='806000042582'`).Scan(&kind); err != nil {
		t.Fatalf("event: %v", err)
	}
	if kind != "telemetry" {
		t.Fatalf("event kind: got %q want telemetry", kind)
	}
}

func TestHandle_Telemetry_UnknownModel_FamilyNull(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
		TsMs:    100,
		PeerUid: "unknownuid01",
		Model:   "unknown(0xAB)",
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	var family sql.NullString
	if err := in.S.DB().QueryRow(`SELECT family FROM inverters WHERE uid='unknownuid01'`).Scan(&family); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if family.Valid {
		t.Fatalf("family should be NULL, got %q", family.String)
	}
}

func TestHandle_Telemetry_EmptyPeerUID(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
		TsMs:    100,
		PeerUid: "",
		Model:   "QS1A",
	}}}
	err := in.Handle(context.Background(), "be", env)
	if err == nil {
		t.Fatal("expected error on empty peer_uid")
	}
	if !strings.Contains(err.Error(), "peer_uid") {
		t.Fatalf("error: got %q", err.Error())
	}
	for _, tbl := range []string{"inverters", "telemetry_live", "events"} {
		var n int
		if err := in.S.DB().QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", tbl, err)
		}
		if n != 0 {
			t.Fatalf("%s: got %d rows, want 0", tbl, n)
		}
	}
}

func TestHandle_DecodeFailed(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_DecodeFailed{DecodeFailed: &wire.DecodeFailed{
		TsMs:  42,
		Error: "L2 CRC mismatch",
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	var kind, severity string
	if err := in.S.DB().QueryRow(`SELECT kind, severity FROM events`).Scan(&kind, &severity); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if kind != "decode_failed" {
		t.Fatalf("kind: got %q want decode_failed", kind)
	}
	if severity != "warn" {
		t.Fatalf("severity: got %q want warn", severity)
	}
}

func TestHandle_EmptyEnvelope(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{} // no body oneof set
	err := in.Handle(context.Background(), "be", env)
	if err == nil {
		t.Fatal("expected error on envelope without body")
	}
	if !strings.Contains(err.Error(), "without body") {
		t.Fatalf("error: got %q", err.Error())
	}
}

func TestHandle_HelloWithoutBody(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: nil}}
	if err := in.Handle(context.Background(), "be", env); err == nil {
		t.Fatal("expected error on hello without body")
	}
}

func TestHandle_TelemetryWithoutBody(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: nil}}
	if err := in.Handle(context.Background(), "be", env); err == nil {
		t.Fatal("expected error on telemetry without body")
	}
}

func TestHandle_DecodeFailedWithoutBody(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_DecodeFailed{DecodeFailed: nil}}
	if err := in.Handle(context.Background(), "be", env); err == nil {
		t.Fatal("expected error on decode_failed without body")
	}
}

func TestFamilyFromModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		model string
		want  string
	}{
		{"QS1A", "qs1a"},
		{"QS1A-something", "qs1a"},
		{"QS1", "qs1"},
		{"QS1-X", "qs1"},
		{"DS3", "ds3"},
		{"DS3-L", "ds3"},
		{"DSP1", "dsp"},
		{"YC600", "yc600"},
		{"YC1000", "yc1000"},
		{"unknown(0xAB)", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := familyFromModel(c.model); got != c.want {
			t.Errorf("familyFromModel(%q) = %q, want %q", c.model, got, c.want)
		}
	}
}

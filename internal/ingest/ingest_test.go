package ingest

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/internal/events"
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

func TestHandle_Hello_NoRowWritten(t *testing.T) {
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
	var n int
	if err := in.S.DB().QueryRow(`SELECT COUNT(*) FROM events`).Scan(&n); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if n != 0 {
		t.Fatalf("events: got %d rows after Hello, want 0", n)
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
		TsMs:      42,
		ShortAddr: 0x1234,
		Error:     "L2 CRC mismatch",
		RawHex:    "deadbeef",
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	var kind, severity string
	var shortAddr sql.NullInt64
	var errMsg, rawHex sql.NullString
	if err := in.S.DB().QueryRow(`SELECT kind, severity, short_addr, error, raw_hex FROM events`).Scan(&kind, &severity, &shortAddr, &errMsg, &rawHex); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if kind != "decode_failed" {
		t.Fatalf("kind: got %q want decode_failed", kind)
	}
	if severity != "warn" {
		t.Fatalf("severity: got %q want warn", severity)
	}
	if !shortAddr.Valid || shortAddr.Int64 != 0x1234 {
		t.Fatalf("short_addr: %+v", shortAddr)
	}
	if !errMsg.Valid || errMsg.String != "L2 CRC mismatch" {
		t.Fatalf("error: %+v", errMsg)
	}
	if !rawHex.Valid || rawHex.String != "deadbeef" {
		t.Fatalf("raw_hex: %+v", rawHex)
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

func TestHandle_RawFrame_QS1A(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	in.Pub = events.New()
	ch, unsub := in.Pub.Subscribe()
	defer unsub()

	env := &wire.Envelope{Body: &wire.Envelope_RawFrame{RawFrame: &wire.RawFrame{
		TsMs:      1234,
		ShortAddr: 0x5011,
		L1Frame:   codec.QS1AFixture,
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// telemetry_live row landed.
	var acV float64
	if err := in.S.DB().QueryRow(`SELECT ac_v FROM telemetry_live WHERE inverter_uid='806000042582'`).Scan(&acV); err != nil {
		t.Fatalf("scan ac_v: %v", err)
	}
	if acV < 200 || acV > 250 {
		t.Errorf("ac_v: got %v want ~223", acV)
	}

	// telemetry event row landed.
	var kind string
	if err := in.S.DB().QueryRow(`SELECT kind FROM events WHERE inverter_uid='806000042582'`).Scan(&kind); err != nil {
		t.Fatalf("event: %v", err)
	}
	if kind != "telemetry" {
		t.Fatalf("event kind: got %q want telemetry", kind)
	}

	// inverter_panels populated (QS1A has 4 channels).
	var panelCount int
	if err := in.S.DB().QueryRow(`SELECT COUNT(*) FROM inverter_panels WHERE inverter_uid='806000042582'`).Scan(&panelCount); err != nil {
		t.Fatalf("count panels: %v", err)
	}
	if panelCount != 4 {
		t.Fatalf("panels: got %d want 4", panelCount)
	}

	// energy_lifetime populated.
	var lifetimeCount int
	if err := in.S.DB().QueryRow(`SELECT COUNT(*) FROM energy_lifetime WHERE inverter_uid='806000042582'`).Scan(&lifetimeCount); err != nil {
		t.Fatalf("count energy_lifetime: %v", err)
	}
	if lifetimeCount != 4 {
		t.Fatalf("energy_lifetime: got %d want 4", lifetimeCount)
	}

	// A Telemetry envelope must have been published.
	select {
	case got := <-ch:
		tel := got.GetTelemetry()
		if tel == nil {
			t.Fatalf("published envelope is not Telemetry: %T", got.GetBody())
		}
		if tel.GetPeerUid() != "806000042582" {
			t.Fatalf("published peer_uid: got %q", tel.GetPeerUid())
		}
	default:
		t.Fatal("no envelope published")
	}
}

func TestHandle_RawFrame_DecodeFailure(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	in.Pub = events.New()
	ch, unsub := in.Pub.Subscribe()
	defer unsub()

	// Garbage L1 frame — too short, will fail ParseL1.
	env := &wire.Envelope{Body: &wire.Envelope_RawFrame{RawFrame: &wire.RawFrame{
		TsMs:    42,
		L1Frame: []byte{0x00, 0x00, 0x00, 0x00},
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	var kind, severity string
	if err := in.S.DB().QueryRow(`SELECT kind, severity FROM events`).Scan(&kind, &severity); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if kind != "decode_failed" || severity != "warn" {
		t.Fatalf("event: kind=%q severity=%q want decode_failed/warn", kind, severity)
	}
	// Nothing should have been published.
	select {
	case got := <-ch:
		t.Fatalf("unexpected publish on decode failure: %T", got.GetBody())
	default:
	}
}

func TestHandle_Telemetry_PublishesToSubscribers(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	in.Pub = events.New()
	ch, unsub := in.Pub.Subscribe()
	defer unsub()

	env := &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
		TsMs:    5,
		PeerUid: "uid-pub-1",
		Model:   "QS1A",
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	select {
	case got := <-ch:
		if got.GetTelemetry().GetPeerUid() != "uid-pub-1" {
			t.Fatalf("publish: got %q", got.GetTelemetry().GetPeerUid())
		}
	default:
		t.Fatal("legacy Telemetry frame did not publish")
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

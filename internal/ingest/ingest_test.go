package ingest

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bolke/inv-driver/codec/codectest"
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

type fakeRouter struct {
	calls   []routedCall
	refuse  bool
}

type routedCall struct {
	backend string
	env     *wire.Envelope
}

func (f *fakeRouter) SendToBackend(backend string, env *wire.Envelope) bool {
	if f.refuse {
		return false
	}
	f.calls = append(f.calls, routedCall{backend: backend, env: env})
	return true
}

func TestHandle_Send_RoutesToBackend(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	r := &fakeRouter{}
	in.Router = r
	in.RouteBackend = "apsystems-stock-zb"
	in.ControllerBackends = []string{"inv-driver-cli"}

	frame := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x55, 0x12, 0x34}
	env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid: "806000042582", Frame: frame,
	}}}
	if err := in.Handle(context.Background(), "inv-driver-cli", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("Router calls: got %d want 1", len(r.calls))
	}
	if r.calls[0].backend != "apsystems-stock-zb" {
		t.Errorf("backend: got %q", r.calls[0].backend)
	}
	if got := r.calls[0].env.GetSend().GetFrame(); !equalBytes(got, frame) {
		t.Errorf("frame mismatch: got % X want % X", got, frame)
	}
}

func TestHandle_Broadcast_RoutesToBackend(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	r := &fakeRouter{}
	in.Router = r
	in.RouteBackend = "apsystems-stock-zb"
	in.ControllerBackends = []string{"inv-driver-cli"}

	frame := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x55, 0x00, 0x00}
	env := &wire.Envelope{Body: &wire.Envelope_Broadcast{Broadcast: &wire.Broadcast{Frame: frame}}}
	if err := in.Handle(context.Background(), "inv-driver-cli", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(r.calls) != 1 || r.calls[0].env.GetBroadcast() == nil {
		t.Fatalf("Broadcast not routed: %+v", r.calls)
	}
}

func TestHandle_Send_NoRouterDrops(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{Frame: []byte{0xAA}}}}
	if err := in.Handle(context.Background(), "x", env); err != nil {
		t.Fatalf("Handle (drop): %v", err)
	}
}

func TestHandle_Send_RouterRefusalIsError(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	r := &fakeRouter{refuse: true}
	in.Router = r
	in.RouteBackend = "apsystems-stock-zb"
	in.ControllerBackends = []string{"inv-driver-cli"}
	env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{Frame: []byte{0xAA}}}}
	err := in.Handle(context.Background(), "inv-driver-cli", env)
	if err == nil {
		t.Fatalf("expected error on router refusal")
	}
}

func TestHandle_Send_LoopbackRejected(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	r := &fakeRouter{}
	in.Router = r
	in.RouteBackend = "apsystems-stock-zb"
	in.ControllerBackends = []string{"inv-driver-cli"}

	env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{Frame: []byte{0xAA}}}}
	err := in.Handle(context.Background(), "apsystems-stock-zb", env)
	if err == nil {
		t.Fatalf("expected loopback rejection")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("error should mention loopback: %v", err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("router was called: %+v", r.calls)
	}
}

func TestHandle_Send_UnknownSenderRejected(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	r := &fakeRouter{}
	in.Router = r
	in.RouteBackend = "apsystems-stock-zb"
	in.ControllerBackends = []string{"inv-driver-cli"}

	env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{Frame: []byte{0xAA}}}}
	err := in.Handle(context.Background(), "stranger", env)
	if err == nil {
		t.Fatalf("expected unknown-sender rejection")
	}
	if len(r.calls) != 0 {
		t.Fatalf("router was called: %+v", r.calls)
	}
}

func TestHandle_Broadcast_LoopbackRejected(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	r := &fakeRouter{}
	in.Router = r
	in.RouteBackend = "apsystems-stock-zb"
	in.ControllerBackends = []string{"inv-driver-cli"}

	env := &wire.Envelope{Body: &wire.Envelope_Broadcast{Broadcast: &wire.Broadcast{Frame: []byte{0xAA}}}}
	err := in.Handle(context.Background(), "apsystems-stock-zb", env)
	if err == nil {
		t.Fatalf("expected loopback rejection")
	}
	if len(r.calls) != 0 {
		t.Fatalf("router was called: %+v", r.calls)
	}
}

func TestHandle_Broadcast_UnknownSenderRejected(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	r := &fakeRouter{}
	in.Router = r
	in.RouteBackend = "apsystems-stock-zb"
	in.ControllerBackends = []string{"inv-driver-cli"}

	env := &wire.Envelope{Body: &wire.Envelope_Broadcast{Broadcast: &wire.Broadcast{Frame: []byte{0xAA}}}}
	err := in.Handle(context.Background(), "stranger", env)
	if err == nil {
		t.Fatalf("expected unknown-sender rejection")
	}
	if len(r.calls) != 0 {
		t.Fatalf("router was called: %+v", r.calls)
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
		PeerUid: "0000000000e1",
		Model:   "unknown(0xAB)",
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	var family sql.NullString
	if err := in.S.DB().QueryRow(`SELECT family FROM inverters WHERE uid='0000000000e1'`).Scan(&family); err != nil {
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
		L1Frame:   codectest.QS1AFixture,
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
		PeerUid: "000000000fa1",
		Model:   "QS1A",
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	select {
	case got := <-ch:
		if got.GetTelemetry().GetPeerUid() != "000000000fa1" {
			t.Fatalf("publish: got %q", got.GetTelemetry().GetPeerUid())
		}
	default:
		t.Fatal("legacy Telemetry frame did not publish")
	}
}

func u32p(v uint32) *uint32 { return &v }
func boolp(b bool) *bool     { return &b }

func TestHandle_InverterInfo_ModelOnly(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	in.Pub = events.New()
	ch, unsub := in.Pub.Subscribe()
	defer unsub()

	env := &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{
		TsMs:      111,
		PeerUid:   "806000042582",
		ShortAddr: 0x5011,
		ModelCode: u32p(0x24),
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var modelCode sql.NullInt64
	var sw, phase, bound, rpt sql.NullInt64
	if err := in.S.DB().QueryRow(`SELECT model_code, software_version, phase, zigbee_bound, turned_off_rpt FROM inverters WHERE uid='806000042582'`).
		Scan(&modelCode, &sw, &phase, &bound, &rpt); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !modelCode.Valid || modelCode.Int64 != 0x24 {
		t.Fatalf("model_code: %+v", modelCode)
	}
	if sw.Valid || phase.Valid || bound.Valid || rpt.Valid {
		t.Fatalf("expected unset columns NULL: sw=%+v phase=%+v bound=%+v rpt=%+v", sw, phase, bound, rpt)
	}

	var kind string
	if err := in.S.DB().QueryRow(`SELECT kind FROM events WHERE inverter_uid='806000042582'`).Scan(&kind); err != nil {
		t.Fatalf("event scan: %v", err)
	}
	if kind != "inverter_info" {
		t.Fatalf("event kind: got %q want inverter_info", kind)
	}

	select {
	case got := <-ch:
		if got.GetInfo().GetPeerUid() != "806000042582" {
			t.Fatalf("published peer_uid: %q", got.GetInfo().GetPeerUid())
		}
	default:
		t.Fatal("inverter_info envelope was not published")
	}
}

func TestHandle_InverterInfo_AllFields(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{
		TsMs:            222,
		PeerUid:         "000000000a11",
		ShortAddr:       0xC459,
		ModelCode:       u32p(0x29),
		SoftwareVersion: u32p(101000),
		Phase:           u32p(3),
		ZigbeeBound:     boolp(true),
		TurnedOffRpt:    boolp(false),
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var modelCode, sw, phase, bound, rpt sql.NullInt64
	if err := in.S.DB().QueryRow(`SELECT model_code, software_version, phase, zigbee_bound, turned_off_rpt FROM inverters WHERE uid='000000000a11'`).
		Scan(&modelCode, &sw, &phase, &bound, &rpt); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if modelCode.Int64 != 0x29 || sw.Int64 != 101000 || phase.Int64 != 3 || bound.Int64 != 1 || rpt.Int64 != 0 {
		t.Fatalf("cols: model=%d sw=%d phase=%d bound=%d rpt=%d",
			modelCode.Int64, sw.Int64, phase.Int64, bound.Int64, rpt.Int64)
	}
}

func TestHandle_InverterInfo_EmptyPeerUID(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{
		TsMs:    100,
		PeerUid: "",
	}}}
	err := in.Handle(context.Background(), "be", env)
	if err == nil {
		t.Fatal("expected error on empty peer_uid")
	}
	if !strings.Contains(err.Error(), "peer_uid") {
		t.Fatalf("error: %q", err)
	}
	var n int
	if err := in.S.DB().QueryRow(`SELECT COUNT(*) FROM inverters`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("inverters rows: got %d want 0", n)
	}
}

func TestHandle_InverterInfo_Sequence_Accumulates(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)

	// 1) seed with model only
	if err := in.Handle(context.Background(), "be", &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{
		TsMs: 1, PeerUid: "000000000acc", ShortAddr: 0x10,
		ModelCode: u32p(0x24),
	}}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// 2) add software version
	if err := in.Handle(context.Background(), "be", &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{
		TsMs: 2, PeerUid: "000000000acc", ShortAddr: 0x10,
		SoftwareVersion: u32p(7000),
	}}}); err != nil {
		t.Fatalf("sw: %v", err)
	}
	// 3) add phase + bound
	if err := in.Handle(context.Background(), "be", &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{
		TsMs: 3, PeerUid: "000000000acc", ShortAddr: 0x10,
		Phase:       u32p(1),
		ZigbeeBound: boolp(true),
	}}}); err != nil {
		t.Fatalf("phase: %v", err)
	}

	var modelCode, sw, phase, bound sql.NullInt64
	if err := in.S.DB().QueryRow(`SELECT model_code, software_version, phase, zigbee_bound FROM inverters WHERE uid='000000000acc'`).
		Scan(&modelCode, &sw, &phase, &bound); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if modelCode.Int64 != 0x24 || sw.Int64 != 7000 || phase.Int64 != 1 || bound.Int64 != 1 {
		t.Fatalf("accumulated cols: model=%d sw=%d phase=%d bound=%d",
			modelCode.Int64, sw.Int64, phase.Int64, bound.Int64)
	}
}

func TestHandle_InverterInfoWithoutBody(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	env := &wire.Envelope{Body: &wire.Envelope_Info{Info: nil}}
	if err := in.Handle(context.Background(), "be", env); err == nil {
		t.Fatal("expected error on inverter_info without body")
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

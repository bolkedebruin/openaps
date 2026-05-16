package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpen_FreshAndIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")

	s1, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := s1.UpsertInverterFromTelemetry(context.Background(), "uid1", 0x1234, "qs1a", "QS1A", 1000); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s2, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s2.Close()

	var n int
	if err := s2.DB().QueryRow(`SELECT COUNT(*) FROM inverters WHERE uid='uid1'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("inverters count: got %d want 1", n)
	}
}

func TestUpsertInverter_FirstInsertThenUpdate(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.UpsertInverterFromTelemetry(ctx, "abc", 0x1111, "qs1a", "QS1A", 1000); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	var pairedAt, lastSeen int64
	if err := s.DB().QueryRow(`SELECT paired_at_ms, last_seen_ms FROM inverters WHERE uid='abc'`).Scan(&pairedAt, &lastSeen); err != nil {
		t.Fatalf("read after first: %v", err)
	}
	if pairedAt != 1000 || lastSeen != 1000 {
		t.Fatalf("first insert: paired_at=%d last_seen=%d, want both 1000", pairedAt, lastSeen)
	}

	if err := s.UpsertInverterFromTelemetry(ctx, "abc", 0x2222, "qs1a", "QS1A", 2000); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	var shortAddr int
	if err := s.DB().QueryRow(`SELECT paired_at_ms, last_seen_ms, short_addr FROM inverters WHERE uid='abc'`).Scan(&pairedAt, &lastSeen, &shortAddr); err != nil {
		t.Fatalf("read after second: %v", err)
	}
	if pairedAt != 1000 {
		t.Fatalf("paired_at must be preserved: got %d want 1000", pairedAt)
	}
	if lastSeen != 2000 {
		t.Fatalf("last_seen must update: got %d want 2000", lastSeen)
	}
	if shortAddr != 0x2222 {
		t.Fatalf("short_addr must update: got 0x%X want 0x2222", shortAddr)
	}
}

func TestWriteTelemetryLive_Overwrites(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.UpsertInverterFromTelemetry(ctx, "uid", 0, "qs1a", "QS1A", 100); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.WriteTelemetryLive(ctx, "uid", 100, 0xB1, 100.0, 230.0, 50.0, 380.0, 60); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := s.WriteTelemetryLive(ctx, "uid", 200, 0xB1, 200.0, 231.0, 50.1, 381.0, 120); err != nil {
		t.Fatalf("second write: %v", err)
	}

	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM telemetry_live`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("telemetry_live count: got %d want 1", n)
	}
	var ts int64
	var acW float64
	var cmd int64
	if err := s.DB().QueryRow(`SELECT ts_ms, cmd, ac_w FROM telemetry_live WHERE inverter_uid='uid'`).Scan(&ts, &cmd, &acW); err != nil {
		t.Fatalf("read: %v", err)
	}
	if ts != 200 || acW != 200.0 || cmd != 0xB1 {
		t.Fatalf("overwrite: ts=%d cmd=%x ac_w=%v, want 200/0xB1/200", ts, cmd, acW)
	}
}

func TestWritePanels_Upserts(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()
	if err := s.UpsertInverterFromTelemetry(ctx, "uid", 0, "qs1a", "QS1A", 100); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.WritePanels(ctx, "uid", 100, []PanelRow{
		{ChannelIdx: 0, DCV: 30.0, DCI: 4.5, W: 135.0},
		{ChannelIdx: 1, DCV: 31.0, DCI: 4.6, W: 142.6},
	}); err != nil {
		t.Fatalf("first WritePanels: %v", err)
	}
	if err := s.WritePanels(ctx, "uid", 200, []PanelRow{
		{ChannelIdx: 0, DCV: 32.0, DCI: 5.0, W: 160.0},
		{ChannelIdx: 1, DCV: 33.0, DCI: 5.1, W: 168.3},
		{ChannelIdx: 2, DCV: 34.0, DCI: 5.2, W: 176.8},
	}); err != nil {
		t.Fatalf("second WritePanels: %v", err)
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM inverter_panels WHERE inverter_uid='uid'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Fatalf("panels count: got %d want 3", n)
	}
	var dcV, w float64
	var lastSeen int64
	if err := s.DB().QueryRow(`SELECT dc_v, w, last_seen_ms FROM inverter_panels WHERE inverter_uid='uid' AND channel_idx=0`).Scan(&dcV, &w, &lastSeen); err != nil {
		t.Fatalf("read ch0: %v", err)
	}
	if dcV != 32.0 || w != 160.0 || lastSeen != 200 {
		t.Fatalf("ch0 after upsert: dc_v=%v w=%v last_seen_ms=%d", dcV, w, lastSeen)
	}
}

func TestWriteEnergyLifetime_Upserts(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()
	if err := s.UpsertInverterFromTelemetry(ctx, "uid", 0, "qs1a", "QS1A", 100); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.WriteEnergyLifetime(ctx, "uid", 100, []EnergyRow{
		{ChannelIdx: 0, Raw: 1000, Scale: 3.756e-11},
		{ChannelIdx: 1, Raw: 2000, Scale: 3.756e-11},
	}); err != nil {
		t.Fatalf("first WriteEnergyLifetime: %v", err)
	}
	if err := s.WriteEnergyLifetime(ctx, "uid", 200, []EnergyRow{
		{ChannelIdx: 0, Raw: 1500, Scale: 3.756e-11},
	}); err != nil {
		t.Fatalf("second WriteEnergyLifetime: %v", err)
	}
	var raw int64
	var scale float64
	var lastUpdate int64
	if err := s.DB().QueryRow(`SELECT raw, scale, last_update_ms FROM energy_lifetime WHERE inverter_uid='uid' AND channel_idx=0`).Scan(&raw, &scale, &lastUpdate); err != nil {
		t.Fatalf("read ch0: %v", err)
	}
	if raw != 1500 || lastUpdate != 200 {
		t.Fatalf("ch0 after upsert: raw=%d last_update_ms=%d", raw, lastUpdate)
	}
	if scale == 0 {
		t.Fatalf("scale should not be zero")
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM energy_lifetime WHERE inverter_uid='uid'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("energy_lifetime count: got %d want 2 (channel 1 must survive)", n)
	}
}

func TestAppendEvent_Defaults(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.AppendEvent(ctx, 10, "uidA", "kindA", ""); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := s.AppendEvent(ctx, 20, "uidB", "kindB", "warn"); err != nil {
		t.Fatalf("append 2: %v", err)
	}

	type row struct {
		id        int64
		uid       sql.NullString
		kind      string
		severity  string
		shortAddr sql.NullInt64
		errMsg    sql.NullString
		rawHex    sql.NullString
	}
	rows, err := s.DB().Query(`SELECT id, inverter_uid, kind, severity, short_addr, error, raw_hex FROM events ORDER BY id`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.uid, &r.kind, &r.severity, &r.shortAddr, &r.errMsg, &r.rawHex); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if len(got) != 2 {
		t.Fatalf("rows: got %d want 2", len(got))
	}
	if got[1].id <= got[0].id {
		t.Fatalf("autoincrement: id2=%d not greater than id1=%d", got[1].id, got[0].id)
	}
	if got[0].severity != "info" {
		t.Fatalf("severity default: got %q want \"info\"", got[0].severity)
	}
	if got[0].shortAddr.Valid || got[0].errMsg.Valid || got[0].rawHex.Valid {
		t.Fatalf("typed columns must be NULL for non-decode_failed rows: %+v", got[0])
	}
	if got[1].severity != "warn" {
		t.Fatalf("severity: got %q want \"warn\"", got[1].severity)
	}
}

func TestAppendDecodeFailed_PopulatesTypedColumns(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()
	if err := s.AppendDecodeFailed(ctx, 99, 0x5011, "L2 CRC mismatch", "deadbeef"); err != nil {
		t.Fatalf("AppendDecodeFailed: %v", err)
	}
	var kind, severity string
	var shortAddr sql.NullInt64
	var errMsg, rawHex sql.NullString
	if err := s.DB().QueryRow(`SELECT kind, severity, short_addr, error, raw_hex FROM events`).Scan(&kind, &severity, &shortAddr, &errMsg, &rawHex); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if kind != "decode_failed" || severity != "warn" {
		t.Fatalf("kind/severity: %q/%q", kind, severity)
	}
	if !shortAddr.Valid || shortAddr.Int64 != 0x5011 {
		t.Fatalf("short_addr: %+v", shortAddr)
	}
	if !errMsg.Valid || errMsg.String != "L2 CRC mismatch" {
		t.Fatalf("error: %+v", errMsg)
	}
	if !rawHex.Valid || rawHex.String != "deadbeef" {
		t.Fatalf("raw_hex: %+v", rawHex)
	}
}

func TestAppendEvent_NullableInverterUID(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()
	if err := s.AppendEvent(ctx, 1, "", "service_start", "info"); err != nil {
		t.Fatalf("append: %v", err)
	}
	var uid sql.NullString
	if err := s.DB().QueryRow(`SELECT inverter_uid FROM events`).Scan(&uid); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if uid.Valid {
		t.Fatalf("expected NULL inverter_uid, got %q", uid.String)
	}
}

func TestNullableHelpers_StoreNULLs(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.UpsertInverterFromTelemetry(ctx, "u", 0, "", "", 1); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.WriteTelemetryLive(ctx, "u", 1, 0, 0, 0, 0, 0, 0); err != nil {
		t.Fatalf("write live: %v", err)
	}

	var family, model sql.NullString
	if err := s.DB().QueryRow(`SELECT family, model FROM inverters WHERE uid='u'`).Scan(&family, &model); err != nil {
		t.Fatalf("scan inverter: %v", err)
	}
	if family.Valid {
		t.Fatalf("family should be NULL, got %q", family.String)
	}
	if model.Valid {
		t.Fatalf("model should be NULL, got %q", model.String)
	}

	var acW, acV, acFreq, busV sql.NullFloat64
	var reportSec sql.NullInt64
	if err := s.DB().QueryRow(`SELECT ac_w, ac_v, ac_freq, bus_v, report_sec FROM telemetry_live WHERE inverter_uid='u'`).Scan(&acW, &acV, &acFreq, &busV, &reportSec); err != nil {
		t.Fatalf("scan live: %v", err)
	}
	for name, v := range map[string]sql.NullFloat64{"ac_w": acW, "ac_v": acV, "ac_freq": acFreq, "bus_v": busV} {
		if v.Valid {
			t.Fatalf("%s should be NULL, got %v", name, v.Float64)
		}
	}
	if reportSec.Valid {
		t.Fatalf("report_sec should be NULL, got %d", reportSec.Int64)
	}
}

func TestConcurrentWrites(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()

	const goroutines = 50
	const perGoroutine = 5

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*perGoroutine)

	for g := 0; g < goroutines; g++ {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				uid := fmt.Sprintf("uid-%03d-%d", g, i)
				ts := int64(1000*g + i)
				if err := s.UpsertInverterFromTelemetry(ctx, uid, uint16(g), "qs1a", "QS1A", ts); err != nil {
					errCh <- fmt.Errorf("upsert %s: %w", uid, err)
					return
				}
				if err := s.WriteTelemetryLive(ctx, uid, ts, 0xB1, float64(i), 230, 50, 380, 60); err != nil {
					errCh <- fmt.Errorf("live %s: %w", uid, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}

	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM telemetry_live`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != goroutines*perGoroutine {
		t.Fatalf("telemetry_live rows: got %d want %d", n, goroutines*perGoroutine)
	}
}

func u32p(v uint32) *uint32 { return &v }
func boolp(b bool) *bool     { return &b }

func TestUpsertInverterInfo_ModelOnly_LeavesOthersNull(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openTestStore(t)

	if err := s.UpsertInverterInfo(ctx, InverterInfoUpdate{
		UID: "u1", TsMs: 100, ShortAddr: 0x5011,
		Model: u32p(0x24),
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	var modelCode, sw, phase sql.NullInt64
	var bound, rpt sql.NullInt64
	if err := s.DB().QueryRow(`SELECT model_code, software_version, phase, zigbee_bound, turned_off_rpt FROM inverters WHERE uid='u1'`).
		Scan(&modelCode, &sw, &phase, &bound, &rpt); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !modelCode.Valid || modelCode.Int64 != 0x24 {
		t.Fatalf("model_code: %+v want 0x24", modelCode)
	}
	for name, v := range map[string]sql.NullInt64{
		"software_version": sw, "phase": phase, "zigbee_bound": bound, "turned_off_rpt": rpt,
	} {
		if v.Valid {
			t.Fatalf("%s should be NULL on model-only insert, got %d", name, v.Int64)
		}
	}

	if err := s.UpsertInverterInfo(ctx, InverterInfoUpdate{
		UID: "u1", TsMs: 200, ShortAddr: 0x5011,
		Model: u32p(0x36),
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if err := s.DB().QueryRow(`SELECT model_code, software_version, phase, zigbee_bound, turned_off_rpt FROM inverters WHERE uid='u1'`).
		Scan(&modelCode, &sw, &phase, &bound, &rpt); err != nil {
		t.Fatalf("scan after second: %v", err)
	}
	if !modelCode.Valid || modelCode.Int64 != 0x36 {
		t.Fatalf("model_code update: %+v want 0x36", modelCode)
	}
	for name, v := range map[string]sql.NullInt64{
		"software_version": sw, "phase": phase, "zigbee_bound": bound, "turned_off_rpt": rpt,
	} {
		if v.Valid {
			t.Fatalf("%s must remain NULL after partial update, got %d", name, v.Int64)
		}
	}
}

func TestUpsertInverterInfo_AllFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openTestStore(t)

	if err := s.UpsertInverterInfo(ctx, InverterInfoUpdate{
		UID: "u2", TsMs: 1000, ShortAddr: 0xC459,
		Model: u32p(0x29), SoftwareVer: u32p(12345), Phase: u32p(3),
		Bound: boolp(true), RptOff: boolp(false),
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var modelCode, sw, phase, bound, rpt sql.NullInt64
	var shortAddr int
	var pairedAt, lastSeen int64
	if err := s.DB().QueryRow(`SELECT short_addr, paired_at_ms, last_seen_ms, model_code, software_version, phase, zigbee_bound, turned_off_rpt FROM inverters WHERE uid='u2'`).
		Scan(&shortAddr, &pairedAt, &lastSeen, &modelCode, &sw, &phase, &bound, &rpt); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if shortAddr != 0xC459 || pairedAt != 1000 || lastSeen != 1000 {
		t.Fatalf("base cols: sa=0x%X paired=%d last=%d", shortAddr, pairedAt, lastSeen)
	}
	if modelCode.Int64 != 0x29 || sw.Int64 != 12345 || phase.Int64 != 3 {
		t.Fatalf("typed cols: model=%d sw=%d phase=%d", modelCode.Int64, sw.Int64, phase.Int64)
	}
	if bound.Int64 != 1 || rpt.Int64 != 0 {
		t.Fatalf("bool cols: bound=%d rpt=%d (want 1/0)", bound.Int64, rpt.Int64)
	}
}

func TestUpsertInverterInfo_PartialPreservesPrior(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openTestStore(t)

	// Seed full row.
	if err := s.UpsertInverterInfo(ctx, InverterInfoUpdate{
		UID: "u3", TsMs: 500, ShortAddr: 0x1111,
		Model: u32p(0x24), SoftwareVer: u32p(7000), Phase: u32p(1),
		Bound: boolp(true), RptOff: boolp(false),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Update only Bound.
	if err := s.UpsertInverterInfo(ctx, InverterInfoUpdate{
		UID: "u3", TsMs: 600, ShortAddr: 0x1111,
		Bound: boolp(false),
	}); err != nil {
		t.Fatalf("partial: %v", err)
	}

	var modelCode, sw, phase, bound, rpt sql.NullInt64
	if err := s.DB().QueryRow(`SELECT model_code, software_version, phase, zigbee_bound, turned_off_rpt FROM inverters WHERE uid='u3'`).
		Scan(&modelCode, &sw, &phase, &bound, &rpt); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if modelCode.Int64 != 0x24 || sw.Int64 != 7000 || phase.Int64 != 1 {
		t.Fatalf("non-Bound cols clobbered: model=%d sw=%d phase=%d", modelCode.Int64, sw.Int64, phase.Int64)
	}
	if bound.Int64 != 0 {
		t.Fatalf("Bound: got %d want 0", bound.Int64)
	}
	if rpt.Int64 != 0 || !rpt.Valid {
		t.Fatalf("rpt: %+v want valid=true val=0", rpt)
	}
}

func TestUpsertInverterInfo_ShortAddrUpdates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openTestStore(t)

	if err := s.UpsertInverterInfo(ctx, InverterInfoUpdate{
		UID: "u4", TsMs: 1, ShortAddr: 0x1111,
	}); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := s.UpsertInverterInfo(ctx, InverterInfoUpdate{
		UID: "u4", TsMs: 2, ShortAddr: 0x2222,
	}); err != nil {
		t.Fatalf("second: %v", err)
	}
	var shortAddr int
	var pairedAt, lastSeen int64
	if err := s.DB().QueryRow(`SELECT short_addr, paired_at_ms, last_seen_ms FROM inverters WHERE uid='u4'`).
		Scan(&shortAddr, &pairedAt, &lastSeen); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if shortAddr != 0x2222 {
		t.Fatalf("short_addr: got 0x%X want 0x2222", shortAddr)
	}
	if pairedAt != 1 {
		t.Fatalf("paired_at must be preserved on update: got %d want 1", pairedAt)
	}
	if lastSeen != 2 {
		t.Fatalf("last_seen must update: got %d want 2", lastSeen)
	}
}

func TestPruneEvents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := openTestStore(t)

	rows := []struct {
		tsMs int64
		kind string
	}{
		{1000, "telemetry"},
		{2000, "telemetry"},
		{3000, "telemetry"},
		{1500, "service_start"},
		{2500, "decode_failed"},
	}
	for _, r := range rows {
		if err := s.AppendEvent(ctx, r.tsMs, "", r.kind, "info"); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}

	countKind := func(kind string) int {
		t.Helper()
		var n int
		if err := s.DB().QueryRow(`SELECT COUNT(*) FROM events WHERE kind = ?`, kind).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", kind, err)
		}
		return n
	}

	deleted, err := s.PruneEvents(ctx, 2500, []string{"telemetry"})
	if err != nil {
		t.Fatalf("PruneEvents telemetry: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("scoped delete: got %d want 2", deleted)
	}
	if got := countKind("telemetry"); got != 1 {
		t.Fatalf("telemetry remaining: got %d want 1", got)
	}
	if got := countKind("service_start"); got != 1 {
		t.Fatalf("service_start must be untouched, got %d", got)
	}
	if got := countKind("decode_failed"); got != 1 {
		t.Fatalf("decode_failed must be untouched, got %d", got)
	}

	deleted, err = s.PruneEvents(ctx, 2500, nil)
	if err != nil {
		t.Fatalf("PruneEvents all: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("unscoped delete: got %d want 1", deleted)
	}
	if got := countKind("service_start"); got != 0 {
		t.Fatalf("service_start must be gone, got %d", got)
	}
	if got := countKind("decode_failed"); got != 1 {
		t.Fatalf("decode_failed (ts=2500, not < cutoff) must remain, got %d", got)
	}
	if got := countKind("telemetry"); got != 1 {
		t.Fatalf("telemetry (ts=3000, not < cutoff) must remain, got %d", got)
	}

	if _, err := s.DB().ExecContext(ctx, `DELETE FROM events`); err != nil {
		t.Fatalf("clean: %v", err)
	}
	deleted, err = s.PruneEvents(ctx, 9999, nil)
	if err != nil {
		t.Fatalf("PruneEvents empty: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("empty prune: got %d want 0", deleted)
	}
}

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
	// Insert a row, then close, then re-Open and verify it's still there
	// (proving schema apply on existing DB is a no-op).
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

	// Second upsert with later ts.
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
	if err := s.WriteTelemetryLive(ctx, "uid", 100, 100.0, 230.0, 50.0, 380.0, 60, map[string]int{"v": 1}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := s.WriteTelemetryLive(ctx, "uid", 200, 200.0, 231.0, 50.1, 381.0, 120, map[string]int{"v": 2}); err != nil {
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
	if err := s.DB().QueryRow(`SELECT ts_ms, ac_w FROM telemetry_live WHERE inverter_uid='uid'`).Scan(&ts, &acW); err != nil {
		t.Fatalf("read: %v", err)
	}
	if ts != 200 || acW != 200.0 {
		t.Fatalf("overwrite: ts=%d ac_w=%v, want 200/200", ts, acW)
	}
}

func TestAppendEvent_DefaultsAndNullPayload(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.AppendEvent(ctx, 10, "uidA", "kindA", "", nil); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := s.AppendEvent(ctx, 20, "uidB", "kindB", "warn", map[string]string{"foo": "bar"}); err != nil {
		t.Fatalf("append 2: %v", err)
	}

	type row struct {
		id       int64
		uid      sql.NullString
		kind     string
		severity string
		payload  sql.NullString
	}
	rows, err := s.DB().Query(`SELECT id, inverter_uid, kind, severity, payload_json FROM events ORDER BY id`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.uid, &r.kind, &r.severity, &r.payload); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if len(got) != 2 {
		t.Fatalf("rows: got %d want 2", len(got))
	}
	if got[0].id == got[1].id {
		t.Fatalf("autoincrement: ids equal: %d %d", got[0].id, got[1].id)
	}
	if got[1].id <= got[0].id {
		t.Fatalf("autoincrement: id2=%d not greater than id1=%d", got[1].id, got[0].id)
	}
	// First: empty severity -> "info"; nil payload -> NULL.
	if got[0].severity != "info" {
		t.Fatalf("severity default: got %q want \"info\"", got[0].severity)
	}
	if got[0].payload.Valid {
		t.Fatalf("expected NULL payload, got %q", got[0].payload.String)
	}
	// Second: severity preserved; payload non-NULL.
	if got[1].severity != "warn" {
		t.Fatalf("severity: got %q want \"warn\"", got[1].severity)
	}
	if !got[1].payload.Valid {
		t.Fatal("expected non-NULL payload")
	}
}

func TestAppendEvent_NullableInverterUID(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()
	if err := s.AppendEvent(ctx, 1, "", "backend_hello", "info", nil); err != nil {
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
	if err := s.WriteTelemetryLive(ctx, "u", 1, 0, 0, 0, 0, 0, struct{}{}); err != nil {
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
				if err := s.WriteTelemetryLive(ctx, uid, ts, float64(i), 230, 50, 380, 60, map[string]int{"i": i}); err != nil {
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
		{1500, "backend_hello"},
		{2500, "decode_failed"},
	}
	for _, r := range rows {
		if err := s.AppendEvent(ctx, r.tsMs, "", r.kind, "info", nil); err != nil {
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
	if got := countKind("backend_hello"); got != 1 {
		t.Fatalf("backend_hello must be untouched, got %d", got)
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
	if got := countKind("backend_hello"); got != 0 {
		t.Fatalf("backend_hello must be gone, got %d", got)
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


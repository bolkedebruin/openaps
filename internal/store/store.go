// Package store wraps the inv-driver SQLite state-store. It is the
// only writer in the process; readers in other processes link this
// package as a library (per INV_DRIVER_DESIGN.md §2).
package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"database/sql"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store is a thread-safe SQLite handle. modernc.org/sqlite serialises
// writes internally via the connection pool; WAL mode allows
// concurrent readers in other processes.
type Store struct {
	db *sql.DB
}

// Open opens or creates the SQLite database at path and applies the
// embedded schema. Caller must Close.
func Open(ctx context.Context, path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store.Open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.Open: ping: %w", err)
	}
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.Open: schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the underlying handle.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying *sql.DB for read-only queries from
// sibling packages (dump-* CLI subcommands).
func (s *Store) DB() *sql.DB { return s.db }

// UpsertInverterFromTelemetry promotes a never-before-seen inverter
// to the inventory and bumps last_seen on every subsequent telemetry
// frame. Pairing is currently implicit (first telemetry from a UID
// creates the row); a future PairEvent will set this earlier with
// proper provenance.
func (s *Store) UpsertInverterFromTelemetry(ctx context.Context, uid string, shortAddr uint16, family, model string, tsMs int64) error {
	const q = `
INSERT INTO inverters (uid, short_addr, family, model, paired_at_ms, last_seen_ms)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(uid) DO UPDATE SET
    short_addr   = excluded.short_addr,
    family       = COALESCE(excluded.family, inverters.family),
    model        = COALESCE(excluded.model,  inverters.model),
    last_seen_ms = excluded.last_seen_ms
`
	_, err := s.db.ExecContext(ctx, q, uid, int(shortAddr), nullable(family), nullable(model), tsMs, tsMs)
	if err != nil {
		return fmt.Errorf("UpsertInverter: %w", err)
	}
	return nil
}

// WriteTelemetryLive overwrites the in-place "latest sample" row for
// this inverter. payload is marshalled as JSON.
func (s *Store) WriteTelemetryLive(ctx context.Context, uid string, tsMs int64, acW, acV, acFreq, busV float64, reportSec uint32, payload any) error {
	pj, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("WriteTelemetryLive: marshal: %w", err)
	}
	const q = `
INSERT INTO telemetry_live (inverter_uid, ts_ms, ac_w, ac_v, ac_freq, bus_v, report_sec, payload_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(inverter_uid) DO UPDATE SET
    ts_ms        = excluded.ts_ms,
    ac_w         = excluded.ac_w,
    ac_v         = excluded.ac_v,
    ac_freq      = excluded.ac_freq,
    bus_v        = excluded.bus_v,
    report_sec   = excluded.report_sec,
    payload_json = excluded.payload_json
`
	_, err = s.db.ExecContext(ctx, q, uid, tsMs,
		nullableFloat(acW), nullableFloat(acV), nullableFloat(acFreq), nullableFloat(busV),
		nullableUint32(reportSec), string(pj))
	if err != nil {
		return fmt.Errorf("WriteTelemetryLive: %w", err)
	}
	return nil
}

// AppendEvent inserts an append-only event row. payload may be nil.
func (s *Store) AppendEvent(ctx context.Context, tsMs int64, inverterUID, kind, severity string, payload any) error {
	var pj sql.NullString
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("AppendEvent: marshal: %w", err)
		}
		pj = sql.NullString{String: string(b), Valid: true}
	}
	if severity == "" {
		severity = "info"
	}
	const q = `
INSERT INTO events (ts_ms, inverter_uid, kind, severity, payload_json)
VALUES (?, ?, ?, ?, ?)
`
	_, err := s.db.ExecContext(ctx, q, tsMs, nullable(inverterUID), kind, severity, pj)
	if err != nil {
		return fmt.Errorf("AppendEvent: %w", err)
	}
	return nil
}

// PruneEvents deletes rows from events with ts_ms < cutoffMs. When
// kinds is non-empty the delete is scoped to those kinds; otherwise
// every kind is eligible. Returns the number of rows deleted.
//
// SQLite's WAL auto-checkpoint keeps the -wal file bounded after the
// delete; we deliberately do not VACUUM (it would briefly lock the
// DB and the goal is bounded growth, not minimum file size — freed
// pages get reused by subsequent inserts).
func (s *Store) PruneEvents(ctx context.Context, cutoffMs int64, kinds []string) (int64, error) {
	if len(kinds) == 0 {
		const q = `DELETE FROM events WHERE ts_ms < ?`
		res, err := s.db.ExecContext(ctx, q, cutoffMs)
		if err != nil {
			return 0, fmt.Errorf("PruneEvents: %w", err)
		}
		return res.RowsAffected()
	}
	// Build `IN (?, ?, ...)` placeholders. Kinds are caller-supplied
	// from code, not user input — but parameterise anyway for hygiene.
	ph := make([]byte, 0, 2*len(kinds)-1)
	args := make([]any, 0, len(kinds)+1)
	args = append(args, cutoffMs)
	for i, k := range kinds {
		if i > 0 {
			ph = append(ph, ',')
		}
		ph = append(ph, '?')
		args = append(args, k)
	}
	q := "DELETE FROM events WHERE ts_ms < ? AND kind IN (" + string(ph) + ")"
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("PruneEvents: %w", err)
	}
	return res.RowsAffected()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableFloat treats 0.0 as SQL NULL. This is a v0 expedient — it
// conflates "no reading" with "reading was exactly zero", which is
// fine for the current ac_w / ac_v / ac_freq / bus_v columns (real
// inverters don't produce true zeros for V/Hz, and zero W is rare
// enough we accept the conflation). TODO: switch to *float64 (or a
// nullable wrapper) when telemetry_history lands and zero is a
// meaningful value.
func nullableFloat(f float64) any {
	if f == 0 {
		return nil
	}
	return f
}

func nullableUint32(u uint32) any {
	if u == 0 {
		return nil
	}
	return int64(u)
}

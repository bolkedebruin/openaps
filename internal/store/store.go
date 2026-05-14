// Package store wraps the inv-driver SQLite state-store. It is the
// only writer in the process; readers in other processes link this
// package as a library.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

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

// PanelRow is one MPPT channel sample for WritePanels.
type PanelRow struct {
	ChannelIdx int
	DCV        float64
	DCI        float64
	W          float64
}

// EnergyRow is one channel's lifetime energy accumulator.
type EnergyRow struct {
	ChannelIdx int
	Raw        uint64
	Scale      float64
}

// UpsertInverterFromTelemetry promotes a never-before-seen inverter
// to the inventory and bumps last_seen on every subsequent telemetry
// frame.
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
// this inverter.
func (s *Store) WriteTelemetryLive(ctx context.Context, uid string, tsMs int64, cmd uint32, acW, acV, acFreq, busV float64, reportSec uint32) error {
	const q = `
INSERT INTO telemetry_live (inverter_uid, ts_ms, cmd, ac_w, ac_v, ac_freq, bus_v, report_sec)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(inverter_uid) DO UPDATE SET
    ts_ms      = excluded.ts_ms,
    cmd        = excluded.cmd,
    ac_w       = excluded.ac_w,
    ac_v       = excluded.ac_v,
    ac_freq    = excluded.ac_freq,
    bus_v      = excluded.bus_v,
    report_sec = excluded.report_sec
`
	_, err := s.db.ExecContext(ctx, q, uid, tsMs, int64(cmd),
		nullableFloat(acW), nullableFloat(acV), nullableFloat(acFreq), nullableFloat(busV),
		nullableUint32(reportSec))
	if err != nil {
		return fmt.Errorf("WriteTelemetryLive: %w", err)
	}
	return nil
}

// WritePanels upserts the per-channel MPPT rows for an inverter.
// Existing rows on (inverter_uid, channel_idx) are overwritten.
func (s *Store) WritePanels(ctx context.Context, uid string, tsMs int64, panels []PanelRow) error {
	if len(panels) == 0 {
		return nil
	}
	const q = `
INSERT INTO inverter_panels (inverter_uid, channel_idx, dc_v, dc_i, w, last_seen_ms)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(inverter_uid, channel_idx) DO UPDATE SET
    dc_v          = excluded.dc_v,
    dc_i          = excluded.dc_i,
    w             = excluded.w,
    last_seen_ms  = excluded.last_seen_ms
`
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("WritePanels: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return fmt.Errorf("WritePanels: prepare: %w", err)
	}
	defer stmt.Close()
	for _, p := range panels {
		if _, err := stmt.ExecContext(ctx, uid, p.ChannelIdx,
			nullableFloat(p.DCV), nullableFloat(p.DCI), nullableFloat(p.W), tsMs); err != nil {
			return fmt.Errorf("WritePanels: exec ch=%d: %w", p.ChannelIdx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("WritePanels: commit: %w", err)
	}
	return nil
}

// WriteEnergyLifetime upserts per-channel lifetime energy counters.
func (s *Store) WriteEnergyLifetime(ctx context.Context, uid string, tsMs int64, perChannel []EnergyRow) error {
	if len(perChannel) == 0 {
		return nil
	}
	const q = `
INSERT INTO energy_lifetime (inverter_uid, channel_idx, raw, scale, last_update_ms)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(inverter_uid, channel_idx) DO UPDATE SET
    raw            = excluded.raw,
    scale          = excluded.scale,
    last_update_ms = excluded.last_update_ms
`
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("WriteEnergyLifetime: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return fmt.Errorf("WriteEnergyLifetime: prepare: %w", err)
	}
	defer stmt.Close()
	for _, e := range perChannel {
		if _, err := stmt.ExecContext(ctx, uid, e.ChannelIdx, int64(e.Raw), e.Scale, tsMs); err != nil {
			return fmt.Errorf("WriteEnergyLifetime: exec ch=%d: %w", e.ChannelIdx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("WriteEnergyLifetime: commit: %w", err)
	}
	return nil
}

// AppendEvent inserts an append-only event row with the typed columns.
// short_addr / error / raw_hex stay NULL for non-decode_failed kinds —
// use AppendDecodeFailed for those.
func (s *Store) AppendEvent(ctx context.Context, tsMs int64, inverterUID, kind, severity string) error {
	if severity == "" {
		severity = "info"
	}
	const q = `
INSERT INTO events (ts_ms, inverter_uid, kind, severity)
VALUES (?, ?, ?, ?)
`
	_, err := s.db.ExecContext(ctx, q, tsMs, nullable(inverterUID), kind, severity)
	if err != nil {
		return fmt.Errorf("AppendEvent: %w", err)
	}
	return nil
}

// AppendDecodeFailed inserts a decode_failed event row with the
// short_addr/error/raw_hex columns populated.
func (s *Store) AppendDecodeFailed(ctx context.Context, tsMs int64, shortAddr uint32, errMsg, rawHex string) error {
	const q = `
INSERT INTO events (ts_ms, inverter_uid, kind, severity, short_addr, error, raw_hex)
VALUES (?, NULL, 'decode_failed', 'warn', ?, ?, ?)
`
	_, err := s.db.ExecContext(ctx, q, tsMs, nullableUint32(shortAddr), nullable(errMsg), nullable(rawHex))
	if err != nil {
		return fmt.Errorf("AppendDecodeFailed: %w", err)
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

// nullableFloat treats 0.0 as SQL NULL. A v0 expedient — it conflates
// "no reading" with "reading was exactly zero". Real inverters don't
// emit true zeros for V/Hz, and zero W is rare enough that the
// conflation is acceptable.
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

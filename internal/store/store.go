// Package store wraps the inv-driver SQLite state-store. It is the
// only writer in the process; readers in other processes link this
// package as a library.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	"github.com/bolke/inv-driver/codec"
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

// energyAnomalySafetyFactor multiplies the physical max-energy-per-
// elapsed-window to set the anomaly threshold. Safety covers transient
// inverter peaks and scheduling jitter; at 5× we ignore healthy frames
// at twice their nameplate average and flag genuine spikes (>5× max).
const energyAnomalySafetyFactor = 5.0

// minElapsedSecForCheck guards against zero / tiny deltas-t. Below
// this, the cap would collapse and false-flag everything; we skip the
// log line. Real poll windows are 17–60+ s.
const minElapsedSecForCheck = 5.0

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

// InverterInfoUpdate carries a partial update from an InverterInfo
// envelope. Pointer-typed fields are optional: nil leaves the existing
// column value untouched.
type InverterInfoUpdate struct {
	UID         string
	TsMs        int64
	ShortAddr   uint16
	Model       *uint32
	SoftwareVer *uint32
	Phase       *uint32
	Bound       *bool
	RptOff      *bool
}

// UpsertInverterInfo upserts the inverters row keyed by UID with the
// optional identity / pair-state columns. On INSERT, paired_at_ms is
// set to TsMs; on UPDATE it is preserved. last_seen_ms and short_addr
// always take the supplied value (matching UpsertInverterFromTelemetry
// semantics). Nil pointer fields fall back to the existing column via
// COALESCE so a partial update never clobbers a known value with NULL.
func (s *Store) UpsertInverterInfo(ctx context.Context, in InverterInfoUpdate) error {
	const q = `
INSERT INTO inverters (
    uid, short_addr, family, model, paired_at_ms, last_seen_ms,
    model_code, software_version, phase, zigbee_bound, turned_off_rpt
)
VALUES (?, ?, NULL, NULL, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(uid) DO UPDATE SET
    short_addr       = excluded.short_addr,
    last_seen_ms     = excluded.last_seen_ms,
    model_code       = COALESCE(?, inverters.model_code),
    software_version = COALESCE(?, inverters.software_version),
    phase            = COALESCE(?, inverters.phase),
    zigbee_bound     = COALESCE(?, inverters.zigbee_bound),
    turned_off_rpt   = COALESCE(?, inverters.turned_off_rpt)
`
	mc := nullableUint32Ptr(in.Model)
	sv := nullableUint32Ptr(in.SoftwareVer)
	ph := nullableUint32Ptr(in.Phase)
	zb := nullableBoolPtr(in.Bound)
	to := nullableBoolPtr(in.RptOff)
	_, err := s.db.ExecContext(ctx, q,
		in.UID, int(in.ShortAddr), in.TsMs, in.TsMs,
		mc, sv, ph, zb, to,
		mc, sv, ph, zb, to,
	)
	if err != nil {
		return fmt.Errorf("UpsertInverterInfo: %w", err)
	}
	return nil
}

// intervalEWMAAlpha weights a fresh inter-frame interval against the
// running average. 0.1 = 10% weight on the new sample → ~10-sample
// rolling smoothing.
const intervalEWMAAlpha = 0.1

// intervalSanityCeilingMs caps which intervals contribute to the
// EWMA. Gaps above this (typically fill_up_data busy-wait windows,
// reboots) skip the EWMA update so the rolling average tracks
// "normal poll cadence" rather than "rare hour-long gaps." last
// is still recorded so operators can spot outliers.
const intervalSanityCeilingMs = 60 * 60 * 1000 // 1 hour

// RecordInterval updates the per-UID inverter_metrics row for one
// new telemetry frame. prevTsMs is the previous frame's timestamp
// (0 means no prior). Returns the just-computed interval (ms).
func (s *Store) RecordInterval(ctx context.Context, uid string, prevTsMs, nowTsMs int64) (int64, error) {
	if prevTsMs == 0 || nowTsMs <= prevTsMs {
		return 0, nil
	}
	interval := nowTsMs - prevTsMs
	// Read existing EWMA + count.
	var prevAvg sql.NullFloat64
	var sampleCount sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT avg_interval_ms, sample_count FROM inverter_metrics WHERE inverter_uid=?`,
		uid).Scan(&prevAvg, &sampleCount)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("RecordInterval: select: %w", err)
	}
	var newAvg float64
	switch {
	case interval > intervalSanityCeilingMs:
		// Don't poison the EWMA with hour-long gaps.
		newAvg = prevAvg.Float64
	case !prevAvg.Valid || sampleCount.Int64 == 0:
		newAvg = float64(interval)
	default:
		newAvg = intervalEWMAAlpha*float64(interval) + (1-intervalEWMAAlpha)*prevAvg.Float64
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO inverter_metrics (inverter_uid, last_interval_ms, avg_interval_ms, sample_count, last_update_ms)
VALUES (?, ?, ?, 1, ?)
ON CONFLICT(inverter_uid) DO UPDATE SET
    last_interval_ms = excluded.last_interval_ms,
    avg_interval_ms  = excluded.avg_interval_ms,
    sample_count     = inverter_metrics.sample_count + 1,
    last_update_ms   = excluded.last_update_ms
`, uid, interval, newAvg, nowTsMs)
	if err != nil {
		return 0, fmt.Errorf("RecordInterval: upsert: %w", err)
	}
	return interval, nil
}

// PrevTelemetryTsMs returns the ts_ms of the most recent telemetry
// frame previously recorded for uid, or 0 if none. Read separately so
// callers can compute the inter-frame interval before WriteTelemetryLive
// overwrites the row.
func (s *Store) PrevTelemetryTsMs(ctx context.Context, uid string) (int64, error) {
	var ts sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT ts_ms FROM telemetry_live WHERE inverter_uid=?`, uid).Scan(&ts)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("PrevTelemetryTsMs: %w", err)
	}
	return ts.Int64, nil
}

// FleetSummary aggregates per-inverter nameplate watts across every
// row in the inverters table. Returns (totalW, count, err); totalW
// excludes inverters whose model_code is unknown to
// codec.NameplateWattsForModel, while count includes them.
func (s *Store) FleetSummary(ctx context.Context) (totalW uint32, count uint32, err error) {
	rows, err := s.db.QueryContext(ctx, `SELECT model_code FROM inverters`)
	if err != nil {
		return 0, 0, fmt.Errorf("FleetSummary: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var mc sql.NullInt64
		if err := rows.Scan(&mc); err != nil {
			return 0, 0, fmt.Errorf("FleetSummary: scan: %w", err)
		}
		count++
		if mc.Valid {
			totalW += codec.NameplateWattsForModel(uint8(mc.Int64))
		}
	}
	return totalW, count, rows.Err()
}

// FleetLifetimeWh returns the cumulative fleet energy in watt-hours,
// summing cumulative_wh across every row in energy_lifetime.
// Monotonically non-decreasing across calls — wraparound handling
// happens in WriteEnergyLifetime, not here.
func (s *Store) FleetLifetimeWh(ctx context.Context) (float64, error) {
	var v sql.NullFloat64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cumulative_wh), 0) FROM energy_lifetime`).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("FleetLifetimeWh: %w", err)
	}
	return v.Float64, nil
}

// PeriodEnergyWh returns the energy in watt-hours since the start of
// the current period (day/month/year), anchored at first observation.
// Inserts a fresh anchor when none exists for the current period_key,
// so the first call within a new period seeds the baseline and
// subsequent calls return the delta.
//
// now is taken in the local time zone; period boundaries are local
// midnight / first-of-month / Jan 1.
func (s *Store) PeriodEnergyWh(ctx context.Context, period string, now time.Time, currentLifetimeWh float64) (float64, error) {
	key, err := periodKey(period, now)
	if err != nil {
		return 0, err
	}
	var anchor float64
	err = s.db.QueryRowContext(ctx,
		`SELECT anchor_wh FROM energy_period_anchor WHERE period=? AND period_key=?`,
		period, key).Scan(&anchor)
	switch {
	case err == sql.ErrNoRows:
		// First observation in this period — seed the anchor at the
		// current cumulative lifetime so this period reports 0 W·h.
		_, ierr := s.db.ExecContext(ctx,
			`INSERT INTO energy_period_anchor (period, period_key, anchor_wh, captured_ms)
			 VALUES (?, ?, ?, ?)`,
			period, key, currentLifetimeWh, now.UnixMilli())
		if ierr != nil {
			return 0, fmt.Errorf("PeriodEnergyWh: insert anchor: %w", ierr)
		}
		return 0, nil
	case err != nil:
		return 0, fmt.Errorf("PeriodEnergyWh: %w", err)
	}
	delta := currentLifetimeWh - anchor
	if delta < 0 {
		// Anchor in the future relative to current lifetime — only
		// happens if the cumulative number ever decreases (panel
		// replaced, DB wipe, etc.). Re-anchor and report 0.
		_, _ = s.db.ExecContext(ctx,
			`UPDATE energy_period_anchor SET anchor_wh=?, captured_ms=?
			 WHERE period=? AND period_key=?`,
			currentLifetimeWh, now.UnixMilli(), period, key)
		return 0, nil
	}
	return delta, nil
}

// periodKey returns the canonical key string for a (period, time) pair.
func periodKey(period string, t time.Time) (string, error) {
	switch period {
	case "day":
		return t.Format("2006-01-02"), nil
	case "month":
		return t.Format("2006-01"), nil
	case "year":
		return t.Format("2006"), nil
	}
	return "", fmt.Errorf("unknown period %q", period)
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

// WriteEnergyLifetime applies the per-poll raw counters to the
// per-channel cumulative_wh via delta accumulation.
//
// The on-wire raw counter is a wrap-prone short-window value — main.exe
// also tracks deltas, not absolute raw, into its own ltpower table. We
// mirror that behaviour: each call computes (new_raw - prev_raw) *
// scale * 1000 Wh and adds it to cumulative_wh. If the counter went
// down (wrap, reboot, transient mis-frame), we re-anchor at the new
// raw without adding anything — under-counting a small unknown window
// is better than the alternative of adding the full new_raw (which
// would double-count historical energy already represented in
// prev_raw). First observation on a fresh row also contributes 0.
//
// maxChannelW is the rated AC watts for a single channel of this
// inverter family. Used to derive an anomaly-log threshold scaled to
// the actual time-between-frames for each channel (fleets with many
// inverters poll each one less often, so the ceiling per delta grows
// proportionally). Pass 0 to disable the anomaly check.
func (s *Store) WriteEnergyLifetime(ctx context.Context, uid string, tsMs int64, perChannel []EnergyRow, maxChannelW uint32) error {
	if len(perChannel) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("WriteEnergyLifetime: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	selStmt, err := tx.PrepareContext(ctx,
		`SELECT prev_raw, cumulative_wh, last_update_ms FROM energy_lifetime WHERE inverter_uid=? AND channel_idx=?`)
	if err != nil {
		return fmt.Errorf("WriteEnergyLifetime: prep select: %w", err)
	}
	defer selStmt.Close()
	insStmt, err := tx.PrepareContext(ctx, `
INSERT INTO energy_lifetime (inverter_uid, channel_idx, cumulative_wh, prev_raw, scale, last_update_ms)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(inverter_uid, channel_idx) DO UPDATE SET
    cumulative_wh  = excluded.cumulative_wh,
    prev_raw       = excluded.prev_raw,
    scale          = excluded.scale,
    last_update_ms = excluded.last_update_ms
`)
	if err != nil {
		return fmt.Errorf("WriteEnergyLifetime: prep upsert: %w", err)
	}
	defer insStmt.Close()

	for _, e := range perChannel {
		var prevRaw sql.NullInt64
		var prevCum sql.NullFloat64
		var prevTs sql.NullInt64
		err := selStmt.QueryRowContext(ctx, uid, e.ChannelIdx).Scan(&prevRaw, &prevCum, &prevTs)
		hadRow := err == nil
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("WriteEnergyLifetime: select ch=%d: %w", e.ChannelIdx, err)
		}
		newRaw := int64(e.Raw)
		var newCum float64
		switch {
		case !hadRow || newRaw < prevRaw.Int64:
			// First observation, wrap, or counter reset — anchor
			// at the new raw, no delta committed. We lose the
			// unknown energy in the gap; better than over-counting
			// by treating new_raw as pure post-reset energy.
			newCum = prevCum.Float64
		default:
			delta := newRaw - prevRaw.Int64
			deltaWh := float64(delta) * e.Scale * 1000
			if maxChannelW > 0 && prevTs.Valid {
				elapsedSec := float64(tsMs-prevTs.Int64) / 1000
				if elapsedSec >= minElapsedSecForCheck {
					maxDeltaWh := float64(maxChannelW) * elapsedSec / 3600 * energyAnomalySafetyFactor
					if deltaWh > maxDeltaWh {
						fmt.Printf("ANOMALY-LARGE-DELTA uid=%s ch=%d elapsed_s=%.1f prev=%d new=%d delta=%d scale=%g deltaWh=%.3f thresh=%.3f\n",
							uid, e.ChannelIdx, elapsedSec, prevRaw.Int64, newRaw, delta, e.Scale, deltaWh, maxDeltaWh)
					}
				}
			}
			newCum = prevCum.Float64 + deltaWh
		}
		if _, err := insStmt.ExecContext(ctx, uid, e.ChannelIdx, newCum, newRaw, e.Scale, tsMs); err != nil {
			return fmt.Errorf("WriteEnergyLifetime: upsert ch=%d: %w", e.ChannelIdx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("WriteEnergyLifetime: commit: %w", err)
	}
	return nil
}

// InverterPollRef is one inventory entry for poll dispatch; returned by
// ListInvertersForPoll.
type InverterPollRef struct {
	UID       string
	ShortAddr uint32
}

// ListInvertersForPoll returns every inverter row whose short_addr is
// known and non-zero, ordered by uid. Used by the telemetry poller to
// build its round schedule from inv-driver's own inventory, independent
// of any main.exe state. short_addr = 0 (broadcast/coordinator sentinel)
// is excluded to prevent mis-fall-back to the inventory on the dispatch path.
func (s *Store) ListInvertersForPoll(ctx context.Context) ([]InverterPollRef, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT uid, short_addr
FROM   inverters
WHERE  short_addr IS NOT NULL AND short_addr != 0
ORDER  BY uid ASC`)
	if err != nil {
		return nil, fmt.Errorf("ListInvertersForPoll: %w", err)
	}
	defer rows.Close()
	var out []InverterPollRef
	for rows.Next() {
		var uid string
		var sa int64
		if err := rows.Scan(&uid, &sa); err != nil {
			return nil, fmt.Errorf("ListInvertersForPoll: scan: %w", err)
		}
		out = append(out, InverterPollRef{UID: uid, ShortAddr: uint32(sa)})
	}
	return out, rows.Err()
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

// nullableUint32Ptr returns nil when the pointer is nil, otherwise the
// pointee as int64. Unlike nullableUint32, the value 0 is preserved.
func nullableUint32Ptr(p *uint32) any {
	if p == nil {
		return nil
	}
	return int64(*p)
}

// nullableBoolPtr returns nil when the pointer is nil, otherwise 0/1.
func nullableBoolPtr(p *bool) any {
	if p == nil {
		return nil
	}
	if *p {
		return int64(1)
	}
	return int64(0)
}

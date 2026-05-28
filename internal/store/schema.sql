-- inv-driver v0 schema. CREATE IF NOT EXISTS only — bootstrap on a
-- fresh DB. No migration code; a schema break wipes state.db at deploy.
--
-- Event rotation: telemetry-kind rows are pruned aggressively (short
-- retention); all other kinds keep a longer floor. See pruneOnce /
-- pruneLoop in cmd/inv-driver/main.go and PruneEvents in store.go.

PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS inverters (
    uid              TEXT PRIMARY KEY,
    short_addr       INTEGER,
    family           TEXT,
    model            TEXT,
    paired_at_ms     INTEGER NOT NULL,
    last_seen_ms     INTEGER NOT NULL,
    model_code       INTEGER,
    software_version INTEGER,
    -- phase is the per-leg AC assignment (1/2/3 = leg A/B/C). For
    -- single-phase inverters there is only one leg, so it's always 1.
    -- For three-phase inverters the leg is operator-configured and
    -- stays NULL here until set. The family classifier (single vs
    -- three) is NOT stored; consumers derive it from model_code via
    -- codec.PhaseFromModel.
    phase            INTEGER,
    zigbee_bound     INTEGER,
    turned_off_rpt   INTEGER
);

CREATE TABLE IF NOT EXISTS telemetry_live (
    inverter_uid    TEXT PRIMARY KEY REFERENCES inverters(uid),
    ts_ms           INTEGER NOT NULL,
    cmd             INTEGER NOT NULL,
    ac_w            REAL,
    ac_v            REAL,
    ac_freq         REAL,
    bus_v           REAL,
    report_sec      INTEGER
);

CREATE TABLE IF NOT EXISTS inverter_panels (
    inverter_uid    TEXT NOT NULL REFERENCES inverters(uid),
    channel_idx     INTEGER NOT NULL,
    dc_v            REAL,
    dc_i            REAL,
    w               REAL,
    last_seen_ms    INTEGER NOT NULL,
    PRIMARY KEY (inverter_uid, channel_idx)
);

-- energy_lifetime carries inv-driver's own cumulative energy per
-- channel, in watt-hours. The on-wire raw counter is a wrap-prone
-- per-poll window (main.exe also sums deltas, not raw, into its own
-- ltpower table — verified at 15286 kWh while our raw values give
-- 9 Wh total).
--
-- On every telemetry frame:
--   delta_raw    = max(new_raw - prev_raw, 0) if no wrap
--                  new_raw                    if wrap (new_raw < prev_raw)
--   cumulative_wh += delta_raw * scale * 1000
--   prev_raw      = new_raw
CREATE TABLE IF NOT EXISTS energy_lifetime (
    inverter_uid    TEXT NOT NULL REFERENCES inverters(uid),
    channel_idx     INTEGER NOT NULL,
    cumulative_wh   REAL NOT NULL DEFAULT 0,
    prev_raw        INTEGER NOT NULL DEFAULT 0,
    scale           REAL NOT NULL DEFAULT 0,
    last_update_ms  INTEGER NOT NULL,
    PRIMARY KEY (inverter_uid, channel_idx)
);

-- inverter_metrics holds rolling per-UID response-time stats.
-- last_interval_ms is the gap between this telemetry frame and the
-- previous one for the same UID (proxy for the inverter's effective
-- poll cadence). avg_interval_ms is an EWMA-smoothed view; intervals
-- above the sanity ceiling (e.g. >1 h) reset the EWMA so a busy-wait
-- gap doesn't permanently skew the average.
CREATE TABLE IF NOT EXISTS inverter_metrics (
    inverter_uid     TEXT PRIMARY KEY REFERENCES inverters(uid),
    last_interval_ms INTEGER NOT NULL DEFAULT 0,
    avg_interval_ms  REAL NOT NULL DEFAULT 0,
    sample_count     INTEGER NOT NULL DEFAULT 0,
    last_update_ms   INTEGER NOT NULL
);

-- energy_period_anchor records the fleet cumulative lifetime watt-hours
-- at the start of each day/month/year. Today/month/year energy is then
-- derived as (current_lifetime - anchor_wh). One row per period_key per
-- period; rolling over to a new period inserts a fresh anchor at the
-- current cumulative value, so the next period starts at 0 W·h.
CREATE TABLE IF NOT EXISTS energy_period_anchor (
    period      TEXT NOT NULL,    -- 'day' | 'month' | 'year'
    period_key  TEXT NOT NULL,    -- '2026-05-18' | '2026-05' | '2026'
    anchor_wh   REAL NOT NULL,
    captured_ms INTEGER NOT NULL,
    PRIMARY KEY (period, period_key)
);

CREATE TABLE IF NOT EXISTS events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ts_ms           INTEGER NOT NULL,
    inverter_uid    TEXT,
    kind            TEXT NOT NULL,
    severity        TEXT NOT NULL DEFAULT 'info',
    short_addr      INTEGER,
    error           TEXT,
    raw_hex         TEXT,
    by              TEXT            -- originating backend (Hello name); see Open() for ALTER on legacy DBs
);

CREATE INDEX IF NOT EXISTS events_ts_idx ON events(ts_ms);
CREATE INDEX IF NOT EXISTS events_uid_kind_idx ON events(inverter_uid, kind);

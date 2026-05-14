-- inv-driver v0 schema. Three tables of the eventual sixteen from
-- INV_DRIVER_DESIGN.md §3 — just the ones the v0 ingest path needs.
-- Schema migrations come later; for v0 these are CREATE IF NOT EXISTS
-- so a fresh DB and an upgraded DB look the same.

PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS inverters (
    uid             TEXT PRIMARY KEY,           -- 12-char hex peer UID
    short_addr      INTEGER,
    family          TEXT,                       -- 'qs1a' | 'ds3' | ...
    model           TEXT,                       -- codec.Reply.Model
    paired_at_ms    INTEGER NOT NULL,           -- first seen
    last_seen_ms    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS telemetry_live (
    inverter_uid    TEXT PRIMARY KEY REFERENCES inverters(uid),
    ts_ms           INTEGER NOT NULL,
    ac_w            REAL,
    ac_v            REAL,
    ac_freq         REAL,
    bus_v           REAL,
    report_sec      INTEGER,
    -- Stash the full decoded reply as JSON so additions to the
    -- Telemetry schema don't require a migration in v0. Per the
    -- design, telemetry_live will eventually be a typed row set; the
    -- JSON column is a v0 expedient.
    payload_json    TEXT NOT NULL
);

-- Rotation: telemetry-kind events are pruned aggressively (default
-- 24h), other kinds keep a longer floor (default 7d). The daemon runs
-- one prune at startup and then on a configurable ticker; see
-- pruneOnce / pruneLoop in cmd/inv-driver/main.go and PruneEvents in
-- store.go. Tunable via -retain-telemetry / -retain-other /
-- -prune-interval.
CREATE TABLE IF NOT EXISTS events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ts_ms           INTEGER NOT NULL,
    inverter_uid    TEXT,                       -- nullable: not all events bind to an inverter
    kind            TEXT NOT NULL,              -- 'telemetry' | 'decode_failed' | 'backend_hello' | ...
    severity        TEXT NOT NULL DEFAULT 'info', -- 'info' | 'warn' | 'error'
    payload_json    TEXT
);

CREATE INDEX IF NOT EXISTS events_ts_idx ON events(ts_ms);
CREATE INDEX IF NOT EXISTS events_uid_kind_idx ON events(inverter_uid, kind);

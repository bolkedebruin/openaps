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
    uid             TEXT PRIMARY KEY,
    short_addr      INTEGER,
    family          TEXT,
    model           TEXT,
    paired_at_ms    INTEGER NOT NULL,
    last_seen_ms    INTEGER NOT NULL
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

CREATE TABLE IF NOT EXISTS energy_lifetime (
    inverter_uid    TEXT NOT NULL REFERENCES inverters(uid),
    channel_idx     INTEGER NOT NULL,
    raw             INTEGER NOT NULL,
    scale           REAL NOT NULL,
    last_update_ms  INTEGER NOT NULL,
    PRIMARY KEY (inverter_uid, channel_idx)
);

CREATE TABLE IF NOT EXISTS events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ts_ms           INTEGER NOT NULL,
    inverter_uid    TEXT,
    kind            TEXT NOT NULL,
    severity        TEXT NOT NULL DEFAULT 'info',
    short_addr      INTEGER,
    error           TEXT,
    raw_hex         TEXT
);

CREATE INDEX IF NOT EXISTS events_ts_idx ON events(ts_ms);
CREATE INDEX IF NOT EXISTS events_uid_kind_idx ON events(inverter_uid, kind);

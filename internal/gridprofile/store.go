package gridprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ApplySchema creates (if not already present) the four gridprofile tables in
// the given database.  Idempotent: safe to call on every startup.
func ApplySchema(ctx context.Context, db *sql.DB) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS gp_base_profiles (
    id          TEXT PRIMARY KEY,
    json        TEXT NOT NULL,
    version     TEXT,
    imported_ms INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS gp_active_base (
    singleton   INTEGER PRIMARY KEY CHECK(singleton = 0),
    profile_id  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS gp_overlays (
    uid        TEXT PRIMARY KEY,
    id         TEXT NOT NULL,
    json       TEXT NOT NULL,
    updated_ms INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS gp_apply_log (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    ts_ms    INTEGER NOT NULL,
    uid      TEXT NOT NULL,
    aps_code TEXT NOT NULL,
    intended REAL,
    readback REAL,
    result   TEXT NOT NULL
);
`
	_, err := db.ExecContext(ctx, ddl)
	if err != nil {
		return fmt.Errorf("gridprofile.ApplySchema: %w", err)
	}
	return nil
}

// Store wraps a *sql.DB for gridprofile operations.  The caller is
// responsible for calling ApplySchema before using the Store.  Use
// NewStore(db) after calling ApplySchema.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by the given database handle.
// The caller must have already called ApplySchema on the same db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ListProfiles returns a slice of all stored base-profile IDs and their
// deserialized profiles.
func (s *Store) ListProfiles(ctx context.Context) ([]Profile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT json FROM gp_base_profiles ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("ListProfiles: %w", err)
	}
	defer rows.Close()
	var out []Profile
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("ListProfiles: scan: %w", err)
		}
		var p Profile
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return nil, fmt.Errorf("ListProfiles: unmarshal: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProfile returns the profile with the given id, or (zero, sql.ErrNoRows)
// if it does not exist.
func (s *Store) GetProfile(ctx context.Context, id string) (Profile, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT json FROM gp_base_profiles WHERE id = ?`, id).Scan(&raw)
	if err != nil {
		return Profile{}, err
	}
	var p Profile
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return Profile{}, fmt.Errorf("GetProfile %q: unmarshal: %w", id, err)
	}
	return p, nil
}

// UpsertProfile inserts or replaces a profile.  version is stored for
// informational purposes only.
func (s *Store) UpsertProfile(ctx context.Context, p Profile, version string) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("UpsertProfile %q: marshal: %w", p.ID, err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO gp_base_profiles (id, json, version, imported_ms)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    json        = excluded.json,
    version     = excluded.version,
    imported_ms = excluded.imported_ms
`, p.ID, string(raw), nullableString(version), time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("UpsertProfile %q: %w", p.ID, err)
	}
	return nil
}

// SetActiveBase sets the single active base profile ID.  Pass "" to clear.
func (s *Store) SetActiveBase(ctx context.Context, id string) error {
	if id == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM gp_active_base`)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO gp_active_base (singleton, profile_id) VALUES (0, ?)
ON CONFLICT(singleton) DO UPDATE SET profile_id = excluded.profile_id
`, id)
	if err != nil {
		return fmt.Errorf("SetActiveBase %q: %w", id, err)
	}
	return nil
}

// ActiveBase returns the active base profile ID, or ("", sql.ErrNoRows) if
// none is set.
func (s *Store) ActiveBase(ctx context.Context) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT profile_id FROM gp_active_base WHERE singleton = 0`).Scan(&id)
	return id, err
}

// GetOverlay returns the overlay for a given inverter UID, or
// (zero, sql.ErrNoRows) if none exists.
func (s *Store) GetOverlay(ctx context.Context, uid string) (Overlay, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT json FROM gp_overlays WHERE uid = ?`, uid).Scan(&raw)
	if err != nil {
		return Overlay{}, err
	}
	var o Overlay
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		return Overlay{}, fmt.Errorf("GetOverlay %q: unmarshal: %w", uid, err)
	}
	return o, nil
}

// UpsertOverlay inserts or replaces the overlay for an inverter UID.
func (s *Store) UpsertOverlay(ctx context.Context, uid string, o Overlay) error {
	raw, err := json.Marshal(o)
	if err != nil {
		return fmt.Errorf("UpsertOverlay %q: marshal: %w", uid, err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO gp_overlays (uid, id, json, updated_ms)
VALUES (?, ?, ?, ?)
ON CONFLICT(uid) DO UPDATE SET
    id         = excluded.id,
    json       = excluded.json,
    updated_ms = excluded.updated_ms
`, uid, o.ID, string(raw), time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("UpsertOverlay %q: %w", uid, err)
	}
	return nil
}

// ClearOverlay removes the overlay for an inverter UID (no-op if absent).
func (s *Store) ClearOverlay(ctx context.Context, uid string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM gp_overlays WHERE uid = ?`, uid)
	return err
}

// ListOverlays returns one Overlay per overlay id. An overlay applied to
// several inverters is stored once per UID with identical JSON (each row's
// body carries the full uids list), so grouping by id yields the distinct
// named overlays with their complete target lists.
func (s *Store) ListOverlays(ctx context.Context) ([]Overlay, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT json FROM gp_overlays GROUP BY id ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("ListOverlays: %w", err)
	}
	defer rows.Close()
	var out []Overlay
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("ListOverlays: scan: %w", err)
		}
		var o Overlay
		if err := json.Unmarshal([]byte(raw), &o); err != nil {
			return nil, fmt.Errorf("ListOverlays: unmarshal: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// AppendApplyLog records one apply-then-read attempt for a single parameter.
// result is one of: "in_sync", "applied", "unconfirmed", "drift",
// "unsupported", "no_readback", "unknown", "encode_error: ...", "send_error: ...",
// "context_cancelled", or "unsupported_broadcast".
func (s *Store) AppendApplyLog(ctx context.Context, uid, apsCode string, intended, readback float64, result string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO gp_apply_log (ts_ms, uid, aps_code, intended, readback, result)
VALUES (?, ?, ?, ?, ?, ?)
`, time.Now().UnixMilli(), uid, apsCode,
		nullableFloat64(intended), nullableFloat64(readback), result)
	if err != nil {
		return fmt.Errorf("AppendApplyLog: %w", err)
	}
	return nil
}

// LoadProfilesFromDir reads all *.json files in dir, validates each against
// the v1 schema, and upserts them.  Files that fail validation are skipped
// with an error collected into a multi-error return; all valid files are
// still upserted.
func (s *Store) LoadProfilesFromDir(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("LoadProfilesFromDir %q: %w", dir, err)
	}
	var errs []error
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", e.Name(), err))
			continue
		}
		if err := ValidateProfile(raw); err != nil {
			errs = append(errs, fmt.Errorf("validate %s: %w", e.Name(), err))
			continue
		}
		var p Profile
		if err := json.Unmarshal(raw, &p); err != nil {
			errs = append(errs, fmt.Errorf("unmarshal %s: %w", e.Name(), err))
			continue
		}
		if err := s.UpsertProfile(ctx, p, p.Source.Version); err != nil {
			errs = append(errs, fmt.Errorf("upsert %s: %w", e.Name(), err))
		}
	}
	return joinErrors(errs)
}

// LoadOverlaysFromDir reads all *.json files in dir, validates each against
// the v1 schema as an overlay, and upserts them.  Each overlay may cover
// multiple UIDs (the uids field); each UID gets its own row.
func (s *Store) LoadOverlaysFromDir(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("LoadOverlaysFromDir %q: %w", dir, err)
	}
	var errs []error
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", e.Name(), err))
			continue
		}
		if err := ValidateOverlay(raw); err != nil {
			errs = append(errs, fmt.Errorf("validate %s: %w", e.Name(), err))
			continue
		}
		var o Overlay
		if err := json.Unmarshal(raw, &o); err != nil {
			errs = append(errs, fmt.Errorf("unmarshal %s: %w", e.Name(), err))
			continue
		}
		for _, uid := range o.UIDs {
			if err := s.UpsertOverlay(ctx, uid, o); err != nil {
				errs = append(errs, fmt.Errorf("upsert %s uid %s: %w", e.Name(), uid, err))
			}
		}
	}
	return joinErrors(errs)
}

// nullableString returns nil for an empty string so SQLite stores NULL
// rather than an empty text value.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableFloat64 returns a sql.NullFloat64 so that a genuine 0.0 readback
// is stored as the numeric value 0, not as NULL.  Only an explicit sentinel
// (NaN or the zero-value of the struct) would be stored as NULL.
func nullableFloat64(f float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: f, Valid: true}
}

// joinErrors combines multiple errors into a single error, or nil if the
// slice is empty.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	msg := ""
	for i, e := range errs {
		if i > 0 {
			msg += "; "
		}
		msg += e.Error()
	}
	return fmt.Errorf("%s", msg)
}

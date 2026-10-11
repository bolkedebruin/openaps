package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bolkedebruin/openaps/codec"
)

// PowerCapIntent returns the recorded per-panel output cap for uid.
// ok=false when none is recorded.
func (s *Store) PowerCapIntent(ctx context.Context, uid string) (panelW int, ok bool, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT panel_w FROM power_cap_intent WHERE inverter_uid=?`, uid).Scan(&panelW)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("PowerCapIntent %q: %w", uid, err)
	}
	return panelW, true, nil
}

// SetPowerCapIntent records the per-panel output cap uid is meant to hold.
// by names the controller that set it (empty → NULL).
func (s *Store) SetPowerCapIntent(ctx context.Context, uid string, panelW int, tsMs int64, by string) error {
	const q = `
INSERT INTO power_cap_intent (inverter_uid, panel_w, set_ms, by)
VALUES (?, ?, ?, ?)
ON CONFLICT(inverter_uid) DO UPDATE SET
    panel_w = excluded.panel_w,
    set_ms  = excluded.set_ms,
    by      = excluded.by
`
	if _, err := s.db.ExecContext(ctx, q, uid, panelW, tsMs, nullable(by)); err != nil {
		return fmt.Errorf("SetPowerCapIntent %q: %w", uid, err)
	}
	return nil
}

// InverterUIDsInFamily returns the uids of every inverter whose model code
// maps to fam. Inverters with no model code yet are skipped.
func (s *Store) InverterUIDsInFamily(ctx context.Context, fam codec.Family) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT uid, model_code FROM inverters WHERE model_code IS NOT NULL ORDER BY uid`)
	if err != nil {
		return nil, fmt.Errorf("InverterUIDsInFamily: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var uid string
		var code int64
		if err := rows.Scan(&uid, &code); err != nil {
			return nil, fmt.Errorf("InverterUIDsInFamily: scan: %w", err)
		}
		if codec.FamilyOf(uint8(code)) == fam {
			out = append(out, uid)
		}
	}
	return out, rows.Err()
}

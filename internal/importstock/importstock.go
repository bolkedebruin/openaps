// Package importstock seeds inv-driver's inverter inventory from the
// stock APsystems SQLite database.
//
// A brownfield migration disables main.exe (the stock poller) and makes
// inv-driver the sole poller. The telemetry poller only polls inverters
// already present in its inventory, and passive discovery only fires when
// an inverter spontaneously announces (at dawn power-up). A migration done
// mid-day would therefore see a silent wire until the next dawn. Importing
// the stock DB's already-paired list closes that gap: every inverter with
// a valid short address becomes pollable immediately.
package importstock

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/internal/store"

	_ "modernc.org/sqlite"
)

// Target is the subset of the inv-driver store the import writes through.
// *store.Store satisfies it; tests substitute a fake.
type Target interface {
	UpsertInverterInfo(ctx context.Context, in store.InverterInfoUpdate) (inserted bool, err error)
}

// StockRow is one row of the stock `id` table, after scanning.
type StockRow struct {
	Serial       string // id column — the 12-digit inverter serial
	ShortAddr    int64  // short_address — ZigBee short addr (0 = unbound)
	Model        int64  // APsystems numeric model code (e.g. 24 = QS1A)
	SoftwareVer  int64
	BindZigbee   int64 // bind_zigbee_flag
	TurnedOffRpt int64 // turned_off_rpt_flag
}

// Result summarises an import run.
type Result struct {
	Imported      int // rows written (inserted or identity-updated)
	Inserted      int // of Imported, rows that were new
	Updated       int // of Imported, rows that already existed
	Unbound       int // of Imported, rows with short_addr==0 (not yet pollable)
	Skipped       int // rows skipped (empty serial)
	InvalidSerial int // rows skipped (serial not the expected 12-digit form)
	Invalid       int // rows whose short_address was out of the uint16 range; imported as unbound
}

// maxShortAddr is the largest valid ZigBee short address. The stock
// id.short_address column is INTEGER (int64 when scanned); a value outside
// [0, maxShortAddr] is corrupt and must not be narrowed to uint16 (which
// would wrap modulo 65536 and could either point a poll at the wrong wire
// address or silently drop the inverter).
const maxShortAddr = 0xFFFF

// serialLen is the fixed width of an APsystems inverter serial as stored in
// the stock id.id column: 12 decimal digits. A row whose serial is not this
// exact form is corrupt (truncated/garbage) and is rejected rather than
// promoted to a primary key in the inverters table.
const serialLen = 12

// validSerial reports whether s is the expected 12-digit decimal serial.
func validSerial(s string) bool {
	if len(s) != serialLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// OpenStockDB opens the stock APsystems database read-only. Caller closes.
func OpenStockDB(ctx context.Context, path string) (*sql.DB, error) {
	dsn := "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open stock db: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open stock db: ping: %w", err)
	}
	return db, nil
}

// ReadStockRows reads every inverter row from the stock `id` table.
func ReadStockRows(ctx context.Context, db *sql.DB) ([]StockRow, error) {
	const q = `
SELECT id, short_address, model, software_version, bind_zigbee_flag, turned_off_rpt_flag
FROM   id
ORDER  BY id ASC`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("read stock id table: %w", err)
	}
	defer rows.Close()
	var out []StockRow
	for rows.Next() {
		var (
			r  StockRow
			sa sql.NullInt64
			mo sql.NullInt64
			sv sql.NullInt64
			bz sql.NullInt64
			to sql.NullInt64
		)
		if err := rows.Scan(&r.Serial, &sa, &mo, &sv, &bz, &to); err != nil {
			return nil, fmt.Errorf("scan stock row: %w", err)
		}
		r.ShortAddr = sa.Int64
		r.Model = mo.Int64
		r.SoftwareVer = sv.Int64
		r.BindZigbee = bz.Int64
		r.TurnedOffRpt = to.Int64
		out = append(out, r)
	}
	return out, rows.Err()
}

// Import maps each stock row to an inverters-identity row and upserts it
// through the target. When dryRun is true the target is not written and
// the result reflects what would have happened. nowMs stamps paired_at_ms
// / last_seen_ms on freshly inserted rows (inject a fixed clock in tests).
func Import(ctx context.Context, t Target, rows []StockRow, nowMs int64, dryRun bool) (Result, error) {
	var res Result
	for _, r := range rows {
		if r.Serial == "" {
			res.Skipped++
			continue
		}
		if !validSerial(r.Serial) {
			// A serial that is not the expected 12-digit form would become a
			// bogus inverters primary key; reject it instead of importing.
			res.InvalidSerial++
			continue
		}
		in, invalid := rowToImport(r, nowMs)
		if invalid {
			// Out-of-range stock short_address: imported as unbound (not
			// pollable) rather than narrowed to a wrong wire address.
			res.Invalid++
		}
		if in.ShortAddr == 0 {
			res.Unbound++
		}
		if dryRun {
			res.Imported++
			// Without a write we cannot know insert-vs-update; count as
			// imported only. Inserted/Updated stay 0 under dry-run.
			continue
		}
		inserted, err := t.UpsertInverterInfo(ctx, in)
		if err != nil {
			return res, fmt.Errorf("import serial %q: %w", r.Serial, err)
		}
		res.Imported++
		if inserted {
			res.Inserted++
		} else {
			res.Updated++
		}
	}
	return res, nil
}

// rowToImport maps a stock row into the store's identity-update shape,
// deriving family/model/model_code from the codec's model-code mapping.
// An unknown model code still imports — family and the durable model label
// are both left NULL (Model pointer nil) so a later telemetry/codec update
// can fill the real label, while the raw model_code is preserved for that
// later classification.
//
// The stock model column is range-guarded the same way as short_address: a
// value outside [0, 0xFF] (the on-disk INTEGER is wider than the 1-byte model
// code) is not narrowed via uint8() — which would wrap 256 into 0, a real
// model code — but classified as unknown (family/model NULL). The raw value
// is recorded in model_code only when it fits the byte, so a bogus wide value
// never masquerades as a valid code.
//
// The stock short_address is range-checked before narrowing to uint16: a
// value outside [0, maxShortAddr] (corrupt/garbage in an on-disk stock DB)
// is treated as unbound (ShortAddr=0, not pollable) and reported via the
// returned invalid flag, rather than wrapping modulo 65536 into a wrong —
// possibly pollable — wire address.
func rowToImport(r StockRow, nowMs int64) (in store.InverterInfoUpdate, invalid bool) {
	upd := store.InverterInfoUpdate{
		UID:              r.Serial,
		TsMs:             nowMs,
		PreserveLastSeen: true,
	}

	// Model code: only an in-byte-range value classifies and is recorded.
	// Out-of-range (corrupt) leaves family/model NULL and model_code unset.
	if r.Model >= 0 && r.Model <= 0xFF {
		code := uint8(r.Model)
		mc := uint32(code)
		upd.ModelCode = &mc
		if fam := codec.FamilyOf(code).String(); fam != "" {
			upd.Family = &fam
		}
		if codec.IsKnownModel(code) {
			label := codec.ModelLabelForCode(code)
			upd.Model = &label
		}
		// Unknown-but-in-range code: Model stays nil (NULL label), like
		// family, so a later telemetry update can fill the real label.
	}

	sw := uint32(r.SoftwareVer)
	bound := r.BindZigbee != 0
	rptOff := r.TurnedOffRpt != 0
	upd.SoftwareVer = &sw
	upd.Bound = &bound
	upd.RptOff = &rptOff

	if r.ShortAddr < 0 || r.ShortAddr > maxShortAddr {
		invalid = true // leave ShortAddr at 0 (unbound, not pollable)
	} else {
		upd.ShortAddr = uint16(r.ShortAddr)
	}

	return upd, invalid
}

package importstock

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/bolkedebruin/openaps/internal/store"

	_ "modernc.org/sqlite"
)

// fakeTarget records imports and lets tests drive insert-vs-update.
type fakeTarget struct {
	seen     map[string]store.InverterInfoUpdate
	calls    []store.InverterInfoUpdate
	errOnUID string
}

func newFakeTarget() *fakeTarget {
	return &fakeTarget{seen: map[string]store.InverterInfoUpdate{}}
}

func (f *fakeTarget) UpsertInverterInfo(_ context.Context, in store.InverterInfoUpdate) (bool, error) {
	if in.UID == f.errOnUID {
		return false, sql.ErrConnDone
	}
	f.calls = append(f.calls, in)
	_, existed := f.seen[in.UID]
	f.seen[in.UID] = in
	return !existed, nil
}

func TestRowToImportMapping(t *testing.T) {
	const now = int64(1_700_000_000_000)
	cases := []struct {
		name string
		row  StockRow
		// wantFamily/wantModel are pointers so a case can assert NULL (nil)
		// vs a specific label. A nil expectation means the column is left
		// untouched (stored as SQL NULL).
		wantFamily *string
		wantModel  *string
		wantCode   *uint32
		wantBound  bool
		wantRpt    bool
	}{
		{
			name:       "qs1a model 24",
			row:        StockRow{Serial: "806000036962", ShortAddr: 10339, Model: 24, SoftwareVer: 5, BindZigbee: 1, TurnedOffRpt: 0},
			wantFamily: strptr("qs1"),
			wantModel:  strptr("QS1A"),
			wantCode:   u32ptr(24),
			wantBound:  true,
			wantRpt:    false,
		},
		{
			name:       "ds3 model 32",
			row:        StockRow{Serial: "703000012345", ShortAddr: 4242, Model: 32, BindZigbee: 0, TurnedOffRpt: 1},
			wantFamily: strptr("ds3"),
			wantModel:  strptr("DS3"),
			wantCode:   u32ptr(32),
			wantBound:  false,
			wantRpt:    true,
		},
		{
			// Unknown-but-in-range model code: family and the durable model
			// label are both left NULL so a later codec update can fill the
			// real label; only the raw model_code is recorded.
			name:       "unknown model leaves label null",
			row:        StockRow{Serial: "999000099999", ShortAddr: 1, Model: 0x29},
			wantFamily: nil,
			wantModel:  nil,
			wantCode:   u32ptr(0x29),
		},
		{
			// Out-of-range model code (256) must not wrap via uint8() into 0
			// (a real model code); family/model/model_code are all left unset.
			name:       "out-of-range model code unclassified",
			row:        StockRow{Serial: "111000011100", ShortAddr: 2, Model: 256},
			wantFamily: nil,
			wantModel:  nil,
			wantCode:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, invalid := rowToImport(tc.row, now)
			if invalid {
				t.Errorf("invalid = true, want false for in-range short_addr")
			}
			if got.UID != tc.row.Serial {
				t.Errorf("UID = %q, want %q", got.UID, tc.row.Serial)
			}
			if !eqStrPtr(got.Family, tc.wantFamily) {
				t.Errorf("Family = %v, want %v", derefStr(got.Family), derefStr(tc.wantFamily))
			}
			if !eqStrPtr(got.Model, tc.wantModel) {
				t.Errorf("Model = %v, want %v", derefStr(got.Model), derefStr(tc.wantModel))
			}
			if !eqU32Ptr(got.ModelCode, tc.wantCode) {
				t.Errorf("ModelCode = %v, want %v", got.ModelCode, tc.wantCode)
			}
			if got.ShortAddr != uint16(tc.row.ShortAddr) {
				t.Errorf("ShortAddr = %d, want %d", got.ShortAddr, tc.row.ShortAddr)
			}
			if got.Bound == nil || *got.Bound != tc.wantBound {
				t.Errorf("Bound = %v, want %v", got.Bound, tc.wantBound)
			}
			if got.RptOff == nil || *got.RptOff != tc.wantRpt {
				t.Errorf("RptOff = %v, want %v", got.RptOff, tc.wantRpt)
			}
			if !got.PreserveLastSeen {
				t.Errorf("PreserveLastSeen = false, want true (import must not age back live rows)")
			}
			if got.TsMs != now {
				t.Errorf("TsMs = %d, want %d", got.TsMs, now)
			}
		})
	}
}

func strptr(s string) *string { return &s }
func u32ptr(u uint32) *uint32 { return &u }
func derefStr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}
func eqStrPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
func eqU32Ptr(a, b *uint32) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// TestImportRejectsInvalidSerial asserts a serial that is not the expected
// 12-digit decimal form is rejected (counted InvalidSerial, never written),
// while a valid 12-digit serial in the same batch still imports.
func TestImportRejectsInvalidSerial(t *testing.T) {
	ft := newFakeTarget()
	rows := []StockRow{
		{Serial: "806000036962", ShortAddr: 10339, Model: 24}, // valid
		{Serial: "80600003696", ShortAddr: 1, Model: 24},      // 11 digits
		{Serial: "8060000369620", ShortAddr: 1, Model: 24},    // 13 digits
		{Serial: "80600003696X", ShortAddr: 1, Model: 24},     // non-numeric
		{Serial: "806000036 62", ShortAddr: 1, Model: 24},     // space
	}
	res, err := Import(context.Background(), ft, rows, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 1 || res.InvalidSerial != 4 {
		t.Fatalf("Imported=%d InvalidSerial=%d, want 1/4", res.Imported, res.InvalidSerial)
	}
	if len(ft.calls) != 1 || ft.calls[0].UID != "806000036962" {
		t.Fatalf("target calls = %+v, want only the valid serial", ft.calls)
	}
}

// TestImportInsertedVsUpdatedAgainstStore asserts the transaction-wrapped
// upsert reports inserted-vs-updated correctly across two runs against a real
// store: first run all new, second run all updates.
func TestImportInsertedVsUpdatedAgainstStore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	rows := []StockRow{
		{Serial: "806000036962", ShortAddr: 10339, Model: 24},
		{Serial: "703000012345", ShortAddr: 4242, Model: 32},
	}
	res1, err := Import(ctx, st, rows, 1000, false)
	if err != nil {
		t.Fatal(err)
	}
	if res1.Inserted != 2 || res1.Updated != 0 {
		t.Fatalf("run1 Inserted=%d Updated=%d, want 2/0", res1.Inserted, res1.Updated)
	}
	res2, err := Import(ctx, st, rows, 2000, false)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Inserted != 0 || res2.Updated != 2 {
		t.Fatalf("run2 Inserted=%d Updated=%d, want 0/2", res2.Inserted, res2.Updated)
	}
}

// TestImportUnknownModelLeavesLabelNull asserts an unknown model code persists
// model_code but leaves the model label column NULL so a later codec update can
// fill it.
func TestImportUnknownModelLeavesLabelNull(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if _, err := Import(ctx, st, []StockRow{
		{Serial: "999000099999", ShortAddr: 7, Model: 0x29},
	}, 1000, false); err != nil {
		t.Fatal(err)
	}
	var model sql.NullString
	var modelCode sql.NullInt64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT model, model_code FROM inverters WHERE uid=?`, "999000099999").
		Scan(&model, &modelCode); err != nil {
		t.Fatal(err)
	}
	if model.Valid {
		t.Errorf("model = %q, want NULL for unknown code", model.String)
	}
	if !modelCode.Valid || modelCode.Int64 != 0x29 {
		t.Errorf("model_code = %v, want 0x29 preserved", modelCode)
	}
}

// TestRowToImportRejectsOutOfRangeShortAddr asserts a corrupt stock
// short_address (outside [0,0xFFFF]) is not narrowed into a wrong — and
// possibly pollable — wire address, but imported as unbound (ShortAddr=0)
// and flagged invalid.
func TestRowToImportRejectsOutOfRangeShortAddr(t *testing.T) {
	const now = int64(1_700_000_000_000)
	cases := []struct {
		name string
		sa   int64
	}{
		{"wraps to 1", 65537}, // uint16(65537) == 1 — a pollable wrong address
		{"wraps to 0", 65536}, // uint16(65536) == 0 — silent drop, but still flagged
		{"negative", -1},      // int64 negative also wraps under uint16()
		{"large negative", -65535},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, invalid := rowToImport(StockRow{Serial: "806000036962", ShortAddr: tc.sa, Model: 24}, now)
			if !invalid {
				t.Errorf("invalid = false, want true for short_addr=%d", tc.sa)
			}
			if got.ShortAddr != 0 {
				t.Errorf("ShortAddr = %d, want 0 (out-of-range must import as unbound)", got.ShortAddr)
			}
		})
	}
}

// TestImportCountsInvalid asserts an out-of-range stock short_address is
// counted as Invalid + Unbound and never reaches the poller.
func TestImportCountsInvalid(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	rows := []StockRow{
		{Serial: "806000036962", ShortAddr: 10339, Model: 24},
		{Serial: "703000012345", ShortAddr: 65537, Model: 32}, // corrupt: would wrap to 1
	}
	res, err := Import(ctx, st, rows, 1000, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Invalid != 1 || res.Unbound != 1 {
		t.Fatalf("Invalid=%d Unbound=%d, want 1/1", res.Invalid, res.Unbound)
	}
	refs, err := st.ListInvertersForPoll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].UID != "806000036962" {
		t.Fatalf("ListInvertersForPoll = %+v, want only the in-range inverter", refs)
	}
}

// TestImportPreservesLearnedShortAddr asserts a re-import whose stock row is
// unbound (short_address=0) does not reset a short_addr learned at runtime,
// keeping the inverter pollable. Covers finding 2's regression case.
func TestImportPreservesLearnedShortAddr(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// First import: inverter is bound with a real short address.
	if _, err := Import(ctx, st, []StockRow{
		{Serial: "806000036962", ShortAddr: 10339, Model: 24},
	}, 1000, false); err != nil {
		t.Fatal(err)
	}

	// Re-import against a stale/stock DB where the same inverter is unbound.
	if _, err := Import(ctx, st, []StockRow{
		{Serial: "806000036962", ShortAddr: 0, Model: 24},
	}, 2000, false); err != nil {
		t.Fatal(err)
	}

	var sa int64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT short_addr FROM inverters WHERE uid=?`, "806000036962").Scan(&sa); err != nil {
		t.Fatal(err)
	}
	if sa != 10339 {
		t.Fatalf("short_addr = %d, want 10339 (re-import zeroed a learned address)", sa)
	}
}

func TestImportSkipsEmptySerial(t *testing.T) {
	ft := newFakeTarget()
	rows := []StockRow{
		{Serial: "806000036962", ShortAddr: 10339, Model: 24},
		{Serial: "", ShortAddr: 999, Model: 24}, // skipped
		{Serial: "703000012345", ShortAddr: 4242, Model: 32},
	}
	res, err := Import(context.Background(), ft, rows, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 2 || res.Skipped != 1 {
		t.Fatalf("Imported=%d Skipped=%d, want 2/1", res.Imported, res.Skipped)
	}
	if len(ft.calls) != 2 {
		t.Fatalf("target called %d times, want 2", len(ft.calls))
	}
}

func TestImportCountsUnbound(t *testing.T) {
	ft := newFakeTarget()
	rows := []StockRow{
		{Serial: "806000036962", ShortAddr: 10339, Model: 24},
		{Serial: "703000012345", ShortAddr: 0, Model: 32}, // unbound
	}
	res, err := Import(context.Background(), ft, rows, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 2 || res.Unbound != 1 {
		t.Fatalf("Imported=%d Unbound=%d, want 2/1", res.Imported, res.Unbound)
	}
}

func TestImportDryRunDoesNotWrite(t *testing.T) {
	ft := newFakeTarget()
	rows := []StockRow{
		{Serial: "806000036962", ShortAddr: 10339, Model: 24},
		{Serial: "", Model: 24},
	}
	res, err := Import(context.Background(), ft, rows, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ft.calls) != 0 {
		t.Fatalf("dry-run wrote %d rows, want 0", len(ft.calls))
	}
	if res.Imported != 1 || res.Skipped != 1 {
		t.Fatalf("Imported=%d Skipped=%d, want 1/1", res.Imported, res.Skipped)
	}
}

func TestImportIdempotentAgainstStore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	rows := []StockRow{
		{Serial: "806000036962", ShortAddr: 10339, Model: 24, SoftwareVer: 5, BindZigbee: 1},
		{Serial: "703000012345", ShortAddr: 4242, Model: 32},
	}

	// First run: both new.
	res1, err := Import(ctx, st, rows, 1000, false)
	if err != nil {
		t.Fatal(err)
	}
	if res1.Inserted != 2 || res1.Updated != 0 {
		t.Fatalf("run1 Inserted=%d Updated=%d, want 2/0", res1.Inserted, res1.Updated)
	}

	// Simulate live telemetry advancing last_seen for one inverter.
	const liveSeen = int64(9_999_999)
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE inverters SET last_seen_ms=? WHERE uid=?`, liveSeen, "806000036962"); err != nil {
		t.Fatal(err)
	}

	// Second run: both already present → all updates, no inserts.
	res2, err := Import(ctx, st, rows, 2000, false)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Inserted != 0 || res2.Updated != 2 {
		t.Fatalf("run2 Inserted=%d Updated=%d, want 0/2", res2.Inserted, res2.Updated)
	}

	// last_seen must NOT be clobbered back to nowMs for the live row.
	var seen int64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT last_seen_ms FROM inverters WHERE uid=?`, "806000036962").Scan(&seen); err != nil {
		t.Fatal(err)
	}
	if seen != liveSeen {
		t.Fatalf("last_seen_ms = %d, want %d (import clobbered live telemetry)", seen, liveSeen)
	}

	// The poller view sees both bound inverters.
	refs, err := st.ListInvertersForPoll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("ListInvertersForPoll = %d rows, want 2", len(refs))
	}
}

func TestImportUnboundNotPolled(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	rows := []StockRow{
		{Serial: "806000036962", ShortAddr: 10339, Model: 24},
		{Serial: "703000012345", ShortAddr: 0, Model: 32}, // unbound — imported but not pollable
	}
	if _, err := Import(ctx, st, rows, 1000, false); err != nil {
		t.Fatal(err)
	}
	refs, err := st.ListInvertersForPoll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].UID != "806000036962" {
		t.Fatalf("ListInvertersForPoll = %+v, want only the bound inverter", refs)
	}
}

// TestReadStockRows exercises the SELECT against a temp stock DB shaped
// like the real `id` table.
func TestReadStockRows(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "database.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `
CREATE TABLE id(item INTEGER, id VARCHAR(16), short_address INTEGER, model INTEGER,
  software_version INTEGER, bind_zigbee_flag INTEGER, turned_off_rpt_flag INTEGER,
  flag INTEGER, modbus_addr INTEGER, primary key(id));`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO id(item,id,short_address,model,software_version,bind_zigbee_flag,turned_off_rpt_flag,flag,modbus_addr)
		 VALUES (1,'806000036962',10339,24,5,1,0,0,1),(2,'703000012345',0,32,3,0,1,0,2)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	ro, err := OpenStockDB(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	rows, err := ReadStockRows(ctx, ro)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[1].Serial != "806000036962" || rows[1].ShortAddr != 10339 || rows[1].Model != 24 {
		t.Errorf("row[1] = %+v unexpected (ordered by id ASC)", rows[1])
	}
	if rows[0].Serial != "703000012345" || rows[0].ShortAddr != 0 || rows[0].BindZigbee != 0 {
		t.Errorf("row[0] = %+v unexpected", rows[0])
	}
}

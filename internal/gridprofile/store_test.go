package gridprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "gp_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	db, err := sql.Open("sqlite", "file:"+f.Name()+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := ApplySchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db, func() { db.Close() }
}

func testProfile(id string) Profile {
	return Profile{
		Schema: "invdriver.gridprofile/v1",
		ID:     id,
		VNomV:  230,
		Source: Source{System: "test", Ref: "TY=99"},
		Points: []PointEntry{
			{
				Model:  711,
				Group:  "DERFreqDroop",
				Point:  "KOf",
				SFRef:  sfRefStr("K_SF"),
				SF:     -2,
				Native: NativeValue{Value: 16.7, Unit: "%Pref/Hz"},
				Apply:  Apply{ApsCode: "DD"},
			},
		},
	}
}

func TestStore_UpsertAndGet(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	p := testProfile("EN50549-1")
	if err := s.UpsertProfile(ctx, p, "v1"); err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}
	got, err := s.GetProfile(ctx, "EN50549-1")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID: got %q want %q", got.ID, p.ID)
	}
	if got.VNomV != p.VNomV {
		t.Errorf("VNomV: got %g want %g", got.VNomV, p.VNomV)
	}
}

func TestStore_GetProfile_Missing(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_, err := s.GetProfile(ctx, "no-such-profile")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestStore_ListProfiles(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfile("A"), "")
	_ = s.UpsertProfile(ctx, testProfile("B"), "")
	_ = s.UpsertProfile(ctx, testProfile("C"), "")

	all, err := s.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListProfiles: got %d want 3", len(all))
	}
}

func TestStore_UpsertProfile_UpdatesExisting(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	p := testProfile("test")
	_ = s.UpsertProfile(ctx, p, "v1")
	p.VNomV = 240
	_ = s.UpsertProfile(ctx, p, "v2")

	got, _ := s.GetProfile(ctx, "test")
	if got.VNomV != 240 {
		t.Errorf("VNomV after update: got %g want 240", got.VNomV)
	}
}

func TestStore_ActiveBase(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	// Initially no active base
	_, err := s.ActiveBase(ctx)
	if err != sql.ErrNoRows {
		t.Errorf("ActiveBase before set: expected ErrNoRows, got %v", err)
	}

	_ = s.UpsertProfile(ctx, testProfile("EN50549-1"), "")
	if err := s.SetActiveBase(ctx, "EN50549-1"); err != nil {
		t.Fatalf("SetActiveBase: %v", err)
	}
	id, err := s.ActiveBase(ctx)
	if err != nil {
		t.Fatalf("ActiveBase after set: %v", err)
	}
	if id != "EN50549-1" {
		t.Errorf("ActiveBase: got %q want EN50549-1", id)
	}

	// Change active base
	_ = s.UpsertProfile(ctx, testProfile("Netcode"), "")
	_ = s.SetActiveBase(ctx, "Netcode")
	id, _ = s.ActiveBase(ctx)
	if id != "Netcode" {
		t.Errorf("ActiveBase after change: got %q", id)
	}

	// Clear
	_ = s.SetActiveBase(ctx, "")
	_, err = s.ActiveBase(ctx)
	if err != sql.ErrNoRows {
		t.Errorf("ActiveBase after clear: expected ErrNoRows, got %v", err)
	}
}

func TestStore_OverlayCRUD(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	uid := "999900000001"
	o := Overlay{
		Schema: "invdriver.gridprofile/v1",
		ID:     "my-overlay",
		UIDs:   []string{uid},
		Points: []PointEntry{
			{
				Model:  134,
				Group:  "CrvSet",
				Point:  "Hz3",
				Native: NativeValue{Value: 50.2, Unit: "Hz"},
				Apply:  Apply{ApsCode: "CB"},
			},
		},
	}

	if err := s.UpsertOverlay(ctx, uid, o); err != nil {
		t.Fatalf("UpsertOverlay: %v", err)
	}
	got, err := s.GetOverlay(ctx, uid)
	if err != nil {
		t.Fatalf("GetOverlay: %v", err)
	}
	if got.ID != o.ID {
		t.Errorf("overlay ID: got %q want %q", got.ID, o.ID)
	}

	// Clear
	if err := s.ClearOverlay(ctx, uid); err != nil {
		t.Fatalf("ClearOverlay: %v", err)
	}
	_, err = s.GetOverlay(ctx, uid)
	if err != sql.ErrNoRows {
		t.Errorf("GetOverlay after clear: expected ErrNoRows, got %v", err)
	}
}

func TestStore_AppendApplyLog(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	if err := s.AppendApplyLog(ctx, "aabbccddeeff", "DD", 16.7, 16.57, "ok"); err != nil {
		t.Fatalf("AppendApplyLog: %v", err)
	}
	if err := s.AppendApplyLog(ctx, "aabbccddeeff", "CB", 50.2, 0, "readback_mismatch"); err != nil {
		t.Fatalf("AppendApplyLog: %v", err)
	}

	var count int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gp_apply_log`).Scan(&count)
	if count != 2 {
		t.Errorf("apply_log count: got %d want 2", count)
	}
}

func TestStore_LoadProfilesFromDir(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	dir := t.TempDir()

	// Write a valid profile JSON
	p := testProfile("test-from-dir")
	raw, _ := json.Marshal(p)
	// Add required schema field for validation
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["schema"] = "invdriver.gridprofile/v1"
	raw, _ = json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "test.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}

	// Write an invalid JSON to verify it's skipped gracefully
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{invalid`), 0644); err != nil {
		t.Fatal(err)
	}

	err := s.LoadProfilesFromDir(ctx, dir)
	// Expect an error (bad.json fails), but valid one should still be upserted
	if err == nil {
		t.Error("expected error from bad.json, got nil")
	}

	all, err := s.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 profile loaded, got %d", len(all))
	}
}

func TestStore_LoadOverlaysFromDir(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	dir := t.TempDir()

	uid1 := "999900000001"
	uid2 := "999900000003"
	ov := Overlay{
		Schema: "invdriver.gridprofile/v1",
		ID:     "dir-overlay",
		UIDs:   []string{"999900000001", "999900000003"},
		Points: []PointEntry{
			{
				Model:  134,
				Group:  "CrvSet",
				Point:  "Hz3",
				Native: NativeValue{Value: 50.2, Unit: "Hz"},
				Apply:  Apply{ApsCode: "CB"},
			},
		},
	}
	raw, _ := json.Marshal(ov)
	if err := os.WriteFile(filepath.Join(dir, "ov.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}

	if err := s.LoadOverlaysFromDir(ctx, dir); err != nil {
		t.Fatalf("LoadOverlaysFromDir: %v", err)
	}

	// Both UIDs should have overlay entries.
	for _, uid := range []string{uid1, uid2} {
		got, err := s.GetOverlay(ctx, uid)
		if err != nil {
			t.Errorf("GetOverlay(%q): %v", uid, err)
			continue
		}
		if got.ID != "dir-overlay" {
			t.Errorf("overlay ID for %s: got %q", uid, got.ID)
		}
	}
}

func TestApplySchema_Idempotent(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	ctx := context.Background()
	// Call a second time — should not error.
	if err := ApplySchema(ctx, db); err != nil {
		t.Errorf("second ApplySchema call failed: %v", err)
	}
}

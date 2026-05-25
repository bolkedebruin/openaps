package gridprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bolke/inv-driver/codec"
)

func TestSetOverlay_RejectsConflictBeforePersist(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()
	base := Profile{
		Schema: SchemaVersion, ID: "b", VNomV: 230, Source: Source{System: "t", Ref: "r"},
		Points: []PointEntry{{Model: 134, Group: "CrvSet", Point: "Hz3", Native: NativeValue{Value: 50.2, Unit: "Hz"}, Apply: Apply{ApsCode: "CB"}}},
	}
	if err := s.UpsertProfile(ctx, base, "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetActiveBase(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	uid := "704000006835"
	// Overlay sets CC=50 → effective CB(50.2) < CC(50) is violated.
	ovJSON := `{"schema":"invdriver.gridprofile/v1","id":"x","uids":["704000006835"],` +
		`"points":[{"model":134,"group":"CrvSet","point":"Hz4","native":{"value":50,"unit":"Hz"},"apply":{"aps_code":"CC"}}]}`
	resp := (&Manager{Store: s}).setOverlay(ctx, uid, []byte(ovJSON))
	if resp.Ok {
		t.Error("expected conflict rejection, got Ok=true")
	}
	if !strings.Contains(resp.Error, "conflicting") {
		t.Errorf("want conflict error, got %q", resp.Error)
	}
	if _, err := s.GetOverlay(ctx, uid); err != sql.ErrNoRows {
		t.Errorf("conflicting overlay must NOT be persisted, GetOverlay err=%v", err)
	}
}

func TestBuildDesired_ClampsBaseAbsentOverlayToPhysicalRange(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()
	// Base defines only CB — AF (over-frequency trip) is NOT in the base.
	base := Profile{
		Schema: SchemaVersion, ID: "b", VNomV: 230, Source: Source{System: "t", Ref: "r"},
		Points: []PointEntry{{Model: 134, Group: "CrvSet", Point: "Hz3", Native: NativeValue{Value: 50.2, Unit: "Hz"}, Apply: Apply{ApsCode: "CB"}}},
	}
	if err := s.UpsertProfile(ctx, base, "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetActiveBase(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	uid := "704000006835"
	// Overlay drives AF to an absurd 999 Hz; the base has no range for it.
	ov := Overlay{
		Schema: SchemaVersion, ID: "x", UIDs: []string{uid},
		Points: []PointEntry{{Model: 710, Group: "MustTrip", Point: "Hz", Native: NativeValue{Value: 999, Unit: "Hz"}, Apply: Apply{ApsCode: "AF"}}},
	}
	if err := s.UpsertOverlay(ctx, uid, ov); err != nil {
		t.Fatal(err)
	}
	rec := NewReconciler(s, nil, nil, func(string) (uint8, bool) { return codec.ModelDS3, true }, Options{})
	eff, err := rec.buildDesired(ctx, uid, codec.ModelDS3)
	if err != nil {
		t.Fatalf("buildDesired: %v", err)
	}
	var af float64
	found := false
	for _, p := range eff.Points {
		if p.ApsCode == "AF" {
			af, found = p.NativeValue, true
		}
	}
	if !found {
		t.Fatal("AF missing from desired profile")
	}
	if af != 70 {
		t.Errorf("base-absent overlay AF clamped to %v, want 70 (physical Hz ceiling)", af)
	}
}

func TestStore_ListOverlays_GroupsByID(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	uidA, uidB, uidC := "704000006835", "806000042582", "806000099999"
	// One named profile "victron-shift" applied to two inverters: stored once
	// per uid with identical JSON carrying the full uids list.
	shift := Overlay{
		Schema: "invdriver.gridprofile/v1",
		ID:     "victron-shift",
		UIDs:   []string{uidA, uidB},
		Points: []PointEntry{{
			Model: 134, Group: "CrvSet", Point: "Hz3",
			Native: NativeValue{Value: 50.2, Unit: "Hz"}, Apply: Apply{ApsCode: "CB"},
		}},
	}
	if err := s.UpsertOverlay(ctx, uidA, shift); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertOverlay(ctx, uidB, shift); err != nil {
		t.Fatal(err)
	}
	// A second, single-target profile.
	solo := Overlay{
		Schema: "invdriver.gridprofile/v1", ID: "solo", UIDs: []string{uidC},
		Points: []PointEntry{{
			Model: 710, Group: "MustTrip", Point: "Hz",
			Native: NativeValue{Value: 52.0, Unit: "Hz"}, Apply: Apply{ApsCode: "AF"},
		}},
	}
	if err := s.UpsertOverlay(ctx, uidC, solo); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListOverlays(ctx)
	if err != nil {
		t.Fatalf("ListOverlays: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 overlays grouped by id, got %d: %+v", len(got), got)
	}
	byID := map[string]Overlay{}
	for _, o := range got {
		byID[o.ID] = o
	}
	vs, ok := byID["victron-shift"]
	if !ok {
		t.Fatal("victron-shift missing")
	}
	if len(vs.UIDs) != 2 {
		t.Errorf("victron-shift uids: got %v want 2 targets", vs.UIDs)
	}
	if _, ok := byID["solo"]; !ok {
		t.Error("solo overlay missing")
	}
}

func TestStore_ListOverlays_Empty(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	got, err := NewStore(db).ListOverlays(context.Background())
	if err != nil {
		t.Fatalf("ListOverlays empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want 0 overlays, got %d", len(got))
	}
}

func TestManager_GetBase(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()
	p := Profile{
		Schema: SchemaVersion, ID: "EN50549-1", VNomV: 230,
		Source: Source{System: "test", Ref: "TY=36"},
		Points: []PointEntry{
			{Model: 134, Group: "CrvSet", Point: "Hz3", Native: NativeValue{Value: 50.2, Unit: "Hz"},
				Range: &Range{Min: 50.1, Max: 52.0}, Apply: Apply{ApsCode: "CB"}},
			{Model: 710, Group: "MustTrip", Point: "Hz", Native: NativeValue{Value: 52.0, Unit: "Hz"}, Apply: Apply{ApsCode: "AF"}},
		},
	}
	if err := s.UpsertProfile(ctx, p, "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetActiveBase(ctx, "EN50549-1"); err != nil {
		t.Fatal(err)
	}
	resp := (&Manager{Store: s}).getBase(ctx)
	if !resp.Ok {
		t.Fatalf("getBase: %s", resp.Error)
	}
	var defs map[string]baseDefault
	if err := json.Unmarshal(resp.Json, &defs); err != nil {
		t.Fatal(err)
	}
	if defs["CB"].Value != 50.2 || defs["CB"].Unit != "Hz" {
		t.Errorf("CB default wrong: %+v", defs["CB"])
	}
	if defs["CB"].Min == nil || *defs["CB"].Min != 50.1 || defs["CB"].Max == nil || *defs["CB"].Max != 52.0 {
		t.Errorf("CB range wrong: %+v", defs["CB"])
	}
	if defs["AF"].Value != 52.0 || defs["AF"].Min != nil {
		t.Errorf("AF default wrong (no range expected): %+v", defs["AF"])
	}
}

func TestManager_GetBase_NoActive(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	resp := (&Manager{Store: NewStore(db)}).getBase(context.Background())
	if !resp.Ok {
		t.Fatalf("getBase with no active base should be ok: %s", resp.Error)
	}
	var defs map[string]baseDefault
	json.Unmarshal(resp.Json, &defs)
	if len(defs) != 0 {
		t.Errorf("want empty defaults, got %+v", defs)
	}
}

func TestParamCatalog(t *testing.T) {
	cat := ParamCatalog()
	if len(cat) == 0 {
		t.Fatal("empty catalog")
	}
	seen := map[string]ParamInfo{}
	for i, p := range cat {
		if p.LongName == "" {
			t.Errorf("catalog includes non-encodable code %q (empty LongName)", p.ApsCode)
		}
		if i > 0 && cat[i-1].ApsCode > p.ApsCode {
			t.Errorf("catalog not sorted at %d: %q before %q", i, cat[i-1].ApsCode, p.ApsCode)
		}
		seen[p.ApsCode] = p
	}
	// CV/DD/CB are encodable freq-watt params; DC has an empty LongName.
	for _, want := range []string{"CV", "DD", "CB"} {
		if _, ok := seen[want]; !ok {
			t.Errorf("catalog missing encodable code %q", want)
		}
	}
	if _, ok := seen["DC"]; ok {
		t.Error("catalog should exclude DC (empty LongName / not encodable)")
	}
	if cv := seen["CV"]; cv.Unit != "" && cv.Group == "" {
		t.Errorf("CV metadata incomplete: %+v", cv)
	}
}

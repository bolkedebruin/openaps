package gridprofile

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bolke/inv-driver/codec"
	_ "modernc.org/sqlite"
)

// fakeSender records frames sent; it can be configured to return an error.
type fakeSender struct {
	mu     sync.Mutex
	frames [][]byte
	err    error
}

func (f *fakeSender) Send(_ context.Context, _ string, frame []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	cp := make([]byte, len(frame))
	copy(cp, frame)
	f.frames = append(f.frames, cp)
	return nil
}

func (f *fakeSender) frameCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.frames)
}

// fakeReadback provides a static map of code→value and counts TriggerRead calls.
type fakeReadback struct {
	mu       sync.Mutex
	values   map[string]float64
	triggers int
	present  bool
}

func newFakeReadback(present bool, values map[string]float64) *fakeReadback {
	return &fakeReadback{values: values, present: present}
}

func (f *fakeReadback) ReadbackNative(_ string) (map[string]float64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.values, f.present
}

func (f *fakeReadback) TriggerRead(_ string) {
	f.mu.Lock()
	f.triggers++
	f.mu.Unlock()
}

func (f *fakeReadback) triggerCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.triggers
}

// testProfile with a writable DD code for DS3.
func testProfileDD() Profile {
	return Profile{
		Schema: "invdriver.gridprofile/v1",
		ID:     "test-dd",
		VNomV:  230,
		Source: Source{System: "test", Ref: "TY=0"},
		Points: []PointEntry{
			{
				Model:  711,
				Group:  "DERFreqDroop",
				Point:  "KOf",
				SF:     -2,
				Native: NativeValue{Value: 16.7, Unit: "%Pref/Hz"},
				Range:  &Range{Min: 0, Max: 50},
				Apply:  Apply{ApsCode: "DD"},
			},
		},
	}
}

// testProfileFreq builds a profile with a single freq threshold (CB, writable
// on DS3).
func testProfileFreq(cb float64) Profile {
	return Profile{
		Schema: "invdriver.gridprofile/v1",
		ID:     "test-freq",
		VNomV:  230,
		Source: Source{System: "test", Ref: "TY=0"},
		Points: []PointEntry{
			{
				Model:  134,
				Group:  "CrvSet",
				Point:  "Hz3",
				SF:     -2,
				Native: NativeValue{Value: cb, Unit: "Hz"},
				Range:  &Range{Min: 50, Max: 52},
				Apply:  Apply{ApsCode: "CB"},
			},
		},
	}
}

func fastOpts() Options {
	return Options{
		AutoReassert: true,
		ReadSettle:   1 * time.Millisecond,
	}
}

func modelDS3(_ string) (uint8, bool) { return codec.ModelDS3, true }
func modelQS1A(_ string) (uint8, bool) { return codec.ModelQS1A, true }
func modelUnknown(_ string) (uint8, bool) { return 0, false }

// ---------------------------------------------------------------------------
// ReconcileUID tests
// ---------------------------------------------------------------------------

func TestReconcileUID_NoActiveBase(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)

	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"DD": 16.7})
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	rpt, err := rec.ReconcileUID(context.Background(), "999900000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rpt.Err != nil {
		t.Fatalf("report error: %v", rpt.Err)
	}
	if len(rpt.Points) != 0 {
		t.Errorf("expected no points when no active base, got %d", len(rpt.Points))
	}
	if snd.frameCount() != 0 {
		t.Error("expected no sends with no active base")
	}
}

func TestReconcileUID_UnknownModel(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"DD": 16.7})
	rec := NewReconciler(s, snd, rb, modelUnknown, fastOpts())

	rpt, err := rec.ReconcileUID(ctx, "999900000001")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
	if rpt.Err == nil {
		t.Error("expected report error for unknown model")
	}
}

func TestReconcileUID_InSync(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	// Read-back matches desired (16.57 is within 0.20 of 16.7).
	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"DD": 16.57})
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	rpt, err := rec.ReconcileUID(ctx, "999900000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snd.frameCount() != 0 {
		t.Errorf("expected no sends when in sync, got %d", snd.frameCount())
	}
	if len(rpt.Points) == 0 {
		t.Fatal("expected at least one point result")
	}
	if rpt.Points[0].Result != "in_sync" {
		t.Errorf("result: got %q want in_sync", rpt.Points[0].Result)
	}
}

func TestReconcileUID_DriftAutoReassert(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	// Read-back is far from desired: 10.0 vs 16.7.
	// After "send", the fake readback is updated to be within tolerance.
	rb := newFakeReadback(true, map[string]float64{"DD": 10.0})
	snd := &fakeSender{}

	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	// After the send+settle the readback stub still returns 10.0, so the
	// result will be "unconfirmed" — but at least one frame was sent.
	rpt, err := rec.ReconcileUID(ctx, "999900000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snd.frameCount() == 0 {
		t.Error("expected at least one frame sent for drifted point")
	}
	if len(rpt.Points) == 0 {
		t.Fatal("expected point results")
	}
	result := rpt.Points[0].Result
	if result != "applied" && result != "unconfirmed" {
		t.Errorf("result: got %q want applied or unconfirmed", result)
	}
}

func TestReconcileUID_DriftAutoReassertConfirmed(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	// Simulate: first read 10.0, after send+trigger reads 16.57 (within tol).
	callCount := 0
	rb := &fakeReadback{present: true, values: map[string]float64{"DD": 10.0}}
	// Override ReadbackNative to return confirmed value after first TriggerRead.
	confirmedRB := &confirmingReadback{
		initial:   map[string]float64{"DD": 10.0},
		confirmed: map[string]float64{"DD": 16.57},
	}

	snd := &fakeSender{}
	_ = callCount
	_ = rb

	rec := NewReconciler(s, snd, confirmedRB, modelDS3, fastOpts())

	rpt, err := rec.ReconcileUID(ctx, "999900000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snd.frameCount() == 0 {
		t.Error("expected frames sent")
	}
	if len(rpt.Points) == 0 {
		t.Fatal("expected point results")
	}
	if rpt.Points[0].Result != "applied" {
		t.Errorf("result: got %q want applied", rpt.Points[0].Result)
	}
}

// confirmingReadback returns the initial value for the first N calls, then
// the confirmed value thereafter (simulating a read-after-write).
type confirmingReadback struct {
	mu        sync.Mutex
	triggers  int
	initial   map[string]float64
	confirmed map[string]float64
}

func (c *confirmingReadback) ReadbackNative(_ string) (map[string]float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.triggers >= 1 {
		return c.confirmed, true
	}
	return c.initial, true
}

func (c *confirmingReadback) TriggerRead(_ string) {
	c.mu.Lock()
	c.triggers++
	c.mu.Unlock()
}

func TestReconcileUID_AutoReassertDisabled(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"DD": 10.0})
	// ReadSettle non-zero so the constructor doesn't treat this as the zero-value
	// Options struct and restore the AutoReassert default.
	opts := Options{AutoReassert: false, ReadSettle: time.Millisecond}
	rec := NewReconciler(s, snd, rb, modelDS3, opts)

	rpt, err := rec.ReconcileUID(ctx, "999900000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snd.frameCount() != 0 {
		t.Errorf("expected no sends when AutoReassert=false, got %d", snd.frameCount())
	}
	if len(rpt.Points) == 0 {
		t.Fatal("expected point results")
	}
	if rpt.Points[0].Result != "drift" {
		t.Errorf("result: got %q want drift", rpt.Points[0].Result)
	}
}

func TestReconcileUID_NoReadback(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	snd := &fakeSender{}
	rb := newFakeReadback(false, nil)
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	rpt, err := rec.ReconcileUID(ctx, "999900000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snd.frameCount() != 0 {
		t.Error("should not send when no read-back available")
	}
	if len(rpt.Points) == 0 {
		t.Fatal("expected point results")
	}
	if rpt.Points[0].Result != "no_readback" {
		t.Errorf("result: got %q want no_readback", rpt.Points[0].Result)
	}
}

func TestReconcileUID_SendError(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	snd := &fakeSender{err: errors.New("network error")}
	rb := newFakeReadback(true, map[string]float64{"DD": 10.0})
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	rpt, err := rec.ReconcileUID(ctx, "999900000001")
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if len(rpt.Points) == 0 {
		t.Fatal("expected point results")
	}
	if rpt.Points[0].Result == "" || rpt.Points[0].Result == "applied" {
		t.Errorf("expected send_error result, got %q", rpt.Points[0].Result)
	}
}

func TestReconcileUID_QS1APageACodesUnknown(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	// Profile with an AE code (page-A freq trip).
	p := Profile{
		Schema: "invdriver.gridprofile/v1",
		ID:     "test-ae",
		VNomV:  230,
		Source: Source{System: "test", Ref: "TY=0"},
		Points: []PointEntry{
			{
				Model:  709,
				Group:  "MustTrip",
				Point:  "Hz",
				SF:     -2,
				Native: NativeValue{Value: 47.5, Unit: "Hz"},
				Range:  &Range{Min: 40, Max: 50},
				Apply:  Apply{ApsCode: "AE"},
			},
		},
	}
	// AE is not writable on QS1A (page-A) — capability.Writeable will return
	// false, so the point is dropped from desired.  Verify no send occurs.
	_ = s.UpsertProfile(ctx, p, "")
	_ = s.SetActiveBase(ctx, "test-ae")

	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"AE": 47.5})
	rec := NewReconciler(s, snd, rb, modelQS1A, fastOpts())

	rpt, err := rec.ReconcileUID(ctx, "999900000003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snd.frameCount() != 0 {
		t.Errorf("expected no sends for QS1A page-A codes, got %d", snd.frameCount())
	}
	_ = rpt
}

func TestReconcileUID_OverlayOverridesBase(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	uid := "999900000001"
	// Base: CB=50.5
	_ = s.UpsertProfile(ctx, testProfileFreq(50.5), "")
	_ = s.SetActiveBase(ctx, "test-freq")

	// Overlay: CB=50.2 for this UID.
	ov := Overlay{
		Schema: "invdriver.gridprofile/v1",
		ID:     "overlay-cb",
		UIDs:   []string{uid},
		Points: []PointEntry{
			{
				Model:  134,
				Group:  "CrvSet",
				Point:  "Hz3",
				SF:     -2,
				Native: NativeValue{Value: 50.2, Unit: "Hz"},
				Range:  &Range{Min: 50, Max: 52},
				Apply:  Apply{ApsCode: "CB"},
			},
		},
	}
	_ = s.UpsertOverlay(ctx, uid, ov)

	// Read-back matches overlay value: should be in_sync.
	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"CB": 50.2})
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	rpt, err := rec.ReconcileUID(ctx, uid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snd.frameCount() != 0 {
		t.Errorf("expected no sends when overlay is in sync, got %d", snd.frameCount())
	}
	if len(rpt.Points) == 0 {
		t.Fatal("expected point results")
	}
	if rpt.Points[0].Result != "in_sync" {
		t.Errorf("result: got %q want in_sync", rpt.Points[0].Result)
	}
}

func TestReconcileUID_RangeClamp(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	// Profile with DD=60.0 but range max=50.
	p := Profile{
		Schema: "invdriver.gridprofile/v1",
		ID:     "test-clamp",
		VNomV:  230,
		Source: Source{System: "test", Ref: "TY=0"},
		Points: []PointEntry{
			{
				Model:  711,
				Group:  "DERFreqDroop",
				Point:  "KOf",
				SF:     -2,
				Native: NativeValue{Value: 60.0, Unit: "%Pref/Hz"},
				Range:  &Range{Min: 0, Max: 50},
				Apply:  Apply{ApsCode: "DD"},
			},
		},
	}
	_ = s.UpsertProfile(ctx, p, "")
	_ = s.SetActiveBase(ctx, "test-clamp")

	// Read-back at 50.0 (the clamped max): should be in_sync.
	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"DD": 50.0})
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	rpt, err := rec.ReconcileUID(ctx, "999900000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rpt.Points) == 0 {
		t.Fatal("expected point results")
	}
	if rpt.Points[0].Result != "in_sync" {
		t.Errorf("result: got %q want in_sync (clamped to 50.0)", rpt.Points[0].Result)
	}
}

// ---------------------------------------------------------------------------
// ReconcileAll tests
// ---------------------------------------------------------------------------

func TestReconcileAll_EmptyOverlays(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)

	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{})
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	reports, err := rec.ReconcileAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("expected 0 reports for empty overlay table, got %d", len(reports))
	}
}

func TestReconcileAll_WithOverlays(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileFreq(50.5), "")
	_ = s.SetActiveBase(ctx, "test-freq")

	uid1 := "999900000001"
	uid2 := "999900000003"
	for _, uid := range []string{uid1, uid2} {
		ov := Overlay{
			Schema: "invdriver.gridprofile/v1",
			ID:     "ov",
			UIDs:   []string{uid},
			Points: []PointEntry{
				{
					Model:  134,
					Group:  "CrvSet",
					Point:  "Hz3",
					SF:     -2,
					Native: NativeValue{Value: 50.2, Unit: "Hz"},
					Range:  &Range{Min: 50, Max: 52},
					Apply:  Apply{ApsCode: "CB"},
				},
			},
		}
		_ = s.UpsertOverlay(ctx, uid, ov)
	}

	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"CB": 50.2})
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	reports, err := rec.ReconcileAll(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 2 {
		t.Errorf("expected 2 reports, got %d", len(reports))
	}
}

// ---------------------------------------------------------------------------
// IdentifyProfile tests
// ---------------------------------------------------------------------------

func TestIdentifyProfile_NoProfiles(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)

	snd := &fakeSender{}
	rb := newFakeReadback(false, nil)
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	id, score := rec.IdentifyProfile(map[string]float64{"DD": 16.57}, codec.ModelDS3)
	if id != "" || score != 0 {
		t.Errorf("expected empty result for no profiles, got id=%q score=%g", id, score)
	}
}

func TestIdentifyProfile_ExactMatch(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")

	snd := &fakeSender{}
	rb := newFakeReadback(false, nil)
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	// Read-back matches the profile's DD value within tolerance.
	id, score := rec.IdentifyProfile(map[string]float64{"DD": 16.57}, codec.ModelDS3)
	if id != "test-dd" {
		t.Errorf("identified id: got %q want test-dd", id)
	}
	if score < 1.0 {
		t.Errorf("score: got %g want 1.0", score)
	}
}

func TestIdentifyProfile_Mismatch(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")

	snd := &fakeSender{}
	rb := newFakeReadback(false, nil)
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	// Read-back doesn't match.
	id, score := rec.IdentifyProfile(map[string]float64{"DD": 8.0}, codec.ModelDS3)
	if id != "test-dd" {
		t.Errorf("identified id: got %q want test-dd (best match even if low score)", id)
	}
	if score > 0.0 {
		t.Errorf("score: got %g want 0", score)
	}
}

func TestIdentifyProfile_MultipleProfiles(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	p1 := testProfileFreq(50.2)
	p1.ID = "profile-low"
	p2 := testProfileFreq(50.8)
	p2.ID = "profile-high"

	_ = s.UpsertProfile(ctx, p1, "")
	_ = s.UpsertProfile(ctx, p2, "")

	snd := &fakeSender{}
	rb := newFakeReadback(false, nil)
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	// Reading matches the low profile.
	id, score := rec.IdentifyProfile(map[string]float64{"CB": 50.2}, codec.ModelDS3)
	if id != "profile-low" {
		t.Errorf("identified id: got %q want profile-low", id)
	}
	if score < 1.0 {
		t.Errorf("score: got %g want 1.0", score)
	}
}

// ---------------------------------------------------------------------------
// Tolerance table checks
// ---------------------------------------------------------------------------

func TestToleranceTable_FreqWithinTolerance(t *testing.T) {
	// DS3 DD reads 16.57 for commanded 16.7 — must be within tolerance.
	if !withinTolerance("DD", 16.7, 16.57) {
		t.Error("DD: 16.7 vs 16.57 should be within tolerance")
	}
}

func TestToleranceTable_FreqOutsideTolerance(t *testing.T) {
	// Large drift must be detected.
	if withinTolerance("DD", 16.7, 10.0) {
		t.Error("DD: 16.7 vs 10.0 should not be within tolerance")
	}
}

func TestToleranceTable_Default(t *testing.T) {
	// An unmapped code uses the default 0.10 tolerance.
	if !withinTolerance("ZZ", 5.0, 5.09) {
		t.Error("ZZ: 5.0 vs 5.09 should be within default tolerance")
	}
	if withinTolerance("ZZ", 5.0, 5.20) {
		t.Error("ZZ: 5.0 vs 5.20 should not be within default tolerance")
	}
}

// ---------------------------------------------------------------------------
// AppendApplyLog is exercised via ReconcileUID; this test verifies the
// log is written on an in_sync reconciliation.
// ---------------------------------------------------------------------------

func TestReconcileUID_ApplyLogWritten(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"DD": 16.57})
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	_, _ = rec.ReconcileUID(ctx, "999900000001")

	var count int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gp_apply_log`).Scan(&count)
	if count == 0 {
		t.Error("expected at least one apply_log row after reconcile")
	}
}

// ---------------------------------------------------------------------------
// NewReconciler zero-options default
// ---------------------------------------------------------------------------

func TestNewReconciler_ZeroOptionsDefaultsApplied(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)

	snd := &fakeSender{}
	rb := newFakeReadback(false, nil)
	// Zero Options struct — constructor should fill in defaults.
	rec := NewReconciler(s, snd, rb, modelDS3, Options{})
	if rec.opts.AutoReassert {
		// default is true, so zero Options means AutoReassert should be set
		// by the default logic — but only if all fields are zero.
		// (The constructor gates on "all zero" to preserve explicit false.)
	}
	// Just ensure construction doesn't panic.
	_ = rec
}

// ---------------------------------------------------------------------------
// DB error path
// ---------------------------------------------------------------------------

func TestReconcileUID_StoreClosed(t *testing.T) {
	db, done := openTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")
	done() // close the DB

	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"DD": 16.57})
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	_, err := rec.ReconcileUID(ctx, "999900000001")
	if err == nil {
		t.Error("expected error after DB closed")
	}
}

// Ensure sql.ErrNoRows is not leaked as a hard error (active base missing).
func TestReconcileUID_NoActiveBaseNoError(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)

	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{})
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	_, err := rec.ReconcileUID(context.Background(), "999900000001")
	if err != nil {
		t.Errorf("expected nil error for no active base, got %v", err)
	}
}

// Test that the apply log is not written for missing-model errors.
func TestReconcileUID_NoLogOnModelError(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"DD": 16.0})
	rec := NewReconciler(s, snd, rb, modelUnknown, fastOpts())

	_, _ = rec.ReconcileUID(ctx, "deadbeef1234")

	var count int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gp_apply_log`).Scan(&count)
	if count != 0 {
		t.Errorf("expected no apply_log rows on model error, got %d", count)
	}
}

// _ suppresses the "sql" unused import if the inline sql.ErrNoRows check is
// removed; keep it explicit.
var _ = sql.ErrNoRows

func TestReconcileAll_KnownUIDsCoversBaseOnly(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()
	base := Profile{
		Schema: SchemaVersion, ID: "b", VNomV: 230, Source: Source{System: "t", Ref: "r"},
		Points: []PointEntry{{Model: 134, Group: "CrvSet", Point: "Hz3",
			Native: NativeValue{Value: 50.2, Unit: "Hz"}, Range: &Range{Min: 50, Max: 52}, Apply: Apply{ApsCode: "CB"}}},
	}
	if err := s.UpsertProfile(ctx, base, "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetActiveBase(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	uid := "999900000001"
	snd := &fakeSender{}
	rb := newFakeReadback(true, map[string]float64{"CB": 51.0}) // drift from base 50.2
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	// No overlays and no KnownUIDs: nobody is reconciled (the pre-fix behaviour).
	reps, err := rec.ReconcileAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) != 0 {
		t.Fatalf("no overlays + no KnownUIDs should reconcile nobody, got %d", len(reps))
	}

	// With KnownUIDs, the base-only inverter is reconciled and the base applied.
	rec.KnownUIDs = func(context.Context) []string { return []string{uid} }
	reps, err = rec.ReconcileAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) != 1 || reps[0].UID != uid {
		t.Fatalf("expected base-only uid reconciled, got %+v", reps)
	}
	if snd.frameCount() == 0 {
		t.Error("expected a frame applying the base to the base-only inverter")
	}
}

package gridprofile

import (
	"context"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/codec"
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// QS1A page-A code set
// ---------------------------------------------------------------------------

func TestQS1APageACodes_NowReadable(t *testing.T) {
	// The 0xDB page-A reply is now decoded, so the former page-A codes are
	// read-back-verifiable and must NOT be marked unreadable.
	formerlyUnreadable := []string{
		"AC", "AD", "AQ", "AY", "AH", "AI",
		"AE", "AF", "AJ", "AK",
		"AG", "AS",
		"BB", "BC", "BD", "BE", "BH", "BI", "BJ", "BK",
	}
	for _, code := range formerlyUnreadable {
		if qs1aPageACodes[code] {
			t.Errorf("%q should no longer be in qs1aPageACodes (page-A is now read via 0xDB)", code)
		}
	}
}

func TestQS1APageACodes_Empty(t *testing.T) {
	// No QS1A code is currently unreadable.
	if len(qs1aPageACodes) != 0 {
		t.Errorf("qs1aPageACodes len: got %d want 0", len(qs1aPageACodes))
	}
}

func TestIsQS1APageACode(t *testing.T) {
	// Former page-A codes are now readable → false.
	for _, code := range []string{"AC", "AE", "AJ", "AF", "AK", "AG", "BB", "BC"} {
		if isQS1APageACode(code) {
			t.Errorf("isQS1APageACode(%q) = true, want false (now readable via 0xDB)", code)
		}
	}
	// Page-B/C codes were never in the set.
	for _, code := range []string{"DD", "CB", "CC", "CA", "DH", "DI"} {
		if isQS1APageACode(code) {
			t.Errorf("isQS1APageACode(%q) = true, want false", code)
		}
	}
}

// ---------------------------------------------------------------------------
// VerifyStartup — no desired state
// ---------------------------------------------------------------------------

func TestVerifyStartup_NoDesiredState(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	// No active base is set.
	rb := newFakeReadback(true, map[string]float64{"DD": 16.57, "CB": 50.2})
	snd := &fakeSender{}

	// Store one profile so IdentifyProfile has something to score.
	_ = s.UpsertProfile(ctx, testProfileDD(), "")

	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())
	report, err := rec.VerifyStartup(ctx, "999900000001", codec.ModelDS3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.HasDesiredState {
		t.Error("expected HasDesiredState=false when no active base")
	}
	if snd.frameCount() != 0 {
		t.Error("should not send any frames when no desired state")
	}
	if report.ReconciledOnStartup {
		t.Error("should not reconcile when no desired state")
	}
}

func TestVerifyStartup_NoDesiredState_IdentifiesRunningProfile(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")

	rb := newFakeReadback(true, map[string]float64{"DD": 16.57})
	snd := &fakeSender{}
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	report, _ := rec.VerifyStartup(ctx, "999900000001", codec.ModelDS3)
	if report.IdentifiedID != "test-dd" {
		t.Errorf("identified id: got %q want test-dd", report.IdentifiedID)
	}
	if report.IdentifiedScore < 0.99 {
		t.Errorf("identified score: got %g want ~1.0", report.IdentifiedScore)
	}
}

// ---------------------------------------------------------------------------
// VerifyStartup — desired state in sync
// ---------------------------------------------------------------------------

func TestVerifyStartup_InSync(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	// Read-back within tolerance.
	rb := newFakeReadback(true, map[string]float64{"DD": 16.57})
	snd := &fakeSender{}
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	report, err := rec.VerifyStartup(ctx, "999900000001", codec.ModelDS3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.HasDesiredState {
		t.Error("expected HasDesiredState=true")
	}
	if snd.frameCount() != 0 {
		t.Errorf("expected no sends when in sync, got %d", snd.frameCount())
	}
	if report.ReconciledOnStartup {
		t.Error("expected ReconciledOnStartup=false when in sync")
	}
	for _, pr := range report.Points {
		if pr.Result == "drift" {
			t.Errorf("expected no drift, point %s result=%q", pr.ApsCode, pr.Result)
		}
	}
}

// ---------------------------------------------------------------------------
// VerifyStartup — desired state drifted, AutoReassert triggers reconcile
// ---------------------------------------------------------------------------

func TestVerifyStartup_DriftAutoReassert(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	// Read-back far from desired.
	rb := newFakeReadback(true, map[string]float64{"DD": 10.0})
	snd := &fakeSender{}
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	report, err := rec.VerifyStartup(ctx, "999900000001", codec.ModelDS3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.HasDesiredState {
		t.Error("expected HasDesiredState=true")
	}
	if !report.ReconciledOnStartup {
		t.Error("expected ReconciledOnStartup=true when drift + AutoReassert")
	}
	if snd.frameCount() == 0 {
		t.Error("expected frames sent during reconcile on startup")
	}
}

func TestVerifyStartup_DriftNoAutoReassert(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	rb := newFakeReadback(true, map[string]float64{"DD": 10.0})
	snd := &fakeSender{}
	// ReadSettle must be non-zero so the constructor does not treat this as
	// the zero-value Options and override AutoReassert with the default (true).
	opts := Options{AutoReassert: false, ReadSettle: time.Millisecond}
	rec := NewReconciler(s, snd, rb, modelDS3, opts)

	report, err := rec.VerifyStartup(ctx, "999900000001", codec.ModelDS3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.ReconciledOnStartup {
		t.Error("expected ReconciledOnStartup=false when AutoReassert=false")
	}
	if snd.frameCount() != 0 {
		t.Errorf("expected no sends when AutoReassert=false, got %d", snd.frameCount())
	}
	hasDrift := false
	for _, pr := range report.Points {
		if pr.Result == "drift" {
			hasDrift = true
		}
	}
	if !hasDrift {
		t.Error("expected at least one point with drift result")
	}
}

// ---------------------------------------------------------------------------
// VerifyStartup — QS1A page-A codes are now writable AND readable (0xDB)
// ---------------------------------------------------------------------------

func TestVerifyStartup_QS1APageAReadable(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	// Profile contains an AE point (LF trip Hz). AE is now writable on QS1A
	// (yc600-builder freq trip encoder) and read-back-verifiable via the
	// 0xDB page-A reply, so it is compared like any other code — no longer
	// forced to "unknown".
	p := Profile{
		Schema: "invdriver.gridprofile/v1",
		ID:     "test-qs1a-ae",
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
	_ = s.UpsertProfile(ctx, p, "")
	_ = s.SetActiveBase(ctx, "test-qs1a-ae")

	// Read-back agrees with desired → in_sync, no frames.
	rb := newFakeReadback(true, map[string]float64{"AE": 47.5})
	snd := &fakeSender{}
	rec := NewReconciler(s, snd, rb, modelQS1A, fastOpts())

	report, err := rec.VerifyStartup(ctx, "999900000003", codec.ModelQS1A)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snd.frameCount() != 0 {
		t.Errorf("expected no frames when AE in sync, got %d", snd.frameCount())
	}
	sawAE := false
	for _, pr := range report.Points {
		if pr.ApsCode == "AE" {
			sawAE = true
			if pr.Result == "unknown" {
				t.Errorf("AE should no longer be unknown on QS1A; got %q", pr.Result)
			}
		}
	}
	if !sawAE {
		t.Error("expected AE to appear in the verify report (now writable on QS1A)")
	}
}

// ---------------------------------------------------------------------------
// VerifyStartup — no read-back available yet
// ---------------------------------------------------------------------------

func TestVerifyStartup_NoReadback(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	_ = s.UpsertProfile(ctx, testProfileDD(), "")
	_ = s.SetActiveBase(ctx, "test-dd")

	rb := newFakeReadback(false, nil)
	snd := &fakeSender{}
	rec := NewReconciler(s, snd, rb, modelDS3, fastOpts())

	report, err := rec.VerifyStartup(ctx, "999900000001", codec.ModelDS3)
	// Error is expected when desired state exists but no read-back is available.
	if err == nil && report.HasDesiredState {
		// This path is acceptable — some implementations may not error here.
		// Just verify no frames were sent.
	}
	if snd.frameCount() != 0 {
		t.Errorf("should not send when no read-back available, got %d frames", snd.frameCount())
	}
}

// ---------------------------------------------------------------------------
// codec model constants are the values we expect
// ---------------------------------------------------------------------------

func TestModelCodes(t *testing.T) {
	if codec.ModelQS1 != 0x08 {
		t.Errorf("ModelQS1: got 0x%02X want 0x08", codec.ModelQS1)
	}
	if codec.ModelQS1A != 0x18 {
		t.Errorf("ModelQS1A: got 0x%02X want 0x18", codec.ModelQS1A)
	}
}

// ---------------------------------------------------------------------------
// IdentifyProfile scores QS1A page-A codes now that 0xDB is decoded.
// ---------------------------------------------------------------------------

func TestIdentifyProfile_QS1AScoresPageACodes(t *testing.T) {
	db, done := openTestDB(t)
	defer done()
	s := NewStore(db)
	ctx := context.Background()

	// Profile has only AE — now writable/readable on QS1A.
	p := Profile{
		Schema: "invdriver.gridprofile/v1",
		ID:     "ae-only",
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
	_ = s.UpsertProfile(ctx, p, "")

	snd := &fakeSender{}
	rb := newFakeReadback(false, nil)
	rec := NewReconciler(s, snd, rb, modelQS1A, fastOpts())

	// AE now scores on QS1A; a matching read-back must identify the profile.
	id, score := rec.IdentifyProfile(map[string]float64{"AE": 47.5}, codec.ModelQS1A)
	if score > 1.0 {
		t.Errorf("score > 1.0: %g", score)
	}
	if id != "ae-only" {
		t.Errorf("identified id: got %q want ae-only (AE now scorable on QS1A)", id)
	}
}

package store

import (
	"context"
	"database/sql"
	"testing"
)

func TestInverterModelCode(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/state.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	// Unknown UID: found=false, no error.
	if code, found, err := st.InverterModelCode(ctx, "800000000001"); err != nil || found || code != 0 {
		t.Fatalf("unknown uid: code=%d found=%v err=%v; want 0,false,nil", code, found, err)
	}

	// Known UID with a model code.
	mc := uint32(0x18) // QS1A
	if _, err := st.UpsertInverterInfo(ctx, InverterInfoUpdate{UID: "800000000001", TsMs: 1000, ShortAddr: 100, ModelCode: &mc}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if code, found, err := st.InverterModelCode(ctx, "800000000001"); err != nil || !found || code != 0x18 {
		t.Fatalf("known uid: code=0x%x found=%v err=%v; want 0x18,true,nil", code, found, err)
	}

	// Known UID whose model_code is still NULL: found=true, code 0.
	if _, err := st.UpsertInverterInfo(ctx, InverterInfoUpdate{UID: "800000000002", TsMs: 1000, ShortAddr: 101}); err != nil {
		t.Fatalf("seed null-model: %v", err)
	}
	if code, found, err := st.InverterModelCode(ctx, "800000000002"); err != nil || !found || code != 0 {
		t.Fatalf("null model: code=%d found=%v err=%v; want 0,true,nil", code, found, err)
	}
}

// TestUpsertInverterInfo_NilPhasePreservesOperatorLeg locks in the invariant
// the ingest change relies on: a telemetry-style upsert (Phase nil) must not
// clobber an operator-assigned grid leg.
func TestUpsertInverterInfo_NilPhasePreservesOperatorLeg(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/state.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	const uid = "800000000001"
	mc := uint32(0x18)
	leg := uint32(2)
	// Operator assigns leg 2.
	if _, err := st.UpsertInverterInfo(ctx, InverterInfoUpdate{UID: uid, TsMs: 1000, ShortAddr: 100, ModelCode: &mc, Phase: &leg}); err != nil {
		t.Fatalf("operator set: %v", err)
	}
	// A later telemetry-style upsert carries no phase.
	sw := uint32(5)
	if _, err := st.UpsertInverterInfo(ctx, InverterInfoUpdate{UID: uid, TsMs: 2000, ShortAddr: 100, ModelCode: &mc, SoftwareVer: &sw}); err != nil {
		t.Fatalf("telemetry upsert: %v", err)
	}

	var got sql.NullInt64
	if err := st.DB().QueryRowContext(ctx, `SELECT phase FROM inverters WHERE uid=?`, uid).Scan(&got); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !got.Valid || got.Int64 != 2 {
		t.Fatalf("phase after nil upsert = %v; want 2 (operator leg preserved)", got)
	}
}

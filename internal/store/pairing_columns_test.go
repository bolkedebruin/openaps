package store

import (
	"context"
	"database/sql"
	"testing"
)

func TestPairingColumns_Migration(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/state.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// The three pairing columns must exist (the ALTERs ran on Open).
	for _, col := range []string{"encrypted", "pairing_state", "last_announce_ms"} {
		var dummy sql.NullString
		q := "SELECT " + col + " FROM inverters LIMIT 1"
		if err := st.DB().QueryRowContext(ctx, q).Scan(&dummy); err != nil && err != sql.ErrNoRows {
			t.Fatalf("column %q not queryable: %v", col, err)
		}
	}

	// Re-Open the same DB: the idempotent ALTERs must not error.
	st2, err := Open(ctx, t.TempDir()+"/state2.db")
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	_ = st2.Close()
}

func TestSetInverterEncrypted_AndRead(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/state.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// Seed an inverter via the pairing-state upsert.
	if err := st.SetInverterShortAddr(ctx, "999900000003", 0x1234); err != nil {
		t.Fatalf("SetInverterShortAddr: %v", err)
	}

	// Before setting encrypted, it reads as nil (NULL/unknown).
	row, err := st.GetInverterPairing(ctx, "999900000003")
	if err != nil {
		t.Fatalf("GetInverterPairing: %v", err)
	}
	if row.Encrypted != nil {
		t.Fatalf("encrypted = %v want nil (unknown)", *row.Encrypted)
	}
	if row.ShortAddr != 0x1234 {
		t.Fatalf("short_addr = 0x%X", row.ShortAddr)
	}

	if err := st.SetInverterEncrypted(ctx, "999900000003", true); err != nil {
		t.Fatalf("SetInverterEncrypted: %v", err)
	}
	row, _ = st.GetInverterPairing(ctx, "999900000003")
	if row.Encrypted == nil || !*row.Encrypted {
		t.Fatalf("encrypted not set true: %+v", row)
	}

	if err := st.SetInverterEncrypted(ctx, "999900000003", false); err != nil {
		t.Fatalf("SetInverterEncrypted false: %v", err)
	}
	row, _ = st.GetInverterPairing(ctx, "999900000003")
	if row.Encrypted == nil || *row.Encrypted {
		t.Fatalf("encrypted not set false: %+v", row)
	}
}

func TestSetInverterPairingState_AndDelete(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/state.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if err := st.SetInverterPairingState(ctx, "999900000001", "found", 1234567); err != nil {
		t.Fatalf("SetInverterPairingState: %v", err)
	}
	row, err := st.GetInverterPairing(ctx, "999900000001")
	if err != nil {
		t.Fatalf("GetInverterPairing: %v", err)
	}
	if row.PairingState != "found" || row.LastAnnounceMs != 1234567 {
		t.Fatalf("row = %+v", row)
	}

	// A second update with lastAnnounceMs=0 must preserve the prior value.
	if err := st.SetInverterPairingState(ctx, "999900000001", "bound", 0); err != nil {
		t.Fatalf("update state: %v", err)
	}
	row, _ = st.GetInverterPairing(ctx, "999900000001")
	if row.PairingState != "bound" {
		t.Fatalf("state = %q want bound", row.PairingState)
	}
	if row.LastAnnounceMs != 1234567 {
		t.Fatalf("last_announce_ms = %d want preserved 1234567", row.LastAnnounceMs)
	}

	// DeleteInverter removes the row.
	if err := st.DeleteInverter(ctx, "999900000001"); err != nil {
		t.Fatalf("DeleteInverter: %v", err)
	}
	if _, err := st.GetInverterPairing(ctx, "999900000001"); err != sql.ErrNoRows {
		t.Fatalf("GetInverterPairing after delete = %v want sql.ErrNoRows", err)
	}
}

package store

import (
	"context"
	"database/sql"
	"testing"
)

func TestPairingColumns_Migration(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/state.db"
	st, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// The pairing columns must exist: the `ALTER TABLE` statements ran on
	// Open.
	for _, col := range []string{"encrypted", "pairing_state", "last_announce_ms", "found_channel"} {
		var dummy sql.NullString
		q := "SELECT " + col + " FROM inverters LIMIT 1"
		if err := st.DB().QueryRowContext(ctx, q).Scan(&dummy); err != nil && err != sql.ErrNoRows {
			t.Fatalf("column %q not queryable: %v", col, err)
		}
	}

	// Open the SAME DB again. The `ALTER TABLE` statements run again, and
	// Open must tolerate that. That is the whole point of the
	// duplicate-column guard in Open. A different path would exercise
	// nothing.
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	st2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	_ = st2.Close()
}

func TestSetInverterEncrypted_AndRead(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/state.db"
	st, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

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
	path := t.TempDir() + "/state.db"
	st, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

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

func TestSetInverterFoundChannel(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/state.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// A discovery scan records a serial that no telemetry ever produced.
	// The write must therefore create the row. It must not silently do
	// nothing.
	const fresh = "999900000042"
	if err := st.SetInverterFoundChannel(ctx, fresh, 21); err != nil {
		t.Fatalf("SetInverterFoundChannel: %v", err)
	}
	row, err := st.GetInverterPairing(ctx, fresh)
	if err != nil {
		t.Fatalf("GetInverterPairing: %v", err)
	}
	if row.FoundChannel != 21 {
		t.Fatalf("found_channel = %d, want 21", row.FoundChannel)
	}

	// A later scan on another channel replaces it.
	if err := st.SetInverterFoundChannel(ctx, fresh, 15); err != nil {
		t.Fatalf("SetInverterFoundChannel: %v", err)
	}
	if row, _ = st.GetInverterPairing(ctx, fresh); row.FoundChannel != 15 {
		t.Fatalf("found_channel = %d, want the newer 15", row.FoundChannel)
	}

	// Zero is not a channel. It must not erase a known channel, and it must
	// not create a row for a unit that nobody heard.
	if err := st.SetInverterFoundChannel(ctx, fresh, 0); err != nil {
		t.Fatalf("SetInverterFoundChannel(0): %v", err)
	}
	if row, _ = st.GetInverterPairing(ctx, fresh); row.FoundChannel != 15 {
		t.Fatalf("found_channel = %d after a zero write, want 15 untouched", row.FoundChannel)
	}
	if err := st.SetInverterFoundChannel(ctx, "999900000099", 0); err != nil {
		t.Fatalf("SetInverterFoundChannel(unknown, 0): %v", err)
	}
	if _, err := st.GetInverterPairing(ctx, "999900000099"); err == nil {
		t.Fatal("a zero-channel write created a row for a unit that was never heard")
	}

	// A unit that no scan heard reads as 0, distinct from any real channel.
	if err := st.SetInverterShortAddr(ctx, "999900000003", 0x1234); err != nil {
		t.Fatalf("SetInverterShortAddr: %v", err)
	}
	if row, _ = st.GetInverterPairing(ctx, "999900000003"); row.FoundChannel != 0 {
		t.Fatalf("found_channel = %d for a never-scanned unit, want 0", row.FoundChannel)
	}
}

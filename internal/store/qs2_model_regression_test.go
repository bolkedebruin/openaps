package store

import (
	"context"
	"testing"
)

// A generic DS3-class telemetry label must not overwrite
// a more specific model identity established by model_code.
func TestQS2IdentitySurvivesGenericDS3Telemetry(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	uid := "821000011029"
	model := "QS2"
	family := "ds3"
	code := uint32(0x36)

	// Simulate a correctly identified QS2.
	_, err := s.UpsertInverterInfo(ctx, InverterInfoUpdate{
		UID:       uid,
		TsMs:      1000,
		ShortAddr: 2783,
		Model:     &model,
		Family:    &family,
		ModelCode: &code,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Simulate generic DS3 telemetry arriving later.
	err = s.UpsertInverterFromTelemetry(
		ctx, uid, 2783, "ds3", "DS3", 2000,
	)
	if err != nil {
		t.Fatal(err)
	}

	var gotModel string
	var gotCode int

	err = s.DB().QueryRowContext(ctx,
		"SELECT model, model_code FROM inverters WHERE uid = ?",
		uid,
	).Scan(&gotModel, &gotCode)
	if err != nil {
		t.Fatal(err)
	}

	if gotModel != "QS2" {
		t.Errorf(
			"model overwritten: got %q, want QS2",
			gotModel,
		)
	}

	if gotCode != 0x36 {
		t.Errorf(
			"model_code changed: got 0x%X, want 0x36",
			gotCode,
		)
	}
}

// A previously misidentified DS3 must be corrected when its
// authoritative QS2 model code is received.
func TestQS2ModelCodeCorrectsExistingDS3Name(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	uid := "qs2-existing-ds3"

	if err := s.UpsertInverterFromTelemetry(
		ctx, uid, 2783, "ds3", "DS3", 1000,
	); err != nil {
		t.Fatal(err)
	}

	code := uint32(0x36)
	if _, err := s.UpsertInverterInfo(ctx, InverterInfoUpdate{
		UID:       uid,
		ShortAddr: 2783,
		TsMs:      2000,
		ModelCode: &code,
	}); err != nil {
		t.Fatal(err)
	}

	var model string
	if err := s.DB().QueryRowContext(ctx,
		"SELECT model FROM inverters WHERE uid = ?", uid,
	).Scan(&model); err != nil {
		t.Fatal(err)
	}

	if model != "QS2" {
		t.Errorf("model = %q, want QS2 after receiving model code 0x36", model)
	}
}

// An existing database with a correct model code but stale model
// label must be repaired without waiting for another info reply.
func TestQS2ExistingDatabaseRecoversFromTelemetry(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	uid := "qs2-stale-model"

	model := "QS2"
	code := uint32(0x36)

	if _, err := s.UpsertInverterInfo(ctx, InverterInfoUpdate{
		UID:       uid,
		ShortAddr: 2783,
		TsMs:      1000,
		Model:     &model,
		ModelCode: &code,
	}); err != nil {
		t.Fatal(err)
	}

	// Reproduce the inconsistent database found on the ECU.
	if _, err := s.DB().ExecContext(ctx,
		"UPDATE inverters SET model = 'DS3' WHERE uid = ?", uid,
	); err != nil {
		t.Fatal(err)
	}

	// Ordinary telemetry must not preserve the stale label.
	if err := s.UpsertInverterFromTelemetry(
		ctx, uid, 2783, "ds3", "DS3", 2000,
	); err != nil {
		t.Fatal(err)
	}

	var got string
	if err := s.DB().QueryRowContext(ctx,
		"SELECT model FROM inverters WHERE uid = ?", uid,
	).Scan(&got); err != nil {
		t.Fatal(err)
	}

	if got != "QS2" {
		t.Errorf("model = %q, want QS2", got)
	}
}

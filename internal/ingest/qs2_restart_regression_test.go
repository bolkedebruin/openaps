package ingest

import (
	"context"
	"testing"

	"github.com/bolkedebruin/openaps/internal/store"
)

func TestQS2ModelCodeAvailableAfterRestart(t *testing.T) {
	in := newIngestor(t)
	ctx := context.Background()
	uid := "qs2-restart-test"
	code := uint32(0x36)

	// The database already knows the inverter's real model.
	if _, err := in.S.UpsertInverterInfo(ctx, store.InverterInfoUpdate{
		UID:       uid,
		ShortAddr: 2783,
		TsMs:      1000,
		ModelCode: &code,
	}); err != nil {
		t.Fatal(err)
	}

	// A freshly started ingestor has an empty in-memory cache.
	restarted := &Ingestor{S: in.S}

	if got := restarted.modelFor(uid); got != 0x36 {
		t.Errorf(
			"model after restart = 0x%02X, want 0x36 (QS2)",
			got,
		)
	}
}

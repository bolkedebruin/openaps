package source

import (
	"context"
	"testing"

	"github.com/bolkedebruin/openaps/internal/sunspec/source/invdriver"
)

// The SunSpec SN (Snapshot.ECUID) tracks the inv-driver client's live
// ecu-id: seeded at startup and refreshed when a settings-change broadcast
// updates it — no rebuild of the Builder, no ecu-sunspec restart.
func TestBuilder_EcuIDLiveFromClient(t *testing.T) {
	c := invdriver.New("/nonexistent.sock", "test")
	c.SetEcuID("216200001234")

	b := NewBuilder()
	b.InvDriverClient = c

	s, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if s.ECUID != "216200001234" {
		t.Errorf("ECUID = %q, want 216200001234 (live from client)", s.ECUID)
	}

	// A settings-change broadcast updates the client; the next snapshot picks
	// it up without reconstructing the builder.
	c.SetEcuID("roof-west")
	s2, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if s2.ECUID != "roof-west" {
		t.Errorf("ECUID after live update = %q, want roof-west", s2.ECUID)
	}
}

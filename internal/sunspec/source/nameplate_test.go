package source

import (
	"context"
	"testing"

	"github.com/bolkedebruin/openaps/codec"
)

// Per-inverter NameplateW resolves from codec.NameplateWattsForModel —
// the same corrected datasheet table inv-driver uses for the fleet WRtg —
// so Model 121 WMax can't drift against a separately-maintained file.
func TestNameplateW_CodecIsSourceOfTruth(t *testing.T) {
	for _, mc := range []uint8{codec.ModelQS1A, codec.ModelDS3, codec.ModelDS3H} {
		want := int(codec.NameplateWattsForModel(mc))
		if want == 0 {
			t.Fatalf("codec returned 0 for model 0x%02X — test premise broken", mc)
		}
		if got := (Inverter{Model: int(mc)}).NameplateW(); got != want {
			t.Errorf("NameplateW(model 0x%02X) = %d, want codec value %d", mc, got, want)
		}
	}
}

// A model the codec doesn't catalogue falls through to the TypeCode default,
// then to zero.
func TestNameplateW_FallsBackWhenCodecUnknown(t *testing.T) {
	if got := (Inverter{Model: 0, TypeCode: "04"}).NameplateW(); got != 880 {
		t.Errorf("TypeCode 04 fallback = %d, want 880", got)
	}
	if got := (Inverter{Model: 0, TypeCode: "ZZ"}).NameplateW(); got != 0 {
		t.Errorf("unknown model + TypeCode = %d, want 0", got)
	}
}

// NewBuilder reads no stock /etc/yuneng config: identity is empty until the
// caller sets it from OpenAPS/inv-driver sources, and PollingInterval comes
// from the builder field, not a poll-interval file.
func TestNewBuilder_NoStockConfigReads(t *testing.T) {
	b := NewBuilder()
	if b.ECUID != "" || b.Firmware != "" || b.Model != "" {
		t.Errorf("NewBuilder identity = {%q,%q,%q}, want all empty", b.ECUID, b.Firmware, b.Model)
	}

	b.Firmware = "OpenAPS v9.9.9"
	b.PollingIntervalS = 7
	s, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if s.Firmware != "OpenAPS v9.9.9" {
		t.Errorf("Snapshot.Firmware = %q, want OpenAPS v9.9.9", s.Firmware)
	}
	if s.PollingInterval != 7 {
		t.Errorf("Snapshot.PollingInterval = %d, want 7", s.PollingInterval)
	}
}

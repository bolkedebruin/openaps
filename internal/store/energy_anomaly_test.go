package store

import (
	"context"
	"testing"
)

func TestIsAnomalousDelta(t *testing.T) {
	// QS1A-ish: 1600 W cap, 1 s window -> ceiling 1600*1/3600*5 ≈ 2.22 Wh.
	if isAnomalousDelta(0.5, 1600, 1000) {
		t.Error("0.5 Wh in 1 s should be fine")
	}
	if !isAnomalousDelta(100, 1600, 1000) {
		t.Error("100 Wh in 1 s should be anomalous")
	}
	// Long gap scales the ceiling: 1 h at 1600 W -> 8000 Wh ceiling.
	if isAnomalousDelta(1500, 1600, 3600*1000) {
		t.Error("1500 Wh over 1 h should be fine")
	}
	// maxChannelW 0 disables the check.
	if isAnomalousDelta(1e9, 0, 1000) {
		t.Error("zero cap disables the check")
	}
}

func TestWriteEnergyLifetime_RejectsGarbageDelta(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const scale = 1e-3 // deltaWh = delta * scale * 1000 = delta (1 raw = 1 Wh)
	const maxCh = uint32(1600)
	row := func(raw uint64) []EnergyRow { return []EnergyRow{{ChannelIdx: 0, Raw: raw, Scale: scale}} }

	// energy_lifetime has an FK to inverters; seed the inverter first.
	if err := s.UpsertInverterFromTelemetry(ctx, "u", 1, "qs1a", "QS1A", 1000); err != nil {
		t.Fatal(err)
	}

	// First obs anchors at raw=1000, contributes 0.
	if err := s.WriteEnergyLifetime(ctx, "u", 1000, row(1000), maxCh); err != nil {
		t.Fatal(err)
	}
	// +1 s, raw +1 -> 1 Wh accepted.
	if err := s.WriteEnergyLifetime(ctx, "u", 2000, row(1001), maxCh); err != nil {
		t.Fatal(err)
	}
	// +1 s, garbage raw spike -> ~1e9 Wh delta, must be REJECTED.
	if err := s.WriteEnergyLifetime(ctx, "u", 3000, row(1_000_000_000), maxCh); err != nil {
		t.Fatal(err)
	}
	got, err := s.FleetLifetimeWh(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got > 2 { // should still be ~1 Wh, not ~1e9
		t.Fatalf("garbage delta not rejected: cumulative=%.1f Wh", got)
	}
	// After re-anchor at the spike, a normal +1 resumes accumulating.
	if err := s.WriteEnergyLifetime(ctx, "u", 4000, row(1_000_000_001), maxCh); err != nil {
		t.Fatal(err)
	}
	got2, _ := s.FleetLifetimeWh(ctx)
	if got2 < got || got2 > got+2 {
		t.Errorf("post-reanchor accumulation off: before=%.1f after=%.1f", got, got2)
	}
}

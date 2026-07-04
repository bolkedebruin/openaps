package sunspec

import (
	"testing"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
)

const (
	modelQS1A = 0x18 // single-phase
	modelQT2  = 0x32 // three-phase
)

func TestDetectPhase_ThreePhaseModelForces103(t *testing.T) {
	s := source.Snapshot{Inverters: []source.Inverter{
		{Online: true, Phase: 1, Model: modelQS1A, ACPowerW: 100}, // single, L1
		{Online: true, Model: modelQT2, ACPowerW: 600},            // three-phase
	}}
	if got := detectPhase(s); got != PhaseThree {
		t.Fatalf("with a 3-phase inverter: detectPhase = %v; want PhaseThree", got)
	}
	// An offline three-phase inverter must not force 103.
	s.Inverters[1].Online = false
	if got := detectPhase(s); got != PhaseSingle {
		t.Fatalf("3-phase offline: detectPhase = %v; want PhaseSingle", got)
	}
}

func TestPhaseBuckets_SinglePhaseAssignedLegs(t *testing.T) {
	s := source.Snapshot{GridVoltageV: 230, Inverters: []source.Inverter{
		{Online: true, Phase: 1, Model: modelQS1A, ACPowerW: 300, ACVoltageV: 230},
		{Online: true, Phase: 2, Model: modelQS1A, ACPowerW: 200, ACVoltageV: 231},
		{Online: true, Phase: 3, Model: modelQS1A, ACPowerW: 100, ACVoltageV: 232},
	}}
	pw, v := phaseBuckets(s)
	if pw != [3]float64{300, 200, 100} {
		t.Fatalf("powerW = %v; want [300 200 100]", pw)
	}
	if v != [3]float64{230, 231, 232} {
		t.Fatalf("voltageV = %v; want [230 231 232]", v)
	}
}

// TestPhaseBuckets_ThreePhaseEvenSplit is the regression for the old bug where
// a three-phase inverter's whole output was dumped on L1. Its power must split
// evenly and its measured per-leg voltage must flow through.
func TestPhaseBuckets_ThreePhaseEvenSplit(t *testing.T) {
	s := source.Snapshot{GridVoltageV: 230, Inverters: []source.Inverter{
		{Online: true, Model: modelQT2, ACPowerW: 900, PerLegVoltage: []float64{230, 231, 232}},
	}}
	pw, v := phaseBuckets(s)
	if pw != [3]float64{300, 300, 300} {
		t.Fatalf("powerW = %v; want [300 300 300] (900/3 per leg)", pw)
	}
	if v != [3]float64{230, 231, 232} {
		t.Fatalf("voltageV = %v; want measured per-leg [230 231 232]", v)
	}
}

func TestDerivePhaseCurrents_ThreePhase(t *testing.T) {
	s := source.Snapshot{GridVoltageV: 230, Inverters: []source.Inverter{
		{Online: true, Model: modelQT2, ACPowerW: 690, PerLegVoltage: []float64{230, 230, 230}},
	}}
	total, per := derivePhaseCurrents(s)
	// Each leg: 230 W / 230 V = 1.0 A → ×10 = 10.
	if per != [3]uint16{10, 10, 10} {
		t.Fatalf("perPhase = %v; want [10 10 10]", per)
	}
	if total != 30 {
		t.Fatalf("total = %d; want 30", total)
	}
}

func TestDerivePhaseVoltages_FallsBackToGridWhenLegEmpty(t *testing.T) {
	s := source.Snapshot{GridVoltageV: 237, Inverters: []source.Inverter{
		{Online: true, Phase: 1, Model: modelQS1A, ACPowerW: 100}, // ACVoltageV unset
	}}
	pv := derivePhaseVoltages(s)
	if pv[0] != 237 {
		t.Fatalf("leg L1 voltage = %d; want grid fallback 237", pv[0])
	}
}

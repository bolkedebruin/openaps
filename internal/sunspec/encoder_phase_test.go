package sunspec

import (
	"testing"

	"github.com/bolke/ecu-sunspec/internal/source"
)

func TestEncode_PhaseAutoDetect(t *testing.T) {
	cases := []struct {
		name      string
		phases    []int
		wantModel uint16
		wantUsed  int
	}{
		{"all-phase-1", []int{1, 1, 1}, 101, 1},
		{"split-1-2", []int{1, 2}, 102, 2},
		{"three-1-2-3", []int{1, 2, 3}, 103, 3},
		{"phase-zero-folds-to-A", []int{0, 0}, 101, 1},
		{"two-online-third-offline", []int{1, 2, 3}, 103, 3},
		{"empty-falls-to-101", nil, 101, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := source.Snapshot{}
			for _, p := range c.phases {
				s.Inverters = append(s.Inverters, source.Inverter{
					Online: true, Phase: p, ACPowerW: 100,
				})
			}
			bank := Encode(s, Options{})
			got := bank.At(BaseRegister + 70)
			if got != c.wantModel {
				t.Errorf("model=%d want %d", got, c.wantModel)
			}
		})
	}
}

func TestEncode_ExplicitPhaseOverride(t *testing.T) {
	s := source.Snapshot{
		GridVoltageV: 230,
		Inverters: []source.Inverter{
			{Online: true, Phase: 1, ACPowerW: 100},
		},
	}
	bank := Encode(s, Options{Phase: PhaseThree})
	if got := bank.At(BaseRegister + 70); got != InverterModelThreePhase {
		t.Errorf("expected forced 103, got %d", got)
	}
}

func TestEncode_SinglePhasePadsUnusedRegisters(t *testing.T) {
	s := source.Snapshot{
		GridVoltageV: 237,
		Inverters: []source.Inverter{
			{Online: true, Phase: 1, ACPowerW: 200},
		},
	}
	bank := Encode(s, Options{Phase: PhaseSingle})

	body := uint16(BaseRegister + 70 + 2)

	// AphA used; AphB/C are 0x8000.
	if got := bank.At(body + 1); got == 0xFFFF || got == 0 {
		t.Errorf("AphA should be a real value, got %#04x", got)
	}
	if got := bank.At(body + 2); got != 0xFFFF {
		t.Errorf("AphB=%#04x want 0xFFFF (uint16 not-impl)", got)
	}
	if got := bank.At(body + 3); got != 0xFFFF {
		t.Errorf("AphC=%#04x want 0xFFFF (uint16 not-impl)", got)
	}

	// PhVphA at body+8 is the grid voltage; PhVphB/C should be 0x8000.
	if got := bank.At(body + 8); got != 237 {
		t.Errorf("PhVphA=%d want 237", got)
	}
	if got := bank.At(body + 9); got != 0xFFFF {
		t.Errorf("PhVphB=%#04x want 0xFFFF (uint16 not-impl)", got)
	}
	if got := bank.At(body + 10); got != 0xFFFF {
		t.Errorf("PhVphC=%#04x want 0xFFFF (uint16 not-impl)", got)
	}
}

func TestEncode_OverrideModelName(t *testing.T) {
	s := source.Snapshot{}
	bank := Encode(s, Options{ModelName: "Test Inverter X"})
	got := readString(bank, BaseRegister+20, 16)
	if got != "Test Inverter X" {
		t.Errorf("Md=%q want %q", got, "Test Inverter X")
	}
}

package sunspec

import (
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
)

func TestEncode_Model160_Layout(t *testing.T) {
	bank := Encode(sampleSnapshot(), Options{})

	mpptHeaderBase, ok := findModel(bank, MultiMPPTModelID)
	if !ok {
		t.Fatal("Model 160 missing")
	}
	mpptBodyBase := mpptHeaderBase + 2

	// 3-inverter sample = 1 type-01 (2 panels) + 2 type-03 (4 panels each) = 10 panels.
	const wantN uint16 = 10
	const wantL uint16 = 8 + 20*wantN

	if got := bank.At(mpptHeaderBase + 1); got != wantL {
		t.Errorf("L=%d want %d", got, wantL)
	}

	if got := int16(bank.At(mpptBodyBase + 0)); got != -1 {
		t.Errorf("DCA_SF=%d want -1", got)
	}
	if got := int16(bank.At(mpptBodyBase + 1)); got != 0 {
		t.Errorf("DCV_SF=%d want 0", got)
	}
	if got := bank.At(mpptBodyBase + 6); got != wantN {
		t.Errorf("N=%d want %d", got, wantN)
	}
	if got := bank.At(mpptBodyBase + 7); got != 0xFFFF {
		t.Errorf("TmsPer=%#04x want 0xFFFF (not implemented)", got)
	}
}

func TestEncode_Model160_FirstPanel(t *testing.T) {
	bank := Encode(sampleSnapshot(), Options{})

	mpptHeaderBase, ok := findModel(bank, MultiMPPTModelID)
	if !ok {
		t.Fatal("Model 160 missing")
	}
	module0 := mpptHeaderBase + 2 + 8 // body + fixed block

	// First panel = inv 0 (type 01 DS3, PanelWatts = [247, 221]).
	// Channel 0 → P0 = 247 W.
	if got := bank.At(module0); got != 1 {
		t.Errorf("module 0 ID=%d want 1", got)
	}
	if got := readString(bank, module0+1, 8); got != "000001/A" {
		t.Errorf("module 0 IDStr=%q want %q", got, "000001/A")
	}
	if got := bank.At(module0 + 11); got != 247 {
		t.Errorf("module 0 DCW=%d want 247 (panel A of DS3)", got)
	}
	const offSt uint16 = 17
	if got := bank.At(module0 + offSt); got != StMPPT {
		t.Errorf("module 0 St=%d want StMPPT", got)
	}
}

func TestEncode_Model160_PanelLayoutMatchesInverterTypes(t *testing.T) {
	bank := Encode(sampleSnapshot(), Options{})

	mpptHeaderBase, ok := findModel(bank, MultiMPPTModelID)
	if !ok {
		t.Fatal("Model 160 missing")
	}
	body := mpptHeaderBase + 2 + 8

	// Walk all 10 modules and check IDStr suffixes follow the pattern:
	//   inv1 (type 01): A, B
	//   inv2 (type 03): A, B, C, D
	//   inv3 (type 03): A, B, C, D
	expected := []string{
		"000001/A", "000001/B",
		"000002/A", "000002/B", "000002/C", "000002/D",
		"000003/A", "000003/B", "000003/C", "000003/D",
	}
	for i, want := range expected {
		off := body + uint16(i)*20
		got := readString(bank, off+1, 8)
		if got != want {
			t.Errorf("module %d IDStr=%q want %q", i, got, want)
		}
	}
}

func TestEncode_Model160_OmittedWhenNoInvertersOnline(t *testing.T) {
	s := source.Snapshot{
		Captured:        time.Now(),
		SystemPowerW:    0,
		GridFrequencyHz: 50,
	}
	bank := Encode(s, Options{})

	if _, ok := findModel(bank, MultiMPPTModelID); ok {
		t.Error("Model 160 should be omitted when no panels are present")
	}
}

func TestEncode_Model160_DisableFlagSkipsModel(t *testing.T) {
	bank := Encode(sampleSnapshot(), Options{DisableMPPT: true})

	if _, ok := findModel(bank, MultiMPPTModelID); ok {
		t.Error("DisableMPPT=true: Model 160 should be skipped")
	}
}

func TestEncode_Model160_PanelCountFromMixedTypes(t *testing.T) {
	cases := []struct {
		name      string
		types     []string
		wantPanel int
	}{
		{"all-DS3", []string{"01", "01", "01"}, 6},
		{"all-DS3L", []string{"03", "03"}, 8},
		{"mix", []string{"01", "03", "04"}, 2 + 4 + 2},
		{"single", []string{"01"}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := source.Snapshot{Captured: time.Now()}
			for _, ty := range c.types {
				inv := source.Inverter{
					UID: "INV", Online: true, TypeCode: ty, Phase: 1,
					ACVoltageV: 230, TemperatureC: 30,
				}
				switch ty {
				case "01", "04":
					inv.PanelWatts = []int{100, 80}
				case "03":
					inv.PanelWatts = []int{100, 80, 70, 50}
				}
				s.Inverters = append(s.Inverters, inv)
			}
			bank := Encode(s, Options{})
			mpptHeaderBase, ok := findModel(bank, MultiMPPTModelID)
			if !ok {
				t.Fatal("Model 160 missing")
			}
			gotL := bank.At(mpptHeaderBase + 1)
			wantL := uint16(8 + 20*c.wantPanel)
			if gotL != wantL {
				t.Errorf("L=%d want %d (panels=%d)", gotL, wantL, c.wantPanel)
			}
		})
	}
}

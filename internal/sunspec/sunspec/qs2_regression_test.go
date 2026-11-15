package sunspec

import (
	"testing"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
)

func TestQS2SunSpecModelLabel(t *testing.T) {
	inv := source.Inverter{
		TypeCode: "03",
		Model:    0x36,
	}

	if got := inverterModelLabel(inv); got != "QS2" {
		t.Errorf("QS2 SunSpec model = %q, want QS2", got)
	}
}

// Verify that the QS2 exposes four distinct panel power readings
// through the actual SunSpec Model 160 register bank.
func TestQS2SunSpecModel160FourPanelRegisters(t *testing.T) {
	inv := source.Inverter{
		UID:        "821000011029",
		Online:     true,
		TypeCode:   "03",
		Model:      0x36,
		ACVoltageV: 230,
		PanelWatts: []int{25, 30, 10, 15},
	}

	bank := encodePerInverter(inv, "ECU", 2, Options{})

	mppt, ok := findModel(bank, MultiMPPTModelID)
	if !ok {
		t.Fatal("SunSpec Model 160 missing")
	}

	wantLen := MultiMPPTFixedBlockLen + MultiMPPTPerModuleLen*4
	if got := bank.At(mppt + 1); got != wantLen {
		t.Errorf("Model 160 length = %d, want %d", got, wantLen)
	}

	body := mppt + 2
	if got := bank.At(body + 6); got != 4 {
		t.Errorf("Model 160 module count = %d, want 4", got)
	}

	for i, want := range inv.PanelWatts {
		module := body + MultiMPPTFixedBlockLen +
			uint16(i)*MultiMPPTPerModuleLen

		// Each module begins with its ID, followed by its
		// label, DCA, DCV and DCW registers.
		if got := bank.At(module + 11); got != uint16(want) {
			t.Errorf(
				"panel %d DCW = %d, want %d",
				i+1, got, want,
			)
		}
	}
}

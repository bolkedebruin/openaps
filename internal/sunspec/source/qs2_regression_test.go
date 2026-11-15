package source

import "testing"

func TestQS2SunSpecFourChannels(t *testing.T) {
	typeCode := typeCodeFromModelCode(0x36)

	inv := Inverter{
		Online:     true,
		TypeCode:   typeCode,
		Model:      0x36,
		PanelWatts: []int{25, 30, 1, 1},
	}

	if got := inv.PanelCount(); got != 4 {
		t.Errorf("QS2 panel count = %d, want 4", got)
	}

	if got := len(inv.PanelPowers()); got != 4 {
		t.Errorf("QS2 SunSpec panel powers = %d, want 4", got)
	}
}

package sunspec

import (
	"testing"

	"github.com/bolke/ecu-sunspec/internal/source"
)

// TestEmitControls_WMaxLimPctSF verifies the emitted Model 123 body
// advertises WMaxLimPctSF=-2 at the documented body offset.
func TestEmitControls_WMaxLimPctSF(t *testing.T) {
	s := source.Snapshot{
		Inverters: []source.Inverter{
			{UID: "A", Online: true, TypeCode: "01", LimitedPowerW: 500,
				PanelWatts: []int{0, 0}},
		},
	}
	b := Encode(s, Options{})

	// Find Model 123 in the bank by walking the model headers.
	addr := uint16(BaseRegister + 2) // skip "SunS"
	var bodyStart uint16
	found := false
	for hop := 0; hop < 50; hop++ {
		hdr := b.Slice(addr, 2)
		if hdr[0] == EndModelID {
			break
		}
		if hdr[0] == ControlsModelID {
			bodyStart = addr + 2
			found = true
			break
		}
		addr += 2 + hdr[1]
	}
	if !found {
		t.Fatal("Model 123 not present in encoded bank")
	}

	got := int16(b.At(bodyStart + OffControlsWMaxLimPctSF))
	if got != int16(WMaxLimPctSF) {
		t.Errorf("WMaxLimPct_SF at body+%d = %d, want %d",
			OffControlsWMaxLimPctSF, got, WMaxLimPctSF)
	}
	if got != -2 {
		t.Errorf("WMaxLimPct_SF = %d, want -2", got)
	}
}

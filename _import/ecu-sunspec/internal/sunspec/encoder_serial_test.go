package sunspec

import (
	"testing"

	"github.com/bolke/ecu-sunspec/internal/source"
)

func TestSerialPriority(t *testing.T) {
	cases := []struct {
		name     string
		ecuid    string
		override string
		fallback string
		want     string
	}{
		{"override wins over ecuid", "FROM-FILE", "FORCED", "FB", "FORCED"},
		{"override wins over fallback", "", "FORCED", "FB", "FORCED"},
		{"ecuid wins over fallback", "FROM-FILE", "", "FB", "FROM-FILE"},
		{"fallback when nothing else", "", "", "FB", "FB"},
		{"empty when nothing set", "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := serialFor(
				source.Snapshot{ECUID: c.ecuid},
				Options{SerialOverride: c.override, SerialFallback: c.fallback},
			)
			if got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestEncode_SerialOverridePreservesFirmwareFromSnapshot(t *testing.T) {
	s := source.Snapshot{
		ECUID:    "REAL-SN-12345",
		Firmware: "2.1.29D",
		Model:    "ECU_R_PRO",
	}
	bank := Encode(s, Options{SerialOverride: "FORCED-SN"})

	// SN at base+52, 16 regs.
	gotSN := readString(bank, BaseRegister+52, 16)
	if gotSN != "FORCED-SN" {
		t.Errorf("SN=%q want FORCED-SN", gotSN)
	}
	// Vr at base+44, 8 regs — must still come from snapshot, not be wiped.
	gotVr := readString(bank, BaseRegister+44, 8)
	if gotVr != "2.1.29D" {
		t.Errorf("Vr=%q want 2.1.29D — override should not wipe firmware", gotVr)
	}
}

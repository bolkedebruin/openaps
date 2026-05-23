package codec

import (
	"strings"
	"testing"
)

// TestProtectionEnvelope_RejectsOutOfRange asserts the write-side physical
// safety backstop: a value that is within the wire bit-width but physically
// implausible (a near-zero under-voltage trip, a multi-kV over-voltage trip,
// a 200 Hz over-frequency trip, a 500 s clear time) is rejected before any
// frame is built, while a real EN value of the same param encodes fine.
func TestProtectionEnvelope_RejectsOutOfRange(t *testing.T) {
	cases := []struct {
		name    string
		param   string
		value   float64
		wantErr bool
	}{
		// Volts: envelope [100, 600].
		{"uv ok", "under_voltage_stage_2_90", 196, false},
		{"uv too low (1V)", "under_voltage_stage_2_90", 1, true},
		{"ov too high (12kV)", "over_voltage_slow", 12000, true},
		{"ov ok", "over_voltage_slow", 264, false},
		{"avg-ov ok", "min10_Over_average_voltage", 253, false},
		{"avg-ov too low", "min10_Over_average_voltage", 5, true},
		// Frequency: envelope [40, 70].
		{"of ok", "over_frequency_fast", 52, false},
		{"of too high (200Hz)", "over_frequency_fast", 200, true},
		{"uf too low", "under_frequency_fast", 10, true},
		{"freq-watt ok", "Over_frequency_Watt_High_set", 50.2, false},
		{"freq-watt (upper-case Frequency) too high", "Under_Frequency_Watt_Low_set", 99, true},
		// Clear times: envelope (0, 100] s.
		{"clear ok", "Under_Voltage2_clearance_time", 1.2, false},
		{"clear too big", "Under_Voltage2_clearance_time", 5000, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := EncodeSetProtection(ModelQS1A, tc.param, tc.value)
			if tc.wantErr && err == nil {
				t.Fatalf("%s=%g: expected envelope rejection, got nil error", tc.param, tc.value)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("%s=%g: expected success, got %v", tc.param, tc.value, err)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), "envelope") {
				t.Fatalf("%s=%g: expected an envelope error, got %v", tc.param, tc.value, err)
			}
		})
	}
}

// TestProtectionEnvelope_AppliesOnBroadcast confirms the same backstop guards
// the broadcast encode path, not just unicast.
func TestProtectionEnvelope_AppliesOnBroadcast(t *testing.T) {
	if _, err := EncodeSetProtectionBroadcast(ModelDS3, "over_voltage_slow", 12000); err == nil {
		t.Fatal("broadcast over_voltage_slow=12000V: expected envelope rejection, got nil")
	}
	if _, err := EncodeSetProtectionBroadcast(ModelDS3, "over_voltage_slow", 264); err != nil {
		t.Fatalf("broadcast over_voltage_slow=264V: expected success, got %v", err)
	}
}

// TestProtectionEnvelope_EnumNotClamped ensures the over-frequency mode enum
// (a discrete code, not a Hz value) is not run through the Hz envelope, but is
// bounded by its own enum range.
func TestProtectionEnvelope_EnumNotClamped(t *testing.T) {
	// 13 (AS/NZS) is a valid mode and must encode despite being well outside
	// the Hz envelope [40,70].
	if _, err := EncodeSetProtection(ModelQS1A, "Over_frequency_Watt_set", 13); err != nil {
		t.Fatalf("Over_frequency_Watt_set=13: expected success, got %v", err)
	}
	// A wrapping mode (300 → byte 44) must be rejected so it can't silently
	// select a different mode.
	if _, err := EncodeSetProtection(ModelQS1A, "Over_frequency_Watt_set", 300); err == nil {
		t.Fatal("Over_frequency_Watt_set=300: expected byte-range rejection, got nil")
	}
}

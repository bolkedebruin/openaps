package gridprofile

import (
	"testing"
)

func TestLookup_AllMappedCodes(t *testing.T) {
	// Every code in the forward map must have a non-empty Group and Point.
	for code, e := range forwardMap {
		if e.Group == "" {
			t.Errorf("code %q: empty Group", code)
		}
		if e.Point == "" {
			t.Errorf("code %q: empty Point", code)
		}
	}
}

func TestLongName_Known(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"DH", "Under_Frequency_Watt_Low_set"},
		{"DI", "Under_Frequency_Watt_High_set"},
		{"CB", "Over_frequency_Watt_Low_set"},
		{"CC", "Over_frequency_Watt_High_set"},
		{"AG", "grid_recovery_time"},
		{"DD", "Over_Frequency_Watt_Slope_set"},
		{"CA", "Over_frequency_Watt_Start_set"},
	}
	for _, tc := range cases {
		ln, ok := LongName(tc.code)
		if !ok {
			t.Errorf("LongName(%q): not found", tc.code)
			continue
		}
		if ln != tc.want {
			t.Errorf("LongName(%q): got %q want %q", tc.code, ln, tc.want)
		}
	}
}

func TestLongName_DecodeOnly(t *testing.T) {
	// DC is decode-only (no write encoder); LongName must return false.
	_, ok := LongName("DC")
	if ok {
		t.Error("DC is decode-only: LongName should return false")
	}
}

func TestLongName_TripCodes(t *testing.T) {
	// Trip codes must now have LongNames (encodable via EncodeSetProtection).
	cases := []struct {
		code string
		want string
	}{
		{"AC", "under_voltage_stage_2_90"},
		{"AQ", "under_voltage_stage_2"},
		{"AD", "over_voltage_slow"},
		{"AY", "over_voltage_slow_90"},
		{"AI", "over_voltage_fast"},
		{"AH", "under_voltage_fast"},
		{"AE", "under_frequency_slow"},
		{"AJ", "under_frequency_fast"},
		{"AF", "over_frequency_slow"},
		{"AK", "over_frequency_fast"},
		{"AB", "min10_Over_average_voltage"},
		{"BB", "Under_Voltage1_clearance_time"},
		{"BC", "Over_Voltage1_clearance_time"},
		{"BD", "Under_Voltage2_clearance_time"},
		{"BE", "Over_Voltage2_clearance_time"},
		{"BH", "Under_Frequency1_clearance_time"},
		{"BI", "Over_Frequency1_clearance_time"},
		{"BJ", "Under_Frequency2_clearance_time"},
		{"BK", "Over_Frequency2_clearance_time"},
	}
	for _, tc := range cases {
		ln, ok := LongName(tc.code)
		if !ok {
			t.Errorf("LongName(%q): expected LongName %q, got not-found", tc.code, tc.want)
			continue
		}
		if ln != tc.want {
			t.Errorf("LongName(%q): got %q want %q", tc.code, ln, tc.want)
		}
	}
}

// TestSFPinnedValues verifies that the three corrected scale factors match
// the pinned values from the ecu-sunspec emitters (freq_droop.go, enter_service.go).
func TestSFPinnedValues(t *testing.T) {
	cases := []struct {
		code      string
		wantSF    int
		wantSFRef *string // nil means unscaled
	}{
		{"DD", -1, sfRefStr("K_SF")}, // K_SF=-1 per freq_droop.go
		{"CG", 0, nil},               // RspTms_SF=0 (whole seconds) per freq_droop.go
		{"AS", 0, nil},               // ESRmpTms unscaled per enter_service.go
	}
	for _, tc := range cases {
		e, ok := forwardMap[tc.code]
		if !ok {
			t.Errorf("code %q not in forward map", tc.code)
			continue
		}
		if e.SF != tc.wantSF {
			t.Errorf("code %q: SF=%d want %d", tc.code, e.SF, tc.wantSF)
		}
		if tc.wantSFRef == nil {
			if e.SFRef != nil {
				t.Errorf("code %q: SFRef=%v want nil (unscaled)", tc.code, *e.SFRef)
			}
		} else {
			if e.SFRef == nil {
				t.Errorf("code %q: SFRef=nil want %q", tc.code, *tc.wantSFRef)
			} else if *e.SFRef != *tc.wantSFRef {
				t.Errorf("code %q: SFRef=%q want %q", tc.code, *e.SFRef, *tc.wantSFRef)
			}
		}
	}
}

func TestVoltagePctFlag(t *testing.T) {
	// Voltage trip codes must have IsVoltagePct=true
	voltCodes := []string{"AC", "AQ", "AD", "AY", "AB", "BN", "BO"}
	for _, code := range voltCodes {
		e, ok := forwardMap[code]
		if !ok {
			t.Errorf("code %q not in map", code)
			continue
		}
		if !e.IsVoltagePct {
			t.Errorf("code %q: IsVoltagePct should be true", code)
		}
	}

	// Frequency and time codes must NOT have IsVoltagePct
	noVoltCodes := []string{"AE", "AJ", "AF", "AK", "BJ", "BH", "BK", "BI", "AG", "DD"}
	for _, code := range noVoltCodes {
		e, ok := forwardMap[code]
		if !ok {
			t.Errorf("code %q not in map", code)
			continue
		}
		if e.IsVoltagePct {
			t.Errorf("code %q: IsVoltagePct should be false", code)
		}
	}
}

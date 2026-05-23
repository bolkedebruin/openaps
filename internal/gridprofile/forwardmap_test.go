package gridprofile

import (
	"math"
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

func TestLookup_KnownCode(t *testing.T) {
	e, ok := Lookup("DD")
	if !ok {
		t.Fatal("DD not found")
	}
	if e.Model != 711 {
		t.Errorf("DD model: got %d want 711", e.Model)
	}
	if e.LongName != "Over_Frequency_Watt_Slope_set" {
		t.Errorf("DD LongName: got %q", e.LongName)
	}
}

func TestLookup_UnknownCode(t *testing.T) {
	_, ok := Lookup("ZZ")
	if ok {
		t.Error("ZZ should not be in forward map")
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

func TestVToVNomPct_RoundTrip(t *testing.T) {
	vnom := 230.0
	cases := []float64{184.0, 230.0, 253.0, 259.0}
	for _, v := range cases {
		pct := VToVNomPct(v, vnom)
		got := VNomPctToV(pct, vnom)
		if math.Abs(got-v) > 1e-9 {
			t.Errorf("V %.1f: VToVNomPct→VNomPctToV round trip: got %.6f", v, got)
		}
	}
}

func TestVToVNomPct_Values(t *testing.T) {
	vnom := 230.0
	// 230 V = 100 %VNom
	if pct := VToVNomPct(230.0, vnom); math.Abs(pct-100.0) > 1e-9 {
		t.Errorf("230V→%%VNom: got %g want 100", pct)
	}
	// 0.8 * 230 = 184 V = 80 %VNom
	if pct := VToVNomPct(184.0, vnom); math.Abs(pct-80.0) > 1e-9 {
		t.Errorf("184V→%%VNom: got %g want 80", pct)
	}
}

func TestEncodeSunSpec_RoundTrip(t *testing.T) {
	cases := []struct {
		native float64
		sf     int
		want   float64
	}{
		{16.7, -1, 167},      // DD droop slope (K_SF=-1, per freq_droop.go)
		{50.2, -2, 5020},     // frequency Hz
		{80.87, -2, 8087},    // %VNom for a voltage trip
		{20.0, 0, 20},        // seconds, unscaled (AG/CG/AS)
		{51.5, -2, 5150},     // AK threshold
	}
	for _, tc := range cases {
		got := EncodeSunSpec(tc.native, tc.sf)
		if math.Abs(got-tc.want) > 0.5 {
			t.Errorf("EncodeSunSpec(%g, %d): got %g want %g", tc.native, tc.sf, got, tc.want)
		}
		// Round-trip back
		back := DecodeSunSpec(got, tc.sf)
		// allow for rounding loss
		if math.Abs(back-tc.native) > math.Abs(tc.native)*0.001+0.01 {
			t.Errorf("DecodeSunSpec(EncodeSunSpec(%g, %d)): got %g", tc.native, tc.sf, back)
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
		{"DD", -1, sfRefStr("K_SF")},        // K_SF=-1 per freq_droop.go
		{"CG", 0, nil},                       // RspTms_SF=0 (whole seconds) per freq_droop.go
		{"AS", 0, nil},                        // ESRmpTms unscaled per enter_service.go
	}
	for _, tc := range cases {
		e, ok := Lookup(tc.code)
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
		e, ok := Lookup(code)
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
		e, ok := Lookup(code)
		if !ok {
			t.Errorf("code %q not in map", code)
			continue
		}
		if e.IsVoltagePct {
			t.Errorf("code %q: IsVoltagePct should be false", code)
		}
	}
}

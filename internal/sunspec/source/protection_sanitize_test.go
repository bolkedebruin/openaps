package source

import "testing"

func mkProt(vals map[string]float64) ProtectionParams {
	pp := ProtectionParams{Has: map[string]bool{}}
	for code, v := range vals {
		if f := protCodeField(&pp, code); f != nil {
			*f = v
			pp.Has[code] = true
		}
	}
	return pp
}

func TestSanitizeProtection_QS1A_KeepsInRangePageA(t *testing.T) {
	// QS1A now reads page-A back via 0xDB, so in-range page-A codes are kept.
	pp := mkProt(map[string]float64{
		"AK": 52, "AI": 264, "AC": 196, "AG": 60, // page-A, in physical range
		"BN": 196, "CB": 50.2, "DH": 47, // page-B/C
	})
	out := SanitizeProtection(pp, true)
	for _, c := range []string{"AK", "AI", "AC", "AG", "BN", "CB", "DH"} {
		if !out.Has[c] {
			t.Errorf("QS1A in-range code %s should be kept", c)
		}
	}
}

func TestSanitizeProtection_QS1A_DropsOutOfRange(t *testing.T) {
	// The physical-range backstop still drops garbage on QS1A.
	pp := mkProt(map[string]float64{
		"AI": 74,  // out-of-range volt (≤100) → dropped
		"AC": 196, // in range → kept
	})
	out := SanitizeProtection(pp, true)
	if out.Has["AI"] {
		t.Error("out-of-range AI=74 V should be dropped on QS1A")
	}
	if !out.Has["AC"] {
		t.Error("in-range AC=196 V should be kept on QS1A")
	}
}

func TestSanitizeProtection_DS3_KeepsPageA_DropsOutOfRange(t *testing.T) {
	pp := mkProt(map[string]float64{
		"AK": 52, "AC": 196, "AI": 263, // valid DS3 page-A
		"AF": 999, // out-of-range freq → dropped
	})
	out := SanitizeProtection(pp, false)
	for _, c := range []string{"AK", "AC", "AI"} {
		if !out.Has[c] {
			t.Errorf("DS3 in-range %s should be kept", c)
		}
	}
	if out.Has["AF"] {
		t.Error("out-of-range AF=999 Hz should be dropped")
	}
}

func TestSanitizeProtection_DoesNotMutateInput(t *testing.T) {
	pp := mkProt(map[string]float64{"AK": 52, "AI": 74})
	_ = SanitizeProtection(pp, true)
	if !pp.Has["AI"] {
		t.Error("input Has map must not be mutated")
	}
}

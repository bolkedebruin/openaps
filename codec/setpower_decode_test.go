package codec

import (
	"math"
	"testing"
)

func TestDecodeSetPowerRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		model uint8
		fam   Family
	}{
		{"ds3", ModelDS3, FamilyDS3},
		{"qs1a", ModelQS1A, FamilyQS1},
	}
	for _, c := range cases {
		for _, bc := range []bool{false, true} {
			for _, w := range []uint16{MinPanelLimitW, 123, 375, MaxPanelLimitW} {
				f, err := EncodeSetPower(c.model, w, bc)
				if err != nil {
					t.Fatalf("%s encode %d: %v", c.name, w, err)
				}
				fam, got, gotBC, ok := DecodeSetPower(f)
				if !ok || fam != c.fam || gotBC != bc {
					t.Fatalf("%s bc=%v w=%d: ok=%v fam=%v bc=%v", c.name, bc, w, ok, fam, gotBC)
				}
				if math.Abs(got-float64(w)) > 1 {
					t.Errorf("%s bc=%v: decoded %.3f W, want %d", c.name, bc, got, w)
				}
			}
		}
	}
}

func TestDecodeSetPowerRejectsOtherFrames(t *testing.T) {
	prot, err := EncodeSetProtection(ModelQS1A, "Over_frequency_Watt_Low_set", 50.2)
	if err != nil {
		t.Fatal(err)
	}
	c3, err := EncodeSetPower(ModelYC600, 300, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range [][]byte{prot[0], c3, EncodeOnOff(true, false), nil} {
		if _, _, _, ok := DecodeSetPower(f); ok {
			t.Errorf("DecodeSetPower(% X) ok=true, want false", f)
		}
	}
}

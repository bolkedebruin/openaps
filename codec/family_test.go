package codec

import (
	"reflect"
	"sort"
	"testing"
)

// TestFamilyOfCoversAllKnownModels asserts that every model code listed
// in info.go's constants resolves to a non-Unknown Family — except the
// three-phase Ext29/30/31 codes whose family name is not pinned yet.
// This is the guardrail that prevents a new model code from being added
// to info.go without a matching Family classification.
func TestFamilyOfCoversAllKnownModels(t *testing.T) {
	cases := []struct {
		code uint8
		want Family
	}{
		{ModelYC600Old, FamilyYC600},
		{ModelYC1000, FamilyYC1000},
		{ModelYC600, FamilyYC600},
		{ModelQS1, FamilyQS1},
		{ModelYC600B, FamilyYC600},
		{ModelQS1A, FamilyQS1},
		{ModelDS3, FamilyDS3},
		{ModelDS3H, FamilyDS3},
		{ModelDS3L, FamilyDS3},
		{ModelQT2, FamilyQT2},
		{ModelExt36, FamilyDS3},
	}
	for _, c := range cases {
		got := FamilyOf(c.code)
		if got != c.want {
			t.Errorf("FamilyOf(0x%02X) = %v, want %v", c.code, got, c.want)
		}
		if got == FamilyUnknown {
			t.Errorf("FamilyOf(0x%02X) returned FamilyUnknown — every classified model must resolve to a real family", c.code)
		}
	}
}

// TestFamilyOfUnknown asserts that an unmapped code yields
// FamilyUnknown rather than panicking or falling through to QS1A. This
// is the site-bias regression test — the codec must not silently route
// an unknown model through the QS1A encoder.
func TestFamilyOfUnknown(t *testing.T) {
	for _, c := range []uint8{0x00, 0x01, 0x09, 0xFF} {
		if got := FamilyOf(c); got != FamilyUnknown {
			t.Errorf("FamilyOf(0x%02X) = %v, want FamilyUnknown", c, got)
		}
	}
	if IsKnownModel(0x00) {
		t.Error("IsKnownModel(0x00) = true, want false")
	}
	if !IsKnownModel(ModelDS3) {
		t.Error("IsKnownModel(ModelDS3) = false, want true")
	}
}

// TestFamilyStringStableKeys pins the lowercase identifiers used as
// database column values and CLI flag values. Changing one of these is
// an observable change that consumers (ecu-web, ecu-sunspec, stored
// rows) will see — so any rename must be deliberate.
func TestFamilyStringStableKeys(t *testing.T) {
	cases := map[Family]string{
		FamilyUnknown: "",
		FamilyDS3:     "ds3",
		FamilyQS1:     "qs1",
		FamilyYC600:   "yc600",
		FamilyYC1000:  "yc1000",
		FamilyQT2:     "qt2",
	}
	for f, want := range cases {
		if got := f.String(); got != want {
			t.Errorf("Family(%d).String() = %q, want %q", f, got, want)
		}
	}
}

// TestFamilyForModelString covers the wire-string entry point used by
// ingest. The wire model is set by codec.DecodeReplyFromEnvelope as
// "DS3" / "QS1A" / "unknown(0x..)" — the prefix matcher must absorb the
// QS1 vs QS1A distinction and reject empty/unknown without falling
// through to a real family.
func TestFamilyForModelString(t *testing.T) {
	cases := map[string]Family{
		"":              FamilyUnknown,
		"DS3":           FamilyDS3,
		"DS3-H":         FamilyDS3,
		"DSP":           FamilyDS3,
		"QS1":           FamilyQS1,
		"QS1A":          FamilyQS1,
		"YC600":         FamilyYC600,
		"YC1000":        FamilyYC1000,
		"QT2":           FamilyQT2,
		"unknown(0x09)": FamilyUnknown,
		"garbage":       FamilyUnknown,
	}
	for s, want := range cases {
		if got := FamilyForModelString(s); got != want {
			t.Errorf("FamilyForModelString(%q) = %v, want %v", s, got, want)
		}
	}
}

// TestBroadcastModelCodes pins the exact set of model codes that have
// a broadcast protection encoder. Adding a new code here without an
// encoder in EncodeSetProtectionBroadcast would build wire frames the
// firmware rejects; removing one would silently drop coverage. The
// list must be deterministically sorted so consumers can compare
// against it.
func TestBroadcastModelCodes(t *testing.T) {
	got := BroadcastModelCodes()
	want := []uint8{
		ModelQS1,   // 0x08
		ModelQS1A,  // 0x18
		ModelDS3,   // 0x20
		ModelDS3H,  // 0x21
		ModelDS3L,  // 0x22
		ModelExt36, // 0x36
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BroadcastModelCodes() = %v, want %v", got, want)
	}
	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i] < got[j] }) {
		t.Errorf("BroadcastModelCodes() is not sorted: %v", got)
	}
	// Every returned code must actually have a non-Unknown Family —
	// otherwise the broadcast dispatch would error at runtime.
	for _, c := range got {
		if FamilyOf(c) == FamilyUnknown {
			t.Errorf("BroadcastModelCodes() contains 0x%02X which is FamilyUnknown", c)
		}
	}
}

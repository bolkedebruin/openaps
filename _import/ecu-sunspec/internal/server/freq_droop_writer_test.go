package server

import (
	"bytes"
	"context"
	"testing"

	"github.com/bolke/ecu-sunspec/internal/source"
	"github.com/bolke/inv-driver/codec"
)

// newFreqDroopWriterFixture builds a writer pinned to the DS3 family
// (model 0x20). Tests that need a different family use
// newFreqDroopWriterFixtureModel instead.
func newFreqDroopWriterFixture(t *testing.T, uid uint8, prot source.ProtectionParams, invUIDs ...string) (*FreqDroopWriter, *fakeSender) {
	return newFreqDroopWriterFixtureModel(t, uid, 0x20, prot, invUIDs...)
}

func newFreqDroopWriterFixtureModel(t *testing.T, uid uint8, model int, prot source.ProtectionParams, invUIDs ...string) (*FreqDroopWriter, *fakeSender) {
	t.Helper()
	fake := &fakeSender{}
	invs := make([]source.Inverter, 0, len(invUIDs))
	protMap := map[string]source.ProtectionParams{}
	for _, u := range invUIDs {
		invs = append(invs, source.Inverter{UID: u, Online: true, TypeCode: "01", Model: model})
		protMap[u] = prot
	}
	return &FreqDroopWriter{
		uid:    uid,
		snap:   source.Snapshot{Inverters: invs, Protection: protMap},
		sender: fake,
	}, fake
}

// activeProt returns a typical EN 50549-1 NL profile: OF1=52.0, droop
// starts at 50.2 Hz. Has flags set so cross-param checks have data.
func activeProt() source.ProtectionParams {
	return source.ProtectionParams{
		OFFast:       52.0, // AK
		OFDroopStart: 50.2, // DC
		OFDroopEnd:   52.0, // CC
		OFDroopSlope: 16.7, // DD (units uncertain — see freq_droop_writer.go)
		OFDroopMode:  13,   // CV (AS/NZS variant)
		Has: map[string]bool{
			"AK": true, "DC": true, "CC": true, "DD": true, "CV": true,
		},
	}
}

func encodeDbOf(dbOf uint32) []uint16 {
	return []uint16{uint16(dbOf >> 16), uint16(dbOf & 0xFFFF)}
}

// DbOf → Over_frequency_Watt_Start_set (CA). Only QS1A has a CA encoder
// (DS3 has no CA branch in main.exe). DbOf=50 cHz → HzStart 50.5 Hz,
// which sits below OF1=52.0 (margin 0.5) so the cross-param check passes;
// the encoded frame goes through inv-driver.
func TestFreqDroopWriter_DbOf_QS1A_Frame(t *testing.T) {
	fdw, fake := newFreqDroopWriterFixtureModel(t, 2, 0x18, activeProt(), "INV-A")
	if err := fdw.Apply(context.Background(), freqDroopBodyDbOfHi, encodeDbOf(50)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want, err := codec.EncodeSetProtection(0x18, "Over_frequency_Watt_Start_set", 50.5)
	if err != nil {
		t.Fatalf("EncodeSetProtection: %v", err)
	}
	if got := fake.frameFor("INV-A"); !bytes.Equal(got, want[0]) {
		t.Errorf("frame=% X want % X", got, want[0])
	}
}

// DS3 has no CA branch — DbOf must error (no legacy fallback), nothing sent.
func TestFreqDroopWriter_DbOf_DS3_Unsupported(t *testing.T) {
	fdw, fake := newFreqDroopWriterFixtureModel(t, 2, 0x20, activeProt(), "INV-A")
	if err := fdw.Apply(context.Background(), freqDroopBodyDbOfHi, encodeDbOf(50)); err == nil {
		t.Error("expected error: DS3 has no Over_frequency_Watt_Start_set encoder")
	}
	if len(fake.frames()) != 0 {
		t.Error("must not send a frame for an unsupported param")
	}
}

func TestFreqDroopWriter_DbOfTooCloseToTrip(t *testing.T) {
	fdw, _ := newFreqDroopWriterFixture(t, 2, activeProt(), "INV-A")
	// DbOf = 160 cHz → HzStart = 51.6 Hz. OF1=52.0, margin=0.5 → max=51.5.
	if err := fdw.Apply(context.Background(), freqDroopBodyDbOfHi, encodeDbOf(160)); err == nil {
		t.Error("expected rejection (HzStart too close to OF1 trip)")
	}
}

func TestFreqDroopWriter_DbOfBelowNominal(t *testing.T) {
	fdw, _ := newFreqDroopWriterFixture(t, 2, activeProt(), "INV-A")
	if err := fdw.Apply(context.Background(), freqDroopBodyDbOfHi, encodeDbOf(0)); err == nil {
		t.Error("expected rejection (HzStart=50.0)")
	}
}

func TestFreqDroopWriter_AggregateRejected(t *testing.T) {
	fdw, _ := newFreqDroopWriterFixture(t, 1, activeProt(), "INV-A", "INV-B")
	if err := fdw.Apply(context.Background(), freqDroopBodyDbOfHi, encodeDbOf(50)); err == nil {
		t.Error("expected error for aggregate uid 1")
	}
}

func TestFreqDroopWriter_RequiresAKToValidate(t *testing.T) {
	prot := activeProt()
	prot.Has["AK"] = false
	fdw, _ := newFreqDroopWriterFixtureModel(t, 2, 0x18, prot, "INV-A")
	if err := fdw.Apply(context.Background(), freqDroopBodyDbOfHi, encodeDbOf(50)); err == nil {
		t.Error("expected error when AK unavailable for cross-check")
	}
}

// KOf range validation runs before dispatch, so out-of-range still errors.
func TestFreqDroopWriter_KOfRangeRejection(t *testing.T) {
	fdw, _ := newFreqDroopWriterFixture(t, 2, activeProt(), "INV-A")
	for _, kOf := range []uint16{0, 50, 99, 1501, 5000} {
		if err := fdw.Apply(context.Background(), freqDroopBodyKOf, []uint16{kOf}); err == nil {
			t.Errorf("kOf=%d: expected range error, got nil", kOf)
		}
	}
}

// DS3 slope (CF) and delay (CG) encode and go through inv-driver. KOf is
// deci-percent → %P/Hz (÷10); RspTms is whole seconds.
func TestFreqDroopWriter_DS3_SlopeDelay_Frames(t *testing.T) {
	fdw, fake := newFreqDroopWriterFixtureModel(t, 2, 0x20, activeProt(), "INV-A")
	if err := fdw.Apply(context.Background(), freqDroopBodyKOf, []uint16{400}); err != nil {
		t.Fatalf("KOf Apply: %v", err)
	}
	wantSlope, _ := codec.EncodeSetProtection(0x20, "Over_Frequency_Watt_Slope_set", 40.0)
	if got := fake.frameFor("INV-A"); !bytes.Equal(got, wantSlope[0]) {
		t.Errorf("slope frame=% X want % X", got, wantSlope[0])
	}
	if err := fdw.Apply(context.Background(), freqDroopBodyRspTmsHi, []uint16{0, 5}); err != nil {
		t.Fatalf("RspTms Apply: %v", err)
	}
	wantDelay, _ := codec.EncodeSetProtection(0x20, "Over_frequency_Watt_Delay_Time_set", 5.0)
	if got := fake.frameFor("INV-A"); !bytes.Equal(got, wantDelay[0]) {
		t.Errorf("delay frame=% X want % X", got, wantDelay[0])
	}
}

// On QS1A slope (CF) and delay (CG) are firmware-rejected (routeFor
// Unsupported) — they must error with no frame sent.
func TestFreqDroopWriter_QS1A_SlopeDelayUnsupported(t *testing.T) {
	fdw, fake := newFreqDroopWriterFixtureModel(t, 2, 0x18, activeProt(), "INV-A")
	for _, c := range []struct {
		name string
		off  uint16
		regs []uint16
	}{
		{"slope", freqDroopBodyKOf, []uint16{400}},
		{"delay", freqDroopBodyRspTmsHi, []uint16{0, 5}},
	} {
		if err := fdw.Apply(context.Background(), c.off, c.regs); err == nil {
			t.Errorf("QS1A %s: expected RouteUnsupported error", c.name)
		}
	}
	if len(fake.frames()) != 0 {
		t.Errorf("QS1A slope/delay must send nothing, got %d frame(s)", len(fake.frames()))
	}
}

// Ena → over-frequency mode enum (CV): single frame on DS3, multi-frame
// (3 for mode 15 = disabled) on QS1A.
func TestFreqDroopWriter_Ena_CV(t *testing.T) {
	fdw, fake := newFreqDroopWriterFixtureModel(t, 2, 0x20, activeProt(), "INV-A")
	if err := fdw.Apply(context.Background(), freqDroopBodyEna, []uint16{0}); err != nil {
		t.Fatalf("DS3 Ena: %v", err)
	}
	wantDS3, _ := codec.EncodeSetProtection(0x20, "Over_frequency_Watt_set", float64(freqDroopModeDisabled))
	if got := fake.frames(); len(got) != 1 || !bytes.Equal(got[0].frame, wantDS3[0]) {
		t.Errorf("DS3 Ena: %d frame(s), want 1 matching CV-disabled", len(got))
	}

	fdw2, fake2 := newFreqDroopWriterFixtureModel(t, 2, 0x18, activeProt(), "INV-A")
	if err := fdw2.Apply(context.Background(), freqDroopBodyEna, []uint16{0}); err != nil {
		t.Fatalf("QS1A Ena: %v", err)
	}
	if got := fake2.frames(); len(got) != 3 {
		t.Errorf("QS1A Ena: %d frame(s), want 3 (mode-15 sequence)", len(got))
	}
}

func TestFreqDroopWriter_ReadOnlyFieldsRejected(t *testing.T) {
	fdw, _ := newFreqDroopWriterFixture(t, 2, activeProt(), "INV-A")
	cases := []struct {
		name string
		off  uint16
		val  []uint16
	}{
		{"AdptCtlRslt", 2, []uint16{1}},
		{"NCtl", 3, []uint16{2}},
		{"Db_SF", 9, []uint16{0xFFFE}},
		{"DbUf", 14, encodeDbOf(50)},
		{"PMin", 20, []uint16{50}},
		{"ReadOnly", 21, []uint16{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := fdw.Apply(context.Background(), tc.off, tc.val); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestFreqDroopWriter_UnknownFamilyRejected(t *testing.T) {
	fdw, fake := newFreqDroopWriterFixtureModel(t, 2, 999, activeProt(), "INV-A")
	if err := fdw.Apply(context.Background(), freqDroopBodyDbOfHi, encodeDbOf(50)); err == nil {
		t.Error("expected error for unknown family")
	}
	if len(fake.frames()) != 0 {
		t.Error("must not send for unknown family")
	}
}

// routeFor table-driven sanity check — pin the routing decisions the
// rest of the writer relies on so a careless edit can't silently
// reroute them.
func TestRouteFor(t *testing.T) {
	cases := []struct {
		family InverterFamily
		param  string
		want   WriteRoute
	}{
		{FamilyDS3, "Over_frequency_Watt_recover_High_set", RouteDirect},
		{FamilyDS3, "Over_Frequency_Watt_Slope_set", RouteDirect},
		{FamilyQS1A, "Over_frequency_Watt_Start_set", RouteDirect},
		{FamilyQS1A, "Over_frequency_Watt_High_set", RouteDirect},
		{FamilyQS1A, "Delt_P_Over_HF", RouteDirect},
		{FamilyQS1A, "Over_frequency_Watt_recover_High_set", RouteUnsupported},
		{FamilyQS1A, "Over_Frequency_Watt_Slope_set", RouteUnsupported},
		{FamilyQS1A, "Over_frequency_Watt_Delay_Time_set", RouteUnsupported},
		{FamilyQS1A, "Under_Frequency_Watt_Low_set", RouteDirect},
		{FamilyQS1A, "Under_Frequency_Watt_High_set", RouteDirect},
		{FamilyQS1A, "Over_frequency_Watt_Low_set", RouteDirect},
		{FamilyDS3, "Over_frequency_Watt_Low_set", RouteDirect},
		{FamilyDS3, "Under_Frequency_Watt_Low_set", RouteDirect},
		{FamilyUnknown, "anything", RouteUnsupported},
	}
	for _, tc := range cases {
		got := routeFor(tc.family, tc.param)
		if got != tc.want {
			t.Errorf("routeFor(%d, %q) = %d, want %d", tc.family, tc.param, got, tc.want)
		}
	}
}

// freqWattCurvePoints documents which SunSpec freq-watt registers map
// to which APsystems long-form name.
func TestFreqWattCurvePoints_DbOfMapping(t *testing.T) {
	var found *FreqWattCurvePoint
	for i := range freqWattCurvePoints {
		cp := &freqWattCurvePoints[i]
		if cp.Model == 711 && cp.Body == freqDroopBodyDbOfHi && cp.BodyLo == freqDroopBodyDbOfLo {
			found = cp
			break
		}
	}
	if found == nil {
		t.Fatal("expected a Model 711 curve-point entry covering DbOf body[12..13]")
	}
	if found.Aps != "Over_frequency_Watt_Start_set" {
		t.Errorf("DbOf maps to %q, want Over_frequency_Watt_Start_set", found.Aps)
	}
	if found.Code != "CA" {
		t.Errorf("DbOf code = %q, want CA", found.Code)
	}
	if got := found.Decode(0, 50); got < 50.49 || got > 50.51 {
		t.Errorf("Decode(0,50) = %v, want ~50.5", got)
	}
}

func TestFreqWattCurvePoints_Model134Mapping(t *testing.T) {
	wantByCode := map[string]string{
		"DH": "Under_Frequency_Watt_Low_set",
		"DI": "Under_Frequency_Watt_High_set",
		"CB": "Over_frequency_Watt_Low_set",
		"CC": "Over_frequency_Watt_High_set",
	}
	got := map[string]string{}
	for _, cp := range freqWattCurvePoints {
		if cp.Model != 134 {
			continue
		}
		got[cp.Code] = cp.Aps
	}
	for code, want := range wantByCode {
		if got[code] != want {
			t.Errorf("Model 134 code %s maps to %q, want %q", code, got[code], want)
		}
	}
	for _, cp := range freqWattCurvePoints {
		if cp.Model != 134 {
			continue
		}
		if got := cp.Decode(5020, 0); got < 50.19 || got > 50.21 {
			t.Errorf("Model 134 %s Decode(5020,0) = %v, want ~50.20", cp.Code, got)
		}
	}
}

func TestFamilyForModel(t *testing.T) {
	cases := []struct {
		code int
		want InverterFamily
	}{
		{0x18, FamilyQS1A},
		{8, FamilyQS1A},
		{0x20, FamilyDS3},
		{0x36, FamilyDS3},
		{0x29, FamilyQT2},
		{0x32, FamilyQT2},
		{7, FamilyYC600},
		{0x17, FamilyYC600},
		{5, FamilyYC1000},
		{0, FamilyUnknown},
		{999, FamilyUnknown},
	}
	for _, tc := range cases {
		got := familyForModel(tc.code)
		if got != tc.want {
			t.Errorf("familyForModel(%d) = %d, want %d", tc.code, got, tc.want)
		}
	}
}

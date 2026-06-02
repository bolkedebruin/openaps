package sunspec

import (
	"testing"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
	"github.com/bolkedebruin/openaps/wire"
)

func ds3Faults(setters ...func(*wire.DS3Faults)) *wire.InverterFaults {
	f := &wire.DS3Faults{}
	for _, s := range setters {
		s(f)
	}
	return &wire.InverterFaults{Family: &wire.InverterFaults_Ds3{Ds3: f}}
}

func qs1aFaults(setters ...func(*wire.QS1AFaults)) *wire.InverterFaults {
	f := &wire.QS1AFaults{}
	for _, s := range setters {
		s(f)
	}
	return &wire.InverterFaults{Family: &wire.InverterFaults_Qs1A{Qs1A: f}}
}

func TestFaultsToSunSpecEvt1_NilReturnsZero(t *testing.T) {
	if got := FaultsToSunSpecEvt1(nil); got != 0 {
		t.Errorf("nil faults: got 0x%x want 0", got)
	}
}

func TestFaultsToSunSpecEvt1_DS3GroundFault(t *testing.T) {
	f := ds3Faults(func(d *wire.DS3Faults) { d.DcGroundFault = true })
	if FaultsToSunSpecEvt1(f)&Evt1GroundFault == 0 {
		t.Error("DcGroundFault should set Evt1GroundFault")
	}
}

func TestFaultsToSunSpecEvt1_DS3IsoFaultIsGround(t *testing.T) {
	f := ds3Faults(func(d *wire.DS3Faults) { d.IsoFaultA = true })
	if FaultsToSunSpecEvt1(f)&Evt1GroundFault == 0 {
		t.Error("IsoFaultA should set Evt1GroundFault (SunSpec collapses iso into ground-fault)")
	}
}

// Healthy producing inverters report all-zero status on the wire,
// so FaultsToSunSpecEvt1 must NOT derive Evt1GridDisconnect or
// Evt1DCDisconnect from the state bits — that would alarm
// continuously. State bits inform SunSpec St, not Evt1.
func TestFaultsToSunSpecEvt1_HealthyStateIsZero(t *testing.T) {
	if got := FaultsToSunSpecEvt1(ds3Faults()); got != 0 {
		t.Errorf("healthy DS3 (all-false faults): got 0x%x want 0", got)
	}
	if got := FaultsToSunSpecEvt1(qs1aFaults()); got != 0 {
		t.Errorf("healthy QS1A (all-false faults): got 0x%x want 0", got)
	}
}

func TestFaultsToSunSpecEvt1_DS3FreqBuckets(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*wire.DS3Faults)
		mask  uint32
	}{
		{"OverFreqStage1", func(d *wire.DS3Faults) { d.OverFreqStage1 = true }, Evt1OverFrequency},
		{"OverFreqAux", func(d *wire.DS3Faults) { d.OverFreqAux = true }, Evt1OverFrequency},
		{"UnderFreqStage2", func(d *wire.DS3Faults) { d.UnderFreqStage2 = true }, Evt1UnderFrequency},
		{"AcOverVoltStage1", func(d *wire.DS3Faults) { d.AcOverVoltStage1 = true }, Evt1ACOverVolt},
		{"AcUnderVoltStage2", func(d *wire.DS3Faults) { d.AcUnderVoltStage2 = true }, Evt1ACUnderVolt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if FaultsToSunSpecEvt1(ds3Faults(tc.setup))&tc.mask == 0 {
				t.Errorf("expected mask 0x%x set", tc.mask)
			}
		})
	}
}

func TestFaultsToSunSpecEvt1_QS1AOverTemp(t *testing.T) {
	f := qs1aFaults(func(q *wire.QS1AFaults) { q.OverTemperature = true })
	if FaultsToSunSpecEvt1(f)&Evt1OverTemp == 0 {
		t.Error("OverTemperature should set Evt1OverTemp")
	}
}

func TestFaultsToSunSpecEvt1_QS1AIsoCDPathContributes(t *testing.T) {
	// QS1A has iso C and D; both should land in Evt1GroundFault.
	f := qs1aFaults(func(q *wire.QS1AFaults) { q.IsoFaultC = true })
	if FaultsToSunSpecEvt1(f)&Evt1GroundFault == 0 {
		t.Error("IsoFaultC should set Evt1GroundFault")
	}
}

func TestFaultsToEvtVnd1_NilReturnsZero(t *testing.T) {
	if got := FaultsToEvtVnd1(nil); got != 0 {
		t.Errorf("nil faults: got 0x%x want 0", got)
	}
}

func TestFaultsToEvtVnd1_DS3StageMappings(t *testing.T) {
	// DS3 stage1 → Primary, stage2 → Secondary, aux → Tertiary.
	for _, tc := range []struct {
		name  string
		setup func(*wire.DS3Faults)
		mask  uint32
	}{
		{"GridRelayFault", func(d *wire.DS3Faults) { d.GridRelayFault = true }, EvtVnd1GridRelayFault},
		{"OverFreqStage1→Primary", func(d *wire.DS3Faults) { d.OverFreqStage1 = true }, EvtVnd1OverFreqPrimary},
		{"OverFreqStage2→Secondary", func(d *wire.DS3Faults) { d.OverFreqStage2 = true }, EvtVnd1OverFreqSecondary},
		{"OverFreqAux→Tertiary", func(d *wire.DS3Faults) { d.OverFreqAux = true }, EvtVnd1OverFreqTertiary},
		{"OverFreqExtra→Extra", func(d *wire.DS3Faults) { d.OverFreqExtra = true }, EvtVnd1OverFreqExtra},
		{"IsoFaultB", func(d *wire.DS3Faults) { d.IsoFaultB = true }, EvtVnd1IsoFaultB},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := FaultsToEvtVnd1(ds3Faults(tc.setup)); got&tc.mask == 0 {
				t.Errorf("got 0x%x, expected mask 0x%x set", got, tc.mask)
			}
		})
	}
}

func TestFaultsToEvtVnd1_QS1AFastSlowMappings(t *testing.T) {
	// QS1A fast → Primary, slow → Secondary.
	for _, tc := range []struct {
		name  string
		setup func(*wire.QS1AFaults)
		mask  uint32
	}{
		{"OverFreqFast→Primary", func(q *wire.QS1AFaults) { q.OverFreqFast = true }, EvtVnd1OverFreqPrimary},
		{"OverFreqSlow→Secondary", func(q *wire.QS1AFaults) { q.OverFreqSlow = true }, EvtVnd1OverFreqSecondary},
		{"OverFreqRMS", func(q *wire.QS1AFaults) { q.OverFreqRms = true }, EvtVnd1OverFreqRMS},
		{"OverTemperature", func(q *wire.QS1AFaults) { q.OverTemperature = true }, EvtVnd1OverTemperature},
		{"CommFault", func(q *wire.QS1AFaults) { q.CommFault = true }, EvtVnd1CommFault},
		{"IsoFaultC", func(q *wire.QS1AFaults) { q.IsoFaultC = true }, EvtVnd1IsoFaultC},
		{"IsoFaultD", func(q *wire.QS1AFaults) { q.IsoFaultD = true }, EvtVnd1IsoFaultD},
		{"ZBLinkA", func(q *wire.QS1AFaults) { q.ZbLinkA = true }, EvtVnd1ZBLinkA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := FaultsToEvtVnd1(qs1aFaults(tc.setup)); got&tc.mask == 0 {
				t.Errorf("got 0x%x, expected mask 0x%x set", got, tc.mask)
			}
		})
	}
}

// projectEvents picks the typed path when faults != nil. EvtVnd1
// reflects the new layout; legacy EventBits no longer flow to
// EvtVnd1/2/3 once a typed source is present (avoids mixed layouts).
func TestProjectEvents_TypedPathPreferred(t *testing.T) {
	f := ds3Faults(func(d *wire.DS3Faults) { d.GridRelayFault = true })
	legacy := [4]uint32{0xDEADBEEF, 0xAA, 0xBB, 0xCC} // would normally fill EvtVnd1..3
	_, vnd1, vnd2, vnd3, _ := projectEvents(f, legacy)
	if vnd1&EvtVnd1GridRelayFault == 0 {
		t.Errorf("typed EvtVnd1 missing GridRelayFault: 0x%x", vnd1)
	}
	if vnd1 == legacy[0] {
		t.Errorf("EvtVnd1 should NOT equal legacy bits when typed faults present")
	}
	if vnd2 != 0 || vnd3 != 0 {
		t.Errorf("EvtVnd2/3 should be zero when typed faults present, got %x %x", vnd2, vnd3)
	}
}

// When no typed faults, legacy EventBits flow into the old EvtVnd
// positions verbatim — preserves the bit-perfect output for
// SQLite-only inverters during the hybrid window.
func TestProjectEvents_LegacyFallback(t *testing.T) {
	legacy := [4]uint32{0xDEADBEEF, 0xAA, 0xBB, 0xCC}
	evt1, vnd1, vnd2, vnd3, _ := projectEvents(nil, legacy)
	if vnd1 != legacy[0] || vnd2 != legacy[1] || vnd3 != legacy[2] {
		t.Errorf("legacy fallback: got vnd1=%x vnd2=%x vnd3=%x", vnd1, vnd2, vnd3)
	}
	// Evt1 still comes from the legacy mapper.
	if evt1 != MapAPsystemsToSunSpecEvt1(legacy) {
		t.Errorf("Evt1 mismatch with legacy mapper")
	}
}

// Hybrid window: when faults supply a bit the legacy mapper doesn't,
// Evt1 must show both contributions OR'd together. Pick bit 17
// (GFDI Locked) which only the legacy mapper sets, and a typed
// over-temp (which only the typed mapper sets).
func TestProjectEvents_HybridEvt1IsOred(t *testing.T) {
	legacy := [4]uint32{1 << 17, 0, 0, 0} // GFDI Locked → Evt1GroundFault
	f := qs1aFaults(func(q *wire.QS1AFaults) { q.OverTemperature = true })
	evt1, _, _, _, _ := projectEvents(f, legacy)
	if evt1&Evt1GroundFault == 0 {
		t.Error("hybrid Evt1 should keep legacy GroundFault contribution")
	}
	if evt1&Evt1OverTemp == 0 {
		t.Error("hybrid Evt1 should also include typed OverTemp")
	}
}

func TestAnyInverterHasFaults(t *testing.T) {
	if (source.Snapshot{}).AnyInverterHasFaults() {
		t.Error("empty snapshot should report no faults")
	}
	s := source.Snapshot{Inverters: []source.Inverter{
		{UID: "A"}, {UID: "B"},
	}}
	if s.AnyInverterHasFaults() {
		t.Error("snapshot with no Faults pointers should report none")
	}
	s.Inverters[1].Faults = ds3Faults()
	if !s.AnyInverterHasFaults() {
		t.Error("snapshot with one Faults should report present")
	}
}

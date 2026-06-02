package sunspec

import (
	"github.com/bolke/ecu-sunspec/internal/source"
	"github.com/bolke/inv-driver/wire"
)

// eventOutputForInverter computes the (Evt1, EvtVnd1..4) tuple a
// SunSpec inverter bank emits for one inverter. Picks the typed
// inv-driver path when present and OR's in the SQLite-EventBits
// projection for completeness during the hybrid window. When no
// faults are available, falls back to the legacy bit layout.
func eventOutputForInverter(inv source.Inverter) (evt1, vnd1, vnd2, vnd3, vnd4 uint32) {
	return projectEvents(inv.Faults, inv.EventBits)
}

// eventOutputForAggregate is the fleet-level OR — emits the union
// of all per-inverter alarm signals into the aggregate SunSpec bank
// (slave 1) so a fault on any one inverter shows up at system level.
func eventOutputForAggregate(s source.Snapshot) (evt1, vnd1, vnd2, vnd3, vnd4 uint32) {
	for _, inv := range s.Inverters {
		e, v1, v2, v3, v4 := projectEvents(inv.Faults, inv.EventBits)
		evt1 |= e
		vnd1 |= v1
		vnd2 |= v2
		vnd3 |= v3
		vnd4 |= v4
	}
	return
}

// projectEvents is the per-inverter joiner. Standard Evt1 is the OR
// of both sources (typed + legacy) so the broad SunSpec ecosystem
// sees complete information during hybrid. The new EvtVnd1 layout
// is typed-faults-only — SQLite-only inverters contribute zero, by
// design, since the new layout is incompatible with the old PHP-UI
// bit positions and we don't reverse-translate.
func projectEvents(faults *wire.InverterFaults, legacy [4]uint32) (evt1, vnd1, vnd2, vnd3, vnd4 uint32) {
	evt1 = MapAPsystemsToSunSpecEvt1(legacy) | FaultsToSunSpecEvt1(faults)
	if faults != nil {
		vnd1 = FaultsToEvtVnd1(faults)
	} else {
		// No typed source yet: emit the legacy raw bits at the old
		// PHP-UI positions so consumers reading via the old table
		// still get data while the fleet transitions.
		vnd1 = legacy[0]
		vnd2 = legacy[1]
		vnd3 = legacy[2]
	}
	return
}

// FaultsToSunSpecEvt1 projects a typed wire.InverterFaults onto the
// standard SunSpec Inverter Model Evt1 bitfield. Mirrors what
// MapAPsystemsToSunSpecEvt1 does for the SQLite 86-bit string, but
// works on the per-bit-named faults the codec extracts directly from
// the ZB reply.
//
// Returns 0 when faults is nil.
//
// Bits this projection cannot supply today (CABINET_OPEN,
// BLOWN_STRING_FUSE, UNDER_TEMP, MEMORY_LOSS) stay zero — neither
// DS3 nor QS1A has a body source for them. The standard SunSpec
// surface stays a subset of the legacy mapper; both contribute via
// OR in the encoder during hybrid.
func FaultsToSunSpecEvt1(faults *wire.InverterFaults) uint32 {
	if faults == nil {
		return 0
	}
	var evt uint32
	if ds3 := faults.GetDs3(); ds3 != nil {
		if ds3.GetDcGroundFault() {
			evt |= Evt1GroundFault
		}
		if ds3.GetIsoFaultA() || ds3.GetIsoFaultB() {
			evt |= Evt1GroundFault
		}
		// GridRelayFault / DCContactorFault / DCBusFault are fault
		// bits (1=event, verified via DS3_DS3D_status @ 0x290f8 alarm
		// gate). Not projected to standard Evt1 — the firmware groups
		// these slots differently per family (DS3 puts 0x29a in comm/
		// RTC, QS1A pairs it with over-temp), so a single SunSpec
		// semantic mapping isn't safe without a captured fault frame.
		// Exposed in EvtVnd1 instead.
		if ds3.GetAcOverVoltStage1() || ds3.GetAcOverVoltStage2() {
			evt |= Evt1ACOverVolt
		}
		if ds3.GetAcUnderVoltStage1() || ds3.GetAcUnderVoltStage2() {
			evt |= Evt1ACUnderVolt
		}
		if ds3.GetOverFreqStage1() || ds3.GetOverFreqStage2() ||
			ds3.GetOverFreqAux() || ds3.GetOverFreqExtra() {
			evt |= Evt1OverFrequency
		}
		if ds3.GetUnderFreqStage1() || ds3.GetUnderFreqStage2() ||
			ds3.GetUnderFreqAux() || ds3.GetUnderFreqExtra() {
			evt |= Evt1UnderFrequency
		}
	}
	if qs1a := faults.GetQs1A(); qs1a != nil {
		if qs1a.GetDcGroundFault() {
			evt |= Evt1GroundFault
		}
		if qs1a.GetIsoFaultA() || qs1a.GetIsoFaultB() ||
			qs1a.GetIsoFaultC() || qs1a.GetIsoFaultD() {
			evt |= Evt1GroundFault
		}
		// See DS3 branch: state bits don't drive Evt1.
		if qs1a.GetOverTemperature() {
			evt |= Evt1OverTemp
		}
		if qs1a.GetAcOverVoltFast() || qs1a.GetAcOverVoltSlow() {
			evt |= Evt1ACOverVolt
		}
		if qs1a.GetAcUnderVoltFast() || qs1a.GetAcUnderVoltSlow() {
			evt |= Evt1ACUnderVolt
		}
		if qs1a.GetOverFreqFast() || qs1a.GetOverFreqSlow() ||
			qs1a.GetOverFreqExtra() || qs1a.GetOverFreqRms() {
			evt |= Evt1OverFrequency
		}
		if qs1a.GetUnderFreqFast() || qs1a.GetUnderFreqSlow() ||
			qs1a.GetUnderFreqExtra() || qs1a.GetUnderFreqRms() {
			evt |= Evt1UnderFrequency
		}
	}
	return evt
}

// SunSpec EvtVnd1 bit positions for the new typed-faults layout.
// This layout is owned by ecu-sunspec — NOT the legacy PHP-UI bit
// table. See docs/EVENTS.md.
//
// Bits 26-31 reserved.
const (
	EvtVnd1GridRelayFault      uint32 = 1 << 0
	EvtVnd1DCContactorFault    uint32 = 1 << 1
	EvtVnd1DCBusFault          uint32 = 1 << 2
	EvtVnd1DCGroundFault       uint32 = 1 << 3
	EvtVnd1CommFault           uint32 = 1 << 4
	EvtVnd1OverTemperature     uint32 = 1 << 5
	EvtVnd1IsoFaultA           uint32 = 1 << 6
	EvtVnd1IsoFaultB           uint32 = 1 << 7
	EvtVnd1IsoFaultC           uint32 = 1 << 8
	EvtVnd1IsoFaultD           uint32 = 1 << 9
	EvtVnd1ACOverVoltPrimary   uint32 = 1 << 10
	EvtVnd1ACOverVoltSecondary uint32 = 1 << 11
	EvtVnd1ACUnderVoltPrimary  uint32 = 1 << 12
	EvtVnd1ACUnderVoltSecondary uint32 = 1 << 13
	EvtVnd1OverFreqPrimary     uint32 = 1 << 14
	EvtVnd1OverFreqSecondary   uint32 = 1 << 15
	EvtVnd1OverFreqTertiary    uint32 = 1 << 16
	EvtVnd1OverFreqExtra       uint32 = 1 << 17
	EvtVnd1OverFreqRMS         uint32 = 1 << 18
	EvtVnd1UnderFreqPrimary    uint32 = 1 << 19
	EvtVnd1UnderFreqSecondary  uint32 = 1 << 20
	EvtVnd1UnderFreqTertiary   uint32 = 1 << 21
	EvtVnd1UnderFreqExtra      uint32 = 1 << 22
	EvtVnd1UnderFreqRMS        uint32 = 1 << 23
	EvtVnd1ZBLinkA             uint32 = 1 << 24
	EvtVnd1ZBLinkB             uint32 = 1 << 25
)

// FaultsToEvtVnd1 projects typed faults onto the new EvtVnd1 layout.
// DS3's stage1/stage2/aux → Primary/Secondary/Tertiary; QS1A's
// fast/slow → Primary/Secondary; family-specific Extra/RMS get
// dedicated bits.
//
// Returns 0 when faults is nil.
func FaultsToEvtVnd1(faults *wire.InverterFaults) uint32 {
	if faults == nil {
		return 0
	}
	var v uint32
	if ds3 := faults.GetDs3(); ds3 != nil {
		if ds3.GetGridRelayFault() {
			v |= EvtVnd1GridRelayFault
		}
		if ds3.GetDcContactorFault() {
			v |= EvtVnd1DCContactorFault
		}
		if ds3.GetDcBusFault() {
			v |= EvtVnd1DCBusFault
		}
		if ds3.GetDcGroundFault() {
			v |= EvtVnd1DCGroundFault
		}
		if ds3.GetIsoFaultA() {
			v |= EvtVnd1IsoFaultA
		}
		if ds3.GetIsoFaultB() {
			v |= EvtVnd1IsoFaultB
		}
		if ds3.GetAcOverVoltStage1() {
			v |= EvtVnd1ACOverVoltPrimary
		}
		if ds3.GetAcOverVoltStage2() {
			v |= EvtVnd1ACOverVoltSecondary
		}
		if ds3.GetAcUnderVoltStage1() {
			v |= EvtVnd1ACUnderVoltPrimary
		}
		if ds3.GetAcUnderVoltStage2() {
			v |= EvtVnd1ACUnderVoltSecondary
		}
		if ds3.GetOverFreqStage1() {
			v |= EvtVnd1OverFreqPrimary
		}
		if ds3.GetOverFreqStage2() {
			v |= EvtVnd1OverFreqSecondary
		}
		if ds3.GetOverFreqAux() {
			v |= EvtVnd1OverFreqTertiary
		}
		if ds3.GetOverFreqExtra() {
			v |= EvtVnd1OverFreqExtra
		}
		if ds3.GetUnderFreqStage1() {
			v |= EvtVnd1UnderFreqPrimary
		}
		if ds3.GetUnderFreqStage2() {
			v |= EvtVnd1UnderFreqSecondary
		}
		if ds3.GetUnderFreqAux() {
			v |= EvtVnd1UnderFreqTertiary
		}
		if ds3.GetUnderFreqExtra() {
			v |= EvtVnd1UnderFreqExtra
		}
	}
	if qs1a := faults.GetQs1A(); qs1a != nil {
		if qs1a.GetGridRelayFault() {
			v |= EvtVnd1GridRelayFault
		}
		if qs1a.GetDcContactorFault() {
			v |= EvtVnd1DCContactorFault
		}
		if qs1a.GetDcBusFault() {
			v |= EvtVnd1DCBusFault
		}
		if qs1a.GetDcGroundFault() {
			v |= EvtVnd1DCGroundFault
		}
		if qs1a.GetCommFault() {
			v |= EvtVnd1CommFault
		}
		if qs1a.GetOverTemperature() {
			v |= EvtVnd1OverTemperature
		}
		if qs1a.GetIsoFaultA() {
			v |= EvtVnd1IsoFaultA
		}
		if qs1a.GetIsoFaultB() {
			v |= EvtVnd1IsoFaultB
		}
		if qs1a.GetIsoFaultC() {
			v |= EvtVnd1IsoFaultC
		}
		if qs1a.GetIsoFaultD() {
			v |= EvtVnd1IsoFaultD
		}
		if qs1a.GetAcOverVoltFast() {
			v |= EvtVnd1ACOverVoltPrimary
		}
		if qs1a.GetAcOverVoltSlow() {
			v |= EvtVnd1ACOverVoltSecondary
		}
		if qs1a.GetAcUnderVoltFast() {
			v |= EvtVnd1ACUnderVoltPrimary
		}
		if qs1a.GetAcUnderVoltSlow() {
			v |= EvtVnd1ACUnderVoltSecondary
		}
		if qs1a.GetOverFreqFast() {
			v |= EvtVnd1OverFreqPrimary
		}
		if qs1a.GetOverFreqSlow() {
			v |= EvtVnd1OverFreqSecondary
		}
		if qs1a.GetOverFreqExtra() {
			v |= EvtVnd1OverFreqExtra
		}
		if qs1a.GetOverFreqRms() {
			v |= EvtVnd1OverFreqRMS
		}
		if qs1a.GetUnderFreqFast() {
			v |= EvtVnd1UnderFreqPrimary
		}
		if qs1a.GetUnderFreqSlow() {
			v |= EvtVnd1UnderFreqSecondary
		}
		if qs1a.GetUnderFreqExtra() {
			v |= EvtVnd1UnderFreqExtra
		}
		if qs1a.GetUnderFreqRms() {
			v |= EvtVnd1UnderFreqRMS
		}
		if qs1a.GetZbLinkA() {
			v |= EvtVnd1ZBLinkA
		}
		if qs1a.GetZbLinkB() {
			v |= EvtVnd1ZBLinkB
		}
	}
	return v
}

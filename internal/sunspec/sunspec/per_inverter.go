package sunspec

import (
	"strconv"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
)

// EncodePerInverterWithProtection produces a self-contained SunSpec bank for a
// single microinverter. Layout is the SunSpec equivalent of "one Fronius
// behind the Datalogger" — Common Model identifies the microinverter, the
// Inverter Model (101, single-phase) exposes its electrical state, and
// Multi-MPPT (160) lists its panels. The IEEE 1547-2018 DER trip + Enter
// Service models are populated from the supplied protection params for this
// inverter.
//
// The bank is intended to be served at a per-inverter Modbus unit ID, so a
// SunSpec scanner walking unit IDs 2..N+1 sees N microinverters as separate
// devices.
func EncodePerInverterWithProtection(inv source.Inverter, ecuid string, unitID uint16, opt Options, prot source.ProtectionParams) Bank {
	if opt.Manufacturer == "" {
		opt.Manufacturer = DefaultManufacturer
	}

	bank := Bank{Regs: make([]uint16, 0, 256), Base: BaseRegister}

	bank.put16(0x5375, 0x6E53) // SunS

	// --- Common Model ---
	bank.put16(CommonModelID, CommonModelBodyLen)
	bank.putString(opt.Manufacturer, 16)
	bank.putString(inverterModelLabel(inv), 16)
	bank.putString("", 8)                            // Opt
	bank.putString(strconv.Itoa(inv.SoftwareVer), 8) // Vr
	bank.putString(inv.UID, 16)                      // SN = inverter UID
	bank.put16(unitID)                               // DA — Modbus unit address
	bank.put16(0)                                    // pad

	// --- Inverter Model 101 (single-phase) ---
	bank.put16(InverterModelSinglePhase, InverterModelBodyLen)

	// A / per-phase A — derive from W/V; 0.1 A units (ASF=-1)
	var a uint16
	if inv.ACVoltageV > 0 && inv.ACPowerW >= 0 {
		amps10 := float64(inv.ACPowerW) / float64(inv.ACVoltageV) * 10
		if amps10 < 65535 {
			a = uint16(amps10 + 0.5)
		}
	}
	bank.put16(a)
	bank.put16(a, notImplU16, notImplU16) // AphA, AphB, AphC
	bank.put16(scaleFactor(-1))           // ASF

	// PPVphAB/BC/CA — line-line voltages, not measured
	bank.put16(notImplU16, notImplU16, notImplU16)

	// PhVphA — line-neutral, B/C unimplemented for single-phase
	v := uint16(0)
	if inv.ACVoltageV > 0 && inv.ACVoltageV < 65535 {
		v = uint16(inv.ACVoltageV)
	}
	bank.put16(v, notImplU16, notImplU16)
	bank.put16(scaleFactor(0)) // VSF

	// W / WSF
	w := int32(inv.ACPowerW)
	bank.put16(uint16(int16(clampInt32(w, -32760, 32760))))
	bank.put16(scaleFactor(0))

	// Hz / HzSF — ×100
	hz := uint16(0)
	if inv.FrequencyHz > 0 {
		hz = uint16(inv.FrequencyHz*100 + 0.5)
	}
	bank.put16(hz)
	bank.put16(scaleFactor(-2))

	// VA / VAr / PF — emitted as "not implemented" by design.
	//
	// APsystems' own SunSpec server (port 502 on the ECU) reports values
	// here for DS3-family inverters by computing VA = A_measured × V,
	// PF = W/VA, VAr = √(VA² − W²). For QS1 it derives A from W/V, so
	// VA collapses to W and Q is structurally zero — VAr=0, PF=1.0
	// regardless of actual conditions.
	//
	// We don't capture the inverters' independently measured A in our
	// snapshot today (parameters_app.conf surfaces W and V but not the
	// per-channel measured A on DS3), so geometric Q can't be honestly
	// computed without proxying APsystems' :502. We choose to emit
	// not-impl rather than fake VAr=0/PF=1 stand-ins for inverters that
	// don't measure Q.
	bank.put16(notImplS16)
	bank.put16(scaleFactor(0))
	bank.put16(notImplS16)
	bank.put16(scaleFactor(0))
	bank.put16(notImplS16)
	bank.put16(scaleFactor(0))

	// WH (acc32) — per-inverter lifetime not tracked; emit 0 (acc32 sentinel)
	bank.putAcc32(0)
	bank.put16(scaleFactor(0))

	// DC fields not exposed per-inverter
	bank.put16(notImplU16) // DCA
	bank.put16(scaleFactor(0))
	bank.put16(notImplU16) // DCV
	bank.put16(scaleFactor(0))
	bank.put16(notImplS16) // DCW
	bank.put16(scaleFactor(0))

	// Tmp
	bank.put16(uint16(int16(clampInt32(int32(inv.TemperatureC), -100, 200))))
	bank.put16(notImplS16) // TmpSnk
	bank.put16(notImplS16) // TmpTrns
	bank.put16(notImplS16) // TmpOt
	bank.put16(scaleFactor(0))

	// St
	bank.put16(inverterOperatingState(inv.Online, inv.ACPowerW))
	bank.put16(0) // StVnd

	// Evt1 + EvtVnd*: typed inv-driver faults preferred, legacy
	// EventBits OR'd into the standard Evt1 for completeness during
	// hybrid. See eventOutputForInverter and docs/EVENTS.md.
	evt1, vnd1, vnd2, vnd3, vnd4 := eventOutputForInverter(inv)
	bank.putAcc32(uint64(evt1))
	bank.putAcc32(uint64(notImplU32)) // Evt2 — reserved
	bank.putAcc32(uint64(vnd1))
	bank.putAcc32(uint64(vnd2))
	bank.putAcc32(uint64(vnd3))
	bank.putAcc32(uint64(vnd4))

	// --- Inverter Float Model 111 — same field set as 101, float32 encoding ---
	emitInverterFloatPerInverter(&bank, inv)

	// --- Model 114 — DC Data float (per-panel) ---
	emitDCDataFloatPerInverter(&bank, inv)

	// --- Model 120 — Nameplate Ratings for THIS inverter ---
	// Per-inverter WRtg comes from inv.NameplateW() — codec.NameplateWattsForModel
	// (the same table inv-driver uses for the fleet WRtg), else a
	// TypeCode-based fallback.
	emitNameplatePerInverter(&bank, inv)

	// --- Model 123 — Inverter Controls (read/write) for THIS inverter ---
	pct, ena, conn := PerInverterControlsState(inv)
	emitControls(&bank, pct, ena, conn)

	// --- Models 707/708/709/710 + 703 + 711 — DER trip + Enter Service + Freq Droop ---
	// Sourced from this inverter's active protection_parameters60code row.
	// vNom: prefer this inverter's reported AC voltage, fall back to 230 V.
	vNom := float64(inv.ACVoltageV)
	emitDERTripModels(&bank, prot, vNom)
	emitEnterService(&bank, prot, vNom)
	{
		rslt, req := lookupWriteRslt(opt, uint8(unitID), FreqDroopModelID)
		emitFreqDroop(&bank, prot, rslt, req)
	}
	emitFreqWattCurve(&bank, prot)

	// --- Multi-MPPT (160) — only this inverter's panels ---
	if !opt.DisableMPPT {
		emitPerInverterMPPT(&bank, inv)
	}

	bank.put16(EndModelID, 0)
	return bank
}

func emitPerInverterMPPT(bank *Bank, inv source.Inverter) {
	powers := inv.PanelPowers()
	if len(powers) == 0 {
		return
	}
	n := uint16(len(powers))
	bodyLen := MultiMPPTFixedBlockLen + MultiMPPTPerModuleLen*n

	bank.put16(MultiMPPTModelID, bodyLen)

	// Fixed block (8 regs body)
	bank.put16(scaleFactor(-1))
	bank.put16(scaleFactor(0))
	bank.put16(scaleFactor(0))
	bank.put16(scaleFactor(0))
	bank.putAcc32(uint64(notImplU32)) // Evt — global module events
	bank.put16(n)
	bank.put16(notImplU16)

	// One module per panel
	for i, p := range powers {
		bank.put16(uint16(i + 1))
		bank.putString(panelLabel(inv, i), 8)

		dca := notImplU16
		if inv.ACVoltageV > 0 && p >= 0 {
			amps10 := float64(p) / float64(inv.ACVoltageV) * 10
			if amps10 < 65535 {
				dca = uint16(amps10 + 0.5)
			}
		}
		bank.put16(dca)

		dcv := uint16(notImplU16)
		if inv.ACVoltageV > 0 && inv.ACVoltageV < 65535 {
			dcv = uint16(inv.ACVoltageV)
		}
		bank.put16(dcv)

		dcw := uint16(notImplU16)
		if p >= 0 && p < 65535 {
			dcw = uint16(p)
		}
		bank.put16(dcw)

		bank.putAcc32(0)                  // DCWH — per-panel lifetime not tracked
		bank.putAcc32(uint64(notImplU32)) // Tms — emit "not implemented" sentinel

		bank.put16(uint16(int16(clampInt32(int32(inv.TemperatureC), -100, 200))))

		st := StOff
		if p > 0 {
			st = StMPPT
		}
		bank.put16(st)
		bank.putAcc32(uint64(notImplU32)) // DCEvt — bitfield32 not-impl
	}
}

// inverterModelLabel returns the Common Model `Md` for a per-inverter bank.
//
// The mapping comes from APsystems' wire-protocol type code (parameters_app.conf
// column 2). The same code list is used by HA's homeassistant-apsystems_ecu_reader:
//
//	"01" — YC600 / DS3        (2-channel)
//	"03" — QS1                (4-channel)
//	"04" — DS3-H / DS3D-L     (2-channel)
//	"05" — QT2                (4-channel three-phase)
//	"02" — YC1000             (4-channel three-phase)
//
// The vendor name lives in `Mn`, so don't repeat it here — Venus and HA
// combine Mn + Md when displaying.
func inverterModelLabel(inv source.Inverter) string {
	switch inv.TypeCode {
	case "01":
		return "DS3"
	case "02":
		return "YC1000"
	case "03":
		return "QS1"
	case "04":
		return "DS3-H"
	case "05":
		return "QT2"
	}
	if inv.TypeCode != "" {
		return "type-" + inv.TypeCode
	}
	return "microinverter"
}

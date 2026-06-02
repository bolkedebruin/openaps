package sunspec

import "github.com/bolke/ecu-sunspec/internal/source"

// emitBasicSettings writes SunSpec Model 121 (Basic Settings).
//
// Model 121 declares the inverter's user-set caps and grid limits.
// SunSpec spec marks both WMax and VRef as access=RW: configurable
// setpoints, not live measurements. Live measurements live in Models
// 101/103/111/113 (PhVphA, etc.).
//
// VRef is derived from the ECU's active grid-protection thresholds (the
// 10-min average over-voltage limit at 110% of nominal), not hardcoded —
// it tracks the regulatory grid profile the ECU has actually pushed to
// the inverters. notImplU16 when the ECU hasn't populated those.
//
// Length is fixed at 30 registers per the SunSpec spec.
func emitBasicSettings(bank *Bank, s source.Snapshot) {
	bank.put16(BasicSettingsModelID, BasicSettingsBodyLen)

	limitedW := s.TotalLimitedW()
	if limitedW <= 0 || limitedW > 65535 {
		bank.put16(notImplU16) // WMax — no setpoint
	} else {
		bank.put16(uint16(limitedW))
	}

	if v := s.NominalVRefV(); v > 0 && v < 65535 {
		bank.put16(uint16(v))
	} else {
		bank.put16(notImplU16)
	}

	bank.put16(notImplS16) // VRefOfs — voltage offset (int16)

	// VMax / VMin — if we surfaced grid-protection thresholds we'd plug them
	// here; for now leave as not implemented.
	bank.put16(notImplU16) // VMax
	bank.put16(notImplU16) // VMin

	// VAMax / VArMax{Q1..Q4} / VArAval / WGra / PFMin{Q1..Q4} — all int16/uint16,
	// not measured here.
	bank.put16(notImplU16) // VAMax
	bank.put16(notImplS16) // VArMaxQ1
	bank.put16(notImplS16) // VArMaxQ2
	bank.put16(notImplS16) // VArMaxQ3
	bank.put16(notImplS16) // VArMaxQ4
	bank.put16(notImplU16) // WGra (watt gradient)
	bank.put16(notImplS16) // PFMinQ1
	bank.put16(notImplS16) // PFMinQ2
	bank.put16(notImplS16) // PFMinQ3
	bank.put16(notImplS16) // PFMinQ4
	bank.put16(notImplU16) // VArAct (VAr action)
	bank.put16(notImplU16) // ClcTotVA
	bank.put16(notImplU16) // MaxRmpRte (% of WGra)
	bank.put16(notImplU16) // ECPNomHz (nominal grid frequency × 100)
	bank.put16(notImplU16) // ConnPh (connected phase)

	// Scale factors at the end (SunSpec convention for model 121)
	bank.put16(scaleFactor(0))  // WMax_SF
	bank.put16(scaleFactor(0))  // VRef_SF
	bank.put16(scaleFactor(0))  // VRefOfs_SF
	bank.put16(scaleFactor(0))  // VMinMax_SF
	bank.put16(scaleFactor(0))  // VAMax_SF
	bank.put16(scaleFactor(0))  // VArMax_SF
	bank.put16(scaleFactor(0))  // WGra_SF
	bank.put16(scaleFactor(-3)) // PFMin_SF (×0.001)
	bank.put16(scaleFactor(-2)) // ECPNomHz_SF (×0.01)

	// Pad — body length 30 per spec, 29 fields above + 1 pad.
	bank.put16(0)
}

package sunspec

import (
	"github.com/bolke/ecu-sunspec/internal/source"
)

// VendorModelID is in the SunSpec-reserved vendor range (64000–64999). Pick a
// number that won't collide with known vendors. 64202 is unused at time of
// writing; clients that don't recognize it will skip past via the standard
// length header.
const VendorModelID = 64202

// emitVendor writes a vendor-specific model with APsystems-specific extras
// that don't fit into standard SunSpec fields.
//
// Layout:
//
//	Fixed block (system-wide, 16 regs):
//	  PollingS (uint16)         - /etc/yuneng/polling_interval.conf       (1)
//	  EcuFwVer (8 regs string)  - /etc/yuneng/version.conf                (8)
//	  TodayWh  (acc32)          - daily_energy   × 1000                   (2)
//	  MonthWh  (acc32)          - monthly_energy × 1000                   (2)
//	  YearWh   (acc32)          - yearly_energy  × 1000                   (2)
//	  N        (uint16)         - number of per-inverter rows             (1)
//
//	Per-inverter block (10 regs each):
//	  Idx          (uint16)         - 1-based row index
//	  RSSI         (uint16)         - 0..255, raw signal_strength.signal_strength
//	  LimitedW     (uint16)         - power.limitedpower (per-panel cap, W)
//	  Phase        (uint16)         - id.phase (0..3)
//	  Model        (uint16)         - id.model
//	  SoftwareVer  (uint16)         - id.software_version
//	  PanelCnt     (uint16)         - per-type panel count
//	  NameplateW   (uint16)         - per-type nameplate watts
//	  Online       (uint16)         - 1=online, 0=offline
//	  Pad          (uint16)
//
// Total body length = 16 + 10·N. Skipped entirely if no inverters are present.
func emitVendor(bank *Bank, s source.Snapshot) {
	if len(s.Inverters) == 0 {
		return
	}
	n := uint16(len(s.Inverters))
	body := VendorFixedBlockLen + VendorPerInverterLen*n

	bank.put16(VendorModelID, body)

	// Fixed block
	bank.put16(uint16(clampInt32(int32(s.PollingInterval), 0, 65535)))
	bank.putString(s.Firmware, 8)
	bank.putAcc32(s.TodayEnergyWh)
	bank.putAcc32(s.MonthEnergyWh)
	bank.putAcc32(s.YearEnergyWh)
	bank.put16(n)

	// Per-inverter block
	for i, inv := range s.Inverters {
		bank.put16(uint16(i + 1))
		bank.put16(uint16(clampInt32(int32(inv.SignalStrength), 0, 65535)))
		bank.put16(uint16(clampInt32(int32(inv.LimitedPowerW), 0, 65535)))
		bank.put16(uint16(clampInt32(int32(inv.Phase), 0, 65535)))
		bank.put16(uint16(clampInt32(int32(inv.Model), 0, 65535)))
		bank.put16(uint16(clampInt32(int32(inv.SoftwareVer), 0, 65535)))
		bank.put16(uint16(inv.PanelCount()))
		bank.put16(uint16(clampInt32(int32(inv.NameplateW()), 0, 65535)))
		if inv.Online {
			bank.put16(1)
		} else {
			bank.put16(0)
		}
		bank.put16(0) // pad
	}
}

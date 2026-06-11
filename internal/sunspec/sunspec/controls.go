package sunspec

import "github.com/bolkedebruin/openaps/internal/sunspec/source"

// SunSpec Inverter Controls model. Length 24 registers (body, excluding ID + L).
const (
	ControlsModelID uint16 = 123
	ControlsBodyLen uint16 = 24
)

// Field offsets within the Model 123 body (0 = first reg after the L
// register). Layout per the SunSpec Model 123 (Inverter Controls)
// specification — must match `model_123.json` field order or pysunspec2
// reads will be misaligned:
//
//	+0  Conn_WinTms          uint16  sec
//	+1  Conn_RvrtTms         uint16  sec
//	+2  Conn                 enum16  [DISCONNECT, CONNECT]
//	+3  WMaxLimPct           uint16  % WMax
//	+4  WMaxLimPct_WinTms    uint16  sec
//	+5  WMaxLimPct_RvrtTms   uint16  sec
//	+6  WMaxLimPct_RmpTms    uint16  sec
//	+7  WMaxLim_Ena          enum16  [DISABLED, ENABLED]
//	+8  OutPFSet             int16   cos()  (PF×1000 with sf=OutPFSet_SF)
//	+9  OutPFSet_WinTms      uint16  sec
//	+10 OutPFSet_RvrtTms     uint16  sec
//	+11 OutPFSet_RmpTms      uint16  sec
//	+12 OutPFSet_Ena         enum16  [DISABLED, ENABLED]
//	+13 VArWMaxPct           int16   % WMax
//	+14 VArMaxPct            int16   % VArMax
//	+15 VArAvalPct           int16   % VArAval
//	+16 VArPct_WinTms        uint16  sec
//	+17 VArPct_RvrtTms       uint16  sec
//	+18 VArPct_RmpTms        uint16  sec
//	+19 VArPct_Mod           enum16  [NONE, WMax, VArMax, VArAval]
//	+20 VArPct_Ena           enum16  [DISABLED, ENABLED]
//	+21 WMaxLimPct_SF        sunssf
//	+22 OutPFSet_SF          sunssf
//	+23 VArPct_SF            sunssf
//
// APsystems firmware doesn't expose Q (reactive power) control or a
// scalar PF setpoint, so OutPFSet, VArWMaxPct, VArMaxPct, VArAvalPct are
// emitted as "not implemented" sentinels (notImplS16) and the *_Ena /
// *_Mod fields stay 0 / DISABLED. Field offsets are exported only for
// the writable subset (Conn, WMaxLimPct, WMaxLim_Ena).
const (
	OffControlsConnWinTms        uint16 = 0
	OffControlsConnRvrtTms       uint16 = 1
	OffControlsConn              uint16 = 2
	OffControlsWMaxLimPct        uint16 = 3
	OffControlsWMaxLimPctWinTms  uint16 = 4
	OffControlsWMaxLimPctRvrtTms uint16 = 5
	OffControlsWMaxLimPctRmpTms  uint16 = 6
	OffControlsWMaxLimEna        uint16 = 7
	OffControlsOutPFSet          uint16 = 8
	OffControlsWMaxLimPctSF      uint16 = 21
)

// WMaxLimPctSF is the advertised scale factor for WMaxLimPct. SF=-2 means
// raw integer × 10^-2 = percent, range 0..10000 → 0.00..100.00%, matching
// IEEE 1547 / Model 705 conventions for 0.01% setpoint resolution.
const WMaxLimPctSF int8 = -2

// WMaxLimPctRawFull is the raw WMaxLimPct value that represents 100.00% of
// nameplate under WMaxLimPctSF. Writes above this are clamped down.
const WMaxLimPctRawFull uint16 = 10000

// emitControls writes a Model 123 (Inverter Controls) section. Values reflect
// the current state read from the snapshot — what ALL inverters average to
// for the aggregate bank, or this one inverter's state for a per-inverter
// bank.
//
// The model is emit-only here; clients accept writes via the server's
// Modbus-write handlers in internal/sunspec/server (ControlsWriter).
func emitControls(bank *Bank, currentPct uint16, ena uint16, conn uint16) {
	bank.put16(ControlsModelID, ControlsBodyLen)

	// Timer/window fields (WinTms/RvrtTms/RmpTms) emit notImplU16 because
	// auto-revert and ramp behavior aren't implemented; per the SunSpec
	// spec the uint16 "not implemented" sentinel is 0xFFFF, not 0.
	// APsystems' own port-502 server uses the same convention.
	bank.put16(notImplU16) // +0  Conn_WinTms
	bank.put16(notImplU16) // +1  Conn_RvrtTms
	bank.put16(conn)       // +2  Conn              — 1=CONNECT, 0=DISCONNECT
	bank.put16(currentPct) // +3  WMaxLimPct        — current cap %
	bank.put16(notImplU16) // +4  WMaxLimPct_WinTms
	bank.put16(notImplU16) // +5  WMaxLimPct_RvrtTms
	bank.put16(notImplU16) // +6  WMaxLimPct_RmpTms
	bank.put16(ena)        // +7  WMaxLim_Ena       — 1=ENABLED, 0=DISABLED

	bank.put16(notImplS16) // +8  OutPFSet          — APsystems has no scalar PF setpoint
	bank.put16(notImplU16) // +9  OutPFSet_WinTms
	bank.put16(notImplU16) // +10 OutPFSet_RvrtTms
	bank.put16(notImplU16) // +11 OutPFSet_RmpTms
	bank.put16(0)          // +12 OutPFSet_Ena      — DISABLED (enum16, 0=DISABLED is a real value)

	bank.put16(notImplS16) // +13 VArWMaxPct        — APsystems is real-power only
	bank.put16(notImplS16) // +14 VArMaxPct
	bank.put16(notImplS16) // +15 VArAvalPct
	bank.put16(notImplU16) // +16 VArPct_WinTms
	bank.put16(notImplU16) // +17 VArPct_RvrtTms
	bank.put16(notImplU16) // +18 VArPct_RmpTms
	bank.put16(0)          // +19 VArPct_Mod        — NONE (enum16)
	bank.put16(0)          // +20 VArPct_Ena        — DISABLED (enum16)

	bank.put16(scaleFactor(WMaxLimPctSF)) // +21 WMaxLimPct_SF — raw × 1e-2 = %
	bank.put16(scaleFactor(0))            // +22 OutPFSet_SF
	bank.put16(scaleFactor(0))            // +23 VArPct_SF
}

// invCapPct converts a per-inverter panel-cap state to a SunSpec
// WMaxLimPct raw value: the effective output ceiling (panel_cap ×
// panel_count) expressed as a fraction of the inverter's AC nameplate,
// scaled by 10000 to match the advertised WMaxLimPctSF=-2.
//
// Return range is 0..10000. Per-panel cap above MaxPanelLimitW is
// treated as uncapped (returns 10000). Inverters with no known nameplate
// fall back to panel-cap-as-fraction of MaxPanelLimitW.
func invCapPct(inv source.Inverter) int {
	cap := inv.LimitedPowerW
	if cap == 0 {
		cap = source.MaxPanelLimitW
	}
	full := int(WMaxLimPctRawFull)
	nameplate := inv.NameplateW()
	if nameplate <= 0 {
		p := (cap * full) / source.MaxPanelLimitW
		if p > full {
			p = full
		}
		return p
	}
	effective := cap * inv.PanelCount()
	if effective > nameplate {
		effective = nameplate
	}
	p := (effective * full) / nameplate
	if p > full {
		p = full
	}
	return p
}

// AggregateControlsState computes the current Model 123 values for the
// aggregate bank (uid 1) from the snapshot.
//
// WMaxLim_Pct: average across inverters of (effective output cap / nameplate),
// scaled by WMaxLimPctSF=-2 (raw 0..10000 = 0..100.00%).
// WMaxLim_Ena: 1 if any inverter has limitedpower < max, else 0.
// Conn: 1 if any inverter online, 0 if all offline.
func AggregateControlsState(s source.Snapshot) (pct uint16, ena uint16, conn uint16) {
	if len(s.Inverters) == 0 {
		return WMaxLimPctRawFull, 0, 0
	}
	totalPct := 0
	limited := 0
	online := 0
	for _, inv := range s.Inverters {
		totalPct += invCapPct(inv)
		cap := inv.LimitedPowerW
		if cap == 0 {
			cap = source.MaxPanelLimitW
		}
		if cap < source.MaxPanelLimitW {
			limited++
		}
		if inv.Online {
			online++
		}
	}
	avgPct := totalPct / len(s.Inverters)
	pct = uint16(avgPct)
	if limited > 0 {
		ena = 1
	}
	if online > 0 {
		conn = 1
	}
	return
}

// PerInverterControlsState computes Model 123 values for one inverter
// (uid 2..N+1).
func PerInverterControlsState(inv source.Inverter) (pct uint16, ena uint16, conn uint16) {
	pct = uint16(invCapPct(inv))
	cap := inv.LimitedPowerW
	if cap == 0 {
		cap = source.MaxPanelLimitW
	}
	if cap < source.MaxPanelLimitW {
		ena = 1
	}
	if inv.Online {
		conn = 1
	}
	return
}

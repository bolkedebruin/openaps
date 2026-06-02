// Package sunspec encodes a SunSpec register bank for an APsystems ECU.
//
// We export:
//
//	Common Model        (ID  1, length 66)
//	Inverter Model      (101/102/103 — int+SF, length 50; phase-mode picks one)
//	Inverter Float      (111/112/113 — float32, length 60; matching phase model)
//	DC Data Float       (ID 114, length 48 — 8-channel DCV/DCA/DCW arrays)
//	Multi-MPPT Model    (ID 160, length 8 + 20·N, optional)
//	End marker          (0xFFFF, length 0)
//
// Layout starts at base register BaseRegister (default 40000) per the SunSpec
// specification. Each register is 16 bits, big-endian on the wire.
package sunspec

import (
	"encoding/binary"

	"github.com/bolke/ecu-sunspec/internal/source"
)

// BaseRegister is where SunSpec discovery starts.
//
// Victron's Fronius detector polls 40000 (0-indexed); some clients use 40001
// (1-indexed). We respond at both because Modbus PDU offsets are 16-bit and
// callers index either way in practice.
const BaseRegister = 40000

// DefaultManufacturer is the SunSpec Common Model `Mn` value.
//
// Empirically Venus' Fronius/SunSpec driver accepts arbitrary manufacturer
// strings (it doesn't gate on the literal "Fronius" the way some sources
// claim), so we use the truthful vendor name.
const DefaultManufacturer = "APsystems"

// DefaultModelName is the SunSpec Common Model `Md` value. Venus combines
// Mn + Md for display ("APsystems ECU-R-Pro"), so don't repeat the vendor
// in this string.
const DefaultModelName = "ECU-R-Pro"

// PhaseMode controls which SunSpec inverter model the encoder emits.
//
// SunSpec defines three integer-with-SF inverter models — 101 single-phase,
// 102 split-phase, 103 three-phase — with identical 50-register layouts.
// The chosen model ID tells the client whether AphB/PhVphB and AphC/PhVphC
// are valid; we mark unused phase registers as 0x8000 (not implemented).
type PhaseMode int

const (
	PhaseAuto PhaseMode = iota
	PhaseSingle
	PhaseSplit
	PhaseThree
)

func (p PhaseMode) modelID() uint16 {
	switch p {
	case PhaseSplit:
		return InverterModelSplitPhase
	case PhaseThree:
		return InverterModelThreePhase
	default:
		return InverterModelSinglePhase
	}
}

func (p PhaseMode) phasesUsed() int {
	switch p {
	case PhaseSplit:
		return 2
	case PhaseThree:
		return 3
	default:
		return 1
	}
}

// SunSpec model IDs (standard models + our vendor extension).
//
// Standard IDs come from the SunSpec specification. The vendor ID lives in
// the reserved 64000–64999 range; clients that don't recognize it walk past
// using the L header.
const (
	CommonModelID            uint16 = 1
	InverterModelSinglePhase uint16 = 101
	InverterModelSplitPhase  uint16 = 102
	InverterModelThreePhase  uint16 = 103
	NameplateModelID         uint16 = 120
	BasicSettingsModelID     uint16 = 121
	MultiMPPTModelID         uint16 = 160
	EndModelID               uint16 = 0xFFFF
)

// Standard SunSpec body lengths (registers, excluding ID + L).
const (
	CommonModelBodyLen     uint16 = 66
	InverterModelBodyLen   uint16 = 50
	NameplateBodyLen       uint16 = 26
	BasicSettingsBodyLen   uint16 = 30
	MultiMPPTFixedBlockLen uint16 = 8  // body length contribution of the Multi-MPPT header
	MultiMPPTPerModuleLen  uint16 = 20 // 1 module
)

// Vendor model layout (64202).
const (
	VendorFixedBlockLen  uint16 = 16
	VendorPerInverterLen uint16 = 10
)

// SunSpec "not implemented" sentinels.
//
// Per the SunSpec spec, the value used to flag an unimplemented field depends
// on the field's type, not the field's role. Wrong sentinels read as real
// values: a uint16 PhVphB written as 0x8000 reads as 32768 V, and Venus then
// mis-renders the device as multi-phase.
const (
	notImplU16 uint16 = 0xFFFF // uint16 fields (V, A, Hz)
	notImplS16 uint16 = 0x8000 // int16 fields (W, VA, VAr, PF, DCW, Tmp*)
	notImplU32        = uint32(0xFFFFFFFF)
	notImplS32        = uint32(0x80000000)
)

// Operating states (SunSpec Inverter Model "St" enum).
const (
	StOff     uint16 = 1
	StSleep   uint16 = 2
	StStart   uint16 = 3
	StMPPT    uint16 = 4
	StThrottl uint16 = 5
	StShutDwn uint16 = 6
	StFault   uint16 = 7
	StStandby uint16 = 8
)

// aggregateOperatingState maps online count + system power to the St
// enum used by Models 101/103/111/113. The online count is the
// authoritative gate: SunSpec semantics treat StSleep as "off but will
// start producing again", which is exactly what microinverters do at
// night. Power is only meaningful when at least one inverter is
// online; otherwise it's a stale historical sample.
func aggregateOperatingState(onlineCount int, systemPowerW int32) uint16 {
	if onlineCount == 0 {
		return StSleep
	}
	if systemPowerW > 0 {
		return StMPPT
	}
	return StStandby
}

// inverterOperatingState is the per-inverter equivalent of
// aggregateOperatingState. An offline inverter is StSleep, an online
// inverter producing power is StMPPT, and an online inverter idle is
// StStandby.
func inverterOperatingState(online bool, acPowerW int) uint16 {
	if !online {
		return StSleep
	}
	if acPowerW > 0 {
		return StMPPT
	}
	return StStandby
}

// Bank is a flat slice of holding registers starting at BaseRegister.
// Length is the total register count (Common 70 + Inverter 52 + End 2 = 124).
type Bank struct {
	Regs []uint16
	Base uint16
}

// Options tunes encoder choices that aren't derivable from the snapshot.
type Options struct {
	Manufacturer   string    // defaults to DefaultManufacturer
	ModelName      string    // Common Model `Md`; defaults to DefaultModelName
	SerialOverride string    // if set, wins over snapshot.ECUID (use to force a fresh device id)
	SerialFallback string    // used only if SerialOverride is empty AND snapshot.ECUID is empty
	Phase          PhaseMode // defaults to PhaseAuto (detected from snapshot)
	DisableMPPT    bool      // skip Multi-MPPT Model 160; default emits it when inverters are present

	// WriteRslt returns the async write status (rslt enum, request counter)
	// for SunSpec models that publish AdptCtlRslt / AdptCrvRslt fields
	// (Model 711 today). The unit ID is the Modbus address we're emitting
	// for (1 = aggregate, 2..N+1 = per-inverter); modelID is the SunSpec
	// model we're emitting. Returning (1, 0) for unknowns is the safe
	// default — it maps to COMPLETED, "no write in flight".
	//
	// nil → all writable models emit COMPLETED with reqCounter=0.
	WriteRslt func(uid uint8, modelID uint16) (rslt uint16, reqCounter uint16)
}

// Encode produces a Bank from a Snapshot.
func Encode(s source.Snapshot, opt Options) Bank {
	if opt.Manufacturer == "" {
		opt.Manufacturer = DefaultManufacturer
	}
	// Md priority: explicit --model-name flag → /etc/yuneng/model.conf →
	// hard-coded fallback. The conf-file value is the truthful name (e.g.
	// "ECU_R_PRO" or "ECU_C") so prefer it over the static default.
	if opt.ModelName == "" {
		opt.ModelName = strOr(s.Model, DefaultModelName)
	}
	if opt.Phase == PhaseAuto {
		opt.Phase = detectPhase(s)
	}

	bank := Bank{
		Regs: make([]uint16, 0, 128),
		Base: BaseRegister,
	}

	// Magic: "SunS" in two registers, big-endian.
	bank.put16(0x5375, 0x6E53)

	// --- Common Model (ID 1) ---
	bank.put16(CommonModelID, CommonModelBodyLen)
	bank.putString(opt.Manufacturer, 16)
	bank.putString(opt.ModelName, 16)
	bank.putString("", 8) // Options
	bank.putString(strOr(s.Firmware, "0"), 8)
	bank.putString(serialFor(s, opt), 16)
	bank.put16(1) // DA  (device address)
	bank.put16(0) // Pad

	// --- Inverter Model 101/102/103 (int+SF, identical 50-register layout) ---
	bank.put16(opt.Phase.modelID(), InverterModelBodyLen)
	phases := opt.Phase.phasesUsed()

	totalA, perPhaseA := derivePhaseCurrents(s)
	bank.put16(totalA)
	bank.put16(
		phaseValueU16(perPhaseA[0], 0, phases),
		phaseValueU16(perPhaseA[1], 1, phases),
		phaseValueU16(perPhaseA[2], 2, phases),
	)
	bank.put16(scaleFactor(-1)) // ASF: 0.1 A

	// PPVphAB/BC/CA — line-line voltages: not measured.
	bank.put16(notImplU16, notImplU16, notImplU16)
	// PhVphA/B/C — line-neutral voltages, scaled ×1; pad unused phases.
	v := uint16(s.GridVoltageV + 0.5)
	bank.put16(
		phaseValueU16(v, 0, phases),
		phaseValueU16(v, 1, phases),
		phaseValueU16(v, 2, phases),
	)
	bank.put16(scaleFactor(0)) // VSF: 1 V

	// W (active power) — int16 signed, scaled ×1.
	bank.put16(uint16(int16(clampInt32(s.SystemPowerW, -32760, 32760))))
	bank.put16(scaleFactor(0))

	// Hz — uint16, scaled ×0.01.
	bank.put16(uint16(s.GridFrequencyHz*100 + 0.5))
	bank.put16(scaleFactor(-2))

	// VA/VAr/PF — int16, not measured.
	bank.put16(notImplS16)
	bank.put16(scaleFactor(0))
	bank.put16(notImplS16)
	bank.put16(scaleFactor(0))
	bank.put16(notImplS16)
	bank.put16(scaleFactor(0))

	// WH — lifetime acc32 in Wh.
	bank.putAcc32(s.LifetimeEnergyWh)
	bank.put16(scaleFactor(0))

	// DCA (uint16) / DCV (uint16) / DCW (int16) — APsystems doesn't expose
	// DC measurements, so leave all three unimplemented. Earlier we mirrored
	// AC power into DCW as a "PV input" stand-in, but some clients then
	// double-counted W and DCW; keeping DC fields clean is safer.
	bank.put16(notImplU16) // DCA
	bank.put16(scaleFactor(0))
	bank.put16(notImplU16) // DCV
	bank.put16(scaleFactor(0))
	bank.put16(notImplS16) // DCW
	bank.put16(scaleFactor(0))

	// TmpCab — int16 °C scaled ×1; secondary temperatures not exposed.
	bank.put16(uint16(int16(clampInt32(int32(s.MaxTemperatureC), -100, 200))))
	bank.put16(notImplS16) // TmpSnk
	bank.put16(notImplS16) // TmpTrns
	bank.put16(notImplS16) // TmpOt
	bank.put16(scaleFactor(0))

	// St — operating state. Online count is the gate, not power: at night
	// each_system_power's most recent row is stale (last sample logged
	// before sundown) so a SystemPowerW>0 read can persist long after the
	// micros went to sleep. Reporting MPPT in that window is wrong and
	// surfaces in Victron as "Running (MPPT)" overnight.
	st := aggregateOperatingState(s.InverterOnlineCount, s.SystemPowerW)
	bank.put16(st)
	bank.put16(0) // StVnd (vendor state)

	// Evt1: SunSpec-standard event flags. Sourced from typed inv-driver
	// faults when present (preferred) and OR'd with the legacy SQLite
	// EventBits projection so the broad SunSpec ecosystem sees complete
	// data during the hybrid window.
	// EvtVnd1: new typed-faults layout (see docs/EVENTS.md). Falls back
	// to the legacy raw bits at PHP-UI bit positions when no typed
	// faults are available for an inverter.
	// EvtVnd4 stays reserved.
	evt1, vnd1, vnd2, vnd3, vnd4 := eventOutputForAggregate(s)
	bank.putAcc32(uint64(evt1))       // Evt1 — standard SunSpec
	bank.putAcc32(uint64(notImplU32)) // Evt2 — reserved
	bank.putAcc32(uint64(vnd1))       // EvtVnd1
	bank.putAcc32(uint64(vnd2))       // EvtVnd2 (legacy fallback only)
	bank.putAcc32(uint64(vnd3))       // EvtVnd3 (legacy fallback only)
	bank.putAcc32(uint64(vnd4))       // EvtVnd4 — unused

	// --- Inverter Float Model 111/112/113 — same field set as 101/103, float32 encoding ---
	emitInverterFloat(&bank, s, opt.Phase)

	// --- Model 114 — DC Data float (per-channel) ---
	emitDCDataFloat(&bank, s)

	// --- Model 120 — Nameplate Ratings ---
	emitNameplate(&bank, s)

	// --- Model 121 — Basic Settings (read-only here) ---
	emitBasicSettings(&bank, s)

	// --- Model 123 — Inverter Controls (read/write) ---
	pct, ena, conn := AggregateControlsState(s)
	emitControls(&bank, pct, ena, conn)

	// --- Models 707/708/709/710 (DER Trip LV/HV/LF/HF) + 703 (Enter Service) + 711 (Freq Droop) ---
	// Aggregate bank: use the first inverter's protection params as a
	// representative profile. Per-inverter banks expose each inverter's
	// individual values. Empty params produce models with Ena=0 / ActPt=0.
	emitDERTripModels(&bank, aggregateProtection(s), s.GridVoltageV)
	emitEnterService(&bank, aggregateProtection(s), s.GridVoltageV)
	{
		rslt, req := lookupWriteRslt(opt, 1, FreqDroopModelID)
		emitFreqDroop(&bank, aggregateProtection(s), rslt, req)
	}
	emitFreqWattCurve(&bank, aggregateProtection(s))

	// --- Multi-MPPT Model 160 (one module per panel of every online inverter) ---
	if !opt.DisableMPPT {
		emitMultiMPPT(&bank, s)
	}

	// --- Vendor model 64202 — APsystems extras ---
	emitVendor(&bank, s)

	// --- End marker ---
	bank.put16(EndModelID, 0)

	return bank
}

// Bytes returns the wire-format big-endian bytes for the bank.
func (b Bank) Bytes() []byte {
	out := make([]byte, len(b.Regs)*2)
	for i, r := range b.Regs {
		binary.BigEndian.PutUint16(out[i*2:], r)
	}
	return out
}

// At returns the register at the given absolute address (40000-based) or 0 if
// out-of-range.
func (b Bank) At(addr uint16) uint16 {
	if addr < b.Base {
		return 0
	}
	idx := int(addr - b.Base)
	if idx >= len(b.Regs) {
		return 0
	}
	return b.Regs[idx]
}

// Slice returns up to count registers starting at addr. Out-of-range bytes are
// zero-filled.
func (b Bank) Slice(addr, count uint16) []uint16 {
	out := make([]uint16, count)
	for i := uint16(0); i < count; i++ {
		out[i] = b.At(addr + i)
	}
	return out
}

// Contains reports whether [addr, addr+count) lies fully inside the bank.
// Used to reject out-of-range reads with Modbus exception 02 (Illegal Data
// Address), which is what real SunSpec devices do — and what scanners use to
// discriminate one inverter family from another.
func (b Bank) Contains(addr, count uint16) bool {
	if count == 0 {
		return false
	}
	if addr < b.Base {
		return false
	}
	end := uint32(addr) + uint32(count)
	return end <= uint32(b.Base)+uint32(len(b.Regs))
}

// internal helpers

func (b *Bank) put16(vals ...uint16) {
	b.Regs = append(b.Regs, vals...)
}

func (b *Bank) putAcc32(v uint64) {
	hi := uint16(v >> 16)
	lo := uint16(v)
	b.Regs = append(b.Regs, hi, lo)
}

func (b *Bank) putString(s string, registerCount int) {
	bytes := registerCount * 2
	buf := make([]byte, bytes)
	copy(buf, s)
	for i := 0; i < bytes; i += 2 {
		b.Regs = append(b.Regs, uint16(buf[i])<<8|uint16(buf[i+1]))
	}
}

// scaleFactor encodes a signed int8 as the SunSpec sunssf register form.
func scaleFactor(sf int8) uint16 {
	return uint16(int16(sf))
}

func clampInt32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func strOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// serialFor implements the SN priority chain: explicit override > snapshot
// ECUID (from /etc/yuneng/ecuid.conf) > fallback. Letting the override beat
// the file lets users re-spawn the device under a fresh SN — handy when
// Venus's PV-inverter list won't let go of a stale entry.
func serialFor(s source.Snapshot, opt Options) string {
	if opt.SerialOverride != "" {
		return opt.SerialOverride
	}
	if s.ECUID != "" {
		return s.ECUID
	}
	return opt.SerialFallback
}

// phaseValueU16 returns v if phase index `idx` is active for `phasesUsed`,
// else the uint16 SunSpec "not implemented" sentinel (0xFFFF).
//
// Use this for V/A fields. int16 fields (W, VA, VAr, PF, DCW, Tmp*) use
// notImplS16 = 0x8000 — never mix the two: a 0x8000 written into a uint16
// field decodes as 32768 instead of "not implemented".
func phaseValueU16(v uint16, idx, phasesUsed int) uint16 {
	if idx < phasesUsed {
		return v
	}
	return notImplU16
}

// detectPhase examines the snapshot's per-inverter Phase field and returns
// the smallest model that fits. Inverters with phase=0 are folded onto phase 1.
func detectPhase(s source.Snapshot) PhaseMode {
	var seen [4]bool
	for _, inv := range s.Inverters {
		if !inv.Online {
			continue
		}
		ph := inv.Phase
		if ph < 1 || ph > 3 {
			ph = 1
		}
		seen[ph] = true
	}
	count := 0
	for i := 1; i <= 3; i++ {
		if seen[i] {
			count++
		}
	}
	switch count {
	case 0, 1:
		return PhaseSingle
	case 2:
		return PhaseSplit
	default:
		return PhaseThree
	}
}

// emitMultiMPPT appends a SunSpec Model 160 (Multi-MPPT) section to the bank.
//
// One module per **panel** (PV input channel) on every online microinverter.
// A DS3 contributes 2 modules, a QS1 contributes 4 — the count derives from
// the inverter type, never hardcoded.
//
// APsystems doesn't expose per-panel DC voltage/current — only per-panel
// power. We map:
//
//	DCW   ← per-panel power (W)
//	DCV   ← per-inverter AC voltage stand-in, shared across the inverter's panels
//	DCA   ← derived = panel_W / inverter_Vac, scale -1 (0.1 A)
//	Tmp   ← per-inverter cabinet temperature, shared
//	DCSt  ← MPPT(4) when panel power > 0, Off(1) otherwise
//	IDStr ← "<last8(UID)>/<A|B|C|D>"
func emitMultiMPPT(bank *Bank, s source.Snapshot) {
	type panelEntry struct {
		inv     source.Inverter
		channel int // 0..3
		powerW  int
	}
	var panels []panelEntry
	for _, inv := range s.Inverters {
		if !inv.Online {
			continue
		}
		ch := inv.PanelPowers()
		if len(ch) == 0 {
			continue
		}
		for i, p := range ch {
			panels = append(panels, panelEntry{inv: inv, channel: i, powerW: p})
		}
	}
	if len(panels) == 0 {
		return
	}
	n := uint16(len(panels))

	bodyLen := MultiMPPTFixedBlockLen + MultiMPPTPerModuleLen*n
	bank.put16(MultiMPPTModelID, bodyLen)

	// Fixed block (8 regs body)
	bank.put16(scaleFactor(-1))       // DCA_SF
	bank.put16(scaleFactor(0))        // DCV_SF
	bank.put16(scaleFactor(0))        // DCW_SF
	bank.put16(scaleFactor(0))        // DCWH_SF
	bank.putAcc32(uint64(notImplU32)) // Evt — global module events bitfield32, not-impl
	bank.put16(n)                     // N
	bank.put16(notImplU16)            // TmsPer

	// Tms is per-module timestamp. APsystems polls every inverter in a single
	// cycle, so every panel would carry the same value here — 10 identical
	// "Timestamp" sensors in HA. Emit the uint32 not-implemented sentinel
	// instead; clients that care about freshness can read the snapshot's
	// vendor-model fields or the SunSpec Common Model timestamp.
	const tmsNotImpl uint64 = uint64(notImplU32)

	for i, e := range panels {
		bank.put16(uint16(i + 1))
		bank.putString(panelLabel(e.inv, e.channel), 8)

		// DCA derived
		var dca uint16 = notImplU16
		if e.inv.ACVoltageV > 0 && e.powerW >= 0 {
			amps10 := float64(e.powerW) / float64(e.inv.ACVoltageV) * 10
			if amps10 < 65535 {
				dca = uint16(amps10 + 0.5)
			}
		}
		bank.put16(dca)

		// DCV — share inverter's AC voltage stand-in
		dcv := uint16(notImplU16)
		if e.inv.ACVoltageV > 0 && e.inv.ACVoltageV < 65535 {
			dcv = uint16(e.inv.ACVoltageV)
		}
		bank.put16(dcv)

		// DCW = panel power
		dcw := uint16(notImplU16)
		if e.powerW >= 0 && e.powerW < 65535 {
			dcw = uint16(e.powerW)
		}
		bank.put16(dcw)

		bank.putAcc32(0)          // DCWH — per-panel lifetime not tracked
		bank.putAcc32(tmsNotImpl) // Tms — same for all panels; expose as not-implemented

		// Tmp — share inverter cabinet temperature
		bank.put16(uint16(int16(clampInt32(int32(e.inv.TemperatureC), -100, 200))))

		// DCSt — per-panel state
		st := StOff
		if e.powerW > 0 {
			st = StMPPT
		}
		bank.put16(st)
		bank.putAcc32(uint64(notImplU32)) // DCEvt — bitfield32 not-impl
	}
}

// panelLabel returns "<last8(UID)>/<A|B|C|D>", padded by putString into 16
// bytes (8 registers).
func panelLabel(inv source.Inverter, channel int) string {
	uid := inv.UID
	if len(uid) > 6 {
		uid = uid[len(uid)-6:]
	}
	suffix := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	if channel >= 0 && channel < len(suffix) {
		return uid + "/" + suffix[channel]
	}
	return uid
}

// derivePhaseCurrents returns total A and per-phase A from the snapshot.
//
// APsystems doesn't measure AC current directly. We back-derive from W/V per
// phase. SunSpec wants amps × 10 (ASF=-1).
func derivePhaseCurrents(s source.Snapshot) (total uint16, perPhase [3]uint16) {
	if s.GridVoltageV <= 0 {
		return 0, perPhase
	}
	// Bucket inverters by phase from the id table; phase=0 => phase A.
	var phaseW [3]int
	for _, inv := range s.Inverters {
		if !inv.Online {
			continue
		}
		ph := inv.Phase - 1
		if ph < 0 || ph > 2 {
			ph = 0
		}
		phaseW[ph] += inv.ACPowerW
	}
	v := s.GridVoltageV
	var sum int
	for i := 0; i < 3; i++ {
		a := float64(phaseW[i]) / v // amps
		perPhase[i] = uint16(a*10 + 0.5)
		sum += phaseW[i]
	}
	total = uint16(float64(sum)/v*10 + 0.5)
	return total, perPhase
}

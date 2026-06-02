package sunspec

import (
	"math"

	"github.com/bolke/ecu-sunspec/internal/source"
)

// SunSpec inverter monitoring models that encode every measurement as float32
// instead of the int+SF pair used by 101/102/103. APsystems publishes these in
// their SunSpec spec (Rev 3.3, Jan 2025) alongside the integer models, and
// SunSpec clients that prefer floats (HA's home-assistant-sunspec, the SunSpec
// Test Tools) discover them at the same base register and read whichever they
// like.
//
// Body layout (60 regs) is the same field set as 101/103 with float32 in place
// of (value, scale-factor) pairs:
//
//	+0   A (float32)
//	+2   AphA, +4 AphB, +6 AphC
//	+8   PPVphAB, +10 PPVphBC, +12 PPVphCA
//	+14  PhVphA, +16 PhVphB, +18 PhVphC
//	+20  W
//	+22  Hz
//	+24  VA
//	+26  VAr
//	+28  PF
//	+30  WH
//	+32  DCA
//	+34  DCV
//	+36  DCW
//	+38  TmpCab
//	+40  TmpSnk, +42 TmpTrns, +44 TmpOt
//	+46  St (enum16)
//	+47  StVnd (enum16)
//	+48  Evt1 (bitfield32)
//	+50  Evt2
//	+52  EvtVnd1
//	+54  EvtVnd2
//	+56  EvtVnd3
//	+58  EvtVnd4
const (
	InverterModelSinglePhaseFloat uint16 = 111
	InverterModelSplitPhaseFloat  uint16 = 112
	InverterModelThreePhaseFloat  uint16 = 113
	InverterModelFloatBodyLen     uint16 = 60
)

// DC Data (float). 8 channels × (V, A, W), all float32. Per the APsystems
// SunSpec spec only the first 1–2 channels are guaranteed populated (DS3 = 2
// MPPT, QS1 = 4 MPPT). Unused channels emit the float "not implemented"
// sentinel (NaN).
const (
	DCDataFloatModelID  uint16 = 114
	DCDataFloatBodyLen  uint16 = 48
	DCDataFloatChannels        = 8
)

// notImplFloat32 is the IEEE-754 quiet-NaN payload SunSpec uses to flag an
// unimplemented float field. Real measurements are never NaN, so a NaN read is
// unambiguous.
const notImplFloat32 uint32 = 0x7FC00000

func (b *Bank) putFloat32(v float32) {
	bits := math.Float32bits(v)
	b.Regs = append(b.Regs, uint16(bits>>16), uint16(bits))
}

func (b *Bank) putFloat32NotImpl() {
	b.Regs = append(b.Regs, uint16(notImplFloat32>>16), uint16(notImplFloat32&0xFFFF))
}

// putFloat32Phase emits v for active phase indices, NaN otherwise. Same role
// as phaseValueU16 for the int+SF inverter models.
func (b *Bank) putFloat32Phase(v float32, idx, phasesUsed int) {
	if idx < phasesUsed {
		b.putFloat32(v)
		return
	}
	b.putFloat32NotImpl()
}

// emitInverterFloat appends a Model 111/112/113 (float) section for the
// aggregate snapshot. PhaseMode picks the model ID; unused phase fields are
// NaN.
func emitInverterFloat(bank *Bank, s source.Snapshot, phase PhaseMode) {
	bank.put16(phase.modelID()+10, InverterModelFloatBodyLen) // 101→111, 102→112, 103→113
	phases := phase.phasesUsed()

	// Per-phase amps from W/V split.
	totalA, perPhaseA := derivePhaseCurrents(s)
	bank.putFloat32(float32(totalA) / 10) // ASF in int model is -1, here we emit raw amps
	bank.putFloat32Phase(float32(perPhaseA[0])/10, 0, phases)
	bank.putFloat32Phase(float32(perPhaseA[1])/10, 1, phases)
	bank.putFloat32Phase(float32(perPhaseA[2])/10, 2, phases)

	// PPVphAB/BC/CA — line-line voltages: not measured.
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()

	// PhVphA/B/C — line-neutral.
	v := float32(s.GridVoltageV)
	bank.putFloat32Phase(v, 0, phases)
	bank.putFloat32Phase(v, 1, phases)
	bank.putFloat32Phase(v, 2, phases)

	// W
	bank.putFloat32(float32(s.SystemPowerW))

	// Hz
	bank.putFloat32(float32(s.GridFrequencyHz))

	// VA / VAr / PF — not measured.
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()

	// WH — lifetime energy in Wh.
	bank.putFloat32(float32(s.LifetimeEnergyWh))

	// DCA / DCV / DCW — APsystems doesn't expose system-wide DC; per-panel
	// DC lives in Models 160 and 114. Leave system DC fields as NaN.
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()

	// TmpCab — peak inverter temperature.
	bank.putFloat32(float32(s.MaxTemperatureC))
	bank.putFloat32NotImpl() // TmpSnk
	bank.putFloat32NotImpl() // TmpTrns
	bank.putFloat32NotImpl() // TmpOt

	// St — operating state. Same gating as Model 101 — see
	// aggregateOperatingState for the rationale.
	st := aggregateOperatingState(s.InverterOnlineCount, s.SystemPowerW)
	bank.put16(st)
	bank.put16(0) // StVnd

	// Evt1 / Evt2 / EvtVnd1..4 — same mapping as Model 101/103.
	evt1, vnd1, vnd2, vnd3, vnd4 := eventOutputForAggregate(s)
	bank.putAcc32(uint64(evt1))
	bank.putAcc32(uint64(notImplU32))
	bank.putAcc32(uint64(vnd1))
	bank.putAcc32(uint64(vnd2))
	bank.putAcc32(uint64(vnd3))
	bank.putAcc32(uint64(vnd4))
}

// emitInverterFloatPerInverter appends a Model 111 (single-phase float)
// section for one microinverter.
func emitInverterFloatPerInverter(bank *Bank, inv source.Inverter) {
	bank.put16(InverterModelSinglePhaseFloat, InverterModelFloatBodyLen)

	var a float32
	if inv.ACVoltageV > 0 && inv.ACPowerW >= 0 {
		a = float32(inv.ACPowerW) / float32(inv.ACVoltageV)
	}
	bank.putFloat32(a)
	bank.putFloat32(a)       // AphA — single-phase
	bank.putFloat32NotImpl() // AphB
	bank.putFloat32NotImpl() // AphC

	// Line-line voltages — not measured.
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()

	v := float32(inv.ACVoltageV)
	bank.putFloat32(v)       // PhVphA
	bank.putFloat32NotImpl() // PhVphB
	bank.putFloat32NotImpl() // PhVphC

	bank.putFloat32(float32(inv.ACPowerW))

	hz := float32(inv.FrequencyHz)
	if hz > 0 {
		bank.putFloat32(hz)
	} else {
		bank.putFloat32NotImpl()
	}

	// VA/VAr/PF — not measured.
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()

	// WH — per-inverter lifetime not tracked.
	bank.putFloat32(0)

	// DC fields — see Model 114 for per-panel DC; system DC at Model 111 stays NaN.
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()
	bank.putFloat32NotImpl()

	bank.putFloat32(float32(inv.TemperatureC))
	bank.putFloat32NotImpl() // TmpSnk
	bank.putFloat32NotImpl() // TmpTrns
	bank.putFloat32NotImpl() // TmpOt

	bank.put16(inverterOperatingState(inv.Online, inv.ACPowerW))
	bank.put16(0) // StVnd

	evt1, vnd1, vnd2, vnd3, vnd4 := eventOutputForInverter(inv)
	bank.putAcc32(uint64(evt1))
	bank.putAcc32(uint64(notImplU32))
	bank.putAcc32(uint64(vnd1))
	bank.putAcc32(uint64(vnd2))
	bank.putAcc32(uint64(vnd3))
	bank.putAcc32(uint64(vnd4))
}

// emitDCDataFloat appends a Model 114 (DC Data float) section for the
// aggregate snapshot. The 8-channel array is filled by walking online
// inverters and assigning their panels to consecutive channels until the
// 8-channel array is full; remaining channels are NaN. APsystems doesn't
// publish per-panel DC voltage or current — we derive DCV from the inverter's
// AC voltage stand-in and DCA from W/V (same approach as Model 160).
//
// DCW carries true per-panel power.
func emitDCDataFloat(bank *Bank, s source.Snapshot) {
	bank.put16(DCDataFloatModelID, DCDataFloatBodyLen)

	type ch struct{ dcv, dca, dcw float32 }
	channels := make([]ch, 0, DCDataFloatChannels)
	for _, inv := range s.Inverters {
		if !inv.Online {
			continue
		}
		powers := inv.PanelPowers()
		for _, p := range powers {
			if len(channels) >= DCDataFloatChannels {
				break
			}
			var dcv, dca float32
			if inv.ACVoltageV > 0 {
				dcv = float32(inv.ACVoltageV)
				dca = float32(p) / dcv
			}
			channels = append(channels, ch{dcv: dcv, dca: dca, dcw: float32(p)})
		}
		if len(channels) >= DCDataFloatChannels {
			break
		}
	}

	// DCV1..DCV8
	for i := 0; i < DCDataFloatChannels; i++ {
		if i < len(channels) {
			bank.putFloat32(channels[i].dcv)
		} else {
			bank.putFloat32NotImpl()
		}
	}
	// DCA1..DCA8
	for i := 0; i < DCDataFloatChannels; i++ {
		if i < len(channels) {
			bank.putFloat32(channels[i].dca)
		} else {
			bank.putFloat32NotImpl()
		}
	}
	// DCW1..DCW8
	for i := 0; i < DCDataFloatChannels; i++ {
		if i < len(channels) {
			bank.putFloat32(channels[i].dcw)
		} else {
			bank.putFloat32NotImpl()
		}
	}
}

// emitDCDataFloatPerInverter appends a Model 114 section for one inverter.
// Channels 1..N (where N = inverter's panel count) carry the panels; the rest
// are NaN.
func emitDCDataFloatPerInverter(bank *Bank, inv source.Inverter) {
	bank.put16(DCDataFloatModelID, DCDataFloatBodyLen)

	powers := inv.PanelPowers()
	dcv := float32(inv.ACVoltageV)
	hasDCV := dcv > 0

	// DCV1..8
	for i := 0; i < DCDataFloatChannels; i++ {
		if i < len(powers) && hasDCV {
			bank.putFloat32(dcv)
		} else {
			bank.putFloat32NotImpl()
		}
	}
	// DCA1..8 — derived
	for i := 0; i < DCDataFloatChannels; i++ {
		if i < len(powers) && hasDCV {
			bank.putFloat32(float32(powers[i]) / dcv)
		} else {
			bank.putFloat32NotImpl()
		}
	}
	// DCW1..8 — actual
	for i := 0; i < DCDataFloatChannels; i++ {
		if i < len(powers) {
			bank.putFloat32(float32(powers[i]))
		} else {
			bank.putFloat32NotImpl()
		}
	}
}

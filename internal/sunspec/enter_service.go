package sunspec

import (
	"github.com/bolke/ecu-sunspec/internal/source"
)

// SunSpec Model 703 — Enter Service.
//
// Defines the post-trip reconnect window: the V/Hz band the grid must hold
// for ESDlyTms seconds before the inverter rejoins. Maps directly to the
// APsystems "reconnection" parameters in protection_parameters60code:
//
//	ESVHi    ← BO (Reconnection AC voltage high)        % VNom, sf=V_SF
//	ESVLo    ← BN (Reconnection AC voltage low)         % VNom, sf=V_SF
//	ESHzHi   ← BQ (Reconnection AC frequency high)      Hz,    sf=Hz_SF
//	ESHzLo   ← BP (Reconnection AC frequency low)       Hz,    sf=Hz_SF
//	ESDlyTms ← AG (Grid recovery time)                  Secs   (uint32)
//	ESRndTms = 0 (no randomized reconnect on this firmware)
//	ESRmpTms = 0 (no soft-start ramp time exposed)
//	ESDlyRemTms = 0xFFFFFFFF (not implemented — no live remaining-delay readout)
//
// Total length: 17 regs body + ID + L = 19 regs.
//
//	Header:  ID, L=17, ES enum (1), ESVHi (1), ESVLo (1), ESHzHi (2),
//	         ESHzLo (2), ESDlyTms (2), ESRndTms (2), ESRmpTms (2),
//	         ESDlyRemTms (2), V_SF (1), Hz_SF (1)
//
// pysunspec2 confirms L=17 (six uint32 fields × 2 regs each + scalar fields).

const (
	EnterServiceModelID uint16 = 703
	EnterServiceBodyLen uint16 = 17
	enterServiceVSF     int8   = -2
	enterServiceHzSF    int8   = -2
)

// emitEnterService writes Model 703 sourced from per-inverter protection params.
//
// vNom is the nominal voltage used to convert BN/BO from absolute V to %VNom.
// If the protection params don't include the BN/BO/BP/BQ/AG codes, the model
// emits with ES=DISABLED and zeros — discoverable but inactive.
func emitEnterService(bank *Bank, p source.ProtectionParams, vNom float64) {
	if vNom <= 0 {
		vNom = derTripVDefault
	}
	bank.put16(EnterServiceModelID, EnterServiceBodyLen)

	// ES — service-enable enum: 1 = ENABLED if AG (reconnect time) is configured.
	es := uint16(0)
	if p.Has["AG"] {
		es = 1
	}
	bank.put16(es)

	// ESVHi / ESVLo (uint16, % VNom × 100)
	bank.put16(pctOfVNom(p.ReconnVHi, vNom))
	bank.put16(pctOfVNom(p.ReconnVLow, vNom))

	// ESHzHi / ESHzLo (uint32 Hz, sf=Hz_SF=-2)
	bank.putAcc32(uint64(hzCenti(p.ReconnFHi)))
	bank.putAcc32(uint64(hzCenti(p.ReconnFLow)))

	// ESDlyTms — grid recovery time, seconds (uint32, no scale factor)
	bank.putAcc32(uint64(uint32(p.ReconnectS + 0.5)))

	// ESRndTms — randomized reconnect window. Not exposed by APsystems firmware.
	bank.putAcc32(0)
	// ESRmpTms — soft-start ramp. Not exposed.
	bank.putAcc32(0)
	// ESDlyRemTms — countdown of remaining delay. Not implemented (no live readout).
	bank.putAcc32(uint64(notImplU32))

	// Scale factors
	bank.put16(scaleFactor(enterServiceVSF))
	bank.put16(scaleFactor(enterServiceHzSF))
}

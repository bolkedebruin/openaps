package sunspec

import (
	"github.com/bolke/ecu-sunspec/internal/source"
)

// SunSpec Model 711 — DER Frequency Droop (DERFreqDroop).
//
// Defines the frequency-droop curve the inverter applies for active-power
// curtailment in response to over- or under-frequency events. This is the
// core "AC-coupled frequency-shift" mechanism for Victron MultiPlus / Quattro
// installations: the Multi pushes frequency upward (e.g. 50.2 → 51.5 Hz),
// the inverter follows the droop curve and curtails proportionally instead
// of trip-disconnecting.
//
// APsystems exposes the OF-side droop via the 2-letter codes
// CC/DC/DD/CV in protection_parameters60code:
//
//	DC = HzStart  (curve start, e.g. 50.2 Hz — curtailment begins here)
//	CC = HzEnd    (curve end,   e.g. 52.0 Hz — output forced to 0 here)
//	DD = Slope    (%P/Hz)
//	CV = Mode     (13 = AS/NZS, 14 = other regions, 15 = disabled)
//
// The UF-side droop has separate Under_frequency_Watt_* parameters in the
// firmware's symbol table but wasn't part of the gridProfile JSON observed
// on this firmware build. Surfaced as not-implemented sentinels until a
// UF-side mapping is added.
//
// Model 711 layout (NCtl=1, 22-reg body, 24 total):
//
//	Header (12 regs):
//	  ID, L=22, Ena enum16 (1), AdptCtlReq uint16 (1), AdptCtlRslt enum16 (1),
//	  NCtl=1 uint16 (1), RvrtTms uint32 (2), RvrtRem uint32 (2), RvrtCtl uint16 (1),
//	  Db_SF, K_SF, RspTms_SF (3 × sunssf = 3)
//
//	Per-Ctl (10 regs):
//	  DbOf uint32 (2, sf=Db_SF, Hz), DbUf uint32 (2, sf=Db_SF, Hz),
//	  KOf uint16 (1, sf=K_SF, %P/Hz), KUf uint16 (1, sf=K_SF, %P/Hz),
//	  RspTms uint32 (2, sf=RspTms_SF, sec), PMin int16 (1, %), ReadOnly enum16 (1)
//
// Scale factors:
//   Db_SF     = -2  (centi-Hertz: 0.2 Hz → 20)
//   K_SF      = -1  (deci-percent: 16.7 %P/Hz → 167)
//   RspTms_SF = 0   (whole seconds)

const (
	FreqDroopModelID  uint16 = 711
	FreqDroopBodyLen  uint16 = 22
	freqDroopBodyLen         = FreqDroopBodyLen // alias for legacy callers
	freqDroopNCtl     uint16 = 1
	freqDroopDbSF     int8   = -2 // centi-Hz
	freqDroopKSF      int8   = -1 // deci-percent
	freqDroopRspTmsSF int8   = 0  // seconds

	// AdptCtlRslt enum values per the SunSpec model definition.
	freqDroopRsltInProgress uint16 = 0
	freqDroopRsltCompleted  uint16 = 1
	freqDroopRsltFailed     uint16 = 2

	// ReadOnly enum: RW=0, R=1.
	freqDroopReadOnlyRW uint16 = 0
	freqDroopReadOnlyR  uint16 = 1
)

// emitFreqDroop emits Model 711 sourced from per-inverter freq-droop params.
// Empty / unknown fields produce 0 / not-implemented sentinels — the model
// is still discoverable so SunSpec scanners can find it; values populate
// once a profile that exercises the OF-droop codes is applied.
//
// rslt and reqCounter are the AdptCtlRslt / AdptCtlReq values to publish —
// supplied by the server's WriteTracker so they reflect the actual
// IN_PROGRESS / COMPLETED / FAILED state of the most recent write.
func emitFreqDroop(bank *Bank, p source.ProtectionParams, rslt uint16, reqCounter uint16) {
	bank.put16(FreqDroopModelID, freqDroopBodyLen)

	// Ena: 1 if APsystems' droop mode is in an active enum value (13 or 14),
	// 0 if disabled (15) or unknown.
	ena := uint16(0)
	switch int(p.OFDroopMode) {
	case 13, 14:
		ena = 1
	}
	bank.put16(ena)

	// AdptCtlReq: echoes the request counter the server has tracked for
	// this (uid, modelID), so clients can correlate "did my latest write
	// land yet" with their own posted counter.
	bank.put16(reqCounter)

	// AdptCtlRslt: server-published async result of the last write request.
	bank.put16(rslt)

	// NCtl
	bank.put16(freqDroopNCtl)

	// RvrtTms: not supported (we don't implement timed-revert)
	bank.putAcc32(0)
	// RvrtRem: not implemented
	bank.putAcc32(uint64(notImplU32))
	// RvrtCtl: not used
	bank.put16(0)

	// Scale factors
	bank.put16(scaleFactor(freqDroopDbSF))
	bank.put16(scaleFactor(freqDroopKSF))
	bank.put16(scaleFactor(freqDroopRspTmsSF))

	// --- single Ctl entry ---

	// DbOf (over-freq deadband from 50 Hz nominal). DC = OFDroopStart in
	// absolute Hz; convert to deadband-from-nominal in cHz.
	dbOf := uint32(0)
	if p.Has["DC"] && p.OFDroopStart > 50.0 {
		dbOf = uint32((p.OFDroopStart-50.0)*100.0 + 0.5)
	}
	bank.putAcc32(uint64(dbOf))

	// DbUf (under-freq deadband from 50 Hz nominal). UF-side not yet mapped.
	bank.putAcc32(uint64(notImplU32))

	// KOf (slope, %P/Hz, sf=-1). DD is the raw %P/Hz; encode with the
	// scale factor (multiply by 10).
	kOf := uint16(0)
	if p.Has["DD"] && p.OFDroopSlope > 0 {
		v := p.OFDroopSlope*10.0 + 0.5
		if v > 65535 {
			v = 65535
		}
		kOf = uint16(v)
	}
	bank.put16(kOf)

	// KUf: not implemented.
	bank.put16(notImplU16)

	// RspTms (seconds): not exposed by APsystems via these codes; emit 0.
	bank.putAcc32(0)

	// PMin (% Pmax to taper to): 0 means "all the way to zero output".
	bank.put16(uint16(int16(0)))

	// ReadOnly: writable for our purposes.
	bank.put16(freqDroopReadOnlyRW)
}

// HzStopFromDroopEnd computes the SunSpec-style HzEnd value (when the curve
// reaches zero output) from the APsystems CC code value. Helper for
// cross-parameter checks ("HzEnd must sit below OF1 trip with margin").
func HzStopFromDroopEnd(p source.ProtectionParams) (float64, bool) {
	if !p.Has["CC"] {
		return 0, false
	}
	return p.OFDroopEnd, true
}

// lookupWriteRslt returns (rslt, reqCounter) for (uid, modelID), or
// (COMPLETED, 0) when no tracker is configured. Centralizes the nil-check
// on Options.WriteRslt so emit functions stay clean.
func lookupWriteRslt(opt Options, uid uint8, modelID uint16) (uint16, uint16) {
	if opt.WriteRslt == nil {
		return freqDroopRsltCompleted, 0
	}
	return opt.WriteRslt(uid, modelID)
}

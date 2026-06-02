package sunspec

import (
	"github.com/bolkedebruin/openaps/internal/sunspec/source"
)

// SunSpec Model 134 — Curve-Based Frequency-Watt (Freq-Watt Crv).
//
// Spec: https://github.com/sunspec/models/blob/master/json/model_134.json
//
// Defines a frequency-watt curve as up to NPt (Hz, %WRef) points per
// curve, with up to NCrv selectable curves. Complements Model 711
// (parametric droop) by exposing the under-frequency side and the
// curve-low / curve-high endpoints which 711 cannot express. Maps
// onto the APsystems firmware's 4-threshold freq-watt design:
//
//	Hz1 = DH (Under_Frequency_Watt_Low_set,  W=0%)
//	Hz2 = DI (Under_Frequency_Watt_High_set, W=100%)
//	Hz3 = CB (Over_frequency_Watt_Low_set,   W=100%)
//	Hz4 = CC (Over_frequency_Watt_High_set,  W=0%)
//
// W% values are intrinsic to the curve role (the inverter holds full
// power inside [DI, CB] and ramps to zero at DH and CC), so we emit
// 0/100/100/0 fixed and accept only Hz writes.
//
// Model 134 layout (NCrv=1, NPt=4 active of 20 max):
//
//	Header (10 regs body, 12 with ID + L):
//	  ID, L, ActCrv, ModEna, WinTms, RvrtTms, RmpTms, NCrv, NPt,
//	  Hz_SF, W_SF, RmpIncDec_SF
//
//	Curve (58 regs body per curve):
//	  ActPt (1) + 20×(Hz uint16 + W int16) (40)
//	  + CrvNam string[8] (8)
//	  + RmpPT1Tms (1) + RmpDecTmm (1) + RmpIncTmm (1) + RmpRsUp (1)
//	  + SnptW bitfield16 (1) + WRef (1) + WRefStrHz (1) + WRefStopHz (1)
//	  + ReadOnly enum16 (1)
//	  = 1 + 40 + 8 + 9 = 58 regs
//
// Body length total = 10 + 58 * NCrv = 68 (NCrv=1).
//
// Scale factors:
//	Hz_SF = -2  (centi-Hz: 50.20 Hz → wire 5020)
//	W_SF  =  0  (whole percent: 100 % → wire 100)
//	RmpIncDec_SF = 0 (we emit zero / unused for ramp fields)
//
// Trailing per-curve fields are emitted as zero / not-implemented:
// the APsystems firmware does not expose a snapshot/ramp mechanism via
// these codes, so the fields are mandatory layout-wise but carry no
// behavioural meaning here. ReadOnly = READWRITE so SunSpec clients can
// rewrite the points; WRef defaults to 100 (i.e. the curve is expressed
// in % of WMax, the SunSpec default).

const (
	FreqWattCurveModelID uint16 = 134

	// Per-curve point capacity in the SunSpec definition (max 20).
	freqWattCurveNPtMax uint16 = 20

	// We expose a 4-point curve (NPt=4 active of 20 declared); aligned
	// 1:1 with APsystems' DH/DI/CB/CC threshold quartet.
	freqWattCurveActPt uint16 = 4

	// Number of curves we publish. SunSpec recommends min. 4 but we
	// only have a single threshold set in firmware to back, so NCrv=1
	// keeps the bank concise.
	freqWattCurveNCrv uint16 = 1

	freqWattCurveHzSF        int8 = -2 // centi-Hz
	freqWattCurveWSF         int8 = 0  // whole-percent
	freqWattCurveRmpIncDecSF int8 = 0

	// Header is 10 regs of body (12 total incl. ID + L). Per-curve
	// block is 1 (ActPt) + 20*2 (Hz/W pairs) + 8 (CrvNam string) + 9
	// (trailing scalars incl. ReadOnly) = 58 regs.
	freqWattCurveHeaderBodyLen uint16 = 10
	freqWattCurvePerCurveLen   uint16 = 58

	// FreqWattCurveBodyLen is the total body length we emit (header +
	// NCrv curves). Server uses this to pin write-range checks.
	FreqWattCurveBodyLen uint16 = freqWattCurveHeaderBodyLen +
		freqWattCurveNCrv*freqWattCurvePerCurveLen

	// ReadOnly enum values per the model definition.
	freqWattCurveReadOnlyRW uint16 = 0
	freqWattCurveReadOnlyR  uint16 = 1

	// Default WRef percent baseline. The model expresses W in % of
	// WRef; we default WRef = 100 so the wire numbers read as % of
	// WMax directly.
	freqWattCurveDefaultWRef uint16 = 100
)

// Body offsets within Model 134 (i.e. relative to the first register
// AFTER ID + L). Header is 10 regs, then the curve starts at body[10].
// For NCrv=1 / NPt=4 (declared NPt=20) the writable points are:
//
//	body[10]              ActPt
//	body[11], body[12]    Hz1, W1 (point 1)
//	body[13], body[14]    Hz2, W2
//	body[15], body[16]    Hz3, W3
//	body[17], body[18]    Hz4, W4
//	body[19..50]          Hz5..W20 (16 unused points)
//	body[51..58]          CrvNam string[8]
//	body[59]              RmpPT1Tms
//	body[60]              RmpDecTmm
//	body[61]              RmpIncTmm
//	body[62]              RmpRsUp
//	body[63]              SnptW
//	body[64]              WRef
//	body[65]              WRefStrHz
//	body[66]              WRefStopHz
//	body[67]              ReadOnly
const (
	freqWattCurveBodyActCrv      uint16 = 0
	freqWattCurveBodyModEna      uint16 = 1
	freqWattCurveBodyWinTms      uint16 = 2
	freqWattCurveBodyRvrtTms     uint16 = 3
	freqWattCurveBodyRmpTms      uint16 = 4
	freqWattCurveBodyNCrv        uint16 = 5
	freqWattCurveBodyNPt         uint16 = 6
	freqWattCurveBodyHzSF        uint16 = 7
	freqWattCurveBodyWSF         uint16 = 8
	freqWattCurveBodyRmpIncDecSF uint16 = 9

	freqWattCurveBodyActPt uint16 = 10

	// Exported point offsets — consulted by the server-side writer to
	// drive routing and write-range checks. Keep these in sync with the
	// body emitted by emitFreqWattCurve.
	FreqWattCurveBodyHz1Off uint16 = 11
	FreqWattCurveBodyW1Off  uint16 = 12
	FreqWattCurveBodyHz2Off uint16 = 13
	FreqWattCurveBodyW2Off  uint16 = 14
	FreqWattCurveBodyHz3Off uint16 = 15
	FreqWattCurveBodyW3Off  uint16 = 16
	FreqWattCurveBodyHz4Off uint16 = 17
	FreqWattCurveBodyW4Off  uint16 = 18

	// Hz5..W20 — 16 unused (Hz, W) pairs occupy body[19..50] (32 regs).
	freqWattCurveBodyUnusedStart uint16 = 19
	freqWattCurveBodyUnusedEnd   uint16 = 50 // inclusive: W20

	freqWattCurveBodyCrvNamStart uint16 = 51 // string[8] → body[51..58]
	freqWattCurveBodyCrvNamEnd   uint16 = 58
	freqWattCurveBodyRmpPT1Tms   uint16 = 59
	freqWattCurveBodyRmpDecTmm   uint16 = 60
	freqWattCurveBodyRmpIncTmm   uint16 = 61
	freqWattCurveBodyRmpRsUp     uint16 = 62
	freqWattCurveBodySnptW       uint16 = 63
	freqWattCurveBodyWRef        uint16 = 64
	freqWattCurveBodyWRefStrHz   uint16 = 65
	freqWattCurveBodyWRefStopHz  uint16 = 66
	freqWattCurveBodyReadOnly    uint16 = 67
)

// emitFreqWattCurve emits Model 134 sourced from per-inverter freq-watt
// thresholds. The four points Hz1..Hz4 are taken from APsystems codes
// DH, DI, CB, CC respectively; W1..W4 are fixed at 0/100/100/0
// because they're intrinsic to the threshold roles, not user-tunable.
//
// If any of the four codes is missing from p.Has, the curve emits with
// ActPt=0 (model still discoverable, but disabled). ModEna defaults to
// 0 (disabled) until all four points are populated AND OFDroopMode
// is in the active enum range — keeps Model 134 from announcing an
// inverter is freq-watt-active when the firmware still says disabled.
func emitFreqWattCurve(bank *Bank, p source.ProtectionParams) {
	bank.put16(FreqWattCurveModelID, FreqWattCurveBodyLen)

	// --- Header (10 regs) ---
	bank.put16(1) // ActCrv = 1 (curve index 1 is the only one we publish)

	// ModEna: 1 if the inverter has freq-watt enabled (CV in active
	// range) AND every threshold is populated. The bitfield16 has a
	// single defined bit (ENABLED at value 0); we publish 1 to mean
	// "active" since clients widely interpret a non-zero bitfield as on.
	// Mode enum values for Over_frequency_Watt_set (CV):
	//   13 = AS/NZS variant (active)
	//   14 = other regions (active)
	//   15 = disabled
	const cvAsNzs = 13
	const cvOther = 14
	hasAll := p.Has["DH"] && p.Has["DI"] && p.Has["CB"] && p.Has["CC"]
	modEna := uint16(0)
	if hasAll && (int(p.OFDroopMode) == cvAsNzs || int(p.OFDroopMode) == cvOther) {
		modEna = 1
	}
	bank.put16(modEna)

	bank.put16(0)                   // WinTms (unused)
	bank.put16(0)                   // RvrtTms
	bank.put16(0)                   // RmpTms
	bank.put16(freqWattCurveNCrv)   // NCrv
	bank.put16(freqWattCurveNPtMax) // NPt — declared capacity (20)
	bank.put16(scaleFactor(freqWattCurveHzSF))
	bank.put16(scaleFactor(freqWattCurveWSF))
	bank.put16(scaleFactor(freqWattCurveRmpIncDecSF))

	// --- Curve 1 (57 regs) ---

	// ActPt: only count as active when all four codes present.
	if hasAll {
		bank.put16(freqWattCurveActPt)
	} else {
		bank.put16(0)
	}

	// Hz1..W4 — the four user-driven Hz from APsystems codes, fixed W%.
	hz1 := hzCenti16(p.OFCurveUFLow)  // DH
	hz2 := hzCenti16(p.OFCurveUFHigh) // DI
	hz3 := hzCenti16(p.OFCurveOFLow)  // CB
	hz4 := hzCenti16(p.OFDroopEnd)    // CC

	bank.put16(hz1) // Hz1
	bank.put16(0)   // W1 = 0%
	bank.put16(hz2) // Hz2
	bank.put16(100) // W2 = 100%
	bank.put16(hz3) // Hz3
	bank.put16(100) // W3 = 100%
	bank.put16(hz4) // Hz4
	bank.put16(0)   // W4 = 0%

	// Hz5..W20 — unused points. Emit zeros (NPt declared = 20, ActPt = 4
	// tells clients only the first 4 are meaningful). 16 unused points
	// × 2 regs each = 32 regs.
	for i := 0; i < 16; i++ {
		bank.put16(0) // Hz_n
		bank.put16(0) // W_n
	}

	// CrvNam — 8 regs of string. "APS-FW1\0\0..." padded by putString.
	bank.putString("APS-FW1", 8)

	// Ramp / snapshot fields — APsystems firmware doesn't expose these,
	// emit zeros. They remain writable per the spec, but we won't act
	// on writes (FreqWattCurveWriter rejects them as not supported).
	bank.put16(0) // RmpPT1Tms
	bank.put16(0) // RmpDecTmm
	bank.put16(0) // RmpIncTmm
	bank.put16(0) // RmpRsUp
	bank.put16(0) // SnptW (snapshot disabled)

	// WRef — reference active power baseline, default 100% of WMax.
	bank.put16(freqWattCurveDefaultWRef)
	bank.put16(0) // WRefStrHz (snapshot trigger Hz, unused)
	bank.put16(0) // WRefStopHz

	// ReadOnly = READWRITE — clients are allowed to rewrite the points.
	bank.put16(freqWattCurveReadOnlyRW)
}

// hzCenti16 encodes Hz as centi-Hertz (Hz_SF=-2) into a uint16. 50.20
// Hz → 5020. Returns 0 for non-positive inputs and clamps to uint16
// range.
func hzCenti16(hz float64) uint16 {
	if hz <= 0 {
		return 0
	}
	v := hz * 100.0
	if v > 65535 {
		return 65535
	}
	return uint16(v + 0.5)
}

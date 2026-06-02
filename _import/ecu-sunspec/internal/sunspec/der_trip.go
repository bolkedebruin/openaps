package sunspec

import (
	"github.com/bolke/ecu-sunspec/internal/source"
)

// SunSpec IEEE 1547-2018 DER trip models.
//
// Models 707/708 expose the low/high voltage trip curves; 709/710 expose the
// low/high frequency trip curves. Each is a 3-tier curve table (MustTrip /
// MayTrip / MomCess) with up to NPt points per tier. APsystems firmware
// exposes a 2-stage protection design (stage 2 + stage 3 / fast); we map both
// stages onto MustTrip and emit MayTrip / MomCess as ActPt=0 placeholders.
//
// We pick NPt = 3 to leave room for the averaged 10-minute over-voltage limit
// (code AB) on Model 708 as a third MustTrip point.
//
// Models 707/708 (voltage) and 709/710 (frequency) have DIFFERENT body lengths
// because the V point is uint16 (1 reg) but the Hz point is uint32 (2 regs):
//
//	707/708 layout (40 regs total, L=38):
//	  Header (9 regs): ID, L, Ena, AdptCrvReq, AdptCrvRslt, NPt=3, NCrvSet=1,
//	                   V_SF, Tms_SF
//	  Curve (31 regs): ReadOnly + 3×Tier; each Tier = ActPt + 3×(V uint16,
//	                   Tms uint32) = 1 + 9 = 10 regs.
//
//	709/710 layout (49 regs total, L=47):
//	  Header (9 regs): same shape but Hz_SF instead of V_SF
//	  Curve (40 regs): ReadOnly + 3×Tier; each Tier = ActPt + 3×(Hz uint32,
//	                   Tms uint32) = 1 + 12 = 13 regs.
//
// Voltage points are in % of nominal voltage with V_SF = -2 (centi-percent).
// VNom default is 230 V (EU). Per-inverter banks use the inverter's reported
// AC voltage when non-zero; aggregate bank uses snapshot.GridVoltageV.
//
// Frequency points are absolute Hz with Hz_SF = -2 (centi-Hertz).
// Time points are seconds × 100 (Tms_SF = -2 → centiseconds), uint32.

const (
	derTripVBodyLen  uint16 = 38 // body length for 707/708 (V is uint16)
	derTripHzBodyLen uint16 = 47 // body length for 709/710 (Hz is uint32)
	derTripNPt       uint16 = 3
	derTripNCrv      uint16 = 1
	derTripVSF       int8   = -2
	derTripHzSF      int8   = -2
	derTripTmsSF     int8   = -2
	derTripVDefault         = 230.0

	DERTripLVModelID uint16 = 707
	DERTripHVModelID uint16 = 708
	DERTripLFModelID uint16 = 709
	DERTripHFModelID uint16 = 710

	// AdptCrvRslt enum values per the SunSpec model definitions for 707-710.
	// We publish COMPLETED for the static snapshot — there's no async curve
	// write in flight; we're just exposing the inverter's currently-loaded
	// trip points. IN_PROGRESS would be misread by clients as "writing now".
	derTripRsltInProgress uint16 = 0
	derTripRsltCompleted  uint16 = 1
	derTripRsltFailed     uint16 = 2
)

// trip3Pt is one tier (MustTrip / MayTrip / MomCess) of a DER-trip curve.
// Each point holds a value (interpreted as uint16 V for 707/708 or uint32 Hz
// for 709/710 by the emitter) and a uint32 time in centiseconds.
type trip3Pt struct {
	ActPt  uint16
	Points [3]struct {
		Val uint32
		Tms uint32
	}
}

// emitDERTripV emits Model 707 or 708 (low or high voltage trip).
func emitDERTripV(bank *Bank, modelID uint16, vNom float64, must trip3Pt) {
	if vNom <= 0 {
		vNom = derTripVDefault
	}
	bank.put16(modelID, derTripVBodyLen)
	ena := uint16(0)
	if must.ActPt > 0 {
		ena = 1
	}
	bank.put16(ena)
	bank.put16(0)                    // AdptCrvReq
	bank.put16(derTripRsltCompleted) // AdptCrvRslt
	bank.put16(derTripNPt)
	bank.put16(derTripNCrv)
	bank.put16(scaleFactor(derTripVSF))
	bank.put16(scaleFactor(derTripTmsSF))
	bank.put16(1) // ReadOnly = R
	emitVTier(bank, must)
	emitVTier(bank, trip3Pt{})
	emitVTier(bank, trip3Pt{})
}

// emitDERTripHz emits Model 709 or 710 (low or high frequency trip).
func emitDERTripHz(bank *Bank, modelID uint16, must trip3Pt) {
	bank.put16(modelID, derTripHzBodyLen)
	ena := uint16(0)
	if must.ActPt > 0 {
		ena = 1
	}
	bank.put16(ena)
	bank.put16(0)                    // AdptCrvReq
	bank.put16(derTripRsltCompleted) // AdptCrvRslt
	bank.put16(derTripNPt)
	bank.put16(derTripNCrv)
	bank.put16(scaleFactor(derTripHzSF))
	bank.put16(scaleFactor(derTripTmsSF))
	bank.put16(1) // ReadOnly = R
	emitHzTier(bank, must)
	emitHzTier(bank, trip3Pt{})
	emitHzTier(bank, trip3Pt{})
}

// emitVTier writes a tier for Models 707/708 (V is uint16, 1 reg).
func emitVTier(bank *Bank, t trip3Pt) {
	bank.put16(t.ActPt)
	for _, pt := range t.Points {
		bank.put16(uint16(pt.Val))
		bank.putAcc32(uint64(pt.Tms))
	}
}

// emitHzTier writes a tier for Models 709/710 (Hz is uint32, 2 regs).
func emitHzTier(bank *Bank, t trip3Pt) {
	bank.put16(t.ActPt)
	for _, pt := range t.Points {
		bank.putAcc32(uint64(pt.Val))
		bank.putAcc32(uint64(pt.Tms))
	}
}

// pctOfVNom converts an absolute volt value to centi-percent of vNom (V_SF=-2).
// Returns 0 for non-positive inputs (callers gate on .Has). vNom defaults to
// 230 V when the supplied value is non-positive — keeps output sane at night
// when the inverter's reported AC voltage is 0.
func pctOfVNom(v, vNom float64) uint16 {
	if v <= 0 {
		return 0
	}
	if vNom <= 0 {
		vNom = derTripVDefault
	}
	pct := v / vNom * 10000.0
	if pct < 0 {
		return 0
	}
	if pct > 65535 {
		return 65535
	}
	return uint16(pct + 0.5)
}

// hzCenti encodes Hz as centi-Hertz (Hz_SF=-2). 47.5 Hz → 4750.
// Returns uint32 because Models 709/710 store Hz as uint32 (2 regs).
func hzCenti(hz float64) uint32 {
	if hz <= 0 {
		return 0
	}
	v := hz * 100.0
	if v > 4294967295 {
		return 4294967295
	}
	return uint32(v + 0.5)
}

// secCenti encodes seconds as centi-seconds (Tms_SF=-2). 0.16 s → 16.
func secCenti(s float64) uint32 {
	if s <= 0 {
		return 0
	}
	v := s * 100.0
	if v > 4294967295 {
		return 4294967295
	}
	return uint32(v + 0.5)
}

// tripPointsFromProtection builds the four MustTrip tiers (LV, HV, LF, HF)
// from per-inverter active protection settings.
//
// Mapping APsystems → 1547-2018 DER trip curve points:
//
//	707 (LV) MustTrip: (AC stage 2, BB clearance), (AQ fast, BD clearance)
//	708 (HV) MustTrip: (AY stage 3, BE clearance), (AD stage 2, BC clearance),
//	                   optionally (AB averaged 10-min, 600 s)
//	709 (LF) MustTrip: (AE slow, BJ clearance), (AJ fast, BH clearance)
//	710 (HF) MustTrip: (AF slow, BK clearance), (AK fast, BI clearance)
func tripPointsFromProtection(p source.ProtectionParams, vNom float64) (lv, hv, lf, hf trip3Pt) {
	{
		var n uint16
		if p.Has["AC"] && p.Has["BB"] {
			lv.Points[n].Val = uint32(pctOfVNom(p.UVStg2, vNom))
			lv.Points[n].Tms = secCenti(p.UV2ClrS)
			n++
		}
		if p.Has["AQ"] && p.Has["BD"] {
			lv.Points[n].Val = uint32(pctOfVNom(p.UVFast, vNom))
			lv.Points[n].Tms = secCenti(p.UV3ClrS)
			n++
		}
		lv.ActPt = n
	}
	{
		var n uint16
		if p.Has["AY"] && p.Has["BE"] {
			hv.Points[n].Val = uint32(pctOfVNom(p.OVStg3, vNom))
			hv.Points[n].Tms = secCenti(p.OV3ClrS)
			n++
		}
		if p.Has["AD"] && p.Has["BC"] {
			hv.Points[n].Val = uint32(pctOfVNom(p.OVStg2, vNom))
			hv.Points[n].Tms = secCenti(p.OV2ClrS)
			n++
		}
		if p.Has["AB"] && n < derTripNPt {
			hv.Points[n].Val = uint32(pctOfVNom(p.AvgOV, vNom))
			hv.Points[n].Tms = secCenti(600)
			n++
		}
		hv.ActPt = n
	}
	{
		var n uint16
		if p.Has["AE"] && p.Has["BJ"] {
			lf.Points[n].Val = hzCenti(p.UFSlow)
			lf.Points[n].Tms = secCenti(p.UF2ClrS)
			n++
		}
		if p.Has["AJ"] && p.Has["BH"] {
			lf.Points[n].Val = hzCenti(p.UFFast)
			lf.Points[n].Tms = secCenti(p.UF1ClrS)
			n++
		}
		lf.ActPt = n
	}
	{
		var n uint16
		if p.Has["AF"] && p.Has["BK"] {
			hf.Points[n].Val = hzCenti(p.OFSlow)
			hf.Points[n].Tms = secCenti(p.OF2ClrS)
			n++
		}
		if p.Has["AK"] && p.Has["BI"] {
			hf.Points[n].Val = hzCenti(p.OFFast)
			hf.Points[n].Tms = secCenti(p.OF1ClrS)
			n++
		}
		hf.ActPt = n
	}
	return
}

// emitDERTripModels emits Models 707/708/709/710 in order.
func emitDERTripModels(bank *Bank, p source.ProtectionParams, vNom float64) {
	lv, hv, lf, hf := tripPointsFromProtection(p, vNom)
	emitDERTripV(bank, DERTripLVModelID, vNom, lv)
	emitDERTripV(bank, DERTripHVModelID, vNom, hv)
	emitDERTripHz(bank, DERTripLFModelID, lf)
	emitDERTripHz(bank, DERTripHFModelID, hf)
}

// aggregateProtection picks one ProtectionParams to represent the whole ECU
// at unit ID 1. We use the first inverter's params (in stable iteration
// order) — averaging is meaningless for trip thresholds, and "most
// restrictive" is hard to define when MustTrip is a curve. Per-inverter
// banks (unit IDs 2..N+1) expose each inverter's actual values for
// consumers that care.
func aggregateProtection(s source.Snapshot) source.ProtectionParams {
	if len(s.Protection) == 0 {
		return source.ProtectionParams{Has: map[string]bool{}}
	}
	for _, inv := range s.Inverters {
		if p, ok := s.Protection[inv.UID]; ok {
			return p
		}
	}
	for _, p := range s.Protection {
		return p
	}
	return source.ProtectionParams{Has: map[string]bool{}}
}

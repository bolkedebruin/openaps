package server

import "github.com/bolke/ecu-sunspec/internal/sunspec"

// Per-family routing for grid-protection parameter writes.
//
// A protection-parameter write routes through one of several
// family-specific frame builders, keyed on the inverter's numeric model
// code. The on-wire opcode and (more importantly) whether the inverter
// firmware honours the resulting frame is family-specific. This file is
// the single source of truth for that routing — everywhere else in the
// server treats it as opaque.
//
// Family routing facts:
//
//	model 7,  0x17       → YC600 family (opcode encoded as direct cmd byte)
//	model 8,  0x18       → QS1 / QS1A   (opcode 0x1C)
//	model 5,  6          → YC1000 / 30k S3 (opcode 0x1B)
//	model 0x20/21/22/36  → DS3 family   (opcode 0xAB)
//	model 0x29/30/31/32  → QT2 family   (opcode 0xAD)
//
// QS1A reject-set (security/qs1-dc-rejection.md, qs1a-probe-results.md,
// memory/qs1a_dc_cg_cf_unwriteable.md):
// per-parameter writes via set_protection_parameters_inverter are
// honoured for CA / CB / CC / DD / DH / DI but the inverter firmware
// silently drops DC / CG / CF on BOTH host-side paths — both the per-
// inverter direct path AND the gridProfile broadcast path
// (singlegridPfile.conf + /tmp/set_sPfile.conf trigger) have been
// confirmed not to land them. The reject is at the QS1A inverter
// firmware level; no host-side mechanism applies. Routes for those
// three are RouteUnsupported.

// InverterFamily groups the model codes that share an on-wire frame
// builder.
type InverterFamily int

const (
	FamilyUnknown InverterFamily = iota
	FamilyDS3
	FamilyQT2
	FamilyQS1A
	FamilyYC600
	FamilyYC1000
)

// familyForModel maps the APsystems numeric model code (as stored in
// the inventory) to a family.
func familyForModel(code int) InverterFamily {
	switch code {
	case 7, 0x17:
		return FamilyYC600
	case 8, 0x18:
		return FamilyQS1A
	case 5, 6:
		return FamilyYC1000
	case 0x20, 0x21, 0x22, 0x36:
		return FamilyDS3
	case 0x29, 0x30, 0x31, 0x32:
		return FamilyQT2
	}
	return FamilyUnknown
}

// WriteRoute names how a value lands on a given (family, param) pair.
// RouteUnsupported means no host-side path lands it — callers must not
// queue anything and should surface an error.
type WriteRoute int

const (
	// RouteDirect — the standard per-inverter path: inv-driver dispatches
	// a single-parameter frame to the inverter. This is what
	// Writer.SetProtectionParam already does.
	RouteDirect WriteRoute = iota

	// RouteUnsupported — no host-side path is known to land the value.
	// Surface to the SunSpec client as an error; do not silently drop.
	// (APsystems' whole-profile push via singlegridPfile.conf is
	// deliberately not used — ecu-sunspec writes only granular
	// per-parameter frames through inv-driver.)
	RouteUnsupported
)

// Long-form parameter names that QS1A firmware silently rejects on the
// per-inverter path (security/qs1a-probe-results.md). All three need
// the gridProfile path to actually move on QS1A.
const (
	apsRecoverHighName = "Over_frequency_Watt_recover_High_set" // DC
	apsDelayTimeName   = "Over_frequency_Watt_Delay_Time_set"   // CG
	apsSlopeName       = "Over_Frequency_Watt_Slope_set"        // CF
)

// routeFor decides where to send a write for (family, paramName).
//
// Default for unknown params is RouteDirect — the bulk of params route
// through the per-inverter path and don't need the whole-profile path.
func routeFor(family InverterFamily, paramName string) WriteRoute {
	switch family {
	case FamilyUnknown:
		return RouteUnsupported

	case FamilyQS1A:
		switch paramName {
		case apsRecoverHighName, apsSlopeName, apsDelayTimeName:
			// DC / CF / CG silently no-op on QS1A regardless of host-
			// side mechanism. Both per-inverter direct
			// (set_protection_parameters_inverter) and gridProfile
			// broadcast (singlegridPfile.conf + set_sPfile.conf) have
			// been confirmed not to land these — the reject is at the
			// QS1A inverter firmware level. See
			// security/qs1-dc-rejection.md and
			// memory/qs1a_dc_cg_cf_unwriteable.md.
			return RouteUnsupported
		}
		// CA / CB / CC / DD / DH / DI and everything else: per-inverter applies.
		return RouteDirect

	case FamilyDS3:
		// No DS3 firmware-level reject ever observed. Per-inverter
		// path handles every mapped param.
		return RouteDirect

	case FamilyQT2:
		// No QT2-specific reject evidence on file. Mirror DS3's
		// behaviour until probe data says otherwise.
		return RouteDirect

	case FamilyYC600, FamilyYC1000:
		// Same situation as QT2 — no negative evidence, default to
		// direct.
		return RouteDirect
	}
	return RouteDirect
}

// FreqWattCurvePoint binds a SunSpec freq-watt register (by its body
// offset within a specific model) to the APsystems long-form parameter
// name written through Writer.SetProtectionParam. The Code field is the
// 2-letter APsystems code (for logs / cross-references with
// protection_parameters60code).
//
// Decode is the function that decodes the wire register(s) at Body
// into a float64 value in the units the long-form name expects. For
// uint16 wire values BodyLo is 0 and only `hi` carries the register.
type FreqWattCurvePoint struct {
	Model  uint16 // SunSpec model ID this offset lives in (711 or 134)
	Body   uint16 // first body offset within the model
	BodyLo uint16 // second body offset for uint32 fields, 0 otherwise
	Aps    string // APsystems long-form parameter name
	Code   string // 2-letter code, for logs only
	Decode func(hi, lo uint16) float64
}

// freqWattCurvePoints describes the SunSpec freq-watt register →
// APsystems long-form mappings the writers route through routeFor.
//
// Model 711 (DERFreqDroop): the parametric droop curve.
//
//	DbOf (body[12..13]) → Over_frequency_Watt_Start_set (CA)
//
// CF (Slope) and CG (Delay) are intentionally omitted from this table —
// they're already handled inline by FreqDroopWriter.Apply via the KOf
// and RspTms branches, both of which consult routeFor directly.
//
// Model 134 (Freq-Watt Crv): a 4-point curve mapping the APsystems
// under/over frequency-watt thresholds onto the SunSpec curve points.
// W% values are intrinsic to the curve role (0/100/100/0); only Hz is
// user-driven.
//
//	Hz1 (curve body[1])  → Under_Frequency_Watt_Low_set  (DH, W=0%)
//	Hz2 (curve body[3])  → Under_Frequency_Watt_High_set (DI, W=100%)
//	Hz3 (curve body[5])  → Over_frequency_Watt_Low_set   (CB, W=100%)
//	Hz4 (curve body[7])  → Over_frequency_Watt_High_set  (CC, W=0%)
//
// Body offsets here are absolute within the Model 134 body (header is
// 10 regs; curve starts at body[10]; ActPt at body[10] then Hz1 at
// body[11], W1 at body[12], etc.). See internal/sunspec/freq_watt_curve.go
// for the layout constants. Hz_SF=-2 → wire = Hz×100.
//
// DD (Delt_P_Over_HF) has no SunSpec Model 711 / 134 register;
// expose through a different write path if/when needed.
var freqWattCurvePoints = []FreqWattCurvePoint{
	{
		Model:  711,
		Body:   12, // freqDroopBodyDbOfHi
		BodyLo: 13, // freqDroopBodyDbOfLo
		Aps:    "Over_frequency_Watt_Start_set",
		Code:   "CA",
		Decode: func(hi, lo uint16) float64 {
			// DbOf is centi-Hz deadband above 50 Hz nominal.
			return 50.0 + float64(uint32(hi)<<16|uint32(lo))/100.0
		},
	},
	{
		Model: 134,
		Body:  sunspec.FreqWattCurveBodyHz1Off,
		Aps:   "Under_Frequency_Watt_Low_set",
		Code:  "DH",
		Decode: func(hi, _ uint16) float64 {
			// Hz_SF = -2 → wire is centi-Hz absolute frequency.
			return float64(hi) / 100.0
		},
	},
	{
		Model: 134,
		Body:  sunspec.FreqWattCurveBodyHz2Off,
		Aps:   "Under_Frequency_Watt_High_set",
		Code:  "DI",
		Decode: func(hi, _ uint16) float64 {
			return float64(hi) / 100.0
		},
	},
	{
		Model: 134,
		Body:  sunspec.FreqWattCurveBodyHz3Off,
		Aps:   "Over_frequency_Watt_Low_set",
		Code:  "CB",
		Decode: func(hi, _ uint16) float64 {
			return float64(hi) / 100.0
		},
	},
	{
		Model: 134,
		Body:  sunspec.FreqWattCurveBodyHz4Off,
		Aps:   "Over_frequency_Watt_High_set",
		Code:  "CC",
		Decode: func(hi, _ uint16) float64 {
			return float64(hi) / 100.0
		},
	},
}

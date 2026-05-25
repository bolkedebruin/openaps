package gridprofile

import (
	"fmt"
	"math"
	"sort"
)

// nativeUnit returns the unit a point's native value is expressed in. Voltage
// thresholds are %VNom in SunSpec but stored/entered as absolute volts, so
// their native unit is "V" rather than the forward map's SunSpec unit label.
func (e mapEntry) nativeUnit() string {
	if e.IsVoltagePct {
		return "V"
	}
	return e.Unit
}

// ParamInfo describes one editable grid-protection parameter, for clients
// (e.g. the web overlay editor) that need the parameter universe and its
// display metadata without duplicating the forward map.
type ParamInfo struct {
	ApsCode  string `json:"aps_code"`
	LongName string `json:"long_name,omitempty"`
	Unit     string `json:"unit"`
	Group    string `json:"group"`
	Model    int    `json:"model"`
}

// ParamCatalog returns every encodable parameter in the forward map (those
// with a firmware encoder, i.e. a non-empty LongName), sorted by aps code.
// It is the single source of truth for what an overlay can set; per-inverter
// writability is then narrowed by Writeable(modelCode, apsCode).
func ParamCatalog() []ParamInfo {
	out := make([]ParamInfo, 0, len(forwardMap))
	for code, e := range forwardMap {
		if e.LongName == "" {
			continue // readable but not encodable — not editable
		}
		out = append(out, ParamInfo{
			ApsCode:  code,
			LongName: e.LongName,
			Unit:     e.nativeUnit(),
			Group:    e.Group,
			Model:    e.Model,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ApsCode < out[j].ApsCode })
	return out
}

// defaultRange returns a per-code physical envelope by native unit, used to
// clamp overlay values for codes the active base profile does NOT define, so an
// override can never reach the encoder unbounded. Mirrors codec's write-side
// envelope and extends it to slopes. nil = no generic bound (e.g. mode enums).
func defaultRange(apsCode string) *Range {
	e, ok := forwardMap[apsCode]
	if !ok {
		return nil
	}
	if e.IsVoltagePct {
		return &Range{Min: 100, Max: 600} // volts
	}
	switch e.Unit {
	case "Hz":
		return &Range{Min: 40, Max: 70}
	case "s":
		return &Range{Min: 0, Max: 1000}
	case "%Pref/Hz":
		return &Range{Min: 0, Max: 200}
	}
	return nil
}

// BuildOverlayPoint constructs a v1 overlay PointEntry for the given aps code
// and native value, filling the SunSpec location and encoding metadata from
// the forward map. SunSpec and range are left unset: reconcile re-encodes from
// the native value and clamps against the base profile's range. Returns an
// error if the code is unknown or has no firmware encoder.
func BuildOverlayPoint(apsCode string, value float64) (PointEntry, error) {
	e, ok := forwardMap[apsCode]
	if !ok {
		return PointEntry{}, fmt.Errorf("aps_code %q not in forward map", apsCode)
	}
	if e.LongName == "" {
		return PointEntry{}, fmt.Errorf("aps_code %q has no firmware encoder", apsCode)
	}
	return PointEntry{
		Model:  e.Model,
		Group:  e.Group,
		Index:  e.Index,
		Point:  e.Point,
		SFRef:  e.SFRef,
		SF:     e.SF,
		Native: NativeValue{Value: value, Unit: e.nativeUnit()},
		Apply:  Apply{ApsCode: apsCode},
	}, nil
}

// mapEntry is one row in the §3 forward map: APsystems 2-letter code →
// SunSpec location + encoding metadata.
type mapEntry struct {
	// SunSpec location
	Model int
	Group string
	Index int    // repeating-group / curve-point index (0-based)
	Point string
	SFRef *string // nil for unscaled points (e.g. ESDlyTms)
	SF    int     // exponent: sunspec_value = round(native / 10^sf)
	Unit  string  // native unit label

	// isVoltagePct signals that this point is expressed as %VNom in the
	// SunSpec model.  When true, conversion must go V ↔ %VNom before
	// applying the SF scale.
	IsVoltagePct bool

	// LongName is the firmware param name accepted by
	// codec.EncodeSetProtection.  An empty string means the code has no
	// known encoder and must not be dispatched for writing.
	LongName string
}

// sfRefStr returns a *string pointing at a static constant, avoiding
// allocs for the common case of a named SF reference.
func sfRefStr(s string) *string { v := s; return &v }

// forwardMap is THE single source of truth for the APsystems 2-letter code
// ↔ SunSpec symbol mapping (build plan §3).
//
// Codes not listed here have no SunSpec representation in this codebase.
// A LongName of "" means the code is readable (present in the forward map)
// but not yet encodable via codec.EncodeSetProtection.
//
// Voltage thresholds (AC/AD/AQ/AY/AB/BN/BO) are stored in the inverter as
// absolute volts but SunSpec models 707/708/703 express them as %VNom;
// IsVoltagePct=true triggers the V↔%VNom conversion in helpers below.
var forwardMap = map[string]mapEntry{
	// --- Voltage trip LV (Model 707 DERTripLV, MustTrip.Pt[]) ---
	// Pt[0]: under_voltage_stage_2_90 (AC) + Under_Voltage1_clearance_time (BB)
	// Pt[1]: under_voltage_stage_2 / under_voltage_fast_90 (AQ) + Under_Voltage2_clearance_time (BD)
	"AC": {Model: 707, Group: "MustTrip", Index: 0, Point: "V", SFRef: sfRefStr("V_SF"), SF: -2, Unit: "%VNom", IsVoltagePct: true, LongName: "under_voltage_stage_2_90"},
	"BB": {Model: 707, Group: "MustTrip", Index: 0, Point: "Tms", SFRef: sfRefStr("Tms_SF"), SF: -2, Unit: "s", LongName: "Under_Voltage1_clearance_time"},
	"AQ": {Model: 707, Group: "MustTrip", Index: 1, Point: "V", SFRef: sfRefStr("V_SF"), SF: -2, Unit: "%VNom", IsVoltagePct: true, LongName: "under_voltage_stage_2"},
	"BD": {Model: 707, Group: "MustTrip", Index: 1, Point: "Tms", SFRef: sfRefStr("Tms_SF"), SF: -2, Unit: "s", LongName: "Under_Voltage2_clearance_time"},

	// --- Voltage trip HV (Model 708 DERTripHV, MustTrip.Pt[]) ---
	// Pt[0]: over_voltage_slow / over_voltage_fast_90 (AD) + Over_Voltage1_clearance_time (BC)
	// Pt[1]: over_voltage_slow_90 (AY) + Over_Voltage2_clearance_time (BE)
	// Pt[2]: 10-min average OV threshold (AB)
	"AD": {Model: 708, Group: "MustTrip", Index: 0, Point: "V", SFRef: sfRefStr("V_SF"), SF: -2, Unit: "%VNom", IsVoltagePct: true, LongName: "over_voltage_slow"},
	"BC": {Model: 708, Group: "MustTrip", Index: 0, Point: "Tms", SFRef: sfRefStr("Tms_SF"), SF: -2, Unit: "s", LongName: "Over_Voltage1_clearance_time"},
	"AY": {Model: 708, Group: "MustTrip", Index: 1, Point: "V", SFRef: sfRefStr("V_SF"), SF: -2, Unit: "%VNom", IsVoltagePct: true, LongName: "over_voltage_slow_90"},
	"BE": {Model: 708, Group: "MustTrip", Index: 1, Point: "Tms", SFRef: sfRefStr("Tms_SF"), SF: -2, Unit: "s", LongName: "Over_Voltage2_clearance_time"},
	// AB: 10-min average OV threshold (HV Pt[2])
	"AB": {Model: 708, Group: "MustTrip", Index: 2, Point: "V", SFRef: sfRefStr("V_SF"), SF: -2, Unit: "%VNom", IsVoltagePct: true, LongName: "min10_Over_average_voltage"},

	// --- Frequency trip LF (Model 709 DERTripLF, MustTrip.Pt[]) ---
	// Pt[0]: under_frequency_slow (AE) + Under_Frequency2_clearance_time (BJ)
	// Pt[1]: under_frequency_fast (AJ) + Under_Frequency1_clearance_time (BH)
	"AE": {Model: 709, Group: "MustTrip", Index: 0, Point: "Hz", SFRef: sfRefStr("Hz_SF"), SF: -2, Unit: "Hz", LongName: "under_frequency_slow"},
	"BJ": {Model: 709, Group: "MustTrip", Index: 0, Point: "Tms", SFRef: sfRefStr("Tms_SF"), SF: -2, Unit: "s", LongName: "Under_Frequency2_clearance_time"},
	"AJ": {Model: 709, Group: "MustTrip", Index: 1, Point: "Hz", SFRef: sfRefStr("Hz_SF"), SF: -2, Unit: "Hz", LongName: "under_frequency_fast"},
	"BH": {Model: 709, Group: "MustTrip", Index: 1, Point: "Tms", SFRef: sfRefStr("Tms_SF"), SF: -2, Unit: "s", LongName: "Under_Frequency1_clearance_time"},

	// --- Frequency trip HF (Model 710 DERTripHF, MustTrip.Pt[]) ---
	// Pt[0]: over_frequency_slow (AF) + Over_Frequency2_clearance_time (BK)
	// Pt[1]: over_frequency_fast (AK) + Over_Frequency1_clearance_time (BI)
	"AF": {Model: 710, Group: "MustTrip", Index: 0, Point: "Hz", SFRef: sfRefStr("Hz_SF"), SF: -2, Unit: "Hz", LongName: "over_frequency_slow"},
	"BK": {Model: 710, Group: "MustTrip", Index: 0, Point: "Tms", SFRef: sfRefStr("Tms_SF"), SF: -2, Unit: "s", LongName: "Over_Frequency2_clearance_time"},
	"AK": {Model: 710, Group: "MustTrip", Index: 1, Point: "Hz", SFRef: sfRefStr("Hz_SF"), SF: -2, Unit: "Hz", LongName: "over_frequency_fast"},
	"BI": {Model: 710, Group: "MustTrip", Index: 1, Point: "Tms", SFRef: sfRefStr("Tms_SF"), SF: -2, Unit: "s", LongName: "Over_Frequency1_clearance_time"},

	// AH/AI: fast voltage trips without a SunSpec MustTrip slot (no Pt mapping in 707/708).
	// They are write-encodable on DS3 (and AH/AI on QS1A via 0x1C); keep them in the map
	// so the reconciler can dispatch them. Slot in HV/LV fast groups per EN 50549-1.
	// No SunSpec point currently assigned — using Model 708/707 extra slots for now.
	// LongName set so codec.EncodeSetProtection can dispatch them.
	"AH": {Model: 707, Group: "MustTrip", Index: 2, Point: "V", SFRef: sfRefStr("V_SF"), SF: -2, Unit: "%VNom", IsVoltagePct: true, LongName: "under_voltage_fast"},
	"AI": {Model: 708, Group: "MustTrip", Index: 3, Point: "V", SFRef: sfRefStr("V_SF"), SF: -2, Unit: "%VNom", IsVoltagePct: true, LongName: "over_voltage_fast"},

	// --- Enter-service (Model 703 DEREnterService, scalars) ---
	"BN": {Model: 703, Group: "DEREnterService", Point: "ESVLo", SFRef: sfRefStr("V_SF"), SF: -2, Unit: "%VNom", IsVoltagePct: true},
	"BO": {Model: 703, Group: "DEREnterService", Point: "ESVHi", SFRef: sfRefStr("V_SF"), SF: -2, Unit: "%VNom", IsVoltagePct: true},
	"BP": {Model: 703, Group: "DEREnterService", Point: "ESHzLo", SFRef: sfRefStr("Hz_SF"), SF: -2, Unit: "Hz"},
	"BQ": {Model: 703, Group: "DEREnterService", Point: "ESHzHi", SFRef: sfRefStr("Hz_SF"), SF: -2, Unit: "Hz"},
	// AG: grid_recovery / reconnect delay; no SF (unscaled in SunSpec ESDlyTms, seconds)
	"AG": {Model: 703, Group: "DEREnterService", Point: "ESDlyTms", SFRef: nil, SF: 0, Unit: "s", LongName: "grid_recovery_time"},
	// AS: start ramp (ESRmpTms in seconds); unscaled (no SF) per enter_service.go emitter.
	"AS": {Model: 703, Group: "DEREnterService", Point: "ESRmpTms", SFRef: nil, SF: 0, Unit: "s"},

	// --- Freq-Watt droop (Model 711 DERFreqDroop, Ctl[0]) ---
	// CV: over-freq-watt enable/mode enum (Ena)
	"CV": {Model: 711, Group: "DERFreqDroop", Index: 0, Point: "Ena", SFRef: nil, SF: 0, Unit: "", LongName: "Over_frequency_Watt_set"},
	// CA: over-freq deadband start (DbOf); canonical write code for over-freq curve start.
	// DC is a read-only alias for the same SunSpec point (decode-only, LongName="").
	"CA": {Model: 711, Group: "DERFreqDroop", Index: 0, Point: "DbOf", SFRef: sfRefStr("Db_SF"), SF: -2, Unit: "Hz", LongName: "Over_frequency_Watt_Start_set"},
	// DD: droop slope (%Pref/Hz → KOf); K_SF=-1 per freq_droop.go emitter.
	"DD": {Model: 711, Group: "DERFreqDroop", Index: 0, Point: "KOf", SFRef: sfRefStr("K_SF"), SF: -1, Unit: "%Pref/Hz", LongName: "Over_Frequency_Watt_Slope_set"},
	// CG: response delay (RspTms); RspTms_SF=0 (whole seconds) per freq_droop.go emitter.
	"CG": {Model: 711, Group: "DERFreqDroop", Index: 0, Point: "RspTms", SFRef: nil, SF: 0, Unit: "s", LongName: "Over_frequency_Watt_Delay_Time_set"},
	// DC: decode-only alias for CA (DbOf); LongName="" prevents write dispatch.
	// The canonical write code for over-freq curve start is CA.
	"DC": {Model: 711, Group: "DERFreqDroop", Index: 0, Point: "DbOf", SFRef: sfRefStr("Db_SF"), SF: -2, Unit: "Hz", LongName: ""},

	// --- Legacy freq-watt curve (Model 134, Hz1..Hz4) ---
	"DH": {Model: 134, Group: "CrvSet", Index: 0, Point: "Hz1", SFRef: sfRefStr("Hz_SF"), SF: -2, Unit: "Hz", LongName: "Under_Frequency_Watt_Low_set"},
	"DI": {Model: 134, Group: "CrvSet", Index: 0, Point: "Hz2", SFRef: sfRefStr("Hz_SF"), SF: -2, Unit: "Hz", LongName: "Under_Frequency_Watt_High_set"},
	"CB": {Model: 134, Group: "CrvSet", Index: 0, Point: "Hz3", SFRef: sfRefStr("Hz_SF"), SF: -2, Unit: "Hz", LongName: "Over_frequency_Watt_Low_set"},
	"CC": {Model: 134, Group: "CrvSet", Index: 0, Point: "Hz4", SFRef: sfRefStr("Hz_SF"), SF: -2, Unit: "Hz", LongName: "Over_frequency_Watt_High_set"},
}

// Lookup returns the map entry for an APsystems 2-letter code, or
// (zero, false) if the code is not in the forward map.
func Lookup(apsCode string) (mapEntry, bool) {
	e, ok := forwardMap[apsCode]
	return e, ok
}

// LongName returns the firmware param name for an APsystems code, or
// ("", false) if the code has no known encoder.
func LongName(apsCode string) (string, bool) {
	e, ok := forwardMap[apsCode]
	if !ok || e.LongName == "" {
		return "", false
	}
	return e.LongName, true
}

// VToVNomPct converts an absolute voltage to %VNom.
// vnom must be > 0.
func VToVNomPct(v, vnom float64) float64 {
	return v / vnom * 100.0
}

// VNomPctToV converts %VNom back to an absolute voltage.
// vnom must be > 0.
func VNomPctToV(pct, vnom float64) float64 {
	return pct / 100.0 * vnom
}

// EncodeSunSpec converts a native value to its SunSpec integer representation:
// round(native / 10^sf).
func EncodeSunSpec(native float64, sf int) float64 {
	scale := math.Pow10(sf)
	return math.Round(native / scale)
}

// DecodeSunSpec converts a SunSpec integer back to native units:
// sunspec_value * 10^sf.
func DecodeSunSpec(sunspec float64, sf int) float64 {
	return sunspec * math.Pow10(sf)
}

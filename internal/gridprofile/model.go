// Package gridprofile manages SunSpec-symbol-keyed grid-protection profiles
// for APsystems inverters. A profile is a set of named parameter points;
// overlays are per-inverter sparse overrides on top of a base profile.
package gridprofile

// SchemaVersion is the value of the "schema" field in every v1 profile and
// overlay document (the schemaVersion const in v1.schema.json).
const SchemaVersion = "invdriver.gridprofile/v1"

// Source describes the origin of a profile (e.g. an APsystems TY catalog
// entry). All fields are informational; system+ref are required.
type Source struct {
	System  string `json:"system"`
	Ref     string `json:"ref"`
	Bundle  string `json:"bundle,omitempty"`
	Version string `json:"version,omitempty"`
	Fetched string `json:"fetched,omitempty"`
}

// NativeValue is a physical value with its engineering unit.
type NativeValue struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// SunSpecValue is the scaled integer representation of a native value:
// sunspec_value = round(native / 10^sf).
type SunSpecValue struct {
	Value float64 `json:"value"`
}

// Range is the min/max bounds a native value must satisfy.
// min <= max is enforced by the loader; the JSON schema cannot cross-check
// sibling values.
type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// Apply carries the APsystems 2-letter firmware code used to dispatch a
// write for this point.
type Apply struct {
	ApsCode string `json:"aps_code"`
}

// PointEntry is one grid-protection parameter entry in a profile or overlay,
// described in SunSpec terms and cross-referenced to the APsystems firmware
// parameter name.
type PointEntry struct {
	Model   int           `json:"model"`
	Group   string        `json:"group"`
	Index   int           `json:"index,omitempty"`
	Point   string        `json:"point"`
	SFRef   *string       `json:"sf_ref,omitempty"`
	SF      int           `json:"sf,omitempty"`
	Native  NativeValue   `json:"native"`
	SunSpec *SunSpecValue `json:"sunspec,omitempty"`
	Range   *Range        `json:"range,omitempty"`
	Apply   Apply         `json:"apply"`
}

// Profile is a complete base grid-protection profile.  id is a human-readable
// name that is also the filename stem (profiles/<id>.json); vnom_v is the
// nominal AC voltage used to convert absolute-volt thresholds to %VNom.
type Profile struct {
	Schema string       `json:"schema"`
	ID     string       `json:"id"`
	VNomV  float64      `json:"vnom_v"`
	Source Source       `json:"source"`
	Points []PointEntry `json:"points"`
}

// Overlay is a sparse per-inverter override on top of a base profile.
// Only the points listed here differ from the base; absent points inherit
// the base value.  uids lists the inverter UIDs this overlay applies to
// (all must match ^[0-9A-Fa-f]{12}$).
type Overlay struct {
	Schema string       `json:"schema"`
	ID     string       `json:"id"`
	VNomV  float64      `json:"vnom_v,omitempty"`
	UIDs   []string     `json:"uids"`
	Points []PointEntry `json:"points"`
}

// EffectivePoint is one resolved parameter value for a specific inverter,
// ready for encoding.  It carries the target native value (already clamped
// to Range), the APS 2-letter code, and enough context for the encoder.
type EffectivePoint struct {
	ApsCode     string
	NativeValue float64
	Unit        string
	Range       *Range
	SF          int
}

// EffectiveProfile is the full resolved desired state for one inverter:
// the base profile merged with any overlay, with all values clamped to
// their declared ranges and filtered to only the points that are writable
// for that inverter's model family.
type EffectiveProfile struct {
	InverterUID string
	ModelCode   uint8
	Points      []EffectivePoint
}

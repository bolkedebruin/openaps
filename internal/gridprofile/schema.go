package gridprofile

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed v1.schema.json
var v1SchemaJSON []byte

// schemaID is the $id declared in v1.schema.json.
const schemaID = "https://github.com/bolke/inv-driver/gridprofile/v1.schema.json"

// compiled holds the pre-compiled v1 schema; populated by init().
var compiled *jsonschema.Schema

func init() {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(v1SchemaJSON)))
	if err != nil {
		panic("gridprofile: failed to parse v1.schema.json: " + err.Error())
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaID, doc); err != nil {
		panic("gridprofile: failed to add v1.schema.json resource: " + err.Error())
	}
	compiled, err = c.Compile(schemaID)
	if err != nil {
		panic("gridprofile: failed to compile v1.schema.json: " + err.Error())
	}
}

// Validate checks raw JSON bytes against the v1 schema.  Returns a
// descriptive error if validation fails.
func Validate(raw []byte) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("gridprofile: json parse: %w", err)
	}
	if err := compiled.Validate(v); err != nil {
		return fmt.Errorf("gridprofile: schema validation: %w", err)
	}
	return nil
}

// ValidateProfile validates raw JSON bytes as a profileFile and, if valid,
// additionally enforces range.min <= range.max for every point entry (a
// constraint the JSON Schema cannot express).
func ValidateProfile(raw []byte) error {
	if err := Validate(raw); err != nil {
		return err
	}
	return validateRanges(raw)
}

// ValidateOverlay validates raw JSON bytes as an overlayFile and additionally
// enforces range.min <= range.max.
func ValidateOverlay(raw []byte) error {
	if err := Validate(raw); err != nil {
		return err
	}
	return validateRanges(raw)
}

// validateRanges checks that every point entry with a declared range has
// min <= max.  Called after schema validation so the raw bytes are known
// to have valid structure.
func validateRanges(raw []byte) error {
	// Unmarshal into a minimal struct to avoid a full decode cycle.
	type rngEntry struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	}
	type pointEntry struct {
		Apply struct {
			ApsCode string `json:"aps_code"`
		} `json:"apply"`
		Range *rngEntry `json:"range,omitempty"`
	}
	type doc struct {
		Points []pointEntry `json:"points"`
	}
	var d doc
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("gridprofile: range check unmarshal: %w", err)
	}
	for _, p := range d.Points {
		if p.Range != nil && p.Range.Min > p.Range.Max {
			return fmt.Errorf("gridprofile: aps_code %q range.min %g > range.max %g",
				p.Apply.ApsCode, p.Range.Min, p.Range.Max)
		}
	}
	return nil
}

// SchemaBytes returns the embedded v1 schema JSON for external use.
func SchemaBytes() []byte {
	cp := make([]byte, len(v1SchemaJSON))
	copy(cp, v1SchemaJSON)
	return cp
}


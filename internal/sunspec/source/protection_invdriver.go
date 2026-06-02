package source

import "github.com/bolkedebruin/openaps/wire"

// protectionFromWire overlays an inv-driver Protection envelope onto a
// base ProtectionParams. Each proto field is optional (a pointer); a
// present field sets the corresponding value and marks the Has map under
// its 2-letter code, while absent fields keep the base. The base is
// normally a zero ProtectionParams (inv-driver is the only source), so the
// result carries exactly the codes inv-driver decoded; codes it doesn't
// report stay unset and SanitizeProtection drops them. The base's Has map
// is cloned so the merge doesn't mutate the caller's copy. The code→field
// mapping lives once in protCodeField.
func protectionFromWire(base ProtectionParams, p *wire.Protection) ProtectionParams {
	has := make(map[string]bool, len(base.Has)+32)
	for k, v := range base.Has {
		has[k] = v
	}
	base.Has = has
	pp := &base
	// Overlay each reported aps_code (Protection.values) onto its field via the
	// single code→field map; codes with no field (vendor extras) are ignored.
	for code, v := range p.GetValues() {
		if dst := protCodeField(pp, code); dst != nil {
			*dst = v
			pp.Has[code] = true
		}
	}
	return *pp
}

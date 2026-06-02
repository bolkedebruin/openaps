package source

// protCodeField maps a 2-letter protection code to the matching
// ProtectionParams field pointer — the single source of the code→field
// mapping, shared by the inv-driver overlay and the sanitizer. Returns
// nil for an unknown code.
func protCodeField(pp *ProtectionParams, code string) *float64 {
	switch code {
	case "AC":
		return &pp.UVStg2
	case "AQ":
		return &pp.UVFast
	case "AD":
		return &pp.OVStg2
	case "AY":
		return &pp.OVStg3
	case "AB":
		return &pp.AvgOV
	case "AH":
		return &pp.VWinLow
	case "AI":
		return &pp.VWinHi
	case "AE":
		return &pp.UFSlow
	case "AF":
		return &pp.OFSlow
	case "AJ":
		return &pp.UFFast
	case "AK":
		return &pp.OFFast
	case "AG":
		return &pp.ReconnectS
	case "AS":
		return &pp.StartS
	case "BB":
		return &pp.UV2ClrS
	case "BC":
		return &pp.OV2ClrS
	case "BD":
		return &pp.UV3ClrS
	case "BE":
		return &pp.OV3ClrS
	case "BH":
		return &pp.UF1ClrS
	case "BI":
		return &pp.OF1ClrS
	case "BJ":
		return &pp.UF2ClrS
	case "BK":
		return &pp.OF2ClrS
	case "BN":
		return &pp.ReconnVLow
	case "BO":
		return &pp.ReconnVHi
	case "BP":
		return &pp.ReconnFLow
	case "BQ":
		return &pp.ReconnFHi
	case "CH":
		return &pp.PFMode
	case "DC":
		return &pp.OFDroopStart
	case "CC":
		return &pp.OFDroopEnd
	case "DD":
		return &pp.OFDroopSlope
	case "CV":
		return &pp.OFDroopMode
	case "DH":
		return &pp.OFCurveUFLow
	case "DI":
		return &pp.OFCurveUFHigh
	case "CB":
		return &pp.OFCurveOFLow
	}
	return nil
}

// qs1PageACodes is the set of protection codes QS1A never returns on a
// read-back. QS1/QS1A answers the 0xDD page-A query with a 0xDB-tagged
// reply that inv-driver now decodes (codec.qs1ReadPageA), so the volt/freq
// trips, recovery/start times, and clearance times ARE read-back-verified.
// No QS1A code is currently unverifiable, so this set is empty; it remains
// the seam for any code later found genuinely absent. The per-code physical
// range backstop below still applies to every family.
var qs1PageACodes = map[string]bool{}

// protCodeRange returns the plausible physical bound for a code's
// quantity, or ok=false for enums/slopes that have no simple range.
func protCodeRange(code string) (lo, hi float64, ok bool) {
	switch code {
	case "AC", "AD", "AQ", "AY", "AB", "AH", "AI", "BN", "BO": // volts
		return 100, 600, true
	case "AE", "AF", "AJ", "AK", "BP", "BQ", "DC", "CC", "DH", "DI", "CB": // Hz
		return 40, 70, true
	case "AG", "AS", "BB", "BC", "BD", "BE", "BH", "BI", "BJ", "BK": // seconds
		return 0, 10000, true
	}
	return 0, 0, false // CH/CV/DD enums/slope — no simple range
}

// isQS1Family reports whether the inverter belongs to the family that does
// not support page-A protection read-back; other families keep all codes.
func isQS1Family(inv Inverter) bool {
	return inv.Model == 0x08 || inv.Model == 0x18 || inv.TypeCode == "03"
}

// SanitizeProtection drops protection codes that must not be reported as
// live: on families without page-A read-back, the unverifiable page-A
// trips; for any family, a value outside its physical range (a
// garbage/mismapped read). Dropped codes become absent (Has=false) so the
// SunSpec encoder emits not-implemented. The Has map is cloned so the
// caller's copy is untouched.
func SanitizeProtection(pp ProtectionParams, qs1 bool) ProtectionParams {
	has := make(map[string]bool, len(pp.Has))
	for k, v := range pp.Has {
		has[k] = v
	}
	pp.Has = has
	for code := range pp.Has {
		if qs1 && qs1PageACodes[code] {
			delete(pp.Has, code)
			continue
		}
		if lo, hi, ok := protCodeRange(code); ok {
			if f := protCodeField(&pp, code); f != nil && (*f < lo || *f > hi) {
				delete(pp.Has, code)
			}
		}
	}
	return pp
}

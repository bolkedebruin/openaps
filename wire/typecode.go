package wire

import "strings"

// TypeCodeForModel returns the leading "type" column used in
// /tmp/parameters_app.conf for the given inverter family. Only
// families that have been verified on real hardware are listed; any
// unknown model returns "" so callers fail loudly instead of mis-
// categorising the inverter.
//
// Verified mappings (from live captures on firmware 2.1.29D):
//
//	"01"  DS3 / DS3D / YC600 (2-channel)
//	"03"  QS1 / QS1A (4-channel)
func TypeCodeForModel(model string) string {
	switch {
	case strings.HasPrefix(model, "QS1A"), strings.HasPrefix(model, "QS1"):
		return "03"
	case strings.HasPrefix(model, "DS3"):
		return "01"
	}
	return ""
}

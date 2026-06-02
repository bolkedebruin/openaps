package codec

import (
	"sort"
	"strings"
)

// Family classifies an inverter model code by the on-wire protocol
// family the codec dispatches on (set-power, set-protection, protection
// read). It is the canonical replacement for ad-hoc string switches on
// the human model label.
//
// Several model codes map to the same Family because their wire frame
// builders are byte-identical — e.g. DS3, DS3H, DS3L and the un-pinned
// 0x36 variant all share the DS3 encoders, and QS1 (0x08) shares the
// QS1A (0x18) encoders. The Family grain matches the dispatch grain in
// protection.go / setpower.go / protection_read.go.
type Family uint8

const (
	FamilyUnknown Family = iota
	FamilyDS3
	FamilyQS1
	FamilyYC600
	FamilyYC1000
	FamilyQT2
)

// String returns a lowercase identifier for the family, suitable for
// use as a database column value or a CLI flag. FamilyUnknown returns
// the empty string so callers can store it as NULL.
func (f Family) String() string {
	switch f {
	case FamilyDS3:
		return "ds3"
	case FamilyQS1:
		return "qs1"
	case FamilyYC600:
		return "yc600"
	case FamilyYC1000:
		return "yc1000"
	case FamilyQT2:
		return "qt2"
	default:
		return ""
	}
}

// FamilyOf returns the Family for an APsystems numeric model code, or
// FamilyUnknown for codes not yet classified. The mapping is anchored
// off the per-family dispatch switches in protection.go, setpower.go
// and protection_read.go — adding a new code here without a matching
// encoder will not magically make it writable.
func FamilyOf(modelCode uint8) Family {
	switch modelCode {
	case ModelDS3, ModelDS3H, ModelDS3L, ModelExt36:
		return FamilyDS3
	case ModelQS1, ModelQS1A:
		return FamilyQS1
	case ModelYC600Old, ModelYC600, ModelYC600B:
		return FamilyYC600
	case ModelYC1000:
		return FamilyYC1000
	case ModelQT2:
		return FamilyQT2
	default:
		return FamilyUnknown
	}
}

// IsKnownModel reports whether modelCode has a non-Unknown Family
// mapping. The three-phase Ext29/30/31 codes are deliberately not
// included — their wire family is not yet pinned.
func IsKnownModel(modelCode uint8) bool {
	return FamilyOf(modelCode) != FamilyUnknown
}

// FamilyForModelString parses one of the human model strings the codec
// emits on wire telemetry ("DS3", "QS1A", "unknown(0x..)", "") and
// returns the matching Family. It is the bridge for callers that only
// see the string form on the proto wire and have no model code at hand.
//
// Matching is by prefix to absorb minor variants ("DS3-H", "QS1A").
// Unknown / empty input returns FamilyUnknown.
func FamilyForModelString(model string) Family {
	if model == "" {
		return FamilyUnknown
	}
	m := strings.ToUpper(model)
	switch {
	case strings.HasPrefix(m, "QS1"): // QS1, QS1A — both QS1 family
		return FamilyQS1
	case strings.HasPrefix(m, "DS3"), strings.HasPrefix(m, "DSP"):
		return FamilyDS3
	case strings.HasPrefix(m, "YC600"):
		return FamilyYC600
	case strings.HasPrefix(m, "YC1000"):
		return FamilyYC1000
	case strings.HasPrefix(m, "QT2"):
		return FamilyQT2
	}
	return FamilyUnknown
}

// BroadcastModelCodes returns the sorted list of model codes that have
// a working broadcast protection encoder (EncodeSetProtectionBroadcast
// can build a frame for them). The list is anchored off protection.go's
// dispatch — currently the DS3 family (0x20/0x21/0x22/0x36) and the QS1
// family (0x08/0x18). It is NOT extended speculatively to families that
// lack a broadcast encoder; adding a code here without a matching
// encoder would produce wire frames that the firmware rejects.
func BroadcastModelCodes() []uint8 {
	codes := []uint8{
		ModelQS1, ModelQS1A,
		ModelDS3, ModelDS3H, ModelDS3L, ModelExt36,
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	return codes
}

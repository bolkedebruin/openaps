package gridprofile

import (
	"errors"
	"fmt"

	"github.com/bolke/inv-driver/codec"
)

// familyHardRejects lists, per family, the codes the firmware silently no-ops on
// every write path even though the codec can encode them (confirmed by live
// probe; rejected at the device level, not the protocol level).
var familyHardRejects = map[codec.Family]map[string]bool{
	codec.FamilyQS1: {
		"CA": true, // Over_frequency_Watt_Start_set (canonical DbOf write)
		"DC": true, // decode-only DbOf alias (same physical param as CA)
		"CG": true, // Over_frequency_Watt_Delay_Time_set
		"CF": true, // not in the QS1 encoder set
	},
}

// family returns the protection family for a model code, expressed as
// the legacy string key ("DS3" / "QS1" / "") so existing log strings
// and error messages stay stable. The classification is delegated to
// codec.FamilyOf so adding a model code is a single-line edit in codec.
func family(modelCode uint8) string {
	switch codec.FamilyOf(modelCode) {
	case codec.FamilyDS3:
		return "DS3"
	case codec.FamilyQS1:
		return "QS1"
	}
	return ""
}

func isDS3Family(modelCode uint8) bool { return codec.FamilyOf(modelCode) == codec.FamilyDS3 }
func isQS1Family(modelCode uint8) bool { return codec.FamilyOf(modelCode) == codec.FamilyQS1 }

// unsupportedReason returns a human-readable explanation for why a code is
// not writable for the given model, or "" if it is writable.
func unsupportedReason(modelCode uint8, apsCode string) string {
	// 1. Check the forward map — only mapped codes can be dispatched.
	entry, ok := forwardMap[apsCode]
	if !ok {
		return fmt.Sprintf("aps_code %q not in forward map", apsCode)
	}
	if entry.LongName == "" {
		return fmt.Sprintf("aps_code %q has no firmware encoder (LongName empty)", apsCode)
	}

	// 2. Only families with a protection encoder are dispatchable.
	fam := codec.FamilyOf(modelCode)
	if fam != codec.FamilyDS3 && fam != codec.FamilyQS1 {
		return fmt.Sprintf("model 0x%02X has no protection encoder", modelCode)
	}

	// 3. Family firmware hard-rejects (device-level no-op the codec can't detect).
	if familyHardRejects[fam][apsCode] {
		return fmt.Sprintf("aps_code %q rejected by %s firmware on all write paths", apsCode, family(modelCode))
	}

	// 4. CA is rejected on QS1A via qs1aHardRejects (above); for DS3 the
	//    codec probe (step 5) is the authority. No special-case needed here.

	// 5. Probe the codec to confirm the (family, longName) pair is ENCODABLE.
	//    Only an unsupported-param/family error means "not writable". A
	//    value-range (physical envelope) or bit-width error from the fixed
	//    probe value just means 50.0 is unsuitable for THIS param's unit (e.g.
	//    50 V is below the voltage envelope) — the param is still writable, so
	//    those errors must NOT mark it unsupported.
	_, err := codec.EncodeSetProtection(modelCode, entry.LongName, 50.0)
	if errors.Is(err, codec.ErrUnsupportedProtectionParam) ||
		errors.Is(err, codec.ErrUnsupportedProtectionFamily) {
		return fmt.Sprintf("codec.EncodeSetProtection rejects %q for model 0x%02X: %v", entry.LongName, modelCode, err)
	}
	return ""
}

// Writeable reports whether apsCode can be written to an inverter of the
// given modelCode.  Returns false for any unsupported code/family combination.
func Writeable(modelCode uint8, apsCode string) bool {
	return unsupportedReason(modelCode, apsCode) == ""
}

// UnsupportedReason returns the human-readable reason a code is not writable,
// or "" if it is writable.  Useful for structured error messages.
func UnsupportedReason(modelCode uint8, apsCode string) string {
	return unsupportedReason(modelCode, apsCode)
}

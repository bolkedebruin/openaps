package gridprofile

import (
	"errors"
	"fmt"

	"github.com/bolke/inv-driver/codec"
)

// familyDS3 groups the DS3 model codes (all accept the same protection param
// set, differing only in power rating).
func isDS3Family(modelCode uint8) bool {
	switch modelCode {
	case codec.ModelDS3, codec.ModelDS3H, codec.ModelDS3L, codec.ModelExt36:
		return true
	}
	return false
}

// isQS1Family returns true for QS1 and QS1A model codes.
func isQS1Family(modelCode uint8) bool {
	return modelCode == codec.ModelQS1 || modelCode == codec.ModelQS1A
}

// qs1aHardRejects lists APsystems codes that the QS1A firmware silently
// no-ops on every write path (per-inverter unicast AND gridProfile broadcast).
// Confirmed by live probe on 806000042582; the firmware rejects at the device
// level, not at the protocol level.
var qs1aHardRejects = map[string]bool{
	"CA": true, // Over_frequency_Watt_Start_set (canonical DbOf write) — reject on QS1A
	"DC": true, // decode-only DbOf alias — reject on QS1A (same physical param as CA)
	"CG": true, // Over_frequency_Watt_Delay_Time_set — reject on QS1A
	"CF": true, // not in QS1 encoder set; reject on QS1A
}

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

	// 2. EncodeSetProtection only handles DS3 and QS1 families.
	if !isDS3Family(modelCode) && !isQS1Family(modelCode) {
		return fmt.Sprintf("model 0x%02X has no protection encoder", modelCode)
	}

	// 3. QS1A hard-rejects for DC/CG/CF at firmware level.
	if isQS1Family(modelCode) && qs1aHardRejects[apsCode] {
		return fmt.Sprintf("aps_code %q rejected by QS1A firmware on all write paths", apsCode)
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

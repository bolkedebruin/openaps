package codec

import (
	"errors"
	"fmt"
)

// ErrUnsupportedProtectionFamily is returned when the model code has no
// known protection-write encoder.
var ErrUnsupportedProtectionFamily = errors.New("set-protection: unsupported model family")

// ErrUnsupportedProtectionParam is returned when there is no single-frame
// encoding for (family, paramName). This covers params genuinely absent
// on a family (e.g. Over_frequency_Watt_Start_set has no DS3 branch in
// main.exe) AND params whose firmware encoding is env-gated or multi-frame
// (the over-frequency mode enum, the slope/delay terms, the reconnect
// time) — those are deliberately not emitted here until their device-state
// dependence is resolved and validated on-wire.
var ErrUnsupportedProtectionParam = errors.New("set-protection: param not encodable for this family")

// IsProtectionWrite reports whether an L2 frame is a grid-protection
// write (as opposed to set-power or on/off), so the driver can schedule a
// read-back after one. The opcode/sub-byte that distinguishes a
// protection write from set-power is family-specific, so the predicates
// live in ds3.go / qs1a.go; this only parses the frame and combines them.
func IsProtectionWrite(frame []byte) bool {
	if len(frame) < 5 {
		return false
	}
	cmd, sub := frame[3], frame[4]
	return isDS3ProtectionWrite(cmd, sub) || isQS1ProtectionWrite(cmd, sub)
}

// EncodeSetProtection returns the unicast L2 frame(s) that program one
// grid-protection param on a given inverter. value is in the param's
// native unit: Hz for the frequency thresholds, %P/Hz for the slope,
// seconds for the delay/reconnect, the mode enum for the over-frequency
// switch. It mirrors main.exe's per-inverter dispatcher
// (set_paraName_paraValue_inverter @ 0x69bdc, unicast), not the broadcast
// profile path; the per-family encoders live in ds3.go / qs1a.go.
//
// Returns a slice because a few params are multi-frame on the wire (the
// QS1 over-frequency mode enum emits 2-3 frames); single-frame params
// return a one-element slice. Caller sends each frame in order.
//
// grid_recovery_time uses the line-frequency cycle count on DS3 — encoded
// for 50 Hz (env 0/1); a 60 Hz site would need ×60 (see ds3.go).
func EncodeSetProtection(modelCode uint8, paramName string, value float64) ([][]byte, error) {
	if value <= 0 {
		return nil, fmt.Errorf("set-protection: %q: value must be > 0 (got %g)", paramName, value)
	}
	switch modelCode {
	case ModelDS3, ModelDS3H, ModelDS3L, ModelExt36:
		return encodeProtectionDS3(paramName, value)
	case ModelQS1, ModelQS1A:
		return encodeProtectionQS1A(paramName, value)
	default:
		return nil, fmt.Errorf("%w: model 0x%02X", ErrUnsupportedProtectionFamily, modelCode)
	}
}

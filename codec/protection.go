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

// EncodeSetProtection returns the unicast L2 frame that programs one
// grid-protection param on a given inverter. value is in the param's
// native unit: Hz for the frequency thresholds, %P/Hz for the slope,
// seconds for the delay. It mirrors main.exe's per-inverter dispatcher
// (set_paraName_paraValue_inverter @ 0x69bdc, unicast), not the broadcast
// profile path; the per-family encoders live in ds3.go / qs1a.go.
//
// Encoded today: frequency thresholds (CB/CC/DH/DI both families; CA on
// QS1A only) plus DS3 slope (CF) and DS3 delay (CG). Params whose
// firmware encoding is multi-frame (the over-frequency mode enum) or
// needs the env/line-frequency multiplier on a non-DS3 path
// (grid_recovery_time) still return ErrUnsupportedProtectionParam.
func EncodeSetProtection(modelCode uint8, paramName string, value float64) ([]byte, error) {
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

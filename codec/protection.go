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
// grid-protection frequency threshold on a given inverter. hz is the
// absolute threshold frequency (e.g. 50.3). It mirrors main.exe's
// per-inverter dispatcher (set_paraName_paraValue_inverter @ 0x69bdc,
// unicast), not the broadcast profile path; the per-family encoders live
// in ds3.go / qs1a.go.
//
// Only the single-frame frequency-threshold params (CA/CB/CC/DH/DI) are
// encoded. Params whose firmware encoding is env-gated or multi-frame
// (Over_frequency_Watt_set, Over_Frequency_Watt_Slope_set,
// Over_frequency_Watt_Delay_Time_set, grid_recovery_time) return
// ErrUnsupportedProtectionParam.
func EncodeSetProtection(modelCode uint8, paramName string, hz float64) ([]byte, error) {
	if hz <= 0 {
		return nil, fmt.Errorf("set-protection: %q: frequency must be > 0 (got %g)", paramName, hz)
	}
	switch modelCode {
	case ModelDS3, ModelDS3H, ModelDS3L, ModelExt36:
		return encodeProtectionDS3(paramName, hz)
	case ModelQS1, ModelQS1A:
		return encodeProtectionQS1A(paramName, hz)
	default:
		return nil, fmt.Errorf("%w: model 0x%02X", ErrUnsupportedProtectionFamily, modelCode)
	}
}

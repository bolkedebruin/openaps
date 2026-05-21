package codec

import (
	"errors"
	"fmt"
)

// ErrUnsupportedFamily is returned by EncodeSetPower when modelCode does
// not match a known set-power encoder family.
var ErrUnsupportedFamily = errors.New("set-power: unsupported model family")

// ErrPanelWattsOutOfRange is returned by the set-power encoders when
// panelWatts is not in [MinPanelLimitW, MaxPanelLimitW].
var ErrPanelWattsOutOfRange = errors.New("set-power: panel watts out of range (want 20..500)")

func checkPanelWatts(panelWatts uint16) error {
	if panelWatts < MinPanelLimitW || panelWatts > MaxPanelLimitW {
		return fmt.Errorf("%w: got %d", ErrPanelWattsOutOfRange, panelWatts)
	}
	return nil
}

// EncodeSetPower picks the right encoder for the given inverter model
// code and returns the canonical L2 frame for the configured per-panel
// watt setpoint.
//
// Known model codes:
//
//	0x05, 0x06, 0x07, 0x17       -> C3 family (YC600 / YC1000)
//	0x08, 0x18                   -> QS1-old / QS1A
//	0x20, 0x21, 0x22, 0x36       -> DS3 family
//
// Three-phase models (0x29..0x32) are not supported here.
func EncodeSetPower(modelCode uint8, panelWatts uint16, broadcast bool) ([]byte, error) {
	switch modelCode {
	case ModelYC600Old, ModelYC1000, ModelYC600, ModelYC600B:
		return EncodeSetPowerC3(modelCode, panelWatts, broadcast)
	case ModelQS1, ModelQS1A:
		return EncodeSetPowerQS1A(panelWatts, broadcast)
	case ModelDS3, ModelDS3H, ModelDS3L, ModelExt36:
		return EncodeSetPowerDS3(panelWatts, broadcast)
	default:
		return nil, fmt.Errorf("%w: model 0x%02X", ErrUnsupportedFamily, modelCode)
	}
}

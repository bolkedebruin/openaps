package codec

import (
	"errors"
	"fmt"
	"math"
)

// ErrUnsupportedFamily is returned by EncodeSetPower when modelCode does
// not match a known set-power encoder family.
var ErrUnsupportedFamily = errors.New("set-power: unsupported model family")

// ErrPanelWattsOutOfRange is returned by the set-power encoders when
// panelWatts is not in [20, 500]. The bound mirrors the firmware-imposed
// per-panel watt gate.
var ErrPanelWattsOutOfRange = errors.New("set-power: panel watts out of range (want 20..500)")

const (
	setPowerScaleDSP     uint32  = 0x1CE3 // 7395, used by QS1-old / QS1A and YC600/YC1000 encoders
	setPowerScaleDS3Inv  float64 = 0.06027
	setPowerClampDS3     uint32  = 0x474C
	setPowerWattsMinimum uint16  = 20
	setPowerWattsMaximum uint16  = 500
)

func checkPanelWatts(panelWatts uint16) error {
	if panelWatts < setPowerWattsMinimum || panelWatts > setPowerWattsMaximum {
		return fmt.Errorf("%w: got %d", ErrPanelWattsOutOfRange, panelWatts)
	}
	return nil
}

// EncodeSetPowerDSPNew builds the L2 frame to set max-power on the
// QS1-old / QS1A family (model codes 0x08 and 0x18). broadcast=true
// emits opcode 0x2C; broadcast=false emits 0x1C. Sub-code 0x8C selects
// the max-power parameter; value length is 2.
func EncodeSetPowerDSPNew(panelWatts uint16, broadcast bool) ([]byte, error) {
	if err := checkPanelWatts(panelWatts); err != nil {
		return nil, err
	}
	val := (uint32(panelWatts) * setPowerScaleDSP) >> 8
	body := []byte{
		0x8C,
		0x02,
		byte(val >> 8),
		byte(val & 0xFF),
		0x00,
	}
	cmd := byte(0x1C)
	if broadcast {
		cmd = 0x2C
	}
	return BuildL2Frame(0x06, cmd, body), nil
}

// EncodeSetPowerDS3 builds the L2 frame to set max-power on the DS3
// family (model codes 0x20, 0x21, 0x22, 0x36). broadcast=true emits
// opcode 0xAB; broadcast=false emits 0xAA. Sub-code 0x27 selects the
// "DA" / Rated_power parameter. Value at offsets [7][8], reserved zeros
// at [5][6].
func EncodeSetPowerDS3(panelWatts uint16, broadcast bool) ([]byte, error) {
	if err := checkPanelWatts(panelWatts); err != nil {
		return nil, err
	}
	val := uint32(math.Round(float64(panelWatts) / setPowerScaleDS3Inv))
	if val > setPowerClampDS3 {
		val = setPowerClampDS3
	}
	body := []byte{
		0x27,
		0x00,
		0x00,
		byte(val >> 8),
		byte(val & 0xFF),
	}
	cmd := byte(0xAA)
	if broadcast {
		cmd = 0xAB
	}
	return BuildL2Frame(0x06, cmd, body), nil
}

// EncodeSetPowerC3 builds the L2 frame to set max-power on the C3
// family (YC600 / YC1000 — model codes 5, 6, 7, 0x17). broadcast=true
// emits opcode 0xA3; broadcast=false emits 0xC3. Models 7 and 0x17
// apply an extra `>> 2` to the value.
func EncodeSetPowerC3(modelCode uint8, panelWatts uint16, broadcast bool) ([]byte, error) {
	if err := checkPanelWatts(panelWatts); err != nil {
		return nil, err
	}
	val := (uint32(panelWatts) * setPowerScaleDSP) >> 14
	if modelCode == 0x07 || modelCode == 0x17 {
		val >>= 2
	}
	body := []byte{byte(val & 0xFF), 0x00, 0x00, 0x00, 0x00}
	cmd := byte(0xC3)
	if broadcast {
		cmd = 0xA3
	}
	return BuildL2Frame(0x06, cmd, body), nil
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
	case 0x05, 0x06, 0x07, 0x17:
		return EncodeSetPowerC3(modelCode, panelWatts, broadcast)
	case 0x08, 0x18:
		return EncodeSetPowerDSPNew(panelWatts, broadcast)
	case 0x20, 0x21, 0x22, 0x36:
		return EncodeSetPowerDS3(panelWatts, broadcast)
	default:
		return nil, fmt.Errorf("%w: model 0x%02X", ErrUnsupportedFamily, modelCode)
	}
}

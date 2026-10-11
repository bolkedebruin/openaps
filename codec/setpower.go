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

// IsSetPower reports whether an L2 frame is a per-inverter set-power (output
// cap) write, so the ingest layer can re-read protection afterwards (the cap
// reads back as code "DA"). It matches the unicast set-power opcodes: DS3 0xAA
// with the max-power sub, QS1 0x1C with the max-power sub, and C3 0xC3. (DS3/QS1
// share their opcode with protection writes, distinguished by the sub byte.)
func IsSetPower(frame []byte) bool {
	if len(frame) < 5 {
		return false
	}
	cmd, sub := frame[3], frame[4]
	switch cmd {
	case CmdSetPowerDS3Unicast:
		return sub == SubMaxPowerDS3
	case CmdSetPowerQS1Unicast:
		return sub == SubMaxPowerQS1
	case CmdSetPowerC3Unicast:
		return true
	}
	return false
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
	case ModelDS3, ModelDS3H, ModelDS3L, ModelQS2:
		return EncodeSetPowerDS3(panelWatts, broadcast)
	default:
		return nil, fmt.Errorf("%w: model 0x%02X", ErrUnsupportedFamily, modelCode)
	}
}

// DecodeSetPower recovers the per-panel watt setpoint from a DS3- or
// QS1-family set-power frame (unicast or broadcast), the inverse of
// EncodeSetPowerDS3 / EncodeSetPowerQS1A. It reports the family the frame
// addresses and whether it is a broadcast. ok=false for any other frame,
// including C3-family set-power: those inverters do not read the cap back,
// so there is nothing to compare a decoded setpoint against.
func DecodeSetPower(frame []byte) (fam Family, panelW float64, broadcast bool, ok bool) {
	if len(frame) < 9 {
		return FamilyUnknown, 0, false, false
	}
	cmd, sub := frame[3], frame[4]
	switch {
	case (cmd == CmdSetPowerDS3Unicast || cmd == CmdSetPowerDS3Broadcast) && sub == SubMaxPowerDS3:
		raw := int(frame[7])<<8 | int(frame[8])
		return FamilyDS3, dsMaxPower(raw), cmd == CmdSetPowerDS3Broadcast, true
	case (cmd == CmdSetPowerQS1Unicast || cmd == CmdSetPowerQS1Broadcast) && sub == SubMaxPowerQS1:
		raw := int(frame[6])<<8 | int(frame[7])
		return FamilyQS1, qsMaxPower(raw), cmd == CmdSetPowerQS1Broadcast, true
	}
	return FamilyUnknown, 0, false, false
}

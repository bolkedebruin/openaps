package codec

const setPowerScaleC3Shift = 14

// isYC600VariantWithExtraShift reports whether the C3-family model code
// requires an additional `>> 2` on the encoded register value. Empirically
// models 0x07 (YC600) and 0x17 (YC600B / YC1000-30/60 variants) need it;
// the other C3 codes don't.
func isYC600VariantWithExtraShift(modelCode uint8) bool {
	return modelCode == ModelYC600 || modelCode == ModelYC600B
}

// EncodeSetPowerC3 builds the L2 frame to set max-power on the C3
// family (YC600 / YC1000 — model codes 0x05, 0x06, 0x07, 0x17).
// broadcast=true emits CmdSetPowerC3Broadcast; broadcast=false emits
// CmdSetPowerC3Unicast.
func EncodeSetPowerC3(modelCode uint8, panelWatts uint16, broadcast bool) ([]byte, error) {
	if err := checkPanelWatts(panelWatts); err != nil {
		return nil, err
	}
	val := (uint32(panelWatts) * setPowerScaleDSP) >> setPowerScaleC3Shift
	if isYC600VariantWithExtraShift(modelCode) {
		val >>= 2
	}
	body := []byte{byte(val & 0xFF), 0x00, 0x00, 0x00, 0x00}
	cmd := CmdSetPowerC3Unicast
	if broadcast {
		cmd = CmdSetPowerC3Broadcast
	}
	return BuildL2Frame(cmd, body), nil
}

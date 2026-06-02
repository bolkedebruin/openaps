package codec

// onOffBodyLen is the fixed body length of an on/off frame: 5 zero
// bytes. With the 1-byte command that makes inner_len 0x06.
const onOffBodyLen = 5

// EncodeOnOff returns the L2 frame that switches an inverter on or off.
// The command is family-independent (see opcodes.go), so unlike
// EncodeSetPower it needs no model code and cannot fail — the body is a
// fixed run of zeros and the checksum is derived by BuildL2Frame.
//
// Worked frames:
//
//	on,  unicast    → FB FB 06 C1 00 00 00 00 00 00 C7 FE FE
//	off, unicast    → FB FB 06 C2 00 00 00 00 00 00 C8 FE FE
//	on,  broadcast  → FB FB 06 A1 00 00 00 00 00 00 A7 FE FE
//	off, broadcast  → FB FB 06 A2 00 00 00 00 00 00 A8 FE FE
func EncodeOnOff(on, broadcast bool) []byte {
	cmd := CmdTurnOffUnicast // !on && !broadcast
	switch {
	case on && broadcast:
		cmd = CmdTurnOnBroadcast
	case on:
		cmd = CmdTurnOnUnicast
	case broadcast:
		cmd = CmdTurnOffBroadcast
	}
	return BuildL2Frame(cmd, make([]byte, onOffBodyLen))
}

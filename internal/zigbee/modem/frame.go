// Package modem brings up the ZigBee radio(s) ecu-zb owns on the ECU.
//
// Two radio variants share one wire framing and differ only in their
// bring-up op sequence and device source:
//
//   - apsystems: the ECU's built-in custom-firmware module on /dev/ttyO2.
//     Bring-up = 0x0D liveness ping (+HW reset on failure) then 0x05
//     Set-PANID+Channel.
//   - plain: a USB-attached stock ZigBee module. Bring-up = 0x79
//     get-config (learns the module's ext address, sets PAN/channel)
//     then 0x0D ping. Not yet wired; the frame builder is here so it
//     can be added next to the apsystems one.
package modem

// Config-frame ops. Each occupies byte[4] of the L1 envelope, the slot the
// inverter-query sentinel (0x55) uses for data frames.
const (
	opPing            byte = 0x0D // zb_test_communication: liveness check
	opSetPanidChannel byte = 0x05 // zb_change_ecu_panid: coordinator config
	opGetConfig       byte = 0x79 // init_zigbee2/get_config_of_zigbee2 (plain modem)
)

// ackByte is the module's positive-reply marker for any config op. The
// full reply is three bytes whose first byte is 0xAB; success is checked
// on this first byte only. Bytes 2-3 identify the radio variant — a
// CC2652 returns AB 26 52 ("&R"), while this ECU's CC2530 returns
// AB CD EF. We must not demand a specific variant.
const ackByte = 0xAB

// ackLen is the expected config-op reply length, used to confirm a full
// ack arrived rather than a stray 0xAB byte.
const ackLen = 3

// findAck reports the offset of a complete config-op ack within buf (a
// 0xAB byte followed by at least two more bytes), or -1 if none yet.
func findAck(buf []byte) int {
	for i, b := range buf {
		if b == ackByte && len(buf)-i >= ackLen {
			return i
		}
	}
	return -1
}

// buildConfigFrame builds the fixed 15-byte L1 modem-config frame:
//
//	AA AA AA AA | op | p0 p1 p2 p3 p4 p5 p6 | ckHi ckLo 00
//
// The checksum is the byte-wise sum of op plus the seven param bytes
// (frame bytes [4..11]), packed big-endian into bytes [12],[13]; byte
// [14] is zero.
func buildConfigFrame(op byte, params [7]byte) []byte {
	f := make([]byte, 15)
	f[0], f[1], f[2], f[3] = 0xAA, 0xAA, 0xAA, 0xAA
	f[4] = op
	copy(f[5:12], params[:])
	sum := uint16(op)
	for _, b := range params {
		sum += uint16(b)
	}
	f[12] = byte(sum >> 8)
	f[13] = byte(sum & 0xFF)
	f[14] = 0
	return f
}

// buildPing builds the 0x0D liveness ping (all-zero params).
func buildPing() []byte {
	return buildConfigFrame(opPing, [7]byte{})
}

// buildSetPanidChannel builds the 0x05 frame that puts the radio into
// coordinator mode on the given PAN ID and RF channel. The param layout
// (frame bytes [5..11]) is: 00 00 PANhi PANlo CH 00 00.
func buildSetPanidChannel(pan uint16, channel byte) []byte {
	return buildConfigFrame(opSetPanidChannel, [7]byte{
		0, 0,
		byte(pan >> 8), byte(pan & 0xFF),
		channel,
		0, 0,
	})
}

// buildGetConfig builds the 0x79 get-config frame used by the plain/USB
// modem bring-up. Same param layout as 0x05. Provided for the second
// modem backend; not used by the apsystems bring-up.
func buildGetConfig(pan uint16, channel byte) []byte {
	return buildConfigFrame(opGetConfig, [7]byte{
		0, 0,
		byte(pan >> 8), byte(pan & 0xFF),
		channel,
		0, 0,
	})
}

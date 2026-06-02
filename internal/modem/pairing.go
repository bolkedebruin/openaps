package modem

import (
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/bolke/inv-driver/codec"
)

// serialRe validates a 12-digit DECIMAL serial. A 12-digit decimal serial is
// exactly the hex encoding of the 6 IEEE bytes (every nibble is a decimal
// digit), so hex.DecodeString round-trips it byte-for-byte — but we must
// reject a-f first, since hex.DecodeString would wrongly accept them.
var serialRe = regexp.MustCompile(`^[0-9]{12}$`)

// Pairing/repairing over-the-air ops. These frames travel host→module→OTA
// to reach the inverters (unlike the 0x05/0x0D/0x79 config ops which only
// reach the local module).
//
//   - 0x0E directed short-address query for a known IEEE
//   - 0x0F directed PAN/channel set for a known IEEE
//   - 0x11 directed prime: announce a new PAN/channel to an IEEE
//   - 0x22 broadcast commit: primed inverters jump to the new PAN
//   - 0xD1/0x1D/0xD2 discovery broadcast/announce/turn-off
//   - 0xD3 quiet broadcast (stop discovery)
//   - 0x08 bind by short address + turn report-id off
const (
	opGetInverterShortAddr byte = 0x0E // 0x0E directed: query a known IEEE for its short addr
	opSetInverterPan       byte = 0x0F // 0x0F directed: force an IEEE onto a PAN/channel
	opPrimeInverterPan     byte = 0x11 // 0x11 directed: prime an IEEE with the new PAN/channel
	opCommitPanNow         byte = 0x22 // 0x22 broadcast: primed inverters jump to the new PAN
	opReportIdOn           byte = 0xD1 // 0xD1 broadcast: turn report-id ON (discovery solicit)
	opReportIdOff          byte = 0xD2 // 0xD2: turn report-id OFF for a list of found IEEEs
	opReportIdQuiet        byte = 0xD3 // 0xD3 broadcast: stop-report quiet
	opBindZigbee           byte = 0x08 // 0x08 directed: bind + turn report-id off (by short addr)
)

// Tag bytes that follow the L1 config checksum for the directed OTA ops.
const (
	tagIeee6  byte = 0x06 // 0x0E/0x0F: tag + 6-byte IEEE (21-byte frame)
	tagIeee8  byte = 0x08 // 0x11: tag + 6-byte IEEE + 0x01 trailer (23-byte frame)
	ieeePadA0 byte = 0xA0 // last config param byte for 0x0E/0x0F/0x08
)

// reportIdOnReply / discovery reply markers.
const (
	announceTag    byte = 0x1D // 0x1D 0x1D ... = an inverter announcing itself
	bindAckByte    byte = 0xA5 // bytes [2][3] == A5 A5 in the 0x08 success reply
	broadcastIeeeB byte = 0xFF // broadcast IEEE filler in 0xD1/0xD3
)

// bytesum8 returns the big-endian 16-bit byte-wise sum of f[4:12] (op plus
// the seven config-param bytes). It is split hi,lo big-endian.
func bytesum8(f []byte) (hi, lo byte) {
	var sum uint16
	for _, b := range f[4:12] {
		sum += uint16(b)
	}
	return byte(sum >> 8), byte(sum & 0xFF)
}

// serialToBCD6 packs a 12-digit decimal serial into the 6-byte radio IEEE
// address: two decimal digits per byte, high nibble first. Each nibble is
// a decimal digit — which is precisely hex decoding. We validate the
// all-decimal form first (hex.DecodeString would wrongly accept a-f), then
// hex-decode. The serial must be exactly 12 ASCII decimal digits.
func serialToBCD6(serial string) ([6]byte, error) {
	var out [6]byte
	if !serialRe.MatchString(serial) {
		return out, fmt.Errorf("serial %q: want 12 decimal digits", serial)
	}
	b, err := hex.DecodeString(serial)
	if err != nil {
		return out, fmt.Errorf("serial %q: %w", serial, err)
	}
	copy(out[:], b)
	return out, nil
}

// bcd6ToSerial reverses serialToBCD6: each nibble is a decimal digit, so the
// 6 bytes hex-encode back to the 12-digit decimal serial.
func bcd6ToSerial(ieee [6]byte) string {
	return hex.EncodeToString(ieee[:])
}

// updateCRC advances a CRC-16/CCITT-FALSE running value by one byte
// (poly 0x1021, MSB-first, no reflection).
func updateCRC(crc uint16, b byte) uint16 {
	crc ^= uint16(b) << 8
	for i := 0; i < 8; i++ {
		if crc&0x8000 != 0 {
			crc = crc<<1 ^ 0x1021
		} else {
			crc <<= 1
		}
	}
	return crc
}

// crc16 computes CRC-16/CCITT-FALSE over b (init 0xFFFF).
func crc16(b []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, x := range b {
		crc = updateCRC(crc, x)
	}
	return crc
}

// buildGetInverterShortAddr builds the 0x0E directed query (21 bytes):
//
//	AA AA AA AA 0E 00 00 00 00 00 00 A0 ckHi ckLo 06 <IEEE 6>
//
// The seven config params are 00 00 00 00 00 00 A0; the checksum covers
// f[4:12] (op + params); tag 0x06 and the 6-byte IEEE follow the checksum.
func buildGetInverterShortAddr(ieee [6]byte) []byte {
	f := make([]byte, 0, 21)
	f = append(f, 0xAA, 0xAA, 0xAA, 0xAA,
		opGetInverterShortAddr,
		0, 0, 0, 0, 0, 0, ieeePadA0)
	hi, lo := bytesum8(f)
	f = append(f, hi, lo, tagIeee6)
	f = append(f, ieee[:]...)
	return f
}

// buildSetInverterPan builds the 0x0F directed set-PAN/channel (21 bytes):
//
//	AA AA AA AA 0F 00 00 PANhi PANlo CH 00 A0 ckHi ckLo 06 <IEEE 6>
//
// PANhi/PANlo (the ECU's mac_tail_u16) go at params[2..3] and the channel
// at params[4]; params[5]=00, params[6]=A0. pan may be 0xFFFF for the
// rendezvous PAN.
func buildSetInverterPan(pan uint16, channel byte, ieee [6]byte) []byte {
	f := make([]byte, 0, 21)
	f = append(f, 0xAA, 0xAA, 0xAA, 0xAA,
		opSetInverterPan,
		0, 0,
		byte(pan>>8), byte(pan&0xFF), channel,
		0, ieeePadA0)
	hi, lo := bytesum8(f)
	f = append(f, hi, lo, tagIeee6)
	f = append(f, ieee[:]...)
	return f
}

// buildPrimeInverterPan builds the 0x11 prime frame (23 bytes):
//
//	AA AA AA AA 11 00 00 PANhi PANlo CH 00 00 ckHi ckLo 08 <IEEE 6> 00 01
//
// Differs from 0x0E/0x0F: params[5]=00 and params[6]=00 (not A0); the tag
// after the checksum is 0x08, and the frame ends with 0x00 0x01 after the
// IEEE (the trailing 0x00 is the high byte of the ushort IEEE[5] store;
// the 0x01 is the prime trailer).
func buildPrimeInverterPan(pan uint16, channel byte, ieee [6]byte) []byte {
	f := make([]byte, 0, 23)
	f = append(f, 0xAA, 0xAA, 0xAA, 0xAA,
		opPrimeInverterPan,
		0, 0,
		byte(pan>>8), byte(pan&0xFF), channel,
		0, 0)
	hi, lo := bytesum8(f)
	f = append(f, hi, lo, tagIeee8)
	f = append(f, ieee[:]...)
	f = append(f, 0x00, 0x01)
	return f
}

// buildCommitPanNow builds the 0x22 broadcast commit (15 bytes):
//
//	AA AA AA AA 22 00 00 PANhi PANlo CH 00 00 ckHi ckLo 00
//
// The commit frame is sent three times; the caller is responsible for
// the repetition (sendCommitPanNow does so).
func buildCommitPanNow(pan uint16, channel byte) []byte {
	f := make([]byte, 0, 15)
	f = append(f, 0xAA, 0xAA, 0xAA, 0xAA,
		opCommitPanNow,
		0, 0,
		byte(pan>>8), byte(pan&0xFF), channel,
		0, 0)
	hi, lo := bytesum8(f)
	f = append(f, hi, lo, 0x00)
	return f
}

// buildReportIdOn builds the 0xD1 report-id-ON broadcast (21 bytes):
//
//	AA AA AA AA D1 00 00 00 00 00 00 FF FF FF FF FF FF <count> <seq> crcHi crcLo
//
// The six 0xFF bytes are the broadcast destination IEEE. count is the
// remaining-to-find window byte capped at 0x3C; seq is a running
// sequence number. The trailer is CRC-16/CCITT-FALSE over f[4:19] (op
// through seq), NOT the bytesum used by the directed ops.
func buildReportIdOn(count, seq byte) []byte {
	f := make([]byte, 0, 21)
	f = append(f, 0xAA, 0xAA, 0xAA, 0xAA,
		opReportIdOn,
		0, 0, 0, 0, 0, 0,
		broadcastIeeeB, broadcastIeeeB, broadcastIeeeB,
		broadcastIeeeB, broadcastIeeeB, broadcastIeeeB,
		count, seq)
	crc := crc16(f[4:19])
	f = append(f, byte(crc>>8), byte(crc&0xFF))
	return f
}

// buildReportIdOff builds the 0xD2 report-id-OFF frame for a list of
// already-found IEEEs (length = 6*len(found)+10):
//
//	AA AA AA AA D2 01 <IEEE 6>... FF <seq> crcHi crcLo
//
// Each found inverter's 6-byte IEEE is appended after the 01 sub-byte; a
// 0xFF terminator, the running seq, and a CRC-16/CCITT-FALSE over
// f[4 : 4+6n+4] (op,01,IEEEs,FF,seq) follow. Matches autoSearchInverterID's
// per-round turn-off assembly.
func buildReportIdOff(found [][6]byte, seq byte) []byte {
	n := len(found)
	f := make([]byte, 0, 6*n+10)
	f = append(f, 0xAA, 0xAA, 0xAA, 0xAA, opReportIdOff, 0x01)
	for _, ieee := range found {
		f = append(f, ieee[:]...)
	}
	f = append(f, broadcastIeeeB) // terminator
	f = append(f, seq)
	crc := crc16(f[4 : 4+6*n+4]) // op,01,IEEEs,FF,seq
	f = append(f, byte(crc>>8), byte(crc&0xFF))
	return f
}

// buildReportIdQuiet builds the 0xD3 quiet broadcast (13 bytes):
//
//	AA AA AA AA D3 FF FF FF FF FF FF FF FD
//
// This fixed frame is written at the start and end of a discovery sweep
// to stop the broadcast. It is a constant — no checksum/CRC computation.
func buildReportIdQuiet() []byte {
	return []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		opReportIdQuiet,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFD,
	}
}

// buildBindZigbee builds the 0x08 bind/quiet frame (15 bytes) addressed by
// short address:
//
//	AA AA AA AA 08 SAhi SAlo 00 00 00 00 A0 ckHi ckLo 00
//
// The seven config params are SAhi SAlo 00 00 00 00 A0; checksum covers
// f[4:12]. A success reply has bytes [2]==[3]==0xA5.
func buildBindZigbee(shortAddr uint16) []byte {
	f := make([]byte, 0, 15)
	f = append(f, 0xAA, 0xAA, 0xAA, 0xAA,
		opBindZigbee,
		byte(shortAddr>>8), byte(shortAddr&0xFF),
		0, 0, 0, 0, ieeePadA0)
	hi, lo := bytesum8(f)
	f = append(f, hi, lo, 0x00)
	return f
}

// isEncryptedFrame reports whether a raw module reply arrived AES-wrapped.
// An AES data frame carries the FC FC inbound L1 marker with a gate byte
// (raw[12]) below 0xF0; a plaintext frame's gate byte coincides with the
// L2 SOF (0xFB) which is >= 0xF0-adjacent and is treated as cleartext. A
// reply that does not even start with the FC FC marker (e.g. a bare 1D 1D
// announcement the module already unwrapped) is plaintext.
func isEncryptedFrame(raw []byte) bool {
	if len(raw) < 13 {
		return false
	}
	if raw[0] != codec.L1ReplySOF || raw[1] != codec.L1ReplySOF {
		return false
	}
	return raw[12] < codec.CleartextGateMin
}

// parseShortAddrReply extracts the assigned short address from a 0x0E reply
// and verifies the reply is FOR the inverter we queried. The reply is
// validated by comparing its embedded IEEE against the requested serial,
// so a stray/mis-addressed reply can't bind a wrong short address.
//
// Reply layouts:
//   - inbound L1 frame  `FC FC <SAhi> <SAlo> <flag> <flag> <IEEE 6> ...`:
//     short addr at [2..3], IEEE at the L1 UID offset [6..11].
//   - bare reply        `<SAhi> <SAlo> <?> <flag> <IEEE 6> ...`:
//     short addr at [0..1], IEEE at [4..9].
//
// The assignment is accepted only when the short addr is below 0xFFF8
// (0xFFF8..0xFFFF are reserved). Returns ok=false if the frame is too short,
// the address is reserved, or the embedded IEEE does not match wantIEEE.
func parseShortAddrReply(reply []byte, wantIEEE [6]byte) (uint16, bool) {
	if len(reply) < 4 {
		return 0, false
	}
	var sa uint16
	var ieeeOff int
	if reply[0] == codec.L1ReplySOF && reply[1] == codec.L1ReplySOF {
		sa = uint16(reply[2])<<8 | uint16(reply[3])
		ieeeOff = 6
	} else {
		sa = uint16(reply[0])<<8 | uint16(reply[1])
		ieeeOff = 4
	}
	if sa >= 0xFFF8 {
		return 0, false
	}
	if len(reply) < ieeeOff+6 {
		return 0, false
	}
	for i := 0; i < 6; i++ {
		if reply[ieeeOff+i] != wantIEEE[i] {
			return 0, false
		}
	}
	return sa, true
}

// parseAnnounce parses one 0x1D 0x1D announcement reply, returning the
// announcing inverter's 6-byte IEEE. The reply layout (autoSearchInverterID)
// is `1D 1D <IEEE 6> crcHi crcLo`; the CRC-16/CCITT-FALSE residue over the
// 8 bytes IEEE+crc must be zero. Returns ok=false if the marker, length, or
// CRC check fails.
func parseAnnounce(reply []byte) (ieee [6]byte, ok bool) {
	if len(reply) < 10 {
		return ieee, false
	}
	if reply[0] != announceTag || reply[1] != announceTag {
		return ieee, false
	}
	// crc16 over IEEE(6)+crc(2) must be the CCITT residue (0).
	if crc16(reply[2:10]) != 0 {
		return ieee, false
	}
	copy(ieee[:], reply[2:8])
	return ieee, true
}

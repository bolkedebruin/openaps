package busmgr

import "github.com/bolke/inv-driver/codec"

// BuildL1Frame wraps an L2 body in the outer ZigBee envelope for the
// given short-address. SA=0 produces a broadcast envelope. The 16-bit
// outer checksum covers bytes [4..11] (the 0x55 SOF, the two SA bytes,
// and the five reserved zero bytes).
//
// Layout:
//
//	AA AA AA AA 55 <SA hi> <SA lo> 00 00 00 00 00 <sum hi> <sum lo> <len> <L2 body>
func BuildL1Frame(sa uint16, l2 []byte) []byte {
	saHi := byte(sa >> 8)
	saLo := byte(sa & 0xFF)
	sum := uint16(codec.L1Sentinel) + uint16(saHi) + uint16(saLo)
	out := make([]byte, 0, 15+len(l2))
	out = append(out,
		codec.L1Preamble, codec.L1Preamble, codec.L1Preamble, codec.L1Preamble,
		codec.L1Sentinel,
		saHi, saLo,
		0x00, 0x00, 0x00, 0x00, 0x00,
		byte(sum>>8), byte(sum&0xFF),
		byte(len(l2)&0xFF),
	)
	out = append(out, l2...)
	return out
}

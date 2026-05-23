package codec

import "encoding/hex"

// PairKind identifies the variety of a pair-flow frame.
type PairKind int

const (
	// PairOffReportBind is an outbound 0x08 frame addressed to a
	// known short_addr (per-inverter).
	PairOffReportBind PairKind = iota + 1
	// PairChangePan11 is an outbound 0x11 frame carrying a UID
	// (per-inverter).
	PairChangePan11
	// PairChangePan22 is an outbound 0x22 broadcast frame (no UID,
	// no short_addr filter).
	PairChangePan22
	// PairBindAck is an inbound reply whose L2 body has A5 A5 at
	// offsets 4-5 and a "rpt-off" flag at byte 9.
	PairBindAck
	// PairShortAddrReply is an inbound short-address discovery reply
	// carrying a BE16 short_addr at offsets 0-1 (< 0xFFF8) and a
	// 6-byte UID at offsets 4-9.
	PairShortAddrReply
)

// Direction is the L2 transport direction; pair opcodes overlap so
// parsers cannot infer direction from the body alone.
type Direction int

const (
	DirOutbound Direction = iota
	DirInbound
)

// PairFrame is the typed view of a matched pair-flow frame. Only the
// fields appropriate to Kind are set.
type PairFrame struct {
	Kind      PairKind
	ShortAddr uint16 // set for OffReportBind / ChangePan11 / ShortAddrReply
	PeerUID   string // 12-char hex; set for ChangePan11 / ShortAddrReply
	Bound     bool   // set true on BindAck match
	RptOff    bool   // for BindAck, value of byte 9
}

// ParsePairFrame inspects an L2 body (pair frames use a flat L0
// envelope, not the FB FB / FE FE L2 wrapper). The caller supplies
// the direction. Returns ok=false if no pair shape matched.
func ParsePairFrame(dir Direction, l2 []byte) (PairFrame, bool) {
	if dir == DirOutbound {
		return parseOutboundPair(l2)
	}
	return parseInboundPair(l2)
}

// parseOutboundPair handles 0x08 / 0x11 / 0x22 outbound L0 bodies.
//
//	[0..3] AA AA AA AA magic
//	[4]    opcode
//
// 0x08 OffReportBind, 15 bytes:
//
//	AA AA AA AA 08 <a_hi> <a_lo> 00 00 00 00 A0 <crc_hi> <crc_lo> 00
//
// 0x11 ChangePan11, 23 bytes:
//
//	AA AA AA AA 11 00 00 <mac_hi> <mac_lo> <chan> 00 00 <crc_hi> <crc_lo> 00 08 <uid 0..5> 01
//
// 0x22 ChangePan22, 15 bytes (broadcast; no UID/addr payload):
//
//	AA AA AA AA 22 00 00 <mac_hi> <mac_lo> <chan> 00 00 <crc_hi> <crc_lo> 00
func parseOutboundPair(l2 []byte) (PairFrame, bool) {
	if len(l2) < 5 {
		return PairFrame{}, false
	}
	if l2[0] != 0xAA || l2[1] != 0xAA || l2[2] != 0xAA || l2[3] != 0xAA {
		return PairFrame{}, false
	}
	switch l2[4] {
	case 0x08:
		if len(l2) != 15 || l2[11] != 0xA0 {
			return PairFrame{}, false
		}
		return PairFrame{
			Kind:      PairOffReportBind,
			ShortAddr: uint16(l2[5])<<8 | uint16(l2[6]),
		}, true
	case 0x11:
		if len(l2) != 23 || l2[15] != 0x08 || l2[22] != 0x01 {
			return PairFrame{}, false
		}
		return PairFrame{
			Kind:    PairChangePan11,
			PeerUID: hex.EncodeToString(l2[16:22]),
		}, true
	case 0x22:
		if len(l2) != 15 {
			return PairFrame{}, false
		}
		return PairFrame{Kind: PairChangePan22}, true
	}
	return PairFrame{}, false
}

// parseInboundPair handles A5 A5 bind-ack and short-addr discovery
// reply L2 bodies. Direction is required because outbound 0x11 also
// carries 6 UID bytes near the tail, which would alias against the
// short-addr reply layout if we tried to auto-detect.
//
//	BindAck:        len >= 10, bytes [4..5] = A5 A5, RptOff = (byte 9 != 0)
//	ShortAddrReply: len >= 10, short_addr BE16 at [0..1] (< 0xFFF8),
//	                UID at [4..9]
func parseInboundPair(l2 []byte) (PairFrame, bool) {
	if len(l2) < 10 {
		return PairFrame{}, false
	}
	// Inbound pair replies are flat L0 bodies; they never carry the L2
	// wrapper. A frame that still begins with the L2 SOF (FB FB) is a
	// telemetry / protection / info reply whose first two bytes would be
	// misread by the ShortAddrReply branch below as a short address
	// (0xFBFB). Reject it so such frames fall through to "no decoder
	// matched" instead of corrupting the inverter's stored short_addr.
	if l2[0] == L2SOF1 && l2[1] == L2SOF2 {
		return PairFrame{}, false
	}
	if l2[4] == 0xA5 && l2[5] == 0xA5 {
		return PairFrame{
			Kind:   PairBindAck,
			Bound:  true,
			RptOff: l2[9] != 0,
		}, true
	}
	sa := uint16(l2[0])<<8 | uint16(l2[1])
	if sa < 0xFFF8 {
		return PairFrame{
			Kind:      PairShortAddrReply,
			ShortAddr: sa,
			PeerUID:   hex.EncodeToString(l2[4:10]),
		}, true
	}
	return PairFrame{}, false
}

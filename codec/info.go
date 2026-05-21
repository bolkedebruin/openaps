package codec

import (
	"errors"
	"fmt"
)

// InfoReply is the decoded payload of the 0xDC "Query Inverter Model
// and Version" reply. Three reply shapes exist across inverter
// generations; the decoder dispatches by length.
type InfoReply struct {
	Model           uint8
	SoftwareVersion uint32
}

// ErrUnknownReplyShape is returned when the L2 length does not match
// any of the known 0xDC reply forms.
var ErrUnknownReplyShape = errors.New("info reply: unknown shape (length)")

// outboundInfoQueryL2 is the canonical 0xDC query L2 body, built once
// at init via the shared envelope helper.
var outboundInfoQueryL2 = BuildL2Frame(L2TypeInverterCmd, CmdInfoQuery, []byte{0, 0, 0, 0, 0})

// outboundBBQueryL2 is the canonical 0xBB telemetry query L2 body.
var outboundBBQueryL2 = BuildL2Frame(L2TypeInverterCmd, CmdTelemetryBBQuery, []byte{0, 0, 0, 0, 0})

// OutboundBBQueryL2 returns the canonical 0xBB telemetry query L2 body.
// Callers must not mutate the returned slice; BuildL1Frame copies it on
// the way out so re-entrancy is safe.
func OutboundBBQueryL2() []byte {
	return outboundBBQueryL2
}

// OutboundInfoQueryL2 returns the canonical 0xDC info-query L2 body.
// Callers must not mutate the returned slice.
func OutboundInfoQueryL2() []byte {
	return outboundInfoQueryL2
}

// MatchOutboundInfoQuery reports whether l2 is the canonical 0xDC
// query body.
func MatchOutboundInfoQuery(l2 []byte) bool {
	if len(l2) != len(outboundInfoQueryL2) {
		return false
	}
	for i, b := range outboundInfoQueryL2 {
		if l2[i] != b {
			return false
		}
	}
	return true
}

// DecodeInfoReply parses a 0xDC reply L2 body. Returns
// ErrUnknownReplyShape if the length is not one of the three known
// forms (16, 17, 19). Validates the FB FB SOF, per-form cmd bytes,
// and FE FE EOF.
func DecodeInfoReply(l2 []byte) (InfoReply, error) {
	switch len(l2) {
	case 16:
		return decodeInfoReplyA(l2)
	case 17:
		return decodeInfoReplyB(l2)
	case 19:
		return decodeInfoReplyC(l2)
	default:
		return InfoReply{}, fmt.Errorf("%w: len=%d", ErrUnknownReplyShape, len(l2))
	}
}

// decodeInfoReplyA parses the 16-byte form:
//
//	FB FB 09 DC <model> <hi BE16> ?? <lo BE16> ... FE FE
//
// software_version = hi*1000 + lo.
func decodeInfoReplyA(l2 []byte) (InfoReply, error) {
	if l2[0] != L2SOF1 || l2[1] != L2SOF2 {
		return InfoReply{}, ErrBadL2SOF
	}
	if l2[2] != 0x09 || l2[3] != 0xDC {
		return InfoReply{}, fmt.Errorf("info reply A: bad cmd bytes %02X %02X", l2[2], l2[3])
	}
	if l2[14] != L2EOF1 || l2[15] != L2EOF2 {
		return InfoReply{}, ErrBadL2EOF
	}
	model := l2[4]
	hi := uint32(l2[5])<<8 | uint32(l2[6])
	lo := uint32(l2[8])<<8 | uint32(l2[9])
	return InfoReply{Model: model, SoftwareVersion: hi*1000 + lo}, nil
}

// decodeInfoReplyB parses the 17-byte form:
//
//	FB FB 0A DD DC <model> <hi BE16> ?? <lo BE16> ... FE FE
//
// software_version = hi*1000 + lo.
func decodeInfoReplyB(l2 []byte) (InfoReply, error) {
	if l2[0] != L2SOF1 || l2[1] != L2SOF2 {
		return InfoReply{}, ErrBadL2SOF
	}
	if l2[2] != 0x0A || l2[3] != 0xDD || l2[4] != 0xDC {
		return InfoReply{}, fmt.Errorf("info reply B: bad cmd bytes %02X %02X %02X", l2[2], l2[3], l2[4])
	}
	if l2[15] != L2EOF1 || l2[16] != L2EOF2 {
		return InfoReply{}, ErrBadL2EOF
	}
	model := l2[5]
	hi := uint32(l2[6])<<8 | uint32(l2[7])
	lo := uint32(l2[9])<<8 | uint32(l2[10])
	return InfoReply{Model: model, SoftwareVersion: hi*1000 + lo}, nil
}

// decodeInfoReplyC parses the 19-byte form:
//
//	FB FB 0C DD DC <model> <hi BE16> ?? <mid BE16> ?? <lo BE16> FE FE
//
// software_version = hi*1_000_000 + mid*1_000 + lo.
func decodeInfoReplyC(l2 []byte) (InfoReply, error) {
	if l2[0] != L2SOF1 || l2[1] != L2SOF2 {
		return InfoReply{}, ErrBadL2SOF
	}
	if l2[2] != 0x0C || l2[3] != 0xDD || l2[4] != 0xDC {
		return InfoReply{}, fmt.Errorf("info reply C: bad cmd bytes %02X %02X %02X", l2[2], l2[3], l2[4])
	}
	if l2[17] != L2EOF1 || l2[18] != L2EOF2 {
		return InfoReply{}, ErrBadL2EOF
	}
	model := l2[5]
	hi := uint32(l2[6])<<8 | uint32(l2[7])
	mid := uint32(l2[9])<<8 | uint32(l2[10])
	lo := uint32(l2[13])<<8 | uint32(l2[14])
	return InfoReply{Model: model, SoftwareVersion: hi*1_000_000 + mid*1_000 + lo}, nil
}

// Model codes observed in 0xDC replies. The on-wire dispatch byte is a
// single octet; family identification is our own (the firmware doesn't
// carry these names).
const (
	ModelYC600Old uint8 = 0x05
	ModelYC1000   uint8 = 0x06
	ModelYC600    uint8 = 0x07 // single-phase 2-channel
	ModelQS1      uint8 = 0x08 // single-phase 4-channel (older reporting code)
	ModelYC600B   uint8 = 0x17
	ModelQS1A     uint8 = 0x18 // single-phase 4-channel (newer reporting code)
	ModelDS3      uint8 = 0x20 // single-phase 2-channel
	ModelDS3H     uint8 = 0x21 // single-phase 2-channel (DS3-H variant)
	ModelDS3L     uint8 = 0x22 // single-phase 2-channel (DS3-L / DS3D-L variant)
	ModelExt29    uint8 = 0x29 // three-phase, family name not pinned
	ModelExt30    uint8 = 0x30 // three-phase, family name not pinned
	ModelExt31    uint8 = 0x31 // three-phase, family name not pinned
	ModelQT2      uint8 = 0x32 // three-phase 4-channel (presumed QT2)
	ModelExt36    uint8 = 0x36 // single-phase, firmware lists as such; family name not pinned
)

// PhaseFromModel returns the inverter family classifier: 1 for
// single-phase, 3 for three-phase, 0 for unknown.
func PhaseFromModel(model uint8) uint32 {
	switch model {
	case ModelYC600, ModelQS1, ModelQS1A, ModelDS3, ModelDS3H, ModelDS3L, ModelExt36:
		return 1
	case ModelExt29, ModelExt30, ModelExt31, ModelQT2:
		return 3
	default:
		return 0
	}
}

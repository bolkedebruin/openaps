package codec

import (
	"errors"
	"fmt"
)

// InfoReply is the decoded payload of the 0xDC "Query Inverter Model
// and Version" reply. Three reply shapes exist across inverter
// generations; the decoder dispatches by length.
type InfoReply struct {
	Model           uint32
	SoftwareVersion uint32
}

// ErrUnknownReplyShape is returned when the L2 length does not match
// any of the known 0xDC reply forms.
var ErrUnknownReplyShape = errors.New("info reply: unknown shape (length)")

// outboundInfoQueryL2 is the canonical 0xDC query L2 body. 13 bytes,
// byte-for-byte fixed.
var outboundInfoQueryL2 = [...]byte{
	0xFB, 0xFB, 0x06, 0xDC,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0xE2, 0xFE, 0xFE,
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
// software_version = hi*1000 + lo. Validation matches main.exe's
// assertions on the SOF, cmd bytes, and byte 14 == 0xFE.
func decodeInfoReplyA(l2 []byte) (InfoReply, error) {
	if l2[0] != 0xFB || l2[1] != 0xFB {
		return InfoReply{}, ErrBadL2SOF
	}
	if l2[2] != 0x09 || l2[3] != 0xDC {
		return InfoReply{}, fmt.Errorf("info reply A: bad cmd bytes %02X %02X", l2[2], l2[3])
	}
	if l2[14] != 0xFE || l2[15] != 0xFE {
		return InfoReply{}, ErrBadL2EOF
	}
	model := uint32(l2[4])
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
	if l2[0] != 0xFB || l2[1] != 0xFB {
		return InfoReply{}, ErrBadL2SOF
	}
	if l2[2] != 0x0A || l2[3] != 0xDD || l2[4] != 0xDC {
		return InfoReply{}, fmt.Errorf("info reply B: bad cmd bytes %02X %02X %02X", l2[2], l2[3], l2[4])
	}
	if l2[15] != 0xFE || l2[16] != 0xFE {
		return InfoReply{}, ErrBadL2EOF
	}
	model := uint32(l2[5])
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
	if l2[0] != 0xFB || l2[1] != 0xFB {
		return InfoReply{}, ErrBadL2SOF
	}
	if l2[2] != 0x0C || l2[3] != 0xDD || l2[4] != 0xDC {
		return InfoReply{}, fmt.Errorf("info reply C: bad cmd bytes %02X %02X %02X", l2[2], l2[3], l2[4])
	}
	if l2[17] != 0xFE || l2[18] != 0xFE {
		return InfoReply{}, ErrBadL2EOF
	}
	model := uint32(l2[5])
	hi := uint32(l2[6])<<8 | uint32(l2[7])
	mid := uint32(l2[9])<<8 | uint32(l2[10])
	lo := uint32(l2[13])<<8 | uint32(l2[14])
	return InfoReply{Model: model, SoftwareVersion: hi*1_000_000 + mid*1_000 + lo}, nil
}

// Model codes observed in 0xDC replies and in the firmware's
// set_paraName_paraValue_inverter dispatcher. Family identification is
// our own; the firmware doesn't carry these names.
const (
	ModelYC600 uint32 = 0x07 // single-phase 2-channel
	ModelQS1   uint32 = 0x08 // single-phase 4-channel (older reporting code)
	ModelQS1A  uint32 = 0x18 // single-phase 4-channel (newer reporting code)
	ModelDS3   uint32 = 0x20 // single-phase 2-channel
	ModelDS3H  uint32 = 0x21 // single-phase 2-channel (DS3-H variant)
	ModelDS3L  uint32 = 0x22 // single-phase 2-channel (DS3-L / DS3D-L variant)
	ModelExt36 uint32 = 0x36 // single-phase, firmware lists as such; family name not pinned

	ModelExt29 uint32 = 0x29 // three-phase, family name not pinned
	ModelExt30 uint32 = 0x30 // three-phase, family name not pinned
	ModelExt31 uint32 = 0x31 // three-phase, family name not pinned
	ModelQT2   uint32 = 0x32 // three-phase 4-channel (presumed QT2)
)

// PhaseFromModel returns the inverter family classifier: 1 for
// single-phase, 3 for three-phase, 0 for unknown. Mirrors the
// firmware's compare_model_phase @ 0xb4314 plus ModelQS1A (0x18),
// which the firmware's table omits.
func PhaseFromModel(model uint32) uint32 {
	switch model {
	case ModelYC600, ModelQS1, ModelQS1A, ModelDS3, ModelDS3H, ModelDS3L, ModelExt36:
		return 1
	case ModelExt29, ModelExt30, ModelExt31, ModelQT2:
		return 3
	default:
		return 0
	}
}

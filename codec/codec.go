// Package codec parses APsystems ZigBee L1+L2 reply frames and
// decodes the QueryData (cmd 0xBB → reply cmd 0xB1 for QS1A, 0xBB
// for DS3) telemetry payload into a typed Reply.
//
// The decoders mirror main.exe's resolvedata_60_1200 (QS1A,
// addr 0x1dbec) and resolvedata_DS3 (addr 0x1f45c). Scale constants
// come from the .rodata pools at 0x1dda8+ (QS1A) and 0x1f770+ (DS3).
//
// AES-128-ECB L1 decryption is out of scope here; ECUs running
// AES_flag_ALL=0 emit plaintext frames. The Encrypted bit on
// L1Envelope is set when the gate byte signals ciphertext.
package codec

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
)

// L1Envelope is the parsed L1 reply frame from the modem.
//
//	FC FC <SA hi> <SA lo> <nonce 2> <peer UID 6> <gate=L2[0]> ...
type L1Envelope struct {
	ShortAddr uint16
	Nonce     [2]byte
	PeerUID   [6]byte // 6-byte BCD inverter ID, e.g. 80 60 00 04 25 82
	Encrypted bool    // gate byte < 0xF0
	L2Frame   []byte  // FB FB ... FE FE (plaintext or ciphertext block)
}

// PeerUIDString returns the inverter UID as a 12-char ASCII string,
// matching the form stored in /home/database.db's `id` column.
func (e L1Envelope) PeerUIDString() string {
	return hex.EncodeToString(e.PeerUID[:])
}

// L2Frame is the inner application frame.
//
//	FB FB <type> <cmd> <body...> <chk hi> <chk lo> FE FE
type L2Frame struct {
	Type byte
	Cmd  byte
	Body []byte // bytes after Cmd up to (but excluding) checksum
	ChkHi, ChkLo byte
}

var (
	ErrShortFrame  = errors.New("frame too short")
	ErrBadL1SOF    = errors.New("L1: missing FC FC SOF")
	ErrBadL2SOF    = errors.New("L2: missing FB FB SOF")
	ErrBadL2EOF    = errors.New("L2: missing FE FE EOF")
	ErrEncrypted   = errors.New("L1 ciphertext (AES decryption not implemented in v1)")
	ErrUnknownCmd  = errors.New("unknown reply cmd")
)

// ParseL1 extracts the L1 envelope. raw is the reassembled inbound
// frame (concatenation of all chunks the splice received between
// FC FC and the terminating FE FE).
func ParseL1(raw []byte) (L1Envelope, error) {
	if len(raw) < 14 {
		return L1Envelope{}, fmt.Errorf("%w: L1 needs ≥14 bytes, got %d", ErrShortFrame, len(raw))
	}
	if raw[0] != 0xFC || raw[1] != 0xFC {
		return L1Envelope{}, ErrBadL1SOF
	}
	env := L1Envelope{
		ShortAddr: uint16(raw[2])<<8 | uint16(raw[3]),
		Nonce:     [2]byte{raw[4], raw[5]},
	}
	copy(env.PeerUID[:], raw[6:12])
	gate := raw[12]
	env.Encrypted = gate < 0xF0
	env.L2Frame = raw[12:] // gate byte coincides with L2 SOF FB FB on plaintext frames
	return env, nil
}

// ParseL2 frames an L2 buffer of the form
//
//	FB FB <type> <cmd> <body...> <chk hi> <chk lo> FE FE
//
// On success, returned L2Frame.Body excludes the SOF/header bytes
// AND the trailing checksum + FE FE.
func ParseL2(raw []byte) (L2Frame, error) {
	if len(raw) < 8 {
		return L2Frame{}, fmt.Errorf("%w: L2 needs ≥8 bytes, got %d", ErrShortFrame, len(raw))
	}
	if raw[0] != 0xFB || raw[1] != 0xFB {
		return L2Frame{}, ErrBadL2SOF
	}
	if raw[len(raw)-2] != 0xFE || raw[len(raw)-1] != 0xFE {
		return L2Frame{}, ErrBadL2EOF
	}
	// layout: [FB FB][type][cmd][body...][chk hi][chk lo][FE FE]
	end := len(raw) - 2 // strip FE FE
	if end < 6 {
		return L2Frame{}, fmt.Errorf("%w: L2 too small after EOF strip", ErrShortFrame)
	}
	return L2Frame{
		Type:  raw[2],
		Cmd:   raw[3],
		Body:  raw[4 : end-2],
		ChkHi: raw[end-2],
		ChkLo: raw[end-1],
	}, nil
}

// Reply is the decoded inverter telemetry. Models populate the
// fields they support; absent fields stay at zero.
type Reply struct {
	ShortAddr uint16
	PeerUID   string // 12-char hex, matches `id` column
	Cmd       byte   // L2 cmd byte (0xB1 QS1A, 0xBB DS3, etc.)
	Model     string // "QS1A" / "DS3" / "unknown"

	GridV         float64
	BusV          float64 // QS1A internal DC link
	FreqHz        float64
	ReportSec     uint32  // inverter's internal counter at poll time
	Panels        []Panel
	ActivePowerW  float64
	ReactivePower float64

	// LifetimeRaw is the raw multi-byte big-endian counter per panel.
	// Multiplied by LifetimeScale it gives kWh; raw + scale stay
	// separate so callers can verify or override.
	LifetimeRaw   []uint64
	LifetimeScale float64 // kWh per raw unit
}

// Panel is one MPPT input.
type Panel struct {
	Index int
	DCV   float64
	DCI   float64
	W     float64 // V * I, capped per model
}

// DecodeReply parses + dispatches a complete L1 reply (everything
// from FC FC to FE FE) into a Reply.
func DecodeReply(raw []byte) (Reply, error) {
	env, err := ParseL1(raw)
	if err != nil {
		return Reply{}, err
	}
	return DecodeReplyFromEnvelope(env)
}

// DecodeReplyFromEnvelope decodes telemetry from an already-parsed
// L1 envelope. Callers that have alternative decoders to try (info
// reply, pair frames, ...) parse L1 once and reuse the envelope.
func DecodeReplyFromEnvelope(env L1Envelope) (Reply, error) {
	if env.Encrypted {
		return Reply{}, ErrEncrypted
	}
	l2, err := ParseL2(env.L2Frame)
	if err != nil {
		return Reply{}, fmt.Errorf("L2: %w", err)
	}
	r := Reply{
		ShortAddr: env.ShortAddr,
		PeerUID:   env.PeerUIDString(),
		Cmd:       l2.Cmd,
	}
	switch l2.Cmd {
	case 0xB1:
		r.Model = "QS1A"
		decodeQS1A(l2.Body, &r)
	case 0xBB:
		r.Model = "DS3"
		decodeDS3(l2.Body, &r)
	default:
		r.Model = fmt.Sprintf("unknown(0x%02X)", l2.Cmd)
		return r, fmt.Errorf("%w: 0x%02X", ErrUnknownCmd, l2.Cmd)
	}
	return r, nil
}

// charsToUint reads `n` big-endian bytes from b starting at off and
// returns the value as uint64. main.exe's charsToLongLong does the
// same thing, used for the 4–5 byte panel lifetime accumulators.
func charsToUint(b []byte, off, n int) uint64 {
	var v uint64
	for i := 0; i < n && off+i < len(b); i++ {
		v = (v << 8) | uint64(b[off+i])
	}
	return v
}

// be16 reads a 2-byte big-endian unsigned int.
func be16(b []byte, off int) uint16 {
	if off+1 >= len(b) {
		return 0
	}
	return uint16(b[off])<<8 | uint16(b[off+1])
}

// be24 reads a 3-byte big-endian unsigned int.
func be24(b []byte, off int) uint32 {
	if off+2 >= len(b) {
		return 0
	}
	return uint32(b[off])<<16 | uint32(b[off+1])<<8 | uint32(b[off+2])
}

// fround rounds to 3 decimals so JSON output stays readable.
func fround(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	return math.Round(x*1000) / 1000
}

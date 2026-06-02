// Package codec parses APsystems ZigBee L1+L2 reply frames and
// decodes the QueryData (cmd 0xBB → reply cmd 0xB1 for QS1A, 0xBB
// for DS3) telemetry payload into a typed Reply.
//
// The decoders parse the QS1A and DS3 telemetry payloads using the
// per-family scale constants documented in the codec.
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
//	FC FC <SA hi> <SA lo> <RSSI byte 4> <RSSI byte 5> <peer UID 6> <gate=L2[0]> ...
//
// Bytes 4 and 5 are signal-quality bytes the ZigBee modem stamps on every
// received reply. Byte 4 is the primary RSSI exposed to operators, clamped
// so values < 0x10 are replaced with 0xff (sentinel for "tiny / unknown").
// Byte 5 is a secondary signal metric.
type L1Envelope struct {
	ShortAddr uint16
	RSSI      byte    // byte 4 of L1 — primary RSSI
	LQI       byte    // byte 5 of L1 — secondary signal metric
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
	Type         byte
	Cmd          byte
	Body         []byte // bytes after Cmd up to (but excluding) checksum
	ChkHi, ChkLo byte
}

var (
	ErrShortFrame = errors.New("frame too short")
	ErrBadL1SOF   = errors.New("L1: missing FC FC SOF")
	ErrBadL2SOF   = errors.New("L2: missing FB FB SOF")
	ErrBadL2EOF   = errors.New("L2: missing FE FE EOF")
	ErrEncrypted  = errors.New("L1 ciphertext (AES decryption not implemented in v1)")
	ErrUnknownCmd = errors.New("unknown reply cmd")
)

// ParseL1 extracts the L1 envelope. raw is the reassembled inbound
// frame (concatenation of all chunks the splice received between
// FC FC and the terminating FE FE).
func ParseL1(raw []byte) (L1Envelope, error) {
	if len(raw) < 14 {
		return L1Envelope{}, fmt.Errorf("%w: L1 needs ≥14 bytes, got %d", ErrShortFrame, len(raw))
	}
	if raw[0] != L1ReplySOF || raw[1] != L1ReplySOF {
		return L1Envelope{}, ErrBadL1SOF
	}
	env := L1Envelope{
		ShortAddr: uint16(raw[2])<<8 | uint16(raw[3]),
		RSSI:      raw[4],
		LQI:       raw[5],
	}
	copy(env.PeerUID[:], raw[6:12])
	gate := raw[12]
	env.Encrypted = gate < CleartextGateMin
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
	if raw[0] != L2SOF1 || raw[1] != L2SOF2 {
		return L2Frame{}, ErrBadL2SOF
	}
	if raw[len(raw)-2] != L2EOF1 || raw[len(raw)-1] != L2EOF2 {
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
	Family    Family // dispatching family classification; FamilyUnknown for unrecognised replies

	// Signal-quality bytes from the L1 envelope.
	// RSSI is the primary signal strength value (with clamp byte < 0x10
	// → 0xff applied at consumer level).
	RSSI byte
	LQI  byte

	GridV         float64
	BusV          float64 // QS1A internal DC link
	FreqHz        float64
	ReportSec     uint32 // inverter's internal counter at poll time
	Panels        []Panel
	ActivePowerW  float64
	ReactivePower float64

	// LifetimeRaw is the raw multi-byte big-endian counter per panel.
	// Multiplied by LifetimeScale it gives kWh; raw + scale stay
	// separate so callers can verify or override.
	LifetimeRaw   []uint64
	LifetimeScale float64 // kWh per raw unit

	// Status holds the family-agnostic aggregator booleans (DC bus,
	// fault A/B, warning A/B) derived at the tail of every per-family
	// resolver. Populated for every recognised family.
	Status InverterStatus

	// ExtendedStatus is the family-specific raw-status block (DS3Status
	// for DS3-family replies, QS1AStatus for QS1A-family replies). Nil
	// for unrecognised families. Callers project through the
	// ExtendedStatus interface (ModbusStatus / InverterStatus) and reach
	// the family-specific accessors via a type assertion.
	ExtendedStatus ExtendedStatus
}

// ModelLabel returns the legacy human model string this codec emits on
// the proto wire for Telemetry.Model: "DS3", "QS1A", or
// "unknown(0xNN)". It exists for the proto-boundary callers that
// publish a string-typed model column; in-process consumers should
// branch on r.Family directly.
func (r Reply) ModelLabel() string {
	switch r.Cmd {
	case CmdReplyDS3:
		return "DS3"
	case CmdReplyQS1A:
		return "QS1A"
	}
	return fmt.Sprintf("unknown(0x%02X)", r.Cmd)
}

// InverterStatus collects the five aggregator signals each per-family
// resolver derives. Each codec populates this from its own byte map.
// All bits are fault-active: true means a fault event is present.
type InverterStatus struct {
	DCBusFault         bool
	FaultA, FaultB     bool
	WarningA, WarningB bool
}

// modbusStatusInputs is the union of bits both families project into
// the SunSpec Model 103 St + Evt1 registers via packModbusStatus.
// DS3 leaves CommFault / ZBLink / OverTemp false (no body source);
// QS1A populates all four. Bits not driven by either family stay
// zero in Evt1 — they would otherwise come from other resolvers
// (RCD / DSP-event subsystems) that the codec doesn't see.
type modbusStatusInputs struct {
	GridRelay       bool
	DCContactor     bool
	DCGround        bool
	IsoAny          bool
	CommFault       bool
	ZBLink          bool
	OverTemperature bool
	ACOverVoltAny   bool
	ACUnderVoltAny  bool
	OverFreqAny     bool
	UnderFreqAny    bool
}

// packModbusStatus returns (St, Evt1) as the SunSpec Model 103 16-bit
// big-endian register pair. Per-family ModbusStatus methods build a
// modbusStatusInputs and call here so the Evt1 layout has one source
// of truth.
func packModbusStatus(in modbusStatusInputs) (st, evt1 uint16) {
	if in.GridRelay {
		st = 6
	} else {
		st = 4
	}
	var hi, lo byte
	if in.DCContactor {
		hi |= 0x80
	}
	if in.IsoAny {
		hi |= 0x40
	}
	if in.CommFault {
		hi |= 0x20 | 0x08 // firmware sets bit 5 and bit 3 from one slot
	}
	if in.ZBLink {
		hi |= 0x10
	}
	if in.GridRelay {
		hi |= 0x02
	}
	if in.DCGround {
		hi |= 0x01
	}
	if in.ACOverVoltAny {
		lo |= 0x80
	}
	if in.ACUnderVoltAny {
		lo |= 0x40
	}
	if in.OverFreqAny {
		lo |= 0x20
	}
	if in.UnderFreqAny {
		lo |= 0x10
	}
	if in.OverTemperature {
		lo |= 0x02
	}
	evt1 = uint16(hi)<<8 | uint16(lo)
	return
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
		if !AESEnabled() {
			return Reply{}, ErrEncrypted
		}
		dec, err := decryptEnvelopeInPlace(env)
		if err != nil {
			return Reply{}, fmt.Errorf("L1 AES: %w", err)
		}
		env = dec
	}
	l2, err := ParseL2(env.L2Frame)
	if err != nil {
		return Reply{}, fmt.Errorf("L2: %w", err)
	}
	r := Reply{
		ShortAddr: env.ShortAddr,
		PeerUID:   env.PeerUIDString(),
		Cmd:       l2.Cmd,
		RSSI:      env.RSSI,
		LQI:       env.LQI,
	}
	switch l2.Cmd {
	case CmdReplyQS1A:
		r.Family = FamilyQS1
		decodeQS1A(l2.Body, &r)
	case CmdReplyDS3:
		r.Family = FamilyDS3
		decodeDS3(l2.Body, &r)
	default:
		r.Family = FamilyUnknown
		return r, fmt.Errorf("%w: 0x%02X", ErrUnknownCmd, l2.Cmd)
	}
	return r, nil
}

// charsToUint reads `n` big-endian bytes from b starting at off and
// returns the value as uint64. Used for the 4–5 byte panel lifetime
// accumulators.
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

// LifetimeKWh returns the per-panel lifetime energy in kWh using
// the model's scale. Convenience for callers; equivalent to
// raw * Reply.LifetimeScale, rounded to 3 decimals.
func (r Reply) LifetimeKWh() []float64 {
	if r.LifetimeScale == 0 {
		return nil
	}
	out := make([]float64, len(r.LifetimeRaw))
	for i, raw := range r.LifetimeRaw {
		out[i] = math.Round(float64(raw)*r.LifetimeScale*1000) / 1000
	}
	return out
}

// fround rounds to 3 decimals so JSON output stays readable.
func fround(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	return math.Round(x*1000) / 1000
}

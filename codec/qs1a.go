package codec

import (
	"fmt"
	"math"
)

// QS1YC600ProtCmds is the set of L2 cmd bytes the QS1 yc600 protection
// builder (set_protection_yc600_one @ 0x66728) emits as frame[3] — the sub
// is the cmd, so these frames carry no 0x1C opcode. Includes the slow volt
// trips (AQ/AD/AC/AY), grid freq trips (AJ/AK/AE/AF), and grid_recovery (AG).
//
// These cmds are UNICAST-ONLY: there is no broadcast opcode to swap to (the
// sub IS the cmd), so they must never be emitted on the broadcast path.
// Exported so ecu-zb's bus-mgr can reject them on broadcast and keep its
// allow-list in sync from this single source (no duplicated byte list).
var QS1YC600ProtCmds = map[byte]bool{
	0x11: true, 0x12: true, 0x13: true, 0x14: true, // AQ/AD/AC/AY
	0x57: true, 0x58: true, 0x59: true, 0x5a: true, // AK/AJ/AF/AE
	CmdGridRecoveryQS1: true, // 0x5D — AG
}

// isQS1ProtectionWrite reports whether a QS1 L2 (cmd,sub) is a protection
// write: the 0x1C opcode is shared with set-power (any sub other than the
// set-power sub 0x8C is a protection param), and the yc600-builder frames
// carry the sub directly as the cmd (QS1YC600ProtCmds, incl. 0x5D
// grid_recovery).
func isQS1ProtectionWrite(cmd, sub byte) bool {
	return (cmd == CmdSetPowerQS1Unicast && sub != SubMaxPowerQS1) || QS1YC600ProtCmds[cmd]
}

// isQS1YC600BuilderParam reports whether a QS1 protection param is encoded
// via the yc600 builder (sub-as-cmd at frame[3]) rather than the 0x1C
// builder. These params have no opcode to swap for broadcast.
func isQS1YC600BuilderParam(paramName string) bool {
	if paramName == "grid_recovery_time" {
		return true
	}
	if _, ok := qs1VoltTripSubsYC600[paramName]; ok {
		return true
	}
	_, ok := qs1FreqTripSubsYC600[paramName]
	return ok
}

// qs1VoltTripScale is the QS1A voltage-trip write scale:
// raw = int(V × 1.332 × 4) = int(V × 5.328).
// Source: set_paraName_paraValue_inverter @ 0x69bdc for model 0x18/8:
//
//	iVar1 = int(V × DAT_69ee8 × 4.0)  (DAT_69ee8 = DAT_6a540 = 1.332)
//
// Inverse of qsVolt read: float(int((raw/1.332)/4 + 0.5)).
// Now wire-readable via the 0xDB page-A reply (qs1ReadPageA).
const qs1VoltTripWriteScale = qsProtVoltDenom * 4.0 // 1.332 × 4 = 5.328

// qs1ClearTimeWriteScale is the QS1A clearance-time write scale:
// raw = int(s × DAT_6bc08) = int(s × 100), centiseconds, 16-bit.
// Source: set_paraName_paraValue_inverter @ 0x69bdc for model 0x18/8.
const qs1ClearTimeWriteScale = 100.0

// qs1AvgOvWriteScale is the QS1A 10-min average over-voltage (AB) write
// scale: raw = int(V × DAT_6e920 × DAT_6e928 × 4) = int(V × 1.332 × 600 × 4)
// = int(V × 3196.8), 24-bit, via set_protection_new_one sub 0x7d byte_count 3.
// Source: min10_Over_average_voltage block in set_paraName_paraValue_inverter
// for model 0x18/8. Inverse of qsAvgOv read.
const qs1AvgOvWriteScale = qsProtVoltDenom * 600.0 * 4.0 // 1.332 × 600 × 4 = 3196.8

// qs1ProtFreqSubs maps the long-form param name to its QS1 0x1C sub-byte
// for the single-frame frequency-threshold protection params (freq-watt curve).
var qs1ProtFreqSubs = map[string]byte{
	"Over_frequency_Watt_Start_set": 0x62, // CA
	"Over_frequency_Watt_Low_set":   0x68, // CB
	"Over_frequency_Watt_High_set":  0x65, // CC
	"Under_Frequency_Watt_Low_set":  0xA1, // DH
	"Under_Frequency_Watt_High_set": 0xA4, // DI
	// Reconnect-frequency thresholds (Reconnection_over/under_frequency).
	// QS1A dispatch: sub 0x4e (BQ) / 0x51 (BP), scale int(50e6/Hz),
	// byte_count 3 (same family as CA/CB/CC/DH/DI).
	"Reconnection_over_frequency":  0x4e, // BQ
	"Reconnection_under_frequency": 0x51, // BP
}

// qs1VoltTripSubs maps voltage-trip params that use the QS1 0x1C builder
// (set_protection_new_one — sub at frame[4], byte_count tag at frame[5],
// value left-aligned at frame[6..]). Only AH/AI take this builder for
// model 0x18; the slow volt trips use the yc600 builder (qs1VoltTripSubsYC600).
// Source: set_protection_new_one calls in set_paraName_paraValue_inverter
// @ 0x69bdc for model 8/0x18.
var qs1VoltTripSubs = map[string]byte{
	"over_voltage_fast":  0x90, // AI
	"under_voltage_fast": 0x8e, // AH
}

// qs1VoltTripSubsYC600 maps the slow voltage trips that route through
// set_protection_yc600_one @ 0x66728 for model 8/0x18. That builder places
// the sub directly at frame[3] (the L2 cmd slot) — there is NO 0x1C opcode
// and NO byte_count tag — with the value left-aligned at frame[4..]. Scale
// is int(V × 1.332 × 4), 16-bit. Source: set_paraName_paraValue_inverter
// @ 0x69bdc (over_voltage_fast_90 / under_voltage_fast_90 / *_stage_2 /
// over_voltage_slow_90 blocks).
var qs1VoltTripSubsYC600 = map[string]byte{
	"under_voltage_stage_2":    0x11, // AQ (alias under_voltage_fast_90)
	"over_voltage_slow":        0x12, // AD (alias over_voltage_fast_90)
	"under_voltage_stage_2_90": 0x13, // AC (alias under_voltage_slow)
	"over_voltage_slow_90":     0x14, // AY
}

// qs1FreqTripSubsYC600 maps the four grid frequency trips that route through
// set_protection_yc600_one @ 0x66728 for model 8/0x18 (sub at frame[3],
// value left-aligned at frame[4..6], 24-bit). Scale is int(5e7 / Hz).
// Source: set_paraName_paraValue_inverter @ 0x69bdc (under/over_frequency_
// fast/slow blocks).
var qs1FreqTripSubsYC600 = map[string]byte{
	"under_frequency_fast": 0x58, // AJ
	"over_frequency_fast":  0x57, // AK
	"under_frequency_slow": 0x5a, // AE
	"over_frequency_slow":  0x59, // AF
}

// qs1ClearTimeSubs maps clearance-time params to their QS1 0x1C sub-bytes.
// Source: set_protection_new_one calls in set_paraName_paraValue_inverter
// for model 0x18.
var qs1ClearTimeSubs = map[string]byte{
	"Under_Voltage1_clearance_time":   0x36, // BB
	"Over_Voltage1_clearance_time":    0x38, // BC
	"Under_Voltage2_clearance_time":   0x3A, // BD
	"Over_Voltage2_clearance_time":    0x3C, // BE
	"Under_Frequency1_clearance_time": 0x3E, // BH
	"Over_Frequency1_clearance_time":  0x40, // BI
	"Under_Frequency2_clearance_time": 0x42, // BJ
	"Over_Frequency2_clearance_time":  0x44, // BK
}

// protRecoveryScaleQS1 is the QS1 grid_recovery_time wire scale:
// int(s*100), 16-bit. Uses the YC600 builder (opcode 0x5D), not 0x1C.
const protRecoveryScaleQS1 = 100.0

// QS1 protection-READ scales + page field maps. Offsets are relative to
// the cmd byte (reply[3]); the cmd byte itself is the page (0xDE/0xD9).
// QS1A returns NO 0xDD page-A reply, so the volt/freq trips aren't
// retrievable this way — only reconnect (0xDE) + DH/DI (0xD9), validated
// on live captures vs 60code. Volts = (raw/1.332)/4 rounded; freq =
// 50_000_000/raw (24-bit BE).
// qsProtVoltDenom is the QS1 protection-read voltage divisor (DAT_4fff0):
// volts = (raw/1.332)/4. Distinct from the telemetry grid-V denom (1.32).
const qsProtVoltDenom = 1.332

func qsVolt(raw int) float64 {
	if raw == 0 {
		return 0
	}
	return float64(int((float64(raw)/qsProtVoltDenom)/4 + 0.5))
}

func qsFreq(raw int) float64 {
	if raw == 0 {
		return 0
	}
	return qs1aFreqDivBy / float64(raw)
}

// qsProtAvgOvDenom (DAT_52490) and qsProtDroopScale (DAT_532f0) are the
// QS1 read scales for the 10-min average over-voltage (AB) and the
// over-frequency-watt droop slope (DD).
const (
	qsProtAvgOvDenom = 1.3315
	qsProtDroopScale = 0.169
)

// qsAvgOv is the QS1 10-min average over-voltage (AB), from a 24-bit raw:
// (((raw/600)/1.3315)/4), the firmware integer-dividing by 600 before the
// float convert; the 60code column is the %.0f-rounded result (~253 V).
func qsAvgOv(raw int) float64 {
	if raw == 0 {
		return 0
	}
	return math.Round((float64(raw/600) / qsProtAvgOvDenom) / 4)
}

// qsSec is the QS1 page-A clearance/time read: raw centiseconds → seconds
// (raw / DAT_50838 = raw / 100). Mirrors the page-A SET scale (×100).
func qsSec(raw int) float64 { return float64(raw) / 100.0 }

// qsRecovery is the QS1 page-A grid-recovery / start-ramp time (AG/AS):
// raw / DAT_50c18 = raw / 100, the inverse of the QS1 write scale int(s×100).
func qsRecovery(raw int) float64 { return float64(raw) / 100.0 }

// qsIdent passes a raw 16-bit value through unchanged (page-A AX, which the
// firmware stores without any scale division).
func qsIdent(raw int) float64 { return float64(raw) }

// qsDroop is the QS1 over-frequency-watt droop slope (DD): raw×0.169.
// ~16.7 = EN 50549-1 max.
func qsDroop(raw int) float64 { return float64(raw) * qsProtDroopScale }

// qsMaxPower is the rated/limited output cap (DA), per panel: raw16 × 256/7395
// — the inverse of EncodeSetPowerQS1A's (W × 0x1CE3) >> 8.
func qsMaxPower(raw int) float64 { return float64(raw) * 256.0 / 7395.0 }

// qsModeFromPageB decodes the QS1 over-frequency-watt mode (CV), a nibble
// of base+0x3f selected by the page-variant flag at base+0x4a: flag==0 →
// low nibble; flag=='f' → ((b>>4)&3)+0xc (the page-f packing). Any other
// flag means the firmware leaves the mode unset, so the code isn't reported.
func qsModeFromPageB(f []byte, base int) (float64, bool) {
	nib, flag := base+0x3f, base+0x4a
	if nib >= len(f) || flag >= len(f) {
		return 0, false
	}
	b := f[nib]
	switch f[flag] {
	case 'f':
		return float64(((b >> 4) & 3) + 0xc), true
	case 0:
		return float64(b & 0xf), true
	default:
		return 0, false
	}
}

// qs1PageAEnum builds an fn that extracts an enum field packed into the
// page-A flags byte at base+0x01 (reply byte right after the 0xDB cmd).
// Four enums are packed into this single byte:
// AT=b>>6, AU=(b>>4)&3, AV=(b>>1)&3, AW=b&1.
func qs1PageAEnum(shift, mask byte) func(f []byte, base int) (float64, bool) {
	return func(f []byte, base int) (float64, bool) {
		i := base + 0x01
		if i >= len(f) {
			return 0, false
		}
		return float64((f[i] >> shift) & mask), true
	}
}

var (
	// qs1ReadPageA decodes the QS1/QS1A page-A protection reply (L2 cmd
	// 0xDB). Offsets are relative to base = reply[3] (the 0xDB cmd byte).
	//
	// Field map (base+offset → code → scale):
	//   +0x01 byte → AT/AU/AV/AW enum nibbles
	//   +0x03 → AX (raw 16-bit, no scale)
	//   +0x05/07/09/0b/0d/0f → AQ/AD/AC/AY/AZ/BA volts (raw/1.332/4, %.0f)
	//   +0x11/14/17/1a → AJ/AK/AE/AF freq (24-bit, 5e7/raw)
	//   +0x1d..0x30 → BB..BK clear times (raw/100)
	//   +0x31/33 → AG/AS recovery / start-ramp (raw/100)
	//   +0x35/37 → AH/AI volts (raw/1.332/4)
	//   +0x39/3b → BL/BM volts (raw/100; transtemp-window times)
	// The DW..EE columns the firmware also writes are hardcoded float
	// constants (not reply bytes) — used only as defaults, so they are
	// intentionally omitted here.
	qs1ReadPageA = []protReadField{
		{code: "AT", fn: qs1PageAEnum(6, 3)}, {code: "AU", fn: qs1PageAEnum(4, 3)},
		{code: "AV", fn: qs1PageAEnum(1, 3)}, {code: "AW", fn: qs1PageAEnum(0, 1)},
		{code: "AX", off: 0x03, width: 2, scale: qsIdent},
		{code: "AQ", off: 0x05, width: 2, scale: qsVolt}, {code: "AD", off: 0x07, width: 2, scale: qsVolt},
		{code: "AC", off: 0x09, width: 2, scale: qsVolt}, {code: "AY", off: 0x0b, width: 2, scale: qsVolt},
		{code: "AZ", off: 0x0d, width: 2, scale: qsVolt}, {code: "BA", off: 0x0f, width: 2, scale: qsVolt},
		{code: "AJ", off: 0x11, width: 3, scale: qsFreq}, {code: "AK", off: 0x14, width: 3, scale: qsFreq},
		{code: "AE", off: 0x17, width: 3, scale: qsFreq}, {code: "AF", off: 0x1a, width: 3, scale: qsFreq},
		{code: "BB", off: 0x1d, width: 2, scale: qsSec}, {code: "BC", off: 0x1f, width: 2, scale: qsSec},
		{code: "BD", off: 0x21, width: 2, scale: qsSec}, {code: "BE", off: 0x23, width: 2, scale: qsSec},
		{code: "BF", off: 0x25, width: 2, scale: qsSec}, {code: "BG", off: 0x27, width: 2, scale: qsSec},
		{code: "BH", off: 0x29, width: 2, scale: qsSec}, {code: "BI", off: 0x2b, width: 2, scale: qsSec},
		{code: "BJ", off: 0x2d, width: 2, scale: qsSec}, {code: "BK", off: 0x2f, width: 2, scale: qsSec},
		{code: "AG", off: 0x31, width: 2, scale: qsRecovery}, {code: "AS", off: 0x33, width: 2, scale: qsRecovery},
		{code: "AH", off: 0x35, width: 2, scale: qsVolt}, {code: "AI", off: 0x37, width: 2, scale: qsVolt},
		{code: "BL", off: 0x39, width: 2, scale: qsSec}, {code: "BM", off: 0x3b, width: 2, scale: qsSec},
	}
	qs1ReadPageB = []protReadField{
		{code: "BN", off: 0x01, width: 2, scale: qsVolt}, {code: "BO", off: 0x03, width: 2, scale: qsVolt},
		{code: "BP", off: 0x05, width: 3, scale: qsFreq}, {code: "BQ", off: 0x08, width: 3, scale: qsFreq},
		// 10-min average over-voltage (24-bit) + freq-watt droop slope:
		{code: "AB", off: 0x19, width: 3, scale: qsAvgOv}, {code: "DD", off: 0x45, width: 2, scale: qsDroop},
		// output cap (W/panel), the 2 bytes right after DD:
		{code: "DA", off: 0x47, width: 2, scale: qsMaxPower},
		// over-freq-watt mode nibble (page-variant gated):
		{code: "CV", fn: qsModeFromPageB},
		// freq-watt curve (over-freq), offsets pinned by differential write:
		{code: "CC", off: 0x25, width: 3, scale: qsFreq}, {code: "CB", off: 0x28, width: 3, scale: qsFreq},
	}
	qs1ReadPageC = []protReadField{
		// over-freq-watt recover-high (DC, 24-bit) + under-freq-watt curve:
		{code: "DC", off: 0x01, width: 3, scale: qsFreq},
		{code: "DH", off: 0x0c, width: 3, scale: qsFreq}, {code: "DI", off: 0x0f, width: 3, scale: qsFreq},
	}
)

// qs1ProtectionPage resolves a QS1 protection reply to its field table;
// the cmd byte is the page and data base = byte[3].
func qs1ProtectionPage(_ []byte, cmd byte) ([]protReadField, int, bool) {
	switch cmd {
	case CmdReplyQS1PageA:
		return qs1ReadPageA, 3, true
	case CmdProtReadPageB:
		return qs1ReadPageB, 3, true
	case CmdProtReadPageC:
		return qs1ReadPageC, 3, true
	}
	return nil, 0, false
}

// qs1NewOneBody builds the body for set_protection_new_one @ 0x66f2c:
// frame is FB FB 06 0x1C [sub] [byte_count] [value left-aligned at 6..8].
// width 2 → value big-endian at [6..7], [8]=0; width 3 → at [6..8].
// (BuildL2Frame writes the body starting at frame[4], so the sub lands at
// [4], byte_count at [5], value at [6..].)
func qs1NewOneBody(sub byte, wire int64, width byte) []byte {
	switch width {
	case 3:
		return []byte{sub, 3, byte(wire >> 16), byte(wire >> 8), byte(wire)}
	default:
		return []byte{sub, 2, byte(wire >> 8), byte(wire), 0}
	}
}

// qs1YC600Frame builds a set_protection_yc600_one @ 0x66728 frame: the sub
// is the L2 cmd at frame[3] (no 0x1C opcode, no byte_count tag), and the
// value is left-aligned big-endian at frame[4..]. width 2 → [4..5]; width
// 3 → [4..6]. BuildL2Frame's cmd argument is the sub.
func qs1YC600Frame(sub byte, wire int64, width byte) []byte {
	switch width {
	case 3:
		return BuildL2Frame(sub, []byte{byte(wire >> 16), byte(wire >> 8), byte(wire), 0, 0})
	default:
		return BuildL2Frame(sub, []byte{byte(wire >> 8), byte(wire), 0, 0, 0})
	}
}

// encodeProtectionQS1A builds a QS1 protection frame for one param. The
// opcode argument (CmdSetPowerQS1Unicast 0x1C / CmdSetPowerQS1Broadcast
// 0x2C) is used only for the params that route through the 0x1C builder
// (set_protection_new_one): the freq-watt curve, AH/AI/AB volt trips, and
// the clearance times — there the sub-byte at [4] distinguishes the param
// from set-power on the shared opcode.
//
// The slow voltage trips (AQ/AD/AC/AY) and grid frequency trips
// (AJ/AK/AE/AF) route through set_protection_yc600_one, which puts the sub
// directly at frame[3] (the L2 cmd slot) — those frames ignore the opcode
// argument. grid_recovery_time (AG) uses the 0x5D YC600 builder; CV is a
// multi-frame mode enum. Source: set_paraName_paraValue_inverter @ 0x69bdc
// for model 8/0x18 + the two builders @ 0x66f2c / 0x66728.
func encodeProtectionQS1A(paramName string, value float64, opcode byte) ([][]byte, error) {
	// Freq-watt curve (CA/CB/CC/DH/DI): 0x1C builder, 24-bit, int(50e6/Hz).
	if sub, ok := qs1ProtFreqSubs[paramName]; ok {
		wire := int64(qs1aFreqDivBy / value) // truncate toward zero
		if wire < 0 || wire > 0xFFFFFF {
			return nil, fmt.Errorf("set-protection: %q=%g Hz → wire %d out of 24-bit range", paramName, value, wire)
		}
		return [][]byte{BuildL2Frame(opcode, qs1NewOneBody(sub, wire, 3))}, nil
	}
	// Voltage trips AH/AI: 0x1C builder, 16-bit, int(V × 1.332 × 4).
	if sub, ok := qs1VoltTripSubs[paramName]; ok {
		wire := int64(value * qs1VoltTripWriteScale)
		if wire < 0 || wire > 0xFFFF {
			return nil, fmt.Errorf("set-protection: %q=%g V → wire %d out of 16-bit range", paramName, value, wire)
		}
		return [][]byte{BuildL2Frame(opcode, qs1NewOneBody(sub, wire, 2))}, nil
	}
	// Slow voltage trips AQ/AD/AC/AY: yc600 builder, 16-bit, int(V × 1.332 × 4).
	if sub, ok := qs1VoltTripSubsYC600[paramName]; ok {
		wire := int64(value * qs1VoltTripWriteScale)
		if wire < 0 || wire > 0xFFFF {
			return nil, fmt.Errorf("set-protection: %q=%g V → wire %d out of 16-bit range", paramName, value, wire)
		}
		return [][]byte{qs1YC600Frame(sub, wire, 2)}, nil
	}
	// Grid frequency trips AJ/AK/AE/AF: yc600 builder, 24-bit, int(50e6/Hz).
	if sub, ok := qs1FreqTripSubsYC600[paramName]; ok {
		wire := int64(qs1aFreqDivBy / value) // truncate toward zero
		if wire < 0 || wire > 0xFFFFFF {
			return nil, fmt.Errorf("set-protection: %q=%g Hz → wire %d out of 24-bit range", paramName, value, wire)
		}
		return [][]byte{qs1YC600Frame(sub, wire, 3)}, nil
	}
	// Clearance times BB..BK: 0x1C builder, 16-bit, int(s × 100).
	if sub, ok := qs1ClearTimeSubs[paramName]; ok {
		wire := int64(value * qs1ClearTimeWriteScale)
		if wire < 0 || wire > 0xFFFF {
			return nil, fmt.Errorf("set-protection: %q=%g s → wire %d out of 16-bit range", paramName, value, wire)
		}
		return [][]byte{BuildL2Frame(opcode, qs1NewOneBody(sub, wire, 2))}, nil
	}
	switch paramName {
	case "min10_Over_average_voltage": // AB — 0x1C builder sub 0x7d, 24-bit, int(V × 1.332 × 600 × 4)
		wire := int64(value * qs1AvgOvWriteScale)
		if wire < 0 || wire > 0xFFFFFF {
			return nil, fmt.Errorf("set-protection: min10_Over_average_voltage=%g V → wire %d out of 24-bit range", value, wire)
		}
		return [][]byte{BuildL2Frame(opcode, qs1NewOneBody(0x7d, wire, 3))}, nil
	case "Over_frequency_Watt_set": // CV — multi-frame mode enum
		// The firmware's default branch packs the mode as a single byte, so
		// guard against the int→byte wrap (e.g. 300 → 44) silently selecting
		// a different mode; in-range modes pass through to the builder.
		if mode := int(value); mode < 0 || mode > 0xFF {
			return nil, fmt.Errorf("set-protection: Over_frequency_Watt_set mode %d out of byte range [0,255]", mode)
		}
		return qs1ModeFrames(value, opcode), nil
	case "grid_recovery_time": // AG — YC600 0x5D builder, int(s*100), 16-bit
		wire := int64(value * protRecoveryScaleQS1)
		if wire < 0 || wire > 0xFFFF {
			return nil, fmt.Errorf("set-protection: grid_recovery_time=%g → wire %d out of 16-bit range", value, wire)
		}
		// The opcode argument is intentionally ignored: this param uses the
		// 0x5D YC600 builder on both unicast and broadcast paths.
		return [][]byte{qs1YC600Frame(CmdGridRecoveryQS1, wire, 2)}, nil
	}
	return nil, fmt.Errorf("%w: %q on QS1A", ErrUnsupportedProtectionParam, paramName)
}

// qs1ModeFrames builds the QS1 over-frequency-watt mode enum sequence
// (Over_frequency_Watt_set, sub 0x5F + 0x99), replicating the firmware
// branch+remap: 0xF (disabled) → 5F=0x30, 5F=0x0F, 99=0; {3,4,0xD}
// (AS/NZS, 0xD→4) → 5F=0x10, 5F=v, 99=1; else (e.g. 0xE→1) → 5F=0x20,
// 5F=v. Each frame uses opcode (0x1C unicast or 0x2C broadcast) with
// byte_count 1 (value at [6]).
func qs1ModeFrames(value float64, opcode byte) [][]byte {
	mode := int(value)
	frame := func(sub, v byte) []byte {
		return BuildL2Frame(opcode, []byte{sub, 1, v, 0, 0})
	}
	switch mode {
	case 0xF:
		return [][]byte{frame(0x5F, 0x30), frame(0x5F, 0x0F), frame(0x99, 0x00)}
	case 3, 4, 0xD:
		v := byte(mode)
		if mode == 0xD {
			v = 4
		}
		return [][]byte{frame(0x5F, 0x10), frame(0x5F, v), frame(0x99, 0x01)}
	default:
		v := byte(mode)
		if mode == 0xE {
			v = 1
		}
		return [][]byte{frame(0x5F, 0x20), frame(0x5F, v)}
	}
}

// setPowerScaleDSP is the QS1 / QS1A / YC600-family watts-to-register
// multiplier used by EncodeSetPowerQS1A and EncodeSetPowerC3.
//
//	reg = (panelWatts * setPowerScaleDSP) >> setPowerScaleDSPShift
const (
	setPowerScaleDSP      uint32 = 0x1CE3
	setPowerScaleDSPShift        = 8
	// setPowerQS1ValueLen is the body byte that announces a 2-byte value
	// follows under sub-code SubMaxPowerQS1.
	setPowerQS1ValueLen byte = 0x02
)

// EncodeSetPowerQS1A builds the L2 frame to set max-power on the
// QS1 / QS1A family (model codes 0x08 / 0x18). broadcast=true emits
// CmdSetPowerQS1Broadcast; broadcast=false emits CmdSetPowerQS1Unicast.
// SubMaxPowerQS1 selects the parameter; value length is 2.
func EncodeSetPowerQS1A(panelWatts uint16, broadcast bool) ([]byte, error) {
	if err := checkPanelWatts(panelWatts); err != nil {
		return nil, err
	}
	val := (uint32(panelWatts) * setPowerScaleDSP) >> setPowerScaleDSPShift
	body := []byte{
		SubMaxPowerQS1,
		setPowerQS1ValueLen,
		byte(val >> 8),
		byte(val & 0xFF),
		0x00,
	}
	cmd := CmdSetPowerQS1Unicast
	if broadcast {
		cmd = CmdSetPowerQS1Broadcast
	}
	return BuildL2Frame(cmd, body), nil
}

// QS1A reply payload layout, body offset 0 = byte right after the
// L2 cmd 0xB1.
//
// Wire scale constants:
//
//	busPFBase    = 2.85   (bus V denom)
//	busScale     = 3300.0 (bus V numer)
//	busDenom     = 4092.0 (bus V denom inner)
//	busOffset    = 757.0  (bus V offset)
//	freqDivBy    = 5e7    (Hz = 5e7 / raw_3byte)
//	panelVMax    = 82.5   (panel V scale, 12-bit ADC)
//	panelIMax    = 27.5   (panel I scale, 12-bit ADC)
//	adc12bit     = 4096.0
//	gridVDenom   = 1.32   (V_grid = raw16 / 1.32 / 4)
//	efficiency   = 0.95   (active power scale)
//	lifetime     = 3.756e-11 (per-byte → kWh)
const (
	qs1aBusScale  = 3300.0
	qs1aBusDenom  = 4092.0
	qs1aBusOffset = 757.0
	qs1aBusPFBase = 2.85
	qs1aFreqDivBy = 50_000_000.0 // QS1 freq raw↔Hz dividend; also the protection-encode dividend
	qs1aPanelVMax = 82.5
	qs1aPanelIMax = 27.5
	qs1aADC12bit  = 4096.0
	qs1aGridDenom = 1.32
	qs1aEfficien  = 0.95
	qs1aLifetime  = 3.756e-11
)

// decodeQS1A populates r with QS1A telemetry from body. Body length
// is typically 0x52 = 82 bytes for QS1A (88-byte total reply minus
// 4 L2 SOF/type/cmd minus 2 chk minus 2 FE FE).
//
// Field map (offsets into body):
//
//	[0..1]   internal DC bus voltage (16-bit ADC) → V via the busPF formula
//	         (printed as "bus_vol").
//	[2..4]   grid frequency (24-bit) → Hz = 5e7 / raw
//	[6..8]   panel D 3-byte packed (12-bit V + 12-bit I)
//	[9..11]  panel C 3-byte packed
//	[12..14] panel B 3-byte packed
//	[15..17] panel A 3-byte packed
//	[0x12..0x13]  AC grid voltage (16-bit) → V = raw / 1.32 / 4
//	[0x14..0x15]  inverter report counter (16-bit; cumulative — diff
//	              between polls gives sample window seconds)
//	[0x17..0x1a]  status bit flags
//	[0x1b..0x1f]  panel A lifetime accumulator (5-byte BE)
//	[0x20..0x24]  panel B lifetime
//	[0x25..0x29]  panel C lifetime
//	[0x2a..0x2e]  panel D lifetime
//	[0x2f]   power-factor encoding (resolvedata sets cos/sin from this)
//
// The 3-byte panel packing:
//
//	byte[off+0]    = current[7:0]
//	byte[off+1]    = (voltage[3:0]<<4) | current[11:8]
//	byte[off+2]    = voltage[11:4]
//	V = packed × 82.5 / 4096,  I = packed × 27.5 / 4096
func decodeQS1A(body []byte, r *Reply) {
	r.BusV = fround(qs1aBusV(be16(body, 0)))
	r.GridV = fround(qs1aGridV(be16(body, 0x12)))
	r.FreqHz = fround(qs1aFreqHz(be24(body, 2)))
	r.ReportSec = uint32(be16(body, 0x14))

	// Panels: payload byte order is D, C, B, A — invert so r.Panels
	// is indexed A=0, B=1, C=2, D=3 to match the 12-character UID
	// suffix labels users see (704000006835A/B/C/D).
	for _, pp := range []struct {
		idx, off int
	}{
		{0, 15}, // A — bytes 15..17
		{1, 12}, // B — bytes 12..14
		{2, 9},  // C — bytes 9..11
		{3, 6},  // D — bytes 6..8
	} {
		v := qs1aPanelV(body, pp.off)
		i := qs1aPanelI(body, pp.off)
		r.Panels = append(r.Panels, Panel{
			Index: pp.idx,
			DCV:   fround(v),
			DCI:   fround(i),
			W:     fround(v * i),
		})
	}

	// Active power total = sum of panel watts × 0.95 efficiency.
	// MAXOP and DC-vs-energy tie-break logic are intentionally
	// bypassed (those are for ECU power-limiting, not raw telemetry).
	var w float64
	for _, p := range r.Panels {
		w += p.W
	}
	r.ActivePowerW = fround(w * qs1aEfficien)

	// Lifetime accumulators per panel (5 bytes BE each), payload
	// offset order is A, B, C, D — matches r.Panels order above.
	r.LifetimeRaw = []uint64{
		charsToUint(body, 0x1b, 5),
		charsToUint(body, 0x20, 5),
		charsToUint(body, 0x25, 5),
		charsToUint(body, 0x2a, 5),
	}
	r.LifetimeScale = qs1aLifetime

	r.QS1AStatus = decodeQS1AStatus(body)
	r.Status = r.QS1AStatus.Faults().InverterStatus()
}

// QS1AStatus holds the raw status bytes a QS1A reply spreads across
// the payload. They are unpacked into the same logical slots DS3 uses
// plus an additional grid-side block driven by Grid.
//
// Source layout (in ascending body offset order):
//
//	Main  = body[0x17..0x1a]  — primary fault/warning bits
//	Extra = body[0x34]        — single byte; semantics undocumented.
//	Grid  = body[0x38..0x39]  — grid-side flag table; per-bit
//	                            "absence" counters track loss.
type QS1AStatus struct {
	Main  [4]byte
	Grid  [2]byte
	Extra byte
}

// decodeQS1AStatus returns the raw status bytes plus the
// family-agnostic aggregator booleans. Source-byte mapping (verified
// against resolvedata_60_1200):
//
//	DCBusFault  = body[0x18] bit 4                          (inv+0x29e)
//	WarningA    = body[0x1a] bits 0,2                       (inv+0x2a5,0x2a7)
//	WarningB    = body[0x1a] bits 1,3                       (inv+0x2a6,0x2a8)
//	FaultA      = body[0x1a] bits 4,6 | body[0x19] bit 0    (inv+0x2a9,0x2ab,0x2ad,
//	              | body[0x17] bit 5                              0x2b4)
//	FaultB      = body[0x1a] bits 5,7 | body[0x19] bit 1    (inv+0x2aa,0x2ac,0x2ae,
//	              | body[0x17] bit 6                              0x2b5)
//
// decodeQS1AStatus returns the raw status bytes. The semantic
// aggregator booleans (InverterStatus) are derived from
// QS1AStatus.Faults().InverterStatus() so the bit→meaning mapping has
// one source of truth.
func decodeQS1AStatus(body []byte) QS1AStatus {
	var qs QS1AStatus
	if len(body) < 0x3a {
		return qs
	}
	copy(qs.Main[:], body[0x17:0x1b])
	copy(qs.Grid[:], body[0x38:0x3a])
	qs.Extra = body[0x34]
	return qs
}

// InverterStatus reduces named QS1A fault bits to the firmware's five
// aggregator signals (resolvedata_60_1200's slot-walking aggregator
// at inv+0x3ec / 0x3f0 / 0x3f4 / 0x3f8 / 0x3fc / 0x400).
func (f QS1AFaults) InverterStatus() InverterStatus {
	return InverterStatus{
		DCBusFault: f.DCBusFault,
		FaultA:     f.OverFreqFast || f.OverFreqSlow || f.OverFreqExtra || f.OverFreqRMS,
		FaultB:     f.UnderFreqFast || f.UnderFreqSlow || f.UnderFreqExtra || f.UnderFreqRMS,
		WarningA:   f.ACOverVoltFast || f.ACOverVoltSlow,
		WarningB:   f.ACUnderVoltFast || f.ACUnderVoltSlow,
	}
}

// QS1AFaults names the QS1A status bits whose meaning is pinned by
// the firmware's Modbus packing or by debounce/aggregator placement.
// All bits are fault-active: '1' on the wire means an event/fault is
// present; '0' means healthy. Bits with no named semantic stay
// reachable via QS1AStatus.Main/Grid/Extra.
//
// QS1A reports a strictly richer Evt1 surface than DS3: isolation is
// split per-channel (A/B/C/D), and over-temperature has a body bit
// where DS3 has none. Conversely, QS1A's OF/UF cluster reads fewer
// off-body slots, so Evt1 OF/UF here reflects only what body[0x17,
// 0x19, 0x1a] supplies.
type QS1AFaults struct {
	GridRelayFault   bool // body[0x17] bit 2 → inv+0x29a (1=event; firmware also maps St 4↔6 and Evt1 bit 1)
	DCContactorFault bool // body[0x17] bit 1 → inv+0x299 → Evt1 bit 7
	DCGroundFault    bool // body[0x17] bit 3 → inv+0x298 → Evt1 bit 0
	DCBusFault       bool // body[0x18] bit 4 → inv+0x29e (1=event; counter inv+0x3f0 tracks fault-poll count, inv+0x3ec the healthy streak)
	CommFault        bool // body[0x17] bit 4 → inv+0x29b → Evt1 bits 5 + 3
	OverTemperature  bool // body[0x18] bit 2 → inv+0x2b3 → Evt1 bit 1

	IsoFaultA bool // body[0x19] bit 2 → inv+0x290 → Evt1 bit 6
	IsoFaultB bool // body[0x19] bit 4 → inv+0x292 → Evt1 bit 6
	IsoFaultC bool // body[0x18] bit 0 → inv+0x2b1 → Evt1 bit 6
	IsoFaultD bool // body[0x19] bit 6 → inv+0x2af → Evt1 bit 6

	ACOverVoltFast  bool // body[0x1a] bit 2 → inv+0x2a7 → Evt1 bit 15
	ACOverVoltSlow  bool // body[0x1a] bit 0 → inv+0x2a5 → Evt1 bit 15
	ACUnderVoltFast bool // body[0x1a] bit 3 → inv+0x2a8 → Evt1 bit 14
	ACUnderVoltSlow bool // body[0x1a] bit 1 → inv+0x2a6 → Evt1 bit 14

	OverFreqFast   bool // body[0x1a] bit 6 → inv+0x2ab → Evt1 bit 13
	OverFreqSlow   bool // body[0x1a] bit 4 → inv+0x2a9 → Evt1 bit 13
	OverFreqExtra  bool // body[0x19] bit 0 → inv+0x2ad → Evt1 bit 13
	OverFreqRMS    bool // body[0x17] bit 5 → inv+0x2b4 → Evt1 bit 13 (10-min RMS)
	UnderFreqFast  bool // body[0x1a] bit 7 → inv+0x2ac → Evt1 bit 12
	UnderFreqSlow  bool // body[0x1a] bit 5 → inv+0x2aa → Evt1 bit 12
	UnderFreqExtra bool // body[0x19] bit 1 → inv+0x2ae → Evt1 bit 12
	UnderFreqRMS   bool // body[0x17] bit 6 → inv+0x2b5 → Evt1 bit 12 (10-min RMS)

	// ZBLink{A,B} feed Evt1 bit 4. Polarity reflects the firmware
	// verbatim — the slot value ORs into Evt1 bit 4 regardless of
	// SunSpec's GRID_DISCONNECT convention. Same slots also drive
	// the per-channel absence counters.
	ZBLinkA bool // body[0x39] bit 0 → inv+0x3cc → Evt1 bit 4
	ZBLinkB bool // body[0x39] bit 1 → inv+0x3cd → Evt1 bit 4
}

// Faults decodes the named-meaning bits from the QS1A status bytes.
// Bits without a confirmed semantic (warning bucket, reserved, raw
// "Extra" byte) stay in Main/Grid/Extra.
func (s QS1AStatus) Faults() QS1AFaults {
	b17, b18, b19, b1a := s.Main[0], s.Main[1], s.Main[2], s.Main[3]
	b39 := s.Grid[1]
	return QS1AFaults{
		GridRelayFault:   b17&(1<<2) != 0,
		DCContactorFault: b17&(1<<1) != 0,
		DCGroundFault:    b17&(1<<3) != 0,
		DCBusFault:       b18&(1<<4) != 0,
		CommFault:        b17&(1<<4) != 0,
		OverTemperature:  b18&(1<<2) != 0,
		IsoFaultA:        b19&(1<<2) != 0,
		IsoFaultB:        b19&(1<<4) != 0,
		IsoFaultC:        b18&(1<<0) != 0,
		IsoFaultD:        b19&(1<<6) != 0,
		ACOverVoltFast:   b1a&(1<<2) != 0,
		ACOverVoltSlow:   b1a&(1<<0) != 0,
		ACUnderVoltFast:  b1a&(1<<3) != 0,
		ACUnderVoltSlow:  b1a&(1<<1) != 0,
		OverFreqFast:     b1a&(1<<6) != 0,
		OverFreqSlow:     b1a&(1<<4) != 0,
		OverFreqExtra:    b19&(1<<0) != 0,
		OverFreqRMS:      b17&(1<<5) != 0,
		UnderFreqFast:    b1a&(1<<7) != 0,
		UnderFreqSlow:    b1a&(1<<5) != 0,
		UnderFreqExtra:   b19&(1<<1) != 0,
		UnderFreqRMS:     b17&(1<<6) != 0,
		ZBLinkA:          b39&(1<<0) != 0,
		ZBLinkB:          b39&(1<<1) != 0,
	}
}

// ModbusStatus returns the SunSpec Model 103 (St, Evt1) register pair
// for a QS1A reply via the shared packModbusStatus helper. QS1A
// populates strictly more Evt1 bits than DS3 — isolation (bit 14,
// per-channel A/B/C/D), comm (bits 13 + 11), ZB-link (bit 12), and
// over-temperature (bit 1) all have body sources for QS1A and zero
// body sources for DS3.
func (s QS1AStatus) ModbusStatus() (st, evt1 uint16) {
	f := s.Faults()
	return packModbusStatus(modbusStatusInputs{
		GridRelay:       f.GridRelayFault,
		DCContactor:     f.DCContactorFault,
		DCGround:        f.DCGroundFault,
		IsoAny:          f.IsoFaultA || f.IsoFaultB || f.IsoFaultC || f.IsoFaultD,
		CommFault:       f.CommFault,
		ZBLink:          f.ZBLinkA || f.ZBLinkB,
		OverTemperature: f.OverTemperature,
		ACOverVoltAny:   f.ACOverVoltFast || f.ACOverVoltSlow,
		ACUnderVoltAny:  f.ACUnderVoltFast || f.ACUnderVoltSlow,
		OverFreqAny:     f.OverFreqFast || f.OverFreqSlow || f.OverFreqExtra || f.OverFreqRMS,
		UnderFreqAny:    f.UnderFreqFast || f.UnderFreqSlow || f.UnderFreqExtra || f.UnderFreqRMS,
	})
}

// qs1aBusV applies the internal-DC-link formula:
//
//	V = (raw * 3300/4092 - 757) / 2.85
//
// This is the value printed as "bus_vol".
func qs1aBusV(raw uint16) float64 {
	return (float64(raw)*qs1aBusScale/qs1aBusDenom - qs1aBusOffset) / qs1aBusPFBase
}

// qs1aGridV applies the grid voltage formula:
//
//	V = (raw_16bit / 1.32) / 4
func qs1aGridV(raw uint16) float64 {
	return (float64(raw) / qs1aGridDenom) / 4.0
}

// qs1aFreqHz applies the formula:
//
//	Hz = 5e7 / raw3byte
func qs1aFreqHz(raw uint32) float64 {
	if raw == 0 {
		return 0
	}
	return qs1aFreqDivBy / float64(raw)
}

// qs1aPanelV / qs1aPanelI unpack a 12-bit ADC value out of the
// 3-byte packed format used per panel:
//
//	byte[off+0]    = current[7:0]
//	byte[off+1]    = (voltage[3:0]<<4) | current[11:8]
//	byte[off+2]    = voltage[11:4]
//
// Then voltage = packed * 82.5 / 4096, current = packed * 27.5 / 4096.
func qs1aPanelV(body []byte, off int) float64 {
	if off+2 >= len(body) {
		return 0
	}
	raw := uint16(body[off+2])<<4 | uint16(body[off+1]>>4)
	return float64(raw) * qs1aPanelVMax / qs1aADC12bit
}

func qs1aPanelI(body []byte, off int) float64 {
	if off+1 >= len(body) {
		return 0
	}
	raw := (uint16(body[off+1]&0x0F) << 8) | uint16(body[off])
	return float64(raw) * qs1aPanelIMax / qs1aADC12bit
}

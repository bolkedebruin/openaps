package codec

import (
	"errors"
	"fmt"
	"strings"
)

// Physical safety envelope for protection-write values, in native units.
// The per-family encoders bound a value only to its wire bit-width, so
// without this backstop a value such as a 1 V under-voltage trip, a 12 kV
// over-voltage trip, or a 200 Hz over-frequency trip would encode
// "successfully" and mis-program a live inverter's grid protection. These
// bounds are deliberately generous — they reject gross/garbage values, not
// fine-grained grid-code policy (which lives in the gridprofile layer). The
// envelope mirrors the read-side sanity range so write and read agree on
// what is physically plausible.
const (
	protMinVolt    = 100.0
	protMaxVolt    = 600.0
	protMinHz      = 40.0
	protMaxHz      = 70.0
	protMaxSeconds = 1000.0 // clear/trip/delay/recovery times (s); generous ceiling
	protMaxSlope   = 200.0  // freq/volt-Watt droop slope (%Pref per Hz); generous ceiling
)

// protPhysicalRange classifies a protection param by physical quantity and
// returns its inclusive [lo,hi] envelope. ok=false means the param is not a
// simple bounded physical quantity (a mode enum, a %/Hz or %/V slope, or a
// DC-bus value with a different range) and is left to its own encoder's
// checks. Classification is by name so it covers every family uniformly.
func protPhysicalRange(paramName string) (lo, hi float64, ok bool) {
	if paramName == "Over_frequency_Watt_set" {
		return 0, 0, false // mode enum — bounded in qs1ModeFrames / ds3ModeEnum
	}
	n := strings.ToLower(paramName)
	switch {
	// Any time-domain param (clearance_time, triptime, Delay_Time,
	// recovery_time, start_time) is seconds — check first so the
	// "frequency"/"voltage" substrings in those names don't misclassify them.
	case strings.Contains(n, "time"):
		return 0, protMaxSeconds, true
	case strings.Contains(n, "slope"):
		return 0, protMaxSlope, true // %Pref/Hz droop gradient; bound against absurd values
	case strings.Contains(n, "bus"):
		return 0, 0, false // DC-bus voltage; different range
	case strings.Contains(n, "frequency"):
		return protMinHz, protMaxHz, true
	case strings.Contains(n, "voltage"), strings.Contains(n, "vmax"):
		return protMinVolt, protMaxVolt, true
	default:
		return 0, 0, false
	}
}

// validateProtectionEnvelope rejects a value outside its param's physical
// envelope before any frame is built. It is the single write-side safety
// backstop, applied on both the unicast and broadcast encode paths.
func validateProtectionEnvelope(paramName string, value float64) error {
	lo, hi, ok := protPhysicalRange(paramName)
	if !ok {
		return nil
	}
	if value < lo || value > hi {
		return fmt.Errorf("set-protection: %q value %g outside safe envelope [%g, %g]",
			paramName, value, lo, hi)
	}
	return nil
}

// ErrUnsupportedProtectionFamily is returned when the model code has no
// known protection-write encoder.
var ErrUnsupportedProtectionFamily = errors.New("set-protection: unsupported model family")

// ErrUnsupportedProtectionParam is returned when there is no single-frame
// encoding for (family, paramName). This covers params genuinely absent
// on a family (e.g. Over_frequency_Watt_Start_set has no DS3 branch)
// AND params whose firmware encoding is env-gated or multi-frame
// (the over-frequency mode enum, the slope/delay terms, the reconnect
// time) — those are deliberately not emitted here until their device-state
// dependence is resolved and validated on-wire.
var ErrUnsupportedProtectionParam = errors.New("set-protection: param not encodable for this family")

// ErrUnsupportedBroadcastParam is returned by EncodeSetProtectionBroadcast
// when a param cannot be broadcast via a clean opcode swap. This covers
// params that use a non-standard opcode on unicast (QS1 grid_recovery via
// 0x5D) or whose multi-frame sequence is not safe to broadcast (QS1 CV
// mode enum). These params must be applied via unicast instead.
var ErrUnsupportedBroadcastParam = errors.New("set-protection: param not encodable via broadcast for this family")

// IsProtectionWrite reports whether an L2 frame is a grid-protection
// write (as opposed to set-power or on/off), so the driver can schedule a
// read-back after one. The opcode/sub-byte that distinguishes a
// protection write from set-power is family-specific, so the predicates
// live in ds3.go / qs1a.go; this only parses the frame and combines them.
func IsProtectionWrite(frame []byte) bool {
	if len(frame) < 5 {
		return false
	}
	cmd, sub := frame[3], frame[4]
	return isDS3ProtectionWrite(cmd, sub) || isQS1ProtectionWrite(cmd, sub)
}

// EncodeSetProtection returns the unicast L2 frame(s) that program one
// grid-protection param on a given inverter. value is in the param's
// native unit: Hz for the frequency thresholds, %P/Hz for the slope,
// seconds for the delay/reconnect, the mode enum for the over-frequency
// switch. It dispatches per-inverter (unicast), not via the broadcast
// profile path; the per-family encoders live in ds3.go / qs1a.go.
//
// Returns a slice because a few params are multi-frame on the wire (the
// QS1 over-frequency mode enum emits 2-3 frames); single-frame params
// return a one-element slice. Caller sends each frame in order.
//
// grid_recovery_time uses the line-frequency cycle count on DS3 — encoded
// for 50 Hz (env 0/1); a 60 Hz site would need ×60 (see ds3.go).
func EncodeSetProtection(modelCode uint8, paramName string, value float64) ([][]byte, error) {
	if value <= 0 {
		return nil, fmt.Errorf("set-protection: %q: value must be > 0 (got %g)", paramName, value)
	}
	if err := validateProtectionEnvelope(paramName, value); err != nil {
		return nil, err
	}
	switch modelCode {
	case ModelDS3, ModelDS3H, ModelDS3L, ModelExt36:
		return encodeProtectionDS3(paramName, value, CmdSetPowerDS3Unicast)
	case ModelQS1, ModelQS1A:
		return encodeProtectionQS1A(paramName, value, CmdSetPowerQS1Unicast)
	default:
		return nil, fmt.Errorf("%w: model 0x%02X", ErrUnsupportedProtectionFamily, modelCode)
	}
}

// EncodeSetProtectionBroadcast returns the broadcast L2 frame(s) that
// program one grid-protection param across all enrolled inverters of a
// given family. The frame body (sub-byte, value scaling, width packing,
// checksum range) is byte-identical to the unicast frame produced by
// EncodeSetProtection; only the opcode at byte[3] differs (DS3 0xAB,
// QS1A 0x2C) and addressing is all-zero SA (handled by ecu-zb's bus-mgr).
//
// Params that cannot be cleanly broadcast — QS1 grid_recovery_time (uses
// the non-standard 0x5D YC600-builder opcode, not 0x1C) and QS1 CV mode
// enum (multi-frame sequence with sub 0x5F/0x99 whose broadcast semantics
// are not yet wire-confirmed) — return ErrUnsupportedBroadcastParam.
// Callers must fall back to per-inverter unicast for those params.
//
// DS3 CV mode enum (single sub-0x28 frame) is broadcast-safe: the opcode
// swap to 0xAB is unambiguous.
func EncodeSetProtectionBroadcast(modelCode uint8, paramName string, value float64) ([][]byte, error) {
	if value <= 0 {
		return nil, fmt.Errorf("set-protection: %q: value must be > 0 (got %g)", paramName, value)
	}
	if err := validateProtectionEnvelope(paramName, value); err != nil {
		return nil, err
	}
	switch modelCode {
	case ModelDS3, ModelDS3H, ModelDS3L, ModelExt36:
		return encodeProtectionDS3(paramName, value, CmdSetPowerDS3Broadcast)
	case ModelQS1, ModelQS1A:
		// The yc600-builder params (grid_recovery_time + the slow volt trips
		// AQ/AD/AC/AY + the grid freq trips AJ/AK/AE/AF) carry their sub
		// directly as the L2 cmd at frame[3] — there is no 0x1C opcode to
		// swap to the 0x2C broadcast form, so the unicast/broadcast
		// distinction can't be expressed by an opcode swap and their
		// broadcast semantics are not wire-confirmed.
		if isQS1YC600BuilderParam(paramName) {
			return nil, fmt.Errorf("%w: %q on QS1A (yc600 builder: sub-as-cmd, no clean broadcast opcode swap)",
				ErrUnsupportedBroadcastParam, paramName)
		}
		// Over_frequency_Watt_set (CV) on QS1A is a 2-3 frame 0x5F/0x99
		// sequence; the sub-bytes differ from the standard 0x2C protection
		// path and their broadcast semantics are not wire-confirmed.
		if paramName == "Over_frequency_Watt_set" {
			return nil, fmt.Errorf("%w: %q on QS1A (multi-frame 0x5F/0x99 sequence; broadcast not wire-confirmed)",
				ErrUnsupportedBroadcastParam, paramName)
		}
		return encodeProtectionQS1A(paramName, value, CmdSetPowerQS1Broadcast)
	default:
		return nil, fmt.Errorf("%w: model 0x%02X", ErrUnsupportedProtectionFamily, modelCode)
	}
}

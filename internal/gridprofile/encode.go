package gridprofile

import (
	"errors"
	"fmt"

	"github.com/bolkedebruin/openaps/codec"
)

// ErrUnsupported is returned when a code is not writable for the given
// model family.  It wraps codec sentinels so callers can inspect with
// errors.Is / errors.As.
var ErrUnsupported = errors.New("gridprofile: parameter not writable for this family")

// ErrOutOfRange is returned when a value cannot be clamped into the
// declared range (e.g. range itself is invalid).
var ErrOutOfRange = errors.New("gridprofile: value out of declared range")

// clamp enforces the range constraint.  Returns the clamped value and
// whether clamping was applied.  Returns ErrOutOfRange if the range itself
// is malformed (min > max).
func clamp(v float64, r *Range) (float64, bool, error) {
	if r == nil {
		return v, false, nil
	}
	if r.Min > r.Max {
		return v, false, fmt.Errorf("%w: range.min %g > range.max %g", ErrOutOfRange, r.Min, r.Max)
	}
	clamped := v
	if v < r.Min {
		clamped = r.Min
	} else if v > r.Max {
		clamped = r.Max
	}
	return clamped, clamped != v, nil
}

// EncodeEffectivePoint converts one resolved desired parameter value into
// the L2 wire frame(s) to send to an inverter.
//
// It:
//  1. Validates writability for the model family.
//  2. Clamps the native value to the declared range (logs if clamped).
//  3. Resolves the firmware long-name via the forward map.
//  4. Calls codec.EncodeSetProtection with the clamped native value.
//
// Returns ErrUnsupported (wrapping codec sentinels as needed) for any
// code/family combination that cannot be encoded.  Returns ErrOutOfRange
// if the range declaration is malformed.
func EncodeEffectivePoint(modelCode uint8, ep EffectivePoint) ([][]byte, error) {
	// 1. Capability check.
	if reason := UnsupportedReason(modelCode, ep.ApsCode); reason != "" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, reason)
	}

	// 2. Clamp to declared range.
	clamped, wasClamped, err := clamp(ep.NativeValue, ep.Range)
	if err != nil {
		return nil, err
	}
	if wasClamped {
		// Clamping is intentional per design §6; surface it via logging so
		// callers can decide whether to surface it further.
		_ = wasClamped // future: pass a logger or slog.Warn here
	}
	_ = clamped // used below

	// 3. Firmware long-name lookup.
	longName, ok := LongName(ep.ApsCode)
	if !ok {
		return nil, fmt.Errorf("%w: aps_code %q has no firmware encoder", ErrUnsupported, ep.ApsCode)
	}

	// 4. Encode via codec.
	frames, err := codec.EncodeSetProtection(modelCode, longName, clamped)
	if err != nil {
		if errors.Is(err, codec.ErrUnsupportedProtectionFamily) ||
			errors.Is(err, codec.ErrUnsupportedProtectionParam) {
			return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
		}
		return nil, fmt.Errorf("gridprofile encode %q model 0x%02X: %w", ep.ApsCode, modelCode, err)
	}
	return frames, nil
}

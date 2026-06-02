package server

import (
	"context"
	"fmt"

	"github.com/bolke/ecu-sunspec/internal/source"
	"github.com/bolke/ecu-sunspec/internal/sunspec"
)

// FreqWattCurveWriter handles Modbus writes to SunSpec Model 134
// (Curve-Based Frequency-Watt). Maps the four user-driven Hz points
// onto APsystems' DH/DI/CB/CC threshold codes via routeFor:
//
//	body[11] (Hz1) → Under_Frequency_Watt_Low_set  (DH)
//	body[13] (Hz2) → Under_Frequency_Watt_High_set (DI)
//	body[15] (Hz3) → Over_frequency_Watt_Low_set   (CB)
//	body[17] (Hz4) → Over_frequency_Watt_High_set  (CC)
//
// W1..W4 are intrinsic to the curve role (0/100/100/0); a SunSpec
// client writing them is rejected — the curve cannot be reshaped via
// W% on this firmware, the four thresholds are the only freedom.
//
// The trailing per-curve fields (CrvNam, RmpPT1Tms, RmpDecTmm,
// RmpIncTmm, RmpRsUp, SnptW, WRef, WRefStrHz, WRefStopHz, ReadOnly)
// don't map onto APsystems firmware at all and are read-only here.
//
// Header fields (ActCrv, ModEna, WinTms, RvrtTms, RmpTms, NCrv, NPt,
// scale factors) and the unused points Hz5..W20 are also read-only.
type FreqWattCurveWriter struct {
	uid     uint8
	snap    source.Snapshot
	sender  frameSender
	tracker *WriteTracker
}

// fwHzPoint binds a Model 134 Hz body offset to its APsystems long-form
// param name and 2-letter code.
type fwHzPoint struct {
	body uint16 // body offset of the HzN register
	aps  string
	code string
}

var freqWattCurveHzPoints = []fwHzPoint{
	{body: sunspec.FreqWattCurveBodyHz1Off, aps: "Under_Frequency_Watt_Low_set", code: "DH"},
	{body: sunspec.FreqWattCurveBodyHz2Off, aps: "Under_Frequency_Watt_High_set", code: "DI"},
	{body: sunspec.FreqWattCurveBodyHz3Off, aps: "Over_frequency_Watt_Low_set", code: "CB"},
	{body: sunspec.FreqWattCurveBodyHz4Off, aps: "Over_frequency_Watt_High_set", code: "CC"},
}

// freqWattCurveWritableSet enumerates the body offsets the writer
// accepts. Anything else triggers an "is read-only" error.
var freqWattCurveWritableSet = map[uint16]bool{
	sunspec.FreqWattCurveBodyHz1Off: true,
	sunspec.FreqWattCurveBodyHz2Off: true,
	sunspec.FreqWattCurveBodyHz3Off: true,
	sunspec.FreqWattCurveBodyHz4Off: true,
}

func (fwc *FreqWattCurveWriter) Apply(ctx context.Context, addrOffset uint16, regs []uint16) error {
	if fwc == nil || fwc.sender == nil {
		return fmt.Errorf("writes disabled or inv-driver not configured")
	}
	if fwc.uid <= 1 {
		return fmt.Errorf("Model 134 writes only valid on per-inverter unit IDs (2..N+1)")
	}
	idx := int(fwc.uid) - 2
	if idx < 0 || idx >= len(fwc.snap.Inverters) {
		return fmt.Errorf("unit ID %d not mapped to any inverter", fwc.uid)
	}
	inv := fwc.snap.Inverters[idx]

	// Reject writes that touch any read-only register. W1..W4 are
	// rejected with a specific message — they look writable per the
	// SunSpec spec, but on this APsystems-bridge they're intrinsic to
	// the curve role and can't be retargeted without breaking the
	// freq-watt semantics on the inverter side.
	for off := uint16(0); off < sunspec.FreqWattCurveBodyLen; off++ {
		if !writeTouches(addrOffset, regs, off) {
			continue
		}
		if freqWattCurveWritableSet[off] {
			continue
		}
		switch off {
		case sunspec.FreqWattCurveBodyW1Off,
			sunspec.FreqWattCurveBodyW2Off,
			sunspec.FreqWattCurveBodyW3Off,
			sunspec.FreqWattCurveBodyW4Off:
			return fmt.Errorf("Model 134 W%% values are intrinsic to the curve role "+
				"(0/100/100/0) and cannot be rewritten — only Hz points are writable; "+
				"offset %d", off)
		default:
			return fmt.Errorf("Model 134 body offset %d is read-only "+
				"(only Hz1/Hz2/Hz3/Hz4 are writable on this APsystems bridge)", off)
		}
	}

	// dispatch routes a single (paramName, value) write: frequency
	// thresholds encode to an L2 frame and go through inv-driver,
	// env-gated / multi-frame params fall back to SQLite.
	dispatch := func(paramName string, value float64) error {
		return dispatchProtection(ctx, fwc.sender, inv, paramName, value)
	}

	// Iterate the four points; for each Hz the client touched, decode
	// from centi-Hz wire form and dispatch. Track expected post-write
	// state so the next snapshot reconcile knows what counted as
	// "applied".
	expected := map[string]float64{}
	any := false
	for _, pt := range freqWattCurveHzPoints {
		if !writeTouches(addrOffset, regs, pt.body) {
			continue
		}
		any = true
		wire := readField(addrOffset, regs, pt.body)
		// Hz_SF = -2 → wire is centi-Hz absolute.
		hz := float64(wire) / 100.0
		if hz <= 0 {
			return fmt.Errorf("%s (offset %d): Hz must be > 0 (got wire=%d)", pt.aps, pt.body, wire)
		}
		if err := dispatch(pt.aps, hz); err != nil {
			return fmt.Errorf("set %s: %w", pt.aps, err)
		}
		expected[pt.code] = hz
	}
	if !any {
		return fmt.Errorf("Model 134 write must touch at least one writable field (Hz1..Hz4)")
	}

	if fwc.tracker != nil && len(expected) > 0 {
		fwc.tracker.Track(fwc.uid, sunspec.FreqWattCurveModelID, expected)
	}
	return nil
}

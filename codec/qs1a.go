package codec

import "math"

// QS1A reply payload layout, body offset 0 = byte right after the
// L2 cmd 0xB1 (i.e. main.exe's `param_1` to resolvedata_60_1200,
// which is l2_frame[4..]).
//
// Constants from main.exe .rodata (Ghidra dumps at 0x1dda8..0x1de8
// and 0x1ea00):
//
//	busPFBase    = 2.85   @ 0x1dda8  (bus V denom)
//	busScale     = 3300.0 @ 0x1ddb0  (bus V numer)
//	busDenom     = 4092.0 @ 0x1ddb8  (bus V denom inner)
//	busOffset    = 757.0  @ 0x1ddc0  (bus V offset)
//	freqDivBy    = 5e7    @ 0x1ddc8  (Hz = 5e7 / raw_3byte)
//	panelVMax    = 82.5   @ 0x1ddd0  (panel V scale, 12-bit ADC)
//	panelIMax    = 27.5   @ 0x1ddd8  (panel I scale, 12-bit ADC)
//	adc12bit     = 4096.0 @ 0x1dde0
//	gridVDenom   = 1.32   @ 0x1dde8  (V_grid = raw16 / 1.32 / 4)
//	efficiency   = 0.95   @ 0x1ea00  (active power scale)
//	lifetime     = 3.756e-11           (per-byte → kWh, in main.exe code)
const (
	qs1aBusScale  = 3300.0
	qs1aBusDenom  = 4092.0
	qs1aBusOffset = 757.0
	qs1aBusPFBase = 2.85
	qs1aFreqDivBy = 50_000_000.0
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
//	[0..1]   internal DC bus voltage (16-bit ADC) → V via the busPF formula.
//	         Stored at inv+0x4c/0x1d0 by main.exe; printed as "bus_vol".
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
	type pkt struct{ off int }
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
	// main.exe applies the same 0.95 (DAT_0001ea00) before storing
	// at param_2 + 0x1f4. Bypassing main.exe's MAXOP and DC-vs-energy
	// tie-break logic (those are for ECU power-limiting, not raw
	// telemetry).
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
}

// qs1aBusV applies main.exe's internal-DC-link formula:
//
//	V = (raw * 3300/4092 - 757) / 2.85
//
// This is the value main.exe prints as "bus_vol".
func qs1aBusV(raw uint16) float64 {
	return (float64(raw)*qs1aBusScale/qs1aBusDenom - qs1aBusOffset) / qs1aBusPFBase
}

// qs1aGridV applies main.exe's grid voltage formula:
//
//	V = (raw_16bit / 1.32) / 4
func qs1aGridV(raw uint16) float64 {
	return (float64(raw) / qs1aGridDenom) / 4.0
}

// qs1aFreqHz applies main.exe's formula:
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

// LifetimeKWh returns the per-panel lifetime energy in kWh using
// the model's scale. Convenience for callers; equivalent to
// raw * Reply.LifetimeScale.
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

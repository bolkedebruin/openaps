package codec

// DS3 reply payload layout, body offset 0 = byte right after the L2
// cmd 0xBB (= main.exe `param_1` for resolvedata_DS3, which is
// l2_frame[4..]).
//
// Magic constants from main.exe .rodata (read via Ghidra at
// 0x1f770..0x1f790):
//
//	panelVScale  = 0.020   @ 0x1f770  (panel V = raw16 × 0.020)
//	panelIScale  = 0.01172 @ 0x1f778  (panel I = raw16 × 0.01172)
//	gridVScale   = 0.268   @ 0x1f780  (grid V = raw16 × 0.268)
//	freqScale    = 0.010   @ 0x1f788  (freq Hz = raw16 × 0.010)
//	tempScale    = 0.321   @ 0x1f790  (some temperature, optional)
//	lifetime     = 1.674e-08            (per-byte → kWh, hard-coded)
const (
	ds3PanelVScale = 0.020
	ds3PanelIScale = 0.01172
	ds3GridVScale  = 0.268
	ds3FreqScale   = 0.010
	ds3Lifetime    = 1.674e-08
)

// DS3 reply payload layout (from resolvedata_DS3 @ 0x1f45c):
//
//	[0x10..0x11]  panel A V raw       × 0.020
//	[0x12..0x13]  panel B V raw       × 0.020
//	[0x14..0x15]  panel A I raw       × 0.01172
//	[0x16..0x17]  panel B I raw       × 0.01172
//	[0x18..0x19]  grid V raw          × 0.268
//	[0x1a..0x1b]  freq raw            × 0.010
//	[0x1c..0x1d]  inverter report counter (16-bit BE)
//	[0x1e..0x1f]  active power (u16, W) — stored at inv+0x1f4
//	[0x20..0x21]  reactive power (s16, VAR) — stored at inv+0x1e8
//	[0x28..0x2b]  panel A lifetime accumulator (4-byte BE)
//	[0x2c..0x2f]  panel B lifetime
//
// Status bit flags at [0x0b..0x0f] are decoded by main.exe but we
// skip them here — telemetry is what callers care about for v1.
func decodeDS3(body []byte, r *Reply) {
	if len(body) < 0x30 {
		return
	}
	r.GridV = fround(float64(be16(body, 0x18)) * ds3GridVScale)
	r.FreqHz = fround(float64(be16(body, 0x1a)) * ds3FreqScale)
	r.ReportSec = uint32(be16(body, 0x1c))

	for _, pp := range []struct {
		idx, vOff, iOff int
	}{
		{0, 0x10, 0x14}, // A
		{1, 0x12, 0x16}, // B
	} {
		v := float64(be16(body, pp.vOff)) * ds3PanelVScale
		i := float64(be16(body, pp.iOff)) * ds3PanelIScale
		r.Panels = append(r.Panels, Panel{
			Index: pp.idx,
			DCV:   fround(v),
			DCI:   fround(i),
			W:     fround(v * i),
		})
	}

	r.ActivePowerW = float64(be16(body, 0x1e))
	r.ReactivePower = float64(int16(be16(body, 0x20)))

	r.LifetimeRaw = []uint64{
		charsToUint(body, 0x28, 4),
		charsToUint(body, 0x2c, 4),
	}
	r.LifetimeScale = ds3Lifetime
}

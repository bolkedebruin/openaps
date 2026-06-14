package codec

// QS2 (model code 0x36) is single-phase 4-channel and rides the
// DS3-class wire protocol: its telemetry reply arrives under the same L2
// command 0xBB and 99-byte envelope as DS3, so the firmware
// (zb_query_data) selects this layout by inv->model rather than the
// command byte. The field layout matches DS3 for channels A/B and
// extends it with channels C/D and their lifetime accumulators.
//
// Wire scales (from main.exe resolvedata_QS2 @ 0x21eb0):
//
//	panelVScale = 0.02016   (channel V = raw16 × 0.02016)
//	panelIScale = 0.0117    (channel I = raw16 × 0.0117)
//	gridVScale  = 0.268     (grid V = raw16 × 0.268)
//	freqScale   = 0.010     (freq Hz = raw16 × 0.010)
//	lifetime    = 1.674e-08 (per raw unit → kWh)
const (
	qs2PanelVScale = 0.02016
	qs2PanelIScale = 0.0117
	qs2GridVScale  = 0.268
	qs2FreqScale   = 0.010
	qs2Lifetime    = 1.674e-08
)

func init() {
	registerDS3ClassDecoder(ModelQS2, decodeQS2)
}

// QS2 reply payload layout, body offset 0 = byte right after the L2 cmd
// 0xBB (firmware passes reply+4 to resolvedata_QS2):
//
//	[0x0b..0x0f]  status / fault bitfield — same block as DS3
//	[0x10..0x11]  channel A V raw   × 0.02016
//	[0x12..0x13]  channel B V raw   × 0.02016
//	[0x14..0x15]  channel A I raw   × 0.0117
//	[0x16..0x17]  channel B I raw   × 0.0117
//	[0x18..0x19]  grid V raw        × 0.268
//	[0x1a..0x1b]  freq raw          × 0.010
//	[0x1c..0x1d]  inverter report counter (16-bit BE)
//	[0x1e..0x1f]  active power (u16, W)
//	[0x20..0x21]  reactive power (s16, VAR)
//	[0x28..0x2b]  channel A lifetime accumulator (4-byte BE)
//	[0x2c..0x2f]  channel B lifetime
//	[0x34..0x35]  channel C V raw   × 0.02016
//	[0x36..0x37]  channel D V raw   × 0.02016
//	[0x38..0x39]  channel C I raw   × 0.0117
//	[0x3a..0x3b]  channel D I raw   × 0.0117
//	[0x3c..0x3f]  channel C lifetime
//	[0x40..0x43]  channel D lifetime
//
// Active vs reactive: the firmware stores body[0x1e] at inv+0x1f4 and
// body[0x20] at inv+0x1e8 — the same struct slots DS3 uses for active
// and reactive power respectively — so the assignment follows the
// validated DS3 mapping.
func decodeQS2(body []byte, r *Reply) {
	if len(body) < 0x44 {
		return
	}
	r.GridV = fround(float64(be16(body, 0x18)) * qs2GridVScale)
	r.FreqHz = fround(float64(be16(body, 0x1a)) * qs2FreqScale)
	r.ReportSec = uint32(be16(body, 0x1c))

	for _, pp := range []struct {
		idx, vOff, iOff int
	}{
		{0, 0x10, 0x14}, // A
		{1, 0x12, 0x16}, // B
		{2, 0x34, 0x38}, // C
		{3, 0x36, 0x3a}, // D
	} {
		v := float64(be16(body, pp.vOff)) * qs2PanelVScale
		i := float64(be16(body, pp.iOff)) * qs2PanelIScale
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
		charsToUint(body, 0x28, 4), // A
		charsToUint(body, 0x2c, 4), // B
		charsToUint(body, 0x3c, 4), // C
		charsToUint(body, 0x40, 4), // D
	}
	r.LifetimeScale = qs2Lifetime

	// QS2 carries the same status block at the same offsets as DS3 and
	// is in the DS3 family, so the DS3 fault decoder applies.
	ds := decodeDS3Status(body)
	r.ExtendedStatus = ds
	r.Status = ds.Faults().InverterStatus()
}

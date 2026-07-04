package codec

// QT2 (three-phase) telemetry decode.
//
// QT2 shares the DS3-class reply command (0xBB) but carries a distinct
// 99-byte payload. Layout and scales are from resolvedata_Qt2 @ 0x23894 /
// resolvedata_Qt2D @ 0x254bc in the stock firmware; body offset 0 is the
// byte right after the L2 cmd (== the decoder's param_1 base). Model codes
// 0x29/0x30 dispatch to Qt2, 0x31/0x32 to Qt2D — the layout and scale
// constants are byte-identical; Qt2D additionally multiplies the per-leg
// power by a runtime get_correctionCoefficient gain that is not present on
// the wire, so this decoder treats the gain as 1.
//
// UNVALIDATED: decoded statically from the firmware; not yet confirmed
// against a live QT2 frame (no QT2 in the reference fleet).
//
// The four DC channels are two voltage rails (0x10, 0x12) crossed with four
// currents (0x14/0x16/0x18/0x1a); per-channel power is V×I and the four sum
// to total AC power. The genuinely per-grid-leg AC quantity is the leg
// voltage (0x1e/0x20/0x22 ×0.1). Per-leg AC power/current are NOT on the
// wire — the firmware itself splits total power evenly across the three
// legs — so this decoder surfaces total power plus the three leg voltages
// and leaves the even split to the SunSpec encoder.
const (
	qt2PanelVScale = 0.038   // DAT_00023be0
	qt2PanelIScale = 0.0117  // DAT_00023be8
	qt2GridVScale  = 0.1     // DAT_00023bf0
	qt2FreqScale   = 0.3396  // DAT_00023c00 (line/utility frequency @ 0x24)
	qt2Lifetime    = 3.16e-8 // per-leg energy accumulator raw → Wh
)

// qt2Channels maps each DC channel to its (voltage rail, current) body
// offsets. Two voltage rails feed four currents, per resolvedata_Qt2.
var qt2Channels = []struct{ vOff, iOff int }{
	{0x10, 0x14},
	{0x10, 0x16},
	{0x12, 0x18},
	{0x12, 0x1a},
}

func init() {
	// 0x29/0x30 = QT2, 0x31/0x32 = QT2D. All four share the layout; the
	// QT2D coefficient is a runtime-only gain (absent from the frame).
	//
	// Note the deliberate asymmetry with family.go: telemetry decodes for all
	// four codes, but FamilyOf/IsKnownModel classify only ModelQT2 (0x32) —
	// Ext29-31's wire family is not pinned, so they decode (leg voltages +
	// even power split, model 103) but stay family-unknown (no TypeCode →
	// no MPPT bank). This is partial support by design, not an oversight.
	for _, code := range []uint8{ModelExt29, ModelExt30, ModelExt31, ModelQT2} {
		registerDS3ClassDecoder(code, decodeQT2)
	}
}

func decodeQT2(body []byte, r *Reply) {
	// Need through the last energy accumulator at 0x48..0x4b.
	if len(body) < 0x4c {
		return
	}
	r.Family = FamilyQT2

	var total float64
	for idx, ch := range qt2Channels {
		v := float64(be16(body, ch.vOff)) * qt2PanelVScale
		i := float64(be16(body, ch.iOff)) * qt2PanelIScale
		w := v * i
		total += w
		r.Panels = append(r.Panels, Panel{
			Index: idx,
			DCV:   fround(v),
			DCI:   fround(i),
			W:     fround(w),
		})
	}
	r.ActivePowerW = fround(total)

	// Per-grid-leg AC voltage (L1/L2/L3). r.GridV keeps the L1 value for
	// callers that expect a single scalar.
	r.GridVLeg = []float64{
		fround(float64(be16(body, 0x1e)) * qt2GridVScale),
		fround(float64(be16(body, 0x20)) * qt2GridVScale),
		fround(float64(be16(body, 0x22)) * qt2GridVScale),
	}
	r.GridV = r.GridVLeg[0]

	r.FreqHz = fround(float64(be16(body, 0x24)) * qt2FreqScale)

	r.LifetimeRaw = []uint64{
		charsToUint(body, 0x3c, 4),
		charsToUint(body, 0x40, 4),
		charsToUint(body, 0x44, 4),
		charsToUint(body, 0x48, 4),
	}
	r.LifetimeScale = qt2Lifetime
}

package codec

import "testing"

func TestDecodeQT2(t *testing.T) {
	body := make([]byte, 0x50)
	// Two DC voltage rails.
	putBE16(body, 0x10, 1316) // rail A ~50.0 V
	putBE16(body, 0x12, 1263) // rail B ~48.0 V
	// Four DC currents.
	putBE16(body, 0x14, 427) // ch0 I ~5.0 A
	putBE16(body, 0x16, 342) // ch1 I ~4.0 A
	putBE16(body, 0x18, 256) // ch2 I ~3.0 A
	putBE16(body, 0x1a, 171) // ch3 I ~2.0 A
	// Three grid-leg voltages.
	putBE16(body, 0x1e, 2300) // L1 230.0 V
	putBE16(body, 0x20, 2310) // L2 231.0 V
	putBE16(body, 0x22, 2320) // L3 232.0 V
	// Line frequency.
	putBE16(body, 0x24, 147) // ~50 Hz
	// Energy accumulators.
	for i, off := range []int{0x3c, 0x40, 0x44, 0x48} {
		putBE16(body, off+2, uint16(1000*(i+1))) // low word set, high word 0
	}

	var r Reply
	decodeQT2(body, &r)

	if r.Family != FamilyQT2 {
		t.Errorf("Family = %v; want FamilyQT2", r.Family)
	}
	if len(r.Panels) != 4 {
		t.Fatalf("Panels = %d; want 4", len(r.Panels))
	}

	// ActivePowerW is the sum of the four channel powers (V×I), using the
	// same rail/current crossing the decoder does.
	rail := []float64{
		float64(1316) * qt2PanelVScale, // A
		float64(1263) * qt2PanelVScale, // B
	}
	cur := []float64{
		float64(427) * qt2PanelIScale,
		float64(342) * qt2PanelIScale,
		float64(256) * qt2PanelIScale,
		float64(171) * qt2PanelIScale,
	}
	wantTotal := fround(rail[0]*cur[0] + rail[0]*cur[1] + rail[1]*cur[2] + rail[1]*cur[3])
	if r.ActivePowerW != wantTotal {
		t.Errorf("ActivePowerW = %v; want %v", r.ActivePowerW, wantTotal)
	}

	wantLegs := []float64{230, 231, 232}
	if len(r.GridVLeg) != 3 {
		t.Fatalf("GridVLeg len = %d; want 3", len(r.GridVLeg))
	}
	for i, w := range wantLegs {
		if r.GridVLeg[i] != w {
			t.Errorf("GridVLeg[%d] = %v; want %v", i, r.GridVLeg[i], w)
		}
	}
	if r.GridV != 230 {
		t.Errorf("GridV = %v; want 230 (L1)", r.GridV)
	}
	if want := fround(float64(147) * qt2FreqScale); r.FreqHz != want {
		t.Errorf("FreqHz = %v; want %v", r.FreqHz, want)
	}
	if len(r.LifetimeRaw) != 4 || r.LifetimeScale != qt2Lifetime {
		t.Errorf("lifetime raw=%v scale=%v", r.LifetimeRaw, r.LifetimeScale)
	}
}

func TestDecodeQT2_ShortBodyNoPanic(t *testing.T) {
	var r Reply
	decodeQT2(make([]byte, 0x20), &r) // too short; must be a no-op
	if len(r.Panels) != 0 || len(r.GridVLeg) != 0 {
		t.Fatalf("short body populated fields: panels=%d legs=%d", len(r.Panels), len(r.GridVLeg))
	}
}

// TestQT2RegisteredForThreePhaseCodes proves the DS3-class dispatch routes
// every three-phase model code to the QT2 decoder.
func TestQT2RegisteredForThreePhaseCodes(t *testing.T) {
	for _, code := range []uint8{ModelExt29, ModelExt30, ModelExt31, ModelQT2} {
		if ds3ClassDecoders[code] == nil {
			t.Errorf("model 0x%02x has no DS3-class decoder registered", code)
		}
	}
}

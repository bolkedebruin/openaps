package sunspec

import (
	"math"
	"testing"
)

func TestEncode_EmitsModel111And114_Aggregate(t *testing.T) {
	bank := Encode(sampleSnapshot(), Options{})

	hdr, ok := findModel(bank, InverterModelSinglePhaseFloat)
	if !ok {
		t.Fatal("Model 111 (Inverter Float Single Phase) missing")
	}
	if got := bank.At(hdr + 1); got != InverterModelFloatBodyLen {
		t.Errorf("Model 111 L=%d want %d", got, InverterModelFloatBodyLen)
	}

	// Body starts at hdr+2. Read W (offset 20 in body, float32) and verify
	// it round-trips the snapshot's SystemPowerW.
	body := hdr + 2
	wHi := bank.At(body + 20)
	wLo := bank.At(body + 21)
	w := math.Float32frombits(uint32(wHi)<<16 | uint32(wLo))
	if int32(w) != int32(sampleSnapshot().SystemPowerW) {
		t.Errorf("Model 111 W=%v want %v", w, sampleSnapshot().SystemPowerW)
	}

	// Hz at offset 22.
	hzHi := bank.At(body + 22)
	hzLo := bank.At(body + 23)
	hz := math.Float32frombits(uint32(hzHi)<<16 | uint32(hzLo))
	if math.Abs(float64(hz)-sampleSnapshot().GridFrequencyHz) > 0.01 {
		t.Errorf("Model 111 Hz=%v want %v", hz, sampleSnapshot().GridFrequencyHz)
	}

	// VA / VAr / PF (offsets 24, 26, 28) must be NaN — APsystems doesn't measure them.
	for _, off := range []uint16{24, 26, 28} {
		hi := bank.At(body + off)
		lo := bank.At(body + off + 1)
		f := math.Float32frombits(uint32(hi)<<16 | uint32(lo))
		if !math.IsNaN(float64(f)) {
			t.Errorf("Model 111 offset %d: got %v, want NaN", off, f)
		}
	}

	dc, ok := findModel(bank, DCDataFloatModelID)
	if !ok {
		t.Fatal("Model 114 (DC Data Float) missing")
	}
	if got := bank.At(dc + 1); got != DCDataFloatBodyLen {
		t.Errorf("Model 114 L=%d want %d", got, DCDataFloatBodyLen)
	}
}

func TestEncode_Model111_PhaseGating(t *testing.T) {
	// Force three-phase model — AphB/AphC must be real (or 0), PhVphB/C must be real.
	bank := Encode(sampleSnapshot(), Options{Phase: PhaseThree})

	hdr, ok := findModel(bank, InverterModelThreePhaseFloat)
	if !ok {
		t.Fatal("Model 113 missing under PhaseThree")
	}
	body := hdr + 2

	// AphB at offset 4, AphC at offset 6. Should NOT be NaN under PhaseThree.
	for _, off := range []uint16{4, 6} {
		hi := bank.At(body + off)
		lo := bank.At(body + off + 1)
		f := math.Float32frombits(uint32(hi)<<16 | uint32(lo))
		if math.IsNaN(float64(f)) {
			t.Errorf("PhaseThree: offset %d (AphB/C) is NaN, want real value", off)
		}
	}

	// PhVphB at +16, PhVphC at +18 — should also be real.
	for _, off := range []uint16{16, 18} {
		hi := bank.At(body + off)
		lo := bank.At(body + off + 1)
		f := math.Float32frombits(uint32(hi)<<16 | uint32(lo))
		if math.IsNaN(float64(f)) {
			t.Errorf("PhaseThree: offset %d (PhVphB/C) is NaN, want real value", off)
		}
	}
}

func TestEncode_Model111_SinglePhaseGating(t *testing.T) {
	bank := Encode(sampleSnapshot(), Options{Phase: PhaseSingle})

	hdr, ok := findModel(bank, InverterModelSinglePhaseFloat)
	if !ok {
		t.Fatal("Model 111 missing under PhaseSingle")
	}
	body := hdr + 2

	// AphB / AphC / PhVphB / PhVphC must be NaN.
	for _, off := range []uint16{4, 6, 16, 18} {
		hi := bank.At(body + off)
		lo := bank.At(body + off + 1)
		f := math.Float32frombits(uint32(hi)<<16 | uint32(lo))
		if !math.IsNaN(float64(f)) {
			t.Errorf("PhaseSingle: offset %d: got %v, want NaN", off, f)
		}
	}
}

func TestEncodePerInverter_EmitsModel111And114(t *testing.T) {
	s := sampleSnapshot()
	if len(s.Inverters) == 0 {
		t.Fatal("sample snapshot has no inverters")
	}
	inv := s.Inverters[0]

	bank := EncodePerInverter(inv, s.ECUID, 2, Options{})

	hdr, ok := findModel(bank, InverterModelSinglePhaseFloat)
	if !ok {
		t.Fatal("Per-inverter Model 111 missing")
	}
	if got := bank.At(hdr + 1); got != InverterModelFloatBodyLen {
		t.Errorf("Model 111 L=%d want %d", got, InverterModelFloatBodyLen)
	}

	dc, ok := findModel(bank, DCDataFloatModelID)
	if !ok {
		t.Fatal("Per-inverter Model 114 missing")
	}
	if got := bank.At(dc + 1); got != DCDataFloatBodyLen {
		t.Errorf("Model 114 L=%d want %d", got, DCDataFloatBodyLen)
	}

	// First channel DCW should equal first panel power (if any).
	powers := inv.PanelPowers()
	if len(powers) > 0 {
		body := dc + 2
		// DCW1 lives at body + 16*2 (DCV1..8 use 16 regs, DCA1..8 use 16 regs)
		dcwOff := body + uint16(2*DCDataFloatChannels*2) // 32 regs into body
		hi := bank.At(dcwOff)
		lo := bank.At(dcwOff + 1)
		got := math.Float32frombits(uint32(hi)<<16 | uint32(lo))
		if int(got) != powers[0] {
			t.Errorf("Model 114 DCW1=%v want %d", got, powers[0])
		}
	}
}

func TestModel114_UnusedChannelsAreNaN(t *testing.T) {
	s := sampleSnapshot()
	inv := s.Inverters[0]

	bank := EncodePerInverter(inv, s.ECUID, 2, Options{})
	dc, ok := findModel(bank, DCDataFloatModelID)
	if !ok {
		t.Fatal("Model 114 missing")
	}
	body := dc + 2

	powers := inv.PanelPowers()
	// DCW7 / DCW8 (channels beyond panel count) must be NaN. DCW base is at
	// body + 32; channel i lives at body + 32 + 2*i.
	for ch := len(powers); ch < DCDataFloatChannels; ch++ {
		off := body + uint16(2*DCDataFloatChannels*2) + uint16(2*ch)
		hi := bank.At(off)
		lo := bank.At(off + 1)
		f := math.Float32frombits(uint32(hi)<<16 | uint32(lo))
		if !math.IsNaN(float64(f)) {
			t.Errorf("Model 114 DCW channel %d: got %v, want NaN", ch+1, f)
		}
	}
}

func TestNotImplFloat32IsNaN(t *testing.T) {
	if !math.IsNaN(float64(math.Float32frombits(notImplFloat32))) {
		t.Fatal("notImplFloat32 is not a NaN; SunSpec spec requires NaN sentinel for unimplemented float fields")
	}
}

func TestEncode_BankWalk_IncludesModels111_114(t *testing.T) {
	// Confirms a SunSpec scanner walking the chain hits both models.
	bank := Encode(sampleSnapshot(), Options{})
	seen := map[uint16]bool{}
	addr := bank.Base + 2 // skip "SunS" magic
	for i := 0; i < 64; i++ {
		id := bank.At(addr)
		l := bank.At(addr + 1)
		if id == EndModelID {
			break
		}
		seen[id] = true
		addr += 2 + l
	}
	for _, want := range []uint16{1, 101, InverterModelSinglePhaseFloat, DCDataFloatModelID} {
		if !seen[want] {
			t.Errorf("model %d not found in bank chain (saw %v)", want, seen)
		}
	}
}

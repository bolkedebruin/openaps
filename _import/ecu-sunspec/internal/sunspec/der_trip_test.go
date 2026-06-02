package sunspec

import (
	"testing"

	"github.com/bolke/ecu-sunspec/internal/source"
)

func TestEmitDERTripModels_LengthsAndContent(t *testing.T) {
	// DS3 (UID 704...) active values from live fixture.
	p := source.ProtectionParams{
		UVStg2: 195, UVFast: 183, OVStg2: 263, OVStg3: 259, AvgOV: 260,
		UFSlow: 47.5, UFFast: 47.0, OFSlow: 52.0, OFFast: 52.0,
		UV2ClrS: 0.1, UV3ClrS: 1.2, OV2ClrS: 0.1, OV3ClrS: 0.1,
		UF2ClrS: 0.3, UF1ClrS: 0.16, OF2ClrS: 0.3, OF1ClrS: 0.3,
		Has: map[string]bool{
			"AC": true, "AQ": true, "AD": true, "AY": true, "AB": true,
			"AE": true, "AJ": true, "AF": true, "AK": true,
			"BB": true, "BD": true, "BC": true, "BE": true,
			"BJ": true, "BH": true, "BK": true, "BI": true,
		},
	}
	bank := Bank{Regs: make([]uint16, 0, 200), Base: BaseRegister}
	emitDERTripModels(&bank, p, 230.0)

	// 707 + 708 each are 40 regs (V is uint16); 709 + 710 each are 49 regs
	// (Hz is uint32). Total = 80 + 98 = 178.
	if got := len(bank.Regs); got != 178 {
		t.Fatalf("emitDERTripModels regs=%d want 178", got)
	}
	walk := func(off int, wantID, wantBody uint16) {
		t.Helper()
		if bank.Regs[off] != wantID {
			t.Errorf("offset %d ID=%d want %d", off, bank.Regs[off], wantID)
		}
		if bank.Regs[off+1] != wantBody {
			t.Errorf("offset %d L=%d want %d", off+1, bank.Regs[off+1], wantBody)
		}
	}
	walk(0, 707, derTripVBodyLen)
	walk(40, 708, derTripVBodyLen)
	walk(80, 709, derTripHzBodyLen)
	walk(80+49, 710, derTripHzBodyLen)

	// Model 707 — LV trip — verify MustTrip points
	// Header at 0..8; Curve sub-block starts at 9 with ReadOnly.
	// MustTrip ActPt at idx 10, Pt[0].V at idx 11, Pt[0].Tms at 12..13, Pt[1].V at 14, Pt[1].Tms at 15..16.
	if got := bank.Regs[10]; got != 2 {
		t.Errorf("707 MustTrip ActPt=%d want 2 (AC + AQ)", got)
	}
	// Pt[0].V = AC=195V at VNom=230 → 195/230*10000 = 8478 (centi-pct)
	if got := bank.Regs[11]; got < 8470 || got > 8485 {
		t.Errorf("707 MustTrip Pt[0].V=%d want ~8478 (195V/230V·100·100)", got)
	}
	// Pt[0].Tms = BB=0.1s × 100 = 10
	tms0 := uint32(bank.Regs[12])<<16 | uint32(bank.Regs[13])
	if tms0 != 10 {
		t.Errorf("707 MustTrip Pt[0].Tms=%d want 10 (0.1s × centi-secs)", tms0)
	}
	// Pt[1].V = AQ=183V at VNom=230 → 183/230*10000 = 7956
	if got := bank.Regs[14]; got < 7950 || got > 7960 {
		t.Errorf("707 MustTrip Pt[1].V=%d want ~7956", got)
	}
	// Pt[1].Tms = BD=1.2s × 100 = 120
	tms1 := uint32(bank.Regs[15])<<16 | uint32(bank.Regs[16])
	if tms1 != 120 {
		t.Errorf("707 MustTrip Pt[1].Tms=%d want 120", tms1)
	}

	// Model 710 (HF) header starts at offset 80+49=129. MustTrip ActPt is at
	// header+9 (curve start) + 1 (ReadOnly) = header+10. AF + AK = 2 active.
	hf := 129
	if got := bank.Regs[hf+10]; got != 2 {
		t.Errorf("710 MustTrip ActPt=%d want 2", got)
	}
	// Pt[0].Hz is uint32 starting at hf+11. AF=52.0 → 5200 in centi-Hz.
	hz0 := uint32(bank.Regs[hf+11])<<16 | uint32(bank.Regs[hf+12])
	if hz0 != 5200 {
		t.Errorf("710 MustTrip Pt[0].Hz=%d want 5200", hz0)
	}

	// MayTrip / MomCess ActPt must be 0 across all four models.
	// V models: MayTrip ActPt at base+20, MomCess at base+30.
	for _, base := range []int{0, 40} {
		if bank.Regs[base+20] != 0 || bank.Regs[base+30] != 0 {
			t.Errorf("V model @%d non-zero MayTrip/MomCess ActPt", base)
		}
	}
	// Hz models: tier is 13 regs not 10; MayTrip ActPt at curveBase+13,
	// MomCess at curveBase+26 (curveBase = headerBase+9+1=headerBase+10).
	for _, base := range []int{80, 129} {
		curveBase := base + 10
		if bank.Regs[curveBase+13] != 0 || bank.Regs[curveBase+26] != 0 {
			t.Errorf("Hz model @%d non-zero MayTrip/MomCess ActPt", base)
		}
	}
}

func TestEmitDERTripModels_EmptyProtection(t *testing.T) {
	// No params → all four models still emit, total 178 regs.
	bank := Bank{Regs: make([]uint16, 0, 200), Base: BaseRegister}
	emitDERTripModels(&bank, source.ProtectionParams{Has: map[string]bool{}}, 230)

	if got := len(bank.Regs); got != 178 {
		t.Fatalf("regs=%d want 178", got)
	}
	for _, base := range []int{0, 40, 80, 129} {
		if bank.Regs[base+2] != 0 {
			t.Errorf("model @%d Ena=%d want 0", base, bank.Regs[base+2])
		}
		if bank.Regs[base+10] != 0 {
			t.Errorf("model @%d MustTrip ActPt=%d want 0", base, bank.Regs[base+10])
		}
	}
}

func TestEmitEnterService_Length(t *testing.T) {
	p := source.ProtectionParams{
		ReconnVLow: 195, ReconnVHi: 259, ReconnFLow: 49.5, ReconnFHi: 50.2,
		ReconnectS: 60,
		Has: map[string]bool{
			"AG": true, "BN": true, "BO": true, "BP": true, "BQ": true,
		},
	}
	bank := Bank{Regs: make([]uint16, 0, 32), Base: BaseRegister}
	emitEnterService(&bank, p, 230)

	if got := len(bank.Regs); got != 19 {
		t.Fatalf("model 703 total regs=%d want 19", got)
	}
	if bank.Regs[0] != 703 {
		t.Errorf("ID=%d want 703", bank.Regs[0])
	}
	if bank.Regs[1] != EnterServiceBodyLen {
		t.Errorf("L=%d want %d", bank.Regs[1], EnterServiceBodyLen)
	}
	// ES enabled
	if bank.Regs[2] != 1 {
		t.Errorf("ES=%d want 1", bank.Regs[2])
	}
	// ESVHi at idx 3 = 259V/230V*10000 ≈ 11261
	if got := bank.Regs[3]; got < 11250 || got > 11270 {
		t.Errorf("ESVHi=%d want ~11261", got)
	}
	// ESVLo at idx 4 = 195V/230V*10000 ≈ 8478
	if got := bank.Regs[4]; got < 8470 || got > 8485 {
		t.Errorf("ESVLo=%d want ~8478", got)
	}
	// ESHzHi at idx 5..6 (uint32) = 5020 (50.2 Hz × 100)
	hi := uint32(bank.Regs[5])<<16 | uint32(bank.Regs[6])
	if hi != 5020 {
		t.Errorf("ESHzHi=%d want 5020", hi)
	}
	// ESHzLo at idx 7..8 = 4950
	lo := uint32(bank.Regs[7])<<16 | uint32(bank.Regs[8])
	if lo != 4950 {
		t.Errorf("ESHzLo=%d want 4950", lo)
	}
	// ESDlyTms at idx 9..10 = 60
	dly := uint32(bank.Regs[9])<<16 | uint32(bank.Regs[10])
	if dly != 60 {
		t.Errorf("ESDlyTms=%d want 60", dly)
	}
}

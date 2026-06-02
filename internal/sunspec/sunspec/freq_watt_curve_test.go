package sunspec

import (
	"testing"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
)

func protWithCurve() source.ProtectionParams {
	return source.ProtectionParams{
		OFCurveUFLow:  47.0, // DH (W=0%)
		OFCurveUFHigh: 49.5, // DI (W=100%)
		OFCurveOFLow:  50.5, // CB (W=100%)
		OFDroopEnd:    52.0, // CC (W=0%)
		OFDroopMode:   13,   // CV — AS/NZS active
		Has: map[string]bool{
			"DH": true, "DI": true, "CB": true, "CC": true, "CV": true,
		},
	}
}

func TestEmitFreqWattCurve_LengthAndContent(t *testing.T) {
	bank := Bank{Regs: make([]uint16, 0, 80), Base: BaseRegister}
	emitFreqWattCurve(&bank, protWithCurve())

	// Total = 2 (ID + L) + body. Body = header(10) + curve(58) = 68.
	wantTotal := 2 + int(FreqWattCurveBodyLen)
	if got := len(bank.Regs); got != wantTotal {
		t.Fatalf("regs=%d want %d", got, wantTotal)
	}
	if bank.Regs[0] != FreqWattCurveModelID {
		t.Errorf("ID=%d want %d", bank.Regs[0], FreqWattCurveModelID)
	}
	if bank.Regs[1] != FreqWattCurveBodyLen {
		t.Errorf("L=%d want %d", bank.Regs[1], FreqWattCurveBodyLen)
	}

	body := bank.Regs[2:]
	// Header
	if body[freqWattCurveBodyActCrv] != 1 {
		t.Errorf("ActCrv=%d want 1", body[freqWattCurveBodyActCrv])
	}
	if body[freqWattCurveBodyModEna] != 1 {
		t.Errorf("ModEna=%d want 1 (active mode + all four codes present)", body[freqWattCurveBodyModEna])
	}
	if body[freqWattCurveBodyNCrv] != freqWattCurveNCrv {
		t.Errorf("NCrv=%d want %d", body[freqWattCurveBodyNCrv], freqWattCurveNCrv)
	}
	if body[freqWattCurveBodyNPt] != freqWattCurveNPtMax {
		t.Errorf("NPt=%d want %d", body[freqWattCurveBodyNPt], freqWattCurveNPtMax)
	}
	if int16(body[freqWattCurveBodyHzSF]) != int16(freqWattCurveHzSF) {
		t.Errorf("Hz_SF=%d want %d", int16(body[freqWattCurveBodyHzSF]), freqWattCurveHzSF)
	}

	// ActPt = 4 active points
	if body[freqWattCurveBodyActPt] != freqWattCurveActPt {
		t.Errorf("ActPt=%d want %d", body[freqWattCurveBodyActPt], freqWattCurveActPt)
	}

	// Hz1 = DH = 47.0 Hz → wire 4700
	if got := body[FreqWattCurveBodyHz1Off]; got != 4700 {
		t.Errorf("Hz1=%d want 4700 (47.0 Hz × 100)", got)
	}
	if got := body[FreqWattCurveBodyW1Off]; got != 0 {
		t.Errorf("W1=%d want 0", got)
	}
	// Hz2 = DI = 49.5 → 4950
	if got := body[FreqWattCurveBodyHz2Off]; got != 4950 {
		t.Errorf("Hz2=%d want 4950", got)
	}
	if got := body[FreqWattCurveBodyW2Off]; got != 100 {
		t.Errorf("W2=%d want 100", got)
	}
	// Hz3 = CB = 50.5 → 5050
	if got := body[FreqWattCurveBodyHz3Off]; got != 5050 {
		t.Errorf("Hz3=%d want 5050", got)
	}
	if got := body[FreqWattCurveBodyW3Off]; got != 100 {
		t.Errorf("W3=%d want 100", got)
	}
	// Hz4 = CC = 52.0 → 5200
	if got := body[FreqWattCurveBodyHz4Off]; got != 5200 {
		t.Errorf("Hz4=%d want 5200", got)
	}
	if got := body[FreqWattCurveBodyW4Off]; got != 0 {
		t.Errorf("W4=%d want 0", got)
	}

	// ReadOnly at body[67] = READWRITE (0)
	if body[freqWattCurveBodyReadOnly] != freqWattCurveReadOnlyRW {
		t.Errorf("ReadOnly=%d want RW(%d)", body[freqWattCurveBodyReadOnly], freqWattCurveReadOnlyRW)
	}

	// WRef defaults to 100
	if body[freqWattCurveBodyWRef] != freqWattCurveDefaultWRef {
		t.Errorf("WRef=%d want %d", body[freqWattCurveBodyWRef], freqWattCurveDefaultWRef)
	}
}

func TestEmitFreqWattCurve_DisabledWhenIncomplete(t *testing.T) {
	// Only DH/DI present — over-freq codes missing. Should still emit
	// with the right length but ActPt=0 / ModEna=0.
	p := source.ProtectionParams{
		OFCurveUFLow:  47.0,
		OFCurveUFHigh: 49.5,
		OFDroopMode:   13,
		Has:           map[string]bool{"DH": true, "DI": true, "CV": true},
	}
	bank := Bank{Regs: make([]uint16, 0, 80), Base: BaseRegister}
	emitFreqWattCurve(&bank, p)
	wantTotal := 2 + int(FreqWattCurveBodyLen)
	if got := len(bank.Regs); got != wantTotal {
		t.Fatalf("regs=%d want %d", got, wantTotal)
	}
	body := bank.Regs[2:]
	if body[freqWattCurveBodyActPt] != 0 {
		t.Errorf("ActPt=%d want 0 when codes missing", body[freqWattCurveBodyActPt])
	}
	if body[freqWattCurveBodyModEna] != 0 {
		t.Errorf("ModEna=%d want 0 when codes missing", body[freqWattCurveBodyModEna])
	}
}

func TestEmitFreqWattCurve_Empty(t *testing.T) {
	bank := Bank{Regs: make([]uint16, 0, 80), Base: BaseRegister}
	emitFreqWattCurve(&bank, source.ProtectionParams{Has: map[string]bool{}})
	if got := len(bank.Regs); got != 2+int(FreqWattCurveBodyLen) {
		t.Fatalf("regs=%d want %d", got, 2+int(FreqWattCurveBodyLen))
	}
}

// Pin the body-length math against the model_134.json layout. If the
// header or per-curve constant changes without updating the L value,
// SunSpec clients walking the bank header chain will skip into the
// wrong place and this test will catch it.
func TestFreqWattCurveBodyLen_MatchesModel134Spec(t *testing.T) {
	// Header: ActCrv, ModEna, WinTms, RvrtTms, RmpTms, NCrv, NPt,
	// Hz_SF, W_SF, RmpIncDec_SF = 10 regs.
	const wantHeader = 10
	// Per-curve: ActPt(1) + 20×(Hz+W)(40) + CrvNam(8) + RmpPT1Tms(1)
	// + RmpDecTmm(1) + RmpIncTmm(1) + RmpRsUp(1) + SnptW(1) + WRef(1)
	// + WRefStrHz(1) + WRefStopHz(1) + ReadOnly(1) = 58 regs.
	const wantPerCurve = 58
	if freqWattCurveHeaderBodyLen != wantHeader {
		t.Errorf("header body len %d want %d", freqWattCurveHeaderBodyLen, wantHeader)
	}
	if freqWattCurvePerCurveLen != wantPerCurve {
		t.Errorf("per-curve len %d want %d", freqWattCurvePerCurveLen, wantPerCurve)
	}
	if FreqWattCurveBodyLen != wantHeader+wantPerCurve*freqWattCurveNCrv {
		t.Errorf("body len %d want %d", FreqWattCurveBodyLen, wantHeader+wantPerCurve*freqWattCurveNCrv)
	}
}

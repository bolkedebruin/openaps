package sunspec

import (
	"testing"

	"github.com/bolke/ecu-sunspec/internal/source"
)

func TestEmitFreqDroop_LengthAndContent(t *testing.T) {
	// DS3-style values: enabled, curtails 50.2 → 52.0 Hz at 16.7 %P/Hz.
	p := source.ProtectionParams{
		OFDroopStart: 50.2,
		OFDroopEnd:   52.0,
		OFDroopSlope: 16.7,
		OFDroopMode:  13, // AS/NZS variant = enabled
		Has: map[string]bool{
			"DC": true, "CC": true, "DD": true, "CV": true,
		},
	}
	bank := Bank{Regs: make([]uint16, 0, 32), Base: BaseRegister}
	emitFreqDroop(&bank, p, freqDroopRsltCompleted, 0)

	// Total = 24 regs (ID + L + 22-reg body).
	if got := len(bank.Regs); got != 24 {
		t.Fatalf("emitFreqDroop regs=%d want 24", got)
	}
	if bank.Regs[0] != FreqDroopModelID {
		t.Errorf("ID=%d want %d", bank.Regs[0], FreqDroopModelID)
	}
	if bank.Regs[1] != freqDroopBodyLen {
		t.Errorf("L=%d want %d", bank.Regs[1], freqDroopBodyLen)
	}
	// Header layout (offsets within body, body starts at index 2):
	//  0 Ena, 1 AdptCtlReq, 2 AdptCtlRslt, 3 NCtl,
	//  4-5 RvrtTms, 6-7 RvrtRem, 8 RvrtCtl,
	//  9 Db_SF, 10 K_SF, 11 RspTms_SF
	hdr := bank.Regs[2:]
	if hdr[0] != 1 {
		t.Errorf("Ena=%d want 1 (mode=13 = enabled)", hdr[0])
	}
	if hdr[2] != freqDroopRsltCompleted {
		t.Errorf("AdptCtlRslt=%d want COMPLETED(%d)", hdr[2], freqDroopRsltCompleted)
	}
	if hdr[3] != freqDroopNCtl {
		t.Errorf("NCtl=%d want 1", hdr[3])
	}
	// Per-Ctl block starts at body offset 12 (header 12 regs).
	ctl := bank.Regs[2+12:]
	// DbOf at offset 0..1 (uint32) = (50.2 - 50.0) * 100 = 20 cHz.
	dbOf := uint32(ctl[0])<<16 | uint32(ctl[1])
	if dbOf != 20 {
		t.Errorf("DbOf=%d want 20 (0.2 Hz × 100)", dbOf)
	}
	// KOf at offset 4 = 16.7 × 10 = 167 (sf=-1).
	if ctl[4] != 167 {
		t.Errorf("KOf=%d want 167 (16.7 %%P/Hz × 10)", ctl[4])
	}
	// ReadOnly at the last position of the Ctl block (offset 9).
	if ctl[9] != freqDroopReadOnlyRW {
		t.Errorf("ReadOnly=%d want RW(%d)", ctl[9], freqDroopReadOnlyRW)
	}
}

func TestEmitFreqDroop_Disabled(t *testing.T) {
	// QS1-style: CV=15 means disabled.
	p := source.ProtectionParams{
		OFDroopMode: 15,
		Has:         map[string]bool{"CV": true},
	}
	bank := Bank{Regs: make([]uint16, 0, 32), Base: BaseRegister}
	emitFreqDroop(&bank, p, freqDroopRsltCompleted, 0)

	if bank.Regs[2] != 0 {
		t.Errorf("Ena=%d want 0 (mode=15 = disabled)", bank.Regs[2])
	}
}

func TestEmitFreqDroop_EmptyParams(t *testing.T) {
	bank := Bank{Regs: make([]uint16, 0, 32), Base: BaseRegister}
	emitFreqDroop(&bank, source.ProtectionParams{Has: map[string]bool{}}, freqDroopRsltCompleted, 0)
	if got := len(bank.Regs); got != 24 {
		t.Fatalf("regs=%d want 24 even with empty params", got)
	}
	// Ena=0, all values default to 0 / not-implemented.
	if bank.Regs[2] != 0 {
		t.Errorf("Ena=%d want 0", bank.Regs[2])
	}
}

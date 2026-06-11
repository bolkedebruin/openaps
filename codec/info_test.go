package codec

import (
	"errors"
	"testing"
)

func TestDecodeInfoReply_FormA(t *testing.T) {
	t.Parallel()
	// FB FB 09 DC <model=0x36> <hi=0x0002> 00 <lo=0x01F4=500> 00 00 00 00 FE FE
	// software_version = 2*1000 + 500 = 2500
	l2 := hx(t, "FBFB09DC36 0002 00 01F4 00 00 00 00 FEFE")
	got, err := DecodeInfoReply(l2)
	if err != nil {
		t.Fatalf("DecodeInfoReply: %v", err)
	}
	if got.Model != 0x36 {
		t.Errorf("Model: got 0x%X want 0x36", got.Model)
	}
	if got.SoftwareVersion != 2500 {
		t.Errorf("SoftwareVersion: got %d want 2500", got.SoftwareVersion)
	}
}

func TestDecodeInfoReply_FormB(t *testing.T) {
	t.Parallel()
	// FB FB 0A DD DC <model=0x29> <hi=0x0003> 00 <lo=0x002A=42> 00 00 00 00 00 FE FE
	// software_version = 3*1000 + 42 = 3042
	l2 := hx(t, "FBFB0ADDDC29 0003 00 002A 00 00 00 00 FEFE")
	got, err := DecodeInfoReply(l2)
	if err != nil {
		t.Fatalf("DecodeInfoReply: %v", err)
	}
	if got.Model != 0x29 {
		t.Errorf("Model: got 0x%X want 0x29", got.Model)
	}
	if got.SoftwareVersion != 3042 {
		t.Errorf("SoftwareVersion: got %d want 3042", got.SoftwareVersion)
	}
}

func TestDecodeInfoReply_FormC(t *testing.T) {
	t.Parallel()
	// FB FB 0C DD DC <model=0x30> <hi=0x0002> 00 <mid=0x0003> 00 00 <lo=0x0004> 00 00 FE FE
	// software_version = 2*1_000_000 + 3*1_000 + 4 = 2_003_004
	l2 := hx(t, "FBFB0CDDDC30 0002 00 0003 00 00 0004 00 00 FEFE")
	got, err := DecodeInfoReply(l2)
	if err != nil {
		t.Fatalf("DecodeInfoReply: %v", err)
	}
	if got.Model != 0x30 {
		t.Errorf("Model: got 0x%X want 0x30", got.Model)
	}
	if got.SoftwareVersion != 2_003_004 {
		t.Errorf("SoftwareVersion: got %d want 2003004", got.SoftwareVersion)
	}
}

func TestDecodeInfoReply_UnknownLength(t *testing.T) {
	t.Parallel()
	l2 := make([]byte, 18) // not 16/17/19
	_, err := DecodeInfoReply(l2)
	if !errors.Is(err, ErrUnknownReplyShape) {
		t.Fatalf("err: got %v want ErrUnknownReplyShape", err)
	}
}

func TestDecodeInfoReply_BadSOF(t *testing.T) {
	t.Parallel()
	l2 := hx(t, "0000 09 DC 36 0002 00 01F4 00 00 00 00 FEFE")
	if _, err := DecodeInfoReply(l2); !errors.Is(err, ErrBadL2SOF) {
		t.Fatalf("err: got %v want ErrBadL2SOF", err)
	}
}

func TestDecodeInfoReply_BadEOF(t *testing.T) {
	t.Parallel()
	// 16 bytes, FB FB 09 DC but trailing bytes wrong.
	l2 := hx(t, "FBFB09DC36 0002 00 01F4 00 00 00 00 0000")
	if _, err := DecodeInfoReply(l2); !errors.Is(err, ErrBadL2EOF) {
		t.Fatalf("err: got %v want ErrBadL2EOF", err)
	}
}

func TestDecodeInfoReply_BadCmdBytes(t *testing.T) {
	t.Parallel()
	// 17 bytes but the inner cmd triple is wrong.
	l2 := hx(t, "FBFB0ADDDD29 0003 00 002A 00 00 00 00 FEFE")
	_, err := DecodeInfoReply(l2)
	if err == nil {
		t.Fatalf("expected error for bad form-B cmd bytes")
	}
}

func TestPhaseFromModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		model uint8
		want  uint32
	}{
		{7, 1},
		{8, 1},
		{0x20, 1},
		{0x21, 1},
		{0x22, 1},
		{0x36, 1},
		{0x29, 3},
		{0x30, 3},
		{0x31, 3},
		{0x32, 3},
		{0, 0},
		{0x99, 0},
		{0x36 + 1, 0},
	}
	for _, c := range cases {
		if got := PhaseFromModel(c.model); got != c.want {
			t.Errorf("PhaseFromModel(0x%X) = %d, want %d", c.model, got, c.want)
		}
	}
}

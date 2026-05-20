package codec

import (
	"bytes"
	"errors"
	"testing"
)

func TestBuildL2Frame_InfoQuery(t *testing.T) {
	t.Parallel()
	got := BuildL2Frame(0x06, 0xDC, []byte{0, 0, 0, 0, 0})
	want := hx(t, "FB FB 06 DC 00 00 00 00 00 00 E2 FE FE")
	if !bytes.Equal(got, want) {
		t.Fatalf("BuildL2Frame mismatch:\n  got  % X\n  want % X", got, want)
	}
}

func TestBuildL2Frame_ChecksumWraps16Bit(t *testing.T) {
	t.Parallel()
	got := BuildL2Frame(0xFF, 0xFF, []byte{0xFF, 0xFF})
	want := hx(t, "FB FB FF FF FF FF 03 FC FE FE")
	if !bytes.Equal(got, want) {
		t.Fatalf("16-bit sum mismatch:\n  got  % X\n  want % X", got, want)
	}
}

func TestEncodeSetPowerDSPNew_QS1A_400W(t *testing.T) {
	t.Parallel()
	// (400 * 7395) >> 8 = 11554 = 0x2D22
	// sum = 06+1C+8C+02+2D+22+00 = 0xFF
	got, err := EncodeSetPowerDSPNew(400, false)
	if err != nil {
		t.Fatalf("EncodeSetPowerDSPNew: %v", err)
	}
	want := hx(t, "FB FB 06 1C 8C 02 2D 22 00 00 FF FE FE")
	if !bytes.Equal(got, want) {
		t.Fatalf("QS1A per-inv 400W mismatch:\n  got  % X\n  want % X", got, want)
	}
}

func TestEncodeSetPowerDSPNew_Broadcast_400W(t *testing.T) {
	t.Parallel()
	// cmd 0x2C: sum = 0xFF + 0x10 = 0x10F
	got, err := EncodeSetPowerDSPNew(400, true)
	if err != nil {
		t.Fatalf("EncodeSetPowerDSPNew: %v", err)
	}
	want := hx(t, "FB FB 06 2C 8C 02 2D 22 00 01 0F FE FE")
	if !bytes.Equal(got, want) {
		t.Fatalf("QS1A broadcast 400W mismatch:\n  got  % X\n  want % X", got, want)
	}
}

func TestEncodeSetPowerDSPNew_Boundaries(t *testing.T) {
	t.Parallel()
	if _, err := EncodeSetPowerDSPNew(20, false); err != nil {
		t.Fatalf("20W (min) should be accepted, got %v", err)
	}
	if _, err := EncodeSetPowerDSPNew(500, false); err != nil {
		t.Fatalf("500W (max) should be accepted, got %v", err)
	}
}

func TestEncodeSetPowerDSPNew_RejectsOutOfRange(t *testing.T) {
	t.Parallel()
	if _, err := EncodeSetPowerDSPNew(10, false); !errors.Is(err, ErrPanelWattsOutOfRange) {
		t.Fatalf("10W: got %v want ErrPanelWattsOutOfRange", err)
	}
	if _, err := EncodeSetPowerDSPNew(501, false); !errors.Is(err, ErrPanelWattsOutOfRange) {
		t.Fatalf("501W: got %v want ErrPanelWattsOutOfRange", err)
	}
}

func TestEncodeSetPowerDS3_PerInverter_300W(t *testing.T) {
	t.Parallel()
	// round(300 / 0.06027) = round(4977.5946) = 4978 = 0x1372
	// sum = 06+AA+27+00+00+13+72 = 0x15C
	got, err := EncodeSetPowerDS3(300, false)
	if err != nil {
		t.Fatalf("EncodeSetPowerDS3: %v", err)
	}
	want := hx(t, "FB FB 06 AA 27 00 00 13 72 01 5C FE FE")
	if !bytes.Equal(got, want) {
		t.Fatalf("DS3 per-inv 300W mismatch:\n  got  % X\n  want % X", got, want)
	}
}

func TestEncodeSetPowerDS3_Broadcast_300W(t *testing.T) {
	t.Parallel()
	// cmd 0xAB: sum = 0x15C + 1 = 0x15D
	got, err := EncodeSetPowerDS3(300, true)
	if err != nil {
		t.Fatalf("EncodeSetPowerDS3: %v", err)
	}
	want := hx(t, "FB FB 06 AB 27 00 00 13 72 01 5D FE FE")
	if !bytes.Equal(got, want) {
		t.Fatalf("DS3 broadcast 300W mismatch:\n  got  % X\n  want % X", got, want)
	}
}

func TestEncodeSetPowerDS3_PerInverter_100W(t *testing.T) {
	t.Parallel()
	// round(100 / 0.06027) = round(1659.1969) = 1659 = 0x067B
	// sum = 06+AA+27+00+00+06+7B = 0x158
	got, err := EncodeSetPowerDS3(100, false)
	if err != nil {
		t.Fatalf("EncodeSetPowerDS3: %v", err)
	}
	want := hx(t, "FB FB 06 AA 27 00 00 06 7B 01 58 FE FE")
	if !bytes.Equal(got, want) {
		t.Fatalf("DS3 per-inv 100W mismatch:\n  got  % X\n  want % X", got, want)
	}
}

func TestEncodeSetPowerDS3_Minimum_20W(t *testing.T) {
	t.Parallel()
	// round(20 / 0.06027) = round(331.8414) = 332 = 0x014C
	got, err := EncodeSetPowerDS3(20, false)
	if err != nil {
		t.Fatalf("EncodeSetPowerDS3: %v", err)
	}
	if got[7] != 0x01 || got[8] != 0x4C {
		t.Fatalf("20W value bytes: got %02X %02X want 01 4C", got[7], got[8])
	}
}

func TestEncodeSetPowerDS3_Boundaries(t *testing.T) {
	t.Parallel()
	if _, err := EncodeSetPowerDS3(20, false); err != nil {
		t.Fatalf("20W (min) should be accepted, got %v", err)
	}
	if _, err := EncodeSetPowerDS3(500, false); err != nil {
		t.Fatalf("500W (max) should be accepted, got %v", err)
	}
}

func TestEncodeSetPowerDS3_RejectsOutOfRange(t *testing.T) {
	t.Parallel()
	if _, err := EncodeSetPowerDS3(10, false); !errors.Is(err, ErrPanelWattsOutOfRange) {
		t.Fatalf("10W: got %v want ErrPanelWattsOutOfRange", err)
	}
	if _, err := EncodeSetPowerDS3(501, false); !errors.Is(err, ErrPanelWattsOutOfRange) {
		t.Fatalf("501W: got %v want ErrPanelWattsOutOfRange", err)
	}
}

func TestEncodeSetPowerC3_Model6(t *testing.T) {
	t.Parallel()
	// (300 * 7395) >> 14 = 2218500 / 16384 = 135 = 0x87
	got, err := EncodeSetPowerC3(0x06, 300, false)
	if err != nil {
		t.Fatalf("EncodeSetPowerC3: %v", err)
	}
	if got[4] != 0x87 {
		t.Fatalf("model 6 300W value byte: got 0x%02X want 0x87", got[4])
	}
}

func TestEncodeSetPowerC3_Model7HasExtraShift(t *testing.T) {
	t.Parallel()
	// (300 * 7395) >> 16 = 33
	got, err := EncodeSetPowerC3(0x07, 300, false)
	if err != nil {
		t.Fatalf("EncodeSetPowerC3: %v", err)
	}
	if got[4] != 33 {
		t.Fatalf("model 7 300W value byte: got 0x%02X want 0x21", got[4])
	}
}

func TestEncodeSetPowerC3_Broadcast(t *testing.T) {
	t.Parallel()
	got, err := EncodeSetPowerC3(0x06, 300, true)
	if err != nil {
		t.Fatalf("EncodeSetPowerC3: %v", err)
	}
	if got[3] != 0xA3 {
		t.Fatalf("C3 broadcast cmd: got 0x%02X want 0xA3", got[3])
	}
}

func TestEncodeSetPowerC3_RejectsOutOfRange(t *testing.T) {
	t.Parallel()
	if _, err := EncodeSetPowerC3(0x06, 10, false); !errors.Is(err, ErrPanelWattsOutOfRange) {
		t.Fatalf("10W: got %v want ErrPanelWattsOutOfRange", err)
	}
	if _, err := EncodeSetPowerC3(0x06, 501, false); !errors.Is(err, ErrPanelWattsOutOfRange) {
		t.Fatalf("501W: got %v want ErrPanelWattsOutOfRange", err)
	}
}

func TestEncodeSetPower_FamilyDispatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		model      uint8
		watts      uint16
		broadcast  bool
		wantPrefix []byte
	}{
		{"QS1 per-inv", 0x08, 400, false, []byte{0xFB, 0xFB, 0x06, 0x1C, 0x8C}},
		{"QS1A per-inv", 0x18, 400, false, []byte{0xFB, 0xFB, 0x06, 0x1C, 0x8C}},
		{"QS1A broadcast", 0x18, 400, true, []byte{0xFB, 0xFB, 0x06, 0x2C, 0x8C}},
		{"DS3 model 0x20 per-inv", 0x20, 300, false, []byte{0xFB, 0xFB, 0x06, 0xAA, 0x27}},
		{"DS3 model 0x21 per-inv", 0x21, 300, false, []byte{0xFB, 0xFB, 0x06, 0xAA, 0x27}},
		{"DS3 model 0x22 per-inv", 0x22, 300, false, []byte{0xFB, 0xFB, 0x06, 0xAA, 0x27}},
		{"DS3 model 0x36 per-inv", 0x36, 300, false, []byte{0xFB, 0xFB, 0x06, 0xAA, 0x27}},
		{"DS3 broadcast", 0x20, 300, true, []byte{0xFB, 0xFB, 0x06, 0xAB, 0x27}},
		{"YC600 per-inv", 0x06, 200, false, []byte{0xFB, 0xFB, 0x06, 0xC3}},
		{"YC600 broadcast", 0x05, 200, true, []byte{0xFB, 0xFB, 0x06, 0xA3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncodeSetPower(tc.model, tc.watts, tc.broadcast)
			if err != nil {
				t.Fatalf("EncodeSetPower: %v", err)
			}
			if !bytes.HasPrefix(got, tc.wantPrefix) {
				t.Fatalf("prefix mismatch:\n  got  % X\n  want prefix % X", got, tc.wantPrefix)
			}
		})
	}
}

func TestEncodeSetPower_DS3FamilyAllProduceSameBytes(t *testing.T) {
	t.Parallel()
	want, err := EncodeSetPower(0x20, 300, false)
	if err != nil {
		t.Fatalf("EncodeSetPower: %v", err)
	}
	for _, mc := range []uint8{0x21, 0x22, 0x36} {
		got, err := EncodeSetPower(mc, 300, false)
		if err != nil {
			t.Fatalf("EncodeSetPower(0x%02X): %v", mc, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("model 0x%02X bytes differ from 0x20:\n  got  % X\n  want % X", mc, got, want)
		}
	}
}

func TestEncodeSetPower_UnsupportedFamily(t *testing.T) {
	t.Parallel()
	for _, mc := range []uint8{0x29, 0x30, 0x31, 0x32} {
		if _, err := EncodeSetPower(mc, 300, false); !errors.Is(err, ErrUnsupportedFamily) {
			t.Fatalf("model 0x%02X: err = %v, want ErrUnsupportedFamily", mc, err)
		}
	}
}

func TestEncodeSetPower_PanelWattsOutOfRange(t *testing.T) {
	t.Parallel()
	for _, mc := range []uint8{0x06, 0x18, 0x20} {
		if _, err := EncodeSetPower(mc, 10, false); !errors.Is(err, ErrPanelWattsOutOfRange) {
			t.Fatalf("model 0x%02X 10W: err = %v, want ErrPanelWattsOutOfRange", mc, err)
		}
		if _, err := EncodeSetPower(mc, 501, false); !errors.Is(err, ErrPanelWattsOutOfRange) {
			t.Fatalf("model 0x%02X 501W: err = %v, want ErrPanelWattsOutOfRange", mc, err)
		}
	}
}

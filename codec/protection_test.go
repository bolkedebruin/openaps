package codec

import (
	"bytes"
	"errors"
	"testing"
)

// TestEncodeSetProtection_SingleFrame asserts the exact frames for the
// single-frame params (decoded from set_paraName_paraValue_inverter
// @ 0x69bdc; DS3 AG / QS1 AG match the decompile's worked examples).
func TestEncodeSetProtection_SingleFrame(t *testing.T) {
	cases := []struct {
		name      string
		modelCode uint8
		param     string
		value     float64
		want      []byte
	}{
		{"DS3 CB 50.3", ModelDS3, "Over_frequency_Watt_Low_set", 50.3,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x2B, 0x00, 0x00, 0x13, 0xA6, 0x01, 0x94, 0xFE, 0xFE}},
		{"DS3 CC 51.0", ModelDS3, "Over_frequency_Watt_High_set", 51.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x2A, 0x00, 0x00, 0x13, 0xEC, 0x01, 0xD9, 0xFE, 0xFE}},
		{"QS1A CB 50.3", ModelQS1A, "Over_frequency_Watt_Low_set", 50.3,
			[]byte{0xFB, 0xFB, 0x06, 0x1C, 0x68, 0x03, 0x0F, 0x2A, 0xF3, 0x01, 0xB9, 0xFE, 0xFE}},
		{"QS1A CC 51.0", ModelQS1A, "Over_frequency_Watt_High_set", 51.0,
			[]byte{0xFB, 0xFB, 0x06, 0x1C, 0x65, 0x03, 0x0E, 0xF5, 0xA8, 0x02, 0x35, 0xFE, 0xFE}},
		{"DS3 CF slope 40", ModelDS3, "Over_Frequency_Watt_Slope_set", 40.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x2D, 0x00, 0x00, 0x19, 0x93, 0x01, 0x89, 0xFE, 0xFE}},
		{"DS3 CG delay 5", ModelDS3, "Over_frequency_Watt_Delay_Time_set", 5.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x2E, 0x00, 0x00, 0x00, 0x05, 0x00, 0xE3, 0xFE, 0xFE}},
		// CV mode enum on DS3 (sub 0x28, single frame): 15→0.
		{"DS3 CV disabled", ModelDS3, "Over_frequency_Watt_set", 15,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x28, 0x00, 0x00, 0x00, 0x00, 0x00, 0xD8, 0xFE, 0xFE}},
		// grid_recovery: DS3 int(s*50) sub 0x25; QS1 int(s*100) via 0x5D.
		{"DS3 AG 60s", ModelDS3, "grid_recovery_time", 60,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x25, 0x00, 0x00, 0x0B, 0xB8, 0x01, 0x98, 0xFE, 0xFE}},
		{"QS1A AG 60s", ModelQS1A, "grid_recovery_time", 60,
			[]byte{0xFB, 0xFB, 0x06, 0x5D, 0x17, 0x70, 0x00, 0x00, 0x00, 0x00, 0xEA, 0xFE, 0xFE}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncodeSetProtection(tc.modelCode, tc.param, tc.value)
			if err != nil {
				t.Fatalf("EncodeSetProtection: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d frames, want 1", len(got))
			}
			if !bytes.Equal(got[0], tc.want) {
				t.Errorf("frame=% X want % X", got[0], tc.want)
			}
		})
	}
}

// TestEncodeSetProtection_QS1ModeMultiFrame pins the QS1 over-frequency
// mode enum 2-3 frame sequence (sub 0x5F + 0x99, with 0xD→4 / 0xE→1).
func TestEncodeSetProtection_QS1ModeMultiFrame(t *testing.T) {
	// 15 (disabled) → three frames, fully checked.
	want15 := [][]byte{
		{0xFB, 0xFB, 0x06, 0x1C, 0x5F, 0x01, 0x30, 0x00, 0x00, 0x00, 0xB2, 0xFE, 0xFE},
		{0xFB, 0xFB, 0x06, 0x1C, 0x5F, 0x01, 0x0F, 0x00, 0x00, 0x00, 0x91, 0xFE, 0xFE},
		{0xFB, 0xFB, 0x06, 0x1C, 0x99, 0x01, 0x00, 0x00, 0x00, 0x00, 0xBC, 0xFE, 0xFE},
	}
	got, err := EncodeSetProtection(ModelQS1A, "Over_frequency_Watt_set", 15)
	if err != nil {
		t.Fatalf("CV 15: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("CV 15: %d frames, want 3", len(got))
	}
	for i := range want15 {
		if !bytes.Equal(got[i], want15[i]) {
			t.Errorf("CV 15 frame %d = % X want % X", i, got[i], want15[i])
		}
	}

	// 13 (AS/NZS): 0x5F=0x10, 0x5F=4 (13→4), 0x99=1 — structural check.
	got13, _ := EncodeSetProtection(ModelQS1A, "Over_frequency_Watt_set", 13)
	wantSubVal := []struct{ sub, val byte }{{0x5F, 0x10}, {0x5F, 0x04}, {0x99, 0x01}}
	if len(got13) != 3 {
		t.Fatalf("CV 13: %d frames, want 3", len(got13))
	}
	for i, sv := range wantSubVal {
		if got13[i][3] != CmdSetPowerQS1Unicast || got13[i][4] != sv.sub || got13[i][6] != sv.val {
			t.Errorf("CV 13 frame %d: cmd/sub/val = %#x/%#x/%#x want 1C/%#x/%#x", i, got13[i][3], got13[i][4], got13[i][6], sv.sub, sv.val)
		}
	}

	// 14 (other): two frames, 0x5F=0x20, 0x5F=1 (14→1).
	got14, _ := EncodeSetProtection(ModelQS1A, "Over_frequency_Watt_set", 14)
	if len(got14) != 2 {
		t.Fatalf("CV 14: %d frames, want 2", len(got14))
	}
	if got14[0][4] != 0x5F || got14[0][6] != 0x20 || got14[1][4] != 0x5F || got14[1][6] != 0x01 {
		t.Errorf("CV 14 frames = % X / % X", got14[0], got14[1])
	}
}

// TestEncodeSetProtection_Structure checks DH/DI/CA correct opcode,
// sub-byte, byte_count, embedded value, and self-consistent checksum.
func TestEncodeSetProtection_Structure(t *testing.T) {
	checkSum := func(t *testing.T, f []byte) {
		var s uint16
		for _, b := range f[2:9] {
			s += uint16(b)
		}
		if got := uint16(f[9])<<8 | uint16(f[10]); got != s {
			t.Errorf("checksum %#04x != sum(bytes[2..8]) %#04x", got, s)
		}
	}
	t.Run("DS3 DH/DI int(Hz*100) bc2", func(t *testing.T) {
		for param, sub := range map[string]byte{
			"Under_Frequency_Watt_Low_set":  0x56,
			"Under_Frequency_Watt_High_set": 0x55,
		} {
			got, err := EncodeSetProtection(ModelDS3, param, 47.0)
			if err != nil {
				t.Fatalf("%s: %v", param, err)
			}
			f := got[0]
			if f[3] != CmdSetPowerDS3Unicast || f[4] != sub {
				t.Errorf("%s: opcode/sub = %#x/%#x", param, f[3], f[4])
			}
			if wire := uint16(f[7])<<8 | uint16(f[8]); wire != 4700 {
				t.Errorf("%s: wire=%d want 4700", param, wire)
			}
			checkSum(t, f)
		}
	})
	t.Run("QS1A CA/DH/DI int(50e6/Hz) bc3", func(t *testing.T) {
		hz := 47.0
		want := uint32(int64(50000000.0 / hz))
		for param, sub := range map[string]byte{
			"Over_frequency_Watt_Start_set": 0x62,
			"Under_Frequency_Watt_Low_set":  0xA1,
			"Under_Frequency_Watt_High_set": 0xA4,
		} {
			got, err := EncodeSetProtection(ModelQS1A, param, hz)
			if err != nil {
				t.Fatalf("%s: %v", param, err)
			}
			f := got[0]
			if f[3] != CmdSetPowerQS1Unicast || f[4] != sub || f[5] != 0x03 {
				t.Errorf("%s: opcode/sub/bc = %#x/%#x/%#x", param, f[3], f[4], f[5])
			}
			if wire := uint32(f[6])<<16 | uint32(f[7])<<8 | uint32(f[8]); wire != want {
				t.Errorf("%s: wire=%d want %d", param, wire, want)
			}
			checkSum(t, f)
		}
	})
}

func TestEncodeSetProtection_Unsupported(t *testing.T) {
	// DS3: CA has no branch; recover-high (DC) not encoded.
	for _, p := range []string{"Over_frequency_Watt_Start_set", "Over_frequency_Watt_recover_High_set"} {
		if _, err := EncodeSetProtection(ModelDS3, p, 50.2); !errors.Is(err, ErrUnsupportedProtectionParam) {
			t.Errorf("DS3 %s: want ErrUnsupportedProtectionParam, got %v", p, err)
		}
	}
	// QS1A: slope/delay/recover are firmware-rejected (routeFor Unsupported).
	for _, p := range []string{
		"Over_Frequency_Watt_Slope_set",
		"Over_frequency_Watt_Delay_Time_set",
		"Over_frequency_Watt_recover_High_set",
	} {
		if _, err := EncodeSetProtection(ModelQS1A, p, 50); !errors.Is(err, ErrUnsupportedProtectionParam) {
			t.Errorf("QS1A %s: want ErrUnsupportedProtectionParam, got %v", p, err)
		}
	}
	if _, err := EncodeSetProtection(ModelYC600, "Over_frequency_Watt_Low_set", 50.3); !errors.Is(err, ErrUnsupportedProtectionFamily) {
		t.Errorf("YC600: want ErrUnsupportedProtectionFamily, got %v", err)
	}
	if _, err := EncodeSetProtection(ModelQS1A, "Over_frequency_Watt_Low_set", 0); err == nil {
		t.Error("value=0: want error, got nil")
	}
}

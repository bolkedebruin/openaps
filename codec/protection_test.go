package codec

import (
	"bytes"
	"errors"
	"testing"
)

// TestEncodeSetProtection_WorkedFrames asserts the exact frames decoded
// from main.exe (set_paraName_paraValue_inverter @ 0x69bdc).
func TestEncodeSetProtection_WorkedFrames(t *testing.T) {
	cases := []struct {
		name      string
		modelCode uint8
		param     string
		hz        float64
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncodeSetProtection(tc.modelCode, tc.param, tc.hz)
			if err != nil {
				t.Fatalf("EncodeSetProtection: %v", err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Errorf("frame=% X want % X", got, tc.want)
			}
		})
	}
}

// TestEncodeSetProtection_Structure checks the remaining freq params
// (DH/DI/CA) for correct opcode, sub-byte, byte_count, embedded value,
// and a self-consistent checksum — without hand-computing the sum.
func TestEncodeSetProtection_Structure(t *testing.T) {
	checkSum := func(t *testing.T, f []byte) {
		// inner-length 0x06 → checksum covers bytes [2..8], stored hi@9 lo@10.
		var s uint16
		for _, b := range f[2:9] {
			s += uint16(b)
		}
		got := uint16(f[9])<<8 | uint16(f[10])
		if got != s {
			t.Errorf("checksum %#04x != sum(bytes[2..8]) %#04x", got, s)
		}
	}

	t.Run("DS3 DH/DI int(Hz*100) bc2", func(t *testing.T) {
		for param, sub := range map[string]byte{
			"Under_Frequency_Watt_Low_set":  0x56,
			"Under_Frequency_Watt_High_set": 0x55,
		} {
			f, err := EncodeSetProtection(ModelDS3, param, 47.0)
			if err != nil {
				t.Fatalf("%s: %v", param, err)
			}
			if f[3] != CmdSetPowerDS3Unicast || f[4] != sub {
				t.Errorf("%s: opcode/sub = %#x/%#x want %#x/%#x", param, f[3], f[4], CmdSetPowerDS3Unicast, sub)
			}
			if f[5] != 0 || f[6] != 0 { // bc2: value right-aligned at [7..8]
				t.Errorf("%s: bytes[5..6] should be zero, got % X", param, f[5:7])
			}
			if wire := uint16(f[7])<<8 | uint16(f[8]); wire != 4700 {
				t.Errorf("%s: wire=%d want 4700 (int(47.0*100))", param, wire)
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
			f, err := EncodeSetProtection(ModelQS1A, param, hz)
			if err != nil {
				t.Fatalf("%s: %v", param, err)
			}
			if f[3] != CmdSetPowerQS1Unicast || f[4] != sub || f[5] != 0x03 {
				t.Errorf("%s: opcode/sub/bc = %#x/%#x/%#x want %#x/%#x/3", param, f[3], f[4], f[5], CmdSetPowerQS1Unicast, sub)
			}
			wire := uint32(f[6])<<16 | uint32(f[7])<<8 | uint32(f[8])
			if wire != want {
				t.Errorf("%s: wire=%d want %d", param, wire, want)
			}
			checkSum(t, f)
		}
	})
}

func TestEncodeSetProtection_Unsupported(t *testing.T) {
	// CA has no DS3 branch in main.exe.
	if _, err := EncodeSetProtection(ModelDS3, "Over_frequency_Watt_Start_set", 50.2); !errors.Is(err, ErrUnsupportedProtectionParam) {
		t.Errorf("DS3 CA: want ErrUnsupportedProtectionParam, got %v", err)
	}
	// Env-gated / multi-frame params are deliberately not encoded here.
	for _, p := range []string{
		"Over_frequency_Watt_set",
		"Over_Frequency_Watt_Slope_set",
		"Over_frequency_Watt_Delay_Time_set",
		"grid_recovery_time",
	} {
		if _, err := EncodeSetProtection(ModelQS1A, p, 60); !errors.Is(err, ErrUnsupportedProtectionParam) {
			t.Errorf("%s: want ErrUnsupportedProtectionParam, got %v", p, err)
		}
	}
	// Unknown family.
	if _, err := EncodeSetProtection(ModelYC600, "Over_frequency_Watt_Low_set", 50.3); !errors.Is(err, ErrUnsupportedProtectionFamily) {
		t.Errorf("YC600: want ErrUnsupportedProtectionFamily, got %v", err)
	}
	// Non-positive frequency.
	if _, err := EncodeSetProtection(ModelQS1A, "Over_frequency_Watt_Low_set", 0); err == nil {
		t.Error("hz=0: want error, got nil")
	}
}

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

// TestEncodeSetProtectionBroadcast_OpcodeSwap asserts that the broadcast encoder
// produces the same L2 body as the unicast encoder with only byte[3] changed to
// the broadcast opcode (DS3 0xAB / QS1A 0x2C) and a recomputed checksum.
func TestEncodeSetProtectionBroadcast_OpcodeSwap(t *testing.T) {
	cases := []struct {
		name      string
		modelCode uint8
		param     string
		value     float64
		wantOp    byte // expected broadcast opcode
	}{
		// DS3 freq threshold — must-have per build plan
		{"DS3 CB 50.3", ModelDS3, "Over_frequency_Watt_Low_set", 50.3, CmdSetPowerDS3Broadcast},
		{"DS3 AK 51.5", ModelDS3, "Over_frequency_Watt_High_set", 51.5, CmdSetPowerDS3Broadcast},
		{"DS3 CV mode 15", ModelDS3, "Over_frequency_Watt_set", 15, CmdSetPowerDS3Broadcast},
		{"DS3 AG 60s", ModelDS3, "grid_recovery_time", 60, CmdSetPowerDS3Broadcast},
		// QS1A freq-watt code — must-have per build plan
		{"QS1A CB 50.3", ModelQS1A, "Over_frequency_Watt_Low_set", 50.3, CmdSetPowerQS1Broadcast},
		{"QS1A DH 47.0", ModelQS1A, "Under_Frequency_Watt_Low_set", 47.0, CmdSetPowerQS1Broadcast},
	}

	checkSum := func(t *testing.T, f []byte) {
		t.Helper()
		var s uint16
		for _, b := range f[2:9] {
			s += uint16(b)
		}
		if got := uint16(f[9])<<8 | uint16(f[10]); got != s {
			t.Errorf("checksum 0x%04X != sum(bytes[2..8]) 0x%04X", got, s)
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uniFrames, err := EncodeSetProtection(tc.modelCode, tc.param, tc.value)
			if err != nil {
				t.Fatalf("unicast: %v", err)
			}
			bcastFrames, err := EncodeSetProtectionBroadcast(tc.modelCode, tc.param, tc.value)
			if err != nil {
				t.Fatalf("broadcast: %v", err)
			}
			if len(uniFrames) != len(bcastFrames) {
				t.Fatalf("frame count mismatch: unicast=%d broadcast=%d", len(uniFrames), len(bcastFrames))
			}

			for i := range uniFrames {
				u, b := uniFrames[i], bcastFrames[i]
				if len(u) != len(b) {
					t.Errorf("frame %d length mismatch: unicast=%d broadcast=%d", i, len(u), len(b))
					continue
				}
				// Opcode at byte[3] must be the broadcast opcode.
				if b[3] != tc.wantOp {
					t.Errorf("frame %d: broadcast opcode = 0x%02X, want 0x%02X", i, b[3], tc.wantOp)
				}
				// All bytes except [3] (opcode) and [9..10] (checksum) must
				// match the unicast frame.
				for j, bv := range b {
					if j == 3 || j == 9 || j == 10 {
						continue
					}
					if bv != u[j] {
						t.Errorf("frame %d byte[%d]: broadcast=0x%02X unicast=0x%02X (body diverges)", i, j, bv, u[j])
					}
				}
				checkSum(t, b)
			}
		})
	}
}

// TestEncodeSetProtectionBroadcast_GroundTruth cross-checks the DS3 and QS1A
// broadcast frames against the worked bytes from GRID_PROFILE_BROADCAST_GROUNDTRUTH.md.
// DS3 set-power (0xAB 0x27) is confirmed on-wire; protection frames share the
// same opcode + checksum formula, so this confirms the format.
func TestEncodeSetProtectionBroadcast_GroundTruth(t *testing.T) {
	// DS3 CB 50.3 Hz: same body as unicast (0xAA 0x2B 0x00 0x00 0x13 0xA6)
	// with byte[3]=0xAB and recomputed checksum.
	// unicast = FB FB 06 AA 2B 00 00 13 A6 01 94 FE FE
	// broadcast: sum(06+AB+2B+00+00+13+A6) = 06+AB+2B+00+00+13+A6 = 0x1A5
	ds3, err := EncodeSetProtectionBroadcast(ModelDS3, "Over_frequency_Watt_Low_set", 50.3)
	if err != nil {
		t.Fatalf("DS3 CB broadcast: %v", err)
	}
	if len(ds3) != 1 {
		t.Fatalf("DS3 CB broadcast: want 1 frame, got %d", len(ds3))
	}
	f := ds3[0]
	if f[3] != CmdSetPowerDS3Broadcast {
		t.Errorf("DS3 CB: opcode = 0x%02X, want 0x%02X", f[3], CmdSetPowerDS3Broadcast)
	}
	// sub-byte must match unicast (0x2B = CB)
	if f[4] != 0x2B {
		t.Errorf("DS3 CB: sub = 0x%02X, want 0x2B", f[4])
	}
	// value bytes [7..8] = 0x13 0xA6 (5030 = int(50.3*100))
	if f[7] != 0x13 || f[8] != 0xA6 {
		t.Errorf("DS3 CB: value bytes = 0x%02X 0x%02X, want 0x13 0xA6", f[7], f[8])
	}

	// QS1A DH 47.0 Hz: sub 0xA1, wire = int(50e6/47) = 1063829 = 0x103715
	qs1, err := EncodeSetProtectionBroadcast(ModelQS1A, "Under_Frequency_Watt_Low_set", 47.0)
	if err != nil {
		t.Fatalf("QS1A DH broadcast: %v", err)
	}
	if len(qs1) != 1 {
		t.Fatalf("QS1A DH broadcast: want 1 frame, got %d", len(qs1))
	}
	g := qs1[0]
	if g[3] != CmdSetPowerQS1Broadcast {
		t.Errorf("QS1A DH: opcode = 0x%02X, want 0x%02X", g[3], CmdSetPowerQS1Broadcast)
	}
	if g[4] != 0xA1 {
		t.Errorf("QS1A DH: sub = 0x%02X, want 0xA1", g[4])
	}
	// byte_count at [5] = 3 (24-bit value)
	if g[5] != 3 {
		t.Errorf("QS1A DH: byte_count = %d, want 3", g[5])
	}
	hz47 := 47.0
	wantWire := uint32(int64(50_000_000.0 / hz47))
	gotWire := uint32(g[6])<<16 | uint32(g[7])<<8 | uint32(g[8])
	if gotWire != wantWire {
		t.Errorf("QS1A DH: wire = %d (0x%06X), want %d (0x%06X)", gotWire, gotWire, wantWire, wantWire)
	}
}

// TestEncodeSetProtectionBroadcast_Unsupported asserts ErrUnsupportedBroadcastParam
// for the fail-closed codes, and ErrUnsupportedProtectionFamily for unknown families.
func TestEncodeSetProtectionBroadcast_Unsupported(t *testing.T) {
	// QS1A grid_recovery uses 0x5D builder — no clean broadcast opcode swap.
	if _, err := EncodeSetProtectionBroadcast(ModelQS1A, "grid_recovery_time", 60); !errors.Is(err, ErrUnsupportedBroadcastParam) {
		t.Errorf("QS1A grid_recovery_time: want ErrUnsupportedBroadcastParam, got %v", err)
	}
	// QS1A CV mode enum uses multi-frame 0x5F/0x99 — not broadcast-safe.
	if _, err := EncodeSetProtectionBroadcast(ModelQS1A, "Over_frequency_Watt_set", 15); !errors.Is(err, ErrUnsupportedBroadcastParam) {
		t.Errorf("QS1A CV: want ErrUnsupportedBroadcastParam, got %v", err)
	}
	// Unsupported family.
	if _, err := EncodeSetProtectionBroadcast(ModelYC600, "Over_frequency_Watt_Low_set", 50.3); !errors.Is(err, ErrUnsupportedProtectionFamily) {
		t.Errorf("YC600: want ErrUnsupportedProtectionFamily, got %v", err)
	}
	// value=0 guard.
	if _, err := EncodeSetProtectionBroadcast(ModelDS3, "Over_frequency_Watt_Low_set", 0); err == nil {
		t.Error("value=0: want error, got nil")
	}
}

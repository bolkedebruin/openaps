package codec

import (
	"bytes"
	"testing"
)

// TestDS3TripUnicast_GoldenFrames pins the exact L2 bytes for every DS3
// volt/freq trip threshold and clearance-time write. Golden raw values
// are from iSetPvGrid broadcast captures (main.exe set_parameters_boardcast
// @ 0x440fc); the sub-bytes match set_protection_dsp_DS3_D_all call sites.
//
// Frame layout: FB FB 06 AA <sub> 00 00 <val-hi> <val-lo> <csum-hi> <csum-lo> FE FE
// Scale: volt = int(V/0.268), freq = int(Hz×100), clear-time = int(s×100).
func TestDS3TripUnicast_GoldenFrames(t *testing.T) {
	cases := []struct {
		name      string
		paramName string
		value     float64
		wantFrame []byte // complete 13-byte L2 frame
	}{
		// --- Voltage trip thresholds ---
		// AI over_voltage_fast: 264 V → raw 985 (0x03D9), sub 0x01
		{"AI over_voltage_fast 264V", "over_voltage_fast", 264.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x01, 0x00, 0x00, 0x03, 0xD9, 0x01, 0x8D, 0xFE, 0xFE}},
		// AH under_voltage_fast: 196 V → raw 731 (0x02DB), sub 0x02
		{"AH under_voltage_fast 196V", "under_voltage_fast", 196.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x02, 0x00, 0x00, 0x02, 0xDB, 0x01, 0x8F, 0xFE, 0xFE}},
		// AD over_voltage_slow: 264 V → raw 985 (0x03D9), sub 0x03
		{"AD over_voltage_slow 264V", "over_voltage_slow", 264.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x03, 0x00, 0x00, 0x03, 0xD9, 0x01, 0x8F, 0xFE, 0xFE}},
		// AQ under_voltage_stage_2: 184 V → raw 686 (0x02AE), sub 0x04
		{"AQ under_voltage_stage_2 184V", "under_voltage_stage_2", 184.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x04, 0x00, 0x00, 0x02, 0xAE, 0x01, 0x64, 0xFE, 0xFE}},
		// AY over_voltage_slow_90: 253 V → raw 944 (0x03B0), sub 0x05
		{"AY over_voltage_slow_90 253V", "over_voltage_slow_90", 253.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x05, 0x00, 0x00, 0x03, 0xB0, 0x01, 0x68, 0xFE, 0xFE}},
		// AC under_voltage_stage_2_90: 196 V → raw 731 (0x02DB), sub 0x06
		{"AC under_voltage_stage_2_90 196V", "under_voltage_stage_2_90", 196.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x06, 0x00, 0x00, 0x02, 0xDB, 0x01, 0x93, 0xFE, 0xFE}},

		// --- Frequency trip thresholds ---
		// AK over_frequency_fast: 52.0 Hz → raw 5200 (0x1450), sub 0x09
		{"AK over_frequency_fast 52.0Hz", "over_frequency_fast", 52.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x09, 0x00, 0x00, 0x14, 0x50, 0x01, 0x1D, 0xFE, 0xFE}},
		// AJ under_frequency_fast: 47.0 Hz → raw 4700 (0x125C), sub 0x0A
		{"AJ under_frequency_fast 47.0Hz", "under_frequency_fast", 47.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x0A, 0x00, 0x00, 0x12, 0x5C, 0x01, 0x28, 0xFE, 0xFE}},
		// AF over_frequency_slow: 52.0 Hz → raw 5200 (0x1450), sub 0x0B
		{"AF over_frequency_slow 52.0Hz", "over_frequency_slow", 52.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x0B, 0x00, 0x00, 0x14, 0x50, 0x01, 0x1F, 0xFE, 0xFE}},
		// AE under_frequency_slow: 47.5 Hz → raw 4750 (0x128E), sub 0x0C
		{"AE under_frequency_slow 47.5Hz", "under_frequency_slow", 47.5,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x0C, 0x00, 0x00, 0x12, 0x8E, 0x01, 0x5C, 0xFE, 0xFE}},

		// --- Voltage trip clearance times ---
		// BC Over_Voltage1_clearance_time: 0.10 s → raw 10 (0x000A), sub 0x0F
		{"BC Over_Voltage1_clearance_time 0.10s", "Over_Voltage1_clearance_time", 0.10,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x0F, 0x00, 0x00, 0x00, 0x0A, 0x00, 0xC9, 0xFE, 0xFE}},
		// BB Under_Voltage1_clearance_time: 0.02 s → raw 2 (0x0002), sub 0x10
		{"BB Under_Voltage1_clearance_time 0.02s", "Under_Voltage1_clearance_time", 0.02,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x10, 0x00, 0x00, 0x00, 0x02, 0x00, 0xC2, 0xFE, 0xFE}},
		// BE Over_Voltage2_clearance_time: 0.05 s → raw 5 (0x0005), sub 0x11
		{"BE Over_Voltage2_clearance_time 0.05s", "Over_Voltage2_clearance_time", 0.05,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x11, 0x00, 0x00, 0x00, 0x05, 0x00, 0xC6, 0xFE, 0xFE}},
		// BD Under_Voltage2_clearance_time: 1.2 s → raw 120 (0x0078), sub 0x12
		{"BD Under_Voltage2_clearance_time 1.2s", "Under_Voltage2_clearance_time", 1.2,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x12, 0x00, 0x00, 0x00, 0x78, 0x01, 0x3A, 0xFE, 0xFE}},

		// --- Frequency trip clearance times ---
		// BI Over_Frequency1_clearance_time: 0.30 s → raw 30 (0x001E), sub 0x15
		{"BI Over_Frequency1_clearance_time 0.30s", "Over_Frequency1_clearance_time", 0.30,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x15, 0x00, 0x00, 0x00, 0x1E, 0x00, 0xE3, 0xFE, 0xFE}},
		// BH Under_Frequency1_clearance_time: 0.16 s → raw 16 (0x0010), sub 0x16
		{"BH Under_Frequency1_clearance_time 0.16s", "Under_Frequency1_clearance_time", 0.16,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x16, 0x00, 0x00, 0x00, 0x10, 0x00, 0xD6, 0xFE, 0xFE}},
		// BK Over_Frequency2_clearance_time: 0.30 s → raw 30 (0x001E), sub 0x17
		{"BK Over_Frequency2_clearance_time 0.30s", "Over_Frequency2_clearance_time", 0.30,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x17, 0x00, 0x00, 0x00, 0x1E, 0x00, 0xE5, 0xFE, 0xFE}},
		// BJ Under_Frequency2_clearance_time: 0.30 s → raw 30 (0x001E), sub 0x18
		{"BJ Under_Frequency2_clearance_time 0.30s", "Under_Frequency2_clearance_time", 0.30,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x18, 0x00, 0x00, 0x00, 0x1E, 0x00, 0xE6, 0xFE, 0xFE}},

		// --- 10-min average over-voltage ---
		// AB min10_Over_average_voltage: 253 V → raw 944 (0x03B0), sub 0x42
		{"AB min10_Over_average_voltage 253V", "min10_Over_average_voltage", 253.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAA, 0x42, 0x00, 0x00, 0x03, 0xB0, 0x01, 0xA5, 0xFE, 0xFE}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frames, err := EncodeSetProtection(ModelDS3, tc.paramName, tc.value)
			if err != nil {
				t.Fatalf("EncodeSetProtection: %v", err)
			}
			if len(frames) != 1 {
				t.Fatalf("got %d frames, want 1", len(frames))
			}
			if !bytes.Equal(frames[0], tc.wantFrame) {
				t.Errorf("frame:\n got  % X\n want % X", frames[0], tc.wantFrame)
			}
		})
	}
}

// TestDS3TripBroadcast_GoldenFrames verifies that EncodeSetProtectionBroadcast
// produces the same body as unicast with only opcode 0xAA→0xAB and a recomputed
// checksum. The broadcast frames are the ones actually captured from iSetPvGrid.
func TestDS3TripBroadcast_GoldenFrames(t *testing.T) {
	cases := []struct {
		name      string
		paramName string
		value     float64
		wantFrame []byte // complete 13-byte L2 broadcast frame
	}{
		// Broadcast opcode 0xAB; all body bytes identical to unicast; checksum differs by 1.
		{"AI over_voltage_fast 264V bcast", "over_voltage_fast", 264.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x01, 0x00, 0x00, 0x03, 0xD9, 0x01, 0x8E, 0xFE, 0xFE}},
		{"AH under_voltage_fast 196V bcast", "under_voltage_fast", 196.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x02, 0x00, 0x00, 0x02, 0xDB, 0x01, 0x90, 0xFE, 0xFE}},
		{"AD over_voltage_slow 264V bcast", "over_voltage_slow", 264.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x03, 0x00, 0x00, 0x03, 0xD9, 0x01, 0x90, 0xFE, 0xFE}},
		{"AQ under_voltage_stage_2 184V bcast", "under_voltage_stage_2", 184.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x04, 0x00, 0x00, 0x02, 0xAE, 0x01, 0x65, 0xFE, 0xFE}},
		{"AY over_voltage_slow_90 253V bcast", "over_voltage_slow_90", 253.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x05, 0x00, 0x00, 0x03, 0xB0, 0x01, 0x69, 0xFE, 0xFE}},
		{"AC under_voltage_stage_2_90 196V bcast", "under_voltage_stage_2_90", 196.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x06, 0x00, 0x00, 0x02, 0xDB, 0x01, 0x94, 0xFE, 0xFE}},
		{"AK over_frequency_fast 52.0Hz bcast", "over_frequency_fast", 52.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x09, 0x00, 0x00, 0x14, 0x50, 0x01, 0x1E, 0xFE, 0xFE}},
		{"AJ under_frequency_fast 47.0Hz bcast", "under_frequency_fast", 47.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x0A, 0x00, 0x00, 0x12, 0x5C, 0x01, 0x29, 0xFE, 0xFE}},
		{"AF over_frequency_slow 52.0Hz bcast", "over_frequency_slow", 52.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x0B, 0x00, 0x00, 0x14, 0x50, 0x01, 0x20, 0xFE, 0xFE}},
		{"AE under_frequency_slow 47.5Hz bcast", "under_frequency_slow", 47.5,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x0C, 0x00, 0x00, 0x12, 0x8E, 0x01, 0x5D, 0xFE, 0xFE}},
		{"BC Over_Voltage1_clearance_time 0.10s bcast", "Over_Voltage1_clearance_time", 0.10,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x0F, 0x00, 0x00, 0x00, 0x0A, 0x00, 0xCA, 0xFE, 0xFE}},
		{"BB Under_Voltage1_clearance_time 0.02s bcast", "Under_Voltage1_clearance_time", 0.02,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x10, 0x00, 0x00, 0x00, 0x02, 0x00, 0xC3, 0xFE, 0xFE}},
		{"BE Over_Voltage2_clearance_time 0.05s bcast", "Over_Voltage2_clearance_time", 0.05,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x11, 0x00, 0x00, 0x00, 0x05, 0x00, 0xC7, 0xFE, 0xFE}},
		{"BD Under_Voltage2_clearance_time 1.2s bcast", "Under_Voltage2_clearance_time", 1.2,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x12, 0x00, 0x00, 0x00, 0x78, 0x01, 0x3B, 0xFE, 0xFE}},
		{"BI Over_Frequency1_clearance_time 0.30s bcast", "Over_Frequency1_clearance_time", 0.30,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x15, 0x00, 0x00, 0x00, 0x1E, 0x00, 0xE4, 0xFE, 0xFE}},
		{"BH Under_Frequency1_clearance_time 0.16s bcast", "Under_Frequency1_clearance_time", 0.16,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x16, 0x00, 0x00, 0x00, 0x10, 0x00, 0xD7, 0xFE, 0xFE}},
		{"BK Over_Frequency2_clearance_time 0.30s bcast", "Over_Frequency2_clearance_time", 0.30,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x17, 0x00, 0x00, 0x00, 0x1E, 0x00, 0xE6, 0xFE, 0xFE}},
		{"BJ Under_Frequency2_clearance_time 0.30s bcast", "Under_Frequency2_clearance_time", 0.30,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x18, 0x00, 0x00, 0x00, 0x1E, 0x00, 0xE7, 0xFE, 0xFE}},
		{"AB min10_Over_average_voltage 253V bcast", "min10_Over_average_voltage", 253.0,
			[]byte{0xFB, 0xFB, 0x06, 0xAB, 0x42, 0x00, 0x00, 0x03, 0xB0, 0x01, 0xA6, 0xFE, 0xFE}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frames, err := EncodeSetProtectionBroadcast(ModelDS3, tc.paramName, tc.value)
			if err != nil {
				t.Fatalf("EncodeSetProtectionBroadcast: %v", err)
			}
			if len(frames) != 1 {
				t.Fatalf("got %d frames, want 1", len(frames))
			}
			if !bytes.Equal(frames[0], tc.wantFrame) {
				t.Errorf("frame:\n got  % X\n want % X", frames[0], tc.wantFrame)
			}
		})
	}
}

// TestDS3TripBroadcast_OpcodeBodyRelation verifies that for every trip/clear-time
// param, the broadcast frame is exactly the unicast frame with byte[3]=0xAB and a
// recomputed checksum (bytes [5..8] and the sub-byte at [4] are unchanged).
func TestDS3TripBroadcast_OpcodeBodyRelation(t *testing.T) {
	params := []struct {
		name  string
		value float64
	}{
		{"over_voltage_fast", 264.0},
		{"under_voltage_fast", 196.0},
		{"over_voltage_slow", 264.0},
		{"under_voltage_stage_2", 184.0},
		{"over_voltage_slow_90", 253.0},
		{"under_voltage_stage_2_90", 196.0},
		{"over_frequency_fast", 52.0},
		{"under_frequency_fast", 47.0},
		{"over_frequency_slow", 52.0},
		{"under_frequency_slow", 47.5},
		{"Over_Voltage1_clearance_time", 0.10},
		{"Under_Voltage1_clearance_time", 0.02},
		{"Over_Voltage2_clearance_time", 0.05},
		{"Under_Voltage2_clearance_time", 1.2},
		{"Over_Frequency1_clearance_time", 0.30},
		{"Under_Frequency1_clearance_time", 0.16},
		{"Over_Frequency2_clearance_time", 0.30},
		{"Under_Frequency2_clearance_time", 0.30},
		{"min10_Over_average_voltage", 253.0},
	}

	for _, p := range params {
		t.Run(p.name, func(t *testing.T) {
			uni, err := EncodeSetProtection(ModelDS3, p.name, p.value)
			if err != nil {
				t.Fatalf("unicast: %v", err)
			}
			bcast, err := EncodeSetProtectionBroadcast(ModelDS3, p.name, p.value)
			if err != nil {
				t.Fatalf("broadcast: %v", err)
			}
			if len(uni) != 1 || len(bcast) != 1 {
				t.Fatalf("expected single-frame; uni=%d bcast=%d", len(uni), len(bcast))
			}
			u, b := uni[0], bcast[0]
			if len(u) != len(b) {
				t.Fatalf("length mismatch: uni=%d bcast=%d", len(u), len(b))
			}
			if b[3] != CmdSetPowerDS3Broadcast {
				t.Errorf("broadcast opcode = 0x%02X, want 0x%02X", b[3], CmdSetPowerDS3Broadcast)
			}
			// Sub-byte and body bytes [4..8] must be identical.
			for j := 4; j <= 8; j++ {
				if b[j] != u[j] {
					t.Errorf("byte[%d]: uni=0x%02X bcast=0x%02X (body diverged)", j, u[j], b[j])
				}
			}
			// Verify broadcast checksum.
			var s uint16
			for _, x := range b[2:9] {
				s += uint16(x)
			}
			gotCS := uint16(b[9])<<8 | uint16(b[10])
			if gotCS != s {
				t.Errorf("broadcast checksum 0x%04X != sum(bytes[2..8]) 0x%04X", gotCS, s)
			}
		})
	}
}

// TestQS1ATripEncoders_VoltTrip checks that QS1A encodes AH (under_voltage_fast)
// and AI (over_voltage_fast) using the 0x1C path with subs 0x8E and 0x90 and
// scale int(V × 1.332 × 4). These encoders are decompile-derived; no on-wire
// golden capture exists (QS1A returns no page-A).
func TestQS1ATripEncoders_VoltTrip(t *testing.T) {
	// qs1VoltTripWriteScale = qsProtVoltDenom * 4 = 1.332 * 4 = 5.328
	const qs1VoltTripWriteScale = 5.328

	cases := []struct {
		name      string
		paramName string
		value     float64
		wantSub   byte
	}{
		{"AH under_voltage_fast 230V", "under_voltage_fast", 230.0, 0x8E},
		{"AI over_voltage_fast 253V", "over_voltage_fast", 253.0, 0x90},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frames, err := EncodeSetProtection(ModelQS1A, tc.paramName, tc.value)
			if err != nil {
				t.Fatalf("EncodeSetProtection QS1A %q: %v", tc.paramName, err)
			}
			if len(frames) != 1 {
				t.Fatalf("QS1A %q: got %d frames, want 1", tc.paramName, len(frames))
			}
			f := frames[0]
			if f[3] != CmdSetPowerQS1Unicast {
				t.Errorf("opcode = 0x%02X, want CmdSetPowerQS1Unicast (0x1C)", f[3])
			}
			if f[4] != tc.wantSub {
				t.Errorf("sub = 0x%02X, want 0x%02X", f[4], tc.wantSub)
			}
			// byte_count must be 2 (16-bit value)
			if f[5] != 2 {
				t.Errorf("byte_count = %d, want 2", f[5])
			}
			// Frame layout (set_protection_new_one @ 0x66f2c, byte_count 2):
			// FB FB 06 <cmd> <sub> 02 <val-hi> <val-lo> 00 <cs-hi> <cs-lo> FE FE
			// wire value = int(V × 5.328), big-endian LEFT-aligned at [6..7].
			wantRaw := uint16(int(tc.value * qs1VoltTripWriteScale))
			gotRaw := uint16(f[6])<<8 | uint16(f[7])
			if gotRaw != wantRaw {
				t.Errorf("wire value = %d (0x%04X), want %d (0x%04X)", gotRaw, gotRaw, wantRaw, wantRaw)
			}
			if f[8] != 0 {
				t.Errorf("trailing pad byte [8] = 0x%02X, want 0x00", f[8])
			}
			// checksum
			var s uint16
			for _, b := range f[2:9] {
				s += uint16(b)
			}
			gotCS := uint16(f[9])<<8 | uint16(f[10])
			if gotCS != s {
				t.Errorf("checksum 0x%04X != sum(bytes[2..8]) 0x%04X", gotCS, s)
			}
		})
	}
}

// TestQS1ATripEncoders_ClearTimes verifies that QS1A encodes all 8 clearance-time
// params via the 0x1C path with the correct sub-bytes and int(s×100) scaling.
// Encoders are decompile-derived; no on-wire golden capture exists for QS1A.
func TestQS1ATripEncoders_ClearTimes(t *testing.T) {
	cases := []struct {
		name      string
		paramName string
		value     float64
		wantSub   byte
	}{
		{"BB Under_Voltage1_clearance_time", "Under_Voltage1_clearance_time", 0.10, 0x36},
		{"BC Over_Voltage1_clearance_time", "Over_Voltage1_clearance_time", 0.20, 0x38},
		{"BD Under_Voltage2_clearance_time", "Under_Voltage2_clearance_time", 1.20, 0x3A},
		{"BE Over_Voltage2_clearance_time", "Over_Voltage2_clearance_time", 0.05, 0x3C},
		{"BH Under_Frequency1_clearance_time", "Under_Frequency1_clearance_time", 0.16, 0x3E},
		{"BI Over_Frequency1_clearance_time", "Over_Frequency1_clearance_time", 0.30, 0x40},
		{"BJ Under_Frequency2_clearance_time", "Under_Frequency2_clearance_time", 0.30, 0x42},
		{"BK Over_Frequency2_clearance_time", "Over_Frequency2_clearance_time", 0.30, 0x44},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frames, err := EncodeSetProtection(ModelQS1A, tc.paramName, tc.value)
			if err != nil {
				t.Fatalf("EncodeSetProtection QS1A %q: %v", tc.paramName, err)
			}
			if len(frames) != 1 {
				t.Fatalf("QS1A %q: got %d frames, want 1", tc.paramName, len(frames))
			}
			f := frames[0]
			if f[3] != CmdSetPowerQS1Unicast {
				t.Errorf("opcode = 0x%02X, want CmdSetPowerQS1Unicast (0x1C)", f[3])
			}
			if f[4] != tc.wantSub {
				t.Errorf("sub = 0x%02X, want 0x%02X", f[4], tc.wantSub)
			}
			// byte_count must be 2 (16-bit value)
			if f[5] != 2 {
				t.Errorf("byte_count = %d, want 2", f[5])
			}
			// Frame layout (set_protection_new_one @ 0x66f2c, byte_count 2):
			// FB FB 06 <cmd> <sub> 02 <val-hi> <val-lo> 00 <cs-hi> <cs-lo> FE FE
			// wire value = int(s × 100), big-endian LEFT-aligned at [6..7].
			wantRaw := uint16(int(tc.value * 100.0))
			gotRaw := uint16(f[6])<<8 | uint16(f[7])
			if gotRaw != wantRaw {
				t.Errorf("wire value = %d (0x%04X), want %d (0x%04X)", gotRaw, gotRaw, wantRaw, wantRaw)
			}
			if f[8] != 0 {
				t.Errorf("trailing pad byte [8] = 0x%02X, want 0x00", f[8])
			}
		})
	}
}

// TestQS1ATripEncoders_VoltTripYC600 checks the slow voltage trips that route
// through set_protection_yc600_one @ 0x66728: the sub is the L2 cmd at frame[3]
// (no 0x1C opcode, no byte_count tag), value left-aligned 16-bit at [4..5],
// scale int(V × 1.332 × 4). Decompile-derived; no on-wire golden yet.
func TestQS1ATripEncoders_VoltTripYC600(t *testing.T) {
	const scale = 5.328 // 1.332 × 4
	cases := []struct {
		paramName string
		value     float64
		wantCmd   byte
	}{
		{"under_voltage_stage_2", 196.0, 0x11},    // AQ
		{"over_voltage_slow", 264.0, 0x12},        // AD
		{"under_voltage_stage_2_90", 196.0, 0x13}, // AC
		{"over_voltage_slow_90", 264.0, 0x14},     // AY
	}
	for _, tc := range cases {
		t.Run(tc.paramName, func(t *testing.T) {
			frames, err := EncodeSetProtection(ModelQS1A, tc.paramName, tc.value)
			if err != nil {
				t.Fatalf("EncodeSetProtection QS1A %q: %v", tc.paramName, err)
			}
			if len(frames) != 1 {
				t.Fatalf("got %d frames, want 1", len(frames))
			}
			f := frames[0]
			// sub is the L2 cmd at [3]; value left-aligned at [4..5], [6]=0.
			if f[3] != tc.wantCmd {
				t.Errorf("cmd(sub) = 0x%02X, want 0x%02X", f[3], tc.wantCmd)
			}
			wantRaw := uint16(int(tc.value * scale))
			gotRaw := uint16(f[4])<<8 | uint16(f[5])
			if gotRaw != wantRaw {
				t.Errorf("wire = %d (0x%04X), want %d (0x%04X)", gotRaw, gotRaw, wantRaw, wantRaw)
			}
			if f[6] != 0 {
				t.Errorf("byte[6] = 0x%02X, want 0x00", f[6])
			}
			assertCksum(t, f)
		})
	}
}

// TestQS1ATripEncoders_FreqTripYC600 checks the four grid frequency trips that
// route through set_protection_yc600_one: sub-as-cmd at frame[3], value
// left-aligned 24-bit at [4..6], scale int(5e7 / Hz).
func TestQS1ATripEncoders_FreqTripYC600(t *testing.T) {
	cases := []struct {
		paramName string
		value     float64
		wantCmd   byte
	}{
		{"under_frequency_fast", 47.0, 0x58}, // AJ
		{"over_frequency_fast", 52.0, 0x57},  // AK
		{"under_frequency_slow", 47.5, 0x5a}, // AE
		{"over_frequency_slow", 51.5, 0x59},  // AF
	}
	for _, tc := range cases {
		t.Run(tc.paramName, func(t *testing.T) {
			frames, err := EncodeSetProtection(ModelQS1A, tc.paramName, tc.value)
			if err != nil {
				t.Fatalf("EncodeSetProtection QS1A %q: %v", tc.paramName, err)
			}
			f := frames[0]
			if f[3] != tc.wantCmd {
				t.Errorf("cmd(sub) = 0x%02X, want 0x%02X", f[3], tc.wantCmd)
			}
			want := uint32(int64(50_000_000.0 / tc.value))
			got := uint32(f[4])<<16 | uint32(f[5])<<8 | uint32(f[6])
			if got != want {
				t.Errorf("wire = %d (0x%06X), want %d (0x%06X)", got, got, want, want)
			}
			assertCksum(t, f)
		})
	}
}

// TestQS1ATripEncoders_AvgOV checks min10_Over_average_voltage (AB): 0x1C
// builder sub 0x7d, 24-bit, scale int(V × 1.332 × 600 × 4).
func TestQS1ATripEncoders_AvgOV(t *testing.T) {
	const scale = 3196.8 // 1.332 × 600 × 4
	frames, err := EncodeSetProtection(ModelQS1A, "min10_Over_average_voltage", 253.0)
	if err != nil {
		t.Fatalf("AB: %v", err)
	}
	f := frames[0]
	if f[3] != CmdSetPowerQS1Unicast || f[4] != 0x7d || f[5] != 3 {
		t.Errorf("cmd/sub/bc = 0x%02X/0x%02X/0x%02X, want 0x1C/0x7d/0x03", f[3], f[4], f[5])
	}
	v := 253.0
	want := uint32(int64(v * scale))
	got := uint32(f[6])<<16 | uint32(f[7])<<8 | uint32(f[8])
	if got != want {
		t.Errorf("wire = %d (0x%06X), want %d (0x%06X)", got, got, want, want)
	}
	assertCksum(t, f)
}

func assertCksum(t *testing.T, f []byte) {
	t.Helper()
	var s uint16
	for _, b := range f[2:9] {
		s += uint16(b)
	}
	if got := uint16(f[9])<<8 | uint16(f[10]); got != s {
		t.Errorf("checksum 0x%04X != sum(bytes[2..8]) 0x%04X", got, s)
	}
}

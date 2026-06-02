package codec

import (
	"encoding/hex"
	"math"
	"strings"
	"testing"

	"github.com/bolkedebruin/openaps/codec/codectest"
)

// hx parses a hex string with optional whitespace into bytes.
func hx(t *testing.T, s string) []byte {
	t.Helper()
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' {
			return -1
		}
		return r
	}, s)
	b, err := hex.DecodeString(clean)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	return b
}

// approxEq returns true if a and b are within tolerance.
func approxEq(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

var (
	qs1aFixture = codectest.QS1AFixtureHex
	ds3Fixture  = codectest.DS3FixtureHex
)

func TestParseL1_QS1A(t *testing.T) {
	frame := hx(t, qs1aFixture)
	env, err := ParseL1(frame)
	if err != nil {
		t.Fatalf("ParseL1: %v", err)
	}
	if env.ShortAddr != 0x5011 {
		t.Fatalf("SA: got 0x%04X want 0x5011", env.ShortAddr)
	}
	if env.PeerUIDString() != "999900000003" {
		t.Fatalf("PeerUID: got %q want 999900000003", env.PeerUIDString())
	}
	if env.Encrypted {
		t.Fatal("expected plaintext (gate byte 0xFB ≥ 0xF0)")
	}
	if len(env.L2Frame) < 88 {
		t.Fatalf("L2Frame too short: %d", len(env.L2Frame))
	}
	if env.L2Frame[0] != 0xFB || env.L2Frame[1] != 0xFB {
		t.Fatalf("L2 SOF: got %02x %02x want FB FB", env.L2Frame[0], env.L2Frame[1])
	}
}

func TestParseL2_QS1A(t *testing.T) {
	frame := hx(t, qs1aFixture)
	env, _ := ParseL1(frame)
	l2, err := ParseL2(env.L2Frame)
	if err != nil {
		t.Fatalf("ParseL2: %v", err)
	}
	if l2.Cmd != 0xB1 {
		t.Fatalf("Cmd: got 0x%02X want 0xB1", l2.Cmd)
	}
	if l2.Type != 0x51 {
		t.Fatalf("Type: got 0x%02X want 0x51", l2.Type)
	}
}

func TestDecodeReply_QS1A_FromLiveCapture(t *testing.T) {
	r, err := DecodeReply(hx(t, qs1aFixture))
	if err != nil {
		t.Fatalf("DecodeReply: %v", err)
	}
	if r.Model != "QS1A" {
		t.Fatalf("Model: got %q want QS1A", r.Model)
	}
	if r.PeerUID != "999900000003" {
		t.Fatalf("PeerUID: got %q want 999900000003", r.PeerUID)
	}
	if r.Cmd != 0xB1 {
		t.Fatalf("Cmd: got 0x%02X want 0xB1", r.Cmd)
	}

	// Grid voltage — body[0x12..0x13] = 0x049B = 1179 → 1179/1.32/4 = 223.3
	if !approxEq(r.GridV, 223.3, 0.5) {
		t.Errorf("GridV: got %v want ~223.3", r.GridV)
	}

	// Frequency — body[2..4] = 0x0f3b68 = 998760 → 5e7/998760 = 50.06
	if !approxEq(r.FreqHz, 50.06, 0.05) {
		t.Errorf("FreqHz: got %v want ~50.06", r.FreqHz)
	}

	// Bus voltage — body[0..1] = 0x0401 = 1025 → low DC link reading
	if !approxEq(r.BusV, 24.4, 1.0) {
		t.Errorf("BusV: got %v want ~24", r.BusV)
	}

	if len(r.Panels) != 4 {
		t.Fatalf("expected 4 panels, got %d", len(r.Panels))
	}

	// Sanity: all panels in 30–60V range, currents 0–10A, output 0–500W.
	totalW := 0.0
	for _, p := range r.Panels {
		if p.DCV < 0 || p.DCV > 80 {
			t.Errorf("panel %d DCV out of plausible range: %v", p.Index, p.DCV)
		}
		if p.DCI < 0 || p.DCI > 30 {
			t.Errorf("panel %d DCI out of plausible range: %v", p.Index, p.DCI)
		}
		if p.W < 0 || p.W > 500 {
			t.Errorf("panel %d W out of plausible range: %v", p.Index, p.W)
		}
		totalW += p.W
	}

	// Active power = sum × 0.95
	want := totalW * qs1aEfficien
	if !approxEq(r.ActivePowerW, fround(want), 0.5) {
		t.Errorf("ActivePowerW: got %v want %v (= sum×0.95)", r.ActivePowerW, want)
	}

	if len(r.LifetimeRaw) != 4 {
		t.Fatalf("expected 4 lifetime counters, got %d", len(r.LifetimeRaw))
	}
	if r.LifetimeScale != qs1aLifetime {
		t.Errorf("LifetimeScale: got %v want %v", r.LifetimeScale, qs1aLifetime)
	}
}

func TestDecodeReply_DS3_FromLiveCapture(t *testing.T) {
	r, err := DecodeReply(hx(t, ds3Fixture))
	if err != nil {
		t.Fatalf("DecodeReply: %v", err)
	}
	if r.Model != "DS3" {
		t.Fatalf("Model: got %q want DS3", r.Model)
	}
	if r.PeerUID != "999900000001" {
		t.Fatalf("PeerUID: got %q want 999900000001", r.PeerUID)
	}
	if r.Cmd != 0xBB {
		t.Fatalf("Cmd: got 0x%02X want 0xBB", r.Cmd)
	}

	// Grid voltage — body[0x18..0x19] = 0x035A = 858 → 858 × 0.268 = 230.0
	if !approxEq(r.GridV, 230.0, 0.5) {
		t.Errorf("GridV: got %v want ~230", r.GridV)
	}

	// Frequency — body[0x1a..0x1b] = 0x138F = 5007 → 5007 × 0.010 = 50.07
	if !approxEq(r.FreqHz, 50.07, 0.05) {
		t.Errorf("FreqHz: got %v want ~50.07", r.FreqHz)
	}

	// Active power — body[0x1e..0x1f] = 0x0069 = 105 W (low solar)
	if r.ActivePowerW != 105 {
		t.Errorf("ActivePowerW: got %v want 105", r.ActivePowerW)
	}
	// Reactive — body[0x20..0x21] = 0x002B = 43 VAR
	if r.ReactivePower != 43 {
		t.Errorf("ReactivePower: got %v want 43", r.ReactivePower)
	}

	if len(r.Panels) != 2 {
		t.Fatalf("expected 2 DS3 panels, got %d", len(r.Panels))
	}
	// Panel A V — body[0x10..0x11] = 0x0767 = 1895 → 37.9V
	if !approxEq(r.Panels[0].DCV, 37.9, 0.2) {
		t.Errorf("Panel A V: got %v want ~37.9", r.Panels[0].DCV)
	}
	// Panel B V — body[0x12..0x13] = 0x0769 = 1897 → 37.94V
	if !approxEq(r.Panels[1].DCV, 37.94, 0.2) {
		t.Errorf("Panel B V: got %v want ~37.94", r.Panels[1].DCV)
	}

	if len(r.LifetimeRaw) != 2 {
		t.Fatalf("expected 2 lifetime counters, got %d", len(r.LifetimeRaw))
	}
}

func TestDecodeReply_DS3_StatusFlagsAllZeroInLiveCapture(t *testing.T) {
	r, err := DecodeReply(hx(t, ds3Fixture))
	if err != nil {
		t.Fatalf("DecodeReply: %v", err)
	}
	if r.DS3Status.Raw != [5]byte{} {
		t.Errorf("DS3Status.Raw: got %v want all zero", r.DS3Status.Raw)
	}
	if (r.Status != InverterStatus{}) {
		t.Errorf("expected zero InverterStatus, got %+v", r.Status)
	}
	if (r.QS1AStatus != QS1AStatus{}) {
		t.Errorf("QS1AStatus should stay zero for DS3 replies, got %+v", r.QS1AStatus)
	}
}

func TestDecodeReply_DS3_StatusFlagsBitMap(t *testing.T) {
	// Patches the 5 status bytes of the live fixture and re-decodes.
	// body[0x0b] sits at raw[16+0x0b]: 12 L1 bytes + FB FB type cmd = 16.
	const bodyOff = 16
	cases := []struct {
		name                                string
		b, c, d, e, f                       byte
		dcBus, faultA, faultB, warnA, warnB bool
	}{
		{"all zero", 0, 0, 0, 0, 0, false, false, false, false, false},
		{"DC bus fault (byte 0xd bit 4)", 0, 0, 1 << 4, 0, 0, true, false, false, false, false},
		{"fault A bit 0", 0, 0, 0, 0, 1 << 0, false, true, false, false, false},
		{"fault A bit 6", 0, 0, 0, 0, 1 << 6, false, true, false, false, false},
		{"fault B bit 1", 0, 0, 0, 0, 1 << 1, false, false, true, false, false},
		{"fault B bit 7", 0, 0, 0, 0, 1 << 7, false, false, true, false, false},
		{"warning A bit 0", 0, 0, 0, 1 << 0, 0, false, false, false, true, false},
		{"warning A bit 2", 0, 0, 0, 1 << 2, 0, false, false, false, true, false},
		{"warning B bit 1", 0, 0, 0, 1 << 1, 0, false, false, false, false, true},
		{"warning B bit 3", 0, 0, 0, 1 << 3, 0, false, false, false, false, true},
		{"byte 0xe bit 4 not aggregated", 0, 0, 0, 1 << 4, 0, false, false, false, false, false},
		{"all bytes 0xff", 0xff, 0xff, 0xff, 0xff, 0xff, true, true, true, true, true},
	}
	base := hx(t, ds3Fixture)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := append([]byte(nil), base...)
			buf[bodyOff+0x0b] = tc.b
			buf[bodyOff+0x0c] = tc.c
			buf[bodyOff+0x0d] = tc.d
			buf[bodyOff+0x0e] = tc.e
			buf[bodyOff+0x0f] = tc.f
			r, err := DecodeReply(buf)
			if err != nil {
				t.Fatalf("DecodeReply: %v", err)
			}
			wantRaw := [5]byte{tc.b, tc.c, tc.d, tc.e, tc.f}
			if r.DS3Status.Raw != wantRaw {
				t.Errorf("DS3Status.Raw: got %v want %v", r.DS3Status.Raw, wantRaw)
			}
			want := InverterStatus{
				DCBusFault: tc.dcBus,
				FaultA:      tc.faultA,
				FaultB:      tc.faultB,
				WarningA:    tc.warnA,
				WarningB:    tc.warnB,
			}
			if r.Status != want {
				t.Errorf("Status: got %+v want %+v", r.Status, want)
			}
		})
	}
}

func TestDecodeReply_QS1A_StatusFlagsAllZeroInLiveCapture(t *testing.T) {
	r, err := DecodeReply(hx(t, qs1aFixture))
	if err != nil {
		t.Fatalf("DecodeReply: %v", err)
	}
	if r.QS1AStatus.Main != [4]byte{} {
		t.Errorf("QS1AStatus.Main: got %v want all zero", r.QS1AStatus.Main)
	}
	if r.QS1AStatus.Grid != [2]byte{} {
		t.Errorf("QS1AStatus.Grid: got %v want all zero", r.QS1AStatus.Grid)
	}
	if r.QS1AStatus.Extra != 0 {
		t.Errorf("QS1AStatus.Extra: got %v want 0", r.QS1AStatus.Extra)
	}
	if (r.Status != InverterStatus{}) {
		t.Errorf("expected zero InverterStatus, got %+v", r.Status)
	}
	if (r.DS3Status != DS3Status{}) {
		t.Errorf("DS3Status should stay zero for QS1A replies, got %+v", r.DS3Status)
	}
}

func TestDecodeReply_QS1A_StatusFlagsBitMap(t *testing.T) {
	// Patches the QS1A status bytes (body[0x17..0x1a]) of the live
	// fixture and re-decodes. body[0x17] sits at raw[16+0x17]: 12 L1
	// + FB FB type cmd = 16.
	const bodyOff = 16
	cases := []struct {
		name                                string
		b17, b18, b19, b1a                  byte
		dcBus, faultA, faultB, warnA, warnB bool
	}{
		{"all zero", 0, 0, 0, 0, false, false, false, false, false},
		{"DC bus fault (byte 0x18 bit 4)", 0, 1 << 4, 0, 0, true, false, false, false, false},
		{"fault A: 0x1a bit 4", 0, 0, 0, 1 << 4, false, true, false, false, false},
		{"fault A: 0x1a bit 6", 0, 0, 0, 1 << 6, false, true, false, false, false},
		{"fault A: 0x19 bit 0", 0, 0, 1 << 0, 0, false, true, false, false, false},
		{"fault A: 0x17 bit 5", 1 << 5, 0, 0, 0, false, true, false, false, false},
		{"fault B: 0x1a bit 5", 0, 0, 0, 1 << 5, false, false, true, false, false},
		{"fault B: 0x1a bit 7", 0, 0, 0, 1 << 7, false, false, true, false, false},
		{"fault B: 0x19 bit 1", 0, 0, 1 << 1, 0, false, false, true, false, false},
		{"fault B: 0x17 bit 6", 1 << 6, 0, 0, 0, false, false, true, false, false},
		{"warning A: 0x1a bit 0", 0, 0, 0, 1 << 0, false, false, false, true, false},
		{"warning A: 0x1a bit 2", 0, 0, 0, 1 << 2, false, false, false, true, false},
		{"warning B: 0x1a bit 1", 0, 0, 0, 1 << 1, false, false, false, false, true},
		{"warning B: 0x1a bit 3", 0, 0, 0, 1 << 3, false, false, false, false, true},
		{"0x17 bits 0..4,7 not aggregated", 0b10011111, 0, 0, 0, false, false, false, false, false},
		{"0x19 bits 2..7 not aggregated", 0, 0, 0b11111100, 0, false, false, false, false, false},
		{"all 0xff", 0xff, 0xff, 0xff, 0xff, true, true, true, true, true},
	}
	base := hx(t, qs1aFixture)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := append([]byte(nil), base...)
			buf[bodyOff+0x17] = tc.b17
			buf[bodyOff+0x18] = tc.b18
			buf[bodyOff+0x19] = tc.b19
			buf[bodyOff+0x1a] = tc.b1a
			r, err := DecodeReply(buf)
			if err != nil {
				t.Fatalf("DecodeReply: %v", err)
			}
			wantMain := [4]byte{tc.b17, tc.b18, tc.b19, tc.b1a}
			if r.QS1AStatus.Main != wantMain {
				t.Errorf("QS1AStatus.Main: got %v want %v", r.QS1AStatus.Main, wantMain)
			}
			want := InverterStatus{
				DCBusFault: tc.dcBus,
				FaultA:      tc.faultA,
				FaultB:      tc.faultB,
				WarningA:    tc.warnA,
				WarningB:    tc.warnB,
			}
			if r.Status != want {
				t.Errorf("Status: got %+v want %+v", r.Status, want)
			}
		})
	}
}

func TestDecodeReply_QS1A_StatusPreservesGridAndExtra(t *testing.T) {
	const bodyOff = 16
	buf := append([]byte(nil), hx(t, qs1aFixture)...)
	buf[bodyOff+0x34] = 0x42
	buf[bodyOff+0x38] = 0xa5
	buf[bodyOff+0x39] = 0x5a
	r, err := DecodeReply(buf)
	if err != nil {
		t.Fatalf("DecodeReply: %v", err)
	}
	if r.QS1AStatus.Extra != 0x42 {
		t.Errorf("Extra: got 0x%02x want 0x42", r.QS1AStatus.Extra)
	}
	if r.QS1AStatus.Grid != [2]byte{0xa5, 0x5a} {
		t.Errorf("Grid: got %v want [a5 5a]", r.QS1AStatus.Grid)
	}
}

func TestDecodeReply_RejectsTruncated(t *testing.T) {
	cases := []struct{ name, hexStr string }{
		{"empty", ""},
		{"L1 SOF only", "fcfc"},
		{"missing FE FE", "fcfc501100ff999900000003fbfb51b104"},
		{"bad L1 SOF", "feed5011000099990000000300000000fefe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := DecodeReply(hx(t, c.hexStr))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestDecodeReply_UnknownCmdLeavesBodyForInspection(t *testing.T) {
	// Same SA + UID, but cmd 0xAA — used for set/SET ops, not BB query.
	frame := hx(t, "fcfc 5011 00ff 999900000003 fbfb 06 aa 00 00 00 00 00 00 c1 fefe")
	r, err := DecodeReply(frame)
	if err == nil {
		t.Fatal("unknown cmd should return an error")
	}
	if r.PeerUID != "999900000003" {
		t.Fatalf("PeerUID still parsed: got %q", r.PeerUID)
	}
	if r.Cmd != 0xAA {
		t.Fatalf("Cmd still parsed: got 0x%02X", r.Cmd)
	}
	if !strings.Contains(r.Model, "unknown") {
		t.Fatalf("Model: got %q want contains 'unknown'", r.Model)
	}
}

func TestQS1APanelUnpack(t *testing.T) {
	// Force-craft a 3-byte panel slot with known V=2480, I=1140:
	//   V_raw = 2480 (0x9B0)  → byte[2]=0x9B, byte[1] high nibble = 0
	//   I_raw = 1140 (0x474)  → byte[1] low nibble = 4, byte[0]=0x74
	// Combined byte[0..2] = [0x74, 0x04, 0x9B]
	body := make([]byte, 32)
	copy(body[15:], []byte{0x74, 0x04, 0x9B})

	v := qs1aPanelV(body, 15)
	i := qs1aPanelI(body, 15)
	if !approxEq(v, 2480*qs1aPanelVMax/qs1aADC12bit, 0.001) {
		t.Errorf("panel V: got %v", v)
	}
	if !approxEq(i, 1140*qs1aPanelIMax/qs1aADC12bit, 0.001) {
		t.Errorf("panel I: got %v", i)
	}
}

func TestLifetimeKWh(t *testing.T) {
	r := Reply{
		LifetimeRaw:   []uint64{1000000, 2000000},
		LifetimeScale: ds3Lifetime,
	}
	got := r.LifetimeKWh()
	if len(got) != 2 {
		t.Fatalf("len: %d", len(got))
	}
	want0 := math.Round(1000000*ds3Lifetime*1000) / 1000
	if got[0] != want0 {
		t.Errorf("kWh[0]: got %v want %v", got[0], want0)
	}
}

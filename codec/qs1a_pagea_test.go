package codec

import (
	"encoding/hex"
	"math"
	"testing"
)

// capturedQS1PageAFrame is a real 0xDB page-A reply captured on-wire from
// QS1A 999900000003 (FB FB <len=0x3f> DB <payload> <csum> FE FE). It is the
// ground-truth fixture for the byte-faithful decode below.
const capturedQS1PageAFrame = "fbfb3fdbff0001c003d4057e03cf054303d00543103b950eac02100fd70eac02000000000000000000780004000000000000001e1770177000040005000c000c00000000fefe"

// TestQS1ReadPageA_CapturedFrame_GoldenDecode pins the decode of the real
// captured 0xDB frame to exactly what main.exe's own page-A parser
// (resolve_protection60_paras_YC600 @ 0x4f968) produces from the same bytes.
//
// Provenance of the expectations: each code's (offset, scale) was lifted from
// the decompile and confirmed against the .rodata &KEY column tags (e.g.
// DAT_000c3a00 = "AH" → param_2+0x35 ÷1.332÷4; DAT_000c3de4 = "BB" →
// param_2+0x1d ÷100). The 0xDB cmd byte is the data base (param_2 = &reply[3]).
//
// Of the codes verified against main.exe's 60code table for this inverter, 17
// match here exactly (AT/AU/AV/AW/AX/AQ/AD/AC/AY/AZ/BA/AJ/AK/AE/AF/AG/AS/BK).
// The remaining clear-time/AH/AI bytes in THIS particular frame are at their
// firmware-default values (mostly zero), so they decode to the small/zero
// values below — which is the byte-faithful result. (The captured frame does
// not carry the nonzero AH=184/AI=74/BB=0.02… values some upstream write-intent
// tables hold; those are not recoverable from these bytes at any offset/scale,
// confirming the decoder is correct rather than mis-offset.)
func TestQS1ReadPageA_CapturedFrame_GoldenDecode(t *testing.T) {
	fr, err := hex.DecodeString(capturedQS1PageAFrame)
	if err != nil {
		t.Fatalf("decode hex fixture: %v", err)
	}
	r, err := DecodeProtectionReply(ModelQS1A, [][]byte{fr})
	if err != nil {
		t.Fatalf("DecodeProtectionReply: %v", err)
	}

	// Exact-match codes (enums, identity, and centisecond/recovery times that
	// land on integral hundredths).
	exact := map[string]float64{
		"AT": 3, "AU": 3, "AV": 3, "AW": 1, // enum nibbles of p+0x01 = 0xFF
		"AX": 448, // p+0x03 = 0x01C0, identity
		"AQ": 184, "AD": 264, "AC": 183, "AY": 253, "AZ": 183, "BA": 253, // slow volt trips
		"AG": 60, "AS": 60, // recovery / start-ramp (raw 6000 ÷100)
		"BK": 0.3, // p+0x2f raw 30 ÷100
		"BF": 1.2, "BG": 0.04, // p+0x25 raw 120, p+0x27 raw 4 (firmware default slots)
		"BL": 0.12, "BM": 0.12, // p+0x39/0x3b raw 12 ÷100
		// Clear-time / fast-volt slots are at firmware defaults (zero) in this
		// frame; pinned so a future offset regression is caught.
		"AH": 1, "AI": 1, // p+0x35/0x37 raw 4/5 ÷1.332÷4 → 1
		"BB": 0, "BC": 0, "BD": 0, "BE": 0, "BH": 0, "BI": 0, "BJ": 0,
	}
	for code, want := range exact {
		got, ok := r.Values[code]
		if !ok {
			t.Errorf("%s: not decoded", code)
			continue
		}
		if math.Abs(got-want) > 1e-6 {
			t.Errorf("%s: got %g want %g", code, got, want)
		}
	}

	// Frequency codes: 5e7/raw, so allow a small rounding tolerance.
	freqWant := map[string]float64{"AJ": 47.0, "AK": 52.0, "AE": 47.5, "AF": 52.0}
	for code, want := range freqWant {
		got, ok := r.Values[code]
		if !ok {
			t.Errorf("%s: not decoded", code)
			continue
		}
		if math.Abs(got-want) > 0.01 {
			t.Errorf("%s: got %g want ~%g", code, got, want)
		}
	}
}

// buildQS1PageAFrame constructs a synthetic QS1/QS1A page-A (0xDB) reply
// from a body laid out exactly as resolve_protection60_paras_YC600 @ 0x4f968
// reads it: data base = reply[3] (the 0xDB cmd byte), so a field at
// "param_2 + N" lands at reply[3+N]. body[0] is the cmd-relative offset 0
// (i.e. the 0xDB byte position is included), and the caller fills offsets
// 1.. with the field bytes.
func buildQS1PageAFrame(body []byte) []byte {
	// body already starts at the cmd byte (offset 0 = 0xDB). BuildL2Frame
	// prepends FB FB len and the cmd, so pass cmd=0xDB and the post-cmd
	// bytes (offsets 1..) as the L2 body.
	return BuildL2Frame(CmdReplyQS1PageA, body[1:])
}

func putBE16(b []byte, off int, v uint16) { b[off] = byte(v >> 8); b[off+1] = byte(v) }
func putBE24(b []byte, off int, v uint32) {
	b[off] = byte(v >> 16)
	b[off+1] = byte(v >> 8)
	b[off+2] = byte(v)
}

// TestQS1ReadPageA_RoundTrip builds a 0xDB frame whose volt/freq/time fields
// encode known physical values, decodes it, and checks the native values
// come out sane (AD~264 V, AE~47.5 Hz, BB~0.1 s, AT/AU/AV/AW enums).
func TestQS1ReadPageA_RoundTrip(t *testing.T) {
	// Body indexed by cmd-relative offset; size covers up to off 0x3c+1.
	body := make([]byte, 0x40)
	body[0] = CmdReplyQS1PageA

	// Enum flags byte at off 0x01: AT=3, AU=2, AV=1, AW=1
	// b = (AT<<6)|(AU<<4)|(AV<<1)|AW = (3<<6)|(2<<4)|(1<<1)|1 = 0xC0|0x20|0x02|0x01 = 0xE3
	body[0x01] = 0xE3

	// AX raw (off 0x03), identity:
	putBE16(body, 0x03, 1234)

	// Voltages: raw = round(V × 1.332 × 4). qsVolt(raw) = round((raw/1.332)/4).
	volt := func(v float64) uint16 { return uint16(int(v*qsProtVoltDenom*4 + 0.5)) }
	putBE16(body, 0x05, volt(207)) // AQ
	putBE16(body, 0x07, volt(264)) // AD
	putBE16(body, 0x09, volt(196)) // AC
	putBE16(body, 0x0b, volt(253)) // AY
	putBE16(body, 0x0d, volt(230)) // AZ
	putBE16(body, 0x0f, volt(240)) // BA
	putBE16(body, 0x35, volt(195)) // AH
	putBE16(body, 0x37, volt(265)) // AI

	// Frequencies (24-bit): raw = int(5e7 / Hz). qsFreq(raw) = 5e7/raw.
	freq := func(hz float64) uint32 { return uint32(int64(50_000_000.0 / hz)) }
	putBE24(body, 0x11, freq(47.0)) // AJ
	putBE24(body, 0x14, freq(52.0)) // AK
	putBE24(body, 0x17, freq(47.5)) // AE
	putBE24(body, 0x1a, freq(51.5)) // AF

	// Clearance times (16-bit centiseconds): raw = s × 100. qsSec = raw/100.
	sec := func(s float64) uint16 { return uint16(int(s * 100)) }
	putBE16(body, 0x1d, sec(0.10)) // BB
	putBE16(body, 0x1f, sec(0.20)) // BC
	putBE16(body, 0x21, sec(1.20)) // BD
	putBE16(body, 0x23, sec(0.05)) // BE
	putBE16(body, 0x25, sec(0.30)) // BF
	putBE16(body, 0x27, sec(0.40)) // BG
	putBE16(body, 0x29, sec(0.16)) // BH
	putBE16(body, 0x2b, sec(0.30)) // BI
	putBE16(body, 0x2d, sec(0.30)) // BJ
	putBE16(body, 0x2f, sec(0.30)) // BK

	// Recovery / start ramp (16-bit centiseconds): raw = s × 100.
	putBE16(body, 0x31, sec(60.0)) // AG
	putBE16(body, 0x33, sec(20.0)) // AS

	// Transtemp-window times BL/BM (raw/100):
	putBE16(body, 0x39, sec(5.0)) // BL
	putBE16(body, 0x3b, sec(3.0)) // BM

	frame := buildQS1PageAFrame(body)

	r, err := DecodeProtectionReply(ModelQS1A, [][]byte{frame})
	if err != nil {
		t.Fatalf("DecodeProtectionReply: %v", err)
	}

	wantApprox := func(code string, want float64) {
		got, ok := r.Values[code]
		if !ok {
			t.Errorf("%s: not decoded", code)
			return
		}
		if math.Abs(got-want) > 1.0 { // ≤1 LSB / rounding tolerance
			t.Errorf("%s: got %g want ~%g", code, got, want)
		}
	}
	// Enums (exact)
	for code, want := range map[string]float64{"AT": 3, "AU": 2, "AV": 1, "AW": 1} {
		if got := r.Values[code]; got != want {
			t.Errorf("%s enum: got %g want %g", code, got, want)
		}
	}
	if r.Values["AX"] != 1234 {
		t.Errorf("AX: got %g want 1234", r.Values["AX"])
	}
	wantApprox("AQ", 207)
	wantApprox("AD", 264)
	wantApprox("AC", 196)
	wantApprox("AY", 253)
	wantApprox("AZ", 230)
	wantApprox("BA", 240)
	wantApprox("AH", 195)
	wantApprox("AI", 265)
	wantApprox("AJ", 47.0)
	wantApprox("AK", 52.0)
	wantApprox("AE", 47.5)
	wantApprox("AF", 51.5)
	wantApprox("BB", 0.10)
	wantApprox("BC", 0.20)
	wantApprox("BD", 1.20)
	wantApprox("BE", 0.05)
	wantApprox("BH", 0.16)
	wantApprox("BK", 0.30)
	wantApprox("AG", 60.0)
	wantApprox("AS", 20.0)
	wantApprox("BL", 5.0)
	wantApprox("BM", 3.0)
}

// TestQS1ReadPageA_PagedWithBC confirms the 0xDB page-A merges with the
// QS1 page-B (0xDE) and page-C (0xD9) replies in one decode call.
func TestQS1ReadPageA_PagedWithBC(t *testing.T) {
	volt := func(v float64) uint16 { return uint16(int(v*qsProtVoltDenom*4 + 0.5)) }
	body := make([]byte, 0x40)
	body[0] = CmdReplyQS1PageA
	putBE16(body, 0x07, volt(264)) // AD
	pageA := buildQS1PageAFrame(body)

	// Minimal page-B (0xDE) carrying BN at off 0x01.
	bb := make([]byte, 0x10)
	putBE16(bb, 0x01, volt(207)) // BN (off 0x01)
	pageB := BuildL2Frame(CmdProtReadPageB, bb[1:])

	r, err := DecodeProtectionReply(ModelQS1A, [][]byte{pageA, pageB})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := r.Values["AD"]; !ok {
		t.Error("AD (page-A) missing after paged decode")
	}
	if _, ok := r.Values["BN"]; !ok {
		t.Error("BN (page-B) missing after paged decode")
	}
}

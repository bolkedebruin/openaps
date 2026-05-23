package codec

import (
	"bytes"
	"testing"
)

func TestProtectionQueryFrames(t *testing.T) {
	want := [][]byte{
		{0xFB, 0xFB, 0x06, 0xDD, 0, 0, 0, 0, 0, 0x00, 0xE3, 0xFE, 0xFE},
		{0xFB, 0xFB, 0x06, 0xDE, 0, 0, 0, 0, 0, 0x00, 0xE4, 0xFE, 0xFE},
		{0xFB, 0xFB, 0x06, 0xD9, 0, 0, 0, 0, 0, 0x00, 0xDF, 0xFE, 0xFE},
	}
	got := ProtectionQueryFrames()
	if len(got) != 3 {
		t.Fatalf("got %d frames, want 3", len(got))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("frame %d = % X, want % X", i, got[i], want[i])
		}
	}
}

func hexFrame(s string) []byte {
	var b []byte
	for _, p := range splitWS(s) {
		b = append(b, byteFromHex(p))
	}
	return b
}
func splitWS(s string) []string {
	var out, cur []rune
	_ = cur
	res := []string{}
	for _, f := range []rune(s) {
		if f == ' ' {
			if len(out) > 0 {
				res = append(res, string(out))
				out = nil
			}
			continue
		}
		out = append(out, f)
	}
	if len(out) > 0 {
		res = append(res, string(out))
	}
	return res
}
func byteFromHex(s string) byte {
	var v byte
	for _, c := range s {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= byte(c - '0')
		case c >= 'A' && c <= 'F':
			v |= byte(c-'A') + 10
		case c >= 'a' && c <= 'f':
			v |= byte(c-'a') + 10
		}
	}
	return v
}

// TestDecodeProtectionReply_DS3_LiveCapture decodes the real DS3
// 704000006835 page A+B reply frames captured 2026-05-22 and asserts the
// values match the live 60code ground truth.
func TestDecodeProtectionReply_DS3_LiveCapture(t *testing.T) {
	pageA := hexFrame("FB FB 5C DD DD 03 D9 02 DB 03 D9 02 AE 03 B0 02 DB 03 B0 02 AB 14 50 12 5C 14 50 12 8E 00 02 00 02 00 09 00 02 00 04 00 77 00 04 00 77 00 00 1D 00 00 10 FF FF 03 B0 02 DB 13 9C 13 56 00 3C 02 D0 03 B0 02 DB 13 9C 13 56 0B B8 02 D0 DD 20 40 00 00 1D 00 00 1D 01 D9 04 BD 0A FF FF FF FF 1C 5C FE FE")
	pageB := hexFrame("FB FB 5C DD DE 01 00 20 68 01 13 9C 14 50 13 9C 00 3D 19 93 00 3C 00 03 DD 03 A4 03 20 00 03 E8 00 00 00 00 00 64 00 03 DD 03 A5 03 35 03 04 01 2C 06 0D 03 64 03 B1 0E 32 0A 0D F8 04 89 03 52 00 13 74 13 6F 12 5C 00 14 03 20 07 D0 23 A6 03 14 01 BF 03 D9 FF FF FF FF 04 9A FF FF FF FF 1B 6B FE FE")
	r, err := DecodeProtectionReply(ModelDS3, [][]byte{pageA, pageB})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := map[string]float64{
		"AC": 196, "AD": 264, "AQ": 184, "AY": 253, "AH": 195, "AI": 263,
		"AK": 52, "AJ": 47, "AF": 52, "AE": 47.5, "AG": 60, "AS": 60,
		"BN": 196, "BO": 253, "BP": 49.5, "BQ": 50.2,
		"DC": 50.2, "CC": 52, "CB": 50.2, "DH": 47, "DI": 49.75,
		// page-B residuals recovered statically (security/protection-param-read.md):
		"CV": 13, "DD": 16.5698, "AB": 253,
		// page-A clearance times (seconds): capture-sane spot values.
		"BC": 0.09, "BB": 0.02, "BE": 0.04, "BD": 1.19, "BI": 0.29, "BH": 0.16,
	}
	// Voltage thresholds carry a ±1 LSB truncate-vs-round ambiguity in
	// main.exe's 60code storage (the same raw 731 backs AH=195 and
	// AC=196), so allow ±1 V; frequencies/seconds must be exact.
	volts := map[string]bool{"AC": true, "AD": true, "AQ": true, "AY": true,
		"AH": true, "AI": true, "BN": true, "BO": true, "AB": true}
	for code, w := range want {
		got, ok := r.Values[code]
		if !ok {
			t.Errorf("%s: missing", code)
			continue
		}
		tol := 0.001
		if volts[code] {
			tol = 1.0
		}
		if diff := got - w; diff > tol || diff < -tol {
			t.Errorf("%s = %g, want %g (tol %g)", code, got, w, tol)
		}
	}
}

// TestDecodeProtectionReply_QS1_LiveCapture asserts the QS1 fields that
// ARE retrievable (reconnect on 0xDE, DH/DI on 0xD9) against ground
// truth. QS1A returns no 0xDD page, so volt/freq trips are absent.
func TestDecodeProtectionReply_QS1_LiveCapture(t *testing.T) {
	pageB := hexFrame("FB FB 4D DE 04 14 05 43 0F 69 B5 0F 32 AF 04 14 05 43 0F 69 B5 0F 32 AF 06 66 00 00 0C 57 56 00 00 00 00 04 E8 0F 32 AF 0E AC 02 0F 32 AF 05 84 05 32 00 90 75 30 1F 0F 04 00 00 27 10 05 14 00 25 30 30 30 30 19 06 04 00 62 38 6B 3F 66 00 00 00 00 FE FE")
	pageC := hexFrame("FB FB 4D D9 00 00 00 00 00 03 8F 02 6C 00 00 10 3B 95 0F 55 E1 00 00 00 00 64 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 FE FE")
	r, err := DecodeProtectionReply(ModelQS1A, [][]byte{pageB, pageC})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := map[string]float64{"BN": 196, "BO": 253, "BP": 49.5, "BQ": 50.2, "DH": 47, "DI": 49.75,
		"CB": 50.2, "CC": 52,
		// page-B/C residuals recovered statically: AB (24-bit avg-OV), DD
		// (droop), CV (page-f mode nibble), DC (24-bit, zero in this capture).
		"AB": 253, "DD": 16.562, "CV": 15, "DC": 0}
	for code, w := range want {
		got, ok := r.Values[code]
		if !ok {
			t.Errorf("%s: missing", code)
			continue
		}
		if diff := got - w; diff > 0.02 || diff < -0.02 {
			t.Errorf("%s = %g, want %g", code, got, w)
		}
	}
}

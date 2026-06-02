package modem

import (
	"bytes"
	"testing"
)

// Golden frames captured from main.exe's decompiled builders.

func TestBuildPing(t *testing.T) {
	// zb_test_communication: AA AA AA AA 0D 00*7 then ck=0x000D, 00.
	want := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x0D, 0, 0, 0, 0, 0, 0, 0, 0x00, 0x0D, 0x00}
	got := buildPing()
	if !bytes.Equal(got, want) {
		t.Fatalf("buildPing()\n got % X\nwant % X", got, want)
	}
}

func TestBuildSetPanidChannel(t *testing.T) {
	// zb_change_ecu_panid: params 00 00 PANhi PANlo CH 00 00;
	// ck = 0x05 + PANhi + PANlo + CH, big-endian.
	pan := uint16(0x1234)
	ch := byte(0x10)
	sum := uint16(0x05) + 0x12 + 0x34 + uint16(ch)
	want := []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0x05, 0x00, 0x00, 0x12, 0x34, 0x10, 0x00, 0x00,
		byte(sum >> 8), byte(sum & 0xFF), 0x00,
	}
	got := buildSetPanidChannel(pan, ch)
	if !bytes.Equal(got, want) {
		t.Fatalf("buildSetPanidChannel(0x%04X,%d)\n got % X\nwant % X", pan, ch, got, want)
	}
}

func TestBuildGetConfig(t *testing.T) {
	// init_zigbee2 hardcodes PAN=FFFF CH2=0x0C → AA*4 79 00 00 FF FF 0C 00 00
	// ck = 0x79+0xFF+0xFF+0x0C = 0x0283.
	want := []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0x79, 0x00, 0x00, 0xFF, 0xFF, 0x0C, 0x00, 0x00,
		0x02, 0x83, 0x00,
	}
	got := buildGetConfig(0xFFFF, 0x0C)
	if !bytes.Equal(got, want) {
		t.Fatalf("buildGetConfig\n got % X\nwant % X", got, want)
	}
}

func TestFrameLengthAndPreamble(t *testing.T) {
	for _, f := range [][]byte{buildPing(), buildSetPanidChannel(0xABCD, 0x10), buildGetConfig(0xABCD, 0x0C)} {
		if len(f) != 15 {
			t.Errorf("frame len = %d, want 15: % X", len(f), f)
		}
		for i := 0; i < 4; i++ {
			if f[i] != 0xAA {
				t.Errorf("preamble byte %d = 0x%02X, want 0xAA", i, f[i])
			}
		}
	}
}

func TestChecksumIsByteSum(t *testing.T) {
	f := buildSetPanidChannel(0x9A7C, 0x19)
	var sum uint16
	for _, b := range f[4:12] {
		sum += uint16(b)
	}
	if f[12] != byte(sum>>8) || f[13] != byte(sum&0xFF) {
		t.Errorf("checksum % X does not equal byte-sum 0x%04X of [4..11]", f[12:14], sum)
	}
	if f[14] != 0 {
		t.Errorf("frame[14] = 0x%02X, want 0x00", f[14])
	}
}

func TestFindAck(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
		want int
	}{
		{"cc2530 ack", []byte{0xAB, 0xCD, 0xEF}, 0},
		{"cc2652 ack", []byte{0xAB, 0x26, 0x52}, 0},
		{"ack after junk", []byte{0x00, 0xFC, 0xAB, 0xCD, 0xEF}, 2},
		{"only marker no body", []byte{0xAB}, -1},
		{"marker plus one", []byte{0xAB, 0xCD}, -1},
		{"no marker", []byte{0xFC, 0xFC, 0x00, 0x01}, -1},
		{"empty", []byte{}, -1},
	}
	for _, c := range cases {
		if got := findAck(c.buf); got != c.want {
			t.Errorf("findAck(%s, % X) = %d, want %d", c.name, c.buf, got, c.want)
		}
	}
}

func TestParsePAN(t *testing.T) {
	cases := map[string]uint16{
		"00:11:22:33:44:55": 0x4455,
		"001122334455":      0x4455,
		"AA:BB:CC:DD:EE:FF": 0xEEFF,
		"aabbccddeeff\n":    0xEEFF,
	}
	for in, want := range cases {
		got, err := parsePAN(in)
		if err != nil {
			t.Errorf("parsePAN(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parsePAN(%q) = 0x%04X, want 0x%04X", in, got, want)
		}
	}
	if _, err := parsePAN("12"); err == nil {
		t.Errorf("parsePAN(%q): expected error for <4 hex digits", "12")
	}
}

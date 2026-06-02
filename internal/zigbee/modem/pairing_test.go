package modem

import (
	"bytes"
	"testing"
)

// Golden frames verified byte-for-byte against main.exe's decompiled
// builders and .rodata templates (Ghidra project f114, main.exe).

func TestSerialToBCD6RoundTrip(t *testing.T) {
	cases := []struct {
		serial string
		want   [6]byte
	}{
		// Each nibble is a decimal digit: "999900000003" → 99 99 00 00 00 03.
		{"999900000003", [6]byte{0x99, 0x99, 0x00, 0x00, 0x00, 0x03}},
		{"000000000000", [6]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{"999999999999", [6]byte{0x99, 0x99, 0x99, 0x99, 0x99, 0x99}},
		{"123456789012", [6]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0x12}},
	}
	for _, c := range cases {
		got, err := serialToBCD6(c.serial)
		if err != nil {
			t.Errorf("serialToBCD6(%q): %v", c.serial, err)
			continue
		}
		if got != c.want {
			t.Errorf("serialToBCD6(%q) = % X, want % X", c.serial, got, c.want)
		}
		if back := bcd6ToSerial(got); back != c.serial {
			t.Errorf("bcd6ToSerial(% X) = %q, want %q", got, back, c.serial)
		}
	}
}

func TestSerialToBCD6Errors(t *testing.T) {
	for _, bad := range []string{"", "12345", "1234567890123", "99990000000A", "abcdefabcdef"} {
		if _, err := serialToBCD6(bad); err == nil {
			t.Errorf("serialToBCD6(%q): expected error", bad)
		}
	}
}

func TestCRC16CCITTFalse(t *testing.T) {
	// CRC-16/CCITT-FALSE check value for "123456789" is 0x29B1.
	if got := crc16([]byte("123456789")); got != 0x29B1 {
		t.Errorf("crc16(\"123456789\") = 0x%04X, want 0x29B1", got)
	}
	// Empty input returns the init value.
	if got := crc16(nil); got != 0xFFFF {
		t.Errorf("crc16(nil) = 0x%04X, want 0xFFFF", got)
	}
}

func TestBuildGetInverterShortAddr(t *testing.T) {
	// zb_get_inverter_shortaddress_single @0x0000f284:
	// AA*4 0E 00 00 00 00 00 00 A0 ckHi ckLo 06 <IEEE6>, 21 bytes.
	// ck over [4..11] = 0x0E + 0xA0 = 0x00AE.
	ieee := [6]byte{0x99, 0x99, 0x00, 0x00, 0x00, 0x03}
	want := []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0x0E, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xA0,
		0x00, 0xAE, 0x06,
		0x99, 0x99, 0x00, 0x00, 0x00, 0x03,
	}
	got := buildGetInverterShortAddr(ieee)
	if !bytes.Equal(got, want) {
		t.Fatalf("buildGetInverterShortAddr\n got % X\nwant % X", got, want)
	}
	if len(got) != 21 {
		t.Errorf("len = %d, want 21", len(got))
	}
}

func TestBuildSetInverterPan(t *testing.T) {
	// zb_change_inverter_panid_single @0x0000f5b4:
	// AA*4 0F 00 00 PANhi PANlo CH 00 A0 ckHi ckLo 06 <IEEE6>, 21 bytes.
	// ck over [4..11] = 0x0F + 0x0D + 0xCE + 0x10 + 0xA0 = 0x019A.
	ieee := [6]byte{0x99, 0x99, 0x00, 0x00, 0x00, 0x03}
	want := []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0x0F, 0x00, 0x00, 0x0D, 0xCE, 0x10, 0x00, 0xA0,
		0x01, 0x9A, 0x06,
		0x99, 0x99, 0x00, 0x00, 0x00, 0x03,
	}
	got := buildSetInverterPan(0x0DCE, 0x10, ieee)
	if !bytes.Equal(got, want) {
		t.Fatalf("buildSetInverterPan\n got % X\nwant % X", got, want)
	}
}

func TestBuildSetInverterPanRendezvous(t *testing.T) {
	// PAN 0xFFFF rendezvous: ck = 0x0F + 0xFF + 0xFF + 0x10 + 0xA0 = 0x02BD.
	ieee := [6]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0x12}
	want := []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0x0F, 0x00, 0x00, 0xFF, 0xFF, 0x10, 0x00, 0xA0,
		0x02, 0xBD, 0x06,
		0x12, 0x34, 0x56, 0x78, 0x90, 0x12,
	}
	got := buildSetInverterPan(0xFFFF, 0x10, ieee)
	if !bytes.Equal(got, want) {
		t.Fatalf("buildSetInverterPan(0xFFFF)\n got % X\nwant % X", got, want)
	}
}

func TestBuildPrimeInverterPan(t *testing.T) {
	// send11order @0x00075350, template @0x000ca628:
	// AA*4 11 00 00 PANhi PANlo CH 00 00 ckHi ckLo 08 <IEEE6> 00 01, 23 bytes.
	// ck over [4..11] = 0x11 + 0x0D + 0xCE + 0x10 = 0x00FC.
	ieee := [6]byte{0x99, 0x99, 0x00, 0x00, 0x00, 0x03}
	want := []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0x11, 0x00, 0x00, 0x0D, 0xCE, 0x10, 0x00, 0x00,
		0x00, 0xFC, 0x08,
		0x99, 0x99, 0x00, 0x00, 0x00, 0x03,
		0x00, 0x01,
	}
	got := buildPrimeInverterPan(0x0DCE, 0x10, ieee)
	if !bytes.Equal(got, want) {
		t.Fatalf("buildPrimeInverterPan\n got % X\nwant % X", got, want)
	}
	if len(got) != 23 {
		t.Errorf("len = %d, want 23", len(got))
	}
}

func TestBuildCommitPanNow(t *testing.T) {
	// send22order @0x000754e8, template @0x000ca65c:
	// AA*4 22 00 00 PANhi PANlo CH 00 00 ckHi ckLo 00, 15 bytes.
	// ck over [4..11] = 0x22 + 0x0D + 0xCE + 0x10 = 0x010D.
	want := []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0x22, 0x00, 0x00, 0x0D, 0xCE, 0x10, 0x00, 0x00,
		0x01, 0x0D, 0x00,
	}
	got := buildCommitPanNow(0x0DCE, 0x10)
	if !bytes.Equal(got, want) {
		t.Fatalf("buildCommitPanNow\n got % X\nwant % X", got, want)
	}
	if len(got) != 15 {
		t.Errorf("len = %d, want 15", len(got))
	}
}

func TestBuildBindZigbee(t *testing.T) {
	// zb_off_report_id_and_bind @0x00015b14:
	// AA*4 08 SAhi SAlo 00 00 00 00 A0 ckHi ckLo 00, 15 bytes.
	// ck over [4..11] = 0x08 + 0x12 + 0x34 + 0xA0 = 0x00EE.
	want := []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0x08, 0x12, 0x34, 0x00, 0x00, 0x00, 0x00, 0xA0,
		0x00, 0xEE, 0x00,
	}
	got := buildBindZigbee(0x1234)
	if !bytes.Equal(got, want) {
		t.Fatalf("buildBindZigbee\n got % X\nwant % X", got, want)
	}
	if len(got) != 15 {
		t.Errorf("len = %d, want 15", len(got))
	}
}

func TestBuildReportIdOn(t *testing.T) {
	// autoSearchInverterID @0x000b5a58, template @0x000d0258:
	// AA*4 D1 00*6 FF*6 <count> <seq> crcHi crcLo, 21 bytes.
	// CRC-16/CCITT-FALSE over [4..18].
	count := byte(0x3C)
	seq := byte(0x01)
	prefix := []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0xD1, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		count, seq,
	}
	crc := crc16(prefix[4:19])
	want := append(append([]byte{}, prefix...), byte(crc>>8), byte(crc&0xFF))
	got := buildReportIdOn(count, seq)
	if !bytes.Equal(got, want) {
		t.Fatalf("buildReportIdOn\n got % X\nwant % X", got, want)
	}
	if len(got) != 21 {
		t.Errorf("len = %d, want 21", len(got))
	}
}

func TestBuildReportIdOff(t *testing.T) {
	// AA*4 D2 01 <IEEE6>... FF <seq> crcHi crcLo; len = 6n+10.
	found := [][6]byte{
		{0x99, 0x99, 0x00, 0x00, 0x00, 0x03},
		{0x12, 0x34, 0x56, 0x78, 0x90, 0x12},
	}
	seq := byte(0x05)
	got := buildReportIdOff(found, seq)
	if len(got) != 6*2+10 {
		t.Fatalf("len = %d, want %d\n% X", len(got), 6*2+10, got)
	}
	header := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0xD2, 0x01}
	if !bytes.HasPrefix(got, header) {
		t.Fatalf("missing header\n got % X", got)
	}
	// IEEEs then terminator then seq.
	body := got[6 : 6+12]
	wantBody := []byte{0x99, 0x99, 0x00, 0x00, 0x00, 0x03, 0x12, 0x34, 0x56, 0x78, 0x90, 0x12}
	if !bytes.Equal(body, wantBody) {
		t.Errorf("ieee body\n got % X\nwant % X", body, wantBody)
	}
	if got[18] != 0xFF {
		t.Errorf("terminator = 0x%02X, want 0xFF", got[18])
	}
	if got[19] != seq {
		t.Errorf("seq = 0x%02X, want 0x%02X", got[19], seq)
	}
	crc := crc16(got[4 : 4+6*2+4])
	if got[20] != byte(crc>>8) || got[21] != byte(crc&0xFF) {
		t.Errorf("crc % X, want %04X", got[20:22], crc)
	}
}

func TestBuildReportIdQuiet(t *testing.T) {
	// DAT_000d01c0: AA*4 D3 FF*7 FD, 13 bytes.
	want := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0xD3, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFD}
	got := buildReportIdQuiet()
	if !bytes.Equal(got, want) {
		t.Fatalf("buildReportIdQuiet\n got % X\nwant % X", got, want)
	}
}

func TestParseAnnounce(t *testing.T) {
	ieee := [6]byte{0x99, 0x99, 0x00, 0x00, 0x00, 0x03}
	// Build a valid announce: 1D 1D <IEEE6> crcHi crcLo, CRC over IEEE so
	// the residue over IEEE+crc is zero.
	crc := crc16(ieee[:])
	reply := append([]byte{0x1D, 0x1D}, ieee[:]...)
	reply = append(reply, byte(crc>>8), byte(crc&0xFF))

	got, ok := parseAnnounce(reply)
	if !ok {
		t.Fatalf("parseAnnounce rejected a valid reply % X", reply)
	}
	if got != ieee {
		t.Errorf("parseAnnounce IEEE = % X, want % X", got, ieee)
	}
	if bcd6ToSerial(got) != "999900000003" {
		t.Errorf("decoded serial = %q, want 999900000003", bcd6ToSerial(got))
	}

	// Corrupt CRC → rejected.
	bad := append([]byte{}, reply...)
	bad[len(bad)-1] ^= 0xFF
	if _, ok := parseAnnounce(bad); ok {
		t.Errorf("parseAnnounce accepted a bad-CRC reply")
	}
	// Wrong marker → rejected.
	if _, ok := parseAnnounce([]byte{0x0B, 0x0B, 0, 0, 0, 0, 0, 0, 0, 0}); ok {
		t.Errorf("parseAnnounce accepted a non-1D reply")
	}
	// Too short → rejected.
	if _, ok := parseAnnounce([]byte{0x1D, 0x1D}); ok {
		t.Errorf("parseAnnounce accepted a too-short reply")
	}
}

func TestParseShortAddrReply(t *testing.T) {
	want := [6]byte{0x99, 0x99, 0x00, 0x00, 0x00, 0x03} // serial 999900000003

	// FC FC <SAhi> <SAlo> <flag> <flag> <IEEE 6> → SA from [2..3], IEEE [6..11].
	fcfc := append([]byte{0xFC, 0xFC, 0x12, 0x34, 0x00, 0x00}, want[:]...)
	if sa, ok := parseShortAddrReply(fcfc, want); !ok || sa != 0x1234 {
		t.Errorf("parseShortAddrReply(FC FC ... matching IEEE) = 0x%04X,%v want 0x1234,true", sa, ok)
	}

	// Bare reply <SAhi> <SAlo> <?> <flag> <IEEE 6> → SA [0..1], IEEE [4..9].
	bare := append([]byte{0x00, 0x09, 0xAA, 0xBB}, want[:]...)
	if sa, ok := parseShortAddrReply(bare, want); !ok || sa != 0x0009 {
		t.Errorf("parseShortAddrReply(bare matching IEEE) = 0x%04X,%v want 0x0009,true", sa, ok)
	}

	// Reserved SA (>= 0xFFF8) rejected.
	resv := append([]byte{0xFC, 0xFC, 0xFF, 0xF8, 0x00, 0x00}, want[:]...)
	if _, ok := parseShortAddrReply(resv, want); ok {
		t.Errorf("parseShortAddrReply accepted reserved SA 0xFFF8")
	}

	// Mismatched IEEE (reply for a DIFFERENT inverter) must be rejected so a
	// stray reply can't bind the wrong short address.
	other := [6]byte{0x99, 0x99, 0x00, 0x00, 0x00, 0x01}
	wrong := append([]byte{0xFC, 0xFC, 0x12, 0x34, 0x00, 0x00}, other[:]...)
	if sa, ok := parseShortAddrReply(wrong, want); ok {
		t.Errorf("parseShortAddrReply accepted mismatched-IEEE reply (sa=0x%04X)", sa)
	}

	// Too short to carry the IEEE → rejected.
	if _, ok := parseShortAddrReply([]byte{0xFC, 0xFC, 0x12, 0x34}, want); ok {
		t.Errorf("parseShortAddrReply accepted a reply too short to carry the IEEE")
	}
}

func TestIsEncryptedFrame(t *testing.T) {
	// FC FC ... gate byte < 0xF0 → AES.
	enc := make([]byte, 14)
	enc[0], enc[1] = 0xFC, 0xFC
	enc[12] = 0x10 // gate < 0xF0
	if !isEncryptedFrame(enc) {
		t.Errorf("isEncryptedFrame: AES frame (gate 0x10) not detected")
	}
	// FC FC ... gate byte 0xFB (= L2 SOF) → cleartext.
	plain := make([]byte, 14)
	plain[0], plain[1] = 0xFC, 0xFC
	plain[12] = 0xFB
	if isEncryptedFrame(plain) {
		t.Errorf("isEncryptedFrame: plaintext frame (gate 0xFB) misdetected as AES")
	}
	// Bare 1D 1D announcement → not encrypted.
	if isEncryptedFrame([]byte{0x1D, 0x1D, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}) {
		t.Errorf("isEncryptedFrame: bare announce misdetected as AES")
	}
}

package codec

import (
	"bytes"
	"crypto/aes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// hxa is aes_test's hex helper (codec_test.go already has hx but is
// keyed to *testing.T; reuse it directly).
func hxa(t *testing.T, s string) []byte {
	t.Helper()
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' {
			return -1
		}
		return r
	}, s)
	b, err := hex.DecodeString(clean)
	if err != nil {
		t.Fatalf("decode hex %q: %v", s, err)
	}
	return b
}

func TestIDToBCD(t *testing.T) {
	// From docs/AES-DESIGN.md: "806000042582" → 80 60 00 04 25 82.
	got, err := idToBCD("806000042582")
	if err != nil {
		t.Fatalf("idToBCD: %v", err)
	}
	want := [6]byte{0x80, 0x60, 0x00, 0x04, 0x25, 0x82}
	if got != want {
		t.Fatalf("BCD: got %x want %x", got[:], want[:])
	}

	// And "208000004258" → 20 80 00 00 42 58 (the doc's algorithmic vector).
	got, err = idToBCD("208000004258")
	if err != nil {
		t.Fatalf("idToBCD: %v", err)
	}
	want = [6]byte{0x20, 0x80, 0x00, 0x00, 0x42, 0x58}
	if got != want {
		t.Fatalf("BCD: got %x want %x", got[:], want[:])
	}

	if _, err := idToBCD("short"); !errors.Is(err, ErrAESInverterID) {
		t.Fatalf("idToBCD short: want ErrAESInverterID, got %v", err)
	}
}

func TestDeriveFrameKey(t *testing.T) {
	// docs/AES-DESIGN.md algorithmic vector:
	//   id="208000004258", nonce=11 22 33 44 55 66
	//   key = 11 22 33 44 55 66 20 80 00 00 42 58 18 28 45 90
	nonce := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	ieee := []byte{0x20, 0x80, 0x00, 0x00, 0x42, 0x58}
	got := deriveFrameKey(nonce, ieee)
	want := [16]byte{
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66,
		0x20, 0x80, 0x00, 0x00, 0x42, 0x58,
		0x18, 0x28, 0x45, 0x90,
	}
	if got != want {
		t.Fatalf("key: got %x want %x", got[:], want[:])
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// Encrypt a body, then decrypt the resulting frame and assert we
	// recover the original bytes. Cover the zero-pad-to-16 branch at
	// boundaries: 1 (pads to 16), 14 (pads to 16), 15 (pads to 32 —
	// len byte makes it 16+1=17 → 32), 16 (→ 32), 17 (→ 32), 32 (→48),
	// 100 (→112).
	for _, n := range []int{1, 14, 15, 16, 17, 32, 100} {
		body := make([]byte, n)
		for i := range body {
			body[i] = byte(i*7 + 3)
		}
		nonce := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x12, 0x34}
		frame, err := EncryptTX(0xAA, 0x1234, "806000042582", body, nonce)
		if err != nil {
			t.Fatalf("n=%d EncryptTX: %v", n, err)
		}

		// EncryptTX produces a TX frame (AA AA AA AA ... gate at [15]).
		// DecryptRX expects an RX frame (FC FC ... gate at [12]).
		// Build an RX-shaped frame from the encrypt output's nonce +
		// ciphertext so the round-trip exercises both halves.
		rx := buildRXFromTX(frame, "806000042582")
		got, ieee, err := DecryptRX(rx)
		if err != nil {
			t.Fatalf("n=%d DecryptRX: %v", n, err)
		}
		wantIEEE := [6]byte{0x80, 0x60, 0x00, 0x04, 0x25, 0x82}
		if ieee != wantIEEE {
			t.Fatalf("n=%d ieee: got %x want %x", n, ieee[:], wantIEEE[:])
		}
		if !bytes.Equal(got, body) {
			t.Fatalf("n=%d body: got %x want %x", n, got, body)
		}
	}
}

// buildRXFromTX rebuilds an RX-shaped L1 frame (FC FC | sa | sa | rssi
// | lqi | UID6 | nonce6 | ciphertext) from a TX-shaped one (AA AA AA
// AA | op | sa | sa | 00*4 | A0 | ckhi cklo | len | A0/A1 | nonce6 |
// ciphertext). RX and TX are physical-layer-different but the
// ciphertext + nonce + IEEE are interchangeable, which is what we
// need to exercise round-trip.
func buildRXFromTX(tx []byte, id string) []byte {
	nonce := tx[16:22]
	ct := tx[22:]
	ieee, _ := idToBCD(id)
	rx := make([]byte, 0, 18+len(ct))
	rx = append(rx, 0xFC, 0xFC, 0x12, 0x34, 0x55, 0x66)
	rx = append(rx, ieee[:]...)
	rx = append(rx, nonce...)
	rx = append(rx, ct...)
	return rx
}

func TestDecryptRXFromDesignDoc(t *testing.T) {
	// Algorithmic vector from docs/AES-DESIGN.md "Test vectors":
	//   id    = "208000004258"
	//   nonce = 11 22 33 44 55 66
	//   body  = FB FB 06 BB 00 01 02 03 1A 7C FE FE
	// Build the plaintext block (len|body|zero-pad) and encrypt with
	// crypto/aes directly here (NOT via our EncryptTX) — this isolates
	// the decrypt path from the encrypt path so a symmetric bug in both
	// can't pass the test.
	id := "208000004258"
	nonce := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	body := []byte{0xFB, 0xFB, 0x06, 0xBB, 0x00, 0x01, 0x02, 0x03, 0x1A, 0x7C, 0xFE, 0xFE}

	ieee, err := idToBCD(id)
	if err != nil {
		t.Fatalf("idToBCD: %v", err)
	}
	key := deriveFrameKey(nonce, ieee[:])
	wantKey := []byte{
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66,
		0x20, 0x80, 0x00, 0x00, 0x42, 0x58,
		0x18, 0x28, 0x45, 0x90,
	}
	if !bytes.Equal(key[:], wantKey) {
		t.Fatalf("key: got %x want %x", key[:], wantKey)
	}

	// Plaintext block (len=12 → fits in one 16-byte block).
	pt := []byte{0x0C, 0xFB, 0xFB, 0x06, 0xBB, 0x00, 0x01, 0x02, 0x03, 0x1A, 0x7C, 0xFE, 0xFE, 0, 0, 0}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	ct := make([]byte, 16)
	block.Encrypt(ct, pt)

	// Build RX frame and decrypt it through DecryptRX.
	rx := []byte{0xFC, 0xFC, 0x12, 0x34, 0x55, 0x66}
	rx = append(rx, ieee[:]...)
	rx = append(rx, nonce...)
	rx = append(rx, ct...)

	got, gotIEEE, err := DecryptRX(rx)
	if err != nil {
		t.Fatalf("DecryptRX: %v", err)
	}
	if gotIEEE != ieee {
		t.Fatalf("ieee: got %x want %x", gotIEEE[:], ieee[:])
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("body: got %x want %x", got, body)
	}
}

func TestEncryptTX_FrameLayout(t *testing.T) {
	body := []byte{0xFB, 0xFB, 0x06, 0xBB, 0x00, 0x01, 0x02, 0x03, 0x1A, 0x7C, 0xFE, 0xFE}
	nonce := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	opcode := byte(0xAA)
	sa := uint16(0x1234)
	id := "208000004258"

	frame, err := EncryptTX(opcode, sa, id, body, nonce)
	if err != nil {
		t.Fatalf("EncryptTX: %v", err)
	}

	// Expected ciphertext length: body=12, +1 len prefix = 13 → padded
	// to 16.
	wantCTLen := 16
	wantTotal := 22 + wantCTLen
	if len(frame) != wantTotal {
		t.Fatalf("frame len: got %d want %d", len(frame), wantTotal)
	}

	// Offset checks against docs/AES-DESIGN.md TX layout.
	if !bytes.Equal(frame[0:4], []byte{0xAA, 0xAA, 0xAA, 0xAA}) {
		t.Fatalf("SOF: got %x want AAAAAAAA", frame[0:4])
	}
	if frame[4] != opcode {
		t.Fatalf("opcode: got %02x want %02x", frame[4], opcode)
	}
	if frame[5] != byte(sa>>8) || frame[6] != byte(sa) {
		t.Fatalf("short_addr: got %02x%02x want %04x", frame[5], frame[6], sa)
	}
	for i := 7; i < 11; i++ {
		if frame[i] != 0 {
			t.Fatalf("reserved[%d]: got %02x want 00", i, frame[i])
		}
	}
	if frame[11] != 0xA0 {
		t.Fatalf("frame-flag[11]: got %02x want A0", frame[11])
	}
	// Checksum = signed-arith 16-bit sum of out[4..12].
	var sum int
	for i := 4; i < 12; i++ {
		sum += int(frame[i])
	}
	wantCk := uint16(sum)
	gotCk := uint16(frame[12])<<8 | uint16(frame[13])
	if gotCk != wantCk {
		t.Fatalf("checksum: got %04x want %04x", gotCk, wantCk)
	}
	// Length byte: ciphertext_len + 7.
	if frame[14] != byte(wantCTLen+7) {
		t.Fatalf("len[14]: got %02x want %02x", frame[14], wantCTLen+7)
	}
	// Gate byte: unicast (id != "000000000000") → 0xA1.
	if frame[15] != 0xA1 {
		t.Fatalf("gate[15]: got %02x want A1 (unicast)", frame[15])
	}
	// Nonce at [16:22].
	if !bytes.Equal(frame[16:22], nonce) {
		t.Fatalf("nonce: got %x want %x", frame[16:22], nonce)
	}

	// Broadcast variant.
	bcast, err := EncryptTX(opcode, sa, "000000000000", body, nonce)
	if err != nil {
		t.Fatalf("EncryptTX broadcast: %v", err)
	}
	if bcast[15] != 0xA0 {
		t.Fatalf("broadcast gate[15]: got %02x want A0", bcast[15])
	}
}

func TestEncryptTX_RandomNonceWhenNil(t *testing.T) {
	body := []byte{1, 2, 3}
	a, err := EncryptTX(0xAA, 0x0000, "806000042582", body, nil)
	if err != nil {
		t.Fatalf("EncryptTX A: %v", err)
	}
	b, err := EncryptTX(0xAA, 0x0000, "806000042582", body, nil)
	if err != nil {
		t.Fatalf("EncryptTX B: %v", err)
	}
	// Two nonce-less calls with same body should produce different
	// nonces — and therefore different ciphertext.
	if bytes.Equal(a[16:22], b[16:22]) {
		t.Fatalf("expected different random nonces, got %x twice", a[16:22])
	}
}

func TestEncryptTX_RejectsBadInputs(t *testing.T) {
	if _, err := EncryptTX(0xAA, 0, "short", nil, nil); !errors.Is(err, ErrAESInverterID) {
		t.Fatalf("short id: want ErrAESInverterID, got %v", err)
	}
	if _, err := EncryptTX(0xAA, 0, "806000042582", nil, []byte{1, 2, 3}); !errors.Is(err, ErrAESLengthInvalid) {
		t.Fatalf("bad nonce: want ErrAESLengthInvalid, got %v", err)
	}
}

func TestDecryptRX_RejectsBadLengths(t *testing.T) {
	if _, _, err := DecryptRX([]byte{0xFC, 0xFC}); !errors.Is(err, ErrAESLengthInvalid) {
		t.Fatalf("short: want ErrAESLengthInvalid, got %v", err)
	}
	bad := make([]byte, 18+15) // ciphertext 15 bytes — not multiple of 16
	bad[0], bad[1] = 0xFC, 0xFC
	if _, _, err := DecryptRX(bad); !errors.Is(err, ErrAESLengthInvalid) {
		t.Fatalf("non-16: want ErrAESLengthInvalid, got %v", err)
	}
}

// TestEncryptedFrame_RejectedWhenAESDisabled asserts the default path:
// an encrypted L1 envelope (gate < 0xF0) flows through ParseL1 → its
// Encrypted flag is set → DecodeReplyFromEnvelope returns
// ErrEncrypted because AES is OFF.
func TestEncryptedFrame_RejectedWhenAESDisabled(t *testing.T) {
	prev := AESEnabled()
	SetAESEnabled(false)
	t.Cleanup(func() { SetAESEnabled(prev) })

	// Build a minimum-shaped RX-encrypted frame: gate byte (raw[12])
	// is the first nonce byte = 0x11 < 0xF0.
	raw := hxa(t, "FCFC1234 5566  806000042582  11 22 33 44 55 66"+
		"00112233445566778899AABBCCDDEEFF")
	env, err := ParseL1(raw)
	if err != nil {
		t.Fatalf("ParseL1: %v", err)
	}
	if !env.Encrypted {
		t.Fatalf("expected Encrypted=true (gate=0x11)")
	}
	if _, err := DecodeReplyFromEnvelope(env); !errors.Is(err, ErrEncrypted) {
		t.Fatalf("DecodeReplyFromEnvelope: want ErrEncrypted, got %v", err)
	}
}

// TestEncryptedFrame_DecodedWhenAESEnabled asserts the opt-in path:
// with AES on, an encrypted L1 envelope is decrypted in-line and the
// L2 payload is parsed normally.
func TestEncryptedFrame_DecodedWhenAESEnabled(t *testing.T) {
	prev := AESEnabled()
	SetAESEnabled(true)
	t.Cleanup(func() { SetAESEnabled(prev) })

	// Build a plaintext L2 frame that ParseL2 can validate: FB FB
	// <type> <cmd> <body> <ck hi> <ck lo> FE FE. Use cmd 0xBB (DS3
	// telemetry) with a zero-length body — DecodeReply will then
	// invoke decodeDS3 on an empty body, which is safe (it leaves all
	// fields at zero).
	l2 := []byte{0xFB, 0xFB, 0x10, 0xBB, 0x00, 0x00, 0xFE, 0xFE}

	// Encrypt that L2 frame as the L2 body and wrap it RX-style.
	id := "806000042582"
	ieee, err := idToBCD(id)
	if err != nil {
		t.Fatalf("idToBCD: %v", err)
	}
	nonce := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	// Build plaintext (len|body|zero-pad to 16). len=8 → pad to 16.
	pt := make([]byte, 16)
	pt[0] = byte(len(l2))
	copy(pt[1:], l2)
	key := deriveFrameKey(nonce, ieee[:])
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	ct := make([]byte, 16)
	block.Encrypt(ct, pt)
	raw := []byte{0xFC, 0xFC, 0x50, 0x11, 0x55, 0x66}
	raw = append(raw, ieee[:]...)
	raw = append(raw, nonce...)
	raw = append(raw, ct...)

	r, err := DecodeReply(raw)
	if err != nil {
		t.Fatalf("DecodeReply: %v", err)
	}
	if r.Cmd != 0xBB {
		t.Fatalf("cmd: got %02x want BB", r.Cmd)
	}
	if r.Family != FamilyDS3 {
		t.Fatalf("family: got %v want DS3", r.Family)
	}
	if r.PeerUID != id {
		t.Fatalf("uid: got %q want %q", r.PeerUID, id)
	}
	if r.ShortAddr != 0x5011 {
		t.Fatalf("short_addr: got %04x want 5011", r.ShortAddr)
	}
}

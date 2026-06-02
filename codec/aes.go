package codec

// L1 over-the-air AES-128-ECB codec (Scheme B in docs/AES-DESIGN.md).
//
// This is the per-frame-keyed L1 OTA scheme used by APsystems main.exe
// when AES_flag_ALL=1 (i.e. when at least one bound inverter has an
// id whose second character is '2'). It is DIFFERENT from the static-
// key ECU<->modem AES used in ecu-zb/internal/modem.
//
// Status: EXPERIMENTAL. The implementation matches the reverse-
// engineered spec in docs/AES-DESIGN.md (Ghidra-derived, OpenSSL
// AES_ecb_encrypt round-trip), but no real on-wire ciphertext capture
// is available on the maintainer's fleet (all inverter IDs are '8'/'7'-
// prefixed, AES_flag_ALL=0). This codec is therefore implementation-
// correct-by-construction; wire-compat is asserted to be true but not
// verified.
//
// Gated behind the package-level AESConfig.Enabled flag. When disabled
// (the default), encrypted RX frames continue to surface as
// ErrEncrypted just as before.

import (
	"crypto/aes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"
)

// fixedKeyTail is the 4 hard-coded bytes that occupy key[12..16] in
// every per-frame derived key. From main.exe write_AES @ 0xce80 and
// AES_deencryption @ 0xd944 (four immediate byte stores).
var fixedKeyTail = [4]byte{0x18, 0x28, 0x45, 0x90}

// AES codec errors. These are distinct from ErrEncrypted so callers can
// distinguish "AES disabled, frame rejected" (ErrEncrypted) from "AES
// enabled but decrypt failed for a specific reason".
var (
	// ErrAESLengthInvalid is returned when the raw frame is too short to
	// hold an L1 AES header or when the ciphertext length is not a
	// positive multiple of 16, or when the decrypted length prefix is
	// inconsistent with the remaining plaintext.
	ErrAESLengthInvalid = errors.New("L1 AES: frame length invalid")
	// ErrAESDecryptFailed is returned for cipher-level failures on the
	// decrypt path (the only way crypto/aes.NewCipher returns an error
	// is a non-16-byte key, which our derivation cannot produce — but
	// kept for completeness).
	ErrAESDecryptFailed = errors.New("L1 AES: decrypt failed")
	// ErrAESEncryptFailed is returned for failures originating in
	// EncryptTX — currently only a crypto/rand read error while
	// generating the per-frame nonce, plus the cipher-construction
	// guard symmetric with ErrAESDecryptFailed.
	ErrAESEncryptFailed = errors.New("L1 AES: encrypt failed")
	// ErrAESInverterID is returned when an inverter ID is not the
	// expected 12-character ASCII form.
	ErrAESInverterID = errors.New("L1 AES: inverter id must be 12 ASCII chars")
)

// aesEnabled is a package-level switch; the daemon flips it to true at
// startup when -enable-aes-l1 is passed. Read with sync/atomic so
// concurrent ParseL1/DecodeReply callers see a consistent view.
var aesEnabled atomic.Bool

// SetAESEnabled toggles L1 OTA AES at runtime. The daemon calls this
// once at startup from main(); test code may flip it per-test (always
// restore via t.Cleanup).
func SetAESEnabled(on bool) { aesEnabled.Store(on) }

// AESEnabled reports whether L1 OTA AES is enabled.
func AESEnabled() bool { return aesEnabled.Load() }

// deriveFrameKey builds the 16-byte AES key for one L1 OTA frame.
//
//	key[0:6]   = nonce6   (sender-chosen random)
//	key[6:12]  = ieee6    (6-byte BCD inverter ID; same bytes that
//	                       L1Envelope.PeerUID carries on the wire)
//	key[12:16] = 18 28 45 90
//
// The function takes nonce6 and ieee6 by-value-sized slices so callers
// can pass [6]byte directly via slicing without allocating.
func deriveFrameKey(nonce6, ieee6 []byte) [16]byte {
	var key [16]byte
	copy(key[0:6], nonce6)
	copy(key[6:12], ieee6)
	copy(key[12:16], fixedKeyTail[:])
	return key
}

// idToBCD converts the 12-character ASCII inverter ID into its 6-byte
// BCD form, matching the on-wire PeerUID byte layout. Per main.exe:
//
//	for i in 0..6:
//	    bcd[i] = ((id[2i] << 4) + id[2i+1] + 0xD0) & 0xFF
//
// For digit characters '0'..'9' (0x30..0x39) and 'A'..'F' (0x41..0x46),
// this reduces to standard BCD packing: e.g. "806000042582" →
// 80 60 00 04 25 82.
func idToBCD(id string) ([6]byte, error) {
	var out [6]byte
	if len(id) != 12 {
		return out, fmt.Errorf("%w: got %d chars", ErrAESInverterID, len(id))
	}
	for i := 0; i < 6; i++ {
		out[i] = (id[2*i]<<4 + id[2*i+1] + 0xD0) & 0xFF
	}
	return out, nil
}

// decryptCiphertext runs AES-128-ECB on `ct` with the per-frame key,
// then parses the recovered [len_u8][body][zero-pad to 16] plaintext
// envelope and returns the body slice. ct must be a positive multiple
// of 16. The returned body is a fresh allocation owned by the caller.
//
// Errors are wrapped with the "ciphertext: …" prefix to mark them as
// originating in the cipher/envelope layer; the per-direction callers
// (DecryptRX, decryptEnvelopeInPlace) use the "frame: …" prefix for
// errors about their outer frame shape.
func decryptCiphertext(key [16]byte, ct []byte) ([]byte, error) {
	if len(ct) <= 0 || len(ct)%16 != 0 {
		return nil, fmt.Errorf("%w: ciphertext len %d not positive multiple of 16",
			ErrAESLengthInvalid, len(ct))
	}
	block, cErr := aes.NewCipher(key[:])
	if cErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrAESDecryptFailed, cErr)
	}
	pt := make([]byte, len(ct))
	for off := 0; off < len(ct); off += 16 {
		block.Decrypt(pt[off:off+16], ct[off:off+16])
	}
	bodyLen := int(pt[0])
	if 1+bodyLen > len(pt) {
		return nil, fmt.Errorf("%w: ciphertext decoded body length %d exceeds plaintext %d",
			ErrAESLengthInvalid, bodyLen, len(pt)-1)
	}
	body := make([]byte, bodyLen)
	copy(body, pt[1:1+bodyLen])
	return body, nil
}

// DecryptRX decrypts an inbound L1 AES frame. raw is the full inbound
// L1 frame as parsed by the modem splice:
//
//	[0..2)  FC FC
//	[2..4)  short_addr (big-endian)
//	[4]     RSSI
//	[5]     LQI
//	[6..12) PeerUID  (6-byte BCD inverter ID; also key[6..12])
//	[12..18) NONCE   (sender-chosen random; also key[0..6])
//	[18..N) CIPHERTEXT (multiple of 16 bytes)
//
// On success it returns the inner L2 body (the bytes that would
// otherwise have appeared at raw[12..] as a plaintext "FB FB ... FE FE"
// frame). The caller is responsible for prepending FB FB / appending
// FE FE — see DecryptRXIntoEnvelope which does both.
//
// The 6-byte IEEE (BCD PeerUID) is returned for routing convenience;
// it is equal to raw[6:12].
func DecryptRX(raw []byte) (plaintext []byte, ieee [6]byte, err error) {
	if len(raw) < 0x12 || raw[0] != L1ReplySOF || raw[1] != L1ReplySOF {
		return nil, ieee, fmt.Errorf("%w: frame malformed L1 (len=%d)", ErrAESLengthInvalid, len(raw))
	}
	copy(ieee[:], raw[6:12])

	key := deriveFrameKey(raw[12:18], raw[6:12])
	body, dErr := decryptCiphertext(key, raw[18:])
	if dErr != nil {
		return nil, ieee, dErr
	}
	return body, ieee, nil
}

// DecryptRXIntoEnvelope decrypts raw and returns an L1Envelope shaped
// like the plaintext path: L2Frame begins with the decrypted L2 body
// (which itself starts FB FB ... FE FE and is parseable by ParseL2).
// The Encrypted bit stays true so consumers can branch on it for
// audit/UI purposes, but L2Frame is now plaintext.
//
// This is the function that DecodeReplyFromEnvelope dispatches to when
// AES is enabled. The body returned by DecryptRX *is* a full L2 frame
// (it preserves FB FB / FE FE per the spec) so it is dropped into
// L2Frame unmodified.
func DecryptRXIntoEnvelope(raw []byte) (L1Envelope, error) {
	body, ieee, err := DecryptRX(raw)
	if err != nil {
		return L1Envelope{}, err
	}
	env := L1Envelope{
		ShortAddr: uint16(raw[2])<<8 | uint16(raw[3]),
		RSSI:      raw[4],
		LQI:       raw[5],
		PeerUID:   ieee,
		Encrypted: true,
		L2Frame:   body,
	}
	return env, nil
}

// decryptEnvelopeInPlace converts an already-parsed encrypted L1
// envelope into a plaintext one by decrypting its tail (nonce6 ||
// ciphertext) using the PeerUID we already captured. Returns a new
// envelope (env is treated as immutable) with Encrypted=true preserved
// for audit, but L2Frame holding the plaintext L2 body suitable for
// ParseL2.
//
// The encrypted envelope's L2Frame, as produced by ParseL1, is exactly
// raw[12:] — i.e. [nonce6 (6)] [ciphertext (>=16, multiple of 16)].
// PeerUID (= raw[6:12]) is already on env.
func decryptEnvelopeInPlace(env L1Envelope) (L1Envelope, error) {
	tail := env.L2Frame
	if len(tail) < 6+16 {
		return L1Envelope{}, fmt.Errorf("%w: frame encrypted L2 tail %d < 22 bytes",
			ErrAESLengthInvalid, len(tail))
	}
	nonce := tail[0:6]
	ct := tail[6:]
	key := deriveFrameKey(nonce, env.PeerUID[:])
	body, err := decryptCiphertext(key, ct)
	if err != nil {
		return L1Envelope{}, err
	}
	out := env
	out.L2Frame = body
	return out, nil
}

// EncryptTX builds an outbound encrypted L1 frame. The plaintext fed
// to AES is [len(body) | body | zero-pad to 16].
//
//   - opcode is the L2 command byte (e.g. CmdSetPowerDS3Unicast).
//   - shortAddr is the ZigBee 16-bit address; same as the plaintext
//     path.
//   - id is the 12-character ASCII inverter ID (BCD-packed inside the
//     key). Pass "000000000000" for a broadcast frame; any other ID
//     marks the frame unicast (TX gate byte 0xA1 vs 0xA0).
//   - body is the L2 payload (caller-built, no FB FB / no FE FE).
//   - nonce6, if non-nil, is used verbatim as key[0..6] AND as the
//     on-wire nonce. Production callers pass nil and crypto/rand fills
//     6 fresh bytes; tests pass a fixed nonce for deterministic
//     vectors.
//
// The resulting frame layout matches write_AES @ 0xce80:
//
//	[0..4)    AA AA AA AA
//	[4]       opcode
//	[5..7)    short_addr (big-endian)
//	[7..11)   00 00 00 00
//	[11]      0xA0                       (frame-flag, fixed)
//	[12..14)  checksum BE16(out[4..12])
//	[14]      ciphertext_len + 7
//	[15]      0xA0 (broadcast) | 0xA1 (unicast)
//	[16..22)  nonce6
//	[22..)    ciphertext
func EncryptTX(opcode byte, shortAddr uint16, id string, body []byte, nonce6 []byte) ([]byte, error) {
	if _, err := idToBCD(id); err != nil {
		return nil, err
	}
	ieee, _ := idToBCD(id)

	var nonce [6]byte
	if nonce6 == nil {
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, fmt.Errorf("%w: nonce: %v", ErrAESEncryptFailed, err)
		}
	} else {
		if len(nonce6) != 6 {
			return nil, fmt.Errorf("%w: nonce6 must be 6 bytes, got %d",
				ErrAESLengthInvalid, len(nonce6))
		}
		copy(nonce[:], nonce6)
	}

	// Plaintext: [len|body|zero-pad to 16].
	n := len(body) + 1
	if n%16 != 0 {
		n += 16 - n%16
	}
	pt := make([]byte, n)
	pt[0] = byte(len(body))
	copy(pt[1:], body)

	key := deriveFrameKey(nonce[:], ieee[:])
	block, cErr := aes.NewCipher(key[:])
	if cErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrAESEncryptFailed, cErr)
	}
	ct := make([]byte, n)
	for off := 0; off < n; off += 16 {
		block.Encrypt(ct[off:off+16], pt[off:off+16])
	}

	out := make([]byte, 22+n)
	out[0], out[1], out[2], out[3] = L1Preamble, L1Preamble, L1Preamble, L1Preamble
	out[4] = opcode
	out[5] = byte(shortAddr >> 8)
	out[6] = byte(shortAddr)
	// out[7..11] zeros from make()
	out[11] = 0xA0
	var sum int
	for i := 4; i < 12; i++ {
		sum += int(out[i])
	}
	out[12] = byte(sum >> 8)
	out[13] = byte(sum & 0xFF)
	out[14] = byte(n + 7)
	if id == "000000000000" {
		out[15] = 0xA0 // broadcast
	} else {
		out[15] = 0xA1 // unicast
	}
	copy(out[16:22], nonce[:])
	copy(out[22:], ct)
	return out, nil
}

// PeerUIDHexFromBCD is a tiny helper for logging: it returns the
// 12-character ASCII rendering of a 6-byte BCD PeerUID (the inverse of
// idToBCD for the digit-only IDs we see in the wild). It does not
// validate that the bytes round-trip through idToBCD — pure formatter.
func PeerUIDHexFromBCD(ieee [6]byte) string {
	return hex.EncodeToString(ieee[:])
}

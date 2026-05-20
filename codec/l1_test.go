package codec

import (
	"bytes"
	"testing"
)

func TestBuildL1Frame_UnicastShape(t *testing.T) {
	t.Parallel()
	l2 := []byte{0xFB, 0xFB, 0x06, 0xDC, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xE2, 0xFE, 0xFE}
	got := BuildL1Frame(0x5011, l2)
	// Sum range = 0x55 + 0x50 + 0x11 + 5*0x00 = 0xB6.
	want := hx(t, "AA AA AA AA 55 50 11 00 00 00 00 00 00 B6 0D")
	if !bytes.HasPrefix(got, want) {
		t.Fatalf("L1 prefix:\n  got  % X\n  want % X", got[:len(want)], want)
	}
	if !bytes.Equal(got[len(want):], l2) {
		t.Fatalf("L1 body suffix mismatch")
	}
}

func TestBuildL1Frame_BroadcastShape(t *testing.T) {
	t.Parallel()
	l2, err := EncodeSetPowerDSPNew(80, true)
	if err != nil {
		t.Fatalf("EncodeSetPowerDSPNew: %v", err)
	}
	got := BuildL1Frame(0, l2)
	// Broadcast: SA bytes zero, sum = 0x55 + 0 = 0x55.
	want := hx(t, "AA AA AA AA 55 00 00 00 00 00 00 00 00 55 0D")
	if !bytes.HasPrefix(got, want) {
		t.Fatalf("L1 broadcast prefix:\n  got  % X\n  want % X", got[:len(want)], want)
	}
	if !bytes.Equal(got[len(want):], l2) {
		t.Fatalf("L1 body suffix mismatch")
	}
}

func TestBuildL1Frame_LenByteMatchesL2(t *testing.T) {
	t.Parallel()
	l2 := []byte{0xFB, 0xFB, 0x06, 0xDC, 0xFE, 0xFE}
	got := BuildL1Frame(0x1234, l2)
	if got[14] != byte(len(l2)) {
		t.Fatalf("len byte: got 0x%02X want 0x%02X", got[14], len(l2))
	}
}

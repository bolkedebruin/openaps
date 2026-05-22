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

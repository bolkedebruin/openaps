package codec

import (
	"bytes"
	"testing"
)

func TestEncodeOnOff_FramesMatchMainExe(t *testing.T) {
	cases := []struct {
		name      string
		on        bool
		broadcast bool
		want      []byte
	}{
		{"on unicast", true, false, []byte{0xFB, 0xFB, 0x06, 0xC1, 0, 0, 0, 0, 0, 0x00, 0xC7, 0xFE, 0xFE}},
		{"off unicast", false, false, []byte{0xFB, 0xFB, 0x06, 0xC2, 0, 0, 0, 0, 0, 0x00, 0xC8, 0xFE, 0xFE}},
		{"on broadcast", true, true, []byte{0xFB, 0xFB, 0x06, 0xA1, 0, 0, 0, 0, 0, 0x00, 0xA7, 0xFE, 0xFE}},
		{"off broadcast", false, true, []byte{0xFB, 0xFB, 0x06, 0xA2, 0, 0, 0, 0, 0, 0x00, 0xA8, 0xFE, 0xFE}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EncodeOnOff(tc.on, tc.broadcast)
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("EncodeOnOff(on=%v, bcast=%v) = % X, want % X", tc.on, tc.broadcast, got, tc.want)
			}
		})
	}
}

package codec

import "testing"

// A telemetry/protection/info reply's L2Frame begins with the FB FB L2
// SOF. It must NOT be misread as a PairShortAddrReply (which would take
// short_addr from the first two bytes = 0xFBFB and corrupt the stored
// short address of whichever inverter the L1 envelope names).
func TestParseInboundPair_RejectsL2WrappedFrame(t *testing.T) {
	// Real QS1A telemetry L2Frame (raw[12:]) captured on the wire:
	// FB FB <len> <cmd 0xB1> <body...>
	l2 := mustHex("fbfb51b104730f3dcb0083e16e1f527531436e3c0")
	if pf, ok := ParsePairFrame(DirInbound, l2); ok {
		t.Fatalf("FB FB-wrapped frame parsed as pair %v (short_addr=0x%04X); want rejected", pf.Kind, pf.ShortAddr)
	}
}

// A genuine flat ShortAddrReply (no L2 wrapper) still parses, with the
// short address and UID read from their real offsets.
func TestParseInboundPair_GenuineShortAddrReply(t *testing.T) {
	// short_addr 0x5011 at [0:1], not A5A5 at [4:5], UID at [4:10].
	l2 := mustHex("501100008060000425820000")
	pf, ok := ParsePairFrame(DirInbound, l2)
	if !ok || pf.Kind != PairShortAddrReply {
		t.Fatalf("genuine ShortAddrReply not parsed: ok=%v kind=%v", ok, pf.Kind)
	}
	if pf.ShortAddr != 0x5011 {
		t.Fatalf("short_addr = 0x%04X; want 0x5011", pf.ShortAddr)
	}
	if pf.PeerUID != "806000042582" {
		t.Fatalf("uid = %q; want 806000042582", pf.PeerUID)
	}
}

func mustHex(s string) []byte {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(b); i++ {
		var hi, lo byte
		c := s[2*i]
		switch {
		case c >= '0' && c <= '9':
			hi = c - '0'
		case c >= 'a' && c <= 'f':
			hi = c - 'a' + 10
		}
		c = s[2*i+1]
		switch {
		case c >= '0' && c <= '9':
			lo = c - '0'
		case c >= 'a' && c <= 'f':
			lo = c - 'a' + 10
		}
		b[i] = hi<<4 | lo
	}
	return b
}

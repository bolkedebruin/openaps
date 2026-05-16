package codec

import "testing"

func TestParsePairFrame_OffReportBind(t *testing.T) {
	t.Parallel()
	// AA AA AA AA 08 50 11 00 00 00 00 A0 12 34 00
	l2 := hx(t, "AAAAAAAA 08 5011 0000 0000 A0 1234 00")
	got, ok := ParsePairFrame(DirOutbound, l2)
	if !ok {
		t.Fatalf("expected match")
	}
	if got.Kind != PairOffReportBind {
		t.Errorf("Kind: got %d want PairOffReportBind", got.Kind)
	}
	if got.ShortAddr != 0x5011 {
		t.Errorf("ShortAddr: got 0x%X want 0x5011", got.ShortAddr)
	}
}

func TestParsePairFrame_OffReportBind_BadA0(t *testing.T) {
	t.Parallel()
	l2 := hx(t, "AAAAAAAA 08 5011 0000 0000 99 1234 00")
	if _, ok := ParsePairFrame(DirOutbound, l2); ok {
		t.Fatalf("expected no-match for missing A0")
	}
}

func TestParsePairFrame_ChangePan11(t *testing.T) {
	t.Parallel()
	// AA AA AA AA 11 00 00 <mac=AB CD> <chan=1A> 00 00 <crc=11 22> 00 08 <uid=80 60 00 04 25 82> 01
	l2 := hx(t, "AAAAAAAA 11 0000 ABCD 1A 0000 1122 00 08 806000042582 01")
	got, ok := ParsePairFrame(DirOutbound, l2)
	if !ok {
		t.Fatalf("expected match")
	}
	if got.Kind != PairChangePan11 {
		t.Errorf("Kind: got %d want PairChangePan11", got.Kind)
	}
	if got.PeerUID != "806000042582" {
		t.Errorf("PeerUID: got %q want 806000042582", got.PeerUID)
	}
}

func TestParsePairFrame_ChangePan11_BadTrailer(t *testing.T) {
	t.Parallel()
	// trailing byte must be 0x01
	l2 := hx(t, "AAAAAAAA 11 0000 ABCD 1A 0000 1122 00 08 806000042582 02")
	if _, ok := ParsePairFrame(DirOutbound, l2); ok {
		t.Fatalf("expected no-match for trailer != 0x01")
	}
}

func TestParsePairFrame_ChangePan22(t *testing.T) {
	t.Parallel()
	// AA AA AA AA 22 00 00 <mac> <chan> 00 00 <crc> 00 — 15 bytes
	l2 := hx(t, "AAAAAAAA 22 0000 ABCD 1A 0000 1122 00")
	got, ok := ParsePairFrame(DirOutbound, l2)
	if !ok {
		t.Fatalf("expected match")
	}
	if got.Kind != PairChangePan22 {
		t.Errorf("Kind: got %d want PairChangePan22", got.Kind)
	}
}

func TestParsePairFrame_BindAck(t *testing.T) {
	t.Parallel()
	// arbitrary outer wrap, A5 A5 at offsets 4..5, byte 9 sets RptOff.
	l2 := hx(t, "00 00 00 00 A5 A5 00 00 00 01 00 00")
	got, ok := ParsePairFrame(DirInbound, l2)
	if !ok {
		t.Fatalf("expected match")
	}
	if got.Kind != PairBindAck {
		t.Errorf("Kind: got %d want PairBindAck", got.Kind)
	}
	if !got.Bound {
		t.Error("Bound: want true")
	}
	if !got.RptOff {
		t.Error("RptOff: byte 9 == 1, want true")
	}
}

func TestParsePairFrame_BindAck_RptOffFalse(t *testing.T) {
	t.Parallel()
	l2 := hx(t, "00 00 00 00 A5 A5 00 00 00 00 00 00")
	got, ok := ParsePairFrame(DirInbound, l2)
	if !ok {
		t.Fatalf("expected match")
	}
	if got.RptOff {
		t.Error("RptOff: byte 9 == 0, want false")
	}
}

func TestParsePairFrame_ShortAddrReply(t *testing.T) {
	t.Parallel()
	// short_addr BE16 0x5011 at [0..1], 6-byte UID at [4..9].
	l2 := hx(t, "5011 0000 806000042582")
	got, ok := ParsePairFrame(DirInbound, l2)
	if !ok {
		t.Fatalf("expected match")
	}
	if got.Kind != PairShortAddrReply {
		t.Errorf("Kind: got %d want PairShortAddrReply", got.Kind)
	}
	if got.ShortAddr != 0x5011 {
		t.Errorf("ShortAddr: got 0x%X want 0x5011", got.ShortAddr)
	}
	if got.PeerUID != "806000042582" {
		t.Errorf("PeerUID: got %q want 806000042582", got.PeerUID)
	}
}

func TestParsePairFrame_ShortAddrReply_RejectsBroadcastSentinel(t *testing.T) {
	t.Parallel()
	// short_addr == 0xFFFF must not be matched as a discovery reply.
	l2 := hx(t, "FFFF 0000 806000042582")
	if _, ok := ParsePairFrame(DirInbound, l2); ok {
		t.Fatalf("expected no-match for short_addr >= 0xFFF8")
	}
}

func TestParsePairFrame_Outbound_BadMagic(t *testing.T) {
	t.Parallel()
	l2 := hx(t, "AAAABBAA 08 5011 0000 0000 A0 1234 00")
	if _, ok := ParsePairFrame(DirOutbound, l2); ok {
		t.Fatalf("expected no-match for wrong magic")
	}
}

func TestParsePairFrame_Outbound_UnknownOpcode(t *testing.T) {
	t.Parallel()
	l2 := hx(t, "AAAAAAAA 99 5011 0000 0000 A0 1234 00")
	if _, ok := ParsePairFrame(DirOutbound, l2); ok {
		t.Fatalf("expected no-match for unknown opcode")
	}
}

func TestParsePairFrame_Inbound_TooShort(t *testing.T) {
	t.Parallel()
	if _, ok := ParsePairFrame(DirInbound, []byte{1, 2, 3}); ok {
		t.Fatalf("expected no-match on short body")
	}
}

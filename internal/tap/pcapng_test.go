package tap

import (
	"encoding/binary"
	"testing"
	"time"
)

// readU32LE pulls a little-endian uint32 at offset.
func readU32LE(t *testing.T, b []byte, off int) uint32 {
	t.Helper()
	if off+4 > len(b) {
		t.Fatalf("short read at off=%d (len=%d)", off, len(b))
	}
	return binary.LittleEndian.Uint32(b[off : off+4])
}

func TestEncodeHeader_LayoutAndOrder(t *testing.T) {
	hdr := EncodeHeader(SectionInfo{Hardware: "h", OS: "o", UserAppl: "u"})
	if len(hdr) < 24 {
		t.Fatalf("header too short: %d bytes", len(hdr))
	}
	if got := readU32LE(t, hdr, 0); got != blockTypeSHB {
		t.Fatalf("first block is not SHB: got 0x%08x", got)
	}
	shbLen := readU32LE(t, hdr, 4)
	if int(shbLen) > len(hdr) {
		t.Fatalf("SHB total length %d exceeds header buffer %d", shbLen, len(hdr))
	}
	if got := readU32LE(t, hdr, int(shbLen)-4); got != shbLen {
		t.Fatalf("SHB trailer length mismatch: got %d want %d", got, shbLen)
	}
	if got := readU32LE(t, hdr, int(shbLen)); got != blockTypeIDB {
		t.Fatalf("second block is not IDB: got 0x%08x", got)
	}
	idb1Len := readU32LE(t, hdr, int(shbLen)+4)
	off2 := int(shbLen) + int(idb1Len)
	if got := readU32LE(t, hdr, off2); got != blockTypeIDB {
		t.Fatalf("third block is not IDB (need 2 IDBs for wire+inject): got 0x%08x", got)
	}
	idb2Len := readU32LE(t, hdr, off2+4)
	if int(shbLen+idb1Len+idb2Len) != len(hdr) {
		t.Fatalf("header total %d != SHB+IDB+IDB %d", len(hdr), shbLen+idb1Len+idb2Len)
	}
}

func TestEncodeEPB_PayloadFormat(t *testing.T) {
	chunk := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x55, 0x01}
	ts := time.Unix(1700000000, 123456000)
	blk := EncodeEPB(IfaceWire, DirToHost, chunk, ts)
	if got := readU32LE(t, blk, 8); got != IfaceWire {
		t.Fatalf("iface id: got %d want %d", got, IfaceWire)
	}
	blkInj := EncodeEPB(IfaceInject, DirToHost, chunk, ts)
	if got := readU32LE(t, blkInj, 8); got != IfaceInject {
		t.Fatalf("iface id: got %d want %d", got, IfaceInject)
	}

	if got := readU32LE(t, blk, 0); got != blockTypeEPB {
		t.Fatalf("block type: got 0x%08x want EPB", got)
	}
	total := readU32LE(t, blk, 4)
	if int(total) != len(blk) {
		t.Fatalf("block length: header says %d, buffer is %d", total, len(blk))
	}
	if got := readU32LE(t, blk, int(total)-4); got != total {
		t.Fatalf("trailer length mismatch: got %d want %d", got, total)
	}
	// EPB body starts at offset 8: interface(4) + tsHi(4) + tsLo(4) + capLen(4) + origLen(4) + payload
	capLen := readU32LE(t, blk, 8+12)
	origLen := readU32LE(t, blk, 8+16)
	if capLen != uint32(len(chunk)+1) || origLen != capLen {
		t.Fatalf("payload lengths: cap=%d orig=%d want %d", capLen, origLen, len(chunk)+1)
	}
	payloadStart := 8 + 20
	if blk[payloadStart] != DirToHost {
		t.Fatalf("direction byte: got 0x%02x want 0x%02x", blk[payloadStart], DirToHost)
	}
	for i, b := range chunk {
		if blk[payloadStart+1+i] != b {
			t.Fatalf("chunk byte %d: got 0x%02x want 0x%02x", i, blk[payloadStart+1+i], b)
		}
	}
}

func TestBroadcaster_LateSubscriberSeesHeaderOnly(t *testing.T) {
	br := NewBroadcaster(SectionInfo{Hardware: "h", OS: "o", UserAppl: "u"})
	// Publish before any subscriber: drops with no panic.
	br.Publish(DirToModem, []byte{1, 2, 3}, time.Now())

	w := &chunkSink{}
	detach, _ := br.Attach(w)

	// Drain header arrival.
	if !w.waitFor(t, 1, time.Second) {
		t.Fatal("subscriber never received header")
	}

	// Now publish a frame — subscriber should pick it up.
	br.Publish(DirToHost, []byte{0x55}, time.Now())
	if !w.waitFor(t, 2, time.Second) {
		t.Fatal("subscriber never received published frame")
	}

	detach()

	// After detach, further publishes are not delivered.
	br.Publish(DirToHost, []byte{0x99}, time.Now())
	time.Sleep(50 * time.Millisecond)
	if w.count() != 2 {
		t.Fatalf("subscriber got %d writes after detach; want 2", w.count())
	}
}

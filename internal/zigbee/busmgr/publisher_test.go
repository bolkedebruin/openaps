package busmgr

import (
	"bytes"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/zigbee/inventory"
	"github.com/bolkedebruin/openaps/wire"
)

// newOfflineClient builds a Client whose dialer is never started. The
// outbound channel still exists so Enqueue works, and we drain it
// directly via the package-private `out` field.
func newOfflineClient() *Client {
	return New("/nonexistent/busmgr-test.sock", "test-backend", "v-test")
}

func drainOne(t *testing.T, c *Client, timeout time.Duration) *wire.Envelope {
	t.Helper()
	select {
	case env := <-c.out:
		return env
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for envelope on out channel")
		return nil
	}
}

func TestPublisher_RawFrame_BuildsEnvelope(t *testing.T) {
	t.Parallel()

	c := newOfflineClient()
	p := &Publisher{C: c}

	l1 := []byte{0xFC, 0xFC, 0x50, 0x11, 0xDE, 0xAD, 0xBE, 0xEF, 0xFE, 0xFE}

	before := time.Now().UnixMilli()
	p.RawFrame(0x5011, l1)
	after := time.Now().UnixMilli()

	env := drainOne(t, c, time.Second)
	rf := env.GetRawFrame()
	if rf == nil {
		t.Fatalf("RawFrame payload is nil")
	}
	if env.GetHello() != nil || env.GetTelemetry() != nil || env.GetDecodeFailed() != nil {
		t.Errorf("other oneof variants set: hello=%v telemetry=%v decode_failed=%v",
			env.GetHello(), env.GetTelemetry(), env.GetDecodeFailed())
	}
	if rf.TsMs < before-1000 || rf.TsMs > after+1000 {
		t.Errorf("TsMs %d outside [%d, %d] window", rf.TsMs, before, after)
	}
	if rf.ShortAddr != 0x5011 {
		t.Errorf("ShortAddr = 0x%04x, want 0x5011", rf.ShortAddr)
	}
	if !bytes.Equal(rf.L1Frame, l1) {
		t.Errorf("L1Frame = %x, want %x", rf.L1Frame, l1)
	}
}

func TestPublisher_RawFrame_CopiesSlice(t *testing.T) {
	t.Parallel()

	c := newOfflineClient()
	p := &Publisher{C: c}

	buf := []byte{0xFC, 0xFC, 0xC4, 0x59, 0x01, 0x02, 0xFE, 0xFE}
	p.RawFrame(0xC459, buf)

	// Mutate the input. The enqueued envelope must not change.
	buf[4] = 0xFF
	buf[5] = 0xFF

	env := drainOne(t, c, time.Second)
	rf := env.GetRawFrame()
	if rf == nil {
		t.Fatal("nil RawFrame")
	}
	want := []byte{0xFC, 0xFC, 0xC4, 0x59, 0x01, 0x02, 0xFE, 0xFE}
	if !bytes.Equal(rf.L1Frame, want) {
		t.Errorf("L1Frame mutated: got %x want %x", rf.L1Frame, want)
	}
}

func TestPublisher_RawFrame_RecordsInventory(t *testing.T) {
	t.Parallel()

	c := newOfflineClient()
	inv := inventory.NewMap()
	p := &Publisher{C: c, Inventory: inv}

	// FC FC <SA hi> <SA lo> <RSSI> <LQI> <UID 6 bytes> ...
	l1 := []byte{0xFC, 0xFC, 0x50, 0x11, 0x44, 0x55,
		0x99, 0x99, 0x00, 0x00, 0x00, 0x03,
		0xFB, 0xFB, 0xFE, 0xFE}
	p.RawFrame(0x5011, l1)

	sa, ok := inv.LookupSA("999900000003")
	if !ok || sa != 0x5011 {
		t.Fatalf("inventory: got (0x%04X, %v) want (0x5011, true)", sa, ok)
	}
	// Envelope still queued.
	_ = drainOne(t, c, time.Second)
}

func TestPublisher_RawFrame_IgnoresShortFrame(t *testing.T) {
	t.Parallel()

	c := newOfflineClient()
	inv := inventory.NewMap()
	p := &Publisher{C: c, Inventory: inv}

	// Too short to contain a peer UID — must not panic and inventory
	// stays empty.
	p.RawFrame(0xC459, []byte{0xFC, 0xFC, 0xC4, 0x59, 0x01, 0x02})
	if _, ok := inv.LookupSA(""); ok {
		t.Fatal("empty uid should not be in inventory")
	}
	_ = drainOne(t, c, time.Second)
}

func TestPublisher_NilSafe(t *testing.T) {
	t.Parallel()
	// Nil publisher must not panic.
	var p *Publisher
	p.RawFrame(0x1234, []byte{0xFC, 0xFC})
	// Publisher with nil Client must not panic.
	p2 := &Publisher{}
	p2.RawFrame(0x1234, []byte{0xFC, 0xFC})
}

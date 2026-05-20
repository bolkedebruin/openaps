package ingest

import (
	"context"
	"testing"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/internal/events"
	"github.com/bolke/inv-driver/wire"
)

func TestFaultsFromReply_UnknownCmdReturnsNil(t *testing.T) {
	if got := faultsFromReply(codec.Reply{Cmd: 0xAA}); got != nil {
		t.Errorf("expected nil for unknown cmd, got %+v", got)
	}
}

func TestFaultsFromReply_DS3PreservesNamedBits(t *testing.T) {
	// body[0x0d] bit 4 → DCBusFault; body[0x0e] bit 4 → IsoFaultA.
	r := codec.Reply{Cmd: 0xBB, DS3Status: codec.DS3Status{Raw: [5]byte{
		0,          // 0x0b
		0,          // 0x0c
		1 << 4,     // 0x0d — DCBusFault
		1 << 4,     // 0x0e — IsoFaultA
		0,          // 0x0f
	}}}
	got := faultsFromReply(r)
	if got == nil {
		t.Fatal("expected non-nil InverterFaults")
	}
	ds3 := got.GetDs3()
	if ds3 == nil {
		t.Fatal("expected DS3 oneof variant")
	}
	if !ds3.GetDcBusFault() {
		t.Error("DcBusFault: want true")
	}
	if !ds3.GetIsoFaultA() {
		t.Error("IsoFaultA: want true")
	}
	if ds3.GetGridRelayFault() {
		t.Error("GridRelayFault: want false")
	}
}

func TestFaultsFromReply_QS1APreservesNamedBits(t *testing.T) {
	// body[0x17] bit 2 → GridRelayFault; body[0x1a] bit 6 → OverFreqFast.
	r := codec.Reply{Cmd: 0xB1, QS1AStatus: codec.QS1AStatus{
		Main: [4]byte{1 << 2, 0, 0, 1 << 6},
	}}
	got := faultsFromReply(r)
	if got == nil {
		t.Fatal("expected non-nil InverterFaults")
	}
	qs1a := got.GetQs1A()
	if qs1a == nil {
		t.Fatal("expected QS1A oneof variant")
	}
	if !qs1a.GetGridRelayFault() {
		t.Error("GridRelayFault: want true")
	}
	if !qs1a.GetOverFreqFast() {
		t.Error("OverFreqFast: want true")
	}
	if qs1a.GetIsoFaultA() {
		t.Error("IsoFaultA: want false")
	}
}

// Live DS3 + QS1A fixtures report all-zero status under normal
// operation. Verify the projection still attaches an InverterFaults
// envelope with the correct family — absence of a fault is itself a
// signal subscribers may want to observe.
func TestHandle_RawFrame_DS3_FaultsPublished(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	in.Pub = events.New()
	ch, unsub := in.Pub.Subscribe()
	defer unsub()

	env := &wire.Envelope{Body: &wire.Envelope_RawFrame{RawFrame: &wire.RawFrame{
		TsMs: 1, ShortAddr: 0x61F0, L1Frame: codec.DS3Fixture,
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	select {
	case got := <-ch:
		tel := got.GetTelemetry()
		if tel == nil || tel.GetFaults() == nil {
			t.Fatalf("no Faults on telemetry: %+v", got)
		}
		if tel.GetFaults().GetDs3() == nil {
			t.Errorf("expected DS3 family on Faults")
		}
	default:
		t.Fatal("nothing published")
	}
}

func TestHandle_RawFrame_QS1A_FaultsPublished(t *testing.T) {
	t.Parallel()
	in := newIngestor(t)
	in.Pub = events.New()
	ch, unsub := in.Pub.Subscribe()
	defer unsub()

	env := &wire.Envelope{Body: &wire.Envelope_RawFrame{RawFrame: &wire.RawFrame{
		TsMs: 1, ShortAddr: 0x5011, L1Frame: codec.QS1AFixture,
	}}}
	if err := in.Handle(context.Background(), "be", env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	select {
	case got := <-ch:
		tel := got.GetTelemetry()
		if tel == nil || tel.GetFaults() == nil {
			t.Fatalf("no Faults on telemetry: %+v", got)
		}
		if tel.GetFaults().GetQs1A() == nil {
			t.Errorf("expected QS1A family on Faults")
		}
	default:
		t.Fatal("nothing published")
	}
}

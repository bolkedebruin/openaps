package server

import (
	"bytes"
	"context"
	"testing"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
	"github.com/bolkedebruin/openaps/codec"
)

func newEnterServiceWriterFixture(t *testing.T, uid uint8, invUIDs ...string) (*EnterServiceWriter, *fakeSender) {
	t.Helper()
	fake := &fakeSender{}
	invs := make([]source.Inverter, 0, len(invUIDs))
	for _, u := range invUIDs {
		invs = append(invs, source.Inverter{UID: u, Online: true, TypeCode: "01", Model: 0x20})
	}
	return &EnterServiceWriter{
		uid:    uid,
		snap:   source.Snapshot{Inverters: invs},
		sender: fake,
	}, fake
}

// 60 seconds → uint32 = 0x0000003c → high=0, low=60.
func encodeUint32(v uint32) []uint16 {
	return []uint16{uint16(v >> 16), uint16(v & 0xFFFF)}
}

// grid_recovery_time (AG) on DS3 encodes to int(s*50) (sub 0x25) and
// goes through inv-driver.
func TestEnterServiceWriter_AG_Frame(t *testing.T) {
	esw, fake := newEnterServiceWriterFixture(t, 2, "INV-A")
	if err := esw.Apply(context.Background(), 7, encodeUint32(120)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want, err := codec.EncodeSetProtection(0x20, "grid_recovery_time", 120)
	if err != nil {
		t.Fatalf("EncodeSetProtection: %v", err)
	}
	if got := fake.frameFor("INV-A"); !bytes.Equal(got, want[0]) {
		t.Errorf("frame=% X want % X", got, want[0])
	}
}

func TestEnterServiceWriter_RangeRejection(t *testing.T) {
	esw, _ := newEnterServiceWriterFixture(t, 2, "INV-A")
	cases := []uint32{0, 9, 611, 1000}
	for _, sec := range cases {
		err := esw.Apply(context.Background(), 7, encodeUint32(sec))
		if err == nil {
			t.Errorf("seconds=%d: expected range error, got nil", sec)
		}
	}
}

func TestEnterServiceWriter_RegulatoryFieldRejected(t *testing.T) {
	esw, _ := newEnterServiceWriterFixture(t, 2, "INV-A")

	// Try to write ESVHi (offset 1) — must be rejected as a regulatory field.
	if err := esw.Apply(context.Background(), 1, []uint16{12000}); err == nil {
		t.Error("expected error for write to ESVHi (regulatory)")
	}
	// Try to write ESHzHi (offset 3..4, uint32) — must be rejected.
	if err := esw.Apply(context.Background(), 3, encodeUint32(5050)); err == nil {
		t.Error("expected error for write to ESHzHi (regulatory)")
	}
	// Partial ESDlyTms write (only high word) — reject.
	if err := esw.Apply(context.Background(), 7, []uint16{0}); err == nil {
		t.Error("expected error for partial ESDlyTms write (only high word)")
	}
}

func TestEnterServiceWriter_AggregateRejected(t *testing.T) {
	esw, _ := newEnterServiceWriterFixture(t, 1, "INV-A", "INV-B")
	if err := esw.Apply(context.Background(), 7, encodeUint32(120)); err == nil {
		t.Error("expected error for write to aggregate unit ID 1")
	}
}

func TestEnterServiceWriter_UnknownUnitRejected(t *testing.T) {
	esw, _ := newEnterServiceWriterFixture(t, 99, "INV-A")
	if err := esw.Apply(context.Background(), 7, encodeUint32(120)); err == nil {
		t.Error("expected error for unmapped unit ID")
	}
}

package server

import (
	"bytes"
	"context"
	"testing"

	"github.com/bolke/ecu-sunspec/internal/source"
	"github.com/bolke/ecu-sunspec/internal/sunspec"
	"github.com/bolke/inv-driver/codec"
)

func newFreqWattCurveWriterFixture(t *testing.T, uid uint8, model int, prot source.ProtectionParams, invUIDs ...string) (*FreqWattCurveWriter, *fakeSender) {
	t.Helper()
	fake := &fakeSender{}
	invs := make([]source.Inverter, 0, len(invUIDs))
	protMap := map[string]source.ProtectionParams{}
	for _, u := range invUIDs {
		invs = append(invs, source.Inverter{UID: u, Online: true, TypeCode: "01", Model: model})
		protMap[u] = prot
	}
	return &FreqWattCurveWriter{
		uid:    uid,
		snap:   source.Snapshot{Inverters: invs, Protection: protMap},
		sender: fake,
	}, fake
}

func curveProt() source.ProtectionParams {
	return source.ProtectionParams{
		OFFast:        52.0, // AK
		OFCurveUFLow:  47.0, // DH
		OFCurveUFHigh: 49.5, // DI
		OFCurveOFLow:  50.5, // CB
		OFDroopEnd:    52.0, // CC
		OFDroopMode:   13,
		Has: map[string]bool{
			"AK": true, "DH": true, "DI": true, "CB": true, "CC": true, "CV": true,
		},
	}
}

// Each writable Hz point encodes to the right per-family L2 frame and
// goes through inv-driver. Hz_SF=-2 → wire = Hz×100.
func TestFreqWattCurveWriter_Frames(t *testing.T) {
	cases := []struct {
		name  string
		model int
		off   uint16
		wire  uint16
		param string
		hz    float64
	}{
		{"DS3 Hz3 CB", 0x20, sunspec.FreqWattCurveBodyHz3Off, 5120, "Over_frequency_Watt_Low_set", 51.20},
		{"QS1A Hz3 CB", 0x18, sunspec.FreqWattCurveBodyHz3Off, 5050, "Over_frequency_Watt_Low_set", 50.50},
		{"QS1A Hz1 DH", 0x18, sunspec.FreqWattCurveBodyHz1Off, 4700, "Under_Frequency_Watt_Low_set", 47.00},
		{"DS3 Hz4 CC", 0x20, sunspec.FreqWattCurveBodyHz4Off, 5200, "Over_frequency_Watt_High_set", 52.00},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fwc, fake := newFreqWattCurveWriterFixture(t, 2, tc.model, curveProt(), "INV-A")
			if err := fwc.Apply(context.Background(), tc.off, []uint16{tc.wire}); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			want, err := codec.EncodeSetProtection(uint8(tc.model), tc.param, tc.hz)
			if err != nil {
				t.Fatalf("EncodeSetProtection: %v", err)
			}
			if got := fake.frameFor("INV-A"); !bytes.Equal(got, want[0]) {
				t.Errorf("frame=% X want % X", got, want[0])
			}
		})
	}
}

// Writing W (W%) is rejected — the curve roles fix W at 0/100/100/0.
func TestFreqWattCurveWriter_RejectsWWrite(t *testing.T) {
	fwc, fake := newFreqWattCurveWriterFixture(t, 2, 0x20, curveProt(), "INV-A")
	if err := fwc.Apply(context.Background(), sunspec.FreqWattCurveBodyW3Off, []uint16{80}); err == nil {
		t.Error("expected rejection on W3 write")
	}
	if len(fake.frames()) != 0 {
		t.Error("must not send when W% write is rejected")
	}
}

func TestFreqWattCurveWriter_RejectsHeaderWrites(t *testing.T) {
	fwc, _ := newFreqWattCurveWriterFixture(t, 2, 0x20, curveProt(), "INV-A")
	if err := fwc.Apply(context.Background(), 0 /* ActCrv */, []uint16{2}); err == nil {
		t.Error("expected error on ActCrv write")
	}
}

func TestFreqWattCurveWriter_AggregateRejected(t *testing.T) {
	fwc, _ := newFreqWattCurveWriterFixture(t, 1, 0x20, curveProt(), "INV-A", "INV-B")
	if err := fwc.Apply(context.Background(), sunspec.FreqWattCurveBodyHz3Off, []uint16{5120}); err == nil {
		t.Error("expected error for aggregate uid 1")
	}
}

// Unknown family → RouteUnsupported, nothing sent.
func TestFreqWattCurveWriter_UnknownFamilyRejected(t *testing.T) {
	fwc, fake := newFreqWattCurveWriterFixture(t, 2, 999, curveProt(), "INV-A")
	if err := fwc.Apply(context.Background(), sunspec.FreqWattCurveBodyHz3Off, []uint16{5120}); err == nil {
		t.Error("expected error for unknown family")
	}
	if len(fake.frames()) != 0 {
		t.Error("must not send for unknown family")
	}
}

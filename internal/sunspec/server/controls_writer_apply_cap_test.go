package server

import (
	"bytes"
	"context"
	"testing"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
	"github.com/bolkedebruin/openaps/internal/sunspec/sunspec"
	"github.com/bolkedebruin/openaps/codec"
)

// applyCapCase exercises ControlsWriter.applyCap in isolation, verifying
// the SF=-2 contract: raw 0..10000 → 0..100.00% of (nameplate/panels) per
// panel, clamped to [MinPanelLimitW, MaxPanelLimitW], then encoded as the
// family set-power frame and dispatched to inv-driver.
type applyCapCase struct {
	name   string
	inv    source.Inverter
	rawPct uint16
	wantW  int
}

func TestApplyCap_NameplateAwareWithFloorAndCeiling(t *testing.T) {
	// TypeCode "01" → DS3 (model 0x20): NameplateW=730, panels=2 → 365 W/panel.
	// TypeCode "03" → QS1 (model 0x18): NameplateW=1460, panels=4 → 365 W/panel.
	// TypeCode "04" → DS3-H (model 0x21): NameplateW=880, panels=2 → 440 W/panel.
	cases := []applyCapCase{
		{name: "DS3 full scale", inv: source.Inverter{TypeCode: "01", Model: 0x20}, rawPct: 10000, wantW: 365},
		{name: "DS3 80pct", inv: source.Inverter{TypeCode: "01", Model: 0x20}, rawPct: 8000, wantW: 292},
		{name: "DS3 20pct", inv: source.Inverter{TypeCode: "01", Model: 0x20}, rawPct: 2000, wantW: 73},
		{name: "DS3 sub-floor clamps up", inv: source.Inverter{TypeCode: "01", Model: 0x20},
			rawPct: 100, wantW: source.MinPanelLimitW},
		{name: "DS3 above max raw clamps to 10000", inv: source.Inverter{TypeCode: "01", Model: 0x20},
			rawPct: 15000, wantW: 365},
		{name: "QS1 full scale", inv: source.Inverter{TypeCode: "03", Model: 0x18}, rawPct: 10000, wantW: 365},
		{name: "DS3-H full scale per-panel 440", inv: source.Inverter{TypeCode: "04", Model: 0x21},
			rawPct: 10000, wantW: 440},
		{name: "DS3-H 80pct under ceiling", inv: source.Inverter{TypeCode: "04", Model: 0x21},
			rawPct: 8000, wantW: 352},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.inv.UID = "INV-A"
			fake := &fakeSender{}
			cw := &ControlsWriter{
				uid:    2,
				snap:   source.Snapshot{Inverters: []source.Inverter{c.inv}},
				sender: fake,
			}
			if err := cw.applyCap(context.Background(), []string{"INV-A"}, c.rawPct); err != nil {
				t.Fatalf("applyCap: %v", err)
			}
			want, err := codec.EncodeSetPower(uint8(c.inv.Model), uint16(c.wantW), false)
			if err != nil {
				t.Fatalf("EncodeSetPower: %v", err)
			}
			got := fake.frameFor("INV-A")
			if !bytes.Equal(got, want) {
				t.Errorf("frame=% X want % X (rawPct=%d → %d W/panel, type=%s)",
					got, want, c.rawPct, c.wantW, c.inv.TypeCode)
			}
		})
	}
}

func TestApplyCap_RejectsUnknownInverter(t *testing.T) {
	cw := &ControlsWriter{
		uid:    2,
		snap:   source.Snapshot{Inverters: nil},
		sender: &fakeSender{},
	}
	if err := cw.applyCap(context.Background(), []string{"NO-SUCH"}, 5000); err == nil {
		t.Fatal("expected error for unknown UID, got nil")
	}
}

func TestApplyCap_RejectsZeroNameplate(t *testing.T) {
	cw := &ControlsWriter{
		uid: 2,
		snap: source.Snapshot{Inverters: []source.Inverter{
			{UID: "INV-A", TypeCode: "ZZ"}, // unknown TypeCode → NameplateW=0
		}},
		sender: &fakeSender{},
	}
	if err := cw.applyCap(context.Background(), []string{"INV-A"}, 5000); err == nil {
		t.Fatal("expected error for zero nameplate, got nil")
	}
}

// TestControlsEmit_WMaxLimPctSF verifies the emitted Model 123 body
// advertises WMaxLimPctSF=-2 at body offset 21.
func TestControlsEmit_WMaxLimPctSF(t *testing.T) {
	if sunspec.WMaxLimPctSF != -2 {
		t.Errorf("WMaxLimPctSF constant = %d, want -2", sunspec.WMaxLimPctSF)
	}
	if sunspec.OffControlsWMaxLimPctSF != 21 {
		t.Errorf("OffControlsWMaxLimPctSF = %d, want 21", sunspec.OffControlsWMaxLimPctSF)
	}
}

package source

import (
	"context"
	"testing"

	"github.com/bolkedebruin/openaps/wire"
)

func TestQS2SunSpecFourChannels(t *testing.T) {
	typeCode := typeCodeFromModelCode(0x36)

	inv := Inverter{
		Online:     true,
		TypeCode:   typeCode,
		Model:      0x36,
		PanelWatts: []int{25, 30, 1, 1},
	}

	if got := inv.PanelCount(); got != 4 {
		t.Errorf("QS2 panel count = %d, want 4", got)
	}

	if got := len(inv.PanelPowers()); got != 4 {
		t.Errorf("QS2 SunSpec panel powers = %d, want 4", got)
	}
}

// QS2 panel voltage and current must survive the telemetry-to-SunSpec
// snapshot conversion without being replaced by AC measurements.
func TestQS2SnapshotPreservesDCMeasurements(t *testing.T) {
	tel := &wire.Telemetry{
		PeerUid: "821000011029",
		Model:   "QS2",
		GridV:   231.0,
		Panels: []*wire.Panel{
			{Index: 0, DcV: 41.4, DcI: 1.02, W: 42},
			{Index: 1, DcV: 41.6, DcI: 1.18, W: 49},
			{Index: 2, DcV: 41.3, DcI: 1.03, W: 43},
			{Index: 3, DcV: 41.9, DcI: 0.90, W: 38},
		},
	}

	client := startSeededClient(t, tel)
	builder := &Builder{InvDriverClient: client}

	snapshot, err := builder.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Inverters) != 1 {
		t.Fatalf("inverters = %d, want 1", len(snapshot.Inverters))
	}

	inv := snapshot.Inverters[0]
	if len(inv.PanelDCVolts) != 4 || len(inv.PanelDCAmps) != 4 {
		t.Fatalf("DC measurement counts: volts=%d amps=%d, want 4 each",
			len(inv.PanelDCVolts), len(inv.PanelDCAmps))
	}

	if inv.PanelDCVolts[0] != 41.4 || inv.PanelDCAmps[0] != 1.02 {
		t.Errorf("panel 1: voltage=%v current=%v, want 41.4 V / 1.02 A",
			inv.PanelDCVolts[0], inv.PanelDCAmps[0])
	}
}

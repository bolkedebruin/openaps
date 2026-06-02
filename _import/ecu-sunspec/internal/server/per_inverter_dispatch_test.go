package server

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/bolke/ecu-sunspec/internal/source"
	"github.com/bolke/ecu-sunspec/internal/sunspec"
	"github.com/simonvetter/modbus"
)

func TestServer_PerInverterUnitIDs(t *testing.T) {
	snap := source.Snapshot{
		ECUID:        "999999999999",
		SystemPowerW: 300, // aggregate W (the Builder sums per-inverter; we set directly here)
		Inverters: []source.Inverter{
			{UID: "999900000001", Online: true, TypeCode: "01", Phase: 1,
				ACPowerW: 100, ACVoltageV: 230, FrequencyHz: 50, TemperatureC: 30,
				PanelWatts: []int{50, 50}},
			{UID: "999900000002", Online: true, TypeCode: "03", Phase: 1,
				ACPowerW: 200, ACVoltageV: 230, FrequencyHz: 50, TemperatureC: 35,
				PanelWatts: []int{50, 50, 50, 50}},
		},
	}
	port := freePort(t)
	srv := New(fixedProvider{snap}, Config{
		URL:             "tcp://127.0.0.1:" + strconv.Itoa(port),
		RefreshInterval: time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop()

	cli, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:     "tcp://127.0.0.1:" + strconv.Itoa(port),
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := cli.Open(); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer cli.Close()

	// Each unit ID returns a distinct bank with its own SN in the Common Model.
	cases := []struct {
		uid    uint8
		wantSN string
		wantW  int16
	}{
		{1, "999999999999", 300}, // aggregate (system total)
		{2, "999900000001", 100}, // per-inverter A
		{3, "999900000002", 200}, // per-inverter B
	}
	for _, c := range cases {
		if err := cli.SetUnitId(c.uid); err != nil {
			t.Fatalf("SetUnitId %d: %v", c.uid, err)
		}

		// SN at base+52, 16 regs
		sn, err := cli.ReadRegisters(sunspec.BaseRegister+52, 16, modbus.HOLDING_REGISTER)
		if err != nil {
			t.Fatalf("uid=%d SN read: %v", c.uid, err)
		}
		gotSN := decodeStr(sn)
		if gotSN != c.wantSN {
			t.Errorf("uid=%d: SN=%q want %q", c.uid, gotSN, c.wantSN)
		}

		// W field at Inverter-101 body+12. Bank layout differs between aggregate
		// (lots of models in front) and per-inverter (just Common+101). Walk to
		// find it.
		// For per-inverter banks the inverter body starts at base+70; for aggregate
		// it's also base+70 (Common is same length). Use a fixed offset.
		w, err := cli.ReadRegisters(sunspec.BaseRegister+70+2+12, 1, modbus.HOLDING_REGISTER)
		if err != nil {
			t.Fatalf("uid=%d W read: %v", c.uid, err)
		}
		if int16(w[0]) != c.wantW {
			t.Errorf("uid=%d: W=%d want %d", c.uid, int16(w[0]), c.wantW)
		}
	}
}

func decodeStr(regs []uint16) string {
	b := make([]byte, 0, len(regs)*2)
	for _, r := range regs {
		b = append(b, byte(r>>8), byte(r))
	}
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] != 0 {
			return string(b[:i+1])
		}
	}
	return ""
}

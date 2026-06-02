package server

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
	"github.com/bolkedebruin/openaps/internal/sunspec/sunspec"
	"github.com/simonvetter/modbus"
)

type fixedProvider struct{ snap source.Snapshot }

func (f fixedProvider) Build(context.Context) (source.Snapshot, error) {
	return f.snap, nil
}

func TestServer_ServesSunSpecBank(t *testing.T) {
	snap := source.Snapshot{
		ECUID:               "999999999999",
		Firmware:            "2.1.29D",
		Model:               "ECU_R_PRO",
		SystemPowerW:        500,
		LifetimeEnergyWh:    15_000_000,
		GridFrequencyHz:     49.99,
		GridVoltageV:        232,
		MaxTemperatureC:     28,
		InverterCount:       3,
		InverterOnlineCount: 3,
		Inverters: []source.Inverter{
			{UID: "A", Online: true, TypeCode: "01", ACPowerW: 200,
				ACVoltageV: 232, Phase: 1, PanelWatts: []int{100, 100}},
			{UID: "B", Online: true, TypeCode: "01", ACPowerW: 200,
				ACVoltageV: 232, Phase: 1, PanelWatts: []int{100, 100}},
			{UID: "C", Online: true, TypeCode: "01", ACPowerW: 100,
				ACVoltageV: 232, Phase: 1, PanelWatts: []int{50, 50}},
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
		t.Fatalf("client new: %v", err)
	}
	if err := cli.Open(); err != nil {
		t.Fatalf("client open: %v", err)
	}
	defer cli.Close()

	regs, err := cli.ReadRegisters(sunspec.BaseRegister, 4, modbus.HOLDING_REGISTER)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if regs[0] != 0x5375 || regs[1] != 0x6E53 {
		t.Errorf("magic = %#04x %#04x want 0x5375 0x6E53", regs[0], regs[1])
	}
	if regs[2] != 1 || regs[3] != 66 {
		t.Errorf("common header = %d/%d want 1/66", regs[2], regs[3])
	}

	// Read the inverter model header.
	inv, err := cli.ReadRegisters(sunspec.BaseRegister+70, 2, modbus.HOLDING_REGISTER)
	if err != nil {
		t.Fatalf("inverter read: %v", err)
	}
	// Snapshot has all inverters on Phase 1 → auto-detect picks Model 101.
	if inv[0] != 101 || inv[1] != 50 {
		t.Errorf("inverter header = %d/%d want 101/50", inv[0], inv[1])
	}

	// Read W (body+12) and verify.
	w, err := cli.ReadRegisters(sunspec.BaseRegister+70+2+12, 1, modbus.HOLDING_REGISTER)
	if err != nil {
		t.Fatalf("W read: %v", err)
	}
	if int16(w[0]) != 500 {
		t.Errorf("W=%d want 500", int16(w[0]))
	}

	// End marker — walk the bank to find it instead of hardcoding the offset
	// (which now shifts as we add Models 120, 121, 160, vendor 64202, etc.).
	cur := uint16(sunspec.BaseRegister + 2)
	for hops := 0; hops < 50; hops++ {
		hdr, err := cli.ReadRegisters(cur, 2, modbus.HOLDING_REGISTER)
		if err != nil {
			t.Fatalf("walk read at %d: %v", cur, err)
		}
		if hdr[0] == 0xFFFF {
			if hdr[1] != 0 {
				t.Errorf("end L=%d want 0", hdr[1])
			}
			return
		}
		cur += 2 + hdr[1]
	}
	t.Error("did not reach end marker after 50 model hops")
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

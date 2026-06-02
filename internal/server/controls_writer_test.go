package server

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/bolke/ecu-sunspec/internal/config"
	"github.com/bolke/ecu-sunspec/internal/source"
	"github.com/bolke/ecu-sunspec/internal/sunspec"
	"github.com/bolke/inv-driver/codec"
	"github.com/simonvetter/modbus"
)

// ds3Inv builds a DS3 (TypeCode 01, model 0x20) snapshot inverter:
// nameplate 730 W over 2 panels → 365 W/panel.
func ds3Inv(uid string) source.Inverter {
	return source.Inverter{
		UID: uid, Online: true, TypeCode: "01", Model: 0x20, Phase: 1,
		ACPowerW: 50, ACVoltageV: 230, PanelWatts: []int{25, 25},
	}
}

func TestServer_WriteWMaxLimPct_PerInverter(t *testing.T) {
	fake := &fakeSender{}

	snap := source.Snapshot{
		ECUID:        "999999999999",
		SystemPowerW: 100,
		Inverters:    []source.Inverter{ds3Inv("INV-A"), ds3Inv("INV-B")},
	}

	port := freePort(t)
	srv := New(fixedProvider{snap}, Config{
		URL:             "tcp://127.0.0.1:" + strconv.Itoa(port),
		RefreshInterval: time.Second,
		InvDriver:       fake,
		Writes: config.Config{Writes: config.WritesConfig{
			Enabled:   config.BoolPtr(true),
			AllowList: nil, // loopback always allowed
		}},
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
		t.Fatal(err)
	}
	if err := cli.Open(); err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// Walk the per-inverter bank for INV-A (uid 2) to find Model 123.
	cli.SetUnitId(2)
	ctlAddr, ok := walkForModel(cli, sunspec.ControlsModelID)
	if !ok {
		t.Fatal("Model 123 missing from per-inverter bank")
	}
	body := ctlAddr + 2

	// Write WMaxLim_Pct=8000 (= 80.00% under SF=-2) + WMaxLim_Ena=1,
	// targeting offsets 3..7 so Conn is left untouched. DS3 nameplate
	// 730 W / 2 panels → 365 W/panel; 80% = 292 W per panel.
	regs := []uint16{
		8000, // WMaxLim_Pct  (raw, SF=-2 → 80.00%)
		0,    // WMaxLimPct_WinTms
		0,    // WMaxLimPct_RvrtTms
		0,    // WMaxLimPct_RmpTms
		1,    // WMaxLim_Ena
	}
	if err := cli.WriteRegisters(body+sunspec.OffControlsWMaxLimPct, regs); err != nil {
		t.Fatalf("write: %v", err)
	}

	// INV-A: a DS3 292 W/panel set-power frame went to inv-driver.
	want, err := codec.EncodeSetPower(0x20, 292, false)
	if err != nil {
		t.Fatalf("EncodeSetPower: %v", err)
	}
	if got := fake.frameFor("INV-A"); !bytes.Equal(got, want) {
		t.Errorf("INV-A frame=% X want % X (80%% of 365 W/panel)", got, want)
	}

	// INV-B should be untouched (per-inverter write only hits INV-A).
	if got := fake.frameFor("INV-B"); got != nil {
		t.Errorf("INV-B should be untouched, got frame % X", got)
	}
}

func TestServer_WriteRejectedWhenDisabled(t *testing.T) {
	fake := &fakeSender{}

	snap := source.Snapshot{Inverters: []source.Inverter{ds3Inv("INV-A")}}
	port := freePort(t)
	srv := New(fixedProvider{snap}, Config{
		URL:       "tcp://127.0.0.1:" + strconv.Itoa(port),
		InvDriver: fake,
		Writes:    config.Config{Writes: config.WritesConfig{Enabled: config.BoolPtr(false)}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	cli, _ := modbus.NewClient(&modbus.ClientConfiguration{
		URL:     "tcp://127.0.0.1:" + strconv.Itoa(port),
		Timeout: 2 * time.Second,
	})
	cli.Open()
	defer cli.Close()
	cli.SetUnitId(2)

	ctlAddr, _ := walkForModel(cli, sunspec.ControlsModelID)
	if err := cli.WriteRegister(ctlAddr+2+sunspec.OffControlsWMaxLimPct, 50); err == nil {
		t.Error("expected write to fail (writes disabled), got nil error")
	}
	if got := fake.frames(); len(got) != 0 {
		t.Errorf("writes disabled but %d frame(s) sent", len(got))
	}
}

func TestServer_WriteConn_TurnsOff(t *testing.T) {
	fake := &fakeSender{}

	snap := source.Snapshot{Inverters: []source.Inverter{ds3Inv("INV-A")}}
	port := freePort(t)
	srv := New(fixedProvider{snap}, Config{
		URL:       "tcp://127.0.0.1:" + strconv.Itoa(port),
		InvDriver: fake,
		Writes:    config.Config{Writes: config.WritesConfig{Enabled: config.BoolPtr(true)}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.Start(ctx)
	defer srv.Stop()

	cli, _ := modbus.NewClient(&modbus.ClientConfiguration{
		URL: "tcp://127.0.0.1:" + strconv.Itoa(port), Timeout: 2 * time.Second,
	})
	cli.Open()
	defer cli.Close()
	cli.SetUnitId(2)

	ctlAddr, _ := walkForModel(cli, sunspec.ControlsModelID)
	if err := cli.WriteRegister(ctlAddr+2+sunspec.OffControlsConn, 0); err != nil {
		t.Fatal(err)
	}

	// Conn=0 → an off frame (family-independent) went to inv-driver.
	want := codec.EncodeOnOff(false, false)
	if got := fake.frameFor("INV-A"); !bytes.Equal(got, want) {
		t.Errorf("INV-A frame=% X want % X (off)", got, want)
	}
}

// walkForModel scans the standard SunSpec bank starting at 40000 looking for
// the given model ID. Returns the absolute address of the model's ID register
// (i.e., body starts at +2).
func walkForModel(cli *modbus.ModbusClient, id uint16) (uint16, bool) {
	addr := uint16(sunspec.BaseRegister + 2) // skip "SunS"
	for hop := 0; hop < 50; hop++ {
		hdr, err := cli.ReadRegisters(addr, 2, modbus.HOLDING_REGISTER)
		if err != nil {
			return 0, false
		}
		if hdr[0] == sunspec.EndModelID {
			return 0, false
		}
		if hdr[0] == id {
			return addr, true
		}
		addr += 2 + hdr[1]
	}
	return 0, false
}

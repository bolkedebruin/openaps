package source

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/bolke/ecu-sunspec/internal/source/invdriver"
	"github.com/bolke/ecu-sunspec/internal/testhelpers"
	"github.com/bolke/inv-driver/wire"
)

// helper: spin up a UDS server, push a Telemetry frame, return the
// connected client once it has the snapshot cached.
func startSeededClient(t *testing.T, tel *wire.Telemetry) *invdriver.Client {
	t.Helper()
	dir := testhelpers.ShortTempDir(t)
	sock := filepath.Join(dir, "ic.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	connCh := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		var env wire.Envelope
		_ = wire.ReadFrame(c, &env) // Hello
		connCh <- c
	}()

	c := invdriver.New(sock, "test")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(c.Stop)

	select {
	case conn := <-connCh:
		if err := wire.WriteFrame(conn, &wire.Envelope{
			Body: &wire.Envelope_Telemetry{Telemetry: tel},
		}); err != nil {
			t.Fatalf("send: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client connect")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := c.Get(tel.GetPeerUid()); ok {
			return c
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client never received telemetry for %q", tel.GetPeerUid())
	return nil
}

// startSeededClientWithFleet drives the fake UDS server with a
// single FleetSummary envelope, waits for it to land in the cache,
// returns the connected client.
func startSeededClientWithFleet(t *testing.T, fleet *wire.FleetSummary) *invdriver.Client {
	t.Helper()
	dir := testhelpers.ShortTempDir(t)
	sock := filepath.Join(dir, "ic.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	connCh := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		var env wire.Envelope
		_ = wire.ReadFrame(c, &env) // Hello
		connCh <- c
	}()

	c := invdriver.New(sock, "test")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(c.Stop)

	select {
	case conn := <-connCh:
		if err := wire.WriteFrame(conn, &wire.Envelope{
			Body: &wire.Envelope_Fleet{Fleet: fleet},
		}); err != nil {
			t.Fatalf("send fleet: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client connect")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Fleet() != nil {
			return c
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client never received FleetSummary")
	return nil
}

func TestBuilder_InvDriverSource_RSSIFromTelemetry(t *testing.T) {
	cases := []struct {
		name string
		rssi uint32
		want int
	}{
		{"normal RSSI byte", 0xCE, 0xCE},
		{"clamped tiny value", 0x05, 0xFF},
		{"clamp boundary 0x0f", 0x0F, 0xFF},
		{"clamp boundary 0x10", 0x10, 0x10},
		{"zero stays zero (no signal yet)", 0x00, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := startSeededClient(t, &wire.Telemetry{
				PeerUid: "ABCDEF012345", Model: "DS3",
				Rssi: tc.rssi,
			})
			b := &Builder{InvDriverClient: c, InvDriverOnlineMaxAge: 5 * time.Second}
			snap, err := b.Build(context.Background())
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if len(snap.Inverters) != 1 {
				t.Fatalf("inverters=%d", len(snap.Inverters))
			}
			if snap.Inverters[0].SignalStrength != tc.want {
				t.Errorf("SignalStrength=%d want %d", snap.Inverters[0].SignalStrength, tc.want)
			}
		})
	}
}

func TestBuilder_InvDriverSource_FleetSummaryFillsSystemMaxPowerW(t *testing.T) {
	c := startSeededClientWithFleet(t, &wire.FleetSummary{
		NameplateTotalW: 3950,
		InverterCount:   3,
	})
	b := &Builder{InvDriverClient: c, InvDriverOnlineMaxAge: 5 * time.Second}
	snap, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if snap.SystemMaxPowerW != 3950 {
		t.Errorf("SystemMaxPowerW=%d want 3950", snap.SystemMaxPowerW)
	}
}

func TestBuilder_InvDriverSource_PopulatesInverter(t *testing.T) {
	tel := &wire.Telemetry{
		TsMs:          time.Now().UnixMilli(),
		PeerUid:       "AABBCCDDEEFF",
		Model:         "QS1A",
		ActivePowerW:  486.4,
		GridV:         231.7,
		FreqHz:        50.02,
		LifetimeRaw:   []uint64{1000, 1500, 500, 250},
		LifetimeScale: 2.0, // total 6500 Wh
		Panels: []*wire.Panel{
			{Index: 0, W: 121},
			{Index: 1, W: 122},
			{Index: 2, W: 122},
			{Index: 3, W: 121},
		},
	}
	c := startSeededClient(t, tel)

	b := &Builder{InvDriverClient: c, InvDriverOnlineMaxAge: 5 * time.Second}
	snap, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(snap.Inverters) != 1 {
		t.Fatalf("inverters=%d want 1", len(snap.Inverters))
	}
	inv := snap.Inverters[0]
	if inv.UID != "AABBCCDDEEFF" {
		t.Errorf("UID=%q", inv.UID)
	}
	if !inv.Online {
		t.Error("Online=false on fresh frame")
	}
	if inv.TypeCode != "03" {
		t.Errorf("TypeCode=%q want 03 (QS1A)", inv.TypeCode)
	}
	if inv.ACPowerW != 486 {
		t.Errorf("ACPowerW=%d want 486", inv.ACPowerW)
	}
	if inv.ACVoltageV != 232 {
		t.Errorf("ACVoltageV=%d want 232", inv.ACVoltageV)
	}
	if inv.FrequencyHz < 50.0 || inv.FrequencyHz > 50.1 {
		t.Errorf("FrequencyHz=%v", inv.FrequencyHz)
	}
	if got := inv.PanelPowers(); len(got) != 4 {
		t.Errorf("PanelPowers len=%d want 4: %v", len(got), got)
	}
	if snap.SystemPowerW != 486 {
		t.Errorf("SystemPowerW=%d want 486", snap.SystemPowerW)
	}
	// LifetimeEnergyWh is sourced from inv-driver's FleetSummary
	// envelope (not from the per-inverter Telemetry sum). This test
	// only seeds a Telemetry frame, so the field stays at zero.
	if snap.GridFrequencyHz < 50.0 || snap.GridFrequencyHz > 50.1 {
		t.Errorf("GridFrequencyHz=%v", snap.GridFrequencyHz)
	}
	if snap.InverterOnlineCount != 1 {
		t.Errorf("online count=%d want 1", snap.InverterOnlineCount)
	}
}

func TestBuilder_InvDriverSource_StaleMarksOffline(t *testing.T) {
	tel := &wire.Telemetry{
		PeerUid:      "1234567890AB",
		Model:        "DS3",
		ActivePowerW: 200,
		GridV:        230,
		FreqHz:       50.0,
	}
	c := startSeededClient(t, tel)

	b := &Builder{InvDriverClient: c, InvDriverOnlineMaxAge: 1 * time.Nanosecond}
	time.Sleep(2 * time.Millisecond)
	snap, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(snap.Inverters) != 1 {
		t.Fatalf("inverters=%d", len(snap.Inverters))
	}
	if snap.Inverters[0].Online {
		t.Error("expected Online=false past staleness threshold")
	}
	if snap.SystemPowerW != 0 {
		t.Errorf("SystemPowerW=%d want 0 (offline)", snap.SystemPowerW)
	}
}

// startSeededClientWithInfo drives the fake UDS server with one
// Telemetry frame and one InverterInfo frame in sequence, and waits
// for the client to cache identity for the UID. Returned client is
// stopped via t.Cleanup.
func startSeededClientWithInfo(t *testing.T, tel *wire.Telemetry, info *wire.InverterInfo) *invdriver.Client {
	t.Helper()
	dir := testhelpers.ShortTempDir(t)
	sock := filepath.Join(dir, "ic.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	connCh := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		var env wire.Envelope
		_ = wire.ReadFrame(c, &env) // Hello
		connCh <- c
	}()

	c := invdriver.New(sock, "test")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(c.Stop)

	select {
	case conn := <-connCh:
		if tel != nil {
			if err := wire.WriteFrame(conn, &wire.Envelope{
				Body: &wire.Envelope_Telemetry{Telemetry: tel},
			}); err != nil {
				t.Fatalf("send telemetry: %v", err)
			}
		}
		if info != nil {
			if err := wire.WriteFrame(conn, &wire.Envelope{
				Body: &wire.Envelope_Info{Info: info},
			}); err != nil {
				t.Fatalf("send info: %v", err)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client connect")
	}

	uid := ""
	switch {
	case info != nil:
		uid = info.GetPeerUid()
	case tel != nil:
		uid = tel.GetPeerUid()
	}
	if uid == "" {
		return c
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s, ok := c.Get(uid)
		if ok && (info == nil || s.ModelCode != nil || s.Phase != nil || s.SoftwareVersion != nil) {
			return c
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client never received identity for %q", uid)
	return nil
}

// TestBuilder_InvDriverSource_QS1AInfoPopulatesPhase verifies the
// per-leg Phase from an InverterInfo envelope flows through to the
// produced Inverter, and that the QS1A model code maps to TypeCode 03.
func TestBuilder_InvDriverSource_QS1AInfoPopulatesPhase(t *testing.T) {
	tel := &wire.Telemetry{
		PeerUid:      "QS1AINFOUID0",
		Model:        "QS1A",
		ActivePowerW: 480,
		GridV:        231,
		FreqHz:       50.0,
	}
	mc := uint32(0x18) // QS1A
	sv := uint32(0x010203)
	ph := uint32(1)
	info := &wire.InverterInfo{
		PeerUid:         "QS1AINFOUID0",
		ModelCode:       &mc,
		SoftwareVersion: &sv,
		Phase:           &ph,
	}
	c := startSeededClientWithInfo(t, tel, info)

	b := &Builder{InvDriverClient: c, InvDriverOnlineMaxAge: 5 * time.Second}
	snap, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(snap.Inverters) != 1 {
		t.Fatalf("inverters=%d want 1", len(snap.Inverters))
	}
	inv := snap.Inverters[0]
	if inv.Phase != 1 {
		t.Errorf("Phase=%d want 1 (per-leg)", inv.Phase)
	}
	if inv.Model != 0x18 {
		t.Errorf("Model=0x%x want 0x18 (QS1A)", inv.Model)
	}
	if inv.SoftwareVer != 0x010203 {
		t.Errorf("SoftwareVer=0x%x want 0x010203", inv.SoftwareVer)
	}
	if inv.TypeCode != "03" {
		t.Errorf("TypeCode=%q want 03 (QS1A)", inv.TypeCode)
	}
}

// TestBuilder_InvDriverSource_ThreePhaseModelWithoutLeg covers the
// three-phase case where Info carries a model_code but no Phase
// (operator-side per-leg config hasn't arrived yet). The per-leg
// inv.Phase stays zero and the model code is preserved for the
// downstream encoder.
func TestBuilder_InvDriverSource_ThreePhaseModelWithoutLeg(t *testing.T) {
	tel := &wire.Telemetry{
		PeerUid:      "3PHASEUID000",
		Model:        "",
		ActivePowerW: 1200,
		GridV:        230,
		FreqHz:       50.0,
	}
	mc := uint32(0x29) // three-phase code per codec.PhaseFromModel
	sv := uint32(0x040506)
	info := &wire.InverterInfo{
		PeerUid:         "3PHASEUID000",
		ModelCode:       &mc,
		SoftwareVersion: &sv,
	}
	c := startSeededClientWithInfo(t, tel, info)

	b := &Builder{InvDriverClient: c, InvDriverOnlineMaxAge: 5 * time.Second}
	snap, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(snap.Inverters) != 1 {
		t.Fatalf("inverters=%d want 1", len(snap.Inverters))
	}
	inv := snap.Inverters[0]
	if inv.Phase != 0 {
		t.Errorf("Phase=%d want 0 (unset; per-leg not assigned)", inv.Phase)
	}
	if inv.Model != 0x29 {
		t.Errorf("Model=0x%x want 0x29", inv.Model)
	}
}

// TestBuilder_InvDriverSource_LifetimeFromFleetSummary verifies the
// FleetSummary envelope drives all four energy fields (lifetime,
// today, month, year) into the Snapshot — replacing the prior
// SQLite-based path entirely.
func TestBuilder_InvDriverSource_LifetimeFromFleetSummary(t *testing.T) {
	c := startSeededClientWithFleet(t, &wire.FleetSummary{
		NameplateTotalW: 3950,
		InverterCount:   3,
		LifetimeWh:      1_234_567,
		TodayWh:         8_900,
		MonthWh:         234_000,
		YearWh:          1_100_000,
	})
	b := &Builder{InvDriverClient: c, InvDriverOnlineMaxAge: 5 * time.Second}
	snap, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if snap.LifetimeEnergyWh != 1_234_567 {
		t.Errorf("LifetimeEnergyWh=%d want 1_234_567", snap.LifetimeEnergyWh)
	}
	if snap.TodayEnergyWh != 8_900 {
		t.Errorf("TodayEnergyWh=%d want 8_900", snap.TodayEnergyWh)
	}
	if snap.MonthEnergyWh != 234_000 {
		t.Errorf("MonthEnergyWh=%d want 234_000", snap.MonthEnergyWh)
	}
	if snap.YearEnergyWh != 1_100_000 {
		t.Errorf("YearEnergyWh=%d want 1_100_000", snap.YearEnergyWh)
	}
}

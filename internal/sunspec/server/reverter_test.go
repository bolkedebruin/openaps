package server

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/sunspec/config"
	"github.com/bolkedebruin/openaps/internal/sunspec/source"
	"github.com/bolkedebruin/openaps/internal/sunspec/sunspec"
	"github.com/bolkedebruin/openaps/codec"
	"github.com/simonvetter/modbus"
)

// ds3RestoreFrame is the full-power (MaxPanelLimitW) DS3 set-power frame
// the reverter re-sends when a reversion timer fires.
func ds3RestoreFrame(t *testing.T) []byte {
	t.Helper()
	f, err := codec.EncodeSetPower(0x20, uint16(source.MaxPanelLimitW), false)
	if err != nil {
		t.Fatalf("EncodeSetPower restore: %v", err)
	}
	return f
}

// TestReverter_Direct exercises the Schedule / Cancel API in isolation.
func TestReverter_Direct(t *testing.T) {
	fake := &fakeSender{}
	r := NewReverter(fake, nil, nil)
	if r == nil {
		t.Fatal("NewReverter returned nil with non-nil sender")
	}

	r.Schedule(2, 80*time.Millisecond, []revertTarget{{uid: "INV-A", modelCode: 0x20}})
	if !r.pending(2) {
		t.Error("expected pending timer after Schedule")
	}

	time.Sleep(200 * time.Millisecond)
	if r.pending(2) {
		t.Error("timer should be cleared after firing")
	}

	if got := fake.frameFor("INV-A"); !bytes.Equal(got, ds3RestoreFrame(t)) {
		t.Errorf("after reversion frame=% X want % X (full power)", got, ds3RestoreFrame(t))
	}
}

func TestReverter_CancelStopsTimer(t *testing.T) {
	fake := &fakeSender{}
	r := NewReverter(fake, nil, nil)

	r.Schedule(2, 80*time.Millisecond, []revertTarget{{uid: "INV-A", modelCode: 0x20}})
	r.Cancel(2)
	time.Sleep(200 * time.Millisecond)

	if got := fake.frames(); len(got) != 0 {
		t.Errorf("cancelled timer should send nothing, got %d frame(s)", len(got))
	}
}

func TestReverter_RescheduleResetsDeadline(t *testing.T) {
	r := NewReverter(&fakeSender{}, nil, nil)
	tgt := []revertTarget{{uid: "INV-A", modelCode: 0x20}}
	r.Schedule(2, 200*time.Millisecond, tgt)
	time.Sleep(100 * time.Millisecond)
	// Reset before original deadline.
	r.Schedule(2, 200*time.Millisecond, tgt)
	time.Sleep(150 * time.Millisecond)
	if !r.pending(2) {
		t.Error("rescheduled timer should still be pending 150ms after reset")
	}
	r.Cancel(2)
}

// TestServer_RvrtTms_AutoReverts drives the full Modbus path: write
// WMaxLimPct=8000 + Ena=1 + RvrtTms=1, wait 1.5s, verify the cap frame is
// followed by a full-power restore frame.
func TestServer_RvrtTms_AutoReverts(t *testing.T) {
	fake := &fakeSender{}
	snap := source.Snapshot{ECUID: "999999999999", Inverters: []source.Inverter{ds3Inv("INV-A")}}

	port := freePort(t)
	srv := New(fixedProvider{snap}, Config{
		URL:             "tcp://127.0.0.1:" + strconv.Itoa(port),
		RefreshInterval: time.Second,
		InvDriver:       fake,
		Writes:          config.Config{Writes: config.WritesConfig{Enabled: config.BoolPtr(true)}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	cli, _ := modbus.NewClient(&modbus.ClientConfiguration{
		URL: "tcp://127.0.0.1:" + strconv.Itoa(port), Timeout: 2 * time.Second,
	})
	cli.Open()
	defer cli.Close()
	cli.SetUnitId(2)

	ctlAddr, ok := walkForModel(cli, sunspec.ControlsModelID)
	if !ok {
		t.Fatal("Model 123 missing")
	}
	body := ctlAddr + 2

	// WMaxLimPct=8000 (80.00%) + RvrtTms=1 + Ena=1, offsets 3..7 (Conn
	// untouched). DS3 365 W/panel → 292 W.
	regs := []uint16{8000, 0, 1, 0, 1}
	if err := cli.WriteRegisters(body+sunspec.OffControlsWMaxLimPct, regs); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Immediately after write: the cap frame is in place.
	capFrame, _ := codec.EncodeSetPower(0x20, 292, false)
	if got := fake.frameFor("INV-A"); !bytes.Equal(got, capFrame) {
		t.Errorf("after write frame=% X want % X (cap)", got, capFrame)
	}

	// After RvrtTms (+ slack): a full-power restore frame is sent.
	time.Sleep(1500 * time.Millisecond)
	if got := fake.frameFor("INV-A"); !bytes.Equal(got, ds3RestoreFrame(t)) {
		t.Errorf("after RvrtTms frame=% X want % X (auto-revert)", got, ds3RestoreFrame(t))
	}
}

// TestServer_RvrtTms_RefreshKeepsCap simulates Victron refreshing the cap
// before RvrtTms expires — no revert frame should ever be sent.
func TestServer_RvrtTms_RefreshKeepsCap(t *testing.T) {
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
	body := ctlAddr + 2
	write := func() {
		// WMaxLimPct=8000 (80.00%), RvrtTms=2s, Ena=1, offsets 3..7.
		cli.WriteRegisters(body+sunspec.OffControlsWMaxLimPct, []uint16{8000, 0, 2, 0, 1})
	}

	write()
	time.Sleep(700 * time.Millisecond)
	write() // refresh — resets RvrtTms timer
	time.Sleep(700 * time.Millisecond)
	write() // refresh again

	// Total elapsed >= 2.1s but each refresh resets the 2s timer, so no
	// restore frame should have been sent — only cap frames.
	capFrame, _ := codec.EncodeSetPower(0x20, 292, false)
	restore := ds3RestoreFrame(t)
	for _, f := range fake.frames() {
		if bytes.Equal(f.frame, restore) {
			t.Fatal("refresh pattern reverted the cap, but it should persist")
		}
		if !bytes.Equal(f.frame, capFrame) {
			t.Errorf("unexpected frame % X (want cap % X)", f.frame, capFrame)
		}
	}
	if got := fake.frameFor("INV-A"); !bytes.Equal(got, capFrame) {
		t.Errorf("cap not held; last frame=% X want % X", got, capFrame)
	}
}

// TestServer_RvrtTms_NotImplSentinel — RvrtTms = 0xFFFF (uint16 not-impl)
// must NOT schedule a reverter. Some clients send 0xFFFF as "I don't care."
func TestServer_RvrtTms_NotImplSentinel(t *testing.T) {
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
	body := ctlAddr + 2
	cli.WriteRegisters(body+sunspec.OffControlsWMaxLimPct, []uint16{8000, 0, 0xFFFF, 0, 1})

	if srv.reverter.pending(2) {
		t.Error("0xFFFF (not-impl sentinel) must not arm reverter")
	}
}

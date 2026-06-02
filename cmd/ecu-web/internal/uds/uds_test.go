package uds

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/cmd/ecu-web/internal/snapshot"
	"github.com/bolkedebruin/openaps/wire"
)

// shortSock returns a UDS path short enough to fit the OS sun_path limit
// (t.TempDir() on macOS is too long). Cleaned up via t.Cleanup.
func shortSock(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ecuweb")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "d.sock")
}

// fakeDriver is a minimal stand-in for inv-driver: it accepts one
// subscriber, reads its Hello, then streams a fixed set of envelopes.
func fakeDriver(t *testing.T, sock string, frames []*wire.Envelope) {
	t.Helper()
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		var hello wire.Envelope
		if err := wire.ReadFrame(c, &hello); err != nil {
			return
		}
		if hello.GetHello() == nil || hello.GetHello().GetRole() != wire.Role_SUBSCRIBER {
			t.Errorf("expected SUBSCRIBER hello, got %v", hello.GetHello())
		}
		for _, f := range frames {
			if err := wire.WriteFrame(c, f); err != nil {
				return
			}
		}
		// Hold the connection open so the subscriber stays attached.
		select {}
	}()
}

func TestSubscriberFoldsStreamIntoSnapshot(t *testing.T) {
	sock := shortSock(t)
	frames := []*wire.Envelope{
		{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{
			PeerUid: "aabbccddeeff", ModelCode: u32(0x18),
		}}},
		{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
			PeerUid: "aabbccddeeff", TsMs: time.Now().UnixMilli(),
			Model: "QS1A", ActivePowerW: 800, GridV: 230.5, FreqHz: 50.0,
		}}},
		{Body: &wire.Envelope_Fleet{Fleet: &wire.FleetSummary{
			NameplateTotalW: 1600, TodayWh: 4200,
		}}},
	}
	fakeDriver(t, sock, frames)

	snap := snapshot.New(nil)
	sub := &Subscriber{SocketPath: sock, Backend: "ecu-web-test", Snap: snap}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = sub.Run(ctx) }()

	// Poll until the telemetry has folded in (or time out).
	deadline := time.Now().Add(3 * time.Second)
	for {
		f := snap.Fleet(time.Now())
		if f.InverterCount == 1 && f.ActivePowerW == 800 && f.TodayWh == 4200 {
			if !sub.Connected() {
				t.Error("Connected() should be true while attached")
			}
			inv := f.Inverters[0]
			if inv.Model != "QS1A" || inv.NameplateW != 1600 || inv.GridV != 230.5 {
				t.Errorf("unexpected inverter DTO: %+v", inv)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("snapshot not populated in time: %+v", f)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func u32(v uint32) *uint32 { return &v }

package modem

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bolke/inv-driver/wire"
)

// shortSock returns a UDS path short enough to fit the OS sun_path limit
// (t.TempDir() on macOS is too long). Cleaned up via t.Cleanup.
func shortSock(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "zb")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "d.sock")
}

// fakeSettings accepts one connection, reads Hello + SettingsRequest get,
// and replies with a SettingsResponse carrying settings.
func fakeSettings(t *testing.T, sock string, settings *wire.Settings) {
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
		var hello, req wire.Envelope
		if wire.ReadFrame(c, &hello) != nil || wire.ReadFrame(c, &req) != nil {
			return
		}
		if hello.GetHello() == nil {
			t.Errorf("first frame not Hello")
			return
		}
		if req.GetSettingsReq().GetGet() == nil {
			t.Errorf("expected SettingsRequest get, got %T", req.GetBody())
			return
		}
		_ = wire.WriteFrame(c, &wire.Envelope{Body: &wire.Envelope_SettingsResp{
			SettingsResp: &wire.SettingsResponse{Ok: true, Settings: settings},
		}})
	}()
}

func TestFetchSettings(t *testing.T) {
	sock := shortSock(t)
	fakeSettings(t, sock, &wire.Settings{PanOverride: "0DCE", Channel: 20})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pan, channel, ok := FetchSettings(ctx, sock, "apsystems-stock-zb")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if pan != "0DCE" {
		t.Errorf("pan = %q, want %q", pan, "0DCE")
	}
	if channel != 20 {
		t.Errorf("channel = %d, want 20", channel)
	}
}

func TestFetchSettingsDialFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, _, ok := FetchSettings(ctx, "/tmp/zb-nonexistent-12345.sock", "apsystems-stock-zb"); ok {
		t.Fatal("expected ok=false for absent socket")
	}
}

func TestResolveChannel(t *testing.T) {
	// In-range inv-driver setting wins.
	if got := ResolveChannel(20); got != 20 {
		t.Errorf("ResolveChannel(20) = %d, want 20", got)
	}
	// 0 (unset) or out-of-range falls back to channel.conf / DefaultChannel.
	// channel.conf is absent in the test env, so DefaultChannel is returned.
	if got := ResolveChannel(0); got != DefaultChannel {
		t.Errorf("ResolveChannel(0) = %d, want DefaultChannel %d", got, DefaultChannel)
	}
	if got := ResolveChannel(99); got != DefaultChannel {
		t.Errorf("ResolveChannel(99) = %d, want DefaultChannel %d", got, DefaultChannel)
	}
}

func TestParsePANHex(t *testing.T) {
	cases := []struct {
		in     string
		want   uint16
		wantOk bool
	}{
		{"0DCE", 0x0DCE, true},
		{"0x0dce", 0x0dce, true},
		{"FBFB", 0xFBFB, true},
		{" 1 ", 0x0001, true},
		{"", 0, false},
		{"zzz", 0, false},
		{"12345", 0, false},
	}
	for _, c := range cases {
		got, ok := ParsePANHex(c.in)
		if ok != c.wantOk || (ok && got != c.want) {
			t.Errorf("ParsePANHex(%q) = (0x%04X, %v), want (0x%04X, %v)", c.in, got, ok, c.want, c.wantOk)
		}
	}
}

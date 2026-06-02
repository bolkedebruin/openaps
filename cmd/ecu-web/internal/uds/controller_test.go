package uds

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// fakeController accepts one publisher connection, reads Hello +
// SystemStatusRequest, and replies with resp.
func fakeController(t *testing.T, sock string, resp *wire.SystemStatusResponse) {
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
		if hello.GetHello() == nil {
			t.Errorf("first frame not Hello")
			return
		}
		var req wire.Envelope
		if err := wire.ReadFrame(c, &req); err != nil {
			return
		}
		if req.GetSystemStatusReq() == nil {
			t.Errorf("expected SystemStatusRequest, got %T", req.GetBody())
			return
		}
		_ = wire.WriteFrame(c, &wire.Envelope{Body: &wire.Envelope_SystemStatusResp{SystemStatusResp: resp}})
	}()
}

func TestControllerSystemStatus(t *testing.T) {
	sock := shortSock(t)
	want := &wire.SystemStatusResponse{
		Ok:   true,
		TsMs: time.Now().UnixMilli(),
		Ecu:  &wire.EcuIdentity{EcuId: "216200001234"},
		Peers: []*wire.PeerStatus{
			{Backend: "ecu-zb", Role: "PUBLISHER"},
			{Backend: "ecu-web", Role: "SUBSCRIBER", Controller: true},
		},
	}
	fakeController(t, sock, want)

	ctrl := &Controller{SocketPath: sock, Backend: "ecu-web"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got, err := ctrl.SystemStatus(ctx)
	if err != nil {
		t.Fatalf("SystemStatus: %v", err)
	}
	if got.GetEcu().GetEcuId() != "216200001234" || len(got.GetPeers()) != 2 {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestControllerSystemStatusError(t *testing.T) {
	sock := shortSock(t)
	fakeController(t, sock, &wire.SystemStatusResponse{Ok: false, Error: "refused: not a controller"})
	ctrl := &Controller{SocketPath: sock, Backend: "ecu-web"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := ctrl.SystemStatus(ctx); err == nil {
		t.Fatal("expected error from ok=false response")
	}
}

// fakeEventsController accepts one connection, reads Hello + EventsRequest,
// and replies with resp.
func fakeEventsController(t *testing.T, sock string, resp *wire.EventsResponse) {
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
		if req.GetEventsReq() == nil {
			t.Errorf("expected EventsRequest, got %T", req.GetBody())
			return
		}
		_ = wire.WriteFrame(c, &wire.Envelope{Body: &wire.Envelope_EventsResp{EventsResp: resp}})
	}()
}

// fakeSettingsController accepts one connection, reads Hello +
// SettingsRequest, asserts the expected op, and replies with resp.
func fakeSettingsController(t *testing.T, sock string, wantSet bool, resp *wire.SettingsResponse) {
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
		sr := req.GetSettingsReq()
		if sr == nil {
			t.Errorf("expected SettingsRequest, got %T", req.GetBody())
			return
		}
		if wantSet && sr.GetSet() == nil {
			t.Errorf("expected Set op")
		}
		if !wantSet && sr.GetGet() == nil {
			t.Errorf("expected Get op")
		}
		_ = wire.WriteFrame(c, &wire.Envelope{Body: &wire.Envelope_SettingsReq{SettingsReq: nil}})
		_ = wire.WriteFrame(c, &wire.Envelope{Body: &wire.Envelope_SettingsResp{SettingsResp: resp}})
	}()
}

func TestControllerGetSettings(t *testing.T) {
	sock := shortSock(t)
	fakeSettingsController(t, sock, false, &wire.SettingsResponse{
		Ok:       true,
		Settings: &wire.Settings{EcuId: "216200001234", Mac: "80971b000000", ZigbeeType: "apsystems"},
	})
	ctrl := &Controller{SocketPath: sock, Backend: "ecu-web"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := ctrl.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if resp.GetSettings().GetEcuId() != "216200001234" {
		t.Errorf("unexpected settings: %+v", resp.GetSettings())
	}
}

func TestControllerSetSettings(t *testing.T) {
	sock := shortSock(t)
	fakeSettingsController(t, sock, true, &wire.SettingsResponse{
		Ok:       true,
		Settings: &wire.Settings{EcuId: "new-id", ZigbeeType: "general"},
	})
	ctrl := &Controller{SocketPath: sock, Backend: "ecu-web"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := ctrl.SetSettings(ctx, &wire.Settings{EcuId: "new-id", ZigbeeType: "general"})
	if err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	if resp.GetSettings().GetZigbeeType() != "general" {
		t.Errorf("unexpected settings: %+v", resp.GetSettings())
	}
}

func TestControllerSetSettingsError(t *testing.T) {
	sock := shortSock(t)
	fakeSettingsController(t, sock, true, &wire.SettingsResponse{Ok: false, Error: "bad pan"})
	ctrl := &Controller{SocketPath: sock, Backend: "ecu-web"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := ctrl.SetSettings(ctx, &wire.Settings{PanOverride: "ZZZZ"}); err == nil {
		t.Fatal("expected error from ok=false response")
	}
}

func TestControllerEvents(t *testing.T) {
	sock := shortSock(t)
	fakeEventsController(t, sock, &wire.EventsResponse{
		Ok: true,
		Events: []*wire.Event{
			{Id: 2, TsMs: 200, Kind: "decode_failed", Severity: "warn", Detail: "crc"},
			{Id: 1, TsMs: 100, Kind: "paired", Severity: "info", InverterUid: "u"},
		},
	})
	ctrl := &Controller{SocketPath: sock, Backend: "ecu-web"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := ctrl.Events(ctx, &wire.EventsRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(resp.GetEvents()) != 2 || resp.GetEvents()[0].GetKind() != "decode_failed" {
		t.Errorf("unexpected events: %+v", resp.GetEvents())
	}
}

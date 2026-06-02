package uds

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// fakeGridProfileController accepts one publisher connection, reads Hello +
// GridProfileRequest, records the request via onReq, and replies with resp.
func fakeGridProfileController(t *testing.T, sock string, resp *wire.GridProfileResponse, onReq func(*wire.GridProfileRequest)) {
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
		var req wire.Envelope
		if err := wire.ReadFrame(c, &req); err != nil {
			return
		}
		gp := req.GetGridProfileReq()
		if gp == nil {
			t.Errorf("expected GridProfileRequest, got %T", req.GetBody())
			return
		}
		if onReq != nil {
			onReq(gp)
		}
		_ = wire.WriteFrame(c, &wire.Envelope{Body: &wire.Envelope_GridProfileResp{GridProfileResp: resp}})
	}()
}

func TestControllerGridProfileSelectBase(t *testing.T) {
	sock := shortSock(t)
	var gotID string
	fakeGridProfileController(t, sock,
		&wire.GridProfileResponse{Ok: true, Json: []byte(`[]`)},
		func(req *wire.GridProfileRequest) {
			if sb := req.GetSelectBase(); sb != nil {
				gotID = sb.GetId()
			}
		})

	ctrl := &Controller{SocketPath: sock, Backend: "ecu-web"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := ctrl.GridProfile(ctx, &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_SelectBase{SelectBase: &wire.SelectBase{Id: "EN50549-1"}}})
	if err != nil {
		t.Fatalf("GridProfile: %v", err)
	}
	if !resp.GetOk() {
		t.Errorf("ok=false: %+v", resp)
	}
	if gotID != "EN50549-1" {
		t.Errorf("server saw id=%q, want EN50549-1", gotID)
	}
}

func TestControllerGridProfileError(t *testing.T) {
	sock := shortSock(t)
	fakeGridProfileController(t, sock,
		&wire.GridProfileResponse{Ok: false, Error: "refused: not a controller"}, nil)

	ctrl := &Controller{SocketPath: sock, Backend: "ecu-web"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := ctrl.GridProfile(ctx, &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_GetStatus{GetStatus: &wire.Empty{}}})
	if err == nil {
		t.Fatal("expected error from ok=false response")
	}
	if resp == nil || resp.GetError() == "" {
		t.Errorf("want response with error preserved, got %+v", resp)
	}
}

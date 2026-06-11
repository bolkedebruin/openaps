package udsutil

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// shortSock returns a UDS path short enough to fit the OS sun_path limit
// (t.TempDir() on macOS is too long). Cleaned up via t.Cleanup.
func shortSock(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "uds")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "d.sock")
}

// serve accepts one connection, delivers every received envelope on the
// returned channel, and writes each envelope in replies after the first
// non-Hello frame arrives.
func serve(t *testing.T, sock string, replies ...*wire.Envelope) <-chan *wire.Envelope {
	t.Helper()
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	ch := make(chan *wire.Envelope, 8)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var hello wire.Envelope
		if err := wire.ReadFrame(conn, &hello); err != nil {
			return
		}
		ch <- &hello
		var req wire.Envelope
		if err := wire.ReadFrame(conn, &req); err != nil {
			return
		}
		ch <- &req
		for _, r := range replies {
			if err := wire.WriteFrame(conn, r); err != nil {
				return
			}
		}
		// Drain any further requests so multi-request callers don't block.
		for {
			var extra wire.Envelope
			if err := wire.ReadFrame(conn, &extra); err != nil {
				return
			}
			ch <- &extra
		}
	}()
	return ch
}

func helloEnv(backend string) *wire.Envelope {
	return &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend:     backend,
		StartedAtMs: time.Now().UnixMilli(),
	}}}
}

func recv(t *testing.T, ch <-chan *wire.Envelope) *wire.Envelope {
	t.Helper()
	select {
	case env := <-ch:
		return env
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for envelope")
		return nil
	}
}

func TestRoundtrip_MatchSkipsUnrelatedFrames(t *testing.T) {
	sock := shortSock(t)
	ch := serve(t, sock,
		// An unrelated frame the matcher must skip.
		&wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{PeerUid: "u"}}},
		&wire.Envelope{Body: &wire.Envelope_SettingsResp{SettingsResp: &wire.SettingsResponse{
			Ok:       true,
			Settings: &wire.Settings{EcuId: "216200001234"},
		}}},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req := &wire.Envelope{Body: &wire.Envelope_SettingsReq{SettingsReq: &wire.SettingsRequest{
		Op: &wire.SettingsRequest_Get{Get: &wire.Empty{}},
	}}}
	env, err := Roundtrip(ctx, sock, helloEnv("test-client"), func(e *wire.Envelope) bool {
		return e.GetSettingsResp() != nil
	}, req)
	if err != nil {
		t.Fatalf("Roundtrip: %v", err)
	}
	if env.GetSettingsResp().GetSettings().GetEcuId() != "216200001234" {
		t.Errorf("unexpected response: %+v", env)
	}

	if recv(t, ch).GetHello().GetBackend() != "test-client" {
		t.Error("first frame is not the Hello envelope")
	}
	if recv(t, ch).GetSettingsReq() == nil {
		t.Error("second frame is not the SettingsRequest envelope")
	}
}

func TestRoundtrip_NilMatchIsFireAndForget(t *testing.T) {
	sock := shortSock(t)
	ch := serve(t, sock)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	send := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{PeerUid: "999900000003", Frame: []byte{0xFB}}}}
	bcast := &wire.Envelope{Body: &wire.Envelope_Broadcast{Broadcast: &wire.Broadcast{Frame: []byte{0xFC}}}}
	env, err := Roundtrip(ctx, sock, helloEnv("test-client"), nil, send, bcast)
	if err != nil {
		t.Fatalf("Roundtrip: %v", err)
	}
	if env != nil {
		t.Errorf("fire-and-forget returned %+v, want nil", env)
	}

	if recv(t, ch).GetHello() == nil {
		t.Error("first frame is not the Hello envelope")
	}
	if recv(t, ch).GetSend().GetPeerUid() != "999900000003" {
		t.Error("second frame is not the Send envelope")
	}
	if recv(t, ch).GetBroadcast() == nil {
		t.Error("third frame is not the Broadcast envelope")
	}
}

func TestRoundtrip_DialErrorSurfaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := Roundtrip(ctx, filepath.Join("/tmp", "udsutil-absent-12345.sock"), helloEnv("x"), nil)
	if err == nil {
		t.Fatal("expected dial error against absent socket, got nil")
	}
}

func TestRoundtrip_ReadDeadline(t *testing.T) {
	sock := shortSock(t)
	serve(t, sock) // never replies

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := Roundtrip(ctx, sock, helloEnv("x"), func(*wire.Envelope) bool { return true },
		&wire.Envelope{Body: &wire.Envelope_SystemStatusReq{SystemStatusReq: &wire.SystemStatusRequest{}}})
	if err == nil {
		t.Fatal("expected read timeout, got nil")
	}
}

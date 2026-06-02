package invdriver

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/sunspec/testhelpers"
	"github.com/bolkedebruin/openaps/wire"
)

func TestClient_Send_DeliversEnvelope(t *testing.T) {
	sock := filepath.Join(testhelpers.ShortTempDir(t), "send.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	type received struct {
		hello *wire.Hello
		send  *wire.Send
	}
	ch := make(chan received, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var hello, env wire.Envelope
		if err := wire.ReadFrame(conn, &hello); err != nil {
			return
		}
		if err := wire.ReadFrame(conn, &env); err != nil {
			return
		}
		ch <- received{hello: hello.GetHello(), send: env.GetSend()}
	}()

	c := New(sock, "v-test")
	frame := []byte{0xFB, 0xFB, 0x06, 0xC1, 0, 0, 0, 0, 0, 0, 0xC7, 0xFE, 0xFE}
	if err := c.Send(context.Background(), "999900000003", frame); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case got := <-ch:
		if got.hello == nil || got.hello.GetBackend() != backendName {
			t.Errorf("hello backend=%q want %q", got.hello.GetBackend(), backendName)
		}
		if got.send == nil {
			t.Fatal("no Send envelope received")
		}
		if got.send.GetPeerUid() != "999900000003" {
			t.Errorf("peer_uid=%q want 999900000003", got.send.GetPeerUid())
		}
		if !bytes.Equal(got.send.GetFrame(), frame) {
			t.Errorf("frame=% X want % X", got.send.GetFrame(), frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Send envelope")
	}
}

func TestClient_Send_DialErrorSurfaces(t *testing.T) {
	c := New(filepath.Join(testhelpers.ShortTempDir(t), "absent.sock"), "v")
	if err := c.Send(context.Background(), "999900000003", []byte{0xFB}); err == nil {
		t.Fatal("expected dial error against absent socket, got nil")
	}
}

func TestClient_Send_RejectsEmptyArgs(t *testing.T) {
	c := New("/tmp/unused.sock", "v")
	if err := c.Send(context.Background(), "", []byte{0xFB}); err == nil {
		t.Error("expected error for empty uid")
	}
	if err := c.Send(context.Background(), "999900000003", nil); err == nil {
		t.Error("expected error for empty frame")
	}
}

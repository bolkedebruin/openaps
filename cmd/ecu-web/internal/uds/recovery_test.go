package uds

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// fakeRecoveryd accepts one connection and serves a single raw
// AccessRequest/AccessResponse exchange (no Hello), using the shared
// wire framing. It asserts the request matches wantOp ("list"/"add"/
// "remove"/"status") and replies with resp.
func fakeRecoveryd(t *testing.T, sock, wantOp string, resp *wire.AccessResponse) {
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
		var req wire.AccessRequest
		if err := wire.ReadMessage(c, &req); err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		got := ""
		switch req.GetOp().(type) {
		case *wire.AccessRequest_ListKeys:
			got = "list"
		case *wire.AccessRequest_AddKey:
			got = "add"
		case *wire.AccessRequest_RemoveKey:
			got = "remove"
		case *wire.AccessRequest_Status:
			got = "status"
		}
		if got != wantOp {
			t.Errorf("op = %q, want %q", got, wantOp)
		}
		_ = wire.WriteMessage(c, resp)
	}()
}

func TestRecoveryListKeys(t *testing.T) {
	sock := shortSock(t)
	fakeRecoveryd(t, sock, "list", &wire.AccessResponse{
		Ok:       true,
		Provider: "openaps",
		KeyCount: 1,
		Keys: []*wire.SshKey{
			{Pubkey: "ssh-ed25519 AAAA", Comment: "op", Fingerprint: "SHA256:abc", AddedMs: 123},
		},
	})
	rec := &Recovery{SocketPath: sock}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := rec.ListKeys(ctx)
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if resp.GetProvider() != "openaps" || len(resp.GetKeys()) != 1 {
		t.Errorf("unexpected response: %+v", resp)
	}
	if resp.GetKeys()[0].GetFingerprint() != "SHA256:abc" {
		t.Errorf("unexpected fingerprint: %q", resp.GetKeys()[0].GetFingerprint())
	}
}

func TestRecoveryAddKey(t *testing.T) {
	sock := shortSock(t)
	fakeRecoveryd(t, sock, "add", &wire.AccessResponse{Ok: true, Provider: "openaps", KeyCount: 1})
	rec := &Recovery{SocketPath: sock}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := rec.AddKey(ctx, "ssh-ed25519 AAAA", "op"); err != nil {
		t.Fatalf("AddKey: %v", err)
	}
}

func TestRecoveryRemoveKey(t *testing.T) {
	sock := shortSock(t)
	fakeRecoveryd(t, sock, "remove", &wire.AccessResponse{Ok: true, Provider: "openaps", KeyCount: 0})
	rec := &Recovery{SocketPath: sock}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := rec.RemoveKey(ctx, "SHA256:abc"); err != nil {
		t.Fatalf("RemoveKey: %v", err)
	}
}

func TestRecoveryStatus(t *testing.T) {
	sock := shortSock(t)
	fakeRecoveryd(t, sock, "status", &wire.AccessResponse{Ok: true, Provider: "host", HostUser: "ops", KeyCount: 2})
	rec := &Recovery{SocketPath: sock}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := rec.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if resp.GetProvider() != "host" || resp.GetHostUser() != "ops" || resp.GetKeyCount() != 2 {
		t.Errorf("unexpected status: %+v", resp)
	}
}

// TestRecoveryError verifies an ok=false response surfaces as an error that
// still carries the response payload.
func TestRecoveryError(t *testing.T) {
	sock := shortSock(t)
	fakeRecoveryd(t, sock, "add", &wire.AccessResponse{Ok: false, Error: "malformed pubkey"})
	rec := &Recovery{SocketPath: sock}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := rec.AddKey(ctx, "not-a-key", "")
	if err == nil {
		t.Fatal("expected error from ok=false response")
	}
	if resp == nil || resp.GetError() != "malformed pubkey" {
		t.Errorf("expected response payload with error, got %+v", resp)
	}
}

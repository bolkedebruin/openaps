package recoveryd

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// shortSock returns a short UDS path under the OS temp dir. macOS caps a
// UDS path at ~104 bytes, well under t.TempDir()'s long nested paths.
func shortSock(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("/tmp", "rcv*.sock")
	if err != nil {
		t.Fatal(err)
	}
	p := f.Name()
	_ = f.Close()
	_ = os.Remove(p)
	t.Cleanup(func() { _ = os.Remove(p) })
	return p
}

// roundtrip dials the server socket, sends one request, reads one
// response. Mirrors the framing ecu-web's recoveryd client uses.
func roundtrip(t *testing.T, sock string, req *wire.AccessRequest) *wire.AccessResponse {
	t.Helper()
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if err := wire.WriteMessage(c, req); err != nil {
		t.Fatalf("write: %v", err)
	}
	var resp wire.AccessResponse
	if err := wire.ReadMessage(c, &resp); err != nil {
		t.Fatalf("read: %v", err)
	}
	return &resp
}

func startTestServer(t *testing.T) string {
	t.Helper()
	m, _ := newTestManager(t)
	sock := shortSock(t)
	srv := &Server{Manager: m, SocketPath: sock}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = srv.Serve(ctx); close(done) }()
	// wait for socket to appear.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := net.Dial("unix", sock); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() { cancel(); <-done })
	return sock
}

func TestServerAddListRemoveStatus(t *testing.T) {
	sock := startTestServer(t)

	// empty list
	r := roundtrip(t, sock, &wire.AccessRequest{Op: &wire.AccessRequest_ListKeys{ListKeys: &wire.ListKeysOp{}}})
	if !r.GetOk() || r.GetKeyCount() != 0 {
		t.Fatalf("empty list: ok=%v count=%d err=%q", r.GetOk(), r.GetKeyCount(), r.GetError())
	}
	if r.GetProvider() != "openaps" {
		t.Errorf("provider = %q, want openaps", r.GetProvider())
	}

	// add a key
	r = roundtrip(t, sock, &wire.AccessRequest{Op: &wire.AccessRequest_AddKey{AddKey: &wire.AddKeyOp{Pubkey: keyA, Comment: "alice"}}})
	if !r.GetOk() || r.GetKeyCount() != 1 {
		t.Fatalf("add: ok=%v count=%d err=%q", r.GetOk(), r.GetKeyCount(), r.GetError())
	}
	fp := r.GetKeys()[0].GetFingerprint()
	if fp == "" {
		t.Fatal("no fingerprint returned")
	}

	// add malformed -> error
	r = roundtrip(t, sock, &wire.AccessRequest{Op: &wire.AccessRequest_AddKey{AddKey: &wire.AddKeyOp{Pubkey: "garbage"}}})
	if r.GetOk() {
		t.Error("malformed add should fail")
	}

	// status (no keys in payload, count set)
	r = roundtrip(t, sock, &wire.AccessRequest{Op: &wire.AccessRequest_Status{Status: &wire.StatusOp{}}})
	if !r.GetOk() || r.GetKeyCount() != 1 || len(r.GetKeys()) != 0 {
		t.Fatalf("status: ok=%v count=%d keys=%d", r.GetOk(), r.GetKeyCount(), len(r.GetKeys()))
	}

	// remove
	r = roundtrip(t, sock, &wire.AccessRequest{Op: &wire.AccessRequest_RemoveKey{RemoveKey: &wire.RemoveKeyOp{Fingerprint: fp}}})
	if !r.GetOk() || r.GetKeyCount() != 0 {
		t.Fatalf("remove: ok=%v count=%d err=%q", r.GetOk(), r.GetKeyCount(), r.GetError())
	}

	// remove absent -> error
	r = roundtrip(t, sock, &wire.AccessRequest{Op: &wire.AccessRequest_RemoveKey{RemoveKey: &wire.RemoveKeyOp{Fingerprint: fp}}})
	if r.GetOk() {
		t.Error("removing absent key should fail")
	}
}

// An empty AccessRequest marshals to a zero-length body, which the
// framing layer rejects before dispatch (server closes the conn). Test
// the dispatch-level empty-op path directly so the unset-oneof branch is
// covered without depending on framing behaviour.
func TestServerDispatchEmptyOp(t *testing.T) {
	m, _ := newTestManager(t)
	s := &Server{Manager: m}
	r := s.dispatch(&wire.AccessRequest{})
	if r.GetOk() {
		t.Error("empty op should fail")
	}
}

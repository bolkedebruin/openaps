package wire

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// pairConn returns two connected sides of a net.Pipe-equivalent UDS
// socket. UDS rather than net.Pipe because SubscriberSession calls
// conn.Close which net.Pipe handles, but tests further down also want
// the conn to behave like a real socket (deadlines / ErrClosed).
func pairConn(t *testing.T) (a, b net.Conn) {
	t.Helper()
	ln, err := net.Listen("unix", tempSocket(t))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	dialCh := make(chan net.Conn, 1)
	dialErr := make(chan error, 1)
	go func() {
		c, err := net.Dial("unix", ln.Addr().String())
		if err != nil {
			dialErr <- err
			return
		}
		dialCh <- c
	}()
	srv, err := ln.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	select {
	case c := <-dialCh:
		return c, srv
	case err := <-dialErr:
		t.Fatalf("dial: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("dial timeout")
	}
	return nil, nil
}

func helloSub() *Hello {
	return &Hello{
		Backend:     "test",
		Version:     "0.0.0",
		Hostname:    "host",
		StartedAtMs: 1,
		Role:        Role_SUBSCRIBER,
	}
}

// TestSubscriberSession_HappyPath sends Hello + two Telemetry envelopes
// from the server side and verifies both fire the callback.
func TestSubscriberSession_HappyPath(t *testing.T) {
	t.Parallel()
	clientSide, srvSide := pairConn(t)
	defer clientSide.Close()
	defer srvSide.Close()

	// Server: read Hello, write two Telemetry envelopes, then linger.
	srvDone := make(chan error, 1)
	go func() {
		var got Envelope
		if err := ReadFrame(srvSide, &got); err != nil {
			srvDone <- err
			return
		}
		if got.GetHello() == nil || got.GetHello().GetRole() != Role_SUBSCRIBER {
			srvDone <- errors.New("bad hello")
			return
		}
		for i, uid := range []string{"A", "B"} {
			tel := &Telemetry{TsMs: int64(i + 1), PeerUid: uid, Model: "QS1A"}
			if err := WriteFrame(srvSide, &Envelope{Body: &Envelope_Telemetry{Telemetry: tel}}); err != nil {
				srvDone <- err
				return
			}
		}
		srvDone <- nil
	}()

	var mu sync.Mutex
	var got []string
	gotN := make(chan struct{}, 4)
	onTel := func(t *Telemetry) {
		mu.Lock()
		got = append(got, t.GetPeerUid())
		mu.Unlock()
		gotN <- struct{}{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sessDone := make(chan error, 1)
	go func() { sessDone <- SubscriberSession(ctx, clientSide, helloSub(), onTel) }()

	// Wait for two callbacks.
	for i := 0; i < 2; i++ {
		select {
		case <-gotN:
		case <-time.After(2 * time.Second):
			t.Fatalf("callback %d never fired", i)
		}
	}
	if err := <-srvDone; err != nil {
		t.Fatalf("server: %v", err)
	}
	mu.Lock()
	if len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("dispatched UIDs: got %v want [A B]", got)
	}
	mu.Unlock()
	cancel()
	select {
	case <-sessDone:
	case <-time.After(2 * time.Second):
		t.Fatal("SubscriberSession did not return after cancel")
	}
}

// TestSubscriberSession_CtxCancelReturnsClean cancels the context while
// the read loop is blocked on ReadFrame and asserts a nil return within
// 1s.
func TestSubscriberSession_CtxCancelReturnsClean(t *testing.T) {
	t.Parallel()
	clientSide, srvSide := pairConn(t)
	defer clientSide.Close()
	defer srvSide.Close()

	// Server reads Hello then does nothing.
	go func() {
		var got Envelope
		_ = ReadFrame(srvSide, &got)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- SubscriberSession(ctx, clientSide, helloSub(), nil) }()

	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil on cancel, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SubscriberSession did not return within 1s of cancel")
	}
}

// TestSubscriberSession_PeerCloseReturnsClean verifies that when the
// remote side closes the conn, SubscriberSession returns nil (treating
// io.EOF as a clean shutdown).
func TestSubscriberSession_PeerCloseReturnsClean(t *testing.T) {
	t.Parallel()
	clientSide, srvSide := pairConn(t)
	defer clientSide.Close()

	go func() {
		var got Envelope
		_ = ReadFrame(srvSide, &got)
		_ = srvSide.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- SubscriberSession(ctx, clientSide, helloSub(), nil) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil on peer close, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SubscriberSession did not return after peer close")
	}
}

// TestSubscriberSession_NonTelemetryIgnored sends a Hello echo and a
// DecodeFailed envelope and verifies neither fires onTelemetry and the
// session keeps running.
func TestSubscriberSession_NonTelemetryIgnored(t *testing.T) {
	t.Parallel()
	clientSide, srvSide := pairConn(t)
	defer clientSide.Close()
	defer srvSide.Close()

	go func() {
		var got Envelope
		if err := ReadFrame(srvSide, &got); err != nil {
			return
		}
		// Send a Hello echo and a DecodeFailed — both should be dropped.
		_ = WriteFrame(srvSide, &Envelope{Body: &Envelope_Hello{Hello: &Hello{Backend: "echo"}}})
		_ = WriteFrame(srvSide, &Envelope{Body: &Envelope_DecodeFailed{DecodeFailed: &DecodeFailed{Error: "x"}}})
		// Then a real Telemetry so the test has a clear signal to wait on.
		_ = WriteFrame(srvSide, &Envelope{Body: &Envelope_Telemetry{Telemetry: &Telemetry{PeerUid: "Z"}}})
		// Linger.
		buf := make([]byte, 1)
		_, _ = srvSide.Read(buf)
	}()

	var calls atomic.Int32
	hit := make(chan string, 4)
	onTel := func(t *Telemetry) {
		calls.Add(1)
		hit <- t.GetPeerUid()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- SubscriberSession(ctx, clientSide, helloSub(), onTel) }()

	select {
	case uid := <-hit:
		if uid != "Z" {
			t.Fatalf("first callback uid: got %q want Z (Hello/DecodeFailed should not have fired)", uid)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected one Telemetry callback after the dropped frames")
	}
	if calls.Load() != 1 {
		t.Fatalf("callback fired %d times, want 1", calls.Load())
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SubscriberSession did not return after cancel")
	}
}

// TestSubscriberSession_NilArgs guards the explicit nil-conn / nil-hello
// errors so DialLoop integrators get a clear failure instead of a
// panic.
func TestSubscriberSession_NilArgs(t *testing.T) {
	t.Parallel()
	if err := SubscriberSession(context.Background(), nil, helloSub(), nil); err == nil {
		t.Fatal("expected error for nil conn")
	}
	clientSide, srvSide := pairConn(t)
	defer clientSide.Close()
	defer srvSide.Close()
	if err := SubscriberSession(context.Background(), clientSide, nil, nil); err == nil {
		t.Fatal("expected error for nil hello")
	}
}

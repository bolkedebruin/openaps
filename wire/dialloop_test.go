package wire

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func tempSocket(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "s.sock")
	if len(p) < 100 {
		return p
	}
	return filepath.Join(os.TempDir(), "dl-"+strconv.FormatInt(time.Now().UnixNano(), 36)+".sock")
}

// TestDialLoop_DialSuccess verifies a single connect-session-exit run.
func TestDialLoop_DialSuccess(t *testing.T) {
	t.Parallel()
	path := tempSocket(t)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan struct{}, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- struct{}{}
		_ = c.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var connects atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- DialLoop(ctx, DialLoopConfig{
			SocketPath: path,
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 50 * time.Millisecond,
			OnConnect:  func() { connects.Add(1) },
			Session: func(ctx context.Context, conn net.Conn) error {
				// Session returns immediately; caller closes conn.
				return io.EOF
			},
		})
	}()

	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("server never accepted")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DialLoop returned: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DialLoop did not return after cancel")
	}
	if connects.Load() < 1 {
		t.Fatalf("OnConnect never fired")
	}
}

// TestDialLoop_DialFailureThenSuccess verifies retry behaviour: the
// listener comes up after the first attempt, and the loop reconnects.
func TestDialLoop_DialFailureThenSuccess(t *testing.T) {
	t.Parallel()
	path := tempSocket(t)
	// Don't bind yet — first dial must fail.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var dialErrors atomic.Int32
	var connects atomic.Int32

	done := make(chan error, 1)
	go func() {
		done <- DialLoop(ctx, DialLoopConfig{
			SocketPath:  path,
			MinBackoff:  20 * time.Millisecond,
			MaxBackoff:  50 * time.Millisecond,
			OnConnect:   func() { connects.Add(1) },
			OnDialError: func(err error, retryIn time.Duration) { dialErrors.Add(1) },
			Session: func(ctx context.Context, conn net.Conn) error {
				<-ctx.Done()
				return ctx.Err()
			},
		})
	}()

	// Wait for at least one dial-error before binding.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && dialErrors.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if dialErrors.Load() == 0 {
		cancel()
		t.Fatal("no dial error before listener came up")
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		cancel()
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		<-time.After(50 * time.Millisecond)
		_ = c.Close()
	}()

	// Wait for connect.
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && connects.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if connects.Load() == 0 {
		cancel()
		t.Fatal("never connected after listener came up")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("DialLoop did not exit after cancel")
	}
}

// TestDialLoop_CtxCancelStops verifies cancel returns ctx.Err quickly.
func TestDialLoop_CtxCancelStops(t *testing.T) {
	t.Parallel()
	path := tempSocket(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- DialLoop(ctx, DialLoopConfig{
			SocketPath: path,
			MinBackoff: 5 * time.Millisecond,
			MaxBackoff: 10 * time.Millisecond,
			Session: func(ctx context.Context, conn net.Conn) error {
				<-ctx.Done()
				return ctx.Err()
			},
		})
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("DialLoop did not stop within 1s of cancel")
	}
}

// TestDialLoop_JitterInBounds verifies nextBackoff stays within
// [maxB*0.9, maxB*1.1] once it's saturated at maxB.
func TestDialLoop_JitterInBounds(t *testing.T) {
	t.Parallel()
	const maxB = 30 * time.Second
	d := maxB
	const samples = 1000
	for i := 0; i < samples; i++ {
		got := nextBackoff(d, maxB)
		// got = maxB + jitter(maxB); jitter is in [-maxB/10, +maxB/10).
		lo := maxB - maxB/10
		hi := maxB + maxB/10
		if got < lo || got > hi {
			t.Fatalf("nextBackoff out of bounds: got %v want [%v, %v]", got, lo, hi)
		}
	}
}

// TestDialLoop_RejectsBadConfig verifies the helper returns an error
// rather than spinning on missing required fields.
func TestDialLoop_RejectsBadConfig(t *testing.T) {
	t.Parallel()
	if err := DialLoop(context.Background(), DialLoopConfig{SocketPath: "/x"}); err == nil {
		t.Fatal("expected error when Session is nil")
	}
	if err := DialLoop(context.Background(), DialLoopConfig{Session: func(context.Context, net.Conn) error { return nil }}); err == nil {
		t.Fatal("expected error when SocketPath is empty")
	}
}

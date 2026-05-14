package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bolke/inv-driver/internal/paramsfile"
	"github.com/bolke/inv-driver/wire"
)

// socketPath returns a short UDS path inside t.TempDir(). On macOS the
// limit is 104 chars.
func socketPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "s.sock")
	if len(p) < 100 {
		return p
	}
	return filepath.Join(os.TempDir(), "lb-"+filepath.Base(dir)+".sock")
}

// TestLegacyBridge_EndToEnd spins up a fake UDS server that pretends
// to be inv-driver: it accepts one connection, reads the SUBSCRIBER
// Hello, and writes a Telemetry envelope. We then assert the params
// file is rewritten with the expected QS1A line.
func TestLegacyBridge_EndToEnd(t *testing.T) {
	t.Parallel()
	sockPath := socketPath(t)
	paramsPath := filepath.Join(t.TempDir(), "parameters_app.conf")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	helloCh := make(chan *wire.Hello, 1)
	srvDone := make(chan struct{})
	go func() {
		defer close(srvDone)
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		var hello wire.Envelope
		if err := wire.ReadFrame(c, &hello); err != nil {
			t.Logf("server read hello: %v", err)
			return
		}
		helloCh <- hello.GetHello()
		// Push a single QS1A Telemetry envelope.
		env := &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
			TsMs:    1,
			PeerUid: "806000042582",
			Model:   "QS1A",
			FreqHz:  50.0,
			GridV:   223.0,
			BusV:    24,
			Panels: []*wire.Panel{
				{Index: 0, W: 55},
				{Index: 1, W: 54},
				{Index: 2, W: 55},
				{Index: 3, W: 33},
			},
		}}}
		if err := wire.WriteFrame(c, env); err != nil {
			t.Logf("server write tel: %v", err)
			return
		}
		// Keep conn open until ctx tears it down on the client side.
		buf := make([]byte, 1)
		_, _ = c.Read(buf)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	u := paramsfile.New(paramsPath)
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		_ = runLoop(ctx, sockPath, u)
	}()

	// Wait for the hello to arrive (server side).
	select {
	case h := <-helloCh:
		if h.GetRole() != wire.Role_SUBSCRIBER {
			t.Fatalf("hello role: got %s want SUBSCRIBER", h.GetRole())
		}
		if h.GetBackend() != "legacy-bridge" {
			t.Fatalf("hello backend: got %q want legacy-bridge", h.GetBackend())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never received hello")
	}

	// Wait for the params file to materialise.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(paramsPath)
		if err == nil && strings.Contains(string(b), "806000042582,1,03,") {
			cancel()
			<-loopDone
			<-srvDone
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("params file %s never contained expected line", paramsPath)
}

// Package ipc hosts the inv-driver Unix-domain socket server. Backends
// (bus-mgr / ecu-zb) connect, send a Hello envelope, then stream
// telemetry frames. Per INV_DRIVER_DESIGN.md §2 this is the typed
// ingress edge of the daemon — protocol checks live here, business
// logic lives in package ingest.
package ipc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bolke/inv-driver/internal/ingest"
	"github.com/bolke/inv-driver/wire"
)

const (
	// shutdownGrace bounds how long Serve waits for in-flight per-conn
	// goroutines to drain after the context is cancelled.
	shutdownGrace = 2 * time.Second
	// helloDeadline is the read budget for the first frame; a backend
	// that doesn't say hello in this window is treated as dead.
	helloDeadline = 5 * time.Second
	// idleDeadline is the per-frame read budget after Hello. Telemetry
	// flows once per cycle (~15-20s); 5 minutes is generous for steady
	// state while still bounding stuck-conn resource usage.
	idleDeadline = 5 * time.Minute
)

// Server accepts framed protobuf wire connections on a UDS path and
// feeds each envelope into the Ingestor. One Server per process.
type Server struct {
	SocketPath string
	SocketMode os.FileMode
	Ingestor   *ingest.Ingestor

	connSeq atomic.Uint64
	// conns tracks live connections so Serve can close them on
	// shutdown to force-unblock in-flight reads. Keyed by conn id.
	conns sync.Map
}

// Serve binds the socket, accepts connections until ctx is cancelled,
// and returns after in-flight goroutines drain (bounded by
// shutdownGrace). The socket file is removed on return.
func (s *Server) Serve(ctx context.Context) error {
	if s.Ingestor == nil {
		return errors.New("ipc.Server: Ingestor is nil")
	}
	if s.SocketMode == 0 {
		s.SocketMode = 0o600
	}

	if err := removeStaleSocket(s.SocketPath); err != nil {
		return fmt.Errorf("ipc.Serve: %w", err)
	}

	ln, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return fmt.Errorf("ipc.Serve: listen %s: %w", s.SocketPath, err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(s.SocketPath)
	}()

	if err := os.Chmod(s.SocketPath, s.SocketMode); err != nil {
		return fmt.Errorf("ipc.Serve: chmod %s: %w", s.SocketPath, err)
	}
	log.Printf("ipc: listening on %s (mode %o)", s.SocketPath, s.SocketMode)

	// Cancel-driven shutdown: closing the listener unblocks Accept,
	// and closing live conns unblocks in-flight reads.
	var wg sync.WaitGroup
	stopCtx, cancelChildren := context.WithCancel(ctx)
	defer cancelChildren()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
		s.conns.Range(func(_, v any) bool {
			if c, ok := v.(net.Conn); ok {
				_ = c.Close()
			}
			return true
		})
	}()

	for {
		c, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}
			log.Printf("ipc: accept: %v", err)
			continue
		}
		id := s.connSeq.Add(1)
		s.conns.Store(id, c)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer s.conns.Delete(id)
			s.handleConn(stopCtx, id, c)
		}()
	}

	// Bounded drain.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(shutdownGrace):
		log.Printf("ipc: shutdown grace expired; some connection goroutines may still be running")
	}
	return nil
}

func (s *Server) handleConn(ctx context.Context, id uint64, c net.Conn) {
	tag := fmt.Sprintf("conn#%d", id)
	defer func() {
		_ = c.Close()
		log.Printf("ipc %s: closed", tag)
	}()

	// First frame must be Hello, within helloDeadline.
	_ = c.SetReadDeadline(time.Now().Add(helloDeadline))
	var env wire.Envelope
	if err := wire.ReadFrame(c, &env); err != nil {
		log.Printf("ipc %s: hello read: %v", tag, err)
		return
	}
	hello := env.GetHello()
	if hello == nil {
		log.Printf("ipc %s: protocol error: first frame is not Hello", tag)
		return
	}
	backend := hello.GetBackend()
	log.Printf("ipc %s: hello backend=%q version=%q host=%q", tag, backend, hello.GetVersion(), hello.GetHostname())
	if err := s.Ingestor.Handle(ctx, backend, &env); err != nil {
		log.Printf("ipc %s: hello ingest: %v", tag, err)
	}

	// Steady-state loop. Each ReadFrame gets a fresh idleDeadline so a
	// silent peer is bounded. Ctx cancel is delivered via the parent
	// Serve closing this conn (see Serve's shutdown goroutine), which
	// unblocks the read with net.ErrClosed.
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = c.SetReadDeadline(time.Now().Add(idleDeadline))
		var next wire.Envelope
		if err := wire.ReadFrame(c, &next); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("ipc %s: read: %v", tag, err)
			return
		}
		if err := s.Ingestor.Handle(ctx, backend, &next); err != nil {
			// Per-event error: log and continue. Connection is still
			// synchronised on the next length prefix.
			log.Printf("ipc %s: ingest: %v", tag, err)
		}
	}
}

// removeStaleSocket unlinks an existing UDS at path if (and only if)
// it is in fact a socket. Refuses to clobber regular files / dirs /
// symlinks so a misconfigured -socket arg can't delete unrelated data.
// Uses Lstat so a symlinked-to-socket is also refused.
func removeStaleSocket(path string) error {
	st, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if st.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove non-socket at %s (mode=%s)", path, st.Mode())
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale socket %s: %w", path, err)
	}
	return nil
}

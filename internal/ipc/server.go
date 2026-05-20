// Package ipc hosts the inv-driver Unix-domain socket server. Backends
// (bus-mgr / ecu-zb) connect, send a Hello envelope, then stream
// telemetry frames. This is the typed ingress edge of the daemon —
// protocol checks live here, business logic lives in package ingest.
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

	"database/sql"

	"github.com/bolke/inv-driver/internal/events"
	"github.com/bolke/inv-driver/internal/ingest"
	"github.com/bolke/inv-driver/internal/store"
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
	// writeDeadline bounds each subscriber write. A wedged client must
	// not block fan-out beyond this.
	writeDeadline = 5 * time.Second
	// publisherOutCap is the per-publisher downstream queue depth.
	// Drops with a counter when full so a wedged backend can't stall
	// the driver.
	publisherOutCap = 64
)

// Server accepts framed protobuf wire connections on a UDS path and
// feeds each envelope into the Ingestor. One Server per process.
//
// Publisher, when non-nil, supplies the SUBSCRIBER fan-out. Connections
// that say Hello with Role=SUBSCRIBER subscribe to the publisher and
// receive decoded telemetry envelopes instead of feeding the ingest.
type Server struct {
	SocketPath string
	SocketMode os.FileMode
	Ingestor   *ingest.Ingestor
	Publisher  *events.Publisher
	// Store, when non-nil, is read on subscriber attach to push the
	// current InverterInfo snapshot (one Envelope_Info per inverter
	// row) before the live fanout stream starts. Late attachers see
	// current state without waiting on a fresh probe cycle.
	Store *store.Store

	// OnPublisherAttach, when non-nil, runs each time a backend
	// finishes its Hello as a publisher. Invoked in a fresh
	// goroutine; the callback is responsible for its own context
	// observance. Used by the cold-start probe to fire downstream
	// queries the moment a publisher comes online.
	OnPublisherAttach func(backend string)

	connSeq atomic.Uint64
	// conns tracks live connections so Serve can close them on
	// shutdown to force-unblock in-flight reads. Keyed by conn id.
	conns sync.Map

	pubMu      sync.RWMutex
	publishers map[string]*publisherConn
}

// publisherConn is a live publishing peer. The outbound channel is the
// only thread-safe handle into the per-conn writer goroutine; closing
// it tells the writer to drain and exit.
type publisherConn struct {
	backend string
	out     chan *wire.Envelope
	dropped atomic.Uint64
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
	role := hello.GetRole()
	log.Printf("ipc %s: hello backend=%q version=%q host=%q role=%s",
		tag, backend, hello.GetVersion(), hello.GetHostname(), role.String())
	if err := s.Ingestor.Handle(ctx, backend, &env); err != nil {
		log.Printf("ipc %s: hello ingest: %v", tag, err)
	}

	switch role {
	case wire.Role_SUBSCRIBER:
		s.handleSubscriber(ctx, tag, c)
	default:
		// ROLE_UNSPECIFIED and PUBLISHER both run the publisher loop;
		// peers that omit Role default to publishing.
		s.handlePublisher(ctx, tag, backend, c)
	}
}

// handlePublisher runs the steady-state ingest loop for a publishing
// backend AND a writer goroutine for downstream envelopes addressed
// to this backend. Each ReadFrame gets a fresh idleDeadline so a
// silent peer is bounded. Ctx cancel is delivered via the parent
// Serve closing this conn (see Serve's shutdown goroutine), which
// unblocks the read with net.ErrClosed.
func (s *Server) handlePublisher(ctx context.Context, tag, backend string, c net.Conn) {
	pc := &publisherConn{
		backend: backend,
		out:     make(chan *wire.Envelope, publisherOutCap),
	}
	s.registerPublisher(tag, pc)

	if cb := s.OnPublisherAttach; cb != nil {
		go cb(backend)
	}

	writerDone := make(chan struct{})
	go s.publisherWriter(tag, c, pc, writerDone)
	defer func() {
		// Unregister and close the outbound channel atomically so a
		// concurrent SendToBackend never observes the channel after
		// it has been closed. Closing tells the writer to drain and
		// exit; the read loop has already signalled exit by reading
		// past the bottom of the for loop into the deferred close.
		s.unregisterAndClose(pc)
		<-writerDone
	}()

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
			// synchronised on the next length prefix. Unknown body
			// types fall through here; debug-level logging keeps the
			// noise floor reasonable for benign cases (Hello echoes,
			// future fields).
			log.Printf("ipc %s: ingest: %v", tag, err)
		}
	}
}

// publisherWriter drains the per-publisher outbound channel onto the
// conn. Exits when the channel is closed or a write fails.
func (s *Server) publisherWriter(tag string, c net.Conn, pc *publisherConn, done chan<- struct{}) {
	defer close(done)
	for env := range pc.out {
		_ = c.SetWriteDeadline(time.Now().Add(writeDeadline))
		if err := wire.WriteFrame(c, env); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				log.Printf("ipc %s: publisher write: %v", tag, err)
			}
			// Drain remaining queued envelopes so close(pc.out)
			// in the caller's defer can complete without panicking
			// on a closed conn (the writes will keep failing fast
			// after SetWriteDeadline).
			for range pc.out {
			}
			return
		}
	}
}

func (s *Server) registerPublisher(tag string, pc *publisherConn) {
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	if s.publishers == nil {
		s.publishers = make(map[string]*publisherConn)
	}
	if existing, ok := s.publishers[pc.backend]; ok && existing != pc {
		log.Printf("ipc %s: duplicate publisher backend=%q; replacing previous", tag, pc.backend)
	}
	s.publishers[pc.backend] = pc
}

// unregisterAndClose removes pc from the publisher map and closes its
// outbound channel as a single critical section. Senders holding
// pubMu.RLock therefore see either the live registration (and complete
// their send) or no registration at all — never a registered-but-closed
// channel.
func (s *Server) unregisterAndClose(pc *publisherConn) {
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	if cur, ok := s.publishers[pc.backend]; ok && cur == pc {
		delete(s.publishers, pc.backend)
	}
	close(pc.out)
}

// HasPublisher reports whether a publisher with the given backend
// name is currently registered. Primarily useful for test
// synchronisation.
func (s *Server) HasPublisher(backend string) bool {
	s.pubMu.RLock()
	defer s.pubMu.RUnlock()
	_, ok := s.publishers[backend]
	return ok
}

// SendToBackend enqueues an envelope for delivery to the named
// publisher. Returns false if no live publisher with that backend is
// registered, or if the per-publisher queue is full (the dropped
// counter is bumped in that case). The send is performed while
// holding pubMu (read), serialising it against unregisterPublisher so
// the outbound channel can never be closed while a send is in flight.
func (s *Server) SendToBackend(backend string, env *wire.Envelope) bool {
	if env == nil {
		return false
	}
	s.pubMu.RLock()
	defer s.pubMu.RUnlock()
	pc := s.publishers[backend]
	if pc == nil {
		return false
	}
	select {
	case pc.out <- env:
		return true
	default:
		pc.dropped.Add(1)
		log.Printf("ipc: send to backend=%q dropped (queue full); total dropped=%d",
			backend, pc.dropped.Load())
		return false
	}
}

// handleSubscriber feeds decoded envelopes from the in-process
// Publisher to the connected client. The client is expected to be
// read-only on the wire after Hello; we don't enforce that beyond
// closing the conn on any read error.
func (s *Server) handleSubscriber(ctx context.Context, tag string, c net.Conn) {
	if s.Publisher == nil {
		log.Printf("ipc %s: subscriber requested but Publisher is nil; dropping", tag)
		return
	}
	ch, unsubscribe := s.Publisher.Subscribe()
	defer unsubscribe()
	log.Printf("ipc %s: subscriber attached", tag)

	// Push the current InverterInfo snapshot before the live stream.
	// Subscribe-first / replay-second ordering: any live update that
	// fires during replay queues in ch and is drained by the loop
	// below. Subscriber's "latest wins by ts_ms" semantics handle a
	// live envelope that re-asserts replayed state.
	if err := s.replaySubscriberState(ctx, c); err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
			log.Printf("ipc %s: replay: %v", tag, err)
		}
		return
	}

	// Run a read-pump in a separate goroutine. A subscriber is
	// read-silent after Hello; any inbound frame is a protocol
	// violation, and a read error (EOF, closed conn) is the canonical
	// disconnect signal.
	readErr := make(chan error, 1)
	go func() {
		var inbound wire.Envelope
		_ = c.SetReadDeadline(time.Time{})
		if err := wire.ReadFrame(c, &inbound); err != nil {
			readErr <- err
			return
		}
		readErr <- errors.New("subscriber sent unexpected frame after hello")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-readErr:
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("ipc %s: subscriber read: %v; closing", tag, err)
			return
		case env, ok := <-ch:
			if !ok {
				// Publisher evicted us as slow. Drop the conn so the
				// client reconnects fresh.
				log.Printf("ipc %s: subscriber evicted (slow)", tag)
				return
			}
			_ = c.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := wire.WriteFrame(c, env); err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
					log.Printf("ipc %s: subscriber write: %v", tag, err)
				}
				return
			}
		}
	}
}

// replaySubscriberState writes one Envelope_Info per known inverter
// to conn. Called once before the live fanout starts so a subscriber
// that joins after the probe has settled still sees current identity
// state. Skipped silently when Store is nil.
func (s *Server) replaySubscriberState(ctx context.Context, conn net.Conn) error {
	if s.Store == nil {
		return nil
	}
	rows, err := s.Store.DB().QueryContext(ctx, `
SELECT uid, short_addr, model_code, software_version, phase,
       zigbee_bound, turned_off_rpt, last_seen_ms
FROM   inverters
ORDER  BY uid`)
	if err != nil {
		return fmt.Errorf("query inverters: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var (
			uid                                                       string
			shortAddr, model, sw, phase, zigbee, rptOff, lastSeen     sql.NullInt64
		)
		if err := rows.Scan(&uid, &shortAddr, &model, &sw, &phase, &zigbee, &rptOff, &lastSeen); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		info := &wire.InverterInfo{
			TsMs:    lastSeen.Int64,
			PeerUid: uid,
		}
		if shortAddr.Valid {
			info.ShortAddr = uint32(shortAddr.Int64)
		}
		if model.Valid {
			v := uint32(model.Int64)
			info.ModelCode = &v
		}
		if sw.Valid {
			v := uint32(sw.Int64)
			info.SoftwareVersion = &v
		}
		if phase.Valid {
			v := uint32(phase.Int64)
			info.Phase = &v
		}
		if zigbee.Valid {
			v := zigbee.Int64 != 0
			info.ZigbeeBound = &v
		}
		if rptOff.Valid {
			v := rptOff.Int64 != 0
			info.TurnedOffRpt = &v
		}
		env := &wire.Envelope{Body: &wire.Envelope_Info{Info: info}}
		_ = conn.SetWriteDeadline(time.Now().Add(writeDeadline))
		if err := wire.WriteFrame(conn, env); err != nil {
			return err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows: %w", err)
	}
	if count > 0 {
		log.Printf("ipc: replayed %d InverterInfo row(s) to subscriber", count)
	}

	// Follow with one FleetSummary (nameplate + energy aggregates)
	// so the subscriber doesn't have to wait for the next telemetry-
	// or inventory-change event. Errors here are non-fatal.
	if s.Ingestor == nil {
		return nil
	}
	fleet := s.Ingestor.BuildFleetSummary(ctx)
	if fleet == nil || fleet.GetInverterCount() == 0 {
		return nil
	}
	env := &wire.Envelope{Body: &wire.Envelope_Fleet{Fleet: fleet}}
	_ = conn.SetWriteDeadline(time.Now().Add(writeDeadline))
	if err := wire.WriteFrame(conn, env); err != nil {
		return err
	}
	log.Printf("ipc: replayed FleetSummary (nameplate=%dW count=%d lifetime=%dWh today=%dWh) to subscriber",
		fleet.GetNameplateTotalW(), fleet.GetInverterCount(),
		fleet.GetLifetimeWh(), fleet.GetTodayWh())
	return nil
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

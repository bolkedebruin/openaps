package tap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// Server runs the Unix STREAM and TCP listeners that fan the
// broadcaster's stream out to attached consumers. Both listeners are
// optional — pass an empty path/addr to skip.
type Server struct {
	br *Broadcaster

	unixPath string
	tcpAddr  string

	wg        sync.WaitGroup
	listeners []net.Listener

	connsMu     sync.Mutex
	activeConns map[net.Conn]struct{}
}

func NewServer(br *Broadcaster, unixPath, tcpAddr string) *Server {
	return &Server{
		br:          br,
		unixPath:    unixPath,
		tcpAddr:     tcpAddr,
		activeConns: make(map[net.Conn]struct{}),
	}
}

// Start binds both listeners and spawns accept loops. It returns once
// the listeners are bound and serving (or an error from the first
// failed bind).
func (s *Server) Start(ctx context.Context) error {
	if s.unixPath != "" {
		if err := s.startUnix(ctx); err != nil {
			s.Stop()
			return fmt.Errorf("unix listener: %w", err)
		}
	}
	if s.tcpAddr != "" {
		if err := s.startTCP(ctx); err != nil {
			s.Stop()
			return fmt.Errorf("tcp listener: %w", err)
		}
	}
	return nil
}

func (s *Server) startUnix(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.unixPath), 0o755); err != nil {
		return err
	}
	// Stale socket from a prior crashed run: remove if present.
	_ = os.Remove(s.unixPath)
	l, err := net.Listen("unix", s.unixPath)
	if err != nil {
		return err
	}
	s.listeners = append(s.listeners, l)
	s.wg.Add(1)
	go s.acceptLoop(ctx, l, "unix")
	log.Printf("tap: unix listener at %s", s.unixPath)
	return nil
}

func (s *Server) startTCP(ctx context.Context) error {
	l, err := net.Listen("tcp", s.tcpAddr)
	if err != nil {
		return err
	}
	s.listeners = append(s.listeners, l)
	s.wg.Add(1)
	go s.acceptLoop(ctx, l, "tcp")
	log.Printf("tap: tcp listener at %s", l.Addr())
	return nil
}

func (s *Server) acceptLoop(ctx context.Context, l net.Listener, label string) {
	defer s.wg.Done()
	for {
		conn, err := l.Accept()
		if err != nil {
			if isClosedErr(err) || ctx.Err() != nil {
				return
			}
			log.Printf("tap %s: accept: %v", label, err)
			return
		}
		log.Printf("tap %s: subscriber attached %s (now %d)",
			label, conn.RemoteAddr(), s.br.SubscriberCount()+1)
		s.wg.Add(1)
		go s.serveConn(label, conn)
	}
}

func (s *Server) serveConn(label string, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	s.connsMu.Lock()
	s.activeConns[conn] = struct{}{}
	s.connsMu.Unlock()
	defer func() {
		s.connsMu.Lock()
		delete(s.activeConns, conn)
		s.connsMu.Unlock()
	}()

	// Don't read from the subscriber connection. Tools like nc may
	// half-close (shutdown(WR)) when their stdin reaches EOF, which
	// looks like a peer-close to a Read but the connection is still
	// usable for writes. The writer goroutine inside Attach is the
	// authoritative liveness signal: it exits on Write error.
	detach, done := s.br.Attach(conn)
	defer detach()
	<-done
	log.Printf("tap %s: subscriber detached %s", label, conn.RemoteAddr())
}

// Stop closes all listeners and forcibly disconnects active
// subscribers. Without forcibly closing the connections, an
// idle-but-attached client (typical of nc) would leave serveConn's
// `<-done` blocked indefinitely and Stop() would hang.
func (s *Server) Stop() {
	for _, l := range s.listeners {
		_ = l.Close()
	}

	s.connsMu.Lock()
	for c := range s.activeConns {
		_ = c.Close()
	}
	s.connsMu.Unlock()

	s.wg.Wait()
	if s.unixPath != "" {
		_ = os.Remove(s.unixPath)
	}
}

func isClosedErr(err error) bool {
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	return false
}

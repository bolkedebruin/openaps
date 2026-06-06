package recoveryd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/bolkedebruin/openaps/internal/udsutil"
	"github.com/bolkedebruin/openaps/wire"
)

// idleDeadline bounds a single request/response exchange so a stuck peer
// can't pin a goroutine forever.
const idleDeadline = 30 * time.Second

// Server is the local protobuf UDS that exposes the Manager to ecu-web.
// One request, one response, per connection (the client may reuse a
// connection for several exchanges). Every connection is gated on a
// peer-cred uid-0 check (Linux); only root may read or mutate the access
// plane.
type Server struct {
	Manager    *Manager
	SocketPath string
	// SocketMode is the UDS file mode; zero means 0600.
	SocketMode os.FileMode
	// AllowUID is the only peer uid permitted to connect. Zero (the default,
	// and what production uses) means root. Tests set it to the current uid
	// since CI runs as a non-root user.
	AllowUID int

	conns   sync.Map
	connSeq uint64
	seqMu   sync.Mutex
}

// Serve listens on the UDS until ctx is cancelled. The socket is created
// 0600 and re-checked on every connection via SO_PEERCRED.
func (s *Server) Serve(ctx context.Context) error {
	if s.Manager == nil {
		return errors.New("recoveryd.Server: Manager is nil")
	}
	if s.SocketMode == 0 {
		s.SocketMode = 0o600
	}
	if err := udsutil.RemoveStaleSocket(s.SocketPath); err != nil {
		return fmt.Errorf("recoveryd.Serve: %w", err)
	}
	ln, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return fmt.Errorf("recoveryd.Serve: listen %s: %w", s.SocketPath, err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(s.SocketPath)
	}()
	if err := os.Chmod(s.SocketPath, s.SocketMode); err != nil {
		return fmt.Errorf("recoveryd.Serve: chmod %s: %w", s.SocketPath, err)
	}
	log.Printf("recoveryd: listening on %s (mode %o)", s.SocketPath, s.SocketMode)

	var wg sync.WaitGroup
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
			log.Printf("recoveryd: accept: %v", err)
			continue
		}
		s.seqMu.Lock()
		s.connSeq++
		id := s.connSeq
		s.seqMu.Unlock()
		s.conns.Store(id, c)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer s.conns.Delete(id)
			s.handleConn(ctx, c)
		}()
	}
	wg.Wait()
	return nil
}

// handleConn enforces the uid-0 gate then serves request/response
// exchanges until the peer closes or an error occurs.
func (s *Server) handleConn(ctx context.Context, c net.Conn) {
	defer c.Close()

	uid := udsutil.PeerUID(c)
	// On Linux uid is the real peer uid; require AllowUID (0/root by default).
	// On non-Linux the gate is unavailable (uid == -1) and we allow it.
	if uid >= 0 && uid != s.AllowUID {
		log.Printf("recoveryd: refused peer uid=%d (allowed uid=%d)", uid, s.AllowUID)
		_ = c.SetWriteDeadline(time.Now().Add(idleDeadline))
		_ = wire.WriteMessage(c, &wire.AccessResponse{Ok: false, Error: fmt.Sprintf("refused: peer uid=%d not allowed", uid)})
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = c.SetReadDeadline(time.Now().Add(idleDeadline))
		var req wire.AccessRequest
		if err := wire.ReadMessage(c, &req); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("recoveryd: read: %v", err)
			return
		}
		resp := s.dispatch(&req)
		_ = c.SetWriteDeadline(time.Now().Add(idleDeadline))
		if err := wire.WriteMessage(c, resp); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("recoveryd: write: %v", err)
			}
			return
		}
	}
}

// dispatch runs one AccessRequest against the Manager and builds the
// reply. It never panics on a nil oneof — an unset op is a clean error.
func (s *Server) dispatch(req *wire.AccessRequest) *wire.AccessResponse {
	switch op := req.GetOp().(type) {
	case *wire.AccessRequest_ListKeys:
		a := s.Manager.List()
		return s.stateResp(a, true)
	case *wire.AccessRequest_AddKey:
		if _, err := s.Manager.AddKey(op.AddKey.GetPubkey(), op.AddKey.GetComment()); err != nil {
			return &wire.AccessResponse{Ok: false, Error: err.Error()}
		}
		return s.stateResp(s.Manager.List(), true)
	case *wire.AccessRequest_RemoveKey:
		if err := s.Manager.RemoveKey(op.RemoveKey.GetFingerprint()); err != nil {
			return &wire.AccessResponse{Ok: false, Error: err.Error()}
		}
		return s.stateResp(s.Manager.List(), true)
	case *wire.AccessRequest_Status:
		return s.stateResp(s.Manager.List(), false)
	default:
		return &wire.AccessResponse{Ok: false, Error: "empty or unknown op"}
	}
}

// stateResp builds an ok response from access state. withKeys controls
// whether the full key list is included (ListKeys/AddKey/RemoveKey) or
// just the count (Status).
func (s *Server) stateResp(a Access, withKeys bool) *wire.AccessResponse {
	resp := &wire.AccessResponse{
		Ok:       true,
		Provider: string(a.Provider),
		HostUser: a.HostUser,
		KeyCount: uint32(len(a.SSHKeys)),
	}
	if withKeys {
		for _, k := range a.SSHKeys {
			resp.Keys = append(resp.Keys, &wire.SshKey{
				Pubkey:      k.Pubkey,
				Comment:     k.Comment,
				AddedMs:     k.AddedMs,
				Fingerprint: k.Fingerprint,
			})
		}
	}
	return resp
}

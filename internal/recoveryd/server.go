package recoveryd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
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
	// Ensure the socket's parent dir exists (defensive — the default
	// /var/run exists on the ECU and resolves to /run on modern systems, but
	// an operator-supplied -socket path may point somewhere that doesn't).
	if dir := filepath.Dir(s.SocketPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("recoveryd.Serve: mkdir %s: %w", dir, err)
		}
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
	slog.Info("recoveryd: listening", "socket", s.SocketPath, "mode", fmt.Sprintf("%o", s.SocketMode))

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
			slog.Error("recoveryd: accept", "err", err)
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
		slog.Warn("recoveryd: refused peer", "uid", uid, "allowed_uid", s.AllowUID)
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
			slog.Warn("recoveryd: read", "err", err)
			return
		}
		resp := s.dispatch(&req)
		_ = c.SetWriteDeadline(time.Now().Add(idleDeadline))
		if err := wire.WriteMessage(c, resp); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				slog.Warn("recoveryd: write", "err", err)
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
		keys, err := s.Manager.ListKeys()
		if err != nil {
			return &wire.AccessResponse{Ok: false, Error: err.Error()}
		}
		return s.stateResp(keys, true)
	case *wire.AccessRequest_AddKey:
		if _, err := s.Manager.AddKey(op.AddKey.GetPubkey(), op.AddKey.GetComment()); err != nil {
			return &wire.AccessResponse{Ok: false, Error: err.Error()}
		}
		return s.listResp()
	case *wire.AccessRequest_RemoveKey:
		if err := s.Manager.RemoveKey(op.RemoveKey.GetFingerprint()); err != nil {
			return &wire.AccessResponse{Ok: false, Error: err.Error()}
		}
		return s.listResp()
	case *wire.AccessRequest_Status:
		keys, err := s.Manager.ListKeys()
		if err != nil {
			return &wire.AccessResponse{Ok: false, Error: err.Error()}
		}
		return s.stateResp(keys, false)
	default:
		return &wire.AccessResponse{Ok: false, Error: "empty or unknown op"}
	}
}

// listResp re-reads the file and builds a keys response after a mutation.
func (s *Server) listResp() *wire.AccessResponse {
	keys, err := s.Manager.ListKeys()
	if err != nil {
		return &wire.AccessResponse{Ok: false, Error: err.Error()}
	}
	return s.stateResp(keys, true)
}

// stateResp builds an ok response from the parsed key list. withKeys
// controls whether the full list is included (ListKeys/AddKey/RemoveKey) or
// just the count (Status). The provider field reports the managed
// authorized_keys path and host_user the chown-user (empty = root).
func (s *Server) stateResp(keys []Key, withKeys bool) *wire.AccessResponse {
	p := s.Manager.Provider()
	resp := &wire.AccessResponse{
		Ok:       true,
		Provider: p.path(),
		HostUser: p.ChownUser,
		KeyCount: uint32(len(keys)),
	}
	if withKeys {
		for _, k := range keys {
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

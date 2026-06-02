// Package systemstatus answers SystemStatusRequest: it tracks the peers
// currently connected on the UDS (recorded by the ipc server from each
// Hello) and reports ECU identity supplied by inv-driver's settings. It
// is a read-only view of "who is talking to inv-driver and on what box".
package systemstatus

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// peer is one live UDS connection as seen at Hello time.
type peer struct {
	backend     string
	version     string
	hostname    string
	role        string
	peerUID     int
	connectedAt int64
}

// Registry is the concurrency-safe set of connected peers, keyed by the
// ipc server's per-connection id. The ipc server calls Record on Hello
// and Remove on disconnect.
type Registry struct {
	mu    sync.RWMutex
	peers map[uint64]peer
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{peers: make(map[uint64]peer)}
}

// Record adds or updates the peer for connection id.
func (r *Registry) Record(id uint64, backend, version, hostname, role string, peerUID int, atMs int64) {
	r.mu.Lock()
	r.peers[id] = peer{
		backend:     backend,
		version:     version,
		hostname:    hostname,
		role:        role,
		peerUID:     peerUID,
		connectedAt: atMs,
	}
	r.mu.Unlock()
}

// Remove drops the peer for connection id (idempotent).
func (r *Registry) Remove(id uint64) {
	r.mu.Lock()
	delete(r.peers, id)
	r.mu.Unlock()
}

func (r *Registry) snapshot() []peer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]peer, 0, len(r.peers))
	for _, p := range r.peers {
		out = append(out, p)
	}
	return out
}

// Manager implements the SystemStatusRequest handler. It embeds the
// Registry so it can be used both as the ipc peer recorder and the
// request handler.
type Manager struct {
	*Registry
	Hostname    string          // this host's OS name, stamped into EcuIdentity
	Controllers map[string]bool // backends on the controller allow-list (for the controller flag)

	// Identity, when set, supplies operator-owned ECU identity from
	// inv-driver's settings file. Nil leaves the operator fields blank
	// (e.g. before the settings file is written).
	Identity func() *wire.EcuIdentity
}

// NewManager builds a Manager with a fresh registry.
func NewManager(hostname string, controllers []string) *Manager {
	set := make(map[string]bool, len(controllers))
	for _, c := range controllers {
		set[c] = true
	}
	return &Manager{
		Registry:    NewRegistry(),
		Hostname:    hostname,
		Controllers: set,
	}
}

// Handle builds the SystemStatusResponse.
func (m *Manager) Handle(_ context.Context, _ *wire.SystemStatusRequest) *wire.SystemStatusResponse {
	return &wire.SystemStatusResponse{
		Ok:    true,
		TsMs:  time.Now().UnixMilli(),
		Ecu:   m.identity(),
		Peers: m.peerStatuses(),
	}
}

func (m *Manager) peerStatuses() []*wire.PeerStatus {
	peers := m.snapshot()
	out := make([]*wire.PeerStatus, 0, len(peers))
	for _, p := range peers {
		out = append(out, &wire.PeerStatus{
			Backend:       p.backend,
			Version:       p.version,
			Hostname:      p.hostname,
			Role:          p.role,
			ConnectedAtMs: p.connectedAt,
			PeerUid:       int32(p.peerUID),
			Controller:    m.Controllers[p.backend],
		})
	}
	// Stable order (map iteration is randomised): by backend, then role,
	// then connect time, so the UI list doesn't reshuffle between polls.
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Backend != b.Backend {
			return a.Backend < b.Backend
		}
		if a.Role != b.Role {
			return a.Role < b.Role
		}
		return a.ConnectedAtMs < b.ConnectedAtMs
	})
	return out
}

// identity merges the operator-owned identity (from inv-driver settings,
// via the injected Identity func) with the OS hostname.
func (m *Manager) identity() *wire.EcuIdentity {
	var id *wire.EcuIdentity
	if m.Identity != nil {
		id = m.Identity()
	}
	if id == nil {
		id = &wire.EcuIdentity{}
	}
	if id.Hostname == "" {
		id.Hostname = m.Hostname
	}
	return id
}

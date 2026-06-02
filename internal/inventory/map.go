package inventory

import (
	"strings"
	"sync"
)

// Map maintains a runtime peer-UID -> ZigBee short-address index.
// Populated as inbound reply frames are observed (each L1 reply carries
// the 6-byte peer UID and the source SA in the outer envelope).
// Lookups serve downstream Send dispatch, which addresses inverters by
// UID but needs the SA for the outer envelope.
type Map struct {
	mu      sync.RWMutex
	saByUID map[string]uint16
}

// NewMap returns an empty cross-index.
func NewMap() *Map {
	return &Map{saByUID: make(map[string]uint16)}
}

// Record stores peerUID -> sa. peerUID is normalised to lower-case hex
// so callers can mix case without missing a hit.
func (m *Map) Record(peerUID string, sa uint16) {
	if m == nil || peerUID == "" {
		return
	}
	key := strings.ToLower(peerUID)
	m.mu.Lock()
	m.saByUID[key] = sa
	m.mu.Unlock()
}

// LookupSA returns the short-address last seen for peerUID. Lookup is
// case-insensitive.
func (m *Map) LookupSA(peerUID string) (uint16, bool) {
	if m == nil || peerUID == "" {
		return 0, false
	}
	key := strings.ToLower(peerUID)
	m.mu.RLock()
	sa, ok := m.saByUID[key]
	m.mu.RUnlock()
	return sa, ok
}

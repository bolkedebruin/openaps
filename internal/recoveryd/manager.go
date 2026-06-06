package recoveryd

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager is the policy layer over the store and renderer. It serialises
// mutations, validates and dedupes keys, persists the list, and rewrites
// authorized_keys after every change. It is the single owner of the
// access plane.
type Manager struct {
	store *Store
	rend  *Renderer
	mu    sync.Mutex
	// now is overridable in tests.
	now func() int64
}

// NewManager builds a Manager over a store and renderer. It does NOT
// render — call BootRender once at startup so authorized_keys reflects
// the persisted list before dropbear binds.
func NewManager(store *Store, rend *Renderer) *Manager {
	return &Manager{
		store: store,
		rend:  rend,
		now:   func() int64 { return time.Now().UnixMilli() },
	}
}

// BootRender renders authorized_keys from the persisted list at startup.
// This is the anti-brick guarantee: even with no client ever connecting,
// the box's keys are in place before dropbear starts.
func (m *Manager) BootRender() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.store.Get()
	if err := m.rend.Render(a); err != nil {
		return err
	}
	log.Printf("recoveryd: boot-render provider=%s keys=%d", a.Provider, len(a.SSHKeys))
	return nil
}

// List returns the current access state.
func (m *Manager) List() Access {
	return m.store.Get()
}

// AddKey validates pubkey, dedupes by fingerprint, persists, and
// re-renders authorized_keys. Adding an already-present key is a no-op
// success (idempotent). Returns the parsed key.
func (m *Manager) AddKey(pubkey, comment string) (Key, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k, err := ParseKey(pubkey, comment)
	if err != nil {
		return Key{}, err
	}
	a := m.store.Get()
	for _, existing := range a.SSHKeys {
		if existing.Fingerprint == k.Fingerprint {
			log.Printf("recoveryd: add key %s already present (no-op)", k.Fingerprint)
			return existing, nil
		}
	}
	k.AddedMs = m.now()
	a.SSHKeys = append(a.SSHKeys, k)
	if err := m.persistAndRender(a); err != nil {
		return Key{}, err
	}
	log.Printf("recoveryd: ADDED key fp=%s comment=%q (now %d keys)", k.Fingerprint, k.Comment, len(a.SSHKeys))
	return k, nil
}

// RemoveKey removes the key with the given fingerprint, persists, and
// re-renders. Removing an absent key is an error so the caller can tell
// the operator it wasn't found.
func (m *Manager) RemoveKey(fingerprint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a := m.store.Get()
	idx := -1
	for i, k := range a.SSHKeys {
		if k.Fingerprint == fingerprint {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("no key with fingerprint %s", fingerprint)
	}
	a.SSHKeys = append(a.SSHKeys[:idx], a.SSHKeys[idx+1:]...)
	if err := m.persistAndRender(a); err != nil {
		return err
	}
	log.Printf("recoveryd: REMOVED key fp=%s (now %d keys)", fingerprint, len(a.SSHKeys))
	return nil
}

// persistAndRender saves the list then rewrites authorized_keys. The
// store is the source of truth, so it is written first; a render failure
// after a successful save still leaves the next BootRender able to
// recover the file from disk.
func (m *Manager) persistAndRender(a Access) error {
	if err := m.store.Save(a); err != nil {
		return err
	}
	return m.rend.Render(a)
}

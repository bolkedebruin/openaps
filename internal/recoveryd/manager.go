package recoveryd

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Manager is the policy layer over the provider's authorized_keys file. It
// serialises mutations, validates and dedupes keys, and rewrites the file
// atomically after every change. authorized_keys is the single source of
// truth — the Manager holds no key list of its own; it parses the file on
// every read.
type Manager struct {
	prov *Provider
	mu   sync.Mutex
	// now is overridable in tests.
	now func() int64
}

// NewManager builds a Manager over a provider.
func NewManager(prov *Provider) *Manager {
	return &Manager{
		prov: prov,
		now:  func() int64 { return time.Now().UnixMilli() },
	}
}

// Boot ensures the .ssh dir exists and, when managing dropbear, the host
// key — then the daemon serves. There is nothing to render: authorized_keys
// already is the truth. A missing authorized_keys is fine; ListKeys returns
// empty and the operator adds keys via the web UI.
func (m *Manager) Boot() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.prov.ensureSSHDir(); err != nil {
		return err
	}
	if err := m.prov.ensureDropbearHostKey(); err != nil {
		return err
	}
	slog.Info("recoveryd: boot", "authorized_keys", m.prov.path(), "manage_dropbear", m.prov.ManageDropbear)
	return nil
}

// Provider returns the underlying provider (for status reporting).
func (m *Manager) Provider() *Provider { return m.prov }

// ListKeys parses the authorized_keys file and returns the current keys.
func (m *Manager) ListKeys() ([]Key, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prov.readKeys()
}

// AddKey validates pubkey, dedupes by fingerprint against the file,
// appends, and atomically rewrites authorized_keys. Adding an already-present
// key is a no-op success (idempotent). Returns the parsed key.
func (m *Manager) AddKey(pubkey, comment string) (Key, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k, err := ParseKey(pubkey, comment)
	if err != nil {
		return Key{}, err
	}
	keys, err := m.prov.readKeys()
	if err != nil {
		return Key{}, err
	}
	for _, existing := range keys {
		if existing.Fingerprint == k.Fingerprint {
			slog.Info("recoveryd: add key already present (no-op)", "fp", k.Fingerprint)
			return existing, nil
		}
	}
	k.AddedMs = m.now()
	keys = append(keys, k)
	if err := m.prov.writeKeys(keys); err != nil {
		return Key{}, err
	}
	slog.Info("recoveryd: ADDED key", "fp", k.Fingerprint, "comment", k.Comment, "total", len(keys))
	return k, nil
}

// RemoveKey drops the key with the given fingerprint and atomically
// rewrites authorized_keys. Removing an absent key is an error so the caller
// can tell the operator it wasn't found. Removing the LAST remaining key is
// refused — that would lock the operator out of the box. This is the only
// lockout guard the direct-file model needs.
func (m *Manager) RemoveKey(fingerprint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keys, err := m.prov.readKeys()
	if err != nil {
		return err
	}
	idx := -1
	for i, k := range keys {
		if k.Fingerprint == fingerprint {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("no key with fingerprint %s", fingerprint)
	}
	if len(keys) == 1 {
		return fmt.Errorf("refusing to remove the only key — you would lose access")
	}
	keys = append(keys[:idx], keys[idx+1:]...)
	if err := m.prov.writeKeys(keys); err != nil {
		return err
	}
	slog.Info("recoveryd: REMOVED key", "fp", fingerprint, "total", len(keys))
	return nil
}

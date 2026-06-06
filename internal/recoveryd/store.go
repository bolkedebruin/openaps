// Package recoveryd owns the box's SSH access plane: the authorized-key
// list, the provider that renders it into an authorized_keys file, and
// the local protobuf UDS that ecu-web drives. recoveryd is the single
// writer of authorized_keys; the fleet daemon (inv-driver) and the web
// console (ecu-web) never touch it directly.
package recoveryd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/bolkedebruin/openaps/internal/atomicfile"
)

// Provider selects how the key list is rendered.
//
//	openaps — render /root/.ssh/authorized_keys and ensure the dropbear
//	          host key exists (dropbear is the SSH server).
//	host    — render ~<host_user>/.ssh/authorized_keys for a host sshd;
//	          recoveryd does NOT manage a host key.
//	off     — render nothing (no-op).
type Provider string

const (
	ProviderOpenAPS Provider = "openaps"
	ProviderHost    Provider = "host"
	ProviderOff     Provider = "off"
)

// Key is one authorized key in the list. Fingerprint is the stable id
// (ssh.FingerprintSHA256) used for dedupe and removal.
type Key struct {
	Pubkey      string `json:"pubkey"`
	Comment     string `json:"comment"`
	AddedMs     int64  `json:"added_ms"`
	Fingerprint string `json:"fingerprint"`
}

// Access is the on-disk source of truth at /etc/recoveryd/access.json.
type Access struct {
	Provider Provider `json:"provider"`
	HostUser string   `json:"host_user"`
	SSHKeys  []Key    `json:"ssh_keys"`
}

// Store is the concurrency-safe owner of the access file.
type Store struct {
	path string
	mu   sync.RWMutex
	a    Access
}

// Open loads the access file at path. A missing file is not an error —
// it yields a default Access (provider "openaps", no keys) so a fresh
// box renders an empty /root/.ssh/authorized_keys at first boot.
func Open(path string) (*Store, error) {
	st := &Store{path: path, a: Access{Provider: ProviderOpenAPS}}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return st, nil
		}
		return nil, fmt.Errorf("recoveryd: read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, &st.a); err != nil {
		return nil, fmt.Errorf("recoveryd: parse %s: %w", path, err)
	}
	if st.a.Provider == "" {
		st.a.Provider = ProviderOpenAPS
	}
	return st, nil
}

// Get returns a copy of the current access state. The slice is cloned so
// callers can't mutate the store's backing array.
func (st *Store) Get() Access {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.a.clone()
}

// Save replaces the access state and persists it durably: atomicfile.Write
// fsyncs the temp file and the directory before/after the rename so a
// crash can't truncate the source of truth, mode 0600.
func (st *Store) Save(a Access) error {
	if a.Provider == "" {
		a.Provider = ProviderOpenAPS
	}
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicfile.Write(st.path, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("recoveryd: save %s: %w", st.path, err)
	}
	st.mu.Lock()
	st.a = a.clone()
	st.mu.Unlock()
	return nil
}

func (a Access) clone() Access {
	out := a
	if a.SSHKeys != nil {
		out.SSHKeys = make([]Key, len(a.SSHKeys))
		copy(out.SSHKeys, a.SSHKeys)
	}
	return out
}

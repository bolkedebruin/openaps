// Package recoveryd owns the box's SSH access plane: it reads and writes
// the real authorized_keys file directly and serves a local protobuf UDS
// that ecu-web drives. authorized_keys is the single source of truth —
// recoveryd parses it on every list, and every mutation is an atomic
// rewrite of that same file. The fleet daemon (inv-driver) and the web
// console (ecu-web) never touch it directly.
package recoveryd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bolkedebruin/openaps/internal/atomicfile"
)

// DefaultAuthorizedKeys is the authorized_keys file recoveryd owns when no
// -authorized-keys flag is given. It is root's key file for the openaps
// dropbear server.
const DefaultAuthorizedKeys = "/root/.ssh/authorized_keys"

// DefaultDropbearHostKey is the RSA host key dropbear reads. recoveryd
// ensures it exists when managing dropbear so dropbear can start;
// S98-dropbear does the same, idempotently.
const DefaultDropbearHostKey = "/etc/dropbear/dropbear_rsa_host_key"

// Provider names which authorized_keys file recoveryd owns and whether it
// manages the dropbear host key. It carries no key list of its own — the
// authorized_keys file at AuthorizedKeysPath is the source of truth. The
// provider is selected by cmd/recoveryd flags, not a runtime config file.
type Provider struct {
	// AuthorizedKeysPath is the file recoveryd reads and rewrites. Empty
	// uses DefaultAuthorizedKeys.
	AuthorizedKeysPath string
	// ChownUser, when non-empty, is the unix user that must own the .ssh
	// dir and authorized_keys file (the host provider, so a host sshd's
	// StrictModes accepts them). Empty leaves ownership as the writing
	// process (root, the openaps dropbear case).
	ChownUser string
	// ManageDropbear ensures the dropbear RSA host key exists at boot. The
	// openaps case sets it true; the host/Pi case sets it false to defer to
	// the host sshd.
	ManageDropbear bool
	// DropbearKeyPath is the host key ensured when ManageDropbear is true.
	// Empty uses DefaultDropbearHostKey.
	DropbearKeyPath string

	// dropbearKeyGen runs dropbearkey to create a host key. Overridable in
	// tests; nil uses the real dropbearkey binary.
	dropbearKeyGen func(path string) error
	// lookupUser resolves a host user. nil uses os/user.Lookup.
	lookupUser func(name string) (*user.User, error)
}

// path returns the resolved authorized_keys path.
func (p *Provider) path() string {
	if p.AuthorizedKeysPath == "" {
		return DefaultAuthorizedKeys
	}
	return p.AuthorizedKeysPath
}

// resolveOwner returns the uid/gid the .ssh dir and authorized_keys must
// be chowned to, or (-1, -1) when ownership should be left as-is (root).
func (p *Provider) resolveOwner() (uid, gid int, err error) {
	if p.ChownUser == "" {
		return -1, -1, nil
	}
	lu := p.lookupUser
	if lu == nil {
		lu = user.Lookup
	}
	u, err := lu(p.ChownUser)
	if err != nil {
		return -1, -1, fmt.Errorf("recoveryd: lookup chown-user %q: %w", p.ChownUser, err)
	}
	uid, _ = strconv.Atoi(u.Uid)
	gid, _ = strconv.Atoi(u.Gid)
	return uid, gid, nil
}

// readKeys parses the provider's authorized_keys file into a key list. A
// missing file yields an empty list (a fresh box with no keys yet); blank
// lines and # comment lines are skipped. A malformed key line is an error
// so a corrupt file is surfaced rather than silently dropping access.
func (p *Provider) readKeys() ([]Key, error) {
	b, err := os.ReadFile(p.path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("recoveryd: read %s: %w", p.path(), err)
	}
	var keys []Key
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, perr := ParseKey(line, "")
		if perr != nil {
			return nil, fmt.Errorf("recoveryd: %s: %w", p.path(), perr)
		}
		keys = append(keys, k)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("recoveryd: scan %s: %w", p.path(), err)
	}
	return keys, nil
}

// writeKeys atomically rewrites the provider's authorized_keys from keys.
// The .ssh dir is ensured 0700 and the file is 0600; under a chown-user
// both are owned by that user. The temp file is fsync'd before the rename
// and the directory fsync'd after, so a power loss can never publish a
// zero-length or partial authorized_keys.
func (p *Provider) writeKeys(keys []Key) error {
	path := p.path()
	sshDir := filepath.Dir(path)
	uid, gid, err := p.resolveOwner()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("recoveryd: mkdir %s: %w", sshDir, err)
	}
	if uid >= 0 && gid >= 0 {
		// Under a chown-user the .ssh dir must be owned by that user or some
		// sshd StrictModes configs reject it. A half-chowned .ssh (root dir,
		// user file) is a hard error, not best-effort.
		if err := os.Chown(sshDir, uid, gid); err != nil {
			return fmt.Errorf("recoveryd: chown %s: %w", sshDir, err)
		}
	}
	var buf []byte
	for _, k := range keys {
		buf = append(buf, renderLine(k)...)
		buf = append(buf, '\n')
	}
	// Both branches share atomicfile's durable temp+fsync+rename+dir-fsync;
	// the host provider only differs by chowning the temp to the host user
	// before the rename. uid/gid below zero (the root case) skips the chown.
	return atomicfile.WriteOwned(path, buf, 0o600, uid, gid)
}

// ensureSSHDir creates the parent .ssh dir 0700, chowned to the host user
// under a chown-user. recoveryd calls this at boot so the dir exists even
// before the first key is added.
func (p *Provider) ensureSSHDir() error {
	uid, gid, err := p.resolveOwner()
	if err != nil {
		return err
	}
	sshDir := filepath.Dir(p.path())
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("recoveryd: mkdir %s: %w", sshDir, err)
	}
	if uid >= 0 && gid >= 0 {
		if err := os.Chown(sshDir, uid, gid); err != nil {
			return fmt.Errorf("recoveryd: chown %s: %w", sshDir, err)
		}
	}
	return nil
}

// ensureDropbearHostKey generates the dropbear RSA host key if missing.
// It is a no-op when the provider does not manage dropbear. Idempotent: a
// present key is left untouched.
func (p *Provider) ensureDropbearHostKey() error {
	if !p.ManageDropbear {
		return nil
	}
	path := p.DropbearKeyPath
	if path == "" {
		path = DefaultDropbearHostKey
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("recoveryd: stat %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("recoveryd: mkdir %s: %w", filepath.Dir(path), err)
	}
	gen := p.dropbearKeyGen
	if gen == nil {
		gen = realDropbearKeyGen
	}
	if err := gen(path); err != nil {
		return fmt.Errorf("recoveryd: generate dropbear host key %s: %w", path, err)
	}
	return nil
}

// realDropbearKeyGen invokes dropbearkey to create an RSA host key. It
// tries the bundled path then falls back to PATH.
func realDropbearKeyGen(path string) error {
	bin := "/usr/local/sbin/dropbearkey"
	if _, err := os.Stat(bin); err != nil {
		bin = "dropbearkey"
	}
	cmd := exec.Command(bin, "-t", "rsa", "-f", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %v: %s", bin, err, out)
	}
	return nil
}

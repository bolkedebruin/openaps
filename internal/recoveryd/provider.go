package recoveryd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/bolkedebruin/openaps/internal/atomicfile"
)

// DefaultDropbearHostKey is the RSA host key dropbear reads. recoveryd
// ensures it exists under the openaps provider so dropbear can start;
// S98-dropbear does the same, idempotently.
const DefaultDropbearHostKey = "/etc/dropbear/dropbear_rsa_host_key"

// Renderer turns an Access into an authorized_keys file on disk. It is
// the only thing that writes authorized_keys.
type Renderer struct {
	// DropbearKeyPath is the dropbear RSA host key ensured under the
	// openaps provider. Empty uses DefaultDropbearHostKey.
	DropbearKeyPath string
	// dropbearKeyGen runs dropbearkey to create a host key. Overridable
	// in tests; nil uses the real dropbearkey binary.
	dropbearKeyGen func(path string) error
	// rootHome is the home dir used for the openaps provider. Empty uses
	// "/root". Overridable in tests.
	rootHome string
	// lookupUser resolves a host user. nil uses os/user.Lookup.
	lookupUser func(name string) (*user.User, error)
}

// Render writes authorized_keys for the given access state and, under
// the openaps provider, ensures the dropbear host key exists. It is a
// full rewrite from the key list (never an append), so removing a key
// from the list removes it from the file. Render is idempotent and is
// the anti-brick boot guarantee: it runs at startup and after every
// change.
func (r *Renderer) Render(a Access) error {
	home := r.rootHome
	if home == "" {
		home = "/root"
	}
	switch a.Provider {
	case ProviderOff:
		// off does NOT silently leave a previously-rendered openaps
		// authorized_keys in place — that would keep granting shell access
		// after the operator asked recoveryd to stop. Truncate the root
		// authorized_keys to empty so flipping to off actually revokes the
		// keys recoveryd had rendered. (The host provider writes elsewhere;
		// see writeAuthorizedKeys.)
		return writeAuthorizedKeys(filepath.Join(home, ".ssh"), nil, -1, -1)
	case ProviderOpenAPS:
		sshDir := filepath.Join(home, ".ssh")
		// Fail-to-empty guard: rendering ZERO keys would truncate
		// /root/.ssh/authorized_keys. If a non-empty file already exists
		// (operator still has keys, but access.json was blanked/lost), log
		// loudly and skip the truncating rewrite rather than locking the
		// operator out. A deliberate "remove all keys" goes through off.
		if len(a.SSHKeys) == 0 && fileNonEmpty(filepath.Join(sshDir, "authorized_keys")) {
			log.Printf("recoveryd: REFUSING to render an empty authorized_keys over a non-empty one (provider=openaps, zero keys in access.json); existing keys preserved. Use provider=off to deliberately revoke all keys.")
			return r.ensureDropbearHostKey()
		}
		if err := writeAuthorizedKeys(sshDir, a.SSHKeys, -1, -1); err != nil {
			return err
		}
		return r.ensureDropbearHostKey()
	case ProviderHost:
		if a.HostUser == "" {
			return errors.New("recoveryd: host provider requires host_user")
		}
		lu := r.lookupUser
		if lu == nil {
			lu = user.Lookup
		}
		u, err := lu(a.HostUser)
		if err != nil {
			return fmt.Errorf("recoveryd: lookup host_user %q: %w", a.HostUser, err)
		}
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)
		// host provider does NOT manage a host key — the host sshd owns it.
		return writeAuthorizedKeys(filepath.Join(u.HomeDir, ".ssh"), a.SSHKeys, uid, gid)
	default:
		return fmt.Errorf("recoveryd: unknown provider %q", a.Provider)
	}
}

// writeAuthorizedKeys atomically rewrites <sshDir>/authorized_keys from
// keys. sshDir is created 0700 and the file is 0600. When uid/gid are
// >= 0 the directory and file are chowned to them (host provider).
//
// The temp file is fsync'd before the rename and the containing directory
// is fsync'd after, so a power loss can never publish a zero-length or
// partial authorized_keys — exactly the anti-brick file this daemon
// exists to protect.
func writeAuthorizedKeys(sshDir string, keys []Key, uid, gid int) error {
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("recoveryd: mkdir %s: %w", sshDir, err)
	}
	if uid >= 0 && gid >= 0 {
		// Under the host provider the .ssh dir must be owned by the host
		// user or some sshd StrictModes configs reject it. A half-chowned
		// .ssh (root dir, user file) is a hard error, not best-effort.
		if err := os.Chown(sshDir, uid, gid); err != nil {
			return fmt.Errorf("recoveryd: chown %s: %w", sshDir, err)
		}
	}
	var buf []byte
	for _, k := range keys {
		buf = append(buf, renderLine(k)...)
		buf = append(buf, '\n')
	}
	path := filepath.Join(sshDir, "authorized_keys")
	// host provider chowns the temp file to the host user before the
	// rename, so it can't use atomicfile.Write's anonymous temp path.
	if uid >= 0 && gid >= 0 {
		return writeChownedAuthorizedKeys(path, buf, uid, gid)
	}
	// openaps/off: full-rewrite with fsync+rename+dir-fsync so a crash
	// never publishes a zero-length/partial authorized_keys.
	return atomicfile.Write(path, buf, 0o600)
}

// writeChownedAuthorizedKeys writes the authorized_keys for the host
// provider: temp file fsync'd, chowned to the host user, then renamed.
func writeChownedAuthorizedKeys(path string, buf []byte, uid, gid int) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("recoveryd: open %s: %w", tmp, err)
	}
	if _, err := f.Write(buf); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("recoveryd: write %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("recoveryd: fsync %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("recoveryd: close %s: %w", tmp, err)
	}
	if err := os.Chown(tmp, uid, gid); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("recoveryd: chown %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("recoveryd: rename %s: %w", path, err)
	}
	if d, derr := os.Open(filepath.Dir(path)); derr == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// fileNonEmpty reports whether path exists and has a non-zero size.
func fileNonEmpty(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

// ensureDropbearHostKey generates the dropbear RSA host key if missing.
// Idempotent: a present key is left untouched.
func (r *Renderer) ensureDropbearHostKey() error {
	path := r.DropbearKeyPath
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
	gen := r.dropbearKeyGen
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

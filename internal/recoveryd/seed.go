package recoveryd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Seed ingests an authorized_keys-format file (one key per line, blank
// lines and # comments ignored), validates and fingerprints each key,
// dedupes by fingerprint, and writes them into the store under the given
// provider. Existing keys in the store are preserved (merge, not
// replace) so re-running the installer is idempotent. It is used by the
// brownfield installer to seed access.json from the operator's bundled
// key without writing authorized_keys directly — recoveryd renders it at
// boot.
func Seed(store *Store, provider Provider, hostUser, authorizedKeysPath string) (added int, err error) {
	f, err := os.Open(authorizedKeysPath)
	if err != nil {
		return 0, fmt.Errorf("recoveryd: open seed file %s: %w", authorizedKeysPath, err)
	}
	defer f.Close()

	a := store.Get()
	if provider != "" {
		a.Provider = provider
	}
	if hostUser != "" {
		a.HostUser = hostUser
	}
	seen := make(map[string]bool, len(a.SSHKeys))
	for _, k := range a.SSHKeys {
		seen[k.Fingerprint] = true
	}

	now := time.Now().UnixMilli()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, perr := ParseKey(line, "")
		if perr != nil {
			return added, fmt.Errorf("recoveryd: seed line %q: %w", line, perr)
		}
		if seen[k.Fingerprint] {
			continue
		}
		k.AddedMs = now
		a.SSHKeys = append(a.SSHKeys, k)
		seen[k.Fingerprint] = true
		added++
	}
	if err := sc.Err(); err != nil {
		return added, fmt.Errorf("recoveryd: scan seed file: %w", err)
	}
	if err := store.Save(a); err != nil {
		return added, err
	}
	return added, nil
}

// Package udsutil holds the small shared helpers for the local
// Unix-domain-socket servers in this repo: the SO_PEERCRED peer-uid
// resolver used by the root-only gates and the stale-socket cleanup that
// refuses to clobber non-socket paths.
package udsutil

import (
	"errors"
	"fmt"
	"os"
)

// RemoveStaleSocket unlinks an existing UDS at path if (and only if) it is
// in fact a socket. It refuses to clobber regular files, dirs, or symlinks
// so a misconfigured -socket arg can't delete unrelated data. Uses Lstat
// so a symlinked-to-socket is also refused. A missing path is not an error.
func RemoveStaleSocket(path string) error {
	st, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if st.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove non-socket at %s (mode=%s)", path, st.Mode())
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale socket %s: %w", path, err)
	}
	return nil
}

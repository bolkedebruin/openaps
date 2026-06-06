// Package atomicfile writes a file durably and atomically: write to a
// temp file, fsync it, rename over the target, then fsync the containing
// directory. A crash can never leave a zero-length or partial target —
// the reader sees either the old file or the fully-written new one. It is
// shared by the config/key stores whose files must survive power loss on
// the ECU's flash filesystem.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write atomically replaces path with data at the given mode. The temp
// file is created alongside path (same directory) so the rename is on one
// filesystem. The directory holding path is created 0700 if missing.
func Write(path string, data []byte, mode os.FileMode) error {
	return WriteOwned(path, data, mode, -1, -1)
}

// WriteOwned is Write with an optional chown of the temp file before the
// rename, so the published file is owned by uid/gid (e.g. a host sshd's
// user under StrictModes). A uid or gid below zero skips the chown, making
// WriteOwned(..., -1, -1) identical to Write. The chown is applied to the
// temp before the rename so the target is never briefly mis-owned.
func WriteOwned(path string, data []byte, mode os.FileMode, uid, gid int) error {
	dir := filepath.Dir(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("atomicfile: mkdir %s: %w", dir, err)
		}
	}
	tmp := path + ".tmp"
	if err := writeSync(tmp, data, mode); err != nil {
		return err
	}
	if uid >= 0 && gid >= 0 {
		if err := os.Chown(tmp, uid, gid); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("atomicfile: chown %s: %w", tmp, err)
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("atomicfile: rename %s: %w", path, err)
	}
	syncDir(dir)
	return nil
}

// writeSync writes data to path with mode and fsyncs before close. The mode
// is applied with an explicit Chmod because open(2) only honours the mode
// arg when it creates the file — a stale temp left by a prior crash would
// otherwise keep its old (possibly looser) permissions and the rename would
// publish those instead of the requested mode.
func writeSync(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("atomicfile: open %s: %w", path, err)
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("atomicfile: chmod %s: %w", path, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("atomicfile: write %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("atomicfile: fsync %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("atomicfile: close %s: %w", path, err)
	}
	return nil
}

// syncDir fsyncs a directory so a rename's metadata is durable. Failure to
// open or sync the dir is non-fatal — the file fsync is the load-bearing
// one and some filesystems reject fsync on a directory.
func syncDir(dir string) {
	if dir == "" {
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

// Package testhelpers contains small utilities shared across test
// files in this module.
package testhelpers

import (
	"os"
	"testing"
)

// ShortTempDir returns a /tmp-rooted directory short enough to use
// as a Unix-domain socket path on Darwin (sun_path is capped at 104
// bytes, and the default TempDir on macOS is too deep).
func ShortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ecuss-")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

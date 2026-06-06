//go:build !linux

package udsutil

import "net"

// PeerUID is a no-op on non-Linux platforms (developer macOS). It returns
// -1; callers treat -1 as "unknown" and skip the uid-0 gate so tests can
// drive the local UDS. SO_PEERCRED gating is enforced on the Linux target.
func PeerUID(_ net.Conn) int {
	return -1
}

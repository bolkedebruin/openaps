//go:build !linux

package ipc

import "net"

// peerUIDFromConn is a no-op on non-Linux platforms; the ingest UID
// gate skips its check when the resolved UID is -1.
func peerUIDFromConn(_ net.Conn) int {
	return -1
}

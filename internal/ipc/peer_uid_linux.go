//go:build linux

package ipc

import (
	"net"
	"syscall"
)

// peerUIDFromConn returns the OS user-id of the peer of a Unix-domain
// socket via SO_PEERCRED. Returns -1 on any error (caller treats that as
// "unknown" — the ingest gate skips its UID check when the value is -1
// and ControllerUIDs is empty).
func peerUIDFromConn(c net.Conn) int {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return -1
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return -1
	}
	uid := -1
	var inner error
	err = raw.Control(func(fd uintptr) {
		ucred, e := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		if e != nil {
			inner = e
			return
		}
		uid = int(ucred.Uid)
	})
	if err != nil || inner != nil {
		return -1
	}
	return uid
}

//go:build linux

package udsutil

import (
	"net"
	"syscall"
)

// PeerUID returns the OS user-id of the UDS peer via SO_PEERCRED, or -1
// on any error. Callers that gate on uid-0 treat -1 as "unknown" — the
// peer-cred check is only enforced on the Linux target where the daemons
// actually run.
func PeerUID(c net.Conn) int {
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

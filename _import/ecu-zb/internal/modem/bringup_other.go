//go:build !linux

package modem

import "errors"

// errUnsupported mirrors the uart package: ecu-zb only runs on the ECU
// (linux) at runtime. The pure-Go frame/config helpers still build and
// test on other platforms; only the fd I/O is stubbed.
var errUnsupported = errors.New("modem bring-up only supported on linux")

// BringupAPsystems is a no-op stub on non-linux builds.
func BringupAPsystems(_ int, _ uint16, _ byte) error { return errUnsupported }

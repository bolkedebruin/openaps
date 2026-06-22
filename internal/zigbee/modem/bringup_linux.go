//go:build linux

package modem

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sys/unix"
)

// resetDevice is the GPIO reset line for the radio; toggled via
// ioctl(fd, 0, 0).
const resetDevice = "/dev/reset"

// ackTimeout bounds how long we wait for a config-op reply.
const ackTimeout = 5 * time.Second

// errNoAck is returned when a config op gets no AB 26 52 reply in time.
var errNoAck = errors.New("no modem ack")

// BringupAPsystems performs the cold-start configuration of the ECU's
// built-in module on fd (an open /dev/ttyO2): a 0x0D liveness ping,
// hardware-resetting the radio on failure (up to 3 attempts), then a
// single 0x05 Set-PANID+Channel. Runs once at startup; the module
// retains its PAN/channel afterwards.
//
// fd must have no concurrent reader: call this before the splice starts.
// A failure to configure is returned but is not necessarily fatal — an
// already-configured module keeps working — so the caller may log and
// proceed.
func BringupAPsystems(fd int, pan uint16, channel byte) error {
	alive := false
	for attempt := 1; attempt <= 3; attempt++ {
		if err := exchange(fd, buildPing(), "ping"); err == nil {
			alive = true
			break
		}
		slog.Warn("modem 0x0D ping failed, hardware-resetting radio", "attempt", attempt, "max", 3)
		if err := HardwareReset(); err != nil {
			slog.Error("modem hardware reset failed", "err", err)
		}
	}
	if !alive {
		slog.Warn("modem radio unresponsive after 3 resets, sending config regardless")
	}

	if err := exchange(fd, buildSetPanidChannel(pan, channel), "set-panid-channel"); err != nil {
		return fmt.Errorf("set PANID/channel (pan=0x%04X ch=%d): %w", pan, channel, err)
	}
	slog.Info("modem configured as coordinator", "pan", fmt.Sprintf("0x%04X", pan), "channel", channel)
	return nil
}

// exchange flushes both directions, writes a config frame, and waits for
// the module's ack (a reply beginning with 0xAB). Mirrors clear_zbmodem +
// write2 + zb_get_reply_from_module. The full ack bytes are logged so the
// radio variant (CC2530 AB CD EF vs CC2652 AB 26 52) is visible.
func exchange(fd int, frame []byte, what string) error {
	if err := flush(fd); err != nil {
		return fmt.Errorf("flush before %s: %w", what, err)
	}
	// clear_zbmodem settles ~1s after the flush before the module is
	// addressed; match that so the radio is ready to reply. Shared with
	// the pairing runner via the package-level settle constant so the
	// single "post-TCIOFLUSH quiet time" value lives in exactly one place.
	time.Sleep(settle)
	if _, err := unix.Write(fd, frame); err != nil {
		return fmt.Errorf("write %s: %w", what, err)
	}
	ack, err := awaitAck(fd, ackTimeout)
	if err != nil {
		if len(ack) > 0 {
			slog.Warn("modem reply has no 0xAB ack marker", "op", what, "got", fmt.Sprintf("% X", ack))
		}
		return fmt.Errorf("%s: %w", what, err)
	}
	slog.Debug("modem ack", "op", what, "ack", fmt.Sprintf("% X", ack))
	return nil
}

// awaitAck accumulates bytes until a complete config-op ack (a 0xAB byte
// followed by two more) arrives or the deadline passes. Scanning for the
// 0xAB marker rather than demanding an exact read tolerates byte-at-a-
// time UART delivery and ignores any stray bytes already on the wire, so
// a healthy module is not misread as unresponsive. Returns the 3-byte ack
// on success.
func awaitAck(fd int, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	var acc []byte
	tmp := make([]byte, 64)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return acc, errNoAck
		}
		ready, err := waitReadable(fd, remaining)
		if err != nil {
			return acc, err
		}
		if !ready {
			return acc, errNoAck
		}
		n, err := unix.Read(fd, tmp)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return acc, fmt.Errorf("read: %w", err)
		}
		if n <= 0 {
			continue
		}
		acc = append(acc, tmp[:n]...)
		if i := findAck(acc); i >= 0 {
			return acc[i : i+ackLen], nil
		}
		// Bound the scan window so a chatty bus can't grow acc without
		// limit while we wait; keep enough tail to span a split ack.
		if len(acc) > 256 {
			acc = acc[len(acc)-ackLen:]
		}
	}
}

// waitReadable blocks until fd is readable or timeout elapses.
func waitReadable(fd int, timeout time.Duration) (bool, error) {
	var rfds unix.FdSet
	rfds.Zero()
	rfds.Set(fd)
	tv := unix.NsecToTimeval(timeout.Nanoseconds())
	n, err := unix.Select(fd+1, &rfds, nil, nil, &tv)
	if err != nil {
		if err == unix.EINTR {
			return false, nil
		}
		return false, fmt.Errorf("select: %w", err)
	}
	return n > 0, nil
}

// flush discards both the input and output queues (tcflush TCIOFLUSH).
func flush(fd int) error {
	return unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIOFLUSH)
}

// HardwareReset pulses the radio's reset GPIO via /dev/reset
// (ioctl(fd,0,0)) and waits for it to settle.
func HardwareReset() error {
	fd, err := unix.Open(resetDevice, unix.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", resetDevice, err)
	}
	rerr := unix.IoctlSetInt(fd, 0, 0)
	unix.Close(fd)
	if rerr != nil {
		return fmt.Errorf("ioctl %s: %w", resetDevice, rerr)
	}
	slog.Info("modem radio reset asserted, waiting 10s to settle", "device", resetDevice)
	time.Sleep(10 * time.Second)
	return nil
}

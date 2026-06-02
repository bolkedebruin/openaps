//go:build linux

package uart

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// PTY holds a pseudo-terminal master and the resolved slave path. The
// slave is what the host process opens (after we symlink /dev/ttyO2 at it).
type PTY struct {
	Master    *os.File
	SlavePath string
}

// OpenPTY opens /dev/ptmx, unlocks the slave, configures raw 57600 8N1
// on the line discipline (so the host process reads in the same mode
// the real UART runs in), and returns the master fd plus the resolved
// /dev/pts/N path.
//
// The master is opened O_NONBLOCK so writes to a slave-less or
// slow-draining pty return EAGAIN immediately instead of blocking
// the modem→host copy goroutine. The proxy layer drops on EAGAIN;
// reads return EAGAIN when no data is available and the read loop
// handles that as a brief sleep.
func OpenPTY() (*PTY, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/ptmx: %w", err)
	}

	zero := 0
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, zero); err != nil {
		master.Close()
		return nil, fmt.Errorf("TIOCSPTLCK: %w", err)
	}

	idx, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		master.Close()
		return nil, fmt.Errorf("TIOCGPTN: %w", err)
	}
	slavePath := fmt.Sprintf("/dev/pts/%d", idx)

	if err := configureRaw57600(int(master.Fd())); err != nil {
		master.Close()
		return nil, fmt.Errorf("set master termios: %w", err)
	}

	return &PTY{Master: master, SlavePath: slavePath}, nil
}

func (p *PTY) Close() error {
	if p == nil || p.Master == nil {
		return nil
	}
	return p.Master.Close()
}

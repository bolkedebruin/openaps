//go:build linux

package uart

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// OpenSerial opens a UART character device at path, configures it for raw
// 57600 8N1 (matching the termios zb-tap.sh's socat uses), and returns
// the open file. The caller is responsible for closing it.
func OpenSerial(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := configureRaw57600(int(f.Fd())); err != nil {
		f.Close()
		return nil, fmt.Errorf("configure %s: %w", path, err)
	}
	return f, nil
}

// ConfigureRaw57600 applies raw 57600 8N1 termios to an already-open fd.
// Exposed so the pty master can be put into the same mode as the real
// UART without re-opening anything.
func ConfigureRaw57600(fd int) error {
	return configureRaw57600(fd)
}

func configureRaw57600(fd int) error {
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return fmt.Errorf("TCGETS: %w", err)
	}

	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON | unix.IXOFF | unix.IXANY
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	t.Cflag &^= unix.CSIZE | unix.PARENB | unix.CSTOPB | unix.CRTSCTS
	t.Cflag |= unix.CS8 | unix.CREAD | unix.CLOCAL

	t.Cflag &^= unix.CBAUD
	t.Cflag |= unix.B57600
	t.Ispeed = unix.B57600
	t.Ospeed = unix.B57600

	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, t); err != nil {
		return fmt.Errorf("TCSETS: %w", err)
	}
	return nil
}

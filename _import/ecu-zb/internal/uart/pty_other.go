//go:build !linux

package uart

import "os"

type PTY struct {
	Master    *os.File
	SlavePath string
}

func OpenPTY() (*PTY, error) { return nil, errUnsupported }

func (p *PTY) Close() error {
	if p == nil || p.Master == nil {
		return nil
	}
	return p.Master.Close()
}

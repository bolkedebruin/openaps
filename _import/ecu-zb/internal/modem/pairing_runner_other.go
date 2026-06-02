//go:build !linux

package modem

import (
	"sync"
	"time"
)

// PairingRunner is stubbed on non-linux builds; the byte-exact frame
// builders and CRC/BCD helpers still build and test everywhere. Only the
// modem fd I/O is linux-only.
type PairingRunner struct {
	Fd int
	In <-chan []byte
	Mu *sync.Mutex
}

// FoundUnit mirrors the linux definition so callers compile on all platforms.
type FoundUnit struct {
	Serial    string
	Encrypted bool
}

func (r *PairingRunner) SetModulePan(uint16, byte) error      { return errUnsupported }
func (r *PairingRunner) GetShortAddr(string) (uint16, error)  { return 0, errUnsupported }
func (r *PairingRunner) SetInvPan(string, uint16, byte) error { return errUnsupported }
func (r *PairingRunner) PrimeInv(string, uint16, byte) error  { return errUnsupported }
func (r *PairingRunner) CommitPan(uint16, byte) error         { return errUnsupported }
func (r *PairingRunner) BindQuiet(uint16) error               { return errUnsupported }
func (r *PairingRunner) ReportScan(time.Duration, byte) ([]FoundUnit, error) {
	return nil, errUnsupported
}

package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bolkedebruin/openaps/internal/zigbee/modem"
	"github.com/bolkedebruin/openaps/internal/zigbee/proxy"
)

// pairingAdapter implements busmgr.PairingExecutor. It serialises every OTA
// pairing primitive under a single mutex and pauses the splice interception
// for the duration of each one, so the modem fd is held exclusively (the
// copy goroutines neither read the modem nor forward host traffic while
// a request/reply exchange is in flight). The pause is applied to ALL
// primitives, not just the ones that park off the operating PAN: every
// primitive does an exclusive read of the modem reply, which would otherwise
// race the modem→host copy goroutine for the bytes.
type pairingAdapter struct {
	splice *proxy.Splice
	runner *modem.PairingRunner

	mu  sync.Mutex
	seq uint32 // monotonic running-sequence byte source for D1/D2
	// opPAN / opChannel are the operating PAN+channel the radio is bonded to
	// (set at bring-up, updated on a real SetModulePan). ecu-zb owns this so
	// the driver can restore (pan=0) or query (GetOperatingPAN) it without
	// re-deriving the PAN from settings. Guarded by mu.
	opPAN     uint16
	opChannel byte
}

func newPairingAdapter(splice *proxy.Splice, fd int, opPAN uint16, opChannel byte) *pairingAdapter {
	return &pairingAdapter{
		splice: splice,
		// Share the splice's modem-write mutex with the runner so the
		// runner's flush+settle+drain+write sequence is serialised
		// against every other modem writer (splice DirToModem copy,
		// hook InjectToModem, busmgr inject). This is the single
		// modem-fd write lock — any new modem writer must take it.
		runner:    &modem.PairingRunner{Fd: fd, Mu: splice.ModemWriteMu()},
		opPAN:     opPAN,
		opChannel: opChannel,
	}
}

// GetOperatingPAN returns the PAN the radio is bonded to. No modem I/O.
func (a *pairingAdapter) GetOperatingPAN() uint16 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.opPAN
}

// GetOperatingChannel returns the RF channel the radio is on. No modem I/O.
func (a *pairingAdapter) GetOperatingChannel() byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.opChannel
}

// withModem runs fn while holding the pairing mutex and with the splice in
// pairing mode: the splice's single modem reader redirects modem replies to a
// sink the runner reads, and host→modem is paused. This avoids a second
// reader on the modem fd (the dual-reader race that consumed config-op acks).
func (a *pairingAdapter) withModem(fn func() error) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runner.In = a.splice.BeginPairing()
	defer func() {
		a.splice.EndPairing()
		a.runner.In = nil
	}()
	return fn()
}

func (a *pairingAdapter) nextSeq() byte {
	a.seq++
	return byte(a.seq)
}

// SetModulePan interprets the special PAN values (see busmgr.proto):
//
//	0xFFFF → temporary rendezvous park (operating PAN unchanged)
//	0      → restore to the operating PAN+channel (caller's channel ignored)
//	other  → set and adopt as the new operating PAN+channel
//
// withModem already holds the mutex, so the opPAN/opChannel read+update is
// serialised with every other primitive.
func (a *pairingAdapter) SetModulePan(pan uint16, channel byte) error {
	return a.withModem(func() error {
		switch pan {
		case 0xFFFF:
			return a.runner.SetModulePan(0xFFFF, channel)
		case 0:
			return a.runner.SetModulePan(a.opPAN, a.opChannel)
		default:
			if err := a.runner.SetModulePan(pan, channel); err != nil {
				return err
			}
			a.opPAN, a.opChannel = pan, channel
			return nil
		}
	})
}

func (a *pairingAdapter) GetShortAddr(serial string) (uint16, error) {
	var sa uint16
	err := a.withModem(func() error {
		var e error
		sa, e = a.runner.GetShortAddr(serial)
		return e
	})
	return sa, err
}

func (a *pairingAdapter) SetInvPan(serial string, pan uint16, channel byte) error {
	return a.withModem(func() error { return a.runner.SetInvPan(serial, pan, channel) })
}

func (a *pairingAdapter) PrimeInv(serial string, pan uint16, channel byte) error {
	return a.withModem(func() error { return a.runner.PrimeInv(serial, pan, channel) })
}

func (a *pairingAdapter) CommitPan(pan uint16, channel byte) error {
	return a.withModem(func() error { return a.runner.CommitPan(pan, channel) })
}

func (a *pairingAdapter) BindQuiet(shortAddr uint16) error {
	return a.withModem(func() error { return a.runner.BindQuiet(shortAddr) })
}

// Probe pings the module (0x0D) under the pairing lock with the splice
// redirected, so it neither races the modem→host copy nor overlaps a pairing
// op. alive reports the 0xAB ack; a dead module is alive=false with nil err.
func (a *pairingAdapter) Probe(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	var alive bool
	err := a.withModem(func() error {
		var e error
		alive, e = a.runner.Ping()
		return e
	})
	return alive, err
}

// Recover re-arms a wedged radio: a hardware reset pulse followed by re-adopting
// the operating PAN+channel. Without a known operating PAN there is nothing to
// re-arm to, so it reports an error rather than resetting blindly. The
// SetModulePan re-adopt is idempotent on a healthy radio.
func (a *pairingAdapter) Recover(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.withModem(func() error {
		if a.opPAN == 0 {
			return fmt.Errorf("recover: no operating PAN to re-arm")
		}
		if err := modem.HardwareReset(); err != nil {
			return err
		}
		return a.runner.SetModulePan(a.opPAN, a.opChannel)
	})
}

func (a *pairingAdapter) ReportScan(window time.Duration) ([]modem.FoundUnit, error) {
	var found []modem.FoundUnit
	err := a.withModem(func() error {
		var e error
		found, e = a.runner.ReportScan(window, a.nextSeq())
		return e
	})
	return found, err
}

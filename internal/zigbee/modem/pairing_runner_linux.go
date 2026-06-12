//go:build linux

package modem

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// PairingRunner executes OTA pairing primitives. It WRITES frames to the modem
// fd directly, but READS replies from In — the byte stream the splice's single
// modem reader redirects to it during pairing (Splice.BeginPairing). This
// keeps exactly one reader on the modem fd: a second concurrent reader caused
// config-op acks to be consumed by the splice copy goroutine's in-flight read.
//
// PAN/channel come from the inv-driver pairing state machine via the
// PairingCmd; the runner holds no policy. Every primitive flushes the modem
// (TCIOFLUSH) and drains In before sending so a reply it reads next is the
// response to that frame, not a straggler.
type PairingRunner struct {
	// Fd is the open /dev/ttyO2 modem descriptor, used for WRITES only.
	Fd int
	// In is the modem→reply byte stream fed by the splice's single modem
	// reader while pairing is active (see Splice.BeginPairing). The runner
	// reads replies from here rather than the fd, so there is never a second
	// concurrent reader on the modem fd. Set per-primitive by the adapter.
	In <-chan []byte
	// Mu is the single modem-fd write mutex. The runner takes it around
	// the flush+settle+drain+write sequence so no other writer (splice
	// DirToModem copy, hook InjectToModem, busmgr inject) can flush the
	// modem's input buffer or interleave a frame mid-sequence. Wired by
	// the pairing adapter to Splice.ModemWriteMu() so it IS the splice's
	// mutex (one shared lock across every modem writer). If nil the
	// runner falls back to an internal mutex — tests get serialisation
	// without forcing a splice.
	Mu *sync.Mutex

	// fallbackMu backs Mu when the caller didn't wire a shared mutex.
	fallbackMu sync.Mutex
	// fallbackWarn fires a one-time WARNING log if the fallback mutex is ever
	// used (production must always wire Mu — the fallback is a test-only safety
	// net that would otherwise silently mask the dual-writer race).
	fallbackWarn sync.Once
}

// settle is the quiet time after a TCIOFLUSH before the next frame is
// written. The 1s pause gives the modem time to drop any in-flight bytes
// and accept a fresh config-op cleanly. A shorter settle (the previous
// 200ms) was correlated with intermittent "no modem ack" responses on
// rekey rearm/rollback.
const settle = 1 * time.Second

// modemWriteMu returns the shared write mutex if one was wired, else the
// per-runner fallback. The fallback path is intended for tests only; in
// production newPairingAdapter wires the splice's modem write mutex so all
// modem writers (splice DirToModem copy, hook InjectToModem, busmgr inject,
// pairing runner) share one lock. To make a future wiring mistake loud rather
// than silently masking the race that this lock prevents, the first fallback
// use logs a warning.
func (r *PairingRunner) modemWriteMu() *sync.Mutex {
	if r.Mu != nil {
		return r.Mu
	}
	r.fallbackWarn.Do(func() {
		log.Printf("pairing: WARNING PairingRunner.Mu is nil; using internal fallback mutex — production must wire splice.ModemWriteMu()")
	})
	return &r.fallbackMu
}

// writeFrame flushes the kernel queues, settles, drains any sink stragglers,
// then writes one frame — all under the modem-fd write mutex so no other
// writer can flush our buffer mid-flight or interleave a byte between the
// drain and the write. Draining after the settle means a reply we read next
// is the response to THIS frame, not a pre-write straggler.
func (r *PairingRunner) writeFrame(frame []byte, what string) error {
	mu := r.modemWriteMu()
	mu.Lock()
	defer mu.Unlock()
	if err := flush(r.Fd); err != nil {
		return fmt.Errorf("flush before %s: %w", what, err)
	}
	time.Sleep(settle)
	drainChan(r.In)
	if _, err := unix.Write(r.Fd, frame); err != nil {
		return fmt.Errorf("write %s: %w", what, err)
	}
	return nil
}

// drainChan discards any buffered chunks (pre-write stragglers) without
// blocking.
func drainChan(in <-chan []byte) {
	for {
		select {
		case <-in:
		default:
			return
		}
	}
}

// awaitAckChan scans the sink stream for a config-op ack (first byte 0xAB)
// within timeout. Mirrors the fd-based awaitAck but reads from the splice sink
// so the modem fd keeps a single reader.
func awaitAckChan(in <-chan []byte, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	var acc []byte
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return acc, errNoAck
		}
		timer := time.NewTimer(remaining)
		select {
		case chunk, ok := <-in:
			timer.Stop()
			if !ok {
				return acc, errNoAck
			}
			acc = append(acc, chunk...)
			if i := findAck(acc); i >= 0 {
				return acc[i : i+ackLen], nil
			}
			if len(acc) > 256 {
				acc = acc[len(acc)-ackLen:]
			}
		case <-timer.C:
			return acc, errNoAck
		}
	}
}

// readFrameChan accumulates bytes from the sink until a short inter-chunk idle
// delimits a frame or the deadline passes. encrypted reports the AES marker
// (CC EE / FC FC) on the accumulated frame. Replaces the fd-based readRawFrame
// for the pairing path.
func readFrameChan(in <-chan []byte, timeout time.Duration) (frame []byte, encrypted bool, err error) {
	deadline := time.Now().Add(timeout)
	var acc []byte
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if len(acc) > 0 {
				return acc, isEncryptedFrame(acc), nil
			}
			return nil, false, errNoAck
		}
		waitFor := remaining
		if len(acc) > 0 && waitFor > 150*time.Millisecond {
			waitFor = 150 * time.Millisecond
		}
		timer := time.NewTimer(waitFor)
		select {
		case chunk, ok := <-in:
			timer.Stop()
			if !ok {
				if len(acc) > 0 {
					return acc, isEncryptedFrame(acc), nil
				}
				return nil, false, errNoAck
			}
			acc = append(acc, chunk...)
		case <-timer.C:
			if len(acc) > 0 {
				return acc, isEncryptedFrame(acc), nil
			}
		}
	}
}

// Ping sends a 0x0D liveness ping and reports whether the module acked. A
// returned ack (first byte 0xAB) means alive; a no-ack within ackTimeout
// means the module is wedged and is reported as alive=false WITHOUT an error
// — a dead module is the signal, not a failure. err is only a real
// transport/write error.
func (r *PairingRunner) Ping() (alive bool, err error) {
	if err := r.writeFrame(buildPing(), "ping"); err != nil {
		return false, err
	}
	ack, err := awaitAckChan(r.In, ackTimeout)
	if err != nil {
		if errors.Is(err, errNoAck) {
			return false, nil
		}
		return false, fmt.Errorf("ping: %w", err)
	}
	log.Printf("pairing: ping ack % X", ack)
	return true, nil
}

// SetModulePan parks the local module on pan/channel (op 0x05). pan may be
// 0xFFFF for the rendezvous. Confirms the module ack (first byte 0xAB).
func (r *PairingRunner) SetModulePan(pan uint16, channel byte) error {
	if err := r.writeFrame(buildSetPanidChannel(pan, channel), "set-module-pan"); err != nil {
		return err
	}
	ack, err := awaitAckChan(r.In, ackTimeout)
	if err != nil {
		return fmt.Errorf("set-module-pan (pan=0x%04X ch=%d): %w", pan, channel, err)
	}
	log.Printf("pairing: set-module-pan pan=0x%04X ch=%d ack % X", pan, channel, ack)
	return nil
}

// GetShortAddr sends a directed 0x0E and parses the assigned short address
// from the reply. Returns the SA, or an error if the serial is malformed or
// no valid reply arrives.
func (r *PairingRunner) GetShortAddr(serial string) (uint16, error) {
	ieee, err := serialToBCD6(serial)
	if err != nil {
		return 0, err
	}
	if err := r.writeFrame(buildGetInverterShortAddr(ieee), "get-short-addr"); err != nil {
		return 0, err
	}
	reply, _, err := readFrameChan(r.In, ackTimeout)
	if err != nil {
		return 0, fmt.Errorf("get-short-addr %s: %w", serial, err)
	}
	sa, ok := parseShortAddrReply(reply, ieee)
	if !ok {
		return 0, fmt.Errorf("get-short-addr %s: no matching short-addr in reply % X", serial, reply)
	}
	return sa, nil
}

// SetInvPan sends a directed 0x0F to force one inverter onto pan/channel.
func (r *PairingRunner) SetInvPan(serial string, pan uint16, channel byte) error {
	ieee, err := serialToBCD6(serial)
	if err != nil {
		return err
	}
	if err := r.writeFrame(buildSetInverterPan(pan, channel, ieee), "set-inv-pan"); err != nil {
		return err
	}
	// 0x0F is directed-with-no-config-ack (write2 + sleep); wait ~1s
	// for the OTA hop to land.
	time.Sleep(1 * time.Second)
	return nil
}

// PrimeInv sends a directed 0x11 to prime one inverter with the new
// pan/channel (it does not switch until the 0x22 commit).
func (r *PairingRunner) PrimeInv(serial string, pan uint16, channel byte) error {
	ieee, err := serialToBCD6(serial)
	if err != nil {
		return err
	}
	if err := r.writeFrame(buildPrimeInverterPan(pan, channel, ieee), "prime-inv"); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	return nil
}

// CommitPan broadcasts the 0x22 commit three times so primed inverters
// jump to the new PAN, then waits for the migration to settle.
// BROADCAST: every primed inverter is affected.
func (r *PairingRunner) CommitPan(pan uint16, channel byte) error {
	frame := buildCommitPanNow(pan, channel)
	for i := 0; i < 3; i++ {
		if err := r.writeFrame(frame, "commit-pan"); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

// BindQuiet sends a directed 0x08 to bind a short address and turn its
// report-id off. Confirms the A5 A5 success reply.
func (r *PairingRunner) BindQuiet(shortAddr uint16) error {
	if err := r.writeFrame(buildBindZigbee(shortAddr), "bind-quiet"); err != nil {
		return err
	}
	reply, _, err := readFrameChan(r.In, ackTimeout)
	if err != nil {
		return fmt.Errorf("bind-quiet sa=0x%04X: %w", shortAddr, err)
	}
	if len(reply) < 4 || reply[2] != bindAckByte || reply[3] != bindAckByte {
		return fmt.Errorf("bind-quiet sa=0x%04X: no A5 A5 success in reply % X", shortAddr, reply)
	}
	return nil
}

// FoundUnit is one inverter that announced itself during a report-id scan.
type FoundUnit struct {
	Serial    string
	Encrypted bool
}

// ReportScan turns report-id ON (0xD1), collects 0x1D announcements for the
// window, then turns report-id OFF (0xD2) for the units found. Returns the
// de-duplicated set of announcing inverters. encrypted reflects whether the
// announcement frame arrived AES-wrapped (CC EE / FC FC).
func (r *PairingRunner) ReportScan(window time.Duration, seq byte) ([]FoundUnit, error) {
	// count byte is the remaining-to-find window cap; we are not bounded
	// by a target count here, so use the max (0x3C).
	if err := r.writeFrame(buildReportIdOn(0x3C, seq), "report-id-on"); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var found []FoundUnit
	var ieees [][6]byte

	deadline := time.Now().Add(window)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		frame, encrypted, err := readFrameChan(r.In, remaining)
		if err != nil {
			// Timeout/no-more-announcements ends the window cleanly.
			break
		}
		ieee, ok := parseAnnounce(frame)
		if !ok {
			continue
		}
		serial := bcd6ToSerial(ieee)
		if seen[serial] {
			continue
		}
		seen[serial] = true
		found = append(found, FoundUnit{Serial: serial, Encrypted: encrypted})
		ieees = append(ieees, ieee)
	}

	// Quiet the announcers. Send the 0xD2 list-off then a 0xD3 broadcast
	// quiet to end the discovery sweep.
	if err := r.writeFrame(buildReportIdOff(ieees, seq+1), "report-id-off"); err != nil {
		log.Printf("pairing: report-id-off write failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := r.writeFrame(buildReportIdQuiet(), "report-id-quiet"); err != nil {
		log.Printf("pairing: report-id-quiet write failed: %v", err)
	}
	return found, nil
}

package modem

import (
	"errors"
	"time"
)

// Two different silences, two different sentinels.
//
// errNoAck means the MODULE did not answer. The local radio itself acks
// config ops (0x05 set-PAN, 0x0D ping). A timeout there therefore points at
// the module or the UART.
//
// errNoReply means the module was fine but the INVERTER did not answer. The
// addressed unit answers a directed over-the-air query (0x0E get-short-addr,
// 0x08 bind). A timeout there therefore says the unit is out of reach: on
// another channel, asleep, or gone. A report of that as a modem fault sends
// an operator after a radio problem that does not exist.
//
// A sink that closes mid-read maps to errNoReply. It gets no third sentinel.
// Splice.EndPairing drops the sink instead of closing it, so no production
// path reaches that branch. If one ever does, the same "we got no answer"
// message is the honest report.
var (
	errNoAck   = errors.New("no modem ack")
	errNoReply = errors.New("no reply from inverter")
)

// Reply timing. ackTimeout bounds how long a reader waits for any single
// reply. idleFrameGap delimits one over-the-air frame. Once bytes start to
// arrive, a gap this long without more bytes ends the frame.
const (
	ackTimeout   = 5 * time.Second
	idleFrameGap = 150 * time.Millisecond
)

// maxReplyBytes bounds the frame accumulator. A reply frame is tens of bytes
// (a 10-byte announcement, a ~20-byte short-addr reply), so this ceiling is
// far above any real frame. It exists so that a neighbouring device that
// holds the bus busy cannot decide how much memory we accumulate. Without
// it, only baud x timeout bounds the growth.
const maxReplyBytes = 4096

// ackScanWindow bounds the ack accumulator, so a chatty bus cannot grow it
// without limit while we wait. The retained tail is long enough to span an
// ack split across two reads.
const ackScanWindow = 256

// scanAck appends chunk to acc. Once the 0xAB marker and its two trailing
// bytes arrive, it returns the complete 3-byte ack. Otherwise it returns a
// nil ack and the bounded accumulator, so the caller keeps scanning. Both
// readers share it. The fd loop and the sink loop differ in how they wait
// for bytes, not in how they recognise an ack.
func scanAck(acc, chunk []byte) (next, ack []byte) {
	acc = append(acc, chunk...)
	if i := findAck(acc); i >= 0 {
		return acc, acc[i : i+ackLen]
	}
	if len(acc) > ackScanWindow {
		acc = acc[len(acc)-ackLen:]
	}
	return acc, nil
}

// drainChan discards any buffered chunks (pre-write stragglers) without
// blocking. A closed sink returns at once. A receive from a closed channel is
// always ready, so the test of ok is what stops this loop from spinning. It
// would spin while writeFrame holds the shared modem-write mutex, and that
// would wedge every other modem writer as well.
func drainChan(in <-chan []byte) {
	for {
		select {
		case _, ok := <-in:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

// awaitAckChan scans the sink stream for a config-op ack (first byte 0xAB)
// within timeout. It mirrors the fd-based awaitAck, but it reads from the
// splice sink so that the modem fd keeps a single reader.
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
			var ack []byte
			if acc, ack = scanAck(acc, chunk); ack != nil {
				return ack, nil
			}
		case <-timer.C:
			return acc, errNoAck
		}
	}
}

// readFrameChan accumulates bytes from the sink until a short inter-chunk
// idle delimits a frame or the deadline passes. encrypted reports the AES
// marker (CC EE / FC FC) on the accumulated frame. A deadline that passes
// with nothing accumulated yields errNoReply. For a directed query, that
// means the addressed inverter never answered. For the ReportScan collection
// window, it is the end of the window with no further announcements.
func readFrameChan(in <-chan []byte, timeout time.Duration) (frame []byte, encrypted bool, err error) {
	deadline := time.Now().Add(timeout)
	var acc []byte
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if len(acc) > 0 {
				return acc, isEncryptedFrame(acc), nil
			}
			return nil, false, errNoReply
		}
		waitFor := remaining
		if len(acc) > 0 && waitFor > idleFrameGap {
			waitFor = idleFrameGap
		}
		timer := time.NewTimer(waitFor)
		select {
		case chunk, ok := <-in:
			timer.Stop()
			if !ok {
				if len(acc) > 0 {
					return acc, isEncryptedFrame(acc), nil
				}
				return nil, false, errNoReply
			}
			acc = append(acc, chunk...)
			if len(acc) >= maxReplyBytes {
				return acc, isEncryptedFrame(acc), nil
			}
		case <-timer.C:
			if len(acc) > 0 {
				return acc, isEncryptedFrame(acc), nil
			}
		}
	}
}

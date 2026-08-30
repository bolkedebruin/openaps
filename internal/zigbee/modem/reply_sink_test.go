package modem

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/codec"
)

// feed returns a sink channel that already holds chunks. It mimics the
// splice when it hands the runner bytes it already read from the modem.
func feed(chunks ...[]byte) chan []byte {
	ch := make(chan []byte, len(chunks))
	for _, c := range chunks {
		ch <- c
	}
	return ch
}

func TestDrainChanDiscardsStragglers(t *testing.T) {
	in := feed([]byte{0x01}, []byte{0x02})
	drainChan(in)
	if len(in) != 0 {
		t.Fatalf("drainChan left %d chunk(s) buffered", len(in))
	}
	// Draining an empty channel must not block.
	drainChan(in)
}

func TestAwaitAckChanFindsAckAcrossChunks(t *testing.T) {
	// The ack (0xAB + 2 bytes) spans two chunks, behind noise.
	in := feed([]byte{0x00, 0x11, 0xAB}, []byte{0xCD, 0xEF})
	ack, err := awaitAckChan(in, time.Second)
	if err != nil {
		t.Fatalf("awaitAckChan: %v", err)
	}
	if want := []byte{0xAB, 0xCD, 0xEF}; !bytes.Equal(ack, want) {
		t.Fatalf("ack = % X, want % X", ack, want)
	}
}

func TestAwaitAckChanSilenceIsModemFault(t *testing.T) {
	in := make(chan []byte)
	_, err := awaitAckChan(in, 20*time.Millisecond)
	if !errors.Is(err, errNoAck) {
		t.Fatalf("err = %v, want errNoAck", err)
	}
}

func TestAwaitAckChanClosedSinkIsModemFault(t *testing.T) {
	in := make(chan []byte)
	close(in)
	_, err := awaitAckChan(in, time.Second)
	if !errors.Is(err, errNoAck) {
		t.Fatalf("err = %v, want errNoAck", err)
	}
}

func TestReadFrameChanAssemblesBackToBackChunks(t *testing.T) {
	in := feed([]byte{0x1D, 0x1D}, []byte{0x01, 0x02})
	frame, encrypted, err := readFrameChan(in, time.Second)
	if err != nil {
		t.Fatalf("readFrameChan: %v", err)
	}
	if want := []byte{0x1D, 0x1D, 0x01, 0x02}; !bytes.Equal(frame, want) {
		t.Fatalf("frame = % X, want % X", frame, want)
	}
	if encrypted {
		t.Fatal("bare announcement reported as encrypted")
	}
}

// The idle gap is what ends a frame. A chunk that arrives well after the gap
// belongs to the NEXT frame, and the reader must not append it to this one.
// Both chunks arrive long before the deadline, so a reader that ignored the
// gap would return them concatenated.
func TestReadFrameChanIdleGapEndsTheFrame(t *testing.T) {
	in := make(chan []byte, 2)
	in <- []byte{0x1D, 0x1D}
	// An absolute delay, on purpose not expressed in terms of idleFrameGap.
	// A test whose own timings scale with the constant under test cannot
	// detect a wrong value of that constant.
	go func() {
		time.Sleep(time.Second)
		in <- []byte{0xEE}
	}()

	start := time.Now()
	frame, _, err := readFrameChan(in, 5*time.Second)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("readFrameChan: %v", err)
	}
	if want := []byte{0x1D, 0x1D}; !bytes.Equal(frame, want) {
		t.Fatalf("frame = % X, want % X — the late chunk joined the frame", frame, want)
	}
	// The gap is 150ms and the late chunk lands at 1s. A return well before
	// that proves the read ended on the idle gap, not on the late chunk or
	// the deadline.
	if elapsed > 750*time.Millisecond {
		t.Fatalf("returned after %s, want the idle gap to end the frame", elapsed)
	}
}

// A reply shorter than the idle gap still surfaces when the deadline
// truncates it. The reader does not discard it as silence.
func TestReadFrameChanReturnsPartialOnDeadline(t *testing.T) {
	frame, _, err := readFrameChan(feed([]byte{0xA5}), 30*time.Millisecond)
	if err != nil {
		t.Fatalf("readFrameChan: %v", err)
	}
	if want := []byte{0xA5}; !bytes.Equal(frame, want) {
		t.Fatalf("frame = % X, want % X", frame, want)
	}
}

func TestReadFrameChanReportsEncryptedMarker(t *testing.T) {
	// An AES-wrapped inbound frame: FC FC marker with a gate byte below the
	// cleartext threshold at [12].
	raw := make([]byte, 13)
	raw[0], raw[1] = codec.L1ReplySOF, codec.L1ReplySOF
	raw[12] = codec.CleartextGateMin - 1
	_, encrypted, err := readFrameChan(feed(raw), time.Second)
	if err != nil {
		t.Fatalf("readFrameChan: %v", err)
	}
	if !encrypted {
		t.Fatal("AES-wrapped frame not reported as encrypted")
	}
}

// A directed query that gets no answer is the inverter's silence, not the
// module's. The operator reads that distinction in the error message.
func TestReadFrameChanSilenceIsInverterSilence(t *testing.T) {
	in := make(chan []byte)
	_, _, err := readFrameChan(in, 20*time.Millisecond)
	if !errors.Is(err, errNoReply) {
		t.Fatalf("err = %v, want errNoReply", err)
	}
	if errors.Is(err, errNoAck) {
		t.Fatal("inverter silence must not report as a modem ack failure")
	}
}

func TestReadFrameChanClosedSinkIsInverterSilence(t *testing.T) {
	in := make(chan []byte)
	close(in)
	_, _, err := readFrameChan(in, time.Second)
	if !errors.Is(err, errNoReply) {
		t.Fatalf("err = %v, want errNoReply", err)
	}
}

// When the sink closes or the deadline passes, the reader returns the bytes
// it already accumulated as a frame. It does not discard them as a timeout.
func TestReadFrameChanReturnsPartialOnClose(t *testing.T) {
	in := make(chan []byte, 1)
	in <- []byte{0xA5, 0xA5}
	close(in)
	frame, _, err := readFrameChan(in, time.Second)
	if err != nil {
		t.Fatalf("readFrameChan: %v", err)
	}
	if want := []byte{0xA5, 0xA5}; !bytes.Equal(frame, want) {
		t.Fatalf("frame = % X, want % X", frame, want)
	}
}

// scanAck is the shared ack recogniser behind both readers. The accumulator
// it returns must keep enough tail to span an ack split across two reads. It
// must never grow without bound on a chatty bus.
func TestScanAckTrimsAccumulatorButSpansSplitAck(t *testing.T) {
	acc, ack := scanAck(nil, bytes.Repeat([]byte{0x00}, ackScanWindow+10))
	if ack != nil {
		t.Fatalf("ack = % X, want none in an all-zero stream", ack)
	}
	if len(acc) > ackScanWindow {
		t.Fatalf("accumulator grew to %d, want <= %d", len(acc), ackScanWindow)
	}
	// The ack straddles the trim: 0xAB lands in the trimmed tail and its two
	// trailing bytes arrive in the next chunk.
	acc, ack = scanAck(acc, []byte{0xAB})
	if ack != nil {
		t.Fatal("ack reported before its trailing bytes arrived")
	}
	if _, ack = scanAck(acc, []byte{0x26, 0x52}); ack == nil {
		t.Fatal("ack split across chunks was not recognised")
	} else if want := []byte{0xAB, 0x26, 0x52}; !bytes.Equal(ack, want) {
		t.Fatalf("ack = % X, want % X", ack, want)
	}
}

// drainChan runs inside writeFrame while it holds the shared modem-write
// mutex. A closed sink must therefore end the drain. It must not spin on the
// always-ready receive.
func TestDrainChanReturnsOnClosedSink(t *testing.T) {
	in := make(chan []byte, 1)
	in <- []byte{0x01}
	close(in)
	done := make(chan struct{})
	go func() {
		drainChan(in)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("drainChan did not return on a closed sink")
	}
}

// A neighbouring device that holds the bus busy must not decide how much we
// accumulate. The frame reader returns once it reaches the ceiling.
func TestReadFrameChanCapsAccumulator(t *testing.T) {
	in := make(chan []byte, 8)
	go func() {
		for i := 0; i < 8; i++ {
			in <- bytes.Repeat([]byte{0x5A}, 1024)
		}
	}()
	frame, _, err := readFrameChan(in, 5*time.Second)
	if err != nil {
		t.Fatalf("readFrameChan: %v", err)
	}
	if len(frame) > maxReplyBytes+1024 {
		t.Fatalf("accumulated %d bytes, want the reader to stop near %d", len(frame), maxReplyBytes)
	}
}

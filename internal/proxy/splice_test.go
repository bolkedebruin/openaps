package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	tap "github.com/bolke/ecu-zb/internal/tap"
)

// rwPair adapts a separate Reader and Writer into one io.ReadWriter.
type rwPair struct {
	io.Reader
	io.Writer
}

// spliceFixture sets up four io.Pipe pairs simulating the wire on each
// side of the splice.
//
//	test  →  fromHostW  →  splice.Host.Read   →  splice.Modem.Write  →  toModemR  →  test
//	test  →  fromModemW →  splice.Modem.Read  →  splice.Host.Write   →  toHostR   →  test
type spliceFixture struct {
	fromHostW  *io.PipeWriter
	fromModemW *io.PipeWriter
	toHostR    *io.PipeReader
	toModemR   *io.PipeReader

	broadcaster *tap.Broadcaster
	sink        *recordingSink

	splice *Splice

	cancel context.CancelFunc
	done   chan error
}

func newFixture(t *testing.T, hook Hook) *spliceFixture {
	t.Helper()
	fromHostR, fromHostW := io.Pipe()
	fromModemR, fromModemW := io.Pipe()
	toHostR, toHostW := io.Pipe()
	toModemR, toModemW := io.Pipe()

	host := &rwPair{Reader: fromHostR, Writer: toHostW}
	modem := &rwPair{Reader: fromModemR, Writer: toModemW}

	br := tap.NewBroadcaster(tap.SectionInfo{Hardware: "test", OS: "test", UserAppl: "test"})
	sink := &recordingSink{}
	sink.detach, sink.done = br.Attach(sink)

	s := &Splice{
		Modem:   modem,
		Host:    host,
		Hook:    hook,
		Tap:     br,
		BufSize: 64,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	// Drain the broadcaster header that arrived as the first message
	// to the sink; tests below assert on EPB blocks only.
	sink.consumeHeader(t)

	return &spliceFixture{
		fromHostW: fromHostW, fromModemW: fromModemW,
		toHostR: toHostR, toModemR: toModemR,
		broadcaster: br, sink: sink,
		splice: s,
		cancel: cancel, done: done,
	}
}

func (f *spliceFixture) shutdown(t *testing.T) {
	t.Helper()
	f.cancel()
	// Force EOF on both reader pipes so the splice goroutines unwind.
	_ = f.fromHostW.Close()
	_ = f.fromModemW.Close()
	// Drain writer pipes so any in-flight Write doesn't block forever.
	go io.Copy(io.Discard, f.toHostR)
	go io.Copy(io.Discard, f.toModemR)
	select {
	case <-f.done:
	case <-time.After(2 * time.Second):
		t.Fatal("splice did not exit within 2s")
	}
	f.sink.detach()
}

// recordingSink captures every Write so tests can inspect the pcapng
// stream the broadcaster emitted.
type recordingSink struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	written [][]byte
	detach  func()
	done    <-chan struct{}
}

func (s *recordingSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := append([]byte(nil), p...)
	s.written = append(s.written, cp)
	s.buf.Write(p)
	return len(p), nil
}

func (s *recordingSink) blocks() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, len(s.written))
	copy(out, s.written)
	return out
}

func (s *recordingSink) consumeHeader(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(s.blocks()) >= 1 {
			s.mu.Lock()
			s.written = nil
			s.buf.Reset()
			s.mu.Unlock()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("broadcaster header never reached sink")
}

// waitForBlocks polls until at least n EPB blocks have arrived.
func (s *recordingSink) waitForBlocks(t *testing.T, n int) [][]byte {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := s.blocks(); len(got) >= n {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("only got %d blocks, want %d", len(s.blocks()), n)
	return nil
}

// directionByteOf returns the direction byte (offset 28 in the EPB:
// blockType(4) + length(4) + iface(4) + tsHi(4) + tsLo(4) + capLen(4)
// + origLen(4) = 28 bytes header before payload, payload[0] = direction).
func directionByteOf(t *testing.T, block []byte) byte {
	t.Helper()
	if len(block) < 29 {
		t.Fatalf("block too short: %d", len(block))
	}
	return block[28]
}

// ifaceIDOf decodes the interface_id field from an EPB (offset 8).
func ifaceIDOf(t *testing.T, block []byte) uint32 {
	t.Helper()
	if len(block) < 12 {
		t.Fatalf("block too short: %d", len(block))
	}
	return uint32(block[8]) | uint32(block[9])<<8 | uint32(block[10])<<16 | uint32(block[11])<<24
}

// chunkBytesOf returns the raw chunk bytes inside an EPB (skipping the
// 28-byte EPB header and the 1-byte direction prefix).
func chunkBytesOf(t *testing.T, block []byte) []byte {
	t.Helper()
	// EPB layout: blockType(4) + length(4) + iface(4) + tsHi(4) + tsLo(4) +
	// capLen(4) + origLen(4) + payload(capLen, padded) + length(4 trailer).
	if len(block) < 28 {
		t.Fatalf("block too short: %d", len(block))
	}
	capLen := uint32(block[20]) | uint32(block[21])<<8 | uint32(block[22])<<16 | uint32(block[23])<<24
	if capLen < 1 {
		return nil
	}
	return block[29 : 28+int(capLen)]
}

func TestSplice_NoOpHook_PassThrough_HostToModem(t *testing.T) {
	f := newFixture(t, NoOpHook{})
	defer f.shutdown(t)

	want := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x05, 0x00}
	go func() { _, _ = f.fromHostW.Write(want) }()

	got := make([]byte, len(want))
	if _, err := io.ReadFull(f.toModemR, got); err != nil {
		t.Fatalf("read from modem side: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("forwarded bytes mismatch: got %x want %x", got, want)
	}
}

func TestSplice_NoOpHook_PassThrough_ModemToHost(t *testing.T) {
	f := newFixture(t, NoOpHook{})
	defer f.shutdown(t)

	want := []byte{0xFC, 0xFC, 0x55, 0x01, 0xFE, 0xFE}
	go func() { _, _ = f.fromModemW.Write(want) }()

	got := make([]byte, len(want))
	if _, err := io.ReadFull(f.toHostR, got); err != nil {
		t.Fatalf("read from host side: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("forwarded bytes mismatch: got %x want %x", got, want)
	}
}

func TestSplice_TapPublishesBothDirections(t *testing.T) {
	f := newFixture(t, NoOpHook{})
	defer f.shutdown(t)

	go func() { _, _ = f.fromHostW.Write([]byte{0x11}) }()
	_, _ = io.ReadFull(f.toModemR, make([]byte, 1))

	go func() { _, _ = f.fromModemW.Write([]byte{0x22}) }()
	_, _ = io.ReadFull(f.toHostR, make([]byte, 1))

	blocks := f.sink.waitForBlocks(t, 2)
	dirs := []byte{directionByteOf(t, blocks[0]), directionByteOf(t, blocks[1])}
	// Order between the two directions isn't fixed (separate goroutines),
	// so accept either ordering.
	if !((dirs[0] == byte(DirToModem) && dirs[1] == byte(DirToHost)) ||
		(dirs[0] == byte(DirToHost) && dirs[1] == byte(DirToModem))) {
		t.Fatalf("unexpected direction pair: %v", dirs)
	}

	wantBytes := map[byte]byte{byte(DirToModem): 0x11, byte(DirToHost): 0x22}
	for _, blk := range blocks {
		dir := directionByteOf(t, blk)
		got := chunkBytesOf(t, blk)
		if len(got) != 1 || got[0] != wantBytes[dir] {
			t.Fatalf("dir %d chunk mismatch: got %x want %02x", dir, got, wantBytes[dir])
		}
	}
}

// dropDirection drops every chunk in the given direction.
type dropDirection struct{ dir FrameDirection }

func (h dropDirection) OnChunk(d FrameDirection, _ []byte) ChunkAction {
	return ChunkAction{Drop: d == h.dir}
}

func TestSplice_HookDrop_PreventsForwarding(t *testing.T) {
	f := newFixture(t, dropDirection{dir: DirToModem})
	defer f.shutdown(t)

	go func() { _, _ = f.fromHostW.Write([]byte{0x42, 0x42, 0x42}) }()

	// The host→modem direction is dropped: the modem side should
	// receive nothing within the timeout.
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(f.toModemR, make([]byte, 1))
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("modem received bytes despite drop hook: err=%v", err)
	case <-time.After(150 * time.Millisecond):
	}

	// But the modem→host direction must still pass.
	go func() { _, _ = f.fromModemW.Write([]byte{0x99}) }()
	got := make([]byte, 1)
	if _, err := io.ReadFull(f.toHostR, got); err != nil {
		t.Fatalf("modem→host read: %v", err)
	}
	if got[0] != 0x99 {
		t.Fatalf("got %x want 99", got)
	}
}

// xorHook flips every byte in the chunk before forwarding.
type xorHook struct{}

func (xorHook) OnChunk(_ FrameDirection, raw []byte) ChunkAction {
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = b ^ 0xFF
	}
	return ChunkAction{Altered: out}
}

func TestSplice_HookAlter_RewritesForwardedBytes(t *testing.T) {
	f := newFixture(t, xorHook{})
	defer f.shutdown(t)

	in := []byte{0x00, 0x55, 0xFF}
	go func() { _, _ = f.fromHostW.Write(in) }()

	got := make([]byte, len(in))
	if _, err := io.ReadFull(f.toModemR, got); err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []byte{0xFF, 0xAA, 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("altered bytes: got %x want %x", got, want)
	}
}

func TestSplice_EOFOnOneSide_StopsCleanly(t *testing.T) {
	f := newFixture(t, NoOpHook{})
	// don't defer shutdown — this test drives termination explicitly

	// Close the host-input pipe: splice's host→modem goroutine should
	// see io.EOF on Read and return cleanly. The modem→host goroutine
	// is still blocked on its own Read; cancel the context to unwind it.
	_ = f.fromHostW.Close()
	f.cancel()
	_ = f.fromModemW.Close()
	go io.Copy(io.Discard, f.toHostR)
	go io.Copy(io.Discard, f.toModemR)

	select {
	case err := <-f.done:
		// EOF on host side returns nil from copyOne; cancel on the
		// other side surfaces context.Canceled. Either is acceptable.
		if err != nil && err != context.Canceled {
			t.Logf("splice exited with: %v (acceptable)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("splice did not exit after EOF + cancel")
	}
	f.sink.detach()
}

// markMineHook drops every inbound chunk and marks it Mine.
type markMineHook struct{}

func (markMineHook) OnChunk(dir FrameDirection, _ []byte) ChunkAction {
	if dir == DirToHost {
		return ChunkAction{Drop: true, Mine: true}
	}
	return ChunkAction{}
}

func TestSplice_HookMine_RoutesInboundToInjectInterface(t *testing.T) {
	f := newFixture(t, markMineHook{})
	defer f.shutdown(t)

	// Outbound (host→modem): NoOp behavior on outbound, so iface=wire.
	go func() { _, _ = f.fromHostW.Write([]byte{0x11}) }()
	_, _ = io.ReadFull(f.toModemR, make([]byte, 1))

	// Inbound: hook says Drop+Mine, so iface=inject. Bytes don't reach
	// the host pipe (drop) but the tap publishes them with iface=1.
	go func() { _, _ = f.fromModemW.Write([]byte{0x22}) }()
	// Give the splice a moment to process; can't ReadFull on toHostR
	// because the chunk was dropped.
	time.Sleep(100 * time.Millisecond)

	blocks := f.sink.waitForBlocks(t, 2)

	var sawWireOut, sawInjectIn bool
	for _, blk := range blocks {
		dir := directionByteOf(t, blk)
		iface := ifaceIDOf(t, blk)
		switch {
		case dir == byte(DirToModem) && iface == tap.IfaceWire:
			sawWireOut = true
		case dir == byte(DirToHost) && iface == tap.IfaceInject:
			sawInjectIn = true
		case dir == byte(DirToHost) && iface == tap.IfaceWire:
			t.Fatalf("inbound chunk landed on wire iface despite hook.Mine=true: %+v", blk)
		}
	}
	if !sawWireOut {
		t.Fatal("missing outbound on wire iface")
	}
	if !sawInjectIn {
		t.Fatal("missing inbound on inject iface")
	}
}

func TestSplice_InjectToModem_PublishesOnInjectInterface(t *testing.T) {
	f := newFixture(t, NoOpHook{})
	defer f.shutdown(t)

	// Drain the modem side so InjectToModem's Write doesn't block.
	go io.Copy(io.Discard, f.toModemR)

	if err := f.splice.InjectToModem([]byte{0xAA, 0xAA, 0xAA, 0xAA, 0x55}); err != nil {
		t.Fatalf("InjectToModem: %v", err)
	}

	blocks := f.sink.waitForBlocks(t, 1)
	if len(blocks) < 1 {
		t.Fatal("no block published")
	}
	last := blocks[len(blocks)-1]
	if got := ifaceIDOf(t, last); got != tap.IfaceInject {
		t.Fatalf("InjectToModem published on iface=%d, want %d", got, tap.IfaceInject)
	}
	if got := directionByteOf(t, last); got != byte(DirToModem) {
		t.Fatalf("inject direction: got %d want %d", got, DirToModem)
	}
}

func TestSplice_BurstLargerThanBufSize_StreamsInChunks(t *testing.T) {
	f := newFixture(t, NoOpHook{})
	defer f.shutdown(t)

	// BufSize is 64; write 200 bytes — splice should pull them through
	// in multiple Read passes without losing any.
	want := make([]byte, 200)
	for i := range want {
		want[i] = byte(i)
	}
	go func() { _, _ = f.fromHostW.Write(want) }()

	got := make([]byte, len(want))
	if _, err := io.ReadFull(f.toModemR, got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("burst content mismatch: first diff at byte %d", firstDiff(got, want))
	}
}

// faultyHost is an io.ReadWriter that returns syscall.EIO on its
// next Read call (after which a sentinel returnsEOF flips it to
// behave as a closed pipe). Writes always return EIO. It models a
// pty master whose last slave just closed.
type faultyHost struct {
	mu       sync.Mutex
	armed    atomic.Bool
	closedCh chan struct{}
}

func newFaultyHost() *faultyHost {
	h := &faultyHost{closedCh: make(chan struct{})}
	h.armed.Store(true)
	return h
}

func (h *faultyHost) Read(p []byte) (int, error) {
	if h.armed.Swap(false) {
		return 0, &syscallErr{err: syscall.EIO}
	}
	<-h.closedCh
	return 0, io.EOF
}

func (h *faultyHost) Write(p []byte) (int, error) {
	return 0, &syscallErr{err: syscall.EIO}
}

func (h *faultyHost) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.closedCh:
	default:
		close(h.closedCh)
	}
}

// syscallErr wraps a syscall.Errno so errors.Is(err, syscall.EIO)
// works on a non-os.PathError-style error.
type syscallErr struct{ err error }

func (e *syscallErr) Error() string { return e.err.Error() }
func (e *syscallErr) Unwrap() error { return e.err }

func TestSplice_HostEIO_TriggersReopener(t *testing.T) {
	// Start with a faulty host that fails its first Read with EIO.
	// The reopener swaps in a working host (io.Pipe-backed), and
	// the splice should keep running and forward bytes via the new
	// host without exiting.
	bad := newFaultyHost()
	defer bad.close()

	fromHostR, fromHostW := io.Pipe()
	fromModemR, fromModemW := io.Pipe()
	toHostR, toHostW := io.Pipe()
	toModemR, toModemW := io.Pipe()

	good := &rwPair{Reader: fromHostR, Writer: toHostW}
	modem := &rwPair{Reader: fromModemR, Writer: toModemW}

	br := tap.NewBroadcaster(tap.SectionInfo{Hardware: "test", OS: "test", UserAppl: "test"})
	sink := &recordingSink{}
	sink.detach, sink.done = br.Attach(sink)

	var reopens atomic.Int32
	s := &Splice{
		Modem:   modem,
		Host:    bad,
		Hook:    NoOpHook{},
		Tap:     br,
		BufSize: 64,
		HostReopener: func(prev io.ReadWriter) (io.ReadWriter, error) {
			reopens.Add(1)
			if prev != bad {
				t.Errorf("reopener got prev=%T, want *faultyHost", prev)
			}
			return good, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	sink.consumeHeader(t)

	// Forward: data on the new host (via fromHostW) must reach modem.
	want := []byte{0x01, 0x02, 0x03}
	go func() { _, _ = fromHostW.Write(want) }()
	got := make([]byte, len(want))
	if _, err := io.ReadFull(toModemR, got); err != nil {
		t.Fatalf("read from modem after reopen: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("post-reopen forward mismatch: got %x want %x", got, want)
	}
	if r := reopens.Load(); r != 1 {
		t.Fatalf("reopener called %d times, want 1", r)
	}

	// Clean shutdown.
	cancel()
	_ = fromHostW.Close()
	_ = fromModemW.Close()
	go io.Copy(io.Discard, toHostR)
	go io.Copy(io.Discard, toModemR)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("splice did not exit within 2s")
	}
	sink.detach()
}

func TestSplice_HostEIO_NoReopener_ReturnsErrHostFault(t *testing.T) {
	bad := newFaultyHost()
	defer bad.close()

	// Close the modem-side input so the modem→host goroutine
	// exits via io.EOF; the host→modem goroutine then surfaces
	// ErrHostFault as the first non-nil error.
	fromModemR, fromModemW := io.Pipe()
	_ = fromModemW.Close()
	_, toModemW := io.Pipe()

	modem := &rwPair{Reader: fromModemR, Writer: toModemW}

	br := tap.NewBroadcaster(tap.SectionInfo{Hardware: "test", OS: "test", UserAppl: "test"})
	sink := &recordingSink{}
	sink.detach, sink.done = br.Attach(sink)
	defer sink.detach()

	s := &Splice{
		Modem:   modem,
		Host:    bad,
		Hook:    NoOpHook{},
		Tap:     br,
		BufSize: 64,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	sink.consumeHeader(t)

	select {
	case err := <-done:
		if !errors.Is(err, ErrHostFault) {
			t.Fatalf("want ErrHostFault, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("splice did not exit on EIO without reopener")
	}
}

func firstDiff(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// hostReader continuously drains toHostR into a channel so a single read
// position is maintained across pause/resume (avoids leaked partial reads).
type hostReader struct {
	ch chan byte
}

func newHostReader(r io.Reader) *hostReader {
	hr := &hostReader{ch: make(chan byte, 16)}
	go func() {
		b := make([]byte, 1)
		for {
			n, err := r.Read(b)
			if n > 0 {
				hr.ch <- b[0]
			}
			if err != nil {
				return
			}
		}
	}()
	return hr
}

// next returns the next forwarded byte, or ok=false if none within d.
func (hr *hostReader) next(d time.Duration) (byte, bool) {
	select {
	case b := <-hr.ch:
		return b, true
	case <-time.After(d):
		return 0, false
	}
}

// TestBeginPairing_RedirectsModemToSink verifies that during pairing the
// single modem reader delivers modem bytes to the sink channel (not the host),
// and that normal modem→host forwarding resumes after EndPairing. This is the
// fix for the dual-reader race: the runner reads its reply from the sink, so
// there is never a second reader stealing the modem ack.
func TestBeginPairing_RedirectsModemToSink(t *testing.T) {
	f := newFixture(t, NoOpHook{})
	defer f.shutdown(t)
	hr := newHostReader(f.toHostR)

	// Baseline: modem→host forwarding works before pairing.
	go func() { _, _ = f.fromModemW.Write([]byte{0x01}) }()
	if b, ok := hr.next(time.Second); !ok || b != 0x01 {
		t.Fatalf("pre-pairing byte = 0x%02X ok=%v, want 0x01", b, ok)
	}

	sink := f.splice.BeginPairing()

	// Modem bytes now arrive on the sink, not the host.
	go func() { _, _ = f.fromModemW.Write([]byte{0xAB, 0xCD, 0xEF}) }()
	var got []byte
	deadline := time.After(time.Second)
	for len(got) < 3 {
		select {
		case chunk := <-sink:
			got = append(got, chunk...)
		case <-deadline:
			t.Fatalf("sink received %X, want AB CD EF", got)
		}
	}
	if !bytes.Equal(got[:3], []byte{0xAB, 0xCD, 0xEF}) {
		t.Fatalf("sink bytes = %X, want AB CD EF", got)
	}
	// The host must NOT have received the pairing reply.
	if b, ok := hr.next(200 * time.Millisecond); ok {
		t.Fatalf("byte 0x%02X reached host during pairing", b)
	}

	f.splice.EndPairing()
	// After EndPairing modem→host forwarding resumes.
	go func() { _, _ = f.fromModemW.Write([]byte{0x03}) }()
	if _, ok := hr.next(time.Second); !ok {
		t.Fatalf("no byte forwarded after EndPairing")
	}
}

// eagainHost is an io.ReadWriter whose Write always returns EAGAIN — it
// models an O_NONBLOCK pty master whose kernel buffer is full because the
// slave is not being drained (e.g. main.exe stubbed). Read blocks until
// close() so the host→modem copy goroutine doesn't busy-spin.
type eagainHost struct {
	mu       sync.Mutex
	closedCh chan struct{}
	writes   atomic.Int32
}

func newEagainHost() *eagainHost { return &eagainHost{closedCh: make(chan struct{})} }

func (h *eagainHost) Read(p []byte) (int, error) {
	<-h.closedCh
	return 0, io.EOF
}

func (h *eagainHost) Write(p []byte) (int, error) {
	h.writes.Add(1)
	return 0, &syscallErr{err: syscall.EAGAIN}
}

func (h *eagainHost) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.closedCh:
	default:
		close(h.closedCh)
	}
}

// TestSplice_HostEAGAIN_DropsChunkAndContinues verifies that EAGAIN on a
// host write is treated as a drop, not a fault: the splice keeps running,
// subsequent modem→host bytes are still attempted (and still drop), and the
// modem→host copy goroutine never blocks on the unread pty master. This is
// the kernel-pty-buffer back-pressure path: with main.exe stubbed and the
// master O_NONBLOCK, blocking writes would have stalled the modem reader
// and starved a pairing reply waiting on the sink.
func TestSplice_HostEAGAIN_DropsChunkAndContinues(t *testing.T) {
	bad := newEagainHost()
	defer bad.close()

	fromModemR, fromModemW := io.Pipe()
	_, toModemW := io.Pipe()
	modem := &rwPair{Reader: fromModemR, Writer: toModemW}

	br := tap.NewBroadcaster(tap.SectionInfo{Hardware: "test", OS: "test", UserAppl: "test"})
	sink := &recordingSink{}
	sink.detach, sink.done = br.Attach(sink)
	defer sink.detach()

	s := &Splice{
		Modem:   modem,
		Host:    bad,
		Hook:    NoOpHook{},
		Tap:     br,
		BufSize: 64,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	sink.consumeHeader(t)

	// Push two chunks through modem→host; both must be dropped (EAGAIN)
	// without the splice returning an error.
	go func() {
		_, _ = fromModemW.Write([]byte{0x01, 0x02, 0x03})
		_, _ = fromModemW.Write([]byte{0x04, 0x05})
	}()

	// Give the splice time to process both writes.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if bad.writes.Load() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := bad.writes.Load(); got < 2 {
		t.Fatalf("host write attempts = %d, want >= 2", got)
	}

	// The splice must NOT have exited — EAGAIN is not a fault.
	select {
	case err := <-done:
		t.Fatalf("splice exited unexpectedly on EAGAIN write: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	cancel()
	_ = fromModemW.Close()
	bad.close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("splice did not exit after cancel")
	}
}

// TestSplice_ModemWriteMu_SharedAcrossWriters verifies the mutex returned
// by ModemWriteMu is the same instance the internal write path uses, so a
// pairing runner wired with this mutex serialises against splice
// DirToModem writes (no interleaved bytes between a flush and the next
// frame).
func TestSplice_ModemWriteMu_SharedAcrossWriters(t *testing.T) {
	f := newFixture(t, NoOpHook{})
	defer f.shutdown(t)

	mu := f.splice.ModemWriteMu()
	if mu == nil {
		t.Fatal("ModemWriteMu returned nil")
	}
	// Hold the mutex; an InjectToModem must block until we release.
	mu.Lock()
	go io.Copy(io.Discard, f.toModemR)
	injected := make(chan error, 1)
	go func() { injected <- f.splice.InjectToModem([]byte{0x99}) }()
	select {
	case err := <-injected:
		t.Fatalf("InjectToModem completed while ModemWriteMu was held: err=%v", err)
	case <-time.After(100 * time.Millisecond):
	}
	mu.Unlock()
	select {
	case err := <-injected:
		if err != nil {
			t.Fatalf("InjectToModem after unlock: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("InjectToModem did not complete after unlock")
	}
}

// TestBeginPairing_PausesHostToModem verifies host→modem is paused during
// pairing (so main.exe traffic can't interleave with the runner's writes) and
// resumes after EndPairing.
func TestBeginPairing_PausesHostToModem(t *testing.T) {
	f := newFixture(t, NoOpHook{})
	defer f.shutdown(t)
	mr := newHostReader(f.toModemR)

	// Baseline: host→modem forwarding works before pairing.
	go func() { _, _ = f.fromHostW.Write([]byte{0x10}) }()
	if b, ok := mr.next(time.Second); !ok || b != 0x10 {
		t.Fatalf("pre-pairing host→modem byte = 0x%02X ok=%v, want 0x10", b, ok)
	}

	f.splice.BeginPairing()
	time.Sleep(50 * time.Millisecond)
	go func() { _, _ = f.fromHostW.Write([]byte{0x20}) }()
	if b, ok := mr.next(300 * time.Millisecond); ok {
		t.Fatalf("host byte 0x%02X reached modem during pairing", b)
	}

	f.splice.EndPairing()
	go func() { _, _ = f.fromHostW.Write([]byte{0x30}) }()
	if _, ok := mr.next(time.Second); !ok {
		t.Fatalf("no host→modem byte forwarded after EndPairing")
	}
}

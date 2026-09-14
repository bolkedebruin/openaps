package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bolkedebruin/openaps/internal/zigbee/tap"
)

// logEveryDrops bounds how often the EAGAIN-drop counter emits a log
// line. A pty whose slave is permanently unread would otherwise produce
// a stream of identical messages.
const logEveryDrops = 256

// nonblockReadIdle is the brief sleep applied when an O_NONBLOCK read
// returns EAGAIN. Keeps the copy goroutine from busy-spinning when no
// data is available without adding noticeable latency to a real chunk.
const nonblockReadIdle = 2 * time.Millisecond

// modemReopenSettle is the pause after reopening the modem fd, before the
// reader tries again. A tty that has just been hung up and reopened needs a
// moment before it yields data; the pause also bounds the retry rate if the
// fresh fd faults immediately.
const modemReopenSettle = 250 * time.Millisecond

// maxModemReopens bounds how many times the modem fd is reopened without a
// single successful read in between. A device that hangs up again on every
// fresh fd is not coming back in-process, so the splice stops retrying and
// faults instead of reopening forever.
const maxModemReopens = 5

// isEAGAIN reports whether err is the EAGAIN/EWOULDBLOCK condition
// returned by an O_NONBLOCK fd that would otherwise have blocked.
func isEAGAIN(err error) bool {
	return errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK)
}

// ErrHostFault is returned (typically wrapped) when the pty master
// returns EIO and either no HostReopener was configured or the
// reopener itself failed. EIO on the master happens when the last
// slave fd closes — for ecu-zb that means the host reader was killed.
var ErrHostFault = errors.New("host pty fault")

// ErrModemFault is returned when the modem fd hangs up (read reports EOF)
// outside of a shutdown and cannot be recovered in-process: either no
// ModemReopener is configured, the reopener failed, or the fresh fd hung up
// again maxModemReopens times without ever yielding a byte. A hung-up tty
// returns EOF forever, so the fd must be replaced — the modem→host copy
// goroutine is the single reader of that fd, and if it ends the proxy runs
// write-only.
var ErrModemFault = errors.New("modem fd fault")

// Splice copies bytes bidirectionally between the real modem UART and
// the pty master the host process is talking to, mirroring every chunk
// into the broadcaster.
type Splice struct {
	Modem io.ReadWriter // real CC2530 UART
	Host  io.ReadWriter // initial pty master (host sees the slave)

	Hook Hook
	Tap  *tap.Broadcaster

	// BufSize bounds each tty read. Defaults to 64 — the wire chunks
	// are ≤16 bytes per the existing Lua dissector's observations,
	// 64 is a safe ceiling without burning memory.
	BufSize int

	// hostDropCount counts host writes dropped because the pty master
	// returned EAGAIN (kernel buffer full — no slave drainer). Logged
	// once per logEveryDrops drops so a chronically unread pty doesn't
	// flood the log.
	hostDropCount uint64

	// HostReopener, if non-nil, is invoked when the Host pty master
	// returns EIO. The implementation must close prev, allocate a
	// fresh pty master (typically via uart.OpenPTY plus repointing
	// /dev/ttyO2), and return it. Reopens are serialised by an
	// internal mutex; if both copy goroutines hit EIO at the same
	// time, the loser sees the new master already installed and
	// retries against it.
	//
	// If HostReopener is nil, an EIO on Host is fatal and Run
	// returns ErrHostFault.
	HostReopener func(prev io.ReadWriter) (io.ReadWriter, error)

	// modemMu serialises writes to Modem so a hook's InjectToModem can't
	// interleave bytes with the host→modem copy goroutine. The pairing
	// runner takes this same mutex around its flush+sleep+write sequences
	// (see ModemWriteMu) so there is one, and only one, modem-fd write
	// serialiser across every writer (splice DirToModem, hook
	// InjectToModem, busmgr inject, pairing runner). Any new modem writer
	// MUST take this mutex. It doubles as the guard that keeps a reopen
	// from closing the descriptor out from under an in-flight write (see
	// ModemReopener).
	//
	// LOCK ORDER: modemPort's lock is always taken BEFORE modemMu, never
	// while holding it. Writers therefore resolve the port first and lock
	// second; a reopen holds the port lock and then takes modemMu to close
	// the port it replaced.
	modemMu sync.Mutex

	// ModemReopener, if non-nil, is invoked when the modem fd hangs up
	// (read returns EOF while the proxy is still running). The
	// implementation must close prev, open a fresh port on the same
	// device, and return it. A hung-up tty yields EOF on every subsequent
	// read, so replacing the fd is the only in-process recovery; the
	// alternative is a proxy that keeps writing polls the radio answers
	// into a descriptor nobody can read.
	//
	// It MUST close prev while holding ModemWriteMu, so the descriptor
	// number cannot be freed — and reused by an unrelated open —
	// underneath a write still in flight: the pairing runner writes via
	// the raw fd, where that would land ZigBee frames in whatever file
	// inherited the number.
	//
	// If ModemReopener is nil, an EOF on Modem is fatal and Run returns
	// ErrModemFault.
	ModemReopener func(prev io.ReadWriter) (io.ReadWriter, error)

	// hostPort and modemPort hold the descriptor each side is currently
	// using and swap in a replacement when it faults. They are distinct
	// from modemMu, which serialises writers: a reopen must exclude
	// readers too, and a writer blocked on a wedged port must not block
	// the reopen that frees it.
	hostPort  swappablePort
	modemPort swappablePort

	// gate pauses the host→modem copy goroutine during a pairing primitive
	// so host traffic can't interleave with the runner's writes.
	gate interceptGate

	// pairing redirects the SINGLE modem reader (the modem→host copy
	// goroutine) to sink instead of the host, so an OTA pairing primitive
	// reads modem replies through that one reader rather than opening a
	// second reader on the fd. A second concurrent reader caused config-op
	// acks to be consumed by the copy goroutine's in-flight read (the
	// dual-reader race). Guarded by pairingMu.
	pairingMu sync.Mutex
	pairing   bool
	sink      chan []byte
}

// BeginPairing switches the modem→host reader into redirect mode and pauses
// host→modem. It returns the sink channel the pairing runner reads modem
// replies from. Pair with EndPairing (typically deferred). The single modem
// reader keeps owning the fd; the runner never reads it directly.
func (s *Splice) BeginPairing() <-chan []byte {
	sink := make(chan []byte, 256)
	s.pairingMu.Lock()
	s.pairing = true
	s.sink = sink
	s.pairingMu.Unlock()
	s.gate.pause()
	return sink
}

// EndPairing restores normal modem→host forwarding and resumes host→modem.
// The sink is dropped (not closed) so a redirect send racing this call hits a
// nil channel in a select-default and is discarded rather than panicking.
func (s *Splice) EndPairing() {
	s.gate.resume()
	s.pairingMu.Lock()
	s.pairing = false
	s.sink = nil
	s.pairingMu.Unlock()
}

// pairingState returns whether pairing is active and the current sink.
func (s *Splice) pairingState() (bool, chan []byte) {
	s.pairingMu.Lock()
	defer s.pairingMu.Unlock()
	return s.pairing, s.sink
}

// interceptGate is a counting pause barrier. When paused (count > 0) the
// copy goroutines block at the top of each iteration until resumed. The
// barrier is checked between reads, so a read already blocked in the kernel
// is not interrupted — callers must flush the modem after pausing (the
// pairing runner does) so any pre-pause in-flight bytes are discarded.
type interceptGate struct {
	mu     sync.Mutex
	cond   *sync.Cond
	paused int
}

func (g *interceptGate) init() {
	if g.cond == nil {
		g.cond = sync.NewCond(&g.mu)
	}
}

// pause raises the barrier; copy goroutines stop at their next iteration.
func (g *interceptGate) pause() {
	g.mu.Lock()
	g.init()
	g.paused++
	g.mu.Unlock()
}

// resume lowers the barrier and wakes the copy goroutines.
func (g *interceptGate) resume() {
	g.mu.Lock()
	g.init()
	if g.paused > 0 {
		g.paused--
	}
	if g.paused == 0 {
		g.cond.Broadcast()
	}
	g.mu.Unlock()
}

// wait blocks while the barrier is raised. Returns false if ctx is done.
func (g *interceptGate) wait(ctx context.Context) bool {
	g.mu.Lock()
	g.init()
	for g.paused > 0 {
		if ctx.Err() != nil {
			g.mu.Unlock()
			return false
		}
		// Wake periodically so a cancelled ctx during a pause is noticed
		// even though Cond has no ctx-aware Wait.
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				g.mu.Lock()
				g.cond.Broadcast()
				g.mu.Unlock()
			case <-done:
			}
		}()
		g.cond.Wait()
		close(done)
	}
	g.mu.Unlock()
	return ctx.Err() == nil
}

// Run blocks until both copy goroutines have stopped or ctx is done.
// It returns the first non-EOF error observed.
func (s *Splice) Run(ctx context.Context) error {
	if s.Hook == nil {
		s.Hook = NoOpHook{}
	}
	if s.BufSize <= 0 {
		s.BufSize = 64
	}
	s.hostPort.configure(s.Host, s.HostReopener, ErrHostFault)
	s.modemPort.configure(s.Modem, s.ModemReopener, ErrModemFault)

	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	record := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
		cancel()
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		err := s.copyOne(cctx, "host→modem", DirToModem)
		record(err)
	}()
	go func() {
		defer wg.Done()
		err := s.copyOne(cctx, "modem→host", DirToHost)
		record(err)
	}()

	wg.Wait()
	return firstErr
}

// currentHost returns the pty master currently in use, falling back to the
// configured Host for callers that run before Run.
func (s *Splice) currentHost() io.ReadWriter {
	if rw := s.hostPort.current(); rw != nil {
		return rw
	}
	return s.Host
}

// swappablePort holds the descriptor a side of the splice is using and
// replaces it when that descriptor faults. Both sides need this and for the
// same reason: a pty master returns EIO once its last slave closes, and a tty
// returns EOF forever once it has hung up. Neither recovers by retrying — the
// descriptor itself has to go — so the recovery is identical on both sides
// and lives here once.
//
// reopen is the caller-supplied replacement. When it is nil the fault is
// unrecoverable and fault returns noReopener unwrapped, so callers can test
// for their own sentinel.
type swappablePort struct {
	mu         sync.RWMutex
	cur        io.ReadWriter
	reopen     func(prev io.ReadWriter) (io.ReadWriter, error)
	noReopener error
}

// configure installs the starting descriptor and the reopener. Called from
// Run before either copy goroutine starts.
func (p *swappablePort) configure(cur io.ReadWriter, reopen func(io.ReadWriter) (io.ReadWriter, error), noReopener error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cur = cur
	p.reopen = reopen
	p.noReopener = noReopener
}

// current returns the descriptor in use, or nil before configure has run.
func (p *swappablePort) current() io.ReadWriter {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cur
}

// fault swaps in a replacement for prev. If prev is no longer current
// (because the other goroutine already reopened) it is a no-op returning nil
// — the caller should retry with current().
func (p *swappablePort) fault(prev io.ReadWriter) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cur != prev {
		return nil
	}
	if p.reopen == nil {
		return p.noReopener
	}
	next, err := p.reopen(prev)
	if err != nil {
		return fmt.Errorf("%w: %v", p.noReopener, err)
	}
	p.cur = next
	return nil
}

// currentModem returns the modem port currently in use, falling back to the
// configured Modem so callers that run before Run (ModemFd, and tests that
// never start the splice) still see a port.
func (s *Splice) currentModem() io.ReadWriter {
	if rw := s.modemPort.current(); rw != nil {
		return rw
	}
	return s.Modem
}

// ModemFd reports the file descriptor of the modem port currently in use.
// The pairing runner writes via the raw fd, so it must re-read this after a
// reopen rather than caching the number: a stale fd may have been closed, or
// worse, reused by an unrelated open. ok is false when the port does not
// expose an fd (a test double).
func (s *Splice) ModemFd() (int, bool) {
	f, ok := s.currentModem().(interface{ Fd() uintptr })
	if !ok {
		return 0, false
	}
	return int(f.Fd()), true
}

// faultModem swaps in a fresh modem port after a hangup.
func (s *Splice) faultModem(prev io.ReadWriter) error { return s.modemPort.fault(prev) }

// faultHost swaps in a fresh Host master after an EIO.
func (s *Splice) faultHost(prev io.ReadWriter) error { return s.hostPort.fault(prev) }

func (s *Splice) copyOne(ctx context.Context, name string, dir FrameDirection) error {
	buf := make([]byte, s.BufSize)
	// reopens counts modem hangups recovered without a byte read in between,
	// so a port that hangs up again on every fresh fd faults instead of
	// looping. Any successful read clears it.
	reopens := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var (
			r        io.Reader
			curHost  io.ReadWriter
			curModem io.ReadWriter
		)
		switch dir {
		case DirToModem:
			// host→modem is paused for the duration of a pairing primitive so
			// host traffic can't interleave with the runner's writes.
			if !s.gate.wait(ctx) {
				return ctx.Err()
			}
			curHost = s.currentHost()
			r = curHost
		case DirToHost:
			// The modem reader is NEVER paused — it is the single owner of the
			// modem fd. During pairing it redirects to the sink (below).
			curModem = s.currentModem()
			r = curModem
		default:
			return fmt.Errorf("%s: unknown direction %d", name, dir)
		}

		n, rerr := r.Read(buf)
		if n > 0 {
			// The port is yielding data, so any earlier hangup is behind us.
			reopens = 0
		}

		// During pairing the single modem reader hands bytes to the pairing
		// runner via sink instead of forwarding to the host; host→modem bytes
		// during pairing are dropped so they can't corrupt the exchange.
		// Read errors still fall through to the shared handler below.
		pairing, sink := s.pairingState()
		if pairing {
			if dir == DirToHost && n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				s.Tap.PublishOn(tap.IfaceInject, byte(dir), chunk, time.Now())
				select {
				case sink <- chunk:
				default: // sink full or already dropped by EndPairing
				}
			}
		} else if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			ts := time.Now()

			// Ask the hook first so we know which iface to publish on.
			action := s.Hook.OnChunk(dir, chunk)

			iface := tap.IfaceWire
			if action.Mine {
				iface = tap.IfaceInject
			}
			s.Tap.PublishOn(iface, byte(dir), chunk, ts)

			if !action.Drop {
				out := chunk
				if action.Altered != nil {
					out = action.Altered
				}
				if err := s.write(dir, out); err != nil {
					return fmt.Errorf("%s write: %w", name, err)
				}
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				// Shutdown closes both fds, so an EOF racing cancellation
				// is clean whichever direction sees it first.
				if ctx.Err() != nil {
					return ctx.Err()
				}
				// host→modem: the host process let go of the pty slave.
				// The modem link is untouched, so this is a clean end.
				if dir == DirToModem {
					slog.Info("splice copy EOF, host detached", "dir", name)
					return nil
				}
				// modem→host: this goroutine is the SINGLE owner of the
				// modem fd, and a hung-up tty returns EOF forever. Ending
				// here would leave the proxy running write-only — injected
				// polls still reach the radio but nothing is ever read
				// back, the watchdog reads that silence as a wedged module
				// and reset-storms it, and Run stays parked in wg.Wait()
				// on the surviving goroutine so the process never exits.
				// Replace the fd and carry on.
				reopens++
				if reopens > maxModemReopens {
					return fmt.Errorf("%s read: %w: hung up %d times without a read", name, ErrModemFault, reopens-1)
				}
				if err := s.faultModem(curModem); err != nil {
					return fmt.Errorf("%s read: %w", name, err)
				}
				slog.Warn("modem fd hung up, reopened", "dir", name, "attempt", reopens)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(modemReopenSettle):
				}
				continue
			}
			// O_NONBLOCK pty master returns EAGAIN when there is no
			// data to read. Sleep briefly to avoid busy-spinning and
			// retry rather than treating this as a fault.
			if isEAGAIN(rerr) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(nonblockReadIdle):
				}
				continue
			}
			// Host-side EIO: try to recover by swapping the pty master.
			// Gated on ctx.Err() == nil so a shutdown-time EIO doesn't
			// trigger a wasted reopen.
			if dir == DirToModem && ctx.Err() == nil && errors.Is(rerr, syscall.EIO) {
				if err := s.faultHost(curHost); err != nil {
					return fmt.Errorf("%s read: %w", name, err)
				}
				slog.Warn("splice host pty reopened after EIO", "dir", name)
				continue
			}
			return fmt.Errorf("%s read: %w", name, rerr)
		}
	}
}

// write routes a chunk to the appropriate side. Modem writes are
// serialised via modemMu so InjectToModem can run from a hook
// goroutine without interleaving with the host→modem copy.
//
// On a Host-side EIO the master is replaced via HostReopener and the
// write is retried once against the new master. EAGAIN on a host write
// (the pty master's kernel buffer is full because the slave isn't being
// drained) is treated as "drop this chunk" so the modem→host goroutine
// never blocks the modem reader: a blocked host write here would
// back-pressure modem reads and starve any pairing reply waiting on the
// sink. Drops are counted and logged once per logEveryDrops occurrences.
func (s *Splice) write(dir FrameDirection, p []byte) error {
	switch dir {
	case DirToModem:
		// Resolve the port BEFORE taking modemMu, never while holding it.
		// A reopen runs with the port lock held and then closes the old
		// descriptor under modemMu, so acquiring the two in the other order
		// here would deadlock. Holding modemMu across the write is what
		// stops that close from freeing the fd number mid-write; a port
		// that goes stale between the two lines is merely a hung-up tty,
		// which fails the write harmlessly.
		port := s.currentModem()
		s.modemMu.Lock()
		_, err := port.Write(p)
		s.modemMu.Unlock()
		return err
	case DirToHost:
		// EAGAIN-drop is safe ONLY while no consumer reads the pty slave.
		// If a slave reader is ever attached the dropped chunk is a byte
		// that reader expected; before that lands, replace this with a
		// single bounded retry (e.g. select{<-ctx.Done(): <-time.After(2*ms):
		// retry once}) before counting the drop. The tap publish at the
		// caller fires BEFORE this drop, so the pcap stream still shows
		// the chunk for analyser visibility.
		host := s.currentHost()
		_, err := host.Write(p)
		if err != nil && isEAGAIN(err) {
			n := atomic.AddUint64(&s.hostDropCount, 1)
			if n == 1 || n%logEveryDrops == 0 {
				slog.Warn("modem→host pty master EAGAIN, dropped bytes (slave likely unread)", "bytes", len(p), "total_drops", n)
			}
			return nil
		}
		if err != nil && errors.Is(err, syscall.EIO) {
			if ferr := s.faultHost(host); ferr != nil {
				return ferr
			}
			slog.Warn("modem→host host pty reopened after EIO write")
			host = s.currentHost()
			_, err = host.Write(p)
			if err != nil && isEAGAIN(err) {
				n := atomic.AddUint64(&s.hostDropCount, 1)
				if n == 1 || n%logEveryDrops == 0 {
					slog.Warn("modem→host pty master EAGAIN after reopen, dropped bytes", "bytes", len(p), "total_drops", n)
				}
				return nil
			}
		}
		return err
	}
	return fmt.Errorf("unknown direction %d", dir)
}

// ModemWriteMu returns the single mutex that serialises every writer
// to the modem fd. The pairing runner takes this around its
// flush+settle+drain+write sequence so a concurrent splice DirToModem
// write or hook InjectToModem can't slip a frame in between the flush
// and the write (which would corrupt the modem's view of the next
// config-op). Exposed as a getter (rather than promoting modemMu to an
// exported field) so the splice's mutex remains the canonical one.
func (s *Splice) ModemWriteMu() *sync.Mutex {
	return &s.modemMu
}

// InjectToModem writes raw straight to the modem and publishes a
// matching DirToModem block on the IfaceInject interface so
// Wireshark distinguishes our queries from any host-originated traffic.
// The host never sees the bytes — they don't go through the host pty.
// The hook is responsible for filtering the response (modem ack + L1
// reply) out of the modem→host stream so the host doesn't see those
// either, and for marking those response chunks as Mine so they also
// land on IfaceInject in the tap.
func (s *Splice) InjectToModem(raw []byte) error {
	chunk := append([]byte(nil), raw...)
	s.Tap.PublishOn(tap.IfaceInject, byte(DirToModem), chunk, time.Now())
	return s.write(DirToModem, chunk)
}

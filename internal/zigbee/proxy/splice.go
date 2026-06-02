package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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

	// modemMu serialises writes to Modem so a hook's InjectToModem
	// can't interleave bytes with the host→modem copy goroutine. The
	// pairing runner takes this same mutex around its flush+sleep+write
	// sequences (see ModemWriteMu) so there is one, and only one,
	// modem-fd write serialiser across every writer (splice DirToModem,
	// hook InjectToModem, busmgr inject, pairing runner). Any new modem
	// writer MUST take this mutex.
	modemMu sync.Mutex

	// hostMu guards host. Held in write mode during a reopen; in
	// read mode whenever a goroutine fetches the current master.
	hostMu sync.RWMutex
	host   io.ReadWriter

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
	s.hostMu.Lock()
	s.host = s.Host
	s.hostMu.Unlock()

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

// currentHost returns the master fd currently in use. Acquired under
// hostMu so a reopen happening concurrently is observed atomically.
func (s *Splice) currentHost() io.ReadWriter {
	s.hostMu.RLock()
	defer s.hostMu.RUnlock()
	return s.host
}

// faultHost swaps in a fresh Host master after an EIO. If prev is no
// longer the current host (because the other goroutine already
// reopened) it's a no-op and returns nil — the caller should retry
// with currentHost().
func (s *Splice) faultHost(prev io.ReadWriter) error {
	s.hostMu.Lock()
	defer s.hostMu.Unlock()
	if s.host != prev {
		return nil
	}
	if s.HostReopener == nil {
		return ErrHostFault
	}
	next, err := s.HostReopener(prev)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHostFault, err)
	}
	s.host = next
	return nil
}

func (s *Splice) copyOne(ctx context.Context, name string, dir FrameDirection) error {
	buf := make([]byte, s.BufSize)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var (
			r       io.Reader
			curHost io.ReadWriter
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
			r = s.Modem
		default:
			return fmt.Errorf("%s: unknown direction %d", name, dir)
		}

		n, rerr := r.Read(buf)

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
				log.Printf("%s: EOF", name)
				return nil
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
				log.Printf("%s: host pty reopened after EIO", name)
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
		s.modemMu.Lock()
		_, err := s.Modem.Write(p)
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
				log.Printf("modem→host: pty master EAGAIN; dropped %d byte(s) (total drops=%d, slave likely unread)", len(p), n)
			}
			return nil
		}
		if err != nil && errors.Is(err, syscall.EIO) {
			if ferr := s.faultHost(host); ferr != nil {
				return ferr
			}
			log.Printf("modem→host: host pty reopened after EIO write")
			host = s.currentHost()
			_, err = host.Write(p)
			if err != nil && isEAGAIN(err) {
				n := atomic.AddUint64(&s.hostDropCount, 1)
				if n == 1 || n%logEveryDrops == 0 {
					log.Printf("modem→host: pty master EAGAIN after reopen; dropped %d byte(s) (total drops=%d)", len(p), n)
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

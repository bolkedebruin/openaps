package busmgr

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/internal/zigbee/inventory"
	"github.com/bolkedebruin/openaps/wire"
)

// infoQueryL2 returns the canonical 0xDC info-query L2 body. Tests use
// this helper so they stay aligned with the validateFrame contract.
func infoQueryL2() []byte {
	return codec.OutboundInfoQueryL2()
}

// testServer is a minimal UDS listener that records every wire.Envelope
// frame it reads from each accepted connection.
type testServer struct {
	t      *testing.T
	ln     net.Listener
	frames chan *wire.Envelope
	wg     sync.WaitGroup

	mu     sync.Mutex
	conns  []net.Conn
	closed bool
}

func newTestServer(t *testing.T, socketPath string) *testServer {
	t.Helper()
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix %s: %v", socketPath, err)
	}
	s := &testServer{
		t:      t,
		ln:     ln,
		frames: make(chan *wire.Envelope, 64),
	}
	s.wg.Add(1)
	go s.acceptLoop()
	return s
}

func (s *testServer) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		s.conns = append(s.conns, conn)
		s.mu.Unlock()
		s.wg.Add(1)
		go s.handle(conn)
	}
}

func (s *testServer) handle(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()
	for {
		var env wire.Envelope
		if err := wire.ReadFrame(conn, &env); err != nil {
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "use of closed") {
				return
			}
			return
		}
		select {
		case s.frames <- &env:
		default:
		}
	}
}

// sendToClient writes env on every currently-open accepted connection.
// Used by tests that need to push downstream frames (e.g. Envelope_Send).
func (s *testServer) sendToClient(env *wire.Envelope) error {
	s.mu.Lock()
	conns := append([]net.Conn(nil), s.conns...)
	s.mu.Unlock()
	for _, c := range conns {
		if err := wire.WriteFrame(c, env); err != nil {
			return err
		}
	}
	return nil
}

// waitForConn polls until at least one accepted conn exists.
func (s *testServer) waitForConn(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		n := len(s.conns)
		s.mu.Unlock()
		if n > 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func (s *testServer) close() {
	s.mu.Lock()
	s.closed = true
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()
	_ = s.ln.Close()
	// Force every open connection closed so the handle goroutines
	// unblock from ReadFrame.
	for _, c := range conns {
		_ = c.Close()
	}
	s.wg.Wait()
}

// captureLog redirects the standard log package output to a buffer for
// the duration of the test. Returns the buffer plus a restore func
// (already wired into t.Cleanup).
func captureLog(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	prev := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(buf)
	t.Cleanup(func() {
		log.SetOutput(prev)
		log.SetFlags(prevFlags)
	})
	return buf
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// tempSocketPath builds a short unix-socket path. UDS path limits are
// ~104 bytes on macOS, and t.TempDir() encodes the full test name into
// the path which can blow past that for long test names. So we build
// our own short directory under os.TempDir() and clean it up.
func tempSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "bm")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

// waitFor polls cond every 10ms up to d. Returns true on success.
func waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

func TestClient_HelloOnConnect(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	c := New(sock, "apsystems-stock-zb", "v9.9.9")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	select {
	case env := <-srv.frames:
		h := env.GetHello()
		if h == nil {
			t.Fatalf("first frame has no Hello body: %+v", env)
		}
		if h.GetBackend() != "apsystems-stock-zb" {
			t.Errorf("Backend = %q, want %q", h.GetBackend(), "apsystems-stock-zb")
		}
		if h.GetVersion() != "v9.9.9" {
			t.Errorf("Version = %q, want %q", h.GetVersion(), "v9.9.9")
		}
		if h.GetStartedAtMs() == 0 {
			t.Errorf("StartedAtMs = 0, want non-zero")
		}
		if caps := h.GetBusCaps(); len(caps) == 0 || caps[0] != "apsystems-stock-zb" {
			t.Errorf("BusCaps = %v, want [apsystems-stock-zb]", caps)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("did not receive Hello frame within 2s")
	}
}

func TestClient_TelemetryAfterHello(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	c := New(sock, "bk", "v0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	// Wait for Hello.
	var first *wire.Envelope
	select {
	case first = <-srv.frames:
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello")
	}
	if first.GetHello() == nil {
		t.Fatalf("first frame has no Hello body: %+v", first)
	}

	// Wait for connection state to be visible (sentTotal > 0 means
	// Hello was written; reader has it).
	if !waitFor(time.Second, func() bool { return c.Stats().Connected }) {
		t.Fatalf("client never marked Connected")
	}

	c.Enqueue(&wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
		TsMs:      time.Now().UnixMilli(),
		ShortAddr: 0xBEEF,
		PeerUid:   "112233445566",
		Cmd:       0xBB,
		Model:     "DS3",
	}}})

	select {
	case env := <-srv.frames:
		tl := env.GetTelemetry()
		if tl == nil {
			t.Fatalf("second frame has no Telemetry body: %+v", env)
		}
		if tl.GetShortAddr() != 0xBEEF {
			t.Errorf("Telemetry payload mismatch: %+v", tl)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("did not receive Telemetry frame within 2s")
	}
}

func TestClient_DropsWhenDisconnected(t *testing.T) {
	t.Parallel()

	captureLog(t)

	// Path that nothing is listening on.
	sock := filepath.Join(t.TempDir(), "nope.sock")
	c := New(sock, "bk", "v0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	// Enqueue well past queue capacity so drops are guaranteed.
	const overflow = 44
	enqueueN := outboundCap + overflow
	for i := 0; i < enqueueN; i++ {
		c.Enqueue(&wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{}}})
	}

	// Give the dialer a beat to fail and confirm it's not connected.
	time.Sleep(100 * time.Millisecond)

	st := c.Stats()
	if st.Connected {
		t.Errorf("Stats.Connected = true, want false (no server)")
	}
	if st.SentTotal != 0 {
		t.Errorf("Stats.SentTotal = %d, want 0", st.SentTotal)
	}
	// Conservative floor — actual drops are exactly `overflow` since
	// nothing drains the queue. The -4 margin tolerates a brief window
	// where the writer goroutine could pull a frame before failing.
	wantFloor := uint64(overflow - 4)
	if st.DroppedTotal < wantFloor {
		t.Errorf("Stats.DroppedTotal = %d, want >= %d (outboundCap=%d, enqueued=%d)",
			st.DroppedTotal, wantFloor, outboundCap, enqueueN)
	}
}

func TestClient_DropLogRateLimited(t *testing.T) {
	t.Parallel()

	buf := captureLog(t)

	sock := filepath.Join(t.TempDir(), "nope.sock")
	c := New(sock, "bk", "v0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	// Hammer enqueue over ~50ms.
	start := time.Now()
	for i := 0; i < 1000; i++ {
		c.Enqueue(&wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{}}})
		if time.Since(start) > 50*time.Millisecond {
			break
		}
	}

	// Let any pending log output drain.
	time.Sleep(50 * time.Millisecond)

	logOut := buf.String()
	count := strings.Count(logOut, "(queue full)")
	if count > 2 {
		t.Errorf("queue-full log appeared %d times, want <= 2\n--- captured log ---\n%s", count, logOut)
	}
}

func TestClient_ReconnectAfterServerClose(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)

	buf := captureLog(t)

	c := New(sock, "bk", "v1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	// First Hello.
	select {
	case env := <-srv.frames:
		if env.GetHello() == nil {
			t.Fatalf("first frame has no Hello body: %+v", env)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no first Hello")
	}

	// Tear the server down. macOS will refuse a fresh bind on the same
	// path without an explicit unlink, and even then can return EINVAL
	// briefly — so we just close the current connection (the client's
	// session ends, dialer retries) and rebind on a retry loop.
	srv.close()
	// Drain anything queued.
	for {
		select {
		case <-srv.frames:
			continue
		default:
		}
		break
	}

	var srv2 *testServer
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_ = os.Remove(sock)
		ln, err := net.Listen("unix", sock)
		if err == nil {
			srv2 = &testServer{
				t:      t,
				ln:     ln,
				frames: make(chan *wire.Envelope, 64),
			}
			srv2.wg.Add(1)
			go srv2.acceptLoop()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if srv2 == nil {
		t.Fatalf("could not rebind socket %s within 2s", sock)
	}
	defer srv2.close()

	// Drive enqueues at ~50ms cadence so the client's session goroutine
	// notices the peer-closed conn via a failed WriteFrame and bails to
	// re-dial. Without traffic, session sits in a select forever.
	stopFeed := make(chan struct{})
	go func() {
		tk := time.NewTicker(50 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-stopFeed:
				return
			case <-tk.C:
				c.Enqueue(&wire.Envelope{Body: &wire.Envelope_Telemetry{
					Telemetry: &wire.Telemetry{TsMs: time.Now().UnixMilli()},
				}})
			}
		}
	}()
	defer close(stopFeed)

	// Initial backoff is 1s. With write-driven detection of peer close,
	// the client should reconnect within ~1.2s. 5s is generous.
	select {
	case env := <-srv2.frames:
		// First frame after reconnect must be Hello.
		if env.GetHello() == nil {
			t.Fatalf("post-reconnect first frame has no Hello body: %+v", env)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("client did not reconnect within 5s\n--- log ---\n%s", buf.String())
	}
}

func TestClient_CleanShutdown(t *testing.T) {
	t.Parallel()

	captureLog(t)

	sock := filepath.Join(t.TempDir(), "nope.sock")

	// Settle goroutines before measuring.
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	c := New(sock, "bk", "v0")
	ctx, cancel := context.WithCancel(context.Background())
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Let the dialer goroutine actually start.
	time.Sleep(50 * time.Millisecond)

	// Stop should return within 1s.
	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		cancel()
		t.Fatalf("Stop did not return within 1s")
	}
	cancel()

	// Settle and check no orphan goroutines remain (tolerate ±2).
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after-before > 2 {
		t.Errorf("goroutine leak: before=%d after=%d (delta=%d)", before, after, after-before)
	}
}

func TestClient_CleanShutdownViaCtx(t *testing.T) {
	t.Parallel()

	captureLog(t)

	sock := filepath.Join(t.TempDir(), "nope.sock")
	c := New(sock, "bk", "v0")
	ctx, cancel := context.WithCancel(context.Background())
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	cancel()

	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("Stop did not return within 1s after ctx cancel")
	}
}

func TestClient_StopBeforeStart(t *testing.T) {
	t.Parallel()

	c := New("/nonexistent.sock", "bk", "v0")
	// Must not panic.
	c.Stop()
}

// recvCallback captures one or more frames passed to OnSend.
type recvCallback struct {
	mu     sync.Mutex
	frames [][]byte
	errOn  int // index at which to return an error (-1 disables)
	doneCh chan struct{}
	want   int
}

func newRecvCallback(want int) *recvCallback {
	return &recvCallback{doneCh: make(chan struct{}, 1), want: want, errOn: -1}
}

func (r *recvCallback) cb(frame []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frames = append(r.frames, append([]byte(nil), frame...))
	if r.errOn >= 0 && len(r.frames)-1 == r.errOn {
		// Don't signal done on error-injected frames.
		return errors.New("forced cb error")
	}
	if len(r.frames) >= r.want {
		select {
		case r.doneCh <- struct{}{}:
		default:
		}
	}
	return nil
}

func (r *recvCallback) wait(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case <-r.doneCh:
	case <-time.After(d):
		r.mu.Lock()
		got := len(r.frames)
		r.mu.Unlock()
		t.Fatalf("OnSend did not fire %d times within %s (got %d)", r.want, d, got)
	}
}

func (r *recvCallback) snapshot() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]byte, len(r.frames))
	for i, f := range r.frames {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

func TestClient_OnSend_DispatchesFrame(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	rec := newRecvCallback(1)
	c := New(sock, "bk", "v0")
	c.OnSend = rec.cb
	c.Inventory = inventory.NewMap()
	c.Inventory.Record("999900000003", 0xC459)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	// Wait for Hello so we know the conn is up.
	select {
	case env := <-srv.frames:
		if env.GetHello() == nil {
			t.Fatalf("first frame has no Hello body: %+v", env)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello within 2s")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("server has no accepted conn after Hello")
	}

	l2 := infoQueryL2()
	wantL1 := BuildL1Frame(0xC459, l2)
	send := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid:    "999900000003",
		Frame:      l2,
		DeadlineMs: 1500,
	}}}
	if err := srv.sendToClient(send); err != nil {
		t.Fatalf("sendToClient: %v", err)
	}

	rec.wait(t, 2*time.Second)
	got := rec.snapshot()
	if len(got) != 1 || !bytes.Equal(got[0], wantL1) {
		t.Fatalf("OnSend frame mismatch:\n  got  %x\n  want %x", got, wantL1)
	}
	if st := c.Stats(); st.SendOkTotal != 1 || st.SendFailTotal != 0 {
		t.Fatalf("Stats unexpected: %+v", st)
	}
}

func TestClient_OnSend_MultipleInOrder(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	rec := newRecvCallback(3)
	c := New(sock, "bk", "v0")
	c.OnSend = rec.cb
	c.Inventory = inventory.NewMap()
	uids := []string{"000000000001", "000000000002", "000000000003"}
	for i, uid := range uids {
		c.Inventory.Record(uid, uint16(i+1))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	// Wait for Hello.
	select {
	case <-srv.frames:
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("no accepted conn")
	}

	for _, uid := range uids {
		send := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
			PeerUid: uid,
			Frame:   infoQueryL2(),
		}}}
		if err := srv.sendToClient(send); err != nil {
			t.Fatalf("sendToClient: %v", err)
		}
	}

	rec.wait(t, 2*time.Second)
	got := rec.snapshot()
	if len(got) != 3 {
		t.Fatalf("want 3 frames, got %d", len(got))
	}
	// SA low byte sits at frame[6] in the outer envelope.
	for i, f := range got {
		if f[6] != byte(i+1) {
			t.Fatalf("frame %d out of order: %x", i, f)
		}
	}
}

func TestClient_OnSend_UnknownBodyIgnored(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	rec := newRecvCallback(1)
	c := New(sock, "bk", "v0")
	c.OnSend = rec.cb
	c.Inventory = inventory.NewMap()
	c.Inventory.Record("000000000001", 0x0001)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	select {
	case <-srv.frames:
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("no accepted conn")
	}

	// Send a Hello echo — unknown to our handler.
	echo := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{Backend: "echo"}}}
	if err := srv.sendToClient(echo); err != nil {
		t.Fatalf("sendToClient: %v", err)
	}

	// Then a real Send — should still dispatch.
	send := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid: "000000000001", Frame: infoQueryL2(),
	}}}
	if err := srv.sendToClient(send); err != nil {
		t.Fatalf("sendToClient: %v", err)
	}

	rec.wait(t, 2*time.Second)
	if got := rec.snapshot(); len(got) != 1 {
		t.Fatalf("want 1 dispatched, got %d", len(got))
	}
	// Conn must still be open: client should be able to write telemetry.
	c.Enqueue(&wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{ShortAddr: 0x1234}}})
	select {
	case env := <-srv.frames:
		if env.GetTelemetry() == nil || env.GetTelemetry().GetShortAddr() != 0x1234 {
			t.Fatalf("expected telemetry, got %+v", env)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("conn closed after unknown body")
	}
}

func TestClient_OnSend_CallbackErrorDoesNotCloseConn(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	buf := captureLog(t)

	c := New(sock, "bk", "v0")
	c.OnSend = func(frame []byte) error { return errors.New("simulated") }
	c.Inventory = inventory.NewMap()
	c.Inventory.Record("000000000000", 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	select {
	case <-srv.frames:
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("no accepted conn")
	}

	send := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid: "000000000000", Frame: infoQueryL2(),
	}}}
	if err := srv.sendToClient(send); err != nil {
		t.Fatalf("sendToClient: %v", err)
	}

	// Conn must remain open.
	if !waitFor(2*time.Second, func() bool { return c.Stats().SendFailTotal >= 1 }) {
		t.Fatalf("SendFailTotal never incremented; log=%s", buf.String())
	}

	c.Enqueue(&wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{ShortAddr: 0xCAFE}}})
	select {
	case env := <-srv.frames:
		if env.GetTelemetry() == nil {
			t.Fatalf("expected telemetry, got %+v", env)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("conn closed after OnSend error")
	}
}

func TestClient_OnSend_NilCallbackCountsFail(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	c := New(sock, "bk", "v0")
	// OnSend left nil — a Send arrival must be counted as a failure
	// (and the conn must stay open).
	c.Inventory = inventory.NewMap()
	c.Inventory.Record("000000000000", 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	select {
	case <-srv.frames:
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("no accepted conn")
	}

	send := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid: "000000000000", Frame: infoQueryL2(),
	}}}
	if err := srv.sendToClient(send); err != nil {
		t.Fatalf("sendToClient: %v", err)
	}

	if !waitFor(2*time.Second, func() bool { return c.Stats().SendFailTotal >= 1 }) {
		t.Fatalf("SendFailTotal never incremented: %+v", c.Stats())
	}

	// Conn must remain open: enqueuing telemetry should still get through.
	c.Enqueue(&wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{ShortAddr: 0xCAFE}}})
	select {
	case env := <-srv.frames:
		if env.GetTelemetry() == nil {
			t.Fatalf("expected telemetry, got %+v", env)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("conn closed after nil OnSend dispatch")
	}
}

func TestClient_CtxCancelDuringSendWait(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	c := New(sock, "bk", "v0")
	c.OnSend = func([]byte) error { return nil }
	ctx, cancel := context.WithCancel(context.Background())
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	select {
	case <-srv.frames:
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("no accepted conn")
	}

	// Reader is parked in ReadFrame. Cancel and verify clean stop.
	cancel()

	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Stop did not return within 2s after cancel")
	}
}

func TestClient_OnSend_DispatchesBroadcastFrame(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	rec := newRecvCallback(1)
	c := New(sock, "bk", "v0")
	c.OnSend = rec.cb

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	select {
	case env := <-srv.frames:
		if env.GetHello() == nil {
			t.Fatalf("first frame has no Hello body: %+v", env)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("no accepted conn")
	}

	// Broadcast set-power L2 (DSP-new 80% broadcast).
	l2 := []byte{0xFB, 0xFB, 0x06, 0x2C, 0x8C, 0x02, 0x09, 0x06, 0x00, 0x00, 0xCF, 0xFE, 0xFE}
	wantL1 := BuildL1Frame(0, l2)
	bc := &wire.Envelope{Body: &wire.Envelope_Broadcast{Broadcast: &wire.Broadcast{
		Frame:      l2,
		DeadlineMs: 0,
	}}}
	if err := srv.sendToClient(bc); err != nil {
		t.Fatalf("sendToClient: %v", err)
	}

	rec.wait(t, 2*time.Second)
	got := rec.snapshot()
	if len(got) != 1 || !bytes.Equal(got[0], wantL1) {
		t.Fatalf("Broadcast OnSend frame mismatch:\n  got  %x\n  want %x", got, wantL1)
	}
	if st := c.Stats(); st.SendOkTotal != 1 || st.SendFailTotal != 0 {
		t.Fatalf("Stats unexpected: %+v", st)
	}
}

// TestValidateFrame_BroadcastProtection verifies the (cmd, sub) gate for
// broadcast protection frames.
//
// DS3 (0xAB) and QS1A (0x2C) share their broadcast opcode with set-power.
// The sub-byte at L2[4] distinguishes them. With broadcastProtoValidated
// true, validateFrame must ACCEPT protection sub-bytes on those opcodes
// (DS3 on-wire validated 2026-05-20; QS1A live test pending). Set-power
// sub-bytes and unicast protection frames are also accepted.
func TestValidateFrame_BroadcastProtection(t *testing.T) {
	t.Parallel()

	// --- broadcast protection frames must be ACCEPTED ---

	// DS3 broadcast CB 50.3 Hz: opcode 0xAB, sub 0x2B (protection).
	ds3ProtFrames, err := codec.EncodeSetProtectionBroadcast(codec.ModelDS3, "Over_frequency_Watt_Low_set", 50.3)
	if err != nil {
		t.Fatalf("DS3 broadcast encode: %v", err)
	}
	if len(ds3ProtFrames) != 1 {
		t.Fatalf("DS3: want 1 frame, got %d", len(ds3ProtFrames))
	}
	// Confirm the codec produced the expected (cmd, sub).
	if ds3ProtFrames[0][3] != 0xAB {
		t.Fatalf("DS3: opcode = 0x%02X, want 0xAB", ds3ProtFrames[0][3])
	}
	if ds3ProtFrames[0][4] != 0x2B {
		t.Fatalf("DS3: sub = 0x%02X, want 0x2B (CB protection)", ds3ProtFrames[0][4])
	}
	if err := validateFrame(ds3ProtFrames[0]); err != nil {
		t.Errorf("DS3 broadcast protection (0xAB/0x2B) rejected (gate is open): %v\n  frame = % X", err, ds3ProtFrames[0])
	}

	// QS1A broadcast DH 47.0 Hz: opcode 0x2C, sub 0xA1 (protection).
	qs1ProtFrames, err := codec.EncodeSetProtectionBroadcast(codec.ModelQS1A, "Under_Frequency_Watt_Low_set", 47.0)
	if err != nil {
		t.Fatalf("QS1A broadcast encode: %v", err)
	}
	if len(qs1ProtFrames) != 1 {
		t.Fatalf("QS1A: want 1 frame, got %d", len(qs1ProtFrames))
	}
	if qs1ProtFrames[0][3] != 0x2C {
		t.Fatalf("QS1A: opcode = 0x%02X, want 0x2C", qs1ProtFrames[0][3])
	}
	if qs1ProtFrames[0][4] != 0xA1 {
		t.Fatalf("QS1A: sub = 0x%02X, want 0xA1 (DH protection)", qs1ProtFrames[0][4])
	}
	if err := validateFrame(qs1ProtFrames[0]); err != nil {
		t.Errorf("QS1A broadcast protection (0x2C/0xA1) rejected (gate is open): %v\n  frame = % X", err, qs1ProtFrames[0])
	}

	// --- set-power broadcast frames must be ACCEPTED ---

	// DS3 set-power broadcast at some watt limit: opcode 0xAB, sub 0x27.
	ds3PwrFrames, err := codec.EncodeSetPower(codec.ModelDS3, 300, true)
	if err != nil {
		t.Fatalf("DS3 set-power encode: %v", err)
	}
	if ds3PwrFrames[3] != 0xAB {
		t.Fatalf("DS3 set-power: opcode = 0x%02X, want 0xAB", ds3PwrFrames[3])
	}
	if ds3PwrFrames[4] != codec.SubMaxPowerDS3 {
		t.Fatalf("DS3 set-power: sub = 0x%02X, want 0x%02X", ds3PwrFrames[4], codec.SubMaxPowerDS3)
	}
	if err := validateFrame(ds3PwrFrames); err != nil {
		t.Errorf("DS3 set-power broadcast (0xAB/0x27) rejected: %v\n  frame = % X", err, ds3PwrFrames)
	}

	// QS1A set-power broadcast: opcode 0x2C, sub 0x8C.
	qs1PwrFrames, err := codec.EncodeSetPower(codec.ModelQS1A, 300, true)
	if err != nil {
		t.Fatalf("QS1A set-power encode: %v", err)
	}
	if qs1PwrFrames[3] != 0x2C {
		t.Fatalf("QS1A set-power: opcode = 0x%02X, want 0x2C", qs1PwrFrames[3])
	}
	if qs1PwrFrames[4] != codec.SubMaxPowerQS1 {
		t.Fatalf("QS1A set-power: sub = 0x%02X, want 0x%02X", qs1PwrFrames[4], codec.SubMaxPowerQS1)
	}
	if err := validateFrame(qs1PwrFrames); err != nil {
		t.Errorf("QS1A set-power broadcast (0x2C/0x8C) rejected: %v\n  frame = % X", err, qs1PwrFrames)
	}

	// --- unicast protection frames must be ACCEPTED (gate does not apply) ---

	// DS3 unicast CB 50.3 Hz: opcode 0xAA, sub 0x2B.
	ds3UniAll, err := codec.EncodeSetProtection(codec.ModelDS3, "Over_frequency_Watt_Low_set", 50.3)
	if err != nil {
		t.Fatalf("DS3 unicast protection encode: %v", err)
	}
	if len(ds3UniAll) != 1 {
		t.Fatalf("DS3 unicast: want 1 frame, got %d", len(ds3UniAll))
	}
	ds3UniFrame := ds3UniAll[0]
	if ds3UniFrame[3] != 0xAA {
		t.Fatalf("DS3 unicast: opcode = 0x%02X, want 0xAA", ds3UniFrame[3])
	}
	if ds3UniFrame[4] != 0x2B {
		t.Fatalf("DS3 unicast: sub = 0x%02X, want 0x2B", ds3UniFrame[4])
	}
	if err := validateFrame(ds3UniFrame); err != nil {
		t.Errorf("DS3 unicast protection (0xAA/0x2B) rejected (gate must not apply to unicast): %v\n  frame = % X", err, ds3UniFrame)
	}

	// QS1A unicast DH 47.0 Hz: opcode 0x1C, sub 0xA1.
	qs1UniAll, err := codec.EncodeSetProtection(codec.ModelQS1A, "Under_Frequency_Watt_Low_set", 47.0)
	if err != nil {
		t.Fatalf("QS1A unicast protection encode: %v", err)
	}
	if len(qs1UniAll) != 1 {
		t.Fatalf("QS1A unicast: want 1 frame, got %d", len(qs1UniAll))
	}
	qs1UniFrame := qs1UniAll[0]
	if qs1UniFrame[3] != 0x1C {
		t.Fatalf("QS1A unicast: opcode = 0x%02X, want 0x1C", qs1UniFrame[3])
	}
	if qs1UniFrame[4] != 0xA1 {
		t.Fatalf("QS1A unicast: sub = 0x%02X, want 0xA1", qs1UniFrame[4])
	}
	if err := validateFrame(qs1UniFrame); err != nil {
		t.Errorf("QS1A unicast protection (0x1C/0xA1) rejected (gate must not apply to unicast): %v\n  frame = % X", err, qs1UniFrame)
	}

	// --- L1 framing sanity: broadcast SA=0 ---
	// BuildL1Frame(0, ...) must produce all-zero SA bytes. Check using a
	// set-power frame (which passes validateFrame) so the test remains valid
	// whether broadcastProtoValidated is true or false.
	wantL1 := BuildL1Frame(0, ds3PwrFrames)
	if wantL1[5] != 0x00 || wantL1[6] != 0x00 {
		t.Errorf("broadcast L1 SA bytes non-zero: hi=0x%02X lo=0x%02X", wantL1[5], wantL1[6])
	}
	if int(wantL1[14]) != len(ds3PwrFrames) {
		t.Errorf("L1 length byte = %d, want %d", wantL1[14], len(ds3PwrFrames))
	}
}

func TestValidateFrame(t *testing.T) {
	t.Parallel()
	good := infoQueryL2()
	if err := validateFrame(good); err != nil {
		t.Fatalf("min-valid L2 rejected: %v", err)
	}

	mutate := func(idx int, v byte) []byte {
		f := append([]byte(nil), good...)
		f[idx] = v
		return f
	}
	cases := []struct {
		name  string
		frame []byte
	}{
		{"empty", nil},
		{"too short", []byte{0xFB, 0xFB, 0x06}},
		{"too long", make([]byte, maxL2Len+1)},
		{"bad SOF", mutate(0, 0xAA)},
		{"bad inner-length", mutate(2, byte(good[2]+1))},
		{"bad EOF", mutate(len(good)-1, 0x00)},
		{"sum mismatch", mutate(len(good)-3, byte(good[len(good)-3]+1))},
		{"cmd not allowed", mutate(3, 0xFF)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := validateFrame(c.frame); err == nil {
				t.Fatalf("expected rejection for %s", c.name)
			}
		})
	}
}

func TestValidateFrame_OnOff(t *testing.T) {
	t.Parallel()
	// Unicast on/off is allow-listed — ecu-sunspec's Conn path uses it.
	for _, f := range [][]byte{codec.EncodeOnOff(true, false), codec.EncodeOnOff(false, false)} {
		if err := validateFrame(f); err != nil {
			t.Errorf("unicast on/off % X rejected: %v", f, err)
		}
	}
	// Broadcast on/off is intentionally NOT allow-listed (a broadcast OFF
	// is a fleet-wide shutdown).
	for _, f := range [][]byte{codec.EncodeOnOff(true, true), codec.EncodeOnOff(false, true)} {
		if err := validateFrame(f); err == nil {
			t.Errorf("broadcast on/off % X should be rejected by allow-list", f)
		}
	}
}

// TestValidateFrame_QS1YC600Trips confirms the QS1 slow-volt and grid-freq
// trips (yc600 builder: sub-as-cmd at frame[3]) pass the allow-list. These
// frames carry no 0x1C opcode, so each trip's sub-byte is its own L2 cmd.
func TestValidateFrame_QS1YC600Trips(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		param string
		value float64
	}{
		{"AQ under_voltage_stage_2", "under_voltage_stage_2", 196},
		{"AD over_voltage_slow", "over_voltage_slow", 264},
		{"AC under_voltage_stage_2_90", "under_voltage_stage_2_90", 196},
		{"AY over_voltage_slow_90", "over_voltage_slow_90", 264},
		{"AJ under_frequency_fast", "under_frequency_fast", 47},
		{"AK over_frequency_fast", "over_frequency_fast", 52},
		{"AE under_frequency_slow", "under_frequency_slow", 47.5},
		{"AF over_frequency_slow", "over_frequency_slow", 51.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			frames, err := codec.EncodeSetProtection(codec.ModelQS1A, c.param, c.value)
			if err != nil {
				t.Fatalf("encode %s: %v", c.param, err)
			}
			if err := validateFrame(frames[0]); err != nil {
				t.Errorf("yc600 trip %s % X rejected by allow-list: %v", c.param, frames[0], err)
			}
		})
	}
}

func TestClient_OnSend_RejectsInvalidFrame(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	c := New(sock, "bk", "v0")
	c.OnSend = func([]byte) error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	select {
	case <-srv.frames:
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("no accepted conn")
	}

	bad := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{Frame: []byte{0xAA, 0xBB}}}}
	if err := srv.sendToClient(bad); err != nil {
		t.Fatalf("sendToClient: %v", err)
	}

	if !waitFor(2*time.Second, func() bool { return c.Stats().SendFailTotal >= 1 }) {
		t.Fatalf("SendFailTotal never incremented: %+v", c.Stats())
	}
	if got := c.Stats().SendOkTotal; got != 0 {
		t.Fatalf("SendOkTotal: got %d want 0", got)
	}
}

// TestDispatchSend_ShortAddrDirect verifies that a Send with short_addr != 0
// injects to SA=short_addr without requiring an inventory entry. The
// inventory must NOT be mutated — the radio-learned UID↔SA map is
// authoritative and must only be written by inbound telemetry frames.
func TestDispatchSend_ShortAddrDirect(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	rec := newRecvCallback(1)
	c := New(sock, "bk", "v0")
	c.OnSend = rec.cb
	// Inventory is empty — no prior knowledge of the target SA.
	c.Inventory = inventory.NewMap()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	select {
	case env := <-srv.frames:
		if env.GetHello() == nil {
			t.Fatalf("first frame has no Hello body: %+v", env)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello within 2s")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("server has no accepted conn after Hello")
	}

	const targetSA = uint16(0xBEEF)
	const targetUID = "aabbccddeeff"
	l2 := infoQueryL2()
	wantL1 := BuildL1Frame(targetSA, l2)

	// Send with short_addr set; inventory has no entry for targetUID.
	send := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid:    targetUID,
		Frame:      l2,
		DeadlineMs: 1500,
		ShortAddr:  uint32(targetSA),
	}}}
	if err := srv.sendToClient(send); err != nil {
		t.Fatalf("sendToClient: %v", err)
	}

	rec.wait(t, 2*time.Second)
	got := rec.snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 frame, got %d", len(got))
	}
	if !bytes.Equal(got[0], wantL1) {
		t.Fatalf("OnSend frame mismatch:\n  got  %x\n  want %x", got[0], wantL1)
	}

	// Inventory must NOT have been updated from the controller Send.
	if _, ok := c.Inventory.LookupSA(targetUID); ok {
		t.Fatalf("inventory was mutated by a controller Send — must not be (security: prevents poisoning the radio-learned UID↔SA map)")
	}
}

// TestDispatchSend_ShortAddrDoesNotPoisonInventory asserts that sending
// with short_addr != 0 (inv-driver telemetry path) does not overwrite a
// pre-existing radio-learned inventory entry with a different SA.
func TestDispatchSend_ShortAddrDoesNotPoisonInventory(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	rec := newRecvCallback(1)
	c := New(sock, "bk", "v0")
	c.OnSend = rec.cb
	c.Inventory = inventory.NewMap()
	// Seed the inventory with the authentic radio-learned SA.
	const authSA = uint16(0x1111)
	const targetUID = "aabbccddeeff"
	c.Inventory.Record(targetUID, authSA)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	select {
	case env := <-srv.frames:
		if env.GetHello() == nil {
			t.Fatalf("first frame has no Hello body: %+v", env)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello within 2s")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("server has no accepted conn after Hello")
	}

	// Send with a different short_addr than the inventory holds.
	const attackerSA = uint16(0x9999)
	l2 := infoQueryL2()
	send := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid:   targetUID,
		Frame:     l2,
		ShortAddr: uint32(attackerSA),
	}}}
	if err := srv.sendToClient(send); err != nil {
		t.Fatalf("sendToClient: %v", err)
	}

	rec.wait(t, 2*time.Second)

	// The frame must have been sent to the SA in the request (not authSA).
	got := rec.snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 frame, got %d", len(got))
	}
	wantL1 := BuildL1Frame(attackerSA, l2)
	if !bytes.Equal(got[0], wantL1) {
		t.Fatalf("OnSend frame mismatch:\n  got  %x\n  want %x", got[0], wantL1)
	}

	// The pre-existing inventory entry must be unchanged — a controller
	// Send must never overwrite the radio-learned binding.
	if sa, ok := c.Inventory.LookupSA(targetUID); !ok || sa != authSA {
		t.Fatalf("inventory entry for %q was poisoned: got (0x%04X, %v), want (0x%04X, true)",
			targetUID, sa, ok, authSA)
	}
}

// TestDispatchSend_ShortAddrZero_FallsBackToInventory verifies that a Send
// with short_addr == 0 resolves via the inventory lookup unchanged.
func TestDispatchSend_ShortAddrZero_FallsBackToInventory(t *testing.T) {
	t.Parallel()

	sock := tempSocketPath(t)
	srv := newTestServer(t, sock)
	defer srv.close()

	captureLog(t)

	rec := newRecvCallback(1)
	c := New(sock, "bk", "v0")
	c.OnSend = rec.cb
	c.Inventory = inventory.NewMap()
	const targetSA = uint16(0xC459)
	const targetUID = "999900000003"
	c.Inventory.Record(targetUID, targetSA)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	select {
	case env := <-srv.frames:
		if env.GetHello() == nil {
			t.Fatalf("first frame has no Hello body: %+v", env)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no Hello within 2s")
	}
	if !srv.waitForConn(time.Second) {
		t.Fatalf("server has no accepted conn after Hello")
	}

	l2 := infoQueryL2()
	wantL1 := BuildL1Frame(targetSA, l2)

	// Send with short_addr == 0 — must resolve via inventory.
	send := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid:   targetUID,
		Frame:     l2,
		ShortAddr: 0,
	}}}
	if err := srv.sendToClient(send); err != nil {
		t.Fatalf("sendToClient: %v", err)
	}

	rec.wait(t, 2*time.Second)
	got := rec.snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 frame, got %d", len(got))
	}
	if !bytes.Equal(got[0], wantL1) {
		t.Fatalf("OnSend frame mismatch:\n  got  %x\n  want %x", got[0], wantL1)
	}
	if st := c.Stats(); st.SendOkTotal != 1 || st.SendFailTotal != 0 {
		t.Fatalf("Stats unexpected: %+v", st)
	}
}

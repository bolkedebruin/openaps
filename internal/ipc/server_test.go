package ipc

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bolke/inv-driver/internal/events"
	"github.com/bolke/inv-driver/internal/ingest"
	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
)

// socketPath returns a short UDS path inside t.TempDir(). On macOS the
// limit is 104 chars, so we deliberately use a tiny filename.
func socketPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "s.sock")
	if len(p) < 100 {
		return p
	}
	return filepath.Join(os.TempDir(), "iv-"+strconv.FormatInt(time.Now().UnixNano(), 36)+".sock")
}

// newServer constructs a Server with its own fresh store. Returns the
// server, its socket path, and a context cancel func to trigger
// shutdown. Starts Serve in a goroutine and waits until the socket is
// listening before returning.
func newServer(t *testing.T) (*Server, string, context.CancelFunc, <-chan error) {
	t.Helper()
	path := socketPath(t)
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	srv := &Server{
		SocketPath: path,
		Ingestor:   &ingest.Ingestor{S: st},
	}
	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan error, 1)
	go func() { doneCh <- srv.Serve(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if _, err := os.Stat(path); err != nil {
		cancel()
		<-doneCh
		t.Fatalf("server socket never came up: %v", err)
	}
	return srv, path, cancel, doneCh
}

func helloEnv() *wire.Envelope {
	return &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend:     "test-backend",
		Version:     "0.0.1",
		StartedAtMs: 1,
	}}}
}

func telemetryEnv(uid string, ts int64, acW float64) *wire.Envelope {
	return &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
		TsMs:         ts,
		PeerUid:      uid,
		Model:        "QS1A",
		ActivePowerW: acW,
	}}}
}

func dial(t *testing.T, path string) net.Conn {
	t.Helper()
	c, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	return c
}

func waitForRowCount(t *testing.T, srv *Server, query string, want int) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var got int
	for time.Now().Before(deadline) {
		if err := srv.Ingestor.S.DB().QueryRow(query).Scan(&got); err != nil {
			t.Fatalf("query: %v", err)
		}
		if got == want {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	return got
}

func TestServer_HelloFirstContract(t *testing.T) {
	srv, path, cancel, doneCh := newServer(t)
	t.Cleanup(func() { cancel(); <-doneCh })

	c := dial(t, path)
	if err := wire.WriteFrame(c, telemetryEnv("uid1", 100, 50.0)); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, _ = c.Read(buf)
	_ = c.Close()

	for _, tbl := range []string{"inverters", "telemetry_live", "events"} {
		var n int
		if err := srv.Ingestor.S.DB().QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", tbl, err)
		}
		if n != 0 {
			t.Fatalf("%s: got %d rows, want 0", tbl, n)
		}
	}
}

func TestServer_HappyPath(t *testing.T) {
	srv, path, cancel, doneCh := newServer(t)
	t.Cleanup(func() { cancel(); <-doneCh })

	c := dial(t, path)
	defer c.Close()

	if err := wire.WriteFrame(c, helloEnv()); err != nil {
		t.Fatalf("hello: %v", err)
	}
	for i, w := range []float64{100, 200, 300} {
		if err := wire.WriteFrame(c, telemetryEnv("uidA", int64(1000+i), w)); err != nil {
			t.Fatalf("telemetry %d: %v", i, err)
		}
	}

	got := waitForRowCount(t, srv, `SELECT COUNT(*) FROM events WHERE kind='telemetry'`, 3)
	if got != 3 {
		t.Fatalf("telemetry events: got %d want 3", got)
	}

	var acW float64
	if err := srv.Ingestor.S.DB().QueryRow(`SELECT ac_w FROM telemetry_live WHERE inverter_uid='uidA'`).Scan(&acW); err != nil {
		t.Fatalf("scan ac_w: %v", err)
	}
	if acW != 300 {
		t.Fatalf("ac_w: got %v want 300", acW)
	}
}

func TestServer_PerEventErrorTolerance(t *testing.T) {
	srv, path, cancel, doneCh := newServer(t)
	t.Cleanup(func() { cancel(); <-doneCh })

	c := dial(t, path)
	defer c.Close()

	if err := wire.WriteFrame(c, helloEnv()); err != nil {
		t.Fatalf("hello: %v", err)
	}
	bad := &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
		TsMs:    1,
		PeerUid: "",
		Model:   "QS1A",
	}}}
	if err := wire.WriteFrame(c, bad); err != nil {
		t.Fatalf("write bad: %v", err)
	}
	if err := wire.WriteFrame(c, telemetryEnv("good-uid", 2, 42.0)); err != nil {
		t.Fatalf("write good: %v", err)
	}

	got := waitForRowCount(t, srv, `SELECT COUNT(*) FROM telemetry_live WHERE inverter_uid='good-uid'`, 1)
	if got != 1 {
		t.Fatalf("good telemetry not landed: got %d", got)
	}
}

// TestServer_MalformedProtoBodyDropsConn exercises the wire-decode
// failure path: a framed body with invalid proto wire-format causes
// the server to drop the conn without ingesting anything.
func TestServer_MalformedProtoBodyDropsConn(t *testing.T) {
	srv, path, cancel, doneCh := newServer(t)
	t.Cleanup(func() { cancel(); <-doneCh })

	c := dial(t, path)
	defer c.Close()

	// Write a valid frame header followed by garbage bytes (invalid
	// proto wire-format).
	garbage := []byte{0xff, 0xff, 0xff, 0xff}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(garbage)))
	if _, err := c.Write(hdr[:]); err != nil {
		t.Fatalf("write hdr: %v", err)
	}
	if _, err := c.Write(garbage); err != nil {
		t.Fatalf("write body: %v", err)
	}

	// Server should close conn (since first frame failed to decode as
	// proto and therefore as Hello).
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, _ = c.Read(buf)

	for _, tbl := range []string{"inverters", "telemetry_live", "events"} {
		var n int
		if err := srv.Ingestor.S.DB().QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", tbl, err)
		}
		if n != 0 {
			t.Fatalf("%s: got %d rows, want 0", tbl, n)
		}
	}
}

func TestServer_CleanShutdown(t *testing.T) {
	_, path, cancel, doneCh := newServer(t)

	c := dial(t, path)
	if err := wire.WriteFrame(c, helloEnv()); err != nil {
		t.Fatalf("hello: %v", err)
	}

	cancel()
	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("Serve returned: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return within 3s")
	}
	_ = c.Close()

	if conn, err := net.DialTimeout("unix", path, 200*time.Millisecond); err == nil {
		_ = conn.Close()
		t.Fatal("dial after shutdown should fail")
	}
}

func TestServer_StaleSocket_RefusesNonSocket(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.sock")
	if err := os.WriteFile(path, []byte("not a socket"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	srv := &Server{SocketPath: path, Ingestor: &ingest.Ingestor{S: st}}
	err = srv.Serve(context.Background())
	if err == nil {
		t.Fatal("expected Serve to refuse non-socket path")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("non-socket file gone after refusal: %v", statErr)
	}
}

func TestServer_StaleSocket_RefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	path := filepath.Join(dir, "s.sock")
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	srv := &Server{SocketPath: path, Ingestor: &ingest.Ingestor{S: st}}
	err = srv.Serve(context.Background())
	if err == nil {
		t.Fatal("expected Serve to refuse symlink path")
	}
	// Target must still exist.
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("symlink target gone after refusal: %v", statErr)
	}
}

// TestServer_SubscriberReceivesPublishedEnvelopes attaches a publisher
// to the server and a subscriber connection to the UDS, then has a
// publisher connection submit a telemetry frame and asserts the
// subscriber receives the same envelope.
func TestServer_SubscriberReceivesPublishedEnvelopes(t *testing.T) {
	path := socketPath(t)
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	pub := events.New()
	srv := &Server{
		SocketPath: path,
		Ingestor:   &ingest.Ingestor{S: st, Pub: pub},
		Publisher:  pub,
	}
	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan error, 1)
	go func() { doneCh <- srv.Serve(ctx) }()
	t.Cleanup(func() { cancel(); <-doneCh })

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	// Subscriber side.
	sub := dial(t, path)
	defer sub.Close()
	subHello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend: "test-sub", Version: "0.0.0", StartedAtMs: 1, Role: wire.Role_SUBSCRIBER,
	}}}
	if err := wire.WriteFrame(sub, subHello); err != nil {
		t.Fatalf("sub hello: %v", err)
	}

	// Give the server a moment to register the subscriber.
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if pub.Len() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if pub.Len() == 0 {
		t.Fatal("publisher has no subscribers after hello")
	}

	// Publisher side.
	pubConn := dial(t, path)
	defer pubConn.Close()
	if err := wire.WriteFrame(pubConn, helloEnv()); err != nil {
		t.Fatalf("pub hello: %v", err)
	}
	if err := wire.WriteFrame(pubConn, telemetryEnv("uidS", 42, 99.0)); err != nil {
		t.Fatalf("pub telemetry: %v", err)
	}

	// Subscriber should receive a Telemetry envelope.
	_ = sub.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got wire.Envelope
	if err := wire.ReadFrame(sub, &got); err != nil {
		t.Fatalf("subscriber read: %v", err)
	}
	tel := got.GetTelemetry()
	if tel == nil {
		t.Fatalf("expected Telemetry envelope, got %T", got.GetBody())
	}
	if tel.GetPeerUid() != "uidS" || tel.GetActivePowerW() != 99.0 {
		t.Fatalf("payload: %+v", tel)
	}
}

// TestServer_SubscriberClosedOnInboundFrame verifies a subscriber
// that sends a post-Hello frame is closed by the server.
func TestServer_SubscriberClosedOnInboundFrame(t *testing.T) {
	path := socketPath(t)
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	pub := events.New()
	srv := &Server{
		SocketPath: path,
		Ingestor:   &ingest.Ingestor{S: st, Pub: pub},
		Publisher:  pub,
	}
	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan error, 1)
	go func() { doneCh <- srv.Serve(ctx) }()
	t.Cleanup(func() { cancel(); <-doneCh })

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	sub := dial(t, path)
	defer sub.Close()
	subHello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend: "test-sub", Version: "0.0.0", StartedAtMs: 1, Role: wire.Role_SUBSCRIBER,
	}}}
	if err := wire.WriteFrame(sub, subHello); err != nil {
		t.Fatalf("sub hello: %v", err)
	}
	// Wait until the publisher registers us.
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if pub.Len() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if pub.Len() == 0 {
		t.Fatal("publisher has no subscribers")
	}

	// Now send an unexpected frame after Hello. The server should
	// close the connection.
	if err := wire.WriteFrame(sub, helloEnv()); err != nil {
		t.Fatalf("write second frame: %v", err)
	}

	// Verify the subscriber is dropped from the publisher within a
	// short window.
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pub.Len() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not drop subscriber after unexpected inbound frame; pub.Len=%d", pub.Len())
}

// TestServer_SubscriberBlockedWriteIsTornDown verifies a subscriber
// that never reads is dropped within writeDeadline once Publisher
// starts writing.
func TestServer_SubscriberBlockedWriteIsTornDown(t *testing.T) {
	path := socketPath(t)
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	pub := events.New()
	srv := &Server{
		SocketPath: path,
		Ingestor:   &ingest.Ingestor{S: st, Pub: pub},
		Publisher:  pub,
	}
	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan error, 1)
	go func() { doneCh <- srv.Serve(ctx) }()
	t.Cleanup(func() { cancel(); <-doneCh })

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	sub := dial(t, path)
	defer sub.Close()
	subHello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend: "test-sub-block", Version: "0.0.0", StartedAtMs: 1, Role: wire.Role_SUBSCRIBER,
	}}}
	if err := wire.WriteFrame(sub, subHello); err != nil {
		t.Fatalf("sub hello: %v", err)
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if pub.Len() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if pub.Len() == 0 {
		t.Fatal("publisher has no subscribers")
	}

	// Build a large telemetry envelope and publish enough copies that
	// the socket buffer fills and a write hits the deadline.
	big := make([]byte, 32*1024)
	for i := range big {
		big[i] = 'A' + byte(i%26)
	}
	bigEnv := &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
		TsMs:    1,
		PeerUid: "block-uid-1",
		Model:   "QS1A",
		// Stuff a large field so the on-wire payload is huge.
		// reactive_var is just a double, so we pack panels.
		Panels: func() []*wire.Panel {
			out := make([]*wire.Panel, 200)
			for i := range out {
				out[i] = &wire.Panel{Index: int32(i), DcV: float64(i), DcI: float64(i), W: float64(i)}
			}
			return out
		}(),
	}}}

	// The publish loop will fill the per-subscriber chanCap quickly
	// and then evict; the per-write deadline kicks in when the
	// kernel send buffer is also full. Wait until pub drops us.
	doneFill := make(chan struct{})
	go func() {
		defer close(doneFill)
		for i := 0; i < 1000; i++ {
			pub.Publish(bigEnv)
		}
	}()

	dropDeadline := time.Now().Add(writeDeadline + 5*time.Second)
	for time.Now().Before(dropDeadline) {
		if pub.Len() == 0 {
			<-doneFill
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("blocked subscriber not dropped within %s; pub.Len=%d", writeDeadline+5*time.Second, pub.Len())
}

func TestServer_ConcurrentConnections(t *testing.T) {
	srv, path, cancel, doneCh := newServer(t)
	t.Cleanup(func() { cancel(); <-doneCh })

	const clients = 5
	const perClient = 4

	var wg sync.WaitGroup
	errCh := make(chan error, clients)
	for i := 0; i < clients; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := net.DialTimeout("unix", path, 2*time.Second)
			if err != nil {
				errCh <- fmt.Errorf("dial %d: %w", i, err)
				return
			}
			defer c.Close()
			if err := wire.WriteFrame(c, helloEnv()); err != nil {
				errCh <- fmt.Errorf("hello %d: %w", i, err)
				return
			}
			for j := 0; j < perClient; j++ {
				uid := fmt.Sprintf("uid-%d-%d", i, j)
				if err := wire.WriteFrame(c, telemetryEnv(uid, int64(1000*i+j), float64(j))); err != nil {
					errCh <- fmt.Errorf("frame %d/%d: %w", i, j, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("client error: %v", err)
	}

	want := clients * perClient
	got := waitForRowCount(t, srv, `SELECT COUNT(*) FROM inverters`, want)
	if got != want {
		t.Fatalf("inverters: got %d want %d", got, want)
	}
}

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

	"github.com/bolkedebruin/openaps/internal/events"
	"github.com/bolkedebruin/openaps/internal/ingest"
	"github.com/bolkedebruin/openaps/internal/store"
	"github.com/bolkedebruin/openaps/wire"
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
	if err := wire.WriteFrame(c, telemetryEnv("000000000001", 100, 50.0)); err != nil {
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
		if err := wire.WriteFrame(c, telemetryEnv("00000000000a", int64(1000+i), w)); err != nil {
			t.Fatalf("telemetry %d: %v", i, err)
		}
	}

	// Telemetry is stored in telemetry_live (not as per-frame events);
	// wait until the latest sample (300 W) has been ingested.
	got := waitForRowCount(t, srv, `SELECT COUNT(*) FROM telemetry_live WHERE inverter_uid='00000000000a' AND ac_w=300`, 1)
	if got != 1 {
		t.Fatalf("telemetry_live not updated to 300 W (count %d)", got)
	}
	// Telemetry must not write event rows.
	var nEvents int
	if err := srv.Ingestor.S.DB().QueryRow(`SELECT COUNT(*) FROM events`).Scan(&nEvents); err != nil {
		t.Fatal(err)
	}
	if nEvents != 0 {
		t.Fatalf("telemetry should not write events, got %d", nEvents)
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
	if err := wire.WriteFrame(c, telemetryEnv("000000000060", 2, 42.0)); err != nil {
		t.Fatalf("write good: %v", err)
	}

	got := waitForRowCount(t, srv, `SELECT COUNT(*) FROM telemetry_live WHERE inverter_uid='000000000060'`, 1)
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
	if err := wire.WriteFrame(pubConn, telemetryEnv("0000000000a5", 42, 99.0)); err != nil {
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
	if tel.GetPeerUid() != "0000000000a5" || tel.GetActivePowerW() != 99.0 {
		t.Fatalf("payload: %+v", tel)
	}
}

// TestServer_SubscriberReceivesReplayOnAttach seeds the store with an
// inverter row, attaches a SUBSCRIBER, and asserts the row is pushed
// as an Envelope_Info before any live fanout. Late attachers must not
// have to wait for a fresh probe cycle to learn current state.
func TestServer_SubscriberReceivesReplayOnAttach(t *testing.T) {
	path := socketPath(t)
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	// Seed inverters table with a row carrying identity fields.
	ctx0 := context.Background()
	mc := uint32(0x18) // QS1A
	sw := uint32(5203)
	ph := uint32(1)
	tt := true
	if err := st.UpsertInverterFromTelemetry(ctx0, "000000abcdef", 0x1234, "qs1a", "QS1A", 100); err != nil {
		t.Fatalf("seed telemetry: %v", err)
	}
	if err := st.UpsertInverterInfo(ctx0, store.InverterInfoUpdate{
		UID: "000000abcdef", TsMs: 100, ShortAddr: 0x1234,
		Model: &mc, SoftwareVer: &sw, Phase: &ph, Bound: &tt,
	}); err != nil {
		t.Fatalf("seed info: %v", err)
	}

	pub := events.New()
	srv := &Server{
		SocketPath: path,
		Ingestor:   &ingest.Ingestor{S: st, Pub: pub},
		Publisher:  pub,
		Store:      st,
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
	if err := wire.WriteFrame(sub, &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend: "test-sub", Version: "0.0.0", StartedAtMs: 1, Role: wire.Role_SUBSCRIBER,
	}}}); err != nil {
		t.Fatalf("hello: %v", err)
	}

	_ = sub.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got wire.Envelope
	if err := wire.ReadFrame(sub, &got); err != nil {
		t.Fatalf("read replay: %v", err)
	}
	info := got.GetInfo()
	if info == nil {
		t.Fatalf("expected Envelope_Info from replay, got %T", got.GetBody())
	}
	if info.GetPeerUid() != "000000abcdef" {
		t.Fatalf("peer_uid: %q", info.GetPeerUid())
	}
	if info.GetShortAddr() != 0x1234 {
		t.Fatalf("short_addr: %d", info.GetShortAddr())
	}
	if info.ModelCode == nil || *info.ModelCode != 0x18 {
		t.Fatalf("model_code: %v", info.ModelCode)
	}
	if info.SoftwareVersion == nil || *info.SoftwareVersion != 5203 {
		t.Fatalf("software_version: %v", info.SoftwareVersion)
	}
	if info.Phase == nil || *info.Phase != 1 {
		t.Fatalf("phase: %v", info.Phase)
	}
	if info.ZigbeeBound == nil || *info.ZigbeeBound != true {
		t.Fatalf("zigbee_bound: %v", info.ZigbeeBound)
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
		PeerUid: "000000000b01",
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

// TestServer_SendToBackend_DeliversToPublisher verifies a downstream
// envelope queued via SendToBackend is delivered to the matching
// publisher connection.
func TestServer_SendToBackend_DeliversToPublisher(t *testing.T) {
	srv, path, cancel, doneCh := newServer(t)
	t.Cleanup(func() { cancel(); <-doneCh })

	c := dial(t, path)
	defer c.Close()
	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend: "apsystems-stock-zb", Version: "0.0.0", StartedAtMs: 1,
	}}}
	if err := wire.WriteFrame(c, hello); err != nil {
		t.Fatalf("hello: %v", err)
	}

	// Wait for the server to register the publisher.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.HasPublisher("apsystems-stock-zb") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !srv.HasPublisher("apsystems-stock-zb") {
		t.Fatal("publisher never registered")
	}

	sendEnv := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid:    "999900000003",
		Frame:      []byte{0xAA, 0xBB, 0xCC},
		DeadlineMs: 5000,
	}}}
	if ok := srv.SendToBackend("apsystems-stock-zb", sendEnv); !ok {
		t.Fatal("SendToBackend returned false")
	}

	// Publisher conn reads the downstream Send envelope.
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got wire.Envelope
	if err := wire.ReadFrame(c, &got); err != nil {
		t.Fatalf("read: %v", err)
	}
	send := got.GetSend()
	if send == nil {
		t.Fatalf("expected Send body, got %T", got.GetBody())
	}
	if send.GetPeerUid() != "999900000003" {
		t.Fatalf("peer_uid: %q", send.GetPeerUid())
	}
	if string(send.GetFrame()) != string([]byte{0xAA, 0xBB, 0xCC}) {
		t.Fatalf("frame: % X", send.GetFrame())
	}
}

// stubSettings is a SettingsHandler that records whether a set op reached
// the handler. A get op echoes the stored EcuId.
type stubSettings struct {
	mu     sync.Mutex
	ecuID  string
	setHit bool
}

func (s *stubSettings) Handle(_ context.Context, req *wire.SettingsRequest) *wire.SettingsResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	if set := req.GetSet(); set != nil {
		s.setHit = true
		s.ecuID = set.GetEcuId()
		return &wire.SettingsResponse{Ok: true, Settings: &wire.Settings{EcuId: s.ecuID}}
	}
	return &wire.SettingsResponse{Ok: true, Settings: &wire.Settings{EcuId: s.ecuID}}
}

// TestServer_SettingsGate proves a SET op from a non-controller backend is
// refused without reaching the handler, while a GET op is served for the
// same peer. newServer's Ingestor has no ControllerBackends, so any backend
// is a non-controller.
func TestServer_SettingsGate(t *testing.T) {
	srv, path, cancel, doneCh := newServer(t)
	t.Cleanup(func() { cancel(); <-doneCh })

	stub := &stubSettings{ecuID: "stored"}
	srv.Settings = stub

	c := dial(t, path)
	defer c.Close()
	if err := wire.WriteFrame(c, helloEnv()); err != nil {
		t.Fatalf("hello: %v", err)
	}

	// SET is controller-gated; a non-controller peer is refused.
	setEnv := &wire.Envelope{Body: &wire.Envelope_SettingsReq{SettingsReq: &wire.SettingsRequest{
		Op: &wire.SettingsRequest_Set{Set: &wire.Settings{EcuId: "evil"}},
	}}}
	if err := wire.WriteFrame(c, setEnv); err != nil {
		t.Fatalf("set write: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	var setResp wire.Envelope
	if err := wire.ReadFrame(c, &setResp); err != nil {
		t.Fatalf("set read: %v", err)
	}
	r := setResp.GetSettingsResp()
	if r == nil {
		t.Fatalf("expected SettingsResp, got %T", setResp.GetBody())
	}
	if r.GetOk() {
		t.Fatalf("set from non-controller was allowed")
	}
	stub.mu.Lock()
	hit := stub.setHit
	stub.mu.Unlock()
	if hit {
		t.Fatalf("refused set still reached the handler")
	}

	// GET is allowed for any peer.
	getEnv := &wire.Envelope{Body: &wire.Envelope_SettingsReq{SettingsReq: &wire.SettingsRequest{
		Op: &wire.SettingsRequest_Get{Get: &wire.Empty{}},
	}}}
	if err := wire.WriteFrame(c, getEnv); err != nil {
		t.Fatalf("get write: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	var getResp wire.Envelope
	if err := wire.ReadFrame(c, &getResp); err != nil {
		t.Fatalf("get read: %v", err)
	}
	g := getResp.GetSettingsResp()
	if g == nil || !g.GetOk() {
		t.Fatalf("get refused: %+v", g)
	}
	if g.GetSettings().GetEcuId() != "stored" {
		t.Fatalf("get returned %q, want %q", g.GetSettings().GetEcuId(), "stored")
	}
}

// TestServer_SendToBackend_UnknownBackend returns false.
func TestServer_SendToBackend_UnknownBackend(t *testing.T) {
	srv, _, cancel, doneCh := newServer(t)
	t.Cleanup(func() { cancel(); <-doneCh })

	env := &wire.Envelope{Body: &wire.Envelope_Reset_{Reset_: &wire.Reset{}}}
	if ok := srv.SendToBackend("nobody", env); ok {
		t.Fatal("SendToBackend to unknown backend returned true")
	}
}

// TestServer_SendToBackend_FullQueueDrops verifies the per-publisher
// outbound queue drops with the dropped counter rather than blocking
// when the publisher never reads.
func TestServer_SendToBackend_FullQueueDrops(t *testing.T) {
	srv, path, cancel, doneCh := newServer(t)
	t.Cleanup(func() { cancel(); <-doneCh })

	c := dial(t, path)
	defer c.Close()
	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend: "deaf-backend", Version: "0.0.0", StartedAtMs: 1,
	}}}
	if err := wire.WriteFrame(c, hello); err != nil {
		t.Fatalf("hello: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.HasPublisher("deaf-backend") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !srv.HasPublisher("deaf-backend") {
		t.Fatal("publisher never registered")
	}

	// Build an envelope big enough that the kernel send buffer fills
	// after a few writes, so the writer goroutine blocks in WriteFrame
	// and subsequent SendToBackend calls accumulate in pc.out.
	big := make([]byte, 16*1024)
	bigEnv := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid: "00000000fa11", Frame: big, DeadlineMs: 1000,
	}}}

	// Push beyond publisherOutCap. Client never reads.
	dropped := 0
	for i := 0; i < publisherOutCap*4; i++ {
		if ok := srv.SendToBackend("deaf-backend", bigEnv); !ok {
			dropped++
		}
	}
	if dropped == 0 {
		t.Fatal("expected at least one drop when publisher never reads")
	}
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
				uid := fmt.Sprintf("%012x", i*100+j)
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

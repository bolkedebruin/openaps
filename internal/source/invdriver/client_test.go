package invdriver

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bolke/ecu-sunspec/internal/testhelpers"
	"github.com/bolke/inv-driver/wire"
)

// fakeServer accepts one UDS connection at a time, reads a Hello, then
// pushes envelopes pushed onto its Send channel.
type fakeServer struct {
	t        *testing.T
	listener net.Listener
	mu       sync.Mutex
	conn     net.Conn
	hellos   chan *wire.Hello
}

func newFakeServer(t *testing.T) (*fakeServer, string) {
	t.Helper()
	dir := testhelpers.ShortTempDir(t)
	sock := filepath.Join(dir, "ic.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	fs := &fakeServer{t: t, listener: ln, hellos: make(chan *wire.Hello, 4)}
	go fs.accept()
	return fs, sock
}

func (fs *fakeServer) accept() {
	for {
		conn, err := fs.listener.Accept()
		if err != nil {
			return
		}
		fs.mu.Lock()
		if fs.conn != nil {
			_ = fs.conn.Close()
		}
		fs.conn = conn
		fs.mu.Unlock()

		go fs.serve(conn)
	}
}

func (fs *fakeServer) serve(conn net.Conn) {
	var env wire.Envelope
	if err := wire.ReadFrame(conn, &env); err != nil {
		return
	}
	if h := env.GetHello(); h != nil {
		select {
		case fs.hellos <- h:
		default:
		}
	}
	// Hold the conn open; further frames are pushed via Send.
	io.Copy(io.Discard, conn)
}

func (fs *fakeServer) send(t *testing.T, env *wire.Envelope) {
	t.Helper()
	fs.mu.Lock()
	c := fs.conn
	fs.mu.Unlock()
	if c == nil {
		t.Fatalf("send: no client connection yet")
	}
	if err := wire.WriteFrame(c, env); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func (fs *fakeServer) closeConn() {
	fs.mu.Lock()
	c := fs.conn
	fs.conn = nil
	fs.mu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}

func (fs *fakeServer) stop() {
	_ = fs.listener.Close()
	fs.closeConn()
}

func (fs *fakeServer) waitHello(t *testing.T, d time.Duration) *wire.Hello {
	t.Helper()
	select {
	case h := <-fs.hellos:
		return h
	case <-time.After(d):
		t.Fatalf("waitHello: timeout after %s", d)
		return nil
	}
}

func telemetryEnv(uid, model string, watts float64) *wire.Envelope {
	return &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
		TsMs:          time.Now().UnixMilli(),
		PeerUid:       uid,
		Model:         model,
		ActivePowerW:  watts,
		GridV:         231,
		FreqHz:        50.01,
		LifetimeRaw:   []uint64{100, 200},
		LifetimeScale: 1.0,
		Panels: []*wire.Panel{
			{Index: 0, DcV: 35, DcI: 6, W: watts / 2},
			{Index: 1, DcV: 35, DcI: 6, W: watts / 2},
		},
	}}}
}

func waitFor(t *testing.T, d time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("waitFor: condition never true within %s", d)
}

func TestClient_HappyPath(t *testing.T) {
	fs, sock := newFakeServer(t)
	defer fs.stop()

	c := New(sock, "test")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop()

	h := fs.waitHello(t, 2*time.Second)
	if h.GetBackend() != "ecu-sunspec" {
		t.Errorf("backend=%q want ecu-sunspec", h.GetBackend())
	}
	if h.GetRole() != wire.Role_SUBSCRIBER {
		t.Errorf("role=%v want SUBSCRIBER", h.GetRole())
	}
	if h.GetVersion() != "test" {
		t.Errorf("version=%q want test", h.GetVersion())
	}

	fs.send(t, telemetryEnv("ABC0001", "QS1A", 480))
	waitFor(t, 2*time.Second, func() bool {
		_, ok := c.Get("ABC0001")
		return ok
	})

	snap, ok := c.Get("ABC0001")
	if !ok {
		t.Fatal("get returned !ok")
	}
	if snap.Model != "QS1A" || snap.ACWatts != 480 || snap.ACVolts != 231 {
		t.Errorf("snapshot mismatch: %+v", snap)
	}
	if snap.LifetimeWh != 300 {
		t.Errorf("lifetime Wh=%v want 300", snap.LifetimeWh)
	}
	if len(snap.Panels) != 2 {
		t.Errorf("panels=%d want 2", len(snap.Panels))
	}

	st := c.Stats()
	if !st.Connected {
		t.Error("Stats.Connected=false")
	}
	if st.Inverters != 1 {
		t.Errorf("Stats.Inverters=%d want 1", st.Inverters)
	}
}

func TestClient_Reconnect(t *testing.T) {
	dir := testhelpers.ShortTempDir(t)
	sock := filepath.Join(dir, "ic.sock")

	fs1, sock1 := newFakeServerAt(t, sock)
	c := New(sock, "test")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop()

	_ = sock1
	fs1.waitHello(t, 2*time.Second)
	fs1.send(t, telemetryEnv("R1", "DS3", 200))
	waitFor(t, 2*time.Second, func() bool {
		_, ok := c.Get("R1")
		return ok
	})

	// Drop the server. The dial loop should retry against the same path.
	fs1.stop()
	// Recreate listener on the same socket path after a brief pause.
	if err := os.Remove(sock); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove sock: %v", err)
	}
	fs2, _ := newFakeServerAt(t, sock)
	defer fs2.stop()

	fs2.waitHello(t, 5*time.Second)
	fs2.send(t, telemetryEnv("R2", "QS1A", 555))
	waitFor(t, 5*time.Second, func() bool {
		_, ok := c.Get("R2")
		return ok
	})
}

func newFakeServerAt(t *testing.T, sock string) (*fakeServer, string) {
	t.Helper()
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen %s: %v", sock, err)
	}
	fs := &fakeServer{t: t, listener: ln, hellos: make(chan *wire.Hello, 4)}
	go fs.accept()
	return fs, sock
}

func TestClient_StopUnblocks(t *testing.T) {
	fs, sock := newFakeServer(t)
	defer fs.stop()

	c := New(sock, "test")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	fs.waitHello(t, 2*time.Second)

	stopDone := make(chan struct{})
	go func() {
		c.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return within 2s")
	}
}

func infoEnv(uid string, modelCode, swVer, phase uint32) *wire.Envelope {
	mc, sv, ph := modelCode, swVer, phase
	return &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{
		TsMs:            time.Now().UnixMilli(),
		PeerUid:         uid,
		ModelCode:       &mc,
		SoftwareVersion: &sv,
		Phase:           &ph,
	}}}
}

// TestClient_InfoMergesIntoSnapshot verifies that an InverterInfo
// envelope merges identity fields onto an existing telemetry snapshot
// without disturbing telemetry values, and that an Info envelope
// arriving before telemetry creates an identity-only snapshot.
func TestClient_InfoMergesIntoSnapshot(t *testing.T) {
	fs, sock := newFakeServer(t)
	defer fs.stop()

	c := New(sock, "test")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop()
	fs.waitHello(t, 2*time.Second)

	fs.send(t, telemetryEnv("UIDINFO1", "QS1A", 600))
	waitFor(t, 2*time.Second, func() bool {
		_, ok := c.Get("UIDINFO1")
		return ok
	})

	// Info envelope merges identity onto the cached telemetry.
	fs.send(t, infoEnv("UIDINFO1", 0x18, 0x010203, 1))
	waitFor(t, 2*time.Second, func() bool {
		s, ok := c.Get("UIDINFO1")
		return ok && s.ModelCode != nil
	})

	snap, _ := c.Get("UIDINFO1")
	if snap.ModelCode == nil || *snap.ModelCode != 0x18 {
		t.Errorf("ModelCode=%v want 0x18", snap.ModelCode)
	}
	if snap.SoftwareVersion == nil || *snap.SoftwareVersion != 0x010203 {
		t.Errorf("SoftwareVersion=%v want 0x010203", snap.SoftwareVersion)
	}
	if snap.Phase == nil || *snap.Phase != 1 {
		t.Errorf("Phase=%v want 1", snap.Phase)
	}
	if snap.ACWatts != 600 || snap.Model != "QS1A" {
		t.Errorf("telemetry clobbered by Info merge: %+v", snap)
	}
	if snap.InfoFetchedAt.IsZero() {
		t.Error("InfoFetchedAt not set")
	}

	// A subsequent telemetry frame must preserve the identity fields.
	fs.send(t, telemetryEnv("UIDINFO1", "QS1A", 700))
	waitFor(t, 2*time.Second, func() bool {
		s, _ := c.Get("UIDINFO1")
		return s.ACWatts == 700
	})
	snap, _ = c.Get("UIDINFO1")
	if snap.ModelCode == nil || *snap.ModelCode != 0x18 {
		t.Errorf("ModelCode lost after telemetry: %+v", snap.ModelCode)
	}
}

// TestClient_InfoOnlyDoesNotCrash exercises the edge case of an
// InverterInfo envelope arriving for a UID we have never received
// telemetry for, and an envelope carrying nothing but peer_uid.
func TestClient_InfoOnlyDoesNotCrash(t *testing.T) {
	fs, sock := newFakeServer(t)
	defer fs.stop()

	c := New(sock, "test")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop()
	fs.waitHello(t, 2*time.Second)

	// Bare info envelope (peer_uid only).
	fs.send(t, &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{
		PeerUid: "BAREUIDONLY",
	}}})
	waitFor(t, 2*time.Second, func() bool {
		_, ok := c.Get("BAREUIDONLY")
		return ok
	})
	snap, _ := c.Get("BAREUIDONLY")
	if snap.ModelCode != nil || snap.SoftwareVersion != nil || snap.Phase != nil {
		t.Errorf("bare info envelope produced identity fields: %+v", snap)
	}
}

// TestFromTelemetry_HostileScaleClamped verifies that a NaN or
// infinite LifetimeScale paired with a hostile raw counter is
// neutralised by fromTelemetry before it reaches the downstream
// uint64 cast.
func TestFromTelemetry_HostileScaleClamped(t *testing.T) {
	cases := []struct {
		name  string
		scale float64
	}{
		{"NaN", math.NaN()},
		{"+Inf", math.Inf(1)},
		{"-Inf", math.Inf(-1)},
		{"negative", -1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := fromTelemetry(&wire.Telemetry{
				PeerUid:       "X",
				LifetimeRaw:   []uint64{1<<63 - 1, 1<<63 - 1},
				LifetimeScale: tc.scale,
			})
			if snap.LifetimeWh != 0 {
				t.Errorf("LifetimeWh=%v want 0 for scale=%v", snap.LifetimeWh, tc.scale)
			}
		})
	}
}

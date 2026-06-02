// Package invdriver is a UDS subscriber to inv-driver. It keeps an
// in-memory map of the latest decoded telemetry per inverter UID and
// exposes it via Get / All for the SunSpec snapshot builder.
package invdriver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bolke/inv-driver/wire"
)

// backendName is the Hello backend this client identifies as, on both
// the subscriber stream and the control connection. inv-driver must
// list it in -controller-backends for Send to be accepted (see
// packaging/S48-inv-driver).
const backendName = "ecu-sunspec"

// controlDialTimeout bounds dial + write on a one-shot control
// connection. Writes are rare (operator curtailment events), so a short
// budget is fine.
const controlDialTimeout = 1500 * time.Millisecond

// Client subscribes to inv-driver over UDS and caches the latest
// Telemetry envelope per inverter UID.
type Client struct {
	socketPath string
	version    string

	mu      sync.RWMutex
	latest  map[string]Snapshot
	fleet   *wire.FleetSummary          // latest FleetSummary from inv-driver; nil before first frame
	prot    map[string]*wire.Protection // latest grid-protection state per UID
	lastAt  time.Time
	started time.Time

	connected atomic.Bool

	cancel context.CancelFunc
	done   chan struct{}

	// Logger is optional. nil means silent.
	Logger *log.Logger
}

// Stats is a small read-only view of the subscriber's state.
type Stats struct {
	Connected   bool
	Inverters   int
	LastFrameAt time.Time
}

// New constructs a Client. It does not dial; call Start.
func New(socketPath, version string) *Client {
	return &Client{
		socketPath: socketPath,
		version:    version,
		latest:     make(map[string]Snapshot),
		prot:       make(map[string]*wire.Protection),
	}
}

// Start spawns the subscriber goroutine. Non-blocking. The returned
// error is non-nil only for misconfiguration; transport failures are
// retried internally by wire.DialLoop.
func (c *Client) Start(ctx context.Context) error {
	if c.socketPath == "" {
		return errors.New("invdriver: socket path is empty")
	}
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.done = make(chan struct{})
	c.started = time.Now()

	go func() {
		defer close(c.done)
		err := wire.DialLoop(ctx, wire.DialLoopConfig{
			SocketPath: c.socketPath,
			OnConnect: func() {
				c.connected.Store(true)
				c.logf("invdriver: connected %s", c.socketPath)
			},
			OnDialError: func(err error, retryIn time.Duration) {
				c.connected.Store(false)
				c.logf("invdriver: dial/session %s: %v (retry in %s)", c.socketPath, err, retryIn)
			},
			Session: func(ctx context.Context, conn net.Conn) error {
				err := c.session(ctx, conn)
				c.connected.Store(false)
				return err
			},
		})
		c.connected.Store(false)
		if err != nil && !errors.Is(err, context.Canceled) {
			c.logf("invdriver: dial loop exit: %v", err)
		}
	}()
	return nil
}

// Stop cancels the subscriber goroutine and waits for it to return.
func (c *Client) Stop() {
	if c.cancel == nil {
		return
	}
	c.cancel()
	if c.done != nil {
		<-c.done
	}
}

// Stats reports current connectivity and cache state.
func (c *Client) Stats() Stats {
	connected := c.connected.Load()
	c.mu.RLock()
	n := len(c.latest)
	last := c.lastAt
	c.mu.RUnlock()
	return Stats{
		Connected:   connected,
		Inverters:   n,
		LastFrameAt: last,
	}
}

// Get returns the latest Snapshot for uid. ok is false when no
// telemetry has been seen yet.
func (c *Client) Get(uid string) (Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.latest[uid]
	return s, ok
}

// All returns a defensive copy of the entire telemetry map.
func (c *Client) All() map[string]Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]Snapshot, len(c.latest))
	for k, v := range c.latest {
		out[k] = v
	}
	return out
}

// Fleet returns the latest FleetSummary, or nil if inv-driver hasn't
// published one yet (e.g., during the post-attach window before the
// replay frame arrives).
func (c *Client) Fleet() *wire.FleetSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fleet
}

func (c *Client) logf(format string, args ...any) {
	if c.Logger != nil {
		c.Logger.Printf(format, args...)
	}
}

// newHello builds the Hello frame for the given role. Shared by the
// subscriber stream (Role_SUBSCRIBER) and the one-shot control
// connection (Role_ROLE_UNSPECIFIED → routed as a controller).
func (c *Client) newHello(role wire.Role) *wire.Hello {
	hostname, _ := os.Hostname()
	return &wire.Hello{
		Backend:     backendName,
		Version:     c.version,
		Hostname:    hostname,
		StartedAtMs: c.started.UnixMilli(),
		Role:        role,
	}
}

// Send dispatches one L2 frame to the inverter identified by uid through
// inv-driver's downstream path (Envelope_Send → ecu-zb → radio). The
// subscriber stream is read-only after Hello, so Send opens its own
// short-lived control connection: dial, Hello as a controller (role
// unspecified, so the daemon runs its publisher/ingest loop and routes
// the frame), write one envelope, close.
//
// Returning nil means the daemon accepted the frame for downstream
// delivery, not that the inverter applied it — dispatch is
// fire-and-forget, same as the SQLite path it replaces.
func (c *Client) Send(ctx context.Context, uid string, frame []byte) error {
	if c == nil {
		return errors.New("invdriver: client is nil")
	}
	if uid == "" {
		return errors.New("invdriver: empty uid")
	}
	if len(frame) == 0 {
		return errors.New("invdriver: empty frame")
	}
	env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{PeerUid: uid, Frame: frame}}}
	return c.sendControl(ctx, env)
}

// sendControl dials a one-shot control connection, says Hello, writes
// one envelope, and closes. Any dial/hello/write error is surfaced to
// the caller.
func (c *Client) sendControl(ctx context.Context, env *wire.Envelope) error {
	d := net.Dialer{Timeout: controlDialTimeout}
	conn, err := d.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("invdriver: dial %s: %w", c.socketPath, err)
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(controlDialTimeout))
	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: c.newHello(wire.Role_ROLE_UNSPECIFIED)}}
	if err := wire.WriteFrame(conn, hello); err != nil {
		return fmt.Errorf("invdriver: control hello: %w", err)
	}
	if err := wire.WriteFrame(conn, env); err != nil {
		return fmt.Errorf("invdriver: control send: %w", err)
	}
	return nil
}

// session writes the Hello frame and dispatches incoming envelopes to
// applyTelemetry / applyInfo. Other envelope kinds are dropped.
// Returns nil on context cancel or clean peer close; non-nil on
// transport errors so wire.DialLoop can retry.
func (c *Client) session(ctx context.Context, conn net.Conn) error {
	if conn == nil {
		return errors.New("invdriver: nil conn")
	}
	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: c.newHello(wire.Role_SUBSCRIBER)}}
	if err := wire.WriteFrame(conn, hello); err != nil {
		return fmt.Errorf("hello: %w", err)
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	for {
		var in wire.Envelope
		if err := wire.ReadFrame(conn, &in); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		switch body := in.GetBody().(type) {
		case *wire.Envelope_Telemetry:
			c.applyTelemetry(body.Telemetry)
		case *wire.Envelope_Info:
			c.applyInfo(body.Info)
		case *wire.Envelope_Fleet:
			c.applyFleet(body.Fleet)
		case *wire.Envelope_Protection:
			c.applyProtection(body.Protection)
		}
	}
}

// applyTelemetry stores a decoded telemetry snapshot for the
// inverter's UID, preserving any identity fields already populated
// from a prior InverterInfo envelope.
func (c *Client) applyTelemetry(t *wire.Telemetry) {
	if t == nil {
		return
	}
	uid := t.GetPeerUid()
	if uid == "" {
		return
	}
	snap := fromTelemetry(t)
	c.mu.Lock()
	prev := c.latest[uid]
	snap.ModelCode = prev.ModelCode
	snap.SoftwareVersion = prev.SoftwareVersion
	snap.Phase = prev.Phase
	snap.ZigbeeBound = prev.ZigbeeBound
	snap.TurnedOffRpt = prev.TurnedOffRpt
	snap.InfoFetchedAt = prev.InfoFetchedAt
	c.latest[uid] = snap
	c.lastAt = snap.FetchedAt
	c.mu.Unlock()
}

// applyFleet caches the latest FleetSummary. Replaces the previous
// value wholesale — inv-driver always sends a complete snapshot, not
// a delta.
func (c *Client) applyFleet(f *wire.FleetSummary) {
	if f == nil {
		return
	}
	c.mu.Lock()
	c.fleet = f
	c.mu.Unlock()
}

// applyProtection caches the latest grid-protection state for an
// inverter UID. Replaces the previous value wholesale.
func (c *Client) applyProtection(p *wire.Protection) {
	if p == nil || p.GetPeerUid() == "" {
		return
	}
	c.mu.Lock()
	c.prot[p.GetPeerUid()] = p
	c.mu.Unlock()
}

// Protection returns a defensive copy of the latest grid-protection
// state per inverter UID. Empty before inv-driver publishes any.
func (c *Client) Protection() map[string]*wire.Protection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]*wire.Protection, len(c.prot))
	for k, v := range c.prot {
		out[k] = v
	}
	return out
}

// applyInfo merges identity fields from an InverterInfo envelope onto
// the existing snapshot for the UID. Telemetry fields are untouched.
// Optional fields that are unset on the wire leave the prior value in
// place.
func (c *Client) applyInfo(info *wire.InverterInfo) {
	if info == nil {
		return
	}
	uid := info.GetPeerUid()
	if uid == "" {
		return
	}
	c.mu.Lock()
	c.latest[uid] = mergeInfo(c.latest[uid], info)
	c.mu.Unlock()
}

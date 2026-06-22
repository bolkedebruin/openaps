// Package uds connects ecu-web to inv-driver over its Unix-domain
// socket. ecu-web is a pure UDS client: Subscriber attaches as a
// SUBSCRIBER and folds the decoded envelope stream (Telemetry /
// InverterInfo / FleetSummary / Protection) into a snapshot; Controller
// (controller.go) issues request/response control ops.
package uds

import (
	"context"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/bolkedebruin/openaps/cmd/ecu-web/internal/snapshot"
	"github.com/bolkedebruin/openaps/wire"
)

// Subscriber maintains a resilient SUBSCRIBER connection to inv-driver,
// applying every decoded envelope to Snap. It reconnects with backoff
// (via wire.DialLoop) so an inv-driver restart is transparent.
type Subscriber struct {
	SocketPath string
	Backend    string // Hello.backend identity, e.g. "ecu-web"
	Version    string
	Hostname   string
	Snap       *snapshot.Snapshot

	// connected reports whether a live subscriber connection is up. Used
	// by the system-status view.
	connected atomic.Bool
}

// Run blocks until ctx is cancelled, keeping the subscription alive.
func (s *Subscriber) Run(ctx context.Context) error {
	return wire.DialLoop(ctx, wire.DialLoopConfig{
		SocketPath: s.SocketPath,
		Session:    s.session,
		OnConnect:  func() { s.connected.Store(true); slog.Info("uds: connected", "socket", s.SocketPath) },
		OnDialError: func(err error, retryIn time.Duration) {
			s.connected.Store(false)
			slog.Warn("uds: dial", "err", err, "retry_in", retryIn.Round(time.Millisecond))
		},
	})
}

// Connected reports the current connection state.
func (s *Subscriber) Connected() bool { return s.connected.Load() }

func (s *Subscriber) session(ctx context.Context, conn net.Conn) error {
	hello := &wire.Hello{
		Backend:     s.Backend,
		Version:     s.Version,
		Hostname:    s.Hostname,
		StartedAtMs: time.Now().UnixMilli(),
		Role:        wire.Role_SUBSCRIBER,
	}
	env := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: hello}}
	if err := wire.WriteFrame(conn, env); err != nil {
		return err
	}

	// Close the conn on cancel so the blocking ReadFrame unwinds and
	// DialLoop observes the cancellation.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	defer s.connected.Store(false)
	for {
		var in wire.Envelope
		if err := wire.ReadFrame(conn, &in); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		s.dispatch(&in)
	}
}

// dispatch folds one envelope into the snapshot. Unknown bodies are
// ignored — the subscriber stream may carry envelope kinds ecu-web does
// not consume.
func (s *Subscriber) dispatch(env *wire.Envelope) {
	switch {
	case env.GetTelemetry() != nil:
		s.Snap.ApplyTelemetry(env.GetTelemetry())
	case env.GetInfo() != nil:
		s.Snap.ApplyInfo(env.GetInfo())
	case env.GetFleet() != nil:
		s.Snap.ApplyFleet(env.GetFleet())
	case env.GetProtection() != nil:
		s.Snap.ApplyProtection(env.GetProtection())
	}
}

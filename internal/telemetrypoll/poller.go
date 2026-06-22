// Package telemetrypoll originates periodic 0xBB telemetry queries to
// every inverter in inv-driver's own state.db inventory. Replies flow
// back through the normal RawFrame ingest path; only the outbound
// cadence lives here.
package telemetrypoll

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/internal/buslock"
	"github.com/bolkedebruin/openaps/internal/store"
	"github.com/bolkedebruin/openaps/wire"
)

// busLockOwner is the owner label the poll round uses when it acquires
// the shared bus guard. Visible in buslock.Held() diagnostics.
const busLockOwner = "telemetry-poll"

// InverterRef is one inventory entry suitable for poll dispatch.
type InverterRef struct {
	UID       string
	ShortAddr uint32
}

// Store is the subset of the state-store the Poller needs.
type Store interface {
	// ListInvertersForPoll returns inverters whose short_addr is known and non-zero.
	ListInvertersForPoll(ctx context.Context) ([]InverterRef, error)
}

// Sender is the subset of ipc.Server the Poller uses to dispatch frames.
type Sender interface {
	SendToBackend(backend string, env *wire.Envelope) bool
}

// Poller emits one 0xBB query per inverter per tick interval.
// Inverters are spaced SendInterval apart within each round so bursts
// are smoothed across the tick window.
type Poller struct {
	Store   Store
	Server  Sender
	Backend string
	// Interval is the cadence between poll rounds. Defaults to 1s.
	Interval time.Duration
	// SendInterval spaces individual per-inverter Sends within a round.
	// Defaults to 200ms.
	SendInterval time.Duration

	// BusLock, if non-nil, is the process-wide guard that pairing ops
	// and gridprofile broadcasts hold for the duration of a fleet-
	// disruptive operation. The poller is the POLITE holder: each round
	// AcquirePolite's it before emitting any 0xBB frames and Release's at
	// end. AcquirePolite returns a yield channel that closes the instant an
	// operator op begins waiting for the bus; the round selects on it and
	// returns early, so preemption rides a single revocation signal instead
	// of a separate mid-round check the holder must remember. This keeps the
	// poller from interleaving a 0xBB query with a pairing 0x05/0x0E/0x22 on
	// the modem fd AND from starving an operator op (which preempts via the
	// lock's priority acquire) on a large fleet whose round exceeds the
	// interval. nil disables the gate (legacy behaviour); production wiring
	// must set it.
	BusLock *buslock.Lock

	// cold-start: log "no inverters" once, not every round.
	emptyOnce sync.Once
	// busyOnce throttles the "skipped round, lock held" log so a
	// minutes-long pairing op produces one line, not one per second.
	busyOnce sync.Once
}

// Run starts the poll loop and blocks until ctx is cancelled.
// Logs and skips rounds where no backend is registered; resumes
// automatically when the backend reattaches.
func (p *Poller) Run(ctx context.Context) {
	interval := p.Interval
	if interval == 0 {
		interval = 1 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			start := time.Now()
			if err := p.poll(ctx); err != nil {
				slog.Error("telemetrypoll round", "err", err)
			}
			elapsed := time.Since(start)
			if elapsed > interval {
				slog.Warn("telemetrypoll round exceeds interval; effective cadence degraded",
					"elapsed", elapsed.Round(time.Millisecond), "interval", interval)
			}
		}
	}
}

// poll executes one round: enumerate inverters, send one 0xBB query
// each. Returns nil on normal completion (including an empty inventory
// or an absent backend).
//
// When BusLock is wired, the round first AcquirePolite's it. If a pairing
// op (or gridprofile broadcast) already owns the lock, or one is waiting to
// acquire it, the round is skipped — telemetry frames must not interleave
// with pairing primitives on the modem fd, and an operator op must not be
// starved. AcquirePolite hands back a yield channel that closes the instant
// an operator op begins waiting; the round selects on it before each send
// and during each inter-send wait, returning early so the operator op
// preempts immediately rather than at the next send or a poll tick. The
// lock is released before return so the next round (or the operator op
// waiting on it) sees a free bus.
func (p *Poller) poll(ctx context.Context) error {
	if p.Store == nil || p.Server == nil || p.Backend == "" {
		return fmt.Errorf("Store, Server, and Backend must be set")
	}
	// yield closes when an operator op begins waiting for the bus. When
	// BusLock is nil it stays nil, and a nil channel in a select never
	// fires — so the per-inverter loop's selects below are correct without
	// a guard.
	var yield <-chan struct{}
	if p.BusLock != nil {
		ok, y := p.BusLock.AcquirePolite(busLockOwner)
		if !ok {
			p.busyOnce.Do(func() {
				slog.Debug("telemetrypoll bus busy; skipping rounds until released")
			})
			return nil
		}
		yield = y
		// Reset the once so the next contention window also logs once.
		defer func() {
			p.BusLock.Release()
			p.busyOnce = sync.Once{}
		}()
	}
	inverters, err := p.Store.ListInvertersForPoll(ctx)
	if err != nil {
		return fmt.Errorf("list inverters: %w", err)
	}
	if len(inverters) == 0 {
		p.emptyOnce.Do(func() {
			slog.Info("telemetrypoll no inverters in state.db yet; telemetry seeds passively from observed replies")
		})
		return nil
	}

	sendInterval := p.SendInterval
	if sendInterval == 0 {
		sendInterval = 200 * time.Millisecond
	}

	frame := codec.OutboundBBQueryL2()

	for i, inv := range inverters {
		// Bail the instant the bus is revoked (yield closed) or the context
		// is cancelled. The deferred Release then frees the lock so the
		// waiting operator op claims it without running out this round.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-yield:
			return nil
		default:
		}

		env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
			PeerUid:   inv.UID,
			ShortAddr: inv.ShortAddr,
			Frame:     frame,
		}}}
		if ok := p.Server.SendToBackend(p.Backend, env); !ok {
			// Backend absent or queue full — skip the remainder of this
			// round; the next tick will retry.
			slog.Debug("telemetrypoll send refused; skipping rest of round",
				"backend", p.Backend, "uid", inv.UID)
			return nil
		}

		if i < len(inverters)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-yield:
				return nil
			case <-time.After(sendInterval):
			}
		}
	}
	return nil
}

// storeAdapter adapts *store.Store to the Store interface by delegating
// to the canonical store.ListInvertersForPoll query. Used by main.go
// via NewStoreAdapter.
type storeAdapter struct {
	st *store.Store
}

// NewStoreAdapter wraps a *store.Store so main.go can pass it to
// Poller.Store. The canonical SQL lives in store.ListInvertersForPoll;
// this adapter maps store.InverterPollRef → telemetrypoll.InverterRef
// at the boundary.
func NewStoreAdapter(st *store.Store) Store {
	return &storeAdapter{st: st}
}

// ListInvertersForPoll delegates to store.ListInvertersForPoll and maps
// store.InverterPollRef → InverterRef at the package boundary.
func (a *storeAdapter) ListInvertersForPoll(ctx context.Context) ([]InverterRef, error) {
	refs, err := a.st.ListInvertersForPoll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]InverterRef, len(refs))
	for i, r := range refs {
		out[i] = InverterRef{UID: r.UID, ShortAddr: r.ShortAddr}
	}
	return out, nil
}

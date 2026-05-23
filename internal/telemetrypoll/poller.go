// Package telemetrypoll originates periodic 0xBB telemetry queries to
// every inverter in inv-driver's own state.db inventory. Replies flow
// back through the normal RawFrame ingest path; only the outbound
// cadence lives here.
package telemetrypoll

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
)

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

	// cold-start: log "no inverters" once, not every round.
	emptyOnce sync.Once
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
				log.Printf("telemetrypoll: %v", err)
			}
			elapsed := time.Since(start)
			if elapsed > interval {
				log.Printf("telemetrypoll: WARN round took %s, exceeds interval %s; effective cadence degraded",
					elapsed.Round(time.Millisecond), interval)
			}
		}
	}
}

// poll executes one round: enumerate inverters, send one 0xBB query
// each. Returns nil on normal completion (including an empty inventory
// or an absent backend).
func (p *Poller) poll(ctx context.Context) error {
	if p.Store == nil || p.Server == nil || p.Backend == "" {
		return fmt.Errorf("Store, Server, and Backend must be set")
	}
	inverters, err := p.Store.ListInvertersForPoll(ctx)
	if err != nil {
		return fmt.Errorf("list inverters: %w", err)
	}
	if len(inverters) == 0 {
		p.emptyOnce.Do(func() {
			log.Printf("telemetrypoll: no inverters in state.db yet; telemetry seeds passively from observed replies")
		})
		return nil
	}

	sendInterval := p.SendInterval
	if sendInterval == 0 {
		sendInterval = 200 * time.Millisecond
	}

	frame := codec.OutboundBBQueryL2()

	for i, inv := range inverters {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
			log.Printf("telemetrypoll: backend=%q send refused at uid=%s; skipping rest of round",
				p.Backend, inv.UID)
			return nil
		}

		if i < len(inverters)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
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

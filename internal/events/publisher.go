// Package events implements an in-process broadcaster that fans
// inbound wire envelopes out to UDS subscribers. The driver decodes
// each upstream frame once, and any number of subscriber processes
// consume the same Envelope stream.
//
// Slow subscribers are dropped — see Publish. Ingest never blocks on
// a stuck reader.
package events

import (
	"log"
	"sync"

	"github.com/bolke/inv-driver/wire"
)

// chanCap is the per-subscriber buffer. Telemetry flows at <1 Hz per
// inverter so 64 leaves room for a brief stall before eviction.
const chanCap = 64

// Publisher is a thread-safe fan-out broadcaster for wire envelopes.
// Zero value is not ready; use New.
type Publisher struct {
	mu   sync.RWMutex
	subs map[*subscriber]struct{}
}

type subscriber struct {
	ch chan *wire.Envelope
}

// New constructs an empty Publisher.
func New() *Publisher {
	return &Publisher{subs: make(map[*subscriber]struct{})}
}

// Subscribe registers a new subscriber. Returned channel is buffered
// (cap chanCap); unsubscribe closes it and removes the subscriber.
// Calling unsubscribe more than once is safe (it no-ops after the
// first call).
func (p *Publisher) Subscribe() (<-chan *wire.Envelope, func()) {
	s := &subscriber{ch: make(chan *wire.Envelope, chanCap)}
	p.mu.Lock()
	p.subs[s] = struct{}{}
	p.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() { p.remove(s) })
	}
	return s.ch, unsubscribe
}

// Publish delivers env to every live subscriber non-blockingly. A
// subscriber whose channel is full is evicted (channel closed, removed
// from the fan-out) and the event is logged. Ordering across
// concurrent Publish calls is unspecified.
func (p *Publisher) Publish(env *wire.Envelope) {
	if env == nil {
		return
	}
	// Two phases: collect a snapshot of slow subscribers under RLock,
	// then evict under Lock. Keeps Publish lock-free vs other concurrent
	// Publishes while still ensuring the fan-out remains consistent.
	var slow []*subscriber
	p.mu.RLock()
	for s := range p.subs {
		select {
		case s.ch <- env:
		default:
			slow = append(slow, s)
		}
	}
	p.mu.RUnlock()

	if len(slow) == 0 {
		return
	}
	p.mu.Lock()
	for _, s := range slow {
		if _, ok := p.subs[s]; !ok {
			continue
		}
		delete(p.subs, s)
		close(s.ch)
	}
	p.mu.Unlock()
	log.Printf("events: evicted %d slow subscriber(s)", len(slow))
}

func (p *Publisher) remove(s *subscriber) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.subs[s]; !ok {
		return
	}
	delete(p.subs, s)
	close(s.ch)
}

// Len returns the current subscriber count. Useful for tests.
func (p *Publisher) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.subs)
}

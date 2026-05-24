package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// sseClient is one connected browser EventSource.
type sseClient struct {
	ch chan []byte
}

// Hub fans pre-serialised SSE events out to connected browsers. Fleet
// updates are coalesced: the snapshot signals "dirty" on every applied
// envelope, but the publisher loop emits at most one fleet frame per
// minInterval, so a burst of telemetry doesn't translate into a burst of
// pushes.
type Hub struct {
	minInterval time.Duration
	render      func() []byte // produces the current "fleet" event payload

	mu      sync.RWMutex
	clients map[*sseClient]struct{}

	dirty chan struct{}
}

// newHub builds a Hub. render returns the JSON body for a fleet event.
func newHub(minInterval time.Duration, render func() []byte) *Hub {
	return &Hub{
		minInterval: minInterval,
		render:      render,
		clients:     make(map[*sseClient]struct{}),
		dirty:       make(chan struct{}, 1),
	}
}

// MarkDirty signals that the snapshot changed. Non-blocking and
// coalescing: many calls between publishes collapse into one.
func (h *Hub) MarkDirty() {
	select {
	case h.dirty <- struct{}{}:
	default:
	}
}

// run is the publisher loop. It emits a fleet frame when dirty, rate-
// limited to one per minInterval. Exits on ctx cancel.
func (h *Hub) run(ctx context.Context) {
	t := time.NewTicker(h.minInterval)
	defer t.Stop()
	var pending bool
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.dirty:
			pending = true
		case <-t.C:
			if pending {
				pending = false
				h.broadcast("fleet", h.render())
			}
		}
	}
}

// broadcast writes one named SSE event to every client. Slow clients
// that can't keep up are dropped (their channel is full).
func (h *Hub) broadcast(event string, data []byte) {
	frame := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data))
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.ch <- frame:
		default:
		}
	}
}

func (h *Hub) add() *sseClient {
	c := &sseClient{ch: make(chan []byte, 8)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func (h *Hub) remove(c *sseClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// clientCount reports the number of connected SSE clients.
func (h *Hub) clientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeHTTP streams events to one browser. The first frame is the
// current fleet snapshot so a freshly-connected page renders without
// waiting for the next change.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	c := h.add()
	defer h.remove(c)

	// Prime with the current state.
	initial := []byte(fmt.Sprintf("event: fleet\ndata: %s\n\n", h.render()))
	if _, err := w.Write(initial); err != nil {
		return
	}
	flusher.Flush()

	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case frame := <-c.ch:
			if _, err := w.Write(frame); err != nil {
				return
			}
			flusher.Flush()
		case <-keepalive.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

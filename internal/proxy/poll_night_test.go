package proxy

import (
	"testing"
	"time"
)

// TestBusTrackerHook_PendingBoundedDuringSilence simulates night-time:
// we keep injecting (markPending mine=true), no inbound chunks ever
// come back. Without gc-on-add, the pending FIFO would grow by one
// per call — confirm it stays small (≤2 per SA at 1 Hz with default
// 1.5s TTL).
func TestBusTrackerHook_PendingBoundedDuringSilence(t *testing.T) {
	h := &BusTrackerHook{
		ResponseTTL:     1500 * time.Millisecond,
		DroppingTimeout: 2 * time.Second,
		pending:         make(map[uint16][]pendingEntry),
	}

	// Simulate 30 rapid markPending calls with no replies.
	for i := 0; i < 30; i++ {
		h.markPending(0x5011, true)
		// Backdate the entry we just inserted so subsequent gc
		// considers it expired.
		h.mu.Lock()
		q := h.pending[0x5011]
		for j := range q[:len(q)-1] {
			q[j].deadline = time.Now().Add(-time.Millisecond)
		}
		h.mu.Unlock()
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	got := len(h.pending[0x5011])
	if got > 2 {
		t.Fatalf("pending grew to %d entries during silence; want ≤2 with gc-on-add", got)
	}
}

// TestBusTrackerHook_OutboundChunkAlsoTriggersGC verifies main.exe's
// observed outbound queries get gc'd too.
func TestBusTrackerHook_OutboundChunkAlsoTriggersGC(t *testing.T) {
	h := &BusTrackerHook{
		ResponseTTL:     1 * time.Millisecond,
		DroppingTimeout: 2 * time.Second,
		pending: map[uint16][]pendingEntry{
			0x5011: {{
				issuedAt: time.Now().Add(-time.Hour),
				deadline: time.Now().Add(-time.Hour),
				mine:     true,
			}},
		},
	}

	// Outbound chunk for SA 0x5011 (main.exe's own query) — should
	// trigger gc and drop the stale entry, then add the new theirs
	// entry.
	frame := BuildBBQueryFrame(0x5011)
	a := h.OnChunk(DirToModem, frame)
	if a.Drop || a.Mine {
		t.Fatalf("outbound never drops: %+v", a)
	}
	h.mu.Lock()
	q := h.pending[0x5011]
	h.mu.Unlock()
	if len(q) != 1 || q[0].mine {
		t.Fatalf("expected 1 fresh theirs entry, got %d entries (first.mine=%v)",
			len(q), len(q) > 0 && q[0].mine)
	}
}

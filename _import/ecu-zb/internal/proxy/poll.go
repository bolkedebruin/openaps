package proxy

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bolke/inv-driver/codec"
)

// BusTrackerHook correlates outbound inverter queries (from any host
// path or inv-driver's Send path) with their inbound L1 replies. It
// suppresses replies to inv-driver-originated queries from the host
// stream and mirrors every reassembled L1 frame to RawReplySink.
//
// # Connection tracking
//
// The hook keeps a per-short-address FIFO of pending queries. Each
// entry remembers whether *we* injected the query (Mine=true) or
// whether the host sent it (Mine=false). When an L1 reply arrives we
// pop the oldest entry for that SA — that's the matching query, by
// the modem's per-SA serialisation contract. The reply is dropped from
// the host stream iff the popped entry was Mine. Either way the
// fully reassembled frame is handed to RawReplySink so upstream sees
// every reply.
//
// This handles the ugly cases the original counter-based design
// could not:
//
//   - Same-SA collision: if the host queries SA=X within ResponseTTL
//     of our query for the same SA, both sit in the FIFO. The first
//     reply pops the oldest, which is whichever was sent first. The
//     other side's reply pops the remaining entry on the next call.
//
//   - Ack-stealing: there is no `expectedAcks` counter; the inverter-
//     query path doesn't get `AB CD EF` modem acks anyway (only
//     modem-config ops 0x05/0x0D do). Acks pass through untouched.
//
//   - Multi-chunk reassembly: a single per-reply state captures the
//     mine flag at the start chunk and accumulates bytes until a
//     chunk ends with `FE FE`. We use strict `endsWithFEFE` (not
//     `containsFEFE`) to avoid a stray 0xFE 0xFE inside a reply
//     payload terminating reassembly early. `reassemblyDeadline` is
//     a hard timeout (default 2s) so a malformed reply can't wedge
//     the host stream forever.
type BusTrackerHook struct {
	// ResponseTTL: how long an entry stays in `pending` before being
	// gc'd as a no-reply. Default 1500ms.
	ResponseTTL time.Duration

	// DroppingTimeout: hard cap on multi-chunk reassembly. If we
	// don't see `FE FE` within this window after reassembly starts,
	// we force-clear and stop reassembling. Default 2s.
	DroppingTimeout time.Duration

	// RawReplySink, if non-nil, is called with each fully reassembled
	// L1 reply frame and the matched short address regardless of
	// whether the originating query was ours or the host's. The hook
	// owns reassembly only — decoding lives downstream (inv-driver).
	// If nil, replies are not dispatched.
	//
	// Grid-protection read queries are NOT injected here: inv-driver
	// drives those on-demand (first-seen + after a protection write) via
	// Send, and their replies flow back through this same sink.
	RawReplySink func(shortAddr uint16, l1 []byte)

	mu      sync.Mutex
	pending map[uint16][]pendingEntry

	// reassembling state for the in-flight inbound reply. When buf is
	// non-nil we're between an SOF chunk and a chunk that ends with
	// FE FE. mine determines whether host-side bytes are dropped; the
	// sink fires either way once reassembly completes.
	reassembling       bool
	reassemblyMine     bool
	reassemblyBuf      []byte
	reassemblyDeadline time.Time

	injMu sync.Mutex
	inj   Injector
}

// pendingEntry is one outstanding query (ours or the host's).
type pendingEntry struct {
	issuedAt time.Time
	deadline time.Time
	mine     bool
}

// NewBusTrackerHook returns a BusTrackerHook with sane defaults.
// The hook is passive — it does not launch a self-driven poll loop.
// Call Start(inj) to register the injector for InjectTracked.
func NewBusTrackerHook() *BusTrackerHook {
	return &BusTrackerHook{
		ResponseTTL:     1500 * time.Millisecond,
		DroppingTimeout: 2 * time.Second,
		pending:         make(map[uint16][]pendingEntry),
	}
}

// Start registers the injector for subsequent InjectTracked calls.
// It does not launch a poll loop — polling is driven externally by
// inv-driver's Send path via InjectTracked.
func (h *BusTrackerHook) Start(inj Injector) error {
	if h.pending == nil {
		h.pending = make(map[uint16][]pendingEntry)
	}
	if h.ResponseTTL <= 0 {
		h.ResponseTTL = 1500 * time.Millisecond
	}
	if h.DroppingTimeout <= 0 {
		h.DroppingTimeout = 2 * time.Second
	}
	h.injMu.Lock()
	h.inj = inj
	h.injMu.Unlock()
	log.Printf("BusTrackerHook: started (passive, reply-suppression only)")
	return nil
}

// Stop is a no-op — there is no self-driven poll goroutine to stop.
func (h *BusTrackerHook) Stop() {}

// markPending appends a new outstanding-query entry. mine=true for
// our injections, mine=false for the host's outbound queries we
// observed via OnChunk.
//
// Calls gcExpiredLocked first so the per-SA FIFO stays bounded even
// when the wire is silent (inverters offline at night). Without
// that, our polling accumulates entries and the pending map drifts
// toward MB scale by morning.
func (h *BusTrackerHook) markPending(sa uint16, mine bool) {
	now := time.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.gcExpiredLocked()
	h.pending[sa] = append(h.pending[sa], pendingEntry{
		issuedAt: now,
		deadline: now.Add(h.ResponseTTL),
		mine:     mine,
	})
}

// popMinePending undoes the most recent mine=true append for SA.
// Used when an InjectToModem write fails so we don't leave a dangling
// "expecting our reply" entry.
func (h *BusTrackerHook) popMinePending(sa uint16) {
	h.mu.Lock()
	defer h.mu.Unlock()
	q := h.pending[sa]
	for i := len(q) - 1; i >= 0; i-- {
		if q[i].mine {
			h.pending[sa] = append(q[:i], q[i+1:]...)
			return
		}
	}
}

// gcExpiredLocked removes pending entries past their deadline. Caller
// holds h.mu.
func (h *BusTrackerHook) gcExpiredLocked() {
	now := time.Now()
	for sa, entries := range h.pending {
		kept := entries[:0]
		for _, e := range entries {
			if now.Before(e.deadline) {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(h.pending, sa)
		} else {
			h.pending[sa] = kept
		}
	}
}

// OnChunk is called by Splice for every observed wire chunk.
func (h *BusTrackerHook) OnChunk(dir FrameDirection, raw []byte) ChunkAction {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Outbound (host→modem) flows through unchanged. We don't drop
	// here; we just record the host's L0 inverter queries so the
	// inbound matcher knows whose reply to expect. gc here too so
	// silent nights don't leak.
	if dir == DirToModem {
		if sa, ok := parseOutboundQuerySA(raw); ok {
			now := time.Now()
			h.gcExpiredLocked()
			h.pending[sa] = append(h.pending[sa], pendingEntry{
				issuedAt: now,
				deadline: now.Add(h.ResponseTTL),
				mine:     false,
			})
		}
		return ChunkAction{}
	}

	// Inbound: a new SOF (FC FC <SA hi> <SA lo>) always (re)starts
	// reassembly. Any in-flight buffer is discarded — the prior
	// reassembly didn't complete cleanly and the new chunk is the
	// definitive start of a fresh L1 frame.
	if isL1SOF(raw) {
		return h.startReassemblyLocked(raw)
	}

	// Mid-reassembly continuation.
	if h.reassembling {
		if time.Now().After(h.reassemblyDeadline) {
			// Wedged: discard buffered bytes, clear state. The
			// current chunk's host-side disposition stays as it
			// would have been for this reassembly so the host sees
			// a consistent end to the prior reassembled frame.
			mine := h.reassemblyMine
			h.reassembling = false
			h.reassemblyBuf = h.reassemblyBuf[:0]
			return ChunkAction{Drop: mine, Mine: mine}
		}
		h.reassemblyBuf = append(h.reassemblyBuf, raw...)
		mine := h.reassemblyMine
		if endsWithFEFE(raw) {
			h.dispatchReplyLocked()
		}
		return ChunkAction{Drop: mine, Mine: mine}
	}

	return ChunkAction{}
}

// startReassemblyLocked begins a new inbound L1 reassembly from an SOF
// chunk. Pops the oldest pending entry for the SA to determine mine;
// no match counts as mine=false (orphan). Caller holds h.mu.
func (h *BusTrackerHook) startReassemblyLocked(raw []byte) ChunkAction {
	h.gcExpiredLocked()
	sa := uint16(raw[2])<<8 | uint16(raw[3])
	entry, ok := h.popOldestForLocked(sa)
	mine := ok && entry.mine

	h.reassembling = true
	h.reassemblyMine = mine
	h.reassemblyBuf = append(h.reassemblyBuf[:0], raw...)
	h.reassemblyDeadline = time.Now().Add(h.DroppingTimeout)

	if endsWithFEFE(raw) {
		h.dispatchReplyLocked()
	}
	return ChunkAction{Drop: mine, Mine: mine}
}

// dispatchReplyLocked hands the assembled reply to RawReplySink and
// resets reassembly state. The short address is recovered from the L1
// header (bytes 2..3) so we don't have to thread it through the FIFO
// matcher. Fires for both mine=true and mine=false reassemblies.
// Caller holds h.mu.
func (h *BusTrackerHook) dispatchReplyLocked() {
	defer func() {
		h.reassembling = false
		h.reassemblyBuf = h.reassemblyBuf[:0]
	}()
	if len(h.reassemblyBuf) < 4 {
		return
	}
	frame := append([]byte(nil), h.reassemblyBuf...)
	sa := uint16(frame[2])<<8 | uint16(frame[3])

	sink := h.RawReplySink
	if sink == nil {
		return
	}
	// Sink may block (UDS enqueue under back-pressure); run off the
	// chunk path so we don't stall other inbound bytes.
	go sink(sa, frame)
}

// isL1SOF reports whether raw begins with the L1 reply marker plus
// short-address bytes.
func isL1SOF(raw []byte) bool {
	return len(raw) >= 4 && raw[0] == codec.L1ReplySOF && raw[1] == codec.L1ReplySOF
}

// popOldestForLocked removes and returns the oldest pending entry for
// SA, or (zero, false) if none. Caller holds h.mu.
func (h *BusTrackerHook) popOldestForLocked(sa uint16) (pendingEntry, bool) {
	q := h.pending[sa]
	if len(q) == 0 {
		return pendingEntry{}, false
	}
	entry := q[0]
	if len(q) == 1 {
		delete(h.pending, sa)
	} else {
		h.pending[sa] = q[1:]
	}
	return entry, true
}

func endsWithFEFE(b []byte) bool {
	return len(b) >= 2 && b[len(b)-2] == codec.L2EOF1 && b[len(b)-1] == codec.L2EOF2
}

// parseOutboundQuerySA extracts the inverter short-address from an
// L0 inverter-cmd frame: AA AA AA AA 55 <SA hi> <SA lo>. Returns
// false if the chunk doesn't look like a 0x55-op outbound query
// (modem-init ops 0x05/0x0D have no SA).
func parseOutboundQuerySA(raw []byte) (uint16, bool) {
	if !IsOutboundInverterCmdPrefix(raw) {
		return 0, false
	}
	return uint16(raw[5])<<8 | uint16(raw[6]), true
}

// IsOutboundInverterCmdPrefix reports whether raw begins with the
// L1 outbound preamble for an inverter-cmd envelope:
//
//	AA AA AA AA 55 <SA hi> <SA lo>
//
// Returns false if raw is too short or the prefix doesn't match.
// Mirrors the prefix check the busmgr frame validator applies.
func IsOutboundInverterCmdPrefix(raw []byte) bool {
	if len(raw) < 7 {
		return false
	}
	if raw[0] != codec.L1Preamble || raw[1] != codec.L1Preamble || raw[2] != codec.L1Preamble || raw[3] != codec.L1Preamble {
		return false
	}
	return raw[4] == codec.L1Sentinel
}

// InjectTracked writes frame to the modem via the stored Injector and
// records a Mine=true pending entry keyed by the short-address parsed
// out of frame. The matching inbound reply is dropped from the host
// stream and dispatched to RawReplySink.
//
// frame must be a standard inverter-query L0 envelope of the form
// `AA AA AA AA 55 <SA hi> <SA lo> ...`. Any other shape returns an
// error and no injection happens.
func (h *BusTrackerHook) InjectTracked(frame []byte) error {
	sa, ok := parseOutboundQuerySA(frame)
	if !ok {
		return fmt.Errorf("BusTrackerHook.InjectTracked: frame is not an L0 inverter-cmd envelope")
	}
	h.injMu.Lock()
	inj := h.inj
	h.injMu.Unlock()
	if inj == nil {
		return fmt.Errorf("BusTrackerHook.InjectTracked: injector not configured (Start not called)")
	}
	// SA=0 is a broadcast: no inverter reply is expected, so we don't
	// register a pending entry. Without this skip every broadcast would
	// occupy a pending slot until ResponseTTL expires and bloat the FIFO.
	broadcast := sa == 0
	if !broadcast {
		h.markPending(sa, true)
	}
	if err := inj.InjectToModem(frame); err != nil {
		if !broadcast {
			h.popMinePending(sa)
		}
		return fmt.Errorf("BusTrackerHook.InjectTracked: SA=0x%04X: %w", sa, err)
	}
	return nil
}


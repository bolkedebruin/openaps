package proxy

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/zigbee/busmgr"
	"github.com/bolkedebruin/openaps/codec"
)

// BuildBBQueryFrame returns the 28-byte L0+L2 BB query frame for the
// given inverter short-address. Test-only helper: production code must
// not carry a query-frame builder in poll.go.
func BuildBBQueryFrame(sa uint16) []byte {
	return busmgr.BuildL1Frame(sa, codec.OutboundBBQueryL2())
}

// fakeInjector records every InjectToModem so the test can inspect
// what the hook sent.
type fakeInjector struct {
	mu     sync.Mutex
	frames [][]byte
	err    error
}

func (f *fakeInjector) InjectToModem(raw []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.frames = append(f.frames, append([]byte(nil), raw...))
	return nil
}

func (f *fakeInjector) snapshot() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]byte, len(f.frames))
	copy(out, f.frames)
	return out
}

func TestBuildBBQueryFrame_MatchesHistoricalCapture(t *testing.T) {
	// From zigbee-tap/zb-tap.log historical capture for SA=0x61F0:
	//
	//   aa aa aa aa 55 61 f0 00 00 00 00 00 01 a6 0d
	//   fb fb 06 bb 00 00 00 00 00 00 c1 fe fe
	want := []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0x55, 0x61, 0xF0,
		0x00, 0x00, 0x00, 0x00, 0x00,
		0x01, 0xA6,
		0x0D,
		0xFB, 0xFB, 0x06, 0xBB,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xC1,
		0xFE, 0xFE,
	}
	got := BuildBBQueryFrame(0x61F0)
	if !bytes.Equal(got, want) {
		t.Fatalf("frame mismatch:\n  got  %x\n  want %x", got, want)
	}
	if len(got) != 28 {
		t.Fatalf("unexpected length %d (want 28)", len(got))
	}
}

func TestBuildBBQueryFrame_ChecksumVariesBySA(t *testing.T) {
	got := BuildBBQueryFrame(0x5011)
	if got[12] != 0x00 || got[13] != 0xB6 {
		t.Fatalf("SA=0x5011 chk: got %02x %02x want 00 B6", got[12], got[13])
	}
	got = BuildBBQueryFrame(0xC459)
	if got[12] != 0x01 || got[13] != 0x72 {
		t.Fatalf("SA=0xC459 chk: got %02x %02x want 01 72", got[12], got[13])
	}
}

func newTestHook() *BusTrackerHook {
	return &BusTrackerHook{
		ResponseTTL:     time.Second,
		DroppingTimeout: 2 * time.Second,
		pending:         make(map[uint16][]pendingEntry),
	}
}

func TestParseOutboundQuerySA(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
		sa   uint16
		ok   bool
	}{
		{"valid 0x55 query", []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x55, 0x61, 0xF0, 0, 0, 0, 0, 0}, 0x61F0, true},
		{"modem init 0x05", []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x05, 0, 0}, 0, false},
		{"modem config 0x0D", []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x0D, 0, 0}, 0, false},
		{"too short", []byte{0xAA, 0xAA, 0xAA}, 0, false},
		{"wrong preamble", []byte{0xBB, 0xBB, 0xBB, 0xBB, 0x55, 0x12, 0x34}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sa, ok := parseOutboundQuerySA(c.raw)
			if ok != c.ok || sa != c.sa {
				t.Fatalf("got (0x%X, %v), want (0x%X, %v)", sa, ok, c.sa, c.ok)
			}
		})
	}
}

func TestBusTrackerHook_OutboundMainExeQueryRecorded(t *testing.T) {
	h := newTestHook()
	frame := BuildBBQueryFrame(0x5011) // identical to main.exe's BB query for SA=0x5011

	a := h.OnChunk(DirToModem, frame)
	if a.Drop || a.Mine {
		t.Fatalf("outbound never drops or claims Mine here: %+v", a)
	}
	if len(h.pending[0x5011]) != 1 {
		t.Fatalf("expected 1 theirs entry for SA=0x5011, got %d", len(h.pending[0x5011]))
	}
	if h.pending[0x5011][0].mine {
		t.Fatal("entry should be theirs, not mine")
	}
}

func TestBusTrackerHook_FIFO_OursFirst_TheirsSecond(t *testing.T) {
	// Ours queued first, theirs second. First reply pops ours (drop+Mine);
	// second reply pops theirs (forward).
	h := newTestHook()
	now := time.Now()
	h.pending[0x5011] = []pendingEntry{
		{issuedAt: now, deadline: now.Add(time.Second), mine: true},
		{issuedAt: now.Add(time.Millisecond), deadline: now.Add(time.Second), mine: false},
	}
	reply := []byte{0xFC, 0xFC, 0x50, 0x11, 0xCA, 0xFE, 0xFE, 0xFE} // ends FE FE
	a := h.OnChunk(DirToHost, reply)
	if !a.Drop || !a.Mine {
		t.Fatalf("first reply for SA should be ours: %+v", a)
	}
	if len(h.pending[0x5011]) != 1 {
		t.Fatalf("FIFO should retain 1 entry, got %d", len(h.pending[0x5011]))
	}

	a = h.OnChunk(DirToHost, reply)
	if a.Drop || a.Mine {
		t.Fatalf("second reply for SA should be main.exe's: %+v", a)
	}
	if _, ok := h.pending[0x5011]; ok {
		t.Fatal("FIFO should be empty after second reply")
	}
}

func TestBusTrackerHook_FIFO_TheirsFirst_OursSecond(t *testing.T) {
	// Their query was already in flight when ours arrived. The first
	// reply is theirs (forward), the second is ours (drop+Mine).
	h := newTestHook()
	now := time.Now()
	h.pending[0x5011] = []pendingEntry{
		{issuedAt: now, deadline: now.Add(time.Second), mine: false},
		{issuedAt: now.Add(time.Millisecond), deadline: now.Add(time.Second), mine: true},
	}
	reply := []byte{0xFC, 0xFC, 0x50, 0x11, 0xCA, 0xFE, 0xFE, 0xFE}
	a := h.OnChunk(DirToHost, reply)
	if a.Drop || a.Mine {
		t.Fatalf("first reply should be theirs (forward): %+v", a)
	}

	a = h.OnChunk(DirToHost, reply)
	if !a.Drop || !a.Mine {
		t.Fatalf("second reply should be ours (drop+Mine): %+v", a)
	}
}

func TestBusTrackerHook_OrphanReply_ForwardedAndNotMine(t *testing.T) {
	h := newTestHook()
	reply := []byte{0xFC, 0xFC, 0xDE, 0xAD, 0xFE, 0xFE}
	a := h.OnChunk(DirToHost, reply)
	if a.Drop || a.Mine {
		t.Fatalf("orphan reply must pass through cleanly: %+v", a)
	}
}

func TestBusTrackerHook_MultiChunkDropEndsOnFEFEAtChunkEnd(t *testing.T) {
	h := newTestHook()
	now := time.Now()
	h.pending[0xC459] = []pendingEntry{{issuedAt: now, deadline: now.Add(time.Second), mine: true}}

	first := []byte{0xFC, 0xFC, 0xC4, 0x59, 0xFE, 0xFE, 0xAA, 0xBB}
	a := h.OnChunk(DirToHost, first)
	if !a.Drop || !a.Mine {
		t.Fatalf("first chunk should drop+Mine: %+v", a)
	}
	// FE FE present mid-chunk but NOT at end → reassembly must remain
	// active.
	if !h.reassembling {
		t.Fatal("expected reassembling=true since chunk doesn't END with FE FE")
	}

	mid := []byte{0xCC, 0xDD}
	a = h.OnChunk(DirToHost, mid)
	if !a.Drop || !a.Mine {
		t.Fatalf("continuation should drop+Mine: %+v", a)
	}
	if !h.reassembling {
		t.Fatal("reassembling must persist until a chunk actually ends with FE FE")
	}

	last := []byte{0x11, 0x22, 0xFE, 0xFE}
	a = h.OnChunk(DirToHost, last)
	if !a.Drop || !a.Mine {
		t.Fatalf("final chunk should drop+Mine: %+v", a)
	}
	if h.reassembling {
		t.Fatal("reassembling should clear when chunk ends with FE FE")
	}
}

func TestBusTrackerHook_DroppingTimeout_PreventsWedge(t *testing.T) {
	h := &BusTrackerHook{
		ResponseTTL:        time.Second,
		DroppingTimeout:    20 * time.Millisecond, // very short for the test
		pending:            make(map[uint16][]pendingEntry),
		reassembling:       true,
		reassemblyMine:     true,
		reassemblyDeadline: time.Now().Add(-10 * time.Millisecond), // already past
	}
	// First inbound chunk after expiry → still drops THIS chunk (the
	// wedge was mine=true) but clears reassembly so subsequent chunks
	// pass.
	a := h.OnChunk(DirToHost, []byte{0xCC, 0xDD})
	if !a.Drop {
		t.Fatal("the wedged chunk itself is still dropped")
	}
	if h.reassembling {
		t.Fatal("reassembly deadline should have cleared the state")
	}
	a = h.OnChunk(DirToHost, []byte{0xFC, 0xFC, 0xDE, 0xAD, 0xFE, 0xFE})
	if a.Drop || a.Mine {
		t.Fatalf("after timeout, unrelated reply must pass through: %+v", a)
	}
}

func TestBusTrackerHook_ExpiredPendingPassesThrough(t *testing.T) {
	h := newTestHook()
	now := time.Now()
	// Past-deadline entry: gc should remove it before the SA lookup.
	h.pending[0x5011] = []pendingEntry{{
		issuedAt: now.Add(-2 * time.Second),
		deadline: now.Add(-time.Second),
		mine:     true,
	}}
	reply := []byte{0xFC, 0xFC, 0x50, 0x11, 0xFE, 0xFE}
	a := h.OnChunk(DirToHost, reply)
	if a.Drop || a.Mine {
		t.Fatalf("expired pending should be gc'd; reply forwards: %+v", a)
	}
}

func TestBusTrackerHook_PopMinePending_OnFailedInject(t *testing.T) {
	h := newTestHook()
	h.markPending(0x5011, true)
	h.markPending(0x5011, false)
	h.markPending(0x5011, true)
	if got := len(h.pending[0x5011]); got != 3 {
		t.Fatalf("expected 3 entries, got %d", got)
	}
	h.popMinePending(0x5011)
	q := h.pending[0x5011]
	if len(q) != 2 {
		t.Fatalf("expected 2 entries after pop, got %d", len(q))
	}
	// Should have popped the most recent mine=true (the third), leaving
	// [mine=true, mine=false] in that order.
	if !q[0].mine || q[1].mine {
		t.Fatalf("unexpected entries after pop: %+v", q)
	}
}

func TestBusTrackerHook_AcksAlwaysPassThrough(t *testing.T) {
	// The inverter-query path doesn't get AB CD EF acks — we no
	// longer chase them. Confirm acks pass through unchanged even
	// when there's a mine entry pending.
	h := newTestHook()
	h.markPending(0x5011, true)
	a := h.OnChunk(DirToHost, []byte{0xAB, 0xCD, 0xEF})
	if a.Drop || a.Mine {
		t.Fatalf("ack must always pass through: %+v", a)
	}
}

func TestBusTrackerHook_OutboundChunkNotDropped(t *testing.T) {
	h := newTestHook()
	h.markPending(0x5011, true)
	out := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x55, 0x50, 0x11, 0, 0, 0, 0, 0}
	a := h.OnChunk(DirToModem, out)
	if a.Drop || a.Mine {
		t.Fatalf("outbound never dropped: %+v", a)
	}
}

// sinkRecorder captures every reply the hook hands to RawReplySink.
type sinkRecorder struct {
	mu      sync.Mutex
	frames  [][]byte
	addrs   []uint16
	doneCh  chan struct{}
	wantCnt int
}

func newSinkRecorder(want int) *sinkRecorder {
	return &sinkRecorder{doneCh: make(chan struct{}), wantCnt: want}
}

func (s *sinkRecorder) sink(sa uint16, l1 []byte) {
	s.mu.Lock()
	s.frames = append(s.frames, append([]byte(nil), l1...))
	s.addrs = append(s.addrs, sa)
	if len(s.frames) >= s.wantCnt {
		select {
		case <-s.doneCh:
		default:
			close(s.doneCh)
		}
	}
	s.mu.Unlock()
}

func (s *sinkRecorder) wait(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case <-s.doneCh:
	case <-time.After(d):
		s.mu.Lock()
		got := len(s.frames)
		s.mu.Unlock()
		t.Fatalf("sink did not receive %d frames within %s (got %d)", s.wantCnt, d, got)
	}
}

func (s *sinkRecorder) snapshot() ([]uint16, [][]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	addrs := append([]uint16(nil), s.addrs...)
	frames := make([][]byte, len(s.frames))
	for i, f := range s.frames {
		frames[i] = append([]byte(nil), f...)
	}
	return addrs, frames
}

func TestBusTrackerHook_SinkDispatchesMineSingleChunk(t *testing.T) {
	rec := newSinkRecorder(1)
	h := newTestHook()
	h.RawReplySink = rec.sink
	now := time.Now()
	h.pending[0x5011] = []pendingEntry{{issuedAt: now, deadline: now.Add(time.Second), mine: true}}

	reply := []byte{0xFC, 0xFC, 0x50, 0x11, 0xCA, 0xFE, 0xFE}
	a := h.OnChunk(DirToHost, reply)
	if !a.Drop || !a.Mine {
		t.Fatalf("mine reply must drop+Mine: %+v", a)
	}
	rec.wait(t, time.Second)
	addrs, frames := rec.snapshot()
	if len(frames) != 1 || addrs[0] != 0x5011 || !bytes.Equal(frames[0], reply) {
		t.Fatalf("sink mismatch: addrs=%v frames=%x", addrs, frames)
	}
}

func TestBusTrackerHook_SinkDispatchesMineMultiChunk(t *testing.T) {
	rec := newSinkRecorder(1)
	h := newTestHook()
	h.RawReplySink = rec.sink
	now := time.Now()
	h.pending[0xC459] = []pendingEntry{{issuedAt: now, deadline: now.Add(time.Second), mine: true}}

	parts := [][]byte{
		{0xFC, 0xFC, 0xC4, 0x59, 0xAA},
		{0xBB, 0xCC},
		{0xDD, 0xFE, 0xFE},
	}
	for _, p := range parts {
		a := h.OnChunk(DirToHost, p)
		if !a.Drop || !a.Mine {
			t.Fatalf("mine multi-chunk must drop+Mine each chunk: %+v on %x", a, p)
		}
	}
	rec.wait(t, time.Second)
	_, frames := rec.snapshot()
	want := []byte{0xFC, 0xFC, 0xC4, 0x59, 0xAA, 0xBB, 0xCC, 0xDD, 0xFE, 0xFE}
	if !bytes.Equal(frames[0], want) {
		t.Fatalf("reassembled frame mismatch:\n  got  %x\n  want %x", frames[0], want)
	}
}

func TestBusTrackerHook_SinkDispatchesTheirsSingleChunk(t *testing.T) {
	rec := newSinkRecorder(1)
	h := newTestHook()
	h.RawReplySink = rec.sink
	now := time.Now()
	h.pending[0x5011] = []pendingEntry{{issuedAt: now, deadline: now.Add(time.Second), mine: false}}

	reply := []byte{0xFC, 0xFC, 0x50, 0x11, 0x01, 0x02, 0xFE, 0xFE}
	a := h.OnChunk(DirToHost, reply)
	if a.Drop || a.Mine {
		t.Fatalf("theirs reply must pass through cleanly: %+v", a)
	}
	rec.wait(t, time.Second)
	addrs, frames := rec.snapshot()
	if len(frames) != 1 || addrs[0] != 0x5011 || !bytes.Equal(frames[0], reply) {
		t.Fatalf("sink mismatch: addrs=%v frames=%x", addrs, frames)
	}
}

func TestBusTrackerHook_SinkDispatchesTheirsMultiChunk(t *testing.T) {
	rec := newSinkRecorder(1)
	h := newTestHook()
	h.RawReplySink = rec.sink
	now := time.Now()
	h.pending[0x5011] = []pendingEntry{{issuedAt: now, deadline: now.Add(time.Second), mine: false}}

	parts := [][]byte{
		{0xFC, 0xFC, 0x50, 0x11, 0xDE},
		{0xAD, 0xBE, 0xEF},
		{0x42, 0xFE, 0xFE},
	}
	for _, p := range parts {
		a := h.OnChunk(DirToHost, p)
		if a.Drop || a.Mine {
			t.Fatalf("theirs continuation must pass through: %+v on %x", a, p)
		}
	}
	rec.wait(t, time.Second)
	_, frames := rec.snapshot()
	want := []byte{0xFC, 0xFC, 0x50, 0x11, 0xDE, 0xAD, 0xBE, 0xEF, 0x42, 0xFE, 0xFE}
	if !bytes.Equal(frames[0], want) {
		t.Fatalf("reassembled frame mismatch:\n  got  %x\n  want %x", frames[0], want)
	}
}

func TestBusTrackerHook_SinkDispatchesOrphan(t *testing.T) {
	rec := newSinkRecorder(1)
	h := newTestHook()
	h.RawReplySink = rec.sink

	reply := []byte{0xFC, 0xFC, 0xDE, 0xAD, 0x00, 0xFE, 0xFE}
	a := h.OnChunk(DirToHost, reply)
	if a.Drop || a.Mine {
		t.Fatalf("orphan reply must pass through: %+v", a)
	}
	rec.wait(t, time.Second)
	addrs, frames := rec.snapshot()
	if len(frames) != 1 || addrs[0] != 0xDEAD || !bytes.Equal(frames[0], reply) {
		t.Fatalf("sink mismatch: addrs=%v frames=%x", addrs, frames)
	}
}

func TestBusTrackerHook_MidReassemblyNewSOFRestarts(t *testing.T) {
	rec := newSinkRecorder(1)
	h := newTestHook()
	h.RawReplySink = rec.sink
	now := time.Now()
	// First reassembly is theirs; we never finish it (no FE FE chunk).
	h.pending[0x5011] = []pendingEntry{
		{issuedAt: now, deadline: now.Add(time.Second), mine: false},
		{issuedAt: now.Add(time.Millisecond), deadline: now.Add(time.Second), mine: true},
	}
	a := h.OnChunk(DirToHost, []byte{0xFC, 0xFC, 0x50, 0x11, 0xAA})
	if a.Drop || a.Mine {
		t.Fatalf("theirs SOF should pass through: %+v", a)
	}
	// Now a fresh SOF arrives before the prior frame terminated. This
	// is the mine=true entry. The hook should discard the old buffer
	// and start a new reassembly with the new mine flag.
	a = h.OnChunk(DirToHost, []byte{0xFC, 0xFC, 0x50, 0x11, 0xBB, 0xCC, 0xFE, 0xFE})
	if !a.Drop || !a.Mine {
		t.Fatalf("mine SOF should drop+Mine: %+v", a)
	}
	rec.wait(t, time.Second)
	_, frames := rec.snapshot()
	// Only the second reassembly dispatched; the first was abandoned.
	if len(frames) != 1 {
		t.Fatalf("want 1 dispatched frame, got %d", len(frames))
	}
	want := []byte{0xFC, 0xFC, 0x50, 0x11, 0xBB, 0xCC, 0xFE, 0xFE}
	if !bytes.Equal(frames[0], want) {
		t.Fatalf("dispatched frame mismatch:\n  got  %x\n  want %x", frames[0], want)
	}
}

func TestBusTrackerHook_InjectTracked_QueuesAndInjects(t *testing.T) {
	inj := &fakeInjector{}
	h := NewBusTrackerHook()
	if err := h.Start(inj); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Stop()

	// 28-byte info-query frame: same L0 envelope as BB query, the L2
	// payload is application-defined (0xDC in the real path). The
	// tracker only parses the L0 SA.
	frame := append([]byte(nil), BuildBBQueryFrame(0xC459)...)
	frame[16] = 0xDC // tweak L2 cmd byte — irrelevant to tracking, just for clarity.

	if err := h.InjectTracked(frame); err != nil {
		t.Fatalf("InjectTracked: %v", err)
	}

	// fakeInjector recorded the bytes.
	frames := inj.snapshot()
	if len(frames) != 1 {
		t.Fatalf("expected one injection, got %d", len(frames))
	}
	if !bytes.Equal(frames[0], frame) {
		t.Fatalf("injected bytes mismatch:\n  got  %x\n  want %x", frames[0], frame)
	}

	// SA 0xC459 is now pending with mine=true.
	h.mu.Lock()
	q := h.pending[0xC459]
	h.mu.Unlock()
	if len(q) == 0 {
		t.Fatal("expected pending entry for SA=0xC459")
	}
	// Must be a mine=true entry.
	if !q[len(q)-1].mine {
		t.Fatalf("expected last pending entry mine=true, got %+v", q[len(q)-1])
	}
}

func TestBusTrackerHook_InjectTracked_RejectsNonQuery(t *testing.T) {
	inj := &fakeInjector{}
	h := NewBusTrackerHook()
	if err := h.Start(inj); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Stop()

	bad := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x05, 0x00} // op=0x05 modem-init, not 0x55.
	if err := h.InjectTracked(bad); err == nil {
		t.Fatal("expected error for non-query frame, got nil")
	}
	if got := len(inj.snapshot()); got != 0 {
		t.Fatalf("no injection should have occurred, got %d", got)
	}
}

func TestBusTrackerHook_InjectTracked_BeforeStart(t *testing.T) {
	h := NewBusTrackerHook()
	frame := BuildBBQueryFrame(0x5011)
	if err := h.InjectTracked(frame); err == nil {
		t.Fatal("expected error when InjectTracked is called before Start, got nil")
	}
}

func TestBusTrackerHook_InjectTracked_PopsOnInjectError(t *testing.T) {
	inj := &fakeInjector{err: io.ErrClosedPipe}
	h := NewBusTrackerHook()
	if err := h.Start(inj); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Stop()

	frame := BuildBBQueryFrame(0xC459)
	h.mu.Lock()
	before := len(h.pending[0xC459])
	h.mu.Unlock()

	if err := h.InjectTracked(frame); err == nil {
		t.Fatal("expected error when injector errors, got nil")
	}
	h.mu.Lock()
	after := len(h.pending[0xC459])
	h.mu.Unlock()
	if after != before {
		t.Fatalf("expected pending count unchanged on inject error: before=%d after=%d", before, after)
	}
}

func TestBusTrackerHook_InjectTracked_BroadcastSkipsPending(t *testing.T) {
	inj := &fakeInjector{}
	h := NewBusTrackerHook()
	if err := h.Start(inj); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Stop()

	// Build a broadcast frame: SA=0 in the L0 envelope.
	frame := BuildBBQueryFrame(0x0000)
	if err := h.InjectTracked(frame); err != nil {
		t.Fatalf("InjectTracked broadcast: %v", err)
	}

	// SA=0 must not have created any pending entry.
	h.mu.Lock()
	defer h.mu.Unlock()
	if got := len(h.pending[0x0000]); got != 0 {
		t.Fatalf("SA=0 pending entries: got %d want 0", got)
	}
}

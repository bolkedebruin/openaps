package tap

import (
	"io"
	"sync"
	"time"
)

// Direction values match the pcap payload byte 0 convention used by
// zigbee-tap/socat_to_pcap.py and aps_zigbee.lua.
const (
	DirToModem byte = 0 // host → CC2530
	DirToHost  byte = 1 // CC2530 → host
)

// Broadcaster fans pcapng EPBs out to multiple subscribers. Each
// subscriber gets the SHB+IDB preamble at attach time and live frames
// from then on. Late subscribers do NOT receive backlog.
type Broadcaster struct {
	header []byte

	mu   sync.Mutex
	subs map[*subscriber]struct{}
}

type subscriber struct {
	ch     chan []byte
	closed bool
}

// NewBroadcaster builds the header preamble once and returns a ready
// broadcaster.
func NewBroadcaster(s SectionInfo) *Broadcaster {
	return &Broadcaster{
		header: EncodeHeader(s),
		subs:   make(map[*subscriber]struct{}),
	}
}

// Publish encodes one EPB on the wire interface (id=0) and queues it
// on every active subscriber. Slow subscribers drop frames rather
// than block the splice.
func (b *Broadcaster) Publish(dir byte, chunk []byte, ts time.Time) {
	b.PublishOn(IfaceWire, dir, chunk, ts)
}

// PublishOn is the iface-aware variant. Use IfaceInject for chunks
// the splice's hooks identified as belonging to ecu-zb's own
// injection cycle (queries + their replies).
func (b *Broadcaster) PublishOn(ifaceID uint32, dir byte, chunk []byte, ts time.Time) {
	if chunk == nil {
		return
	}
	block := EncodeEPB(ifaceID, dir, chunk, ts)
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs {
		select {
		case s.ch <- block:
		default:
			// consumer is too slow; drop this block for them only
		}
	}
}

// Attach connects w as a new subscriber. The returned done channel
// closes once the writer goroutine exits (either because w returned
// an error — typical on connection close — or because detach was
// called). Callers should `<-done` to learn when the subscriber went
// away, then defer detach() to release the slot in the broadcaster.
func (b *Broadcaster) Attach(w io.Writer) (detach func(), done <-chan struct{}) {
	sub := &subscriber{ch: make(chan []byte, 256)}

	b.mu.Lock()
	b.subs[sub] = struct{}{}
	b.mu.Unlock()

	// Send header as the first queued message. Doing it via the
	// channel (rather than a synchronous write) keeps frame ordering
	// clean if Publish raced our subscription add.
	header := append([]byte(nil), b.header...)
	sub.ch <- header

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		for buf := range sub.ch {
			if _, err := w.Write(buf); err != nil {
				return
			}
		}
	}()

	var once sync.Once
	detach = func() {
		once.Do(func() {
			b.mu.Lock()
			_, present := b.subs[sub]
			if present {
				delete(b.subs, sub)
				sub.closed = true
				close(sub.ch)
			}
			b.mu.Unlock()
			<-doneCh
		})
	}
	return detach, doneCh
}

// SubscriberCount is exposed for diagnostic logging.
func (b *Broadcaster) SubscriberCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

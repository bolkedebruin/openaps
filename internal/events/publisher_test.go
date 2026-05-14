package events

import (
	"sync"
	"testing"
	"time"

	"github.com/bolke/inv-driver/wire"
)

func telEnv(uid string) *wire.Envelope {
	return &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
		PeerUid: uid,
	}}}
}

func TestPublisher_SubscribePublishReceive(t *testing.T) {
	t.Parallel()
	p := New()
	ch, unsub := p.Subscribe()
	defer unsub()

	p.Publish(telEnv("a"))
	p.Publish(telEnv("b"))

	got := readN(t, ch, 2)
	if got[0].GetTelemetry().GetPeerUid() != "a" || got[1].GetTelemetry().GetPeerUid() != "b" {
		t.Fatalf("order: got %q %q", got[0].GetTelemetry().GetPeerUid(), got[1].GetTelemetry().GetPeerUid())
	}
}

func TestPublisher_UnsubscribeClosesChannel(t *testing.T) {
	t.Parallel()
	p := New()
	ch, unsub := p.Subscribe()
	if p.Len() != 1 {
		t.Fatalf("Len: got %d want 1", p.Len())
	}
	unsub()
	if p.Len() != 0 {
		t.Fatalf("Len after unsub: got %d want 0", p.Len())
	}
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after unsubscribe")
	}
	// double-unsubscribe is a no-op.
	unsub()
}

func TestPublisher_MultiSubscriberAllReceive(t *testing.T) {
	t.Parallel()
	p := New()
	ch1, u1 := p.Subscribe()
	defer u1()
	ch2, u2 := p.Subscribe()
	defer u2()

	p.Publish(telEnv("x"))

	if got := readN(t, ch1, 1); got[0].GetTelemetry().GetPeerUid() != "x" {
		t.Fatalf("sub1: %q", got[0].GetTelemetry().GetPeerUid())
	}
	if got := readN(t, ch2, 1); got[0].GetTelemetry().GetPeerUid() != "x" {
		t.Fatalf("sub2: %q", got[0].GetTelemetry().GetPeerUid())
	}
}

func TestPublisher_SlowSubscriberEvicted(t *testing.T) {
	t.Parallel()
	p := New()
	ch, _ := p.Subscribe()
	// Don't read. Overflow the channel.
	for i := 0; i < chanCap+5; i++ {
		p.Publish(telEnv("u"))
	}
	// Sub should have been removed.
	if p.Len() != 0 {
		t.Fatalf("Len: got %d want 0 after slow eviction", p.Len())
	}
	// Channel should be closed; drain it.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, ok := <-ch
		if !ok {
			return
		}
	}
	t.Fatal("channel still open after eviction")
}

func TestPublisher_ConcurrentPublishSubscribe(t *testing.T) {
	t.Parallel()
	p := New()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, unsub := p.Subscribe()
			defer unsub()
			// Read drain loop until publishers finish.
			deadline := time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(deadline) {
				select {
				case <-ch:
				case <-time.After(10 * time.Millisecond):
				}
			}
		}()
	}
	for i := 0; i < 100; i++ {
		p.Publish(telEnv("y"))
	}
	wg.Wait()
}

func readN(t *testing.T, ch <-chan *wire.Envelope, n int) []*wire.Envelope {
	t.Helper()
	out := make([]*wire.Envelope, 0, n)
	for i := 0; i < n; i++ {
		select {
		case env, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed at %d", i)
			}
			out = append(out, env)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for envelope %d", i)
		}
	}
	return out
}

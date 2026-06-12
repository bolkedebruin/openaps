package modem

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// manualClock is an injectable WatchdogClock the tests advance by hand so the
// watchdog logic runs against deterministic, instant time — no wall clock, no
// real sleeps in the decision path.
type manualClock struct {
	mu sync.Mutex
	t  time.Time
}

func newManualClock() *manualClock {
	return &manualClock{t: time.Unix(1_700_000_000, 0)}
}

func (c *manualClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *manualClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// counters tracks injected Probe/Recover invocations.
type counters struct {
	probes   int
	recovers int
}

// newWatchdog wires a watchdog with the standard test timings (Silence 5m,
// Interval 2m, Cooldown 3m) onto the manual clock with the given Probe/Recover.
func newWatchdog(clk *manualClock, probe func(context.Context) (bool, error), recover func(context.Context) error) *Watchdog {
	w := &Watchdog{
		Probe:    probe,
		Recover:  recover,
		Silence:  5 * time.Minute,
		Interval: 2 * time.Minute,
		Cooldown: 3 * time.Minute,
		Now:      clk.now,
		Log:      func(string, ...any) {},
	}
	w.defaults()
	return w
}

// TestTickDecision covers the pure skip-vs-probe decision against the recorded
// windows, driven entirely by the manual clock.
func TestTickDecision(t *testing.T) {
	t.Run("active bus skips (recent inbound)", func(t *testing.T) {
		clk := newManualClock()
		w := newWatchdog(clk, nil, nil)
		w.NoteInbound() // stamps now
		clk.advance(1 * time.Minute)
		if got := w.tickDecision(clk.now()); got != actionSkip {
			t.Fatalf("active bus: got %v, want actionSkip", got)
		}
	})

	t.Run("silent bus probes", func(t *testing.T) {
		clk := newManualClock()
		w := newWatchdog(clk, nil, nil)
		w.NoteInbound()
		clk.advance(6 * time.Minute) // past Silence (5m)
		if got := w.tickDecision(clk.now()); got != actionProbe {
			t.Fatalf("silent bus: got %v, want actionProbe", got)
		}
	})

	t.Run("within cooldown skips even when silent", func(t *testing.T) {
		clk := newManualClock()
		w := newWatchdog(clk, nil, nil)
		// Last inbound long ago (silent) but a recovery just happened.
		w.lastInbound = clk.now().Add(-10 * time.Minute)
		w.lastRecover = clk.now()
		clk.advance(2 * time.Minute) // < Cooldown (3m)
		if got := w.tickDecision(clk.now()); got != actionSkip {
			t.Fatalf("within cooldown: got %v, want actionSkip", got)
		}
	})

	t.Run("after cooldown probes", func(t *testing.T) {
		clk := newManualClock()
		w := newWatchdog(clk, nil, nil)
		w.lastInbound = clk.now().Add(-10 * time.Minute)
		w.lastRecover = clk.now()
		clk.advance(4 * time.Minute) // > Cooldown (3m), still silent
		if got := w.tickDecision(clk.now()); got != actionProbe {
			t.Fatalf("after cooldown: got %v, want actionProbe", got)
		}
	})

	t.Run("NoteInbound during silence flips back to healthy", func(t *testing.T) {
		clk := newManualClock()
		w := newWatchdog(clk, nil, nil)
		w.NoteInbound()
		clk.advance(6 * time.Minute) // would probe
		if got := w.tickDecision(clk.now()); got != actionProbe {
			t.Fatalf("pre-inbound: got %v, want actionProbe", got)
		}
		w.NoteInbound() // telemetry arrives
		if got := w.tickDecision(clk.now()); got != actionSkip {
			t.Fatalf("post-inbound: got %v, want actionSkip", got)
		}
	})
}

// TestRunProbeAlive: a healthy module acks 0x0D — no Recover, no window change.
func TestRunProbeAlive(t *testing.T) {
	clk := newManualClock()
	var c counters
	w := newWatchdog(clk,
		func(context.Context) (bool, error) { c.probes++; return true, nil },
		func(context.Context) error { c.recovers++; return nil },
	)
	w.runProbe(context.Background())
	if c.probes != 1 {
		t.Fatalf("probes = %d, want 1", c.probes)
	}
	if c.recovers != 0 {
		t.Fatalf("recovers = %d, want 0 (alive must not reset)", c.recovers)
	}
	if !w.lastRecover.IsZero() {
		t.Fatalf("lastRecover stamped on an alive probe")
	}
}

// TestRunProbeDeadRecovers: a wedged module triggers exactly one Recover and
// resets both windows on success.
func TestRunProbeDeadRecovers(t *testing.T) {
	clk := newManualClock()
	var c counters
	w := newWatchdog(clk,
		func(context.Context) (bool, error) { c.probes++; return false, nil },
		func(context.Context) error { c.recovers++; return nil },
	)
	// Make the windows stale so we can observe them being reset to now.
	w.lastInbound = clk.now().Add(-30 * time.Minute)
	before := clk.now()

	w.runProbe(context.Background())

	if c.probes != 1 {
		t.Fatalf("probes = %d, want 1", c.probes)
	}
	if c.recovers != 1 {
		t.Fatalf("recovers = %d, want 1", c.recovers)
	}
	if !w.lastRecover.Equal(before) {
		t.Fatalf("lastRecover = %v, want %v (stamped on recover)", w.lastRecover, before)
	}
	if !w.lastInbound.Equal(before) {
		t.Fatalf("lastInbound = %v, want %v (reset on successful recover)", w.lastInbound, before)
	}
}

// TestRunProbeDeadRecoverFails: a failed recovery still stamps lastRecover (so
// the cooldown backs us off) but does NOT reset lastInbound.
func TestRunProbeDeadRecoverFails(t *testing.T) {
	clk := newManualClock()
	var c counters
	w := newWatchdog(clk,
		func(context.Context) (bool, error) { c.probes++; return false, nil },
		func(context.Context) error { c.recovers++; return errors.New("reset failed") },
	)
	staleInbound := clk.now().Add(-30 * time.Minute)
	w.lastInbound = staleInbound
	before := clk.now()

	w.runProbe(context.Background())

	if c.recovers != 1 {
		t.Fatalf("recovers = %d, want 1", c.recovers)
	}
	if !w.lastRecover.Equal(before) {
		t.Fatalf("lastRecover = %v, want %v (must stamp to back off after a failed reset)", w.lastRecover, before)
	}
	if !w.lastInbound.Equal(staleInbound) {
		t.Fatalf("lastInbound = %v, want %v (must NOT reset on a failed recover)", w.lastInbound, staleInbound)
	}
}

// TestRunProbeTransportError: an inconclusive transport error never resets.
func TestRunProbeTransportError(t *testing.T) {
	clk := newManualClock()
	var c counters
	w := newWatchdog(clk,
		func(context.Context) (bool, error) { c.probes++; return false, errors.New("write: EIO") },
		func(context.Context) error { c.recovers++; return nil },
	)
	w.runProbe(context.Background())
	if c.probes != 1 {
		t.Fatalf("probes = %d, want 1", c.probes)
	}
	if c.recovers != 0 {
		t.Fatalf("recovers = %d, want 0 (transport error must not reset)", c.recovers)
	}
	if !w.lastRecover.IsZero() {
		t.Fatalf("lastRecover stamped on a transport error")
	}
}

// TestRunCtxCancel: Run returns promptly when ctx is cancelled while waiting on
// the ticker, and does not probe.
func TestRunCtxCancel(t *testing.T) {
	clk := newManualClock()
	var c counters
	w := newWatchdog(clk,
		func(context.Context) (bool, error) { c.probes++; return true, nil },
		func(context.Context) error { c.recovers++; return nil },
	)
	// Long interval so the only way Run exits in time is via ctx.Done.
	w.Interval = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return promptly after ctx cancel")
	}
	if c.probes != 0 {
		t.Fatalf("probes = %d, want 0 (cancelled before any tick)", c.probes)
	}
}

// TestRunStampsInboundUpFront: Run sets lastInbound at start so the first tick
// can't probe before the fleet has had a chance to report.
func TestRunStampsInboundUpFront(t *testing.T) {
	clk := newManualClock()
	w := newWatchdog(clk,
		func(context.Context) (bool, error) { return true, nil },
		func(context.Context) error { return nil },
	)
	w.Interval = time.Hour // no tick will fire during the test
	w.lastInbound = time.Time{}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	// Give Run a moment to stamp; poll the guarded field.
	deadline := time.Now().Add(time.Second)
	for {
		w.mu.Lock()
		started := w.started
		w.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("Run did not start in time")
		}
	}
	w.mu.Lock()
	stamped := w.lastInbound.Equal(clk.now())
	w.mu.Unlock()
	cancel()
	<-done
	if !stamped {
		t.Fatalf("lastInbound not stamped to now at Run start")
	}
}

// TestRunEndToEndDeadModule drives Run with fast real-time ticks and a fixed
// manual clock: a silent, wedged bus produces exactly one Recover, after which
// the cooldown (held open by the fixed clock) suppresses every later tick. The
// clock is set once, far past Silence and well inside Cooldown, and never moved
// again — so the only state change across ticks is the single recovery and the
// lastRecover stamp it leaves behind.
func TestRunEndToEndDeadModule(t *testing.T) {
	clk := newManualClock()
	var mu sync.Mutex
	var probes, recovers int
	w := &Watchdog{
		Probe: func(context.Context) (bool, error) {
			mu.Lock()
			probes++
			mu.Unlock()
			return false, nil // wedged
		},
		Recover: func(context.Context) error {
			mu.Lock()
			recovers++
			mu.Unlock()
			return nil
		},
		Silence:  5 * time.Minute,
		Interval: 2 * time.Millisecond,
		Cooldown: time.Hour,
		Now:      clk.now,
		Log:      func(string, ...any) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	// Wait until Run has stamped lastInbound = now at start, THEN advance the
	// clock past Silence. Advancing only after the start-stamp makes the silent
	// gap deterministic regardless of goroutine scheduling. The 10m gap is well
	// inside Cooldown (1h), so once a recovery stamps lastRecover = now every
	// later tick skips. The clock is frozen for the rest of the test.
	startDeadline := time.Now().Add(time.Second)
	for {
		w.mu.Lock()
		started := w.started
		w.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(startDeadline) {
			cancel()
			<-done
			t.Fatal("Run did not start in time")
		}
		time.Sleep(time.Millisecond)
	}
	clk.advance(10 * time.Minute)

	// Wait for the first recovery, then let many more ticks elapse to prove the
	// cooldown holds them off.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		r := recovers
		mu.Unlock()
		if r >= 1 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // ~25 ticks, all inside cooldown
	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if recovers != 1 {
		t.Fatalf("recovers = %d, want exactly 1 (cooldown must suppress repeats)", recovers)
	}
	if probes < 1 {
		t.Fatalf("probes = %d, want >= 1", probes)
	}
}

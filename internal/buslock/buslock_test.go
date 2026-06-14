package buslock

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLock_AcquireRelease(t *testing.T) {
	l := New()
	ok, owner := l.TryAcquire("pairing")
	if !ok {
		t.Fatalf("first acquire should succeed")
	}
	if owner != "" {
		t.Fatalf("owner on success should be empty, got %q", owner)
	}

	ok2, cur := l.TryAcquire("gridprofile-broadcast")
	if ok2 {
		t.Fatalf("second acquire should fail while held")
	}
	if cur != "pairing" {
		t.Fatalf("current owner = %q want pairing", cur)
	}

	if held, who := l.Held(); !held || who != "pairing" {
		t.Fatalf("Held = (%v,%q) want (true,pairing)", held, who)
	}

	l.Release()
	if held, _ := l.Held(); held {
		t.Fatalf("lock still held after release")
	}

	// Now the other owner can take it.
	ok3, _ := l.TryAcquire("gridprofile-broadcast")
	if !ok3 {
		t.Fatalf("acquire after release should succeed")
	}
	l.Release()
}

func TestLock_DoubleReleaseSafe(t *testing.T) {
	l := New()
	l.Release() // no-op, must not panic
	ok, _ := l.TryAcquire("x")
	if !ok {
		t.Fatal("acquire after spurious release should succeed")
	}
	l.Release()
	l.Release() // double release, must not panic
}

// AcquirePolite succeeds on a free lock with no waiter, yields while the lock
// is held, and yields while an operator op is pending even when free. On
// success the yield channel is open; the yield ok=false case returns a nil
// channel.
func TestPolite_YieldsAndSucceeds(t *testing.T) {
	l := New()

	// Free, no waiter -> success, open yield channel.
	ok, y := l.AcquirePolite("poller")
	if !ok || y == nil {
		t.Fatalf("polite on free lock = (%v, chan=%v) want (true, non-nil)", ok, y)
	}
	select {
	case <-y:
		t.Fatalf("yield channel should be open while held with no priority op")
	default:
	}

	// Held -> yields with a nil channel.
	ok, y = l.AcquirePolite("poller")
	if ok || y != nil {
		t.Fatalf("polite while held = (%v, chan=%v) want (false, nil)", ok, y)
	}
	l.Release()

	// Free again -> success, a fresh (open) channel.
	ok, y = l.AcquirePolite("poller")
	if !ok || y == nil {
		t.Fatalf("polite after release should succeed with a fresh channel")
	}
	select {
	case <-y:
		t.Fatalf("fresh yield channel should be open")
	default:
	}
	l.Release()

	// Free but an operator op is pending -> yields (nil channel).
	l.mu.Lock()
	l.waiting = 1
	l.mu.Unlock()
	ok, y = l.AcquirePolite("poller")
	if ok || y != nil {
		t.Fatalf("polite while waiter pending = (%v, chan=%v) want (false, nil)", ok, y)
	}
	l.mu.Lock()
	l.waiting = 0
	l.mu.Unlock()
}

// The yield channel handed to a polite holder is closed promptly when an
// AcquirePriority op starts waiting: a receive on it unblocks.
func TestPolite_YieldChannelClosedOnPriorityWait(t *testing.T) {
	l := New()
	ok, y := l.AcquirePolite("poller")
	if !ok {
		t.Fatalf("poller acquire should succeed")
	}

	// A priority op begins waiting and should close the yield channel, then the
	// poller observes it, Releases, and the priority op proceeds.
	go func() {
		<-y
		l.Release()
	}()

	start := time.Now()
	ok, blocker := l.AcquirePriority("pairing", time.Second)
	if !ok {
		t.Fatalf("priority should acquire after poller yields, blocker=%q", blocker)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("yield/preempt took too long: %v", elapsed)
	}
	l.Release()
}

// AcquirePriority preempts a polite holder that selects on its yield channel
// and Releases. After the priority op holds, polite refuses (held). Once it
// releases, polite succeeds again with a fresh open channel.
func TestPriority_PreemptsPoller(t *testing.T) {
	l := New()

	ok, y := l.AcquirePolite("poller")
	if !ok {
		t.Fatalf("poller acquire should succeed")
	}
	go func() {
		<-y // revoked when the priority op starts waiting
		time.Sleep(10 * time.Millisecond)
		l.Release()
	}()

	start := time.Now()
	ok, blocker := l.AcquirePriority("pairing", 500*time.Millisecond)
	if !ok {
		t.Fatalf("priority acquire should succeed, blocker=%q", blocker)
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("priority took too long: %v", elapsed)
	}
	if held, who := l.Held(); !held || who != "pairing" {
		t.Fatalf("Held = (%v,%q) want (true,pairing)", held, who)
	}

	// While the priority op holds, polite refuses (lock held).
	if ok, y := l.AcquirePolite("poller"); ok || y != nil {
		t.Fatalf("polite while priority holds = (%v, chan=%v) want (false, nil)", ok, y)
	}
	l.Release()

	// Poller resumes with a fresh, open channel.
	ok, y = l.AcquirePolite("poller")
	if !ok || y == nil {
		t.Fatalf("poller should resume with a fresh channel after release")
	}
	select {
	case <-y:
		t.Fatalf("resumed yield channel should be open")
	default:
	}
	l.Release()
}

// AcquirePriority times out and reports the blocker when another priority
// holder never releases.
func TestPriority_TimeoutBlocker(t *testing.T) {
	l := New()
	if ok, _ := l.AcquirePriority("op-a", 50*time.Millisecond); !ok {
		t.Fatalf("first priority acquire should succeed")
	}
	// op-a holds and never releases within the window.
	start := time.Now()
	ok, blocker := l.AcquirePriority("op-b", 60*time.Millisecond)
	if ok {
		t.Fatalf("second priority acquire should time out")
	}
	if blocker != "op-a" {
		t.Fatalf("blocker = %q want op-a", blocker)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Fatalf("timed out too early: %v", elapsed)
	}
	// The timed-out waiter must have cleared its pause request: a polite acquire
	// now succeeds (waiting back to 0 once op-b returned, op-a still holds so
	// release it first).
	l.Release()
	if ok, _ := l.AcquirePolite("poller"); !ok {
		t.Fatalf("polite should succeed once the timed-out waiter cleared and lock freed")
	}
	l.Release()
}

// Two priority acquirers racing where the winner holds past the loser's
// timeout: exactly one wins and the other times out reporting the winner as
// blocker. The yield channel is never double-closed (no panic) even with two
// concurrent priority ops, and no polite holder is involved here.
func TestPriority_TwoRacers(t *testing.T) {
	l := New()
	var wins int32
	var loserBlocker atomic.Value
	loserBlocker.Store("")

	var wg sync.WaitGroup
	for _, name := range []string{"pairing", "gridprofile-broadcast"} {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			// Winner holds 120ms, longer than the 60ms timeout, so the loser
			// cannot acquire after the winner — it must time out.
			ok, blocker := l.AcquirePriority(owner, 60*time.Millisecond)
			if ok {
				atomic.AddInt32(&wins, 1)
				time.Sleep(120 * time.Millisecond)
				l.Release()
			} else {
				loserBlocker.Store(blocker)
			}
		}(name)
	}
	wg.Wait()

	if wins != 1 {
		t.Fatalf("exactly one racer should win, got %d", wins)
	}
	if b := loserBlocker.Load().(string); b != "pairing" && b != "gridprofile-broadcast" {
		t.Fatalf("loser blocker = %q want one of the racer owners", b)
	}
	if held, _ := l.Held(); held {
		t.Fatalf("lock must be free after winner releases")
	}
}

// Two priority ops both racing to revoke the SAME polite holder must not
// double-close its yield channel.
func TestPriority_TwoRacersWithPoliteHolder(t *testing.T) {
	l := New()
	ok, y := l.AcquirePolite("poller")
	if !ok {
		t.Fatalf("poller acquire should succeed")
	}
	go func() {
		<-y
		l.Release() // bail promptly on revocation
	}()

	var wg sync.WaitGroup
	var wins int32
	for _, name := range []string{"pairing", "gridprofile-broadcast"} {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			// The winner holds 120ms, past the loser's 50ms timeout, so exactly
			// one acquires.
			ok, _ := l.AcquirePriority(owner, 50*time.Millisecond)
			if ok {
				atomic.AddInt32(&wins, 1)
				time.Sleep(120 * time.Millisecond)
				l.Release()
			}
		}(name)
	}
	wg.Wait() // a double-close would have panicked here

	if wins != 1 {
		t.Fatalf("exactly one racer should win, got %d", wins)
	}
	if held, _ := l.Held(); held {
		t.Fatalf("lock must be free after the winner releases")
	}
}

// No livelock: while a priority op waits (held by a non-yielding holder),
// polite acquire keeps refusing so the poller can't jump ahead; once the
// priority op releases, polite succeeds again.
func TestNoLivelock_PoliteRefusesWhilePriorityWaits(t *testing.T) {
	l := New()

	// A non-priority holder occupies the lock so the priority op must wait.
	if ok, _ := l.TryAcquire("holder"); !ok {
		t.Fatalf("holder acquire should succeed")
	}

	prioDone := make(chan struct{})
	go func() {
		ok, _ := l.AcquirePriority("op", time.Second)
		if !ok {
			t.Errorf("priority should eventually acquire")
		}
		close(prioDone)
	}()

	// Give the priority op time to register its wait (waiting > 0).
	deadline := time.Now().Add(time.Second)
	for {
		l.mu.Lock()
		w := l.waiting
		l.mu.Unlock()
		if w > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("priority op never registered its wait")
		}
		time.Sleep(time.Millisecond)
	}

	// Free the lock but, while the priority op waits, polite must refuse.
	l.Release()
	if ok, y := l.AcquirePolite("poller"); ok || y != nil {
		t.Fatalf("polite must refuse while a priority op waits, got (%v,%v)", ok, y)
	}

	// Let the priority op take it and release.
	<-prioDone
	if held, who := l.Held(); !held || who != "op" {
		t.Fatalf("priority op should hold, got (%v,%q)", held, who)
	}
	l.Release()

	// Now polite succeeds again.
	if ok, _ := l.AcquirePolite("poller"); !ok {
		t.Fatalf("polite should succeed once the priority op released")
	}
	l.Release()
}

// Stress: many polite holders (acquire / select-on-yield / release) plus a few
// priority ops. Everything terminates, no deadlock, and the poller resumes
// (can acquire politely) after the last priority op releases.
func TestStress_NoDeadlockPollerResumes(t *testing.T) {
	l := New()
	stop := make(chan struct{})

	var politeWg sync.WaitGroup
	var politeAcquires int64
	for i := 0; i < 8; i++ {
		politeWg.Add(1)
		go func() {
			defer politeWg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if ok, y := l.AcquirePolite("poller"); ok {
					atomic.AddInt64(&politeAcquires, 1)
					// Hold briefly, but bail immediately on revocation.
					select {
					case <-y:
					case <-time.After(time.Millisecond):
					}
					l.Release()
				} else {
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	var prioWg sync.WaitGroup
	var prioAcquires int64
	for i := 0; i < 5; i++ {
		prioWg.Add(1)
		go func() {
			defer prioWg.Done()
			if ok, _ := l.AcquirePriority("op", 2*time.Second); ok {
				atomic.AddInt64(&prioAcquires, 1)
				time.Sleep(5 * time.Millisecond)
				l.Release()
			}
		}()
	}

	// All priority ops must acquire (none starves) and finish.
	done := make(chan struct{})
	go func() { prioWg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		close(stop)
		t.Fatalf("priority ops deadlocked / did not all complete")
	}
	if got := atomic.LoadInt64(&prioAcquires); got != 5 {
		t.Fatalf("priority acquires = %d want 5 (some starved)", got)
	}

	close(stop)
	politeWg.Wait()

	// Poller resumes: with no waiter and lock free, polite acquire succeeds.
	if ok, _ := l.AcquirePolite("poller"); !ok {
		t.Fatalf("poller failed to resume after priority ops drained")
	}
	l.Release()
	_ = atomic.LoadInt64(&politeAcquires)
}

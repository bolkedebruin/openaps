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

// TryAcquirePolite succeeds on a free lock with no waiter, yields while the
// lock is held, and yields while an operator op is pending even when free.
func TestPolite_YieldsAndSucceeds(t *testing.T) {
	l := New()

	// Free, no waiter -> success.
	ok, holder := l.TryAcquirePolite("poller")
	if !ok || holder != "" {
		t.Fatalf("polite on free lock = (%v,%q) want (true,\"\")", ok, holder)
	}

	// Held -> yields with the holder reported.
	ok, holder = l.TryAcquirePolite("poller")
	if ok || holder != "poller" {
		t.Fatalf("polite while held = (%v,%q) want (false,poller)", ok, holder)
	}
	l.Release()

	// Free again -> success.
	ok, _ = l.TryAcquirePolite("poller")
	if !ok {
		t.Fatalf("polite after release should succeed")
	}
	l.Release()

	// Free but an operator op is pending -> yields (holder "").
	l.mu.Lock()
	l.pausePending = 1
	l.mu.Unlock()
	ok, holder = l.TryAcquirePolite("poller")
	if ok || holder != "" {
		t.Fatalf("polite while waiter pending = (%v,%q) want (false,\"\")", ok, holder)
	}
	l.mu.Lock()
	l.pausePending = 0
	l.mu.Unlock()
}

// AcquirePriority preempts a poller that releases shortly after the priority
// op starts waiting; PausePending is true while it waits and false after.
func TestPriority_PreemptsPoller(t *testing.T) {
	l := New()

	// Poller holds the lock, releases after a short hold.
	if ok, _ := l.TryAcquirePolite("poller"); !ok {
		t.Fatalf("poller acquire should succeed")
	}
	go func() {
		time.Sleep(40 * time.Millisecond)
		l.Release()
	}()

	// Observe PausePending flip true while the priority op waits.
	sawPending := make(chan bool, 1)
	go func() {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if l.PausePending() {
				sawPending <- true
				return
			}
			time.Sleep(time.Millisecond)
		}
		sawPending <- false
	}()

	start := time.Now()
	ok, blocker := l.AcquirePriority("pairing", 500*time.Millisecond)
	if !ok {
		t.Fatalf("priority acquire should succeed, blocker=%q", blocker)
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("priority took too long: %v", elapsed)
	}
	if !<-sawPending {
		t.Fatalf("PausePending never observed true while priority op waited")
	}
	if l.PausePending() {
		t.Fatalf("PausePending should be false after priority op acquired")
	}
	if held, who := l.Held(); !held || who != "pairing" {
		t.Fatalf("Held = (%v,%q) want (true,pairing)", held, who)
	}

	// After priority holds, polite still yields (lock held) until Release.
	if ok, holder := l.TryAcquirePolite("poller"); ok || holder != "pairing" {
		t.Fatalf("polite while priority holds = (%v,%q) want (false,pairing)", ok, holder)
	}
	l.Release()
	if l.PausePending() {
		t.Fatalf("PausePending must be false after release")
	}
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
	// The timed-out waiter must have cleared its pause request.
	if l.PausePending() {
		t.Fatalf("PausePending must be false after the waiter gives up")
	}
	l.Release()
}

// Two priority acquirers racing where the winner holds past the loser's
// timeout: exactly one wins and the other times out reporting the winner as
// blocker. Safe even though no runMu guards them here (broadcast may race
// pairing).
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
	if l.PausePending() {
		t.Fatalf("PausePending must be false after both racers finished")
	}
	if held, _ := l.Held(); held {
		t.Fatalf("lock must be free after winner releases")
	}
}

// Stress: many polite pollers plus a few priority ops. Everything terminates,
// no deadlock, and the poller resumes (can acquire politely) after the last
// priority op releases.
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
				if ok, _ := l.TryAcquirePolite("poller"); ok {
					atomic.AddInt64(&politeAcquires, 1)
					time.Sleep(time.Millisecond)
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
	// (During the contended phase polite pollers legitimately yield to the
	// priority ops; the invariant under test is that they resume afterwards,
	// not that they made progress while operator ops were pending.)
	if ok, _ := l.TryAcquirePolite("poller"); !ok {
		t.Fatalf("poller failed to resume after priority ops drained")
	}
	l.Release()
	_ = atomic.LoadInt64(&politeAcquires)
}

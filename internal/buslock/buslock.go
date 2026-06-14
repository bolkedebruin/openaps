// Package buslock provides the single process-wide guard that serialises
// fleet-disruptive bus operations against one another. The pairing state
// machine (scan / bind / migrate / rekey) and the grid-profile broadcast
// path both park the radio off the steady-state telemetry flow or issue
// broadcast frames; only one such operation may run at a time, fleet-wide.
//
// It is deliberately a separate, dependency-free package so both the
// pairing package and the gridprofile package can import it without an
// import cycle.
package buslock

import (
	"sync"
	"time"
)

// Lock is a non-reentrant try-lock with an owner label for diagnostics.
// The zero value is ready to use; share a single *Lock across every caller
// that must be mutually exclusive.
//
// Two classes of acquirer share the lock with different priority. The
// steady-state telemetry poller acquires politely (AcquirePolite) and is
// revoked the moment an operator op is waiting; disruptive operator ops
// (pairing, grid-profile broadcast) acquire with priority (AcquirePriority),
// raising a pause request so the poller stands down and they win within a
// bounded wait instead of starving behind a continuously-re-acquiring poller.
//
// Preemption is carried by a single revocation signal rather than split across
// cooperative check-points the holder must remember. AcquirePolite hands back a
// yield channel; AcquirePriority closes it when it begins waiting, so the polite
// holder bails on the first select rather than running to the end of its round.
//
// Concurrency invariants: mu guards ALL of held/owner/waiting/yield. The yield
// channel is closed at most once and only by AcquirePriority (nil-guarded), so
// concurrent priority ops cannot double-close it. AcquirePolite refuses while
// waiting > 0 so a just-revoked poller cannot re-acquire ahead of the waiting
// priority op. The yield channel is never closed by Release and is GC'd once the
// polite holder drops its reference, so there is no goroutine or channel leak.
type Lock struct {
	mu    sync.Mutex
	held  bool
	owner string
	// waiting counts operator acquirers currently waiting in AcquirePriority.
	// While it is > 0 the poller's polite acquire refuses so the waiting
	// operator op can preempt it without the poller jumping ahead (livelock).
	waiting int
	// yield is non-nil only while a polite holder holds the lock; it is the
	// channel returned by AcquirePolite. AcquirePriority closes it exactly once
	// (under mu, nil-guarded) to revoke that holder, then sets it to nil.
	yield chan struct{}
}

// New returns a ready-to-use lock. Callers may also use a zero-value Lock.
func New() *Lock { return &Lock{} }

// TryAcquire grabs the lock for owner, returning ok=false (and the current
// owner) when it is already held. It never blocks — a caller that loses the
// race fails its request immediately rather than queueing behind a long
// fleet operation.
func (l *Lock) TryAcquire(owner string) (ok bool, currentOwner string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held {
		return false, l.owner
	}
	l.held = true
	l.owner = owner
	return true, ""
}

// Release frees the lock. Calling Release when the lock is not held is a
// no-op so a double-release in an error path can't panic.
//
// Release clears the yield channel reference but does NOT close it: a polite
// holder returning normally has no waiter selecting on it, and only
// AcquirePriority closes the channel to revoke an in-flight holder.
func (l *Lock) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.held = false
	l.owner = ""
	l.yield = nil
}

// Held reports whether the lock is currently held and by whom.
func (l *Lock) Held() (held bool, owner string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held, l.owner
}

// AcquirePriority claims the lock for a disruptive operator op, preempting the
// steady-state poller. On entry it raises a pause request (waiting++) and, if a
// polite holder currently holds, closes that holder's yield channel so it bails
// immediately — closed exactly once, guarded by the nil check so two concurrent
// priority ops can't double-close. It then waits up to timeout for the lock to
// fall free and claims it. Returns ok=false plus the current holder if it can't
// be obtained within timeout (e.g. another operator op holds it). The pause
// request is always cleared on return, so a timeout never permanently starves
// the poller.
func (l *Lock) AcquirePriority(owner string, timeout time.Duration) (ok bool, blocker string) {
	l.mu.Lock()
	l.waiting++
	// Revoke the current polite holder, if any, so it Releases promptly.
	if l.yield != nil {
		close(l.yield)
		l.yield = nil
	}
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		l.waiting--
		l.mu.Unlock()
	}()

	deadline := time.Now().Add(timeout)
	for {
		l.mu.Lock()
		if !l.held {
			l.held = true
			l.owner = owner
			l.mu.Unlock()
			return true, ""
		}
		cur := l.owner
		l.mu.Unlock()

		if time.Now().After(deadline) {
			return false, cur
		}
		// Bounded poll: this operator path is rare and process-internal, so a
		// short sleep between attempts is acceptable and keeps the lock free of
		// condition-variable bookkeeping. The yield channel already makes the
		// poller bail immediately, so this poll only adds <=20ms to picking up
		// the freed lock — not a full poll round.
		time.Sleep(20 * time.Millisecond)
	}
}

// AcquirePolite acquires for a steady-state background holder (the telemetry
// poller). It succeeds only when the lock is free AND no operator op is waiting;
// otherwise it yields (ok=false, nil channel). On success it returns a yield
// channel that is CLOSED when an operator op (AcquirePriority) starts waiting —
// the holder must select on it and Release promptly. The single revocation
// signal replaces a mid-round pause check the holder would otherwise have to
// remember to make.
func (l *Lock) AcquirePolite(owner string) (ok bool, yield <-chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held || l.waiting > 0 {
		return false, nil
	}
	l.held = true
	l.owner = owner
	l.yield = make(chan struct{})
	return true, l.yield
}

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
// steady-state telemetry poller acquires politely (TryAcquirePolite) and
// yields the moment an operator op is waiting; disruptive operator ops
// (pairing, grid-profile broadcast) acquire with priority (AcquirePriority),
// raising a pause request so the poller stands down and they win within a
// bounded wait instead of starving behind a continuously-re-acquiring poller.
type Lock struct {
	mu    sync.Mutex
	held  bool
	owner string
	// pausePending counts operator acquirers currently waiting in
	// AcquirePriority. While it is > 0 the poller's polite acquire yields so
	// the waiting operator op can preempt it.
	pausePending int
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
func (l *Lock) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.held = false
	l.owner = ""
}

// Held reports whether the lock is currently held and by whom.
func (l *Lock) Held() (held bool, owner string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held, l.owner
}

// AcquirePriority claims the lock for a disruptive operator op, preempting the
// steady-state poller. It raises a pause request — so the poller's polite
// acquire (TryAcquirePolite) yields and any in-flight poll round releases early
// via PausePending — then waits up to timeout for the lock to fall free and
// claims it. Returns ok=false plus the current holder if it can't be obtained
// within timeout (e.g. another operator op holds it). The pause request is
// always cleared on return, so a timeout never permanently starves the poller.
func (l *Lock) AcquirePriority(owner string, timeout time.Duration) (ok bool, blocker string) {
	l.mu.Lock()
	l.pausePending++
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		l.pausePending--
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
		// condition-variable bookkeeping.
		time.Sleep(20 * time.Millisecond)
	}
}

// TryAcquirePolite is the steady-state poller's acquire. It succeeds only when
// the lock is free AND no operator op is waiting (pausePending == 0); otherwise
// it yields (ok=false) so a waiting operator op can preempt the poll cycle. The
// returned holder is the current owner when the lock is held, or "" when the
// poller is yielding purely because an operator op is pending.
func (l *Lock) TryAcquirePolite(owner string) (ok bool, holder string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held {
		return false, l.owner
	}
	if l.pausePending > 0 {
		return false, ""
	}
	l.held = true
	l.owner = owner
	return true, ""
}

// PausePending reports whether an operator op is waiting for the lock. The
// poller checks this mid-round to release the lock early and let the operator
// op preempt it.
func (l *Lock) PausePending() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pausePending > 0
}

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

import "sync"

// Lock is a non-reentrant try-lock with an owner label for diagnostics.
// The zero value is ready to use; share a single *Lock across every caller
// that must be mutually exclusive.
type Lock struct {
	mu    sync.Mutex
	held  bool
	owner string
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

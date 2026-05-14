package wire

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"time"
)

// DialLoopConfig parameterises DialLoop.
type DialLoopConfig struct {
	// SocketPath is the UDS path to dial.
	SocketPath string
	// MinBackoff is the initial sleep after the first dial failure or
	// session end. Defaults to 1s when zero.
	MinBackoff time.Duration
	// MaxBackoff caps the exponential growth. Defaults to 30s when zero.
	MaxBackoff time.Duration
	// Session runs against a freshly dialled conn. It returns when the
	// conn errors out or the caller is otherwise done. The conn is
	// closed by DialLoop after Session returns.
	Session func(ctx context.Context, conn net.Conn) error
	// OnConnect is called once per successful dial, before Session runs.
	OnConnect func()
	// OnDialError is called once per failed dial with the error and the
	// retry sleep that follows.
	OnDialError func(err error, retryIn time.Duration)
}

// DialLoop dials cfg.SocketPath and invokes cfg.Session. On Session
// return or any dial error, it sleeps with centered ±10% jitter and
// retries until ctx is cancelled. Returns ctx.Err on cancellation.
func DialLoop(ctx context.Context, cfg DialLoopConfig) error {
	if cfg.Session == nil {
		return errors.New("wire.DialLoop: Session is nil")
	}
	if cfg.SocketPath == "" {
		return errors.New("wire.DialLoop: SocketPath is empty")
	}
	min := cfg.MinBackoff
	if min <= 0 {
		min = 1 * time.Second
	}
	maxB := cfg.MaxBackoff
	if maxB <= 0 {
		maxB = 30 * time.Second
	}

	backoff := min
	var dialer net.Dialer
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, err := dialer.DialContext(ctx, "unix", cfg.SocketPath)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if cfg.OnDialError != nil {
				cfg.OnDialError(err, backoff)
			}
			if !sleepCtx(ctx, backoff) {
				return ctx.Err()
			}
			backoff = nextBackoff(backoff, maxB)
			continue
		}
		if cfg.OnConnect != nil {
			cfg.OnConnect()
		}
		backoff = min
		sessErr := cfg.Session(ctx, conn)
		_ = conn.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if sessErr != nil && !errors.Is(sessErr, io.EOF) && cfg.OnDialError != nil {
			// Reuse OnDialError as a generic "session ended" notifier so
			// callers don't need a second callback for it.
			cfg.OnDialError(sessErr, backoff)
		}
	}
}

// sleepCtx waits d or until ctx is cancelled. Returns false on cancel.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// nextBackoff doubles d up to maxB, then applies centered ±10% jitter.
func nextBackoff(d, maxB time.Duration) time.Duration {
	n := d * 2
	if n > maxB {
		n = maxB
	}
	return n + jitter(n)
}

// jitter returns a random offset in [-n/10, +n/10).
func jitter(n time.Duration) time.Duration {
	span := int64(n) / 5 // 20% of n
	if span <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(span) - span/2)
}

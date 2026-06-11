package udsutil

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// Roundtrip implements the short-lived UDS request pattern shared by the
// CLI tools and control clients in this repo: dial the unix socket at
// socketPath, write the hello envelope, write each request envelope, and
// — when match is non-nil — read envelopes until match accepts one and
// return it. Unrelated frames are skipped until the deadline. A nil match
// makes the call fire-and-forget: the requests are written and (nil, nil)
// is returned.
//
// The connection deadline is taken from ctx, so callers bound the whole
// call (dial + writes + read) with context.WithTimeout; cancelling ctx
// also unblocks an in-flight write or read.
func Roundtrip(ctx context.Context, socketPath string, hello *wire.Envelope, match func(*wire.Envelope) bool, reqs ...*wire.Envelope) (*wire.Envelope, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	defer stop()

	if err := wire.WriteFrame(conn, hello); err != nil {
		return nil, fmt.Errorf("hello: %w", err)
	}
	for _, req := range reqs {
		if err := wire.WriteFrame(conn, req); err != nil {
			return nil, fmt.Errorf("write request: %w", err)
		}
	}
	if match == nil {
		return nil, nil
	}
	for {
		var env wire.Envelope
		if err := wire.ReadFrame(conn, &env); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		if match(&env) {
			return &env, nil
		}
	}
}

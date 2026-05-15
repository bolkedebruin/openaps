package wire

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
)

// SubscriberSession is a Session function suitable for wire.DialLoop in
// subscriber mode. It sends hello first and then dispatches each
// Envelope_Telemetry it reads to onTelemetry. Other envelope kinds are
// ignored silently. The function returns when ctx is cancelled (clean
// nil) or when the underlying conn read or write fails.
//
// hello is sent verbatim; the caller fills Backend / Version / Hostname /
// StartedAtMs / Role. SUBSCRIBER is the expected role but this helper
// does not enforce it — that policy belongs to the caller.
//
// onTelemetry may be nil, in which case decoded Telemetry envelopes are
// dropped along with the rest.
func SubscriberSession(ctx context.Context, conn net.Conn, hello *Hello, onTelemetry func(*Telemetry)) error {
	if conn == nil {
		return errors.New("wire.SubscriberSession: nil conn")
	}
	if hello == nil {
		return errors.New("wire.SubscriberSession: nil hello")
	}

	env := &Envelope{Body: &Envelope_Hello{Hello: hello}}
	if err := WriteFrame(conn, env); err != nil {
		return fmt.Errorf("hello: %w", err)
	}

	// Closing the conn on ctx.Done unblocks the read loop with
	// net.ErrClosed so DialLoop sees the cancel and exits.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	for {
		var in Envelope
		if err := ReadFrame(conn, &in); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		if t := in.GetTelemetry(); t != nil && onTelemetry != nil {
			onTelemetry(t)
		}
	}
}

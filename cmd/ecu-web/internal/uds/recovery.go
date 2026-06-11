package uds

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// Recovery is the ecu-web client for recoveryd, the daemon that owns the
// box's SSH access plane (the single writer of authorized_keys). Unlike
// Controller, recoveryd speaks raw length-prefixed AccessRequest/
// AccessResponse frames (no Hello handshake, no Envelope wrapper): each
// call dials /var/run/recoveryd.sock, writes one AccessRequest, reads the one
// AccessResponse, and closes. It reuses wire.ReadMessage/WriteMessage so
// the length-prefixed framing lives in exactly one place.
type Recovery struct {
	SocketPath string
}

const recoveryTimeout = 5 * time.Second

// roundtrip dials recoveryd, writes req, and returns its AccessResponse. A
// response with ok=false is surfaced as a non-nil error carrying the
// daemon's message, alongside the response (so callers can still read
// provider/keys when present).
func (r *Recovery) roundtrip(ctx context.Context, req *wire.AccessRequest) (*wire.AccessResponse, error) {
	if r.SocketPath == "" {
		return nil, errors.New("uds: recoveryd socket path is empty")
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", r.SocketPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	deadline := time.Now().Add(recoveryTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetWriteDeadline(deadline)
	if err := wire.WriteMessage(conn, req); err != nil {
		return nil, fmt.Errorf("uds: write recoveryd request: %w", err)
	}
	_ = conn.SetReadDeadline(deadline)
	var resp wire.AccessResponse
	if err := wire.ReadMessage(conn, &resp); err != nil {
		return nil, fmt.Errorf("uds: read recoveryd response: %w", err)
	}
	if !resp.GetOk() {
		return &resp, fmt.Errorf("recoveryd: %s", resp.GetError())
	}
	return &resp, nil
}

// ListKeys returns the full authorized-key list plus the provider state.
func (r *Recovery) ListKeys(ctx context.Context) (*wire.AccessResponse, error) {
	return r.roundtrip(ctx, &wire.AccessRequest{Op: &wire.AccessRequest_ListKeys{ListKeys: &wire.ListKeysOp{}}})
}

// AddKey validates and appends an authorized key. pubkey is the full
// OpenSSH single-line form; comment overrides any trailing comment when
// non-empty. On success the updated key list is returned.
func (r *Recovery) AddKey(ctx context.Context, pubkey, comment string) (*wire.AccessResponse, error) {
	return r.roundtrip(ctx, &wire.AccessRequest{Op: &wire.AccessRequest_AddKey{AddKey: &wire.AddKeyOp{
		Pubkey:  pubkey,
		Comment: comment,
	}}})
}

// RemoveKey removes the key whose SHA256 fingerprint matches. On success
// the updated key list is returned.
func (r *Recovery) RemoveKey(ctx context.Context, fingerprint string) (*wire.AccessResponse, error) {
	return r.roundtrip(ctx, &wire.AccessRequest{Op: &wire.AccessRequest_RemoveKey{RemoveKey: &wire.RemoveKeyOp{
		Fingerprint: fingerprint,
	}}})
}

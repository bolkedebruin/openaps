package uds

import (
	"context"
	"fmt"
	"time"

	"github.com/bolkedebruin/openaps/internal/udsutil"
	"github.com/bolkedebruin/openaps/wire"
)

// Controller issues request/response control ops to inv-driver over a
// short-lived PUBLISHER connection (the same pattern ecu-sunspec uses
// for Send). Each call dials, says Hello, writes one request, reads the
// matching response, and closes. The backend must be on inv-driver's
// -controller-backends allow-list and the connection must satisfy the
// -controller-uids gate (root by default).
type Controller struct {
	SocketPath string
	Backend    string
	Version    string
	Hostname   string
}

const controllerTimeout = 8 * time.Second

// hello builds the Hello envelope for a one-shot connection.
// ROLE_UNSPECIFIED runs the publisher loop, which is where the
// request/response handlers live.
func (c *Controller) hello() *wire.Envelope {
	return &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend:     c.Backend,
		Version:     c.Version,
		Hostname:    c.Hostname,
		StartedAtMs: time.Now().UnixMilli(),
	}}}
}

// roundtrip dials, writes one request envelope, and returns the first
// response envelope for which match returns true. Unrelated frames are
// skipped until the deadline.
func (c *Controller) roundtrip(ctx context.Context, req *wire.Envelope, match func(*wire.Envelope) bool) (*wire.Envelope, error) {
	ctx, cancel := context.WithTimeout(ctx, controllerTimeout)
	defer cancel()
	return udsutil.Roundtrip(ctx, c.SocketPath, c.hello(), match, req)
}

// Send injects one L2 frame addressed to a single inverter (by peer_uid)
// over a one-shot control connection. It is fire-and-forget: a nil return
// means inv-driver accepted the frame for downstream delivery, NOT that the
// inverter applied it (matching ecu-sunspec's Send semantics).
func (c *Controller) Send(ctx context.Context, uid string, frame []byte) error {
	if uid == "" {
		return fmt.Errorf("uds: empty uid")
	}
	if len(frame) == 0 {
		return fmt.Errorf("uds: empty frame")
	}
	env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{PeerUid: uid, Frame: frame}}}
	if _, err := c.roundtrip(ctx, env, nil); err != nil {
		return fmt.Errorf("uds: send: %w", err)
	}
	return nil
}

// SystemStatus fetches ECU identity + connected peers from inv-driver.
func (c *Controller) SystemStatus(ctx context.Context) (*wire.SystemStatusResponse, error) {
	req := &wire.Envelope{Body: &wire.Envelope_SystemStatusReq{SystemStatusReq: &wire.SystemStatusRequest{}}}
	env, err := c.roundtrip(ctx, req, func(e *wire.Envelope) bool { return e.GetSystemStatusResp() != nil })
	if err != nil {
		return nil, err
	}
	resp := env.GetSystemStatusResp()
	if !resp.GetOk() {
		return resp, fmt.Errorf("inv-driver: %s", resp.GetError())
	}
	return resp, nil
}

// Events queries the inv-driver append-only events log.
func (c *Controller) Events(ctx context.Context, req *wire.EventsRequest) (*wire.EventsResponse, error) {
	env, err := c.roundtrip(ctx,
		&wire.Envelope{Body: &wire.Envelope_EventsReq{EventsReq: req}},
		func(e *wire.Envelope) bool { return e.GetEventsResp() != nil })
	if err != nil {
		return nil, err
	}
	resp := env.GetEventsResp()
	if !resp.GetOk() {
		return resp, fmt.Errorf("inv-driver: %s", resp.GetError())
	}
	return resp, nil
}

// GetSettings reads the current ECU settings from inv-driver.
func (c *Controller) GetSettings(ctx context.Context) (*wire.SettingsResponse, error) {
	req := &wire.Envelope{Body: &wire.Envelope_SettingsReq{SettingsReq: &wire.SettingsRequest{
		Op: &wire.SettingsRequest_Get{Get: &wire.Empty{}},
	}}}
	env, err := c.roundtrip(ctx, req, func(e *wire.Envelope) bool { return e.GetSettingsResp() != nil })
	if err != nil {
		return nil, err
	}
	resp := env.GetSettingsResp()
	if !resp.GetOk() {
		return resp, fmt.Errorf("inv-driver: %s", resp.GetError())
	}
	return resp, nil
}

// GridProfile issues one grid-profile management op (the request carries
// exactly one oneof field) and returns inv-driver's response. A response
// with ok=false is returned alongside a non-nil error; the response may
// still carry a Json payload (e.g. select_base returns reconcile reports
// even when one or more points are not confirmed).
func (c *Controller) GridProfile(ctx context.Context, req *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
	env, err := c.roundtrip(ctx,
		&wire.Envelope{Body: &wire.Envelope_GridProfileReq{GridProfileReq: req}},
		func(e *wire.Envelope) bool { return e.GetGridProfileResp() != nil })
	if err != nil {
		return nil, err
	}
	resp := env.GetGridProfileResp()
	if !resp.GetOk() {
		return resp, fmt.Errorf("inv-driver: %s", resp.GetError())
	}
	return resp, nil
}

// Pairing issues one pairing op (the request carries exactly one oneof
// field) and returns inv-driver's response. Like GridProfile it is
// controller-gated. A response with ok=false is returned alongside a
// non-nil error; the response may still carry StatusJson (the in-memory
// PairingStatus snapshot — the source of truth for the progress drawer).
func (c *Controller) Pairing(ctx context.Context, req *wire.PairingRequest) (*wire.PairingResponse, error) {
	env, err := c.roundtrip(ctx,
		&wire.Envelope{Body: &wire.Envelope_PairingReq{PairingReq: req}},
		func(e *wire.Envelope) bool { return e.GetPairingResp() != nil })
	if err != nil {
		return nil, err
	}
	resp := env.GetPairingResp()
	if !resp.GetOk() {
		return resp, fmt.Errorf("inv-driver: %s", resp.GetError())
	}
	return resp, nil
}

// SetSettings writes ECU settings to inv-driver and returns the values in
// effect afterwards.
func (c *Controller) SetSettings(ctx context.Context, s *wire.Settings) (*wire.SettingsResponse, error) {
	req := &wire.Envelope{Body: &wire.Envelope_SettingsReq{SettingsReq: &wire.SettingsRequest{
		Op: &wire.SettingsRequest_Set{Set: s},
	}}}
	env, err := c.roundtrip(ctx, req, func(e *wire.Envelope) bool { return e.GetSettingsResp() != nil })
	if err != nil {
		return nil, err
	}
	resp := env.GetSettingsResp()
	if !resp.GetOk() {
		return resp, fmt.Errorf("inv-driver: %s", resp.GetError())
	}
	return resp, nil
}

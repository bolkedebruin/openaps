package uds

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/bolke/inv-driver/wire"
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

func (c *Controller) dialHello(ctx context.Context) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.SocketPath)
	if err != nil {
		return nil, err
	}
	// ROLE_UNSPECIFIED runs the publisher loop, which is where the
	// request/response handlers live.
	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend:     c.Backend,
		Version:     c.Version,
		Hostname:    c.Hostname,
		StartedAtMs: time.Now().UnixMilli(),
	}}}
	_ = conn.SetWriteDeadline(time.Now().Add(controllerTimeout))
	if err := wire.WriteFrame(conn, hello); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("hello: %w", err)
	}
	return conn, nil
}

// roundtrip dials, writes one request envelope, and returns the first
// response envelope for which match returns true. Unrelated frames are
// skipped until the deadline.
func (c *Controller) roundtrip(ctx context.Context, req *wire.Envelope, match func(*wire.Envelope) bool) (*wire.Envelope, error) {
	conn, err := c.dialHello(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(controllerTimeout))
	if err := wire.WriteFrame(conn, req); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}
	deadline := time.Now().Add(controllerTimeout)
	for {
		_ = conn.SetReadDeadline(deadline)
		var env wire.Envelope
		if err := wire.ReadFrame(conn, &env); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		if match(&env) {
			return &env, nil
		}
	}
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

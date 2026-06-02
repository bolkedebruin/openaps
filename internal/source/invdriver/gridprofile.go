// Grid-profile IPC proxy methods on Client. Each method opens a
// short-lived control connection (same dial+Hello pattern as Send),
// writes one GridProfileRequest envelope, reads one GridProfileResponse,
// and returns a parsed result to the caller.
package invdriver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/bolke/inv-driver/wire"
)

// ProfileSummary mirrors the lightweight wire shape that Manager.listProfiles
// emits: id, vnom_v, source, and point_count. The full points array is never
// sent over the wire, so PointCount is read directly from the JSON field.
type ProfileSummary struct {
	ID     string `json:"id"`
	VNomV  float64 `json:"vnom_v,omitempty"`
	Source struct {
		System  string `json:"system,omitempty"`
		Ref     string `json:"ref,omitempty"`
		Version string `json:"version,omitempty"`
	} `json:"source,omitempty"`
	PointCount int `json:"point_count"`
}

// gridProfileCall opens a one-shot control connection, sends Hello +
// one GridProfileRequest envelope, reads exactly one GridProfileResponse
// envelope, and returns it. The connection is closed on return.
//
// Honor controlDialTimeout for both dial and the combined write+read.
func (c *Client) gridProfileCall(ctx context.Context, req *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
	// Give the round-trip a small budget beyond the dial timeout.
	const callTimeout = 5 * time.Second

	d := net.Dialer{Timeout: controlDialTimeout}
	conn, err := d.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("invdriver: gridprofile dial: %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(callTimeout))

	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: c.newHello(wire.Role_ROLE_UNSPECIFIED)}}
	if err := wire.WriteFrame(conn, hello); err != nil {
		return nil, fmt.Errorf("invdriver: gridprofile hello: %w", err)
	}

	reqEnv := &wire.Envelope{Body: &wire.Envelope_GridProfileReq{GridProfileReq: req}}
	if err := wire.WriteFrame(conn, reqEnv); err != nil {
		return nil, fmt.Errorf("invdriver: gridprofile write req: %w", err)
	}

	var respEnv wire.Envelope
	if err := wire.ReadFrame(conn, &respEnv); err != nil {
		return nil, fmt.Errorf("invdriver: gridprofile read resp: %w", err)
	}

	resp := respEnv.GetGridProfileResp()
	if resp == nil {
		return nil, fmt.Errorf("invdriver: gridprofile: unexpected response type %T", respEnv.GetBody())
	}
	return resp, nil
}

// checkResp returns the response if Ok, or an error wrapping the server
// error message.
func checkResp(op string, resp *wire.GridProfileResponse) error {
	if !resp.GetOk() {
		return fmt.Errorf("invdriver: %s: %s", op, resp.GetError())
	}
	return nil
}

// ListProfiles returns summaries of all stored base profiles.
func (c *Client) ListProfiles(ctx context.Context) ([]ProfileSummary, error) {
	resp, err := c.gridProfileCall(ctx, &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_ListProfiles{ListProfiles: &wire.Empty{}},
	})
	if err != nil {
		return nil, err
	}
	if err := checkResp("ListProfiles", resp); err != nil {
		return nil, err
	}
	var out []ProfileSummary
	if err := json.Unmarshal(resp.GetJson(), &out); err != nil {
		return nil, fmt.Errorf("invdriver: ListProfiles: unmarshal: %w", err)
	}
	return out, nil
}

// RefreshProfiles reloads profiles from the server's configured
// profiles directory and returns the updated list.
func (c *Client) RefreshProfiles(ctx context.Context) ([]ProfileSummary, error) {
	resp, err := c.gridProfileCall(ctx, &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_RefreshProfiles{RefreshProfiles: &wire.Empty{}},
	})
	if err != nil {
		return nil, err
	}
	if err := checkResp("RefreshProfiles", resp); err != nil {
		return nil, err
	}
	var out []ProfileSummary
	if err := json.Unmarshal(resp.GetJson(), &out); err != nil {
		return nil, fmt.Errorf("invdriver: RefreshProfiles: unmarshal: %w", err)
	}
	return out, nil
}

// SelectBase sets the active base profile by id and triggers
// reconciliation of all known inverters. Returns an error if the
// profile is not found or if reconciliation fails.
func (c *Client) SelectBase(ctx context.Context, id string) error {
	resp, err := c.gridProfileCall(ctx, &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_SelectBase{SelectBase: &wire.SelectBase{Id: id}},
	})
	if err != nil {
		return err
	}
	return checkResp("SelectBase", resp)
}

// SetOverlay validates and upserts a per-inverter overlay (sparse
// override on top of the active base profile), then triggers
// reconciliation for that inverter. overlayJSON must be a valid v1
// overlay document.
func (c *Client) SetOverlay(ctx context.Context, uid string, overlayJSON []byte) error {
	resp, err := c.gridProfileCall(ctx, &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_SetOverlay{SetOverlay: &wire.OverlaySet{
			Uid:         uid,
			OverlayJson: overlayJSON,
		}},
	})
	if err != nil {
		return err
	}
	return checkResp("SetOverlay", resp)
}

// ClearOverlay removes the stored overlay for an inverter UID and
// triggers reconciliation.
func (c *Client) ClearOverlay(ctx context.Context, uid string) error {
	resp, err := c.gridProfileCall(ctx, &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_ClearOverlay{ClearOverlay: &wire.ClearOverlay{Uid: uid}},
	})
	if err != nil {
		return err
	}
	return checkResp("ClearOverlay", resp)
}

// GetEffective returns the computed effective profile for one inverter
// as a JSON object. The effective profile is the active base merged
// with any per-inverter overlay, clamped to declared ranges.
func (c *Client) GetEffective(ctx context.Context, uid string) (json.RawMessage, error) {
	resp, err := c.gridProfileCall(ctx, &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_GetEffective{GetEffective: &wire.GetEffective{Uid: uid}},
	})
	if err != nil {
		return nil, err
	}
	if err := checkResp("GetEffective", resp); err != nil {
		return nil, err
	}
	return json.RawMessage(resp.GetJson()), nil
}

// GetStatus returns a JSON object with the active base profile,
// stored profile IDs, and whether the reconciler is available.
func (c *Client) GetStatus(ctx context.Context) (json.RawMessage, error) {
	resp, err := c.gridProfileCall(ctx, &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_GetStatus{GetStatus: &wire.Empty{}},
	})
	if err != nil {
		return nil, err
	}
	if err := checkResp("GetStatus", resp); err != nil {
		return nil, err
	}
	return json.RawMessage(resp.GetJson()), nil
}

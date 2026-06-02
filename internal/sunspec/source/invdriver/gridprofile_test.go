package invdriver

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/sunspec/testhelpers"
	"github.com/bolkedebruin/openaps/wire"
)

// serveFakeGridProfile starts a fake UDS listener that accepts one
// connection, reads Hello + one GridProfileRequest, and responds with
// resp. It returns when the connection closes or the test ends.
func serveFakeGridProfile(t *testing.T, sock string, resp *wire.GridProfileResponse) chan *wire.GridProfileRequest {
	t.Helper()
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	ch := make(chan *wire.GridProfileRequest, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read Hello.
		var hello wire.Envelope
		if err := wire.ReadFrame(conn, &hello); err != nil {
			return
		}

		// Read GridProfileRequest.
		var reqEnv wire.Envelope
		if err := wire.ReadFrame(conn, &reqEnv); err != nil {
			return
		}
		ch <- reqEnv.GetGridProfileReq()

		// Write GridProfileResponse.
		respEnv := &wire.Envelope{Body: &wire.Envelope_GridProfileResp{GridProfileResp: resp}}
		_ = wire.WriteFrame(conn, respEnv)
	}()
	return ch
}

func TestGridProfileCall_SendsHelloAndRequest(t *testing.T) {
	sock := filepath.Join(testhelpers.ShortTempDir(t), "gp.sock")
	wantResp := &wire.GridProfileResponse{Ok: true, Json: []byte(`[]`)}
	ch := serveFakeGridProfile(t, sock, wantResp)

	c := New(sock, "v-test")
	req := &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_ListProfiles{ListProfiles: &wire.Empty{}},
	}

	resp, err := c.gridProfileCall(context.Background(), req)
	if err != nil {
		t.Fatalf("gridProfileCall: %v", err)
	}
	if !resp.GetOk() {
		t.Errorf("resp.Ok=false want true")
	}

	select {
	case got := <-ch:
		if got == nil {
			t.Fatal("no GridProfileRequest received by fake listener")
		}
		if _, ok := got.GetOp().(*wire.GridProfileRequest_ListProfiles); !ok {
			t.Errorf("op type=%T want *GridProfileRequest_ListProfiles", got.GetOp())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for request")
	}
}

func TestGridProfileCall_ErrorResponse(t *testing.T) {
	sock := filepath.Join(testhelpers.ShortTempDir(t), "gperr.sock")
	wantResp := &wire.GridProfileResponse{Ok: false, Error: "profile not found"}
	serveFakeGridProfile(t, sock, wantResp)

	c := New(sock, "v-test")
	req := &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_SelectBase{SelectBase: &wire.SelectBase{Id: "nonexistent"}},
	}
	resp, err := c.gridProfileCall(context.Background(), req)
	if err != nil {
		t.Fatalf("gridProfileCall transport error: %v", err)
	}
	if resp.GetOk() {
		t.Error("expected Ok=false, got true")
	}
	if resp.GetError() != "profile not found" {
		t.Errorf("error=%q want %q", resp.GetError(), "profile not found")
	}
}

func TestListProfiles_ParsesSummary(t *testing.T) {
	sock := filepath.Join(testhelpers.ShortTempDir(t), "list.sock")
	// Wire shape matches the lightweight summary emitted by Manager.listProfiles:
	// point_count is a top-level integer, not a "points" array.
	profilesJSON := []byte(`[
		{"id":"EN50549-1","vnom_v":230,"source":{"system":"apsystems","ref":"TY=36","version":"2.1.29"},"point_count":2}
	]`)
	serveFakeGridProfile(t, sock, &wire.GridProfileResponse{Ok: true, Json: profilesJSON})

	c := New(sock, "v-test")
	profiles, err := c.ListProfiles(context.Background())
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("got %d profiles, want 1", len(profiles))
	}
	p := profiles[0]
	if p.ID != "EN50549-1" {
		t.Errorf("id=%q want EN50549-1", p.ID)
	}
	if p.VNomV != 230 {
		t.Errorf("vnom_v=%.1f want 230", p.VNomV)
	}
	if p.Source.Ref != "TY=36" {
		t.Errorf("source.ref=%q want TY=36", p.Source.Ref)
	}
	if p.PointCount != 2 {
		t.Errorf("point_count=%d want 2", p.PointCount)
	}
}

func TestListProfiles_ErrorPropagated(t *testing.T) {
	sock := filepath.Join(testhelpers.ShortTempDir(t), "listerr.sock")
	serveFakeGridProfile(t, sock, &wire.GridProfileResponse{Ok: false, Error: "store not open"})

	c := New(sock, "v-test")
	_, err := c.ListProfiles(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetEffective_ReturnsRawJSON(t *testing.T) {
	sock := filepath.Join(testhelpers.ShortTempDir(t), "eff.sock")
	payload := []byte(`{"inverter_uid":"999900000003","base_id":"EN50549-1","points":[]}`)
	serveFakeGridProfile(t, sock, &wire.GridProfileResponse{Ok: true, Json: payload})

	c := New(sock, "v-test")
	raw, err := c.GetEffective(context.Background(), "999900000003")
	if err != nil {
		t.Fatalf("GetEffective: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := m["inverter_uid"]; !ok {
		t.Error("result missing inverter_uid key")
	}
}

func TestGetStatus_ReturnsRawJSON(t *testing.T) {
	sock := filepath.Join(testhelpers.ShortTempDir(t), "status.sock")
	payload := []byte(`{"active_base":"EN50549-1","stored_profiles":["EN50549-1"],"reconciler_ready":false}`)
	serveFakeGridProfile(t, sock, &wire.GridProfileResponse{Ok: true, Json: payload})

	c := New(sock, "v-test")
	raw, err := c.GetStatus(context.Background())
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	var s struct {
		ActiveBase string `json:"active_base"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.ActiveBase != "EN50549-1" {
		t.Errorf("active_base=%q want EN50549-1", s.ActiveBase)
	}
}

func TestSelectBase_RoundTrip(t *testing.T) {
	sock := filepath.Join(testhelpers.ShortTempDir(t), "sel.sock")
	ch := serveFakeGridProfile(t, sock, &wire.GridProfileResponse{Ok: true})

	c := New(sock, "v-test")
	if err := c.SelectBase(context.Background(), "EN50549-1"); err != nil {
		t.Fatalf("SelectBase: %v", err)
	}

	select {
	case got := <-ch:
		sb, ok := got.GetOp().(*wire.GridProfileRequest_SelectBase)
		if !ok {
			t.Fatalf("op type=%T want SelectBase", got.GetOp())
		}
		if sb.SelectBase.GetId() != "EN50549-1" {
			t.Errorf("id=%q want EN50549-1", sb.SelectBase.GetId())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestSetOverlay_RoundTrip(t *testing.T) {
	sock := filepath.Join(testhelpers.ShortTempDir(t), "ovl.sock")
	ch := serveFakeGridProfile(t, sock, &wire.GridProfileResponse{Ok: true})

	c := New(sock, "v-test")
	ovlJSON := []byte(`{"schema":"invdriver.gridprofile/v1","id":"custom","uids":["999900000003"],"points":[]}`)
	if err := c.SetOverlay(context.Background(), "999900000003", ovlJSON); err != nil {
		t.Fatalf("SetOverlay: %v", err)
	}

	select {
	case got := <-ch:
		so, ok := got.GetOp().(*wire.GridProfileRequest_SetOverlay)
		if !ok {
			t.Fatalf("op type=%T want SetOverlay", got.GetOp())
		}
		if so.SetOverlay.GetUid() != "999900000003" {
			t.Errorf("uid=%q want 999900000003", so.SetOverlay.GetUid())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestClearOverlay_RoundTrip(t *testing.T) {
	sock := filepath.Join(testhelpers.ShortTempDir(t), "clr.sock")
	ch := serveFakeGridProfile(t, sock, &wire.GridProfileResponse{Ok: true})

	c := New(sock, "v-test")
	if err := c.ClearOverlay(context.Background(), "999900000003"); err != nil {
		t.Fatalf("ClearOverlay: %v", err)
	}

	select {
	case got := <-ch:
		co, ok := got.GetOp().(*wire.GridProfileRequest_ClearOverlay)
		if !ok {
			t.Fatalf("op type=%T want ClearOverlay", got.GetOp())
		}
		if co.ClearOverlay.GetUid() != "999900000003" {
			t.Errorf("uid=%q want 999900000003", co.ClearOverlay.GetUid())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestGridProfileCall_DialError(t *testing.T) {
	c := New(filepath.Join(testhelpers.ShortTempDir(t), "absent.sock"), "v-test")
	_, err := c.gridProfileCall(context.Background(), &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_GetStatus{GetStatus: &wire.Empty{}},
	})
	if err == nil {
		t.Fatal("expected dial error, got nil")
	}
}

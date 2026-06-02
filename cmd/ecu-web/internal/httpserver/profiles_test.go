package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/cmd/ecu-web/internal/snapshot"
	"github.com/bolkedebruin/openaps/internal/gridprofile"
	"github.com/bolkedebruin/openaps/wire"
)

// fakeProfiles answers the read ops (get_status, list_profiles, list_overlays)
// with canned JSON.
func fakeProfiles(_ context.Context, req *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
	switch {
	case req.GetGetStatus() != nil:
		return &wire.GridProfileResponse{Ok: true, Json: []byte(
			`{"active_base":"EN50549-1","reconciler_ready":true}`)}, nil
	case req.GetListProfiles() != nil:
		return &wire.GridProfileResponse{Ok: true, Json: []byte(
			`[{"id":"EN50549-1","vnom_v":230,"source":{"ref":"TY=36"},"point_count":13}]`)}, nil
	case req.GetListOverlays() != nil:
		return &wire.GridProfileResponse{Ok: true, Json: []byte(
			`[{"schema":"invdriver.gridprofile/v1","id":"victron-shift","uids":["999900000001"],` +
				`"points":[{"model":134,"group":"CrvSet","point":"Hz3","native":{"value":50.3,"unit":"Hz"},"apply":{"aps_code":"CB"}}]}]`)}, nil
	case req.GetGetBase() != nil:
		return &wire.GridProfileResponse{Ok: true, Json: []byte(
			`{"CB":{"value":50.2,"unit":"Hz","min":50.1,"max":52.0},"AF":{"value":52.0,"unit":"Hz"}}`)}, nil
	}
	return &wire.GridProfileResponse{Ok: false, Error: "unexpected op"}, errors.New("unexpected op")
}

func snapWith(invs map[string]uint8) *snapshot.Snapshot {
	s := snapshot.New(nil)
	for uid, mc := range invs {
		s.ApplyTelemetry(&wire.Telemetry{PeerUid: uid, TsMs: time.Now().UnixMilli()})
		m := uint32(mc)
		s.ApplyInfo(&wire.InverterInfo{PeerUid: uid, ModelCode: &m})
	}
	return s
}

func withCookies(r *http.Request, cookies []*http.Cookie) *http.Request {
	for _, c := range cookies {
		r.AddCookie(c)
	}
	return r
}

func TestGetProfilesEndpoint(t *testing.T) {
	ds3, qs1a := "999900000001", "999900000003"
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: fakeProfiles,
		Snap:          snapWith(map[string]uint8{ds3: codec.ModelDS3, qs1a: codec.ModelQS1A}),
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(httptest.NewRequest("GET", "/api/profiles", nil), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("profiles => %d (%s)", rec.Code, rec.Body)
	}
	var out profilesDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Base.ActiveBase != "EN50549-1" || !out.Base.ReconcilerReady || len(out.Base.Profiles) != 1 {
		t.Errorf("base wrong: %+v", out.Base)
	}
	if len(out.Overlays) != 1 || out.Overlays[0].ID != "victron-shift" || out.Overlays[0].Points[0].ApsCode != "CB" {
		t.Errorf("overlays wrong: %+v", out.Overlays)
	}
	if len(out.Params) == 0 {
		t.Error("empty param catalog")
	}
	if out.BaseDefaults["CB"].Value != 50.2 || out.BaseDefaults["CB"].Unit != "Hz" {
		t.Errorf("base default for CB wrong: %+v", out.BaseDefaults["CB"])
	}
	if out.BaseDefaults["CB"].Min == nil || *out.BaseDefaults["CB"].Min != 50.1 {
		t.Errorf("base range for CB missing: %+v", out.BaseDefaults["CB"])
	}
	if len(out.Inverters) != 2 {
		t.Fatalf("want 2 inverters, got %d", len(out.Inverters))
	}
	cap := map[string]map[string]bool{}
	for _, inv := range out.Inverters {
		set := map[string]bool{}
		for _, c := range inv.WritableCodes {
			set[c] = true
		}
		cap[inv.UID] = set
	}
	// Capability filter: DD (freq-watt slope) is writable on DS3 but a QS1A
	// firmware hard-reject; CB is writable on both.
	if !cap[ds3]["DD"] {
		t.Error("expected DD writable on DS3")
	}
	if cap[qs1a]["DD"] {
		t.Error("DD must NOT be writable on QS1A (firmware hard-reject)")
	}
	if !cap[ds3]["CB"] || !cap[qs1a]["CB"] {
		t.Error("CB should be writable on both families")
	}
	if len(cap[qs1a]) == 0 {
		t.Error("QS1A should have some writable codes")
	}
}

func TestGetProfilesDegradesButKeepsLocalData(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: func(context.Context, *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
			return nil, errors.New("inv-driver down")
		},
		Snap: snapWith(map[string]uint8{"999900000001": codec.ModelDS3}),
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(httptest.NewRequest("GET", "/api/profiles", nil), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("should degrade to 200, got %d", rec.Code)
	}
	var out profilesDTO
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Error == "" {
		t.Error("expected error field")
	}
	if len(out.Params) == 0 || len(out.Inverters) != 1 {
		t.Errorf("local data (params/inverters) should still be present: %+v", out)
	}
}

func TestPutOverlayBuildsValidDocAndAppliesPerUID(t *testing.T) {
	a, b := "999900000001", "999900000003"
	var seen []*wire.OverlaySet
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: func(_ context.Context, req *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
			if so := req.GetSetOverlay(); so != nil {
				seen = append(seen, so)
				return &wire.GridProfileResponse{Ok: true, Json: []byte(`{}`)}, nil
			}
			return &wire.GridProfileResponse{Ok: false}, errors.New("unexpected op")
		},
	})
	body := `{"id":"victron-shift","uids":["` + a + `","` + b + `"],"points":[{"aps_code":"CB","value":50.3}]}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("PUT", "/api/profiles/overlay", body), cookies))
	// PUT now returns 202 Accepted because inv-driver runs the per-UID apply
	// asynchronously and surfaces outcomes via the events log.
	if rec.Code != http.StatusAccepted {
		t.Fatalf("put overlay => %d (%s); want 202 Accepted", rec.Code, rec.Body)
	}
	if len(seen) != 2 {
		t.Fatalf("want set_overlay for 2 uids, got %d", len(seen))
	}
	// The overlay doc built server-side must validate, and uid must be listed.
	if err := gridprofile.ValidateOverlay(seen[0].GetOverlayJson()); err != nil {
		t.Errorf("built overlay fails schema validation: %v", err)
	}
	var ov gridprofile.Overlay
	json.Unmarshal(seen[0].GetOverlayJson(), &ov)
	if ov.ID != "victron-shift" || len(ov.UIDs) != 2 || ov.Points[0].Apply.ApsCode != "CB" || ov.Points[0].Native.Value != 50.3 {
		t.Errorf("overlay doc wrong: %+v", ov)
	}
	if seen[0].GetUid() != a || seen[1].GetUid() != b {
		t.Errorf("set_overlay uids: got %q,%q", seen[0].GetUid(), seen[1].GetUid())
	}
	// Response shape: {id, status:"queued", uids:[...]}, no per-UID results.
	var qr struct {
		ID     string   `json:"id"`
		Status string   `json:"status"`
		UIDs   []string `json:"uids"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &qr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if qr.ID != "victron-shift" || qr.Status != "queued" || len(qr.UIDs) != 2 {
		t.Errorf("queued response wrong: %+v", qr)
	}
}

// TestSetOverlay_PartialQueueAllReturned verifies the partial-result loop:
// when inv-driver rejects one uid but accepts the other, the handler does
// NOT bail on the first error — it returns 202 with the queued uid in uids[]
// and the rejected uid in failed[]. Closes the regression where the first
// non-OK reply 400'd the whole request and earlier successes vanished from
// the response.
func TestSetOverlay_PartialQueueAllReturned(t *testing.T) {
	a := "999900000001"
	b := "999900000003"
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: func(_ context.Context, req *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
			so := req.GetSetOverlay()
			if so == nil {
				return &wire.GridProfileResponse{Ok: false}, errors.New("unexpected op")
			}
			if so.GetUid() == b {
				// inv-driver rejects this uid (e.g. fleet-membership check).
				return &wire.GridProfileResponse{Ok: false, Error: "uid not in fleet"}, nil
			}
			return &wire.GridProfileResponse{Ok: true, Json: []byte(`{}`)}, nil
		},
	})
	body := `{"id":"victron-shift","uids":["` + a + `","` + b + `"],"points":[{"aps_code":"CB","value":50.3}]}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("PUT", "/api/profiles/overlay", body), cookies))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("partial-queue should still return 202 (at least one queued); got %d: %s", rec.Code, rec.Body)
	}
	var qr struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		UIDs   []string
		Failed []struct {
			UID   string `json:"uid"`
			Error string `json:"error"`
		} `json:"failed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &qr); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body)
	}
	if len(qr.UIDs) != 1 || qr.UIDs[0] != a {
		t.Errorf("uids[] should hold only the queued uid (%s); got %v", a, qr.UIDs)
	}
	if len(qr.Failed) != 1 || qr.Failed[0].UID != b {
		t.Fatalf("failed[] should hold the rejected uid (%s); got %+v", b, qr.Failed)
	}
	if !strings.Contains(qr.Failed[0].Error, "not in fleet") {
		t.Errorf("failed[].error should carry inv-driver's reason; got %q", qr.Failed[0].Error)
	}
}

func TestPutOverlayUnknownCode(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{GridProfileFn: fakeProfiles})
	body := `{"id":"x","uids":["999900000001"],"points":[{"aps_code":"ZZ","value":1}]}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("PUT", "/api/profiles/overlay", body), cookies))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown code => %d, want 400", rec.Code)
	}
}

func TestPutOverlayValidation(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{GridProfileFn: fakeProfiles})
	for _, body := range []string{
		`{"id":"","uids":["999900000001"],"points":[{"aps_code":"CB","value":1}]}`, // no id
		`{"id":"x","uids":[],"points":[{"aps_code":"CB","value":1}]}`,              // no targets
		`{"id":"x","uids":["999900000001"],"points":[]}`,                          // no points
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookies(jsonReq("PUT", "/api/profiles/overlay", body), cookies))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %q => %d, want 400", body, rec.Code)
		}
	}
}

func TestDeleteOverlay(t *testing.T) {
	var cleared []string
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: func(_ context.Context, req *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
			if co := req.GetClearOverlay(); co != nil {
				cleared = append(cleared, co.GetUid())
				return &wire.GridProfileResponse{Ok: true}, nil
			}
			return &wire.GridProfileResponse{Ok: false}, errors.New("unexpected op")
		},
	})
	body := `{"id":"victron-shift","uids":["999900000001","999900000003"]}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("DELETE", "/api/profiles/overlay", body), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete overlay => %d (%s)", rec.Code, rec.Body)
	}
	if len(cleared) != 2 {
		t.Errorf("want clear_overlay for 2 uids, got %v", cleared)
	}
}

func TestSelectBase(t *testing.T) {
	var gotID string
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: func(_ context.Context, req *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
			if sb := req.GetSelectBase(); sb != nil {
				gotID = sb.GetId()
				return &wire.GridProfileResponse{Ok: true, Json: []byte(`[]`)}, nil
			}
			return &wire.GridProfileResponse{Ok: false}, errors.New("unexpected op")
		},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/profiles/base", `{"id":"EN50549-1"}`), cookies))
	if rec.Code != http.StatusOK || gotID != "EN50549-1" {
		t.Fatalf("select base => %d, id=%q", rec.Code, gotID)
	}
}

func TestSelectBaseRejected(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: func(context.Context, *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
			return &wire.GridProfileResponse{Ok: false, Error: "not confirmed: CB=mismatch"},
				errors.New("inv-driver: not confirmed: CB=mismatch")
		},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/profiles/base", `{"id":"EN50549-1"}`), cookies))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "not confirmed") {
		t.Fatalf("rejected base => %d (%s)", rec.Code, rec.Body)
	}
}

func TestGetOverlaysEndpoint(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{GridProfileFn: fakeProfiles})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(httptest.NewRequest("GET", "/api/overlays", nil), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("overlays => %d (%s)", rec.Code, rec.Body)
	}
	var out []localSiteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != "victron-shift" || out[0].UIDs[0] != "999900000001" {
		t.Errorf("overlays wrong: %+v", out)
	}
	if out[0].Points[0].ApsCode != "CB" {
		t.Errorf("overlay point wrong: %+v", out[0].Points)
	}
}

func TestGetOverlaysDegradesToEmpty(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: func(context.Context, *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
			return nil, errors.New("inv-driver down")
		},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(httptest.NewRequest("GET", "/api/overlays", nil), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("should degrade to 200, got %d", rec.Code)
	}
	var out []localSiteDTO
	json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out) != 0 {
		t.Errorf("want empty list on error, got %+v", out)
	}
}

func TestCurrentValues(t *testing.T) {
	m := currentValues(&wire.Protection{Values: map[string]float64{
		"DC": 50.2,  // read alias of the curtailment start -> aliased to CA
		"DD": 16.57, // slope
		"AF": 52.0,  // over-freq trip
		"AB": 253.0, // 10-min avg overvoltage
	}})
	if m["CA"] != 50.2 {
		t.Errorf("CA (DC alias) = %v, want 50.2", m["CA"])
	}
	if m["DD"] != 16.57 || m["AF"] != 52.0 || m["AB"] != 253.0 {
		t.Errorf("values not passed through: %+v", m)
	}
	if _, ok := m["AC"]; ok {
		t.Errorf("absent code should not appear: %+v", m["AC"])
	}
}

func TestProfilesRequireAuth(t *testing.T) {
	_, h := newTestServer(t)
	for _, ep := range []struct{ m, p string }{
		{"GET", "/api/profiles"},
		{"GET", "/api/overlays"},
		{"POST", "/api/profiles/base"},
		{"PUT", "/api/profiles/overlay"},
		{"DELETE", "/api/profiles/overlay"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(ep.m, ep.p, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s => %d, want 401", ep.m, ep.p, rec.Code)
		}
	}
}

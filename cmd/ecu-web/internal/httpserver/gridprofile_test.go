package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bolke/inv-driver/wire"
)

// fakeGridProfile answers get_status and list_profiles with canned JSON,
// matching the shapes inv-driver's grid-profile manager returns.
func fakeGridProfile(_ context.Context, req *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
	switch {
	case req.GetGetStatus() != nil:
		return &wire.GridProfileResponse{Ok: true, Json: []byte(
			`{"active_base":"EN50549-1","reconciler_ready":true,"stored_profiles":["EN50549-1","CEI-0-21"]}`)}, nil
	case req.GetListProfiles() != nil:
		return &wire.GridProfileResponse{Ok: true, Json: []byte(
			`[{"id":"EN50549-1","vnom_v":230,"source":{"ref":"TY=36"},"point_count":13},` +
				`{"id":"CEI-0-21","vnom_v":230,"source":{"ref":"TY=10"},"point_count":12}]`)}, nil
	}
	return &wire.GridProfileResponse{Ok: false, Error: "unexpected op"}, errors.New("unexpected op")
}

func TestGetGridProfileEndpoint(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{GridProfileFn: fakeGridProfile})
	r := httptest.NewRequest("GET", "/api/gridprofile", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("gridprofile => %d (%s)", rec.Code, rec.Body)
	}
	var out gridProfileDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.ActiveBase != "EN50549-1" || !out.ReconcilerReady {
		t.Errorf("status not merged: %+v", out)
	}
	if len(out.Profiles) != 2 {
		t.Fatalf("want 2 profiles, got %d", len(out.Profiles))
	}
	p := out.Profiles[0]
	if p.ID != "EN50549-1" || p.VNomV != 230 || p.SourceRef != "TY=36" || p.PointCount != 13 {
		t.Errorf("profile summary wrong: %+v", p)
	}
}

func TestGetGridProfileDegradesOnError(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: func(context.Context, *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
			return nil, errors.New("inv-driver down")
		},
	})
	r := httptest.NewRequest("GET", "/api/gridprofile", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("should degrade to 200, got %d", rec.Code)
	}
	var out gridProfileDTO
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Error == "" {
		t.Errorf("expected error field, got %+v", out)
	}
}

func TestGetGridProfileUnavailable(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{}) // nil GridProfileFn
	r := httptest.NewRequest("GET", "/api/gridprofile", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("nil GridProfileFn GET => %d, want 200 (degraded)", rec.Code)
	}
	var out gridProfileDTO
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Error == "" {
		t.Errorf("expected error field for unavailable, got %+v", out)
	}
}

func TestSelectGridProfile(t *testing.T) {
	var gotID string
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: func(_ context.Context, req *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
			if sb := req.GetSelectBase(); sb != nil {
				gotID = sb.GetId()
				return &wire.GridProfileResponse{Ok: true, Json: []byte(`[]`)}, nil
			}
			return &wire.GridProfileResponse{Ok: false, Error: "unexpected op"}, errors.New("unexpected op")
		},
	})
	r := jsonReq("POST", "/api/gridprofile/select", `{"id":"EN50549-1"}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("select => %d (%s)", rec.Code, rec.Body)
	}
	if gotID != "EN50549-1" {
		t.Errorf("SelectBase id = %q, want EN50549-1", gotID)
	}
	var out map[string]string
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["active_base"] != "EN50549-1" {
		t.Errorf("response active_base = %q", out["active_base"])
	}
}

func TestSelectGridProfileRejected(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		GridProfileFn: func(context.Context, *wire.GridProfileRequest) (*wire.GridProfileResponse, error) {
			return &wire.GridProfileResponse{Ok: false, Error: "one or more points not confirmed: CB=mismatch"},
				errors.New("inv-driver: one or more points not confirmed: CB=mismatch")
		},
	})
	r := jsonReq("POST", "/api/gridprofile/select", `{"id":"EN50549-1"}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("rejected select => %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not confirmed") {
		t.Errorf("expected reconcile summary in body, got %q", rec.Body.String())
	}
}

func TestSelectGridProfileMissingID(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{GridProfileFn: fakeGridProfile})
	r := jsonReq("POST", "/api/gridprofile/select", `{}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing id => %d, want 400", rec.Code)
	}
}

func TestSelectGridProfileUnavailable(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{}) // nil GridProfileFn
	r := jsonReq("POST", "/api/gridprofile/select", `{"id":"x"}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil GridProfileFn POST => %d, want 503", rec.Code)
	}
}

func TestGetGridProfileRequiresAuth(t *testing.T) {
	_, h := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/gridprofile", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated /api/gridprofile => %d, want 401", rec.Code)
	}
}

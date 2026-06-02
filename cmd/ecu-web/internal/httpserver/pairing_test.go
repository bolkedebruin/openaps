package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bolkedebruin/openaps/wire"
)

// TestPairingChangeChannelStepUp verifies the change-channel endpoint mirrors
// the fleet-rekey gate: it requires a valid single-use step-up (POST
// /api/auth/verify) since it migrates the whole fleet's radio, passes the
// requested channel through verbatim (inv-driver / ecu-zb own validation), and
// consumes the step-up on success.
func TestPairingChangeChannelStepUp(t *testing.T) {
	var last *wire.FleetChangeChannel
	calls := 0
	h, cookies := newSettingsServer(t, Config{
		PairingFn: func(_ context.Context, req *wire.PairingRequest) (*wire.PairingResponse, error) {
			calls++
			last = req.GetChangeChannel()
			return &wire.PairingResponse{Ok: true, StatusJson: []byte(`{"op":"change_channel"}`)}, nil
		},
	})
	withCookies := func(r *http.Request) *http.Request {
		for _, c := range cookies {
			r.AddCookie(c)
		}
		return r
	}
	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/pairing/change-channel", body)))
		return rec
	}
	verify := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/auth/verify", body)))
		return rec
	}

	// WITHOUT a prior verify -> 403, PairingFn not called.
	calls = 0
	if rec := post(`{"channel":20}`); rec.Code != http.StatusForbidden {
		t.Fatalf("change-channel without step-up => %d, want 403; body=%s", rec.Code, rec.Body)
	}
	if calls != 0 {
		t.Errorf("PairingFn called %d times without step-up, want 0", calls)
	}

	// Correct verify, then change-channel -> 202 Accepted, channel passed through.
	if rec := verify(`{"password":"password123"}`); rec.Code != http.StatusOK {
		t.Fatalf("verify => %d (%s)", rec.Code, rec.Body)
	}
	calls = 0
	if rec := post(`{"channel":20}`); rec.Code != http.StatusAccepted {
		t.Fatalf("change-channel after verify => %d, want 202 (%s)", rec.Code, rec.Body)
	}
	if calls != 1 {
		t.Fatalf("PairingFn called %d times, want 1", calls)
	}
	if last == nil || last.GetChannel() != 20 {
		t.Errorf("FleetChangeChannel.channel = %v, want 20", last)
	}

	// Step-up is single-use: a second change-channel inside the TTL is refused.
	calls = 0
	if rec := post(`{"channel":18}`); rec.Code != http.StatusForbidden {
		t.Errorf("second change-channel inside TTL => %d, want 403 (step-up consumed)", rec.Code)
	}
	if calls != 0 {
		t.Errorf("PairingFn called %d times after step-up consumed, want 0", calls)
	}
}

// TestPairingChangeChannelPassThrough confirms ecu-web does NOT validate the
// channel range: an out-of-range value still reaches PairingFn (the backend
// rejects it), and a backend rejection is surfaced as ok=false without
// consuming the step-up.
func TestPairingChangeChannelPassThrough(t *testing.T) {
	var last *wire.FleetChangeChannel
	h, cookies := newSettingsServer(t, Config{
		PairingFn: func(_ context.Context, req *wire.PairingRequest) (*wire.PairingResponse, error) {
			last = req.GetChangeChannel()
			return &wire.PairingResponse{Ok: false, Error: "channel out of range"}, nil
		},
	})
	withCookies := func(r *http.Request) *http.Request {
		for _, c := range cookies {
			r.AddCookie(c)
		}
		return r
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/auth/verify", `{"password":"password123"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("verify => %d", rec.Code)
	}

	// Out-of-range channel reaches the backend (no client-side range gate).
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/pairing/change-channel", `{"channel":30}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("backend reject => %d, want 400 (%s)", rec.Code, rec.Body)
	}
	if last == nil || last.GetChannel() != 30 {
		t.Errorf("channel not passed through: %v", last)
	}

	// The step-up was NOT consumed on a failed op: a follow-up succeeds.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/pairing/change-channel", `{"channel":15}`)))
	// PairingFn still returns ok=false here, so this is 400 — but the point is
	// the step-up gate let it THROUGH (not 403), proving it wasn't consumed.
	if rec.Code == http.StatusForbidden {
		t.Errorf("follow-up => 403, want the step-up to have survived the failed op")
	}
}

// TestPairingChangeChannelUnavailable confirms a nil PairingFn yields 503.
func TestPairingChangeChannelUnavailable(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{}) // no PairingFn
	r := jsonReq("POST", "/api/pairing/change-channel", `{"channel":20}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil PairingFn => %d, want 503", rec.Code)
	}
}

package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bolkedebruin/openaps/wire"
)

func TestHandleSetInverterPhase_HTTP(t *testing.T) {
	var gotUID string
	var gotLeg uint32
	okFn := func(_ context.Context, uid string, leg uint32) (*wire.SetInverterPhaseResponse, error) {
		gotUID, gotLeg = uid, leg
		return &wire.SetInverterPhaseResponse{Ok: true}, nil
	}
	h, cookies := newSettingsServer(t, Config{SetInverterPhaseFn: okFn})

	do := func(hh http.Handler, cs []*http.Cookie, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		hh.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/inverters/phase", body), cs))
		return rec
	}

	// Happy path forwards uid + leg.
	if rec := do(h, cookies, `{"uid":"800000000001","leg":2}`); rec.Code != http.StatusOK {
		t.Fatalf("ok path => %d (%s)", rec.Code, rec.Body.String())
	}
	if gotUID != "800000000001" || gotLeg != 2 {
		t.Fatalf("fn got uid=%q leg=%d; want 800000000001,2", gotUID, gotLeg)
	}

	// Leg out of range and missing uid are 400 before reaching the fn.
	if rec := do(h, cookies, `{"uid":"800000000001","leg":9}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad leg => %d", rec.Code)
	}
	if rec := do(h, cookies, `{"leg":1}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing uid => %d", rec.Code)
	}

	// inv-driver rejection (e.g. three-phase) surfaces as 400 with its message.
	rejFn := func(_ context.Context, _ string, _ uint32) (*wire.SetInverterPhaseResponse, error) {
		return &wire.SetInverterPhaseResponse{Ok: false, Error: "inverter is not single-phase"},
			fmt.Errorf("inv-driver: inverter is not single-phase")
	}
	hRej, cRej := newSettingsServer(t, Config{SetInverterPhaseFn: rejFn})
	rec := do(hRej, cRej, `{"uid":"800000000032","leg":1}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "single-phase") {
		t.Fatalf("reject => %d body=%q", rec.Code, rec.Body.String())
	}

	// Unwired handler returns 503.
	hNil, cNil := newSettingsServer(t, Config{})
	if rec := do(hNil, cNil, `{"uid":"x","leg":1}`); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unwired => %d", rec.Code)
	}
}

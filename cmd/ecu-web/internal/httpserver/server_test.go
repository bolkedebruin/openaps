package httpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bolkedebruin/openaps/cmd/ecu-web/internal/auth"
	"github.com/bolkedebruin/openaps/cmd/ecu-web/internal/snapshot"
	"github.com/bolkedebruin/openaps/wire"
)

func newTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	authMgr, err := auth.NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	snap := snapshot.New(nil)
	snap.ApplyTelemetry(&wire.Telemetry{PeerUid: "u", TsMs: time.Now().UnixMilli(), ActivePowerW: 250})

	assets := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>ecu-web</title>")},
		"assets/app.js": {Data: []byte("// app")},
	}
	srv := New(Config{
		StateDir: t.TempDir(),
		Assets:   assets,
		Snap:     snap,
		Auth:     authMgr,
		Conn:     func() bool { return true },
	})
	return srv, srv.Handler()
}

func TestFleetRequiresAuth(t *testing.T) {
	_, h := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/fleet", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /api/fleet => %d, want 401", rec.Code)
	}
}

func TestSetupThenFleet(t *testing.T) {
	_, h := newTestServer(t)

	// status: unconfigured
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/auth/status", nil))
	var st map[string]bool
	json.Unmarshal(rec.Body.Bytes(), &st)
	if st["configured"] {
		t.Fatal("expected unconfigured")
	}

	// setup
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/setup", `{"password":"password123"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("setup => %d (%s)", rec.Code, rec.Body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("setup did not set a session cookie")
	}

	// fleet with cookie
	rec = httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/fleet", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("authed /api/fleet => %d", rec.Code)
	}
	var f snapshot.FleetDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &f); err != nil {
		t.Fatalf("fleet json: %v", err)
	}
	if f.InverterCount != 1 {
		t.Errorf("InverterCount = %d, want 1", f.InverterCount)
	}
}

func TestSetupRejectedWhenConfigured(t *testing.T) {
	_, h := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/setup", `{"password":"password123"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("first setup => %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/setup", `{"password":"another-one"}`))
	if rec.Code != http.StatusConflict {
		t.Errorf("second setup => %d, want 409", rec.Code)
	}
}

func TestStaticIndexServedAndSPAFallback(t *testing.T) {
	_, h := newTestServer(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ecu-web") {
		t.Fatalf("root => %d body=%q", rec.Code, rec.Body)
	}

	// deep link to a client route falls back to index.html
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/settings", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ecu-web") {
		t.Fatalf("spa fallback => %d body=%q", rec.Code, rec.Body)
	}

	// unknown API path is a 404, not the SPA
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown api => %d, want 404", rec.Code)
	}
}

func jsonReq(method, target, body string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func u32(v uint32) *uint32 { return &v }

func TestSetPowerEndpoint(t *testing.T) {
	authMgr, err := auth.NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	snap := snapshot.New(nil)
	// QS1A (model 0x18), 2 panels, online with live output.
	snap.ApplyInfo(&wire.InverterInfo{PeerUid: "aabbccddeeff", ModelCode: u32(0x18)})
	snap.ApplyTelemetry(&wire.Telemetry{
		PeerUid: "aabbccddeeff", TsMs: time.Now().UnixMilli(), ActivePowerW: 300,
		Panels: []*wire.Panel{{Index: 0}, {Index: 1}},
	})

	type sentFrame struct {
		uid   string
		frame []byte
	}
	var sent []sentFrame
	srv := New(Config{
		StateDir: t.TempDir(),
		Assets:   fstest.MapFS{"index.html": {Data: []byte("x")}},
		Snap:     snap,
		Auth:     authMgr,
		Conn:     func() bool { return true },
		SendFrame: func(_ context.Context, uid string, frame []byte) error {
			sent = append(sent, sentFrame{uid, frame})
			return nil
		},
	})
	h := srv.Handler()
	cookies, _ := setupServer(t, h, "password123")
	authed := func(body string) *http.Request {
		r := jsonReq("POST", "/api/power", body)
		for _, c := range cookies {
			r.AddCookie(c)
		}
		return r
	}
	decode := func(rec *httptest.ResponseRecorder) []powerResultDTO {
		var resp powerRespDTO
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v (%s)", err, rec.Body)
		}
		return resp.Results
	}

	// Unauthenticated -> 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/power", `{"uid":"aabbccddeeff","watts":300}`))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated => %d, want 401", rec.Code)
	}

	// Per-inverter cap: 320 W of 1600 nameplate = 20% → per-panel 100 (in
	// [20,500]) → applied 320, one frame.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{"uid":"aabbccddeeff","watts":320}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("per-inverter => %d (%s)", rec.Code, rec.Body)
	}
	res := decode(rec)
	if len(res) != 1 || !res[0].OK || res[0].AppliedWatts != 320 {
		t.Fatalf("per-inverter results = %+v", res)
	}
	if len(sent) != 1 || sent[0].uid != "aabbccddeeff" || len(sent[0].frame) == 0 {
		t.Fatalf("frame not sent: %+v", sent)
	}

	// Full nameplate reaches per-panel 500 (uncapped) → applied == nameplate.
	sent = nil
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{"uid":"aabbccddeeff","watts":1600}`))
	res = decode(rec)
	if len(res) != 1 || res[0].AppliedWatts != 1600 {
		t.Errorf("nameplate cap applied = %+v, want 1600", res)
	}

	// Tiny watts clamps UP to the per-panel floor: 20/500 × 1600 = 64.
	sent = nil
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{"uid":"aabbccddeeff","watts":1}`))
	res = decode(rec)
	if len(res) != 1 || res[0].AppliedWatts != 64 {
		t.Errorf("floor clamp applied = %+v, want 64", res)
	}

	// Unknown uid -> 400.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{"uid":"ffffffffffff","watts":300}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown uid => %d, want 400", rec.Code)
	}

	// Array cap distributes to online inverters (one unicast here).
	sent = nil
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{"array":true,"watts":400}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("array => %d (%s)", rec.Code, rec.Body)
	}
	if len(sent) != 1 {
		t.Errorf("array sent %d frames, want 1", len(sent))
	}

	// Non-positive watts -> 400.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{"uid":"aabbccddeeff","watts":0}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("zero watts => %d, want 400", rec.Code)
	}
}

func TestSetPowerUnavailable(t *testing.T) {
	_, h := newTestServer(t) // no SendFrame configured
	cookies, _ := setupServer(t, h, "password123")
	r := jsonReq("POST", "/api/power", `{"uid":"u","watts":100}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("no SendFrame => %d, want 503", rec.Code)
	}
}

// setupServer runs first-run setup and returns the session cookies plus the
// one-time recovery code from the response body.
func setupServer(t *testing.T, h http.Handler, pw string) ([]*http.Cookie, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/setup", `{"password":"`+pw+`"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("setup => %d (%s)", rec.Code, rec.Body)
	}
	var body struct {
		RecoveryCode string `json:"recovery_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("setup body: %v", err)
	}
	if body.RecoveryCode == "" {
		t.Fatal("setup did not return a recovery code")
	}
	return rec.Result().Cookies(), body.RecoveryCode
}

func TestChangePasswordEndpoint(t *testing.T) {
	_, h := newTestServer(t)
	cookies, _ := setupServer(t, h, "password123")

	authed := func(body string) *http.Request {
		r := jsonReq("POST", "/api/auth/change-password", body)
		for _, c := range cookies {
			r.AddCookie(c)
		}
		return r
	}

	// No session -> 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/change-password",
		`{"current_password":"password123","new_password":"newpassword"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated change => %d, want 401", rec.Code)
	}

	// Wrong current password -> 401.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{"current_password":"nope","new_password":"newpassword"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong current => %d, want 401", rec.Code)
	}

	// Correct -> 200, and the new password logs in.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{"current_password":"password123","new_password":"newpassword"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("change => %d (%s)", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/login", `{"password":"newpassword"}`))
	if rec.Code != http.StatusOK {
		t.Errorf("login with new password => %d", rec.Code)
	}
}

func TestAuthVerifyEndpoint(t *testing.T) {
	_, h := newTestServer(t)
	cookies, _ := setupServer(t, h, "password123")

	authed := func(body string) *http.Request {
		r := jsonReq("POST", "/api/auth/verify", body)
		for _, c := range cookies {
			r.AddCookie(c)
		}
		return r
	}

	// No session -> 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/verify", `{"password":"password123"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated verify => %d, want 401", rec.Code)
	}

	// Wrong password -> 401.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{"password":"nope"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password => %d, want 401", rec.Code)
	}

	// Correct password -> 200.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{"password":"password123"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("correct password => %d (%s)", rec.Code, rec.Body)
	}

	// Missing password -> 400.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authed(`{}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing password => %d, want 400", rec.Code)
	}
}

func TestRecoverEndpoint(t *testing.T) {
	_, h := newTestServer(t)
	_, code := setupServer(t, h, "password123")

	// Wrong code -> 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/recover",
		`{"recovery_code":"WRONG-CODE-HERE","password":"newpassword"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong recovery code => %d, want 401", rec.Code)
	}

	// Correct code -> 200, rotated code in body, session cookie set.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/recover",
		`{"recovery_code":"`+code+`","password":"newpassword"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("recover => %d (%s)", rec.Code, rec.Body)
	}
	var body struct {
		RecoveryCode string `json:"recovery_code"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.RecoveryCode == "" || body.RecoveryCode == code {
		t.Errorf("recover should return a rotated code: old=%q new=%q", code, body.RecoveryCode)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Error("recover did not set a session cookie")
	}
	// New password works.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/login", `{"password":"newpassword"}`))
	if rec.Code != http.StatusOK {
		t.Errorf("login with recovered password => %d", rec.Code)
	}
}

func TestRegenerateRecoveryEndpoint(t *testing.T) {
	_, h := newTestServer(t)
	cookies, code := setupServer(t, h, "password123")

	// Unauthenticated -> 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/recovery", `{}`))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated regenerate => %d, want 401", rec.Code)
	}

	// Authed -> 200 with a fresh code, and the old code stops working.
	rec = httptest.NewRecorder()
	r := jsonReq("POST", "/api/auth/recovery", `{}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("regenerate => %d (%s)", rec.Code, rec.Body)
	}
	var body struct {
		RecoveryCode string `json:"recovery_code"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.RecoveryCode == "" || body.RecoveryCode == code {
		t.Errorf("regenerate should return a new code: old=%q new=%q", code, body.RecoveryCode)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/recover",
		`{"recovery_code":"`+code+`","password":"newpassword"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("old recovery code after regenerate => %d, want 401", rec.Code)
	}
}

// newSysServer builds a server whose SysStatus fetcher returns sys (or
// err) and counts invocations. Returns the authed cookie too.
func newSysServer(t *testing.T, sys *wire.SystemStatusResponse, err error) (http.Handler, *int, []*http.Cookie) {
	t.Helper()
	authMgr, e := auth.NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	srv := New(Config{
		StateDir: t.TempDir(),
		Assets:   fstest.MapFS{"index.html": {Data: []byte("x")}},
		Snap:     snapshot.New(nil),
		Auth:     authMgr,
		Conn:     func() bool { return true },
		SysStatus: func(context.Context) (*wire.SystemStatusResponse, error) {
			calls++
			return sys, err
		},
	})
	h := srv.Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/setup", `{"password":"password123"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("setup => %d", rec.Code)
	}
	return h, &calls, rec.Result().Cookies()
}

func getSystem(t *testing.T, h http.Handler, cookies []*http.Cookie) (int, systemDTO) {
	t.Helper()
	r := httptest.NewRequest("GET", "/api/system", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	var out systemDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestSystemEndpointMergesStatus(t *testing.T) {
	sys := &wire.SystemStatusResponse{
		Ok:  true,
		Ecu: &wire.EcuIdentity{EcuId: "my-roof-ecu", Hostname: "ecu"},
		Peers: []*wire.PeerStatus{
			{Backend: "ecu-zb", Role: "PUBLISHER"},
			{Backend: "ecu-web", Role: "SUBSCRIBER", Controller: true},
		},
	}
	h, _, cookies := newSysServer(t, sys, nil)
	code, out := getSystem(t, h, cookies)
	if code != http.StatusOK {
		t.Fatalf("status => %d", code)
	}
	if !out.InvdriverConnected || out.ECU == nil || out.ECU.EcuID != "my-roof-ecu" {
		t.Errorf("merge wrong: %+v", out)
	}
	if len(out.Peers) != 2 || !out.Peers[1].Controller {
		t.Errorf("peers wrong: %+v", out.Peers)
	}
}

func TestSystemEndpointDegradesOnError(t *testing.T) {
	h, _, cookies := newSysServer(t, nil, errors.New("inv-driver down"))
	code, out := getSystem(t, h, cookies)
	if code != http.StatusOK {
		t.Fatalf("should degrade to 200, got %d", code)
	}
	if out.StatusError == "" {
		t.Errorf("expected status_error to be set")
	}
	if !out.InvdriverConnected {
		t.Errorf("local fields should still report")
	}
}

func TestEventsEndpoint(t *testing.T) {
	authMgr, _ := auth.NewManager(filepath.Join(t.TempDir(), "auth.json"))
	var gotReq *wire.EventsRequest
	srv := New(Config{
		StateDir: t.TempDir(),
		Assets:   fstest.MapFS{"index.html": {Data: []byte("x")}},
		Snap:     snapshot.New(nil),
		Auth:     authMgr,
		EventsFn: func(_ context.Context, req *wire.EventsRequest) (*wire.EventsResponse, error) {
			gotReq = req
			return &wire.EventsResponse{Ok: true, Events: []*wire.Event{
				{Id: 1, TsMs: 5, Kind: "paired", Severity: "info", InverterUid: "u"},
			}}, nil
		},
	})
	h := srv.Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/setup", `{"password":"password123"}`))
	cookies := rec.Result().Cookies()

	r := httptest.NewRequest("GET", "/api/events?kind=paired&limit=50&since_ms=3", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("events => %d", rec.Code)
	}
	// Query params were parsed into the request.
	if gotReq.GetKind() != "paired" || gotReq.GetLimit() != 50 || gotReq.GetSinceMs() != 3 {
		t.Errorf("request params not parsed: %+v", gotReq)
	}
	var out eventsDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Events) != 1 || out.Events[0].Kind != "paired" {
		t.Errorf("events DTO wrong: %+v", out)
	}
}

func TestEventsEndpointDegradesOnError(t *testing.T) {
	authMgr, _ := auth.NewManager(filepath.Join(t.TempDir(), "auth.json"))
	srv := New(Config{
		StateDir: t.TempDir(),
		Assets:   fstest.MapFS{"index.html": {Data: []byte("x")}},
		Snap:     snapshot.New(nil),
		Auth:     authMgr,
		EventsFn: func(context.Context, *wire.EventsRequest) (*wire.EventsResponse, error) {
			return nil, errors.New("inv-driver down")
		},
	})
	h := srv.Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/setup", `{"password":"password123"}`))
	cookies := rec.Result().Cookies()
	r := httptest.NewRequest("GET", "/api/events", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("should degrade to 200, got %d", rec.Code)
	}
	var out eventsDTO
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Error == "" || len(out.Events) != 0 {
		t.Errorf("expected empty events + error, got %+v", out)
	}
}

// newSettingsServer builds a server with the given settings fetchers and
// returns the handler plus an authed cookie set.
func newSettingsServer(t *testing.T, cfg Config) (http.Handler, []*http.Cookie) {
	t.Helper()
	authMgr, err := auth.NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.StateDir = t.TempDir()
	cfg.Assets = fstest.MapFS{"index.html": {Data: []byte("x")}}
	if cfg.Snap == nil {
		cfg.Snap = snapshot.New(nil)
	}
	cfg.Auth = authMgr
	srv := New(cfg)
	h := srv.Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/setup", `{"password":"password123"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("setup => %d", rec.Code)
	}
	return h, rec.Result().Cookies()
}

func TestGetSettingsEndpoint(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		SettingsGet: func(context.Context) (*wire.SettingsResponse, error) {
			return &wire.SettingsResponse{Ok: true,
				Settings: &wire.Settings{
					EcuId: "216200001234", Mac: "80971b000000", PanOverride: "0DCE", ZigbeeType: "apsystems", Channel: 20,
				},
				Effective: &wire.EffectiveSettings{Mac: "80:97:1b:00:00:00", Pan: "0DCE", ZigbeeType: "apsystems", Channel: 20},
			}, nil
		},
	})
	r := httptest.NewRequest("GET", "/api/settings", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings => %d", rec.Code)
	}
	var out settingsDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.EcuID != "216200001234" || out.PanOverride != "0DCE" || out.ZigbeeType != "apsystems" || out.Channel != 20 {
		t.Errorf("settings DTO wrong: %+v", out)
	}
	if out.Effective == nil || out.Effective.Channel != 20 {
		t.Errorf("effective channel not mapped: %+v", out.Effective)
	}
}

func TestSettingsInverterNamesRoundTrip(t *testing.T) {
	var got *wire.Settings
	h, cookies := newSettingsServer(t, Config{
		SettingsGet: func(context.Context) (*wire.SettingsResponse, error) {
			return &wire.SettingsResponse{Ok: true, Settings: &wire.Settings{
				EcuId: "e", InverterNames: map[string]string{"u1": "Roof South"},
			}}, nil
		},
		SettingsSet: func(_ context.Context, s *wire.Settings) (*wire.SettingsResponse, error) {
			got = s
			return &wire.SettingsResponse{Ok: true, Settings: s}, nil
		},
	})
	withCookies := func(r *http.Request) *http.Request {
		for _, c := range cookies {
			r.AddCookie(c)
		}
		return r
	}
	// GET carries the names.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(httptest.NewRequest("GET", "/api/settings", nil)))
	var out settingsDTO
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.InverterNames["u1"] != "Roof South" {
		t.Errorf("GET dropped inverter_names: %+v", out)
	}
	// PUT passes the names through to inv-driver.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("PUT", "/api/settings", `{"ecu_id":"e","inverter_names":{"u2":"Garage"}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT => %d", rec.Code)
	}
	if got.GetInverterNames()["u2"] != "Garage" {
		t.Errorf("PUT dropped inverter_names: %+v", got.GetInverterNames())
	}
}

func TestGetSettingsDegradesOnError(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		SettingsGet: func(context.Context) (*wire.SettingsResponse, error) {
			return nil, errors.New("inv-driver down")
		},
	})
	r := httptest.NewRequest("GET", "/api/settings", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("should degrade to 200, got %d", rec.Code)
	}
	var out settingsDTO
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Error == "" {
		t.Errorf("expected error field, got %+v", out)
	}
}

func TestSetSettingsEndpoint(t *testing.T) {
	var got *wire.Settings
	h, cookies := newSettingsServer(t, Config{
		SettingsSet: func(_ context.Context, s *wire.Settings) (*wire.SettingsResponse, error) {
			got = s
			return &wire.SettingsResponse{Ok: true, Settings: s}, nil
		},
	})
	// This change touches mac + pan_override, both step-up-gated, so the
	// caller must POST /api/auth/verify first. (No SettingsGet is wired,
	// so the server fail-closes any change to a sensitive field.)
	rec := httptest.NewRecorder()
	rv := jsonReq("POST", "/api/auth/verify", `{"password":"password123"}`)
	for _, c := range cookies {
		rv.AddCookie(c)
	}
	h.ServeHTTP(rec, rv)
	if rec.Code != http.StatusOK {
		t.Fatalf("step-up verify => %d", rec.Code)
	}
	r := jsonReq("PUT", "/api/settings",
		`{"ecu_id":"new-id","mac":"aabbccddeeff","pan_override":"1A2B","zigbee_type":"general","channel":21}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("set settings => %d (%s)", rec.Code, rec.Body)
	}
	if got.GetEcuId() != "new-id" || got.GetZigbeeType() != "general" || got.GetPanOverride() != "1A2B" || got.GetChannel() != 21 {
		t.Errorf("decoded settings wrong: %+v", got)
	}
	var out settingsDTO
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.EcuID != "new-id" {
		t.Errorf("response DTO wrong: %+v", out)
	}
}

func TestSetSettingsRejected(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		SettingsSet: func(context.Context, *wire.Settings) (*wire.SettingsResponse, error) {
			return &wire.SettingsResponse{Ok: false, Error: "invalid pan override"}, errors.New("inv-driver: invalid pan override")
		},
	})
	// pan_override change is step-up-gated; verify first so the write
	// reaches SettingsSet and gets rejected for the right reason.
	rec := httptest.NewRecorder()
	rv := jsonReq("POST", "/api/auth/verify", `{"password":"password123"}`)
	for _, c := range cookies {
		rv.AddCookie(c)
	}
	h.ServeHTTP(rec, rv)
	if rec.Code != http.StatusOK {
		t.Fatalf("step-up verify => %d", rec.Code)
	}
	r := jsonReq("PUT", "/api/settings", `{"pan_override":"ZZZZ"}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("rejected write => %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid pan override") {
		t.Errorf("expected error text in body, got %q", rec.Body.String())
	}
}

// TestSetSettingsStepUpRequired verifies the server-side step-up gate on
// PUT /api/settings: a change to a sensitive field (mac or pan_override)
// is rejected with 403 unless POST /api/auth/verify just succeeded on the
// same session. Non-sensitive changes (e.g. ecu_id only) pass without.
func TestSetSettingsStepUpRequired(t *testing.T) {
	current := &wire.SettingsResponse{Ok: true, Settings: &wire.Settings{
		EcuId: "current-id", Mac: "80971b000000", PanOverride: "0DCE", ZigbeeType: "apsystems",
	}}
	var lastSet *wire.Settings
	h, cookies := newSettingsServer(t, Config{
		SettingsGet: func(context.Context) (*wire.SettingsResponse, error) {
			return current, nil
		},
		SettingsSet: func(_ context.Context, s *wire.Settings) (*wire.SettingsResponse, error) {
			lastSet = s
			return &wire.SettingsResponse{Ok: true, Settings: s}, nil
		},
	})
	withCookies := func(r *http.Request) *http.Request {
		for _, c := range cookies {
			r.AddCookie(c)
		}
		return r
	}
	put := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookies(jsonReq("PUT", "/api/settings", body)))
		return rec
	}
	verify := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/auth/verify", body)))
		return rec
	}

	// Sensitive change (mac) WITHOUT prior Verify -> 403.
	lastSet = nil
	rec := put(`{"ecu_id":"current-id","mac":"aabbccddeeff","pan_override":"0DCE","zigbee_type":"apsystems"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("mac change without step-up => %d, want 403; body=%s", rec.Code, rec.Body)
	}
	if lastSet != nil {
		t.Error("SettingsSet should not have been called when step-up was missing")
	}

	// WRONG password Verify -> 401, NO step-up marked -> subsequent PUT
	// still 403.
	rec = verify(`{"password":"nope"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong-pw verify => %d, want 401", rec.Code)
	}
	lastSet = nil
	rec = put(`{"ecu_id":"current-id","mac":"aabbccddeeff","pan_override":"0DCE","zigbee_type":"apsystems"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("mac change after wrong-pw verify => %d, want 403", rec.Code)
	}
	if lastSet != nil {
		t.Error("SettingsSet should still not have been called")
	}

	// CORRECT password Verify, then PUT with mac change -> 200.
	rec = verify(`{"password":"password123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("correct-pw verify => %d (%s)", rec.Code, rec.Body)
	}
	lastSet = nil
	rec = put(`{"ecu_id":"current-id","mac":"aabbccddeeff","pan_override":"0DCE","zigbee_type":"apsystems"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("mac change after verify => %d (%s)", rec.Code, rec.Body)
	}
	if lastSet == nil || lastSet.GetMac() != "aabbccddeeff" {
		t.Errorf("SettingsSet did not receive the new mac: %+v", lastSet)
	}

	// Non-sensitive change (ecu_id only) -> 200 even without a fresh
	// step-up. Build a fresh server so the previous step-up doesn't mask
	// the test.
	current2 := &wire.SettingsResponse{Ok: true, Settings: &wire.Settings{
		EcuId: "old", Mac: "80971b000000", PanOverride: "0DCE", ZigbeeType: "apsystems",
	}}
	h2, ck2 := newSettingsServer(t, Config{
		SettingsGet: func(context.Context) (*wire.SettingsResponse, error) { return current2, nil },
		SettingsSet: func(_ context.Context, s *wire.Settings) (*wire.SettingsResponse, error) {
			return &wire.SettingsResponse{Ok: true, Settings: s}, nil
		},
	})
	r2 := jsonReq("PUT", "/api/settings",
		`{"ecu_id":"new-name","mac":"80971b000000","pan_override":"0DCE","zigbee_type":"apsystems"}`)
	for _, c := range ck2 {
		r2.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h2.ServeHTTP(rec, r2)
	if rec.Code != http.StatusOK {
		t.Errorf("ecu_id-only change without step-up => %d, want 200", rec.Code)
	}
}

// TestSetSettingsStepUpConsumedOnSensitiveWrite verifies that a successful
// sensitive PUT consumes the step-up flag, so a second sensitive PUT inside
// the TTL window is refused — single-use, not 60-second-window-reusable.
// A non-sensitive (ecu_id only) write must NOT consume the flag.
func TestSetSettingsStepUpConsumedOnSensitiveWrite(t *testing.T) {
	current := &wire.SettingsResponse{Ok: true, Settings: &wire.Settings{
		EcuId: "site-a", Mac: "80971b000000", PanOverride: "0DCE", ZigbeeType: "apsystems",
	}}
	h, cookies := newSettingsServer(t, Config{
		SettingsGet: func(context.Context) (*wire.SettingsResponse, error) { return current, nil },
		SettingsSet: func(_ context.Context, s *wire.Settings) (*wire.SettingsResponse, error) {
			return &wire.SettingsResponse{Ok: true, Settings: s}, nil
		},
	})
	withCookies := func(r *http.Request) *http.Request {
		for _, c := range cookies {
			r.AddCookie(c)
		}
		return r
	}
	put := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookies(jsonReq("PUT", "/api/settings", body)))
		return rec
	}
	verify := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/auth/verify", body)))
		return rec
	}

	// Verify -> mac change PUT (succeeds, consumes step-up).
	if rec := verify(`{"password":"password123"}`); rec.Code != http.StatusOK {
		t.Fatalf("verify: %d (%s)", rec.Code, rec.Body)
	}
	if rec := put(`{"ecu_id":"site-a","mac":"aabbccddeeff","pan_override":"0DCE","zigbee_type":"apsystems"}`); rec.Code != http.StatusOK {
		t.Fatalf("first sensitive PUT: %d (%s)", rec.Code, rec.Body)
	}
	// Second sensitive PUT inside TTL must now be refused: step-up was consumed.
	rec := put(`{"ecu_id":"site-a","mac":"aabbccddee00","pan_override":"0DCE","zigbee_type":"apsystems"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("second sensitive PUT inside TTL => %d, want 403 (step-up should be consumed)", rec.Code)
	}

	// Fresh verify, then a NON-sensitive write (ecu_id only). The non-sensitive
	// write must NOT consume step-up; a follow-up sensitive PUT inside TTL
	// must still succeed.
	if rec := verify(`{"password":"password123"}`); rec.Code != http.StatusOK {
		t.Fatalf("re-verify: %d (%s)", rec.Code, rec.Body)
	}
	if rec := put(`{"ecu_id":"renamed","mac":"80971b000000","pan_override":"0DCE","zigbee_type":"apsystems"}`); rec.Code != http.StatusOK {
		t.Fatalf("non-sensitive PUT: %d (%s)", rec.Code, rec.Body)
	}
	if rec := put(`{"ecu_id":"renamed","mac":"aabbccddeeff","pan_override":"0DCE","zigbee_type":"apsystems"}`); rec.Code != http.StatusOK {
		t.Errorf("sensitive PUT after a non-sensitive PUT => %d, want 200 (non-sensitive must not consume step-up)", rec.Code)
	}
}

// TestSetSettingsStepUpExpires drives an injected clock past stepUpTTL and
// confirms the next sensitive PUT is refused.
func TestSetSettingsStepUpExpires(t *testing.T) {
	authMgr, err := auth.NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Inject a controllable clock so step-up TTL can be exceeded without
	// real-time waits.
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := t0
	authMgr.SetClock(func() time.Time { return clock })

	current := &wire.SettingsResponse{Ok: true, Settings: &wire.Settings{
		EcuId: "x", Mac: "80971b000000", PanOverride: "0DCE", ZigbeeType: "apsystems",
	}}
	srv := New(Config{
		StateDir: t.TempDir(),
		Assets:   fstest.MapFS{"index.html": {Data: []byte("x")}},
		Snap:     snapshot.New(nil),
		Auth:     authMgr,
		SettingsGet: func(context.Context) (*wire.SettingsResponse, error) { return current, nil },
		SettingsSet: func(_ context.Context, s *wire.Settings) (*wire.SettingsResponse, error) {
			return &wire.SettingsResponse{Ok: true, Settings: s}, nil
		},
	})
	h := srv.Handler()

	// Setup -> cookie.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, jsonReq("POST", "/api/auth/setup", `{"password":"password123"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("setup => %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	withCookies := func(req *http.Request) *http.Request {
		for _, c := range cookies {
			req.AddCookie(c)
		}
		return req
	}

	// Verify, then PUT a mac change -> 200.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/auth/verify", `{"password":"password123"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("verify => %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("PUT", "/api/settings",
		`{"ecu_id":"x","mac":"aabbccddeeff","pan_override":"0DCE","zigbee_type":"apsystems"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("mac change immediately after verify => %d", rec.Code)
	}

	// Advance the clock past stepUpTTL: the next sensitive PUT is refused.
	clock = t0.Add(2 * time.Minute)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("PUT", "/api/settings",
		`{"ecu_id":"x","mac":"112233445566","pan_override":"0DCE","zigbee_type":"apsystems"}`)))
	if rec.Code != http.StatusForbidden {
		t.Errorf("mac change after TTL => %d, want 403", rec.Code)
	}
}

func TestSetSettingsUnavailable(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{})
	r := jsonReq("PUT", "/api/settings", `{"ecu_id":"x"}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil SettingsSet => %d, want 503", rec.Code)
	}
}

func TestSystemStatusCached(t *testing.T) {
	sys := &wire.SystemStatusResponse{Ok: true, Ecu: &wire.EcuIdentity{EcuId: "x"}}
	h, calls, cookies := newSysServer(t, sys, nil)
	getSystem(t, h, cookies)
	getSystem(t, h, cookies)
	if *calls != 1 {
		t.Errorf("fetcher called %d times within TTL, want 1", *calls)
	}
}

// TestHTTP2AndSSE exercises the full TLS stack: HTTP/2 negotiation, a
// Secure-cookie login over real https, and the initial fleet frame on
// the SSE stream.
func TestHTTP2AndSSE(t *testing.T) {
	_, h := newTestServer(t)
	ts := httptest.NewUnstartedServer(h)
	ts.EnableHTTP2 = true
	ts.StartTLS()
	defer ts.Close()

	client := ts.Client()
	jar, _ := cookiejar.New(nil)
	client.Jar = jar

	// HTTP/2 negotiated?
	resp, err := client.Get(ts.URL + "/api/auth/status")
	if err != nil {
		t.Fatal(err)
	}
	if resp.ProtoMajor != 2 {
		t.Errorf("ProtoMajor = %d, want 2 (HTTP/2)", resp.ProtoMajor)
	}
	resp.Body.Close()

	// Login via setup (Secure cookie stored by the jar over https).
	resp, err = client.Post(ts.URL+"/api/auth/setup", "application/json",
		strings.NewReader(`{"password":"password123"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup => %d", resp.StatusCode)
	}

	// SSE: read the initial fleet frame.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/stream", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("SSE Content-Type = %q", ct)
	}

	sc := bufio.NewScanner(resp.Body)
	var sawEvent, sawData bool
	for sc.Scan() {
		line := sc.Text()
		if line == "event: fleet" {
			sawEvent = true
		}
		if strings.HasPrefix(line, "data: ") {
			sawData = true
			if !strings.Contains(line, `"inverter_count":1`) {
				t.Errorf("fleet frame missing inverter data: %s", line)
			}
			break
		}
	}
	if !sawEvent || !sawData {
		t.Errorf("did not receive an SSE fleet frame (event=%v data=%v)", sawEvent, sawData)
	}
}

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

	"github.com/bolke/inv-driver/cmd/ecu-web/internal/auth"
	"github.com/bolke/inv-driver/cmd/ecu-web/internal/snapshot"
	"github.com/bolke/inv-driver/wire"
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
			return &wire.SettingsResponse{Ok: true, Settings: &wire.Settings{
				EcuId: "216200001234", Mac: "80971b000000", PanOverride: "0DCE", ZigbeeType: "apsystems",
			}}, nil
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
	if out.EcuID != "216200001234" || out.PanOverride != "0DCE" || out.ZigbeeType != "apsystems" {
		t.Errorf("settings DTO wrong: %+v", out)
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
	r := jsonReq("PUT", "/api/settings",
		`{"ecu_id":"new-id","mac":"aabbccddeeff","pan_override":"1A2B","zigbee_type":"general"}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("set settings => %d (%s)", rec.Code, rec.Body)
	}
	if got.GetEcuId() != "new-id" || got.GetZigbeeType() != "general" || got.GetPanOverride() != "1A2B" {
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
	r := jsonReq("PUT", "/api/settings", `{"pan_override":"ZZZZ"}`)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("rejected write => %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid pan override") {
		t.Errorf("expected error text in body, got %q", rec.Body.String())
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

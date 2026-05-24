// Package httpserver is the ecu-web front door: an HTTP/2 server (TLS,
// self-signed) that serves the embedded Lit SPA, streams the fleet over
// SSE, and exposes a small JSON API. It holds no inverter logic — it
// projects the snapshot (fed by the inv-driver UDS stream) and gates
// every data path behind the auth.Manager.
package httpserver

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bolke/inv-driver/cmd/ecu-web/internal/auth"
	"github.com/bolke/inv-driver/cmd/ecu-web/internal/powerhistory"
	"github.com/bolke/inv-driver/cmd/ecu-web/internal/snapshot"
	"github.com/bolke/inv-driver/wire"
)

// Config parameterises the server.
type Config struct {
	Listen   string // e.g. ":8443"
	StateDir string // TLS material lives here
	Assets   fs.FS  // embedded SPA (already Sub'd to the dist root)
	Snap     *snapshot.Snapshot
	Auth     *auth.Manager
	Conn     func() bool   // reports inv-driver subscriber connectivity
	PushRate time.Duration // min interval between SSE fleet frames
	// SysStatus fetches ECU identity + connected peers from inv-driver.
	// Nil disables the richer /api/system fields (connectivity + SSE
	// count are still reported).
	SysStatus func(context.Context) (*wire.SystemStatusResponse, error)
	// EventsFn queries the inv-driver events log. Nil makes /api/events
	// return a 503.
	EventsFn func(context.Context, *wire.EventsRequest) (*wire.EventsResponse, error)
	// SettingsGet reads the current ECU settings from inv-driver. Nil
	// degrades GET /api/settings to an error field.
	SettingsGet func(context.Context) (*wire.SettingsResponse, error)
	// SettingsSet writes ECU settings to inv-driver. Nil makes PUT
	// /api/settings return a 503.
	SettingsSet func(context.Context, *wire.Settings) (*wire.SettingsResponse, error)
	// HistoryInterval is the power-history sample period (default 60s);
	// HistoryWindow is how far back the chart keeps (default 48h).
	HistoryInterval time.Duration
	HistoryWindow   time.Duration
}

// Server is the assembled HTTP service.
type Server struct {
	cfg  Config
	hub  *Hub
	hist *powerhistory.Ring

	// sysCache coalesces /api/system fetches so many browser polls don't
	// each open a controller connection to inv-driver.
	sysMu   sync.Mutex
	sysAt   time.Time
	sysResp *wire.SystemStatusResponse
}

const sysCacheTTL = 3 * time.Second

// New assembles the server, its SSE hub, and the power-history ring. The
// snapshot's change callback should be wired to s.MarkDirty by the caller.
func New(cfg Config) *Server {
	if cfg.PushRate <= 0 {
		cfg.PushRate = time.Second
	}
	if cfg.HistoryInterval <= 0 {
		cfg.HistoryInterval = time.Minute
	}
	if cfg.HistoryWindow <= 0 {
		cfg.HistoryWindow = 48 * time.Hour
	}
	s := &Server{cfg: cfg}
	s.hub = newHub(cfg.PushRate, s.renderFleet)
	capacity := int(cfg.HistoryWindow/cfg.HistoryInterval) + 1
	path := ""
	if cfg.StateDir != "" {
		path = filepath.Join(cfg.StateDir, "power-history.json")
	}
	s.hist = powerhistory.New(capacity, path)
	return s
}

// MarkDirty signals the SSE hub that the snapshot changed. Wire this to
// snapshot.New's onChange.
func (s *Server) MarkDirty() { s.hub.MarkDirty() }

func (s *Server) renderFleet() []byte {
	b, err := json.Marshal(s.cfg.Snap.Fleet(time.Now()))
	if err != nil {
		return []byte("{}")
	}
	return b
}

// Handler builds the route tree. Static assets and the auth status/login
// endpoints are public; everything that exposes fleet data sits behind
// auth.Require.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public auth endpoints.
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/setup", s.handleAuthSetup)
	mux.HandleFunc("POST /api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleAuthLogout)

	// Protected data endpoints.
	mux.Handle("GET /api/fleet", s.cfg.Auth.Require(http.HandlerFunc(s.handleFleet)))
	mux.Handle("GET /api/system", s.cfg.Auth.Require(http.HandlerFunc(s.handleSystem)))
	mux.Handle("GET /api/events", s.cfg.Auth.Require(http.HandlerFunc(s.handleEvents)))
	mux.Handle("GET /api/settings", s.cfg.Auth.Require(http.HandlerFunc(s.handleGetSettings)))
	mux.Handle("PUT /api/settings", s.cfg.Auth.Require(http.HandlerFunc(s.handleSetSettings)))
	mux.Handle("GET /api/history", s.cfg.Auth.Require(http.HandlerFunc(s.handleHistory)))
	mux.Handle("GET /api/stream", s.cfg.Auth.Require(s.hub))

	// Static SPA (public so the login page can load).
	mux.Handle("/", s.staticHandler())
	return logRequests(mux)
}

// Run starts the HTTP/2 TLS listener and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	cert, err := loadOrCreateCert(s.cfg.StateDir)
	if err != nil {
		return err
	}
	go s.hub.run(ctx)
	go s.sampleHistory(ctx)

	srv := &http.Server{
		Addr:    s.cfg.Listen,
		Handler: s.Handler(),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{"h2", "http/1.1"},
		},
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		log.Printf("httpserver: listening on %s (HTTP/2, TLS)", s.cfg.Listen)
		// Certs supplied via TLSConfig, so the file args are empty.
		errc <- srv.ListenAndServeTLS("", "")
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// --- handlers ---

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"configured":    s.cfg.Auth.Configured(),
		"authenticated": s.cfg.Auth.Authenticated(r),
	})
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Auth.Configured() {
		http.Error(w, "already configured", http.StatusConflict)
		return
	}
	pw, ok := readPassword(w, r)
	if !ok {
		return
	}
	if err := s.cfg.Auth.SetPassword(pw); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tok, err := s.cfg.Auth.NewSession()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	auth.SetCookie(w, tok, true)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	pw, ok := readPassword(w, r)
	if !ok {
		return
	}
	tok, err := s.cfg.Auth.Login(pw)
	if err != nil {
		if errors.Is(err, auth.ErrNotConfigured) {
			http.Error(w, "not configured", http.StatusConflict)
			return
		}
		http.Error(w, "invalid password", http.StatusUnauthorized)
		return
	}
	auth.SetCookie(w, tok, true)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	s.cfg.Auth.Logout(r)
	auth.ClearCookie(w, true)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFleet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Snap.Fleet(time.Now()))
}

// handleHistory returns the downsampled fleet-power history (oldest→newest).
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.hist.Points())
}

// sampleHistory records one fleet-power sample per HistoryInterval and
// persists the ring periodically (and on shutdown).
func (s *Server) sampleHistory(ctx context.Context) {
	t := time.NewTicker(s.cfg.HistoryInterval)
	defer t.Stop()
	sinceSave := 0
	for {
		select {
		case <-ctx.Done():
			_ = s.hist.Save()
			return
		case <-t.C:
			now := time.Now()
			s.hist.Add(now.UnixMilli(), s.cfg.Snap.Fleet(now).ActivePowerW)
			if sinceSave++; sinceSave >= 5 {
				sinceSave = 0
				_ = s.hist.Save()
			}
		}
	}
}

// ecuDTO / peerDTO / systemDTO shape the /api/system response.
type ecuDTO struct {
	EcuID    string `json:"ecu_id"`
	Hostname string `json:"hostname"`
}

type peerDTO struct {
	Backend       string `json:"backend"`
	Version       string `json:"version"`
	Hostname      string `json:"hostname"`
	Role          string `json:"role"`
	ConnectedAtMs int64  `json:"connected_at_ms"`
	PeerUID       int32  `json:"peer_uid"`
	Controller    bool   `json:"controller"`
}

type systemDTO struct {
	InvdriverConnected bool      `json:"invdriver_connected"`
	SSEClients         int       `json:"sse_clients"`
	ECU                *ecuDTO   `json:"ecu,omitempty"`
	Peers              []peerDTO `json:"peers"`
	StatusError        string    `json:"status_error,omitempty"`
}

// handleSystem reports ECU-side status: inv-driver connectivity, the SSE
// client count, and — via the inv-driver SystemStatus API — ECU identity
// and the live peer set. A SystemStatus fetch failure degrades to the
// local-only fields plus status_error, never a 5xx.
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	out := systemDTO{
		SSEClients: s.hub.clientCount(),
		Peers:      []peerDTO{},
	}
	if s.cfg.Conn != nil {
		out.InvdriverConnected = s.cfg.Conn()
	}

	resp, err := s.systemStatus(r.Context())
	if err != nil {
		out.StatusError = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	if ecu := resp.GetEcu(); ecu != nil {
		out.ECU = &ecuDTO{
			EcuID:    ecu.GetEcuId(),
			Hostname: ecu.GetHostname(),
		}
	}
	for _, p := range resp.GetPeers() {
		out.Peers = append(out.Peers, peerDTO{
			Backend:       p.GetBackend(),
			Version:       p.GetVersion(),
			Hostname:      p.GetHostname(),
			Role:          p.GetRole(),
			ConnectedAtMs: p.GetConnectedAtMs(),
			PeerUID:       p.GetPeerUid(),
			Controller:    p.GetController(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// eventDTO mirrors one inv-driver event for the web UI.
type eventDTO struct {
	ID          int64  `json:"id"`
	TsMs        int64  `json:"ts_ms"`
	InverterUID string `json:"inverter_uid,omitempty"`
	Kind        string `json:"kind"`
	Severity    string `json:"severity"`
	ShortAddr   uint32 `json:"short_addr,omitempty"`
	Detail      string `json:"detail,omitempty"`
	RawHex      string `json:"raw_hex,omitempty"`
}

type eventsDTO struct {
	Events []eventDTO `json:"events"`
	Error  string     `json:"error,omitempty"`
}

// handleEvents queries the inv-driver events log. Filters come from query
// params (since_ms, kind, severity, inverter_uid, limit). An inv-driver
// fetch failure degrades to an empty list + error, never a 5xx.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if s.cfg.EventsFn == nil {
		http.Error(w, "events unavailable", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	req := &wire.EventsRequest{
		Kind:        q.Get("kind"),
		Severity:    q.Get("severity"),
		InverterUid: q.Get("inverter_uid"),
	}
	if v := q.Get("since_ms"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			req.SinceMs = n
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			req.Limit = uint32(n)
		}
	}

	resp, err := s.cfg.EventsFn(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusOK, eventsDTO{Events: []eventDTO{}, Error: err.Error()})
		return
	}
	out := eventsDTO{Events: make([]eventDTO, 0, len(resp.GetEvents()))}
	for _, e := range resp.GetEvents() {
		out.Events = append(out.Events, eventDTO{
			ID:          e.GetId(),
			TsMs:        e.GetTsMs(),
			InverterUID: e.GetInverterUid(),
			Kind:        e.GetKind(),
			Severity:    e.GetSeverity(),
			ShortAddr:   e.GetShortAddr(),
			Detail:      e.GetDetail(),
			RawHex:      e.GetRawHex(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// settingsDTO shapes the /api/settings request and response bodies.
type settingsDTO struct {
	EcuID         string            `json:"ecu_id"`
	Mac           string            `json:"mac"`
	PanOverride   string            `json:"pan_override"`
	ZigbeeType    string            `json:"zigbee_type"`
	InverterNames map[string]string `json:"inverter_names,omitempty"`
	Error         string            `json:"error,omitempty"`
}

func settingsToDTO(s *wire.Settings) settingsDTO {
	return settingsDTO{
		EcuID:         s.GetEcuId(),
		Mac:           s.GetMac(),
		PanOverride:   s.GetPanOverride(),
		ZigbeeType:    s.GetZigbeeType(),
		InverterNames: s.GetInverterNames(),
	}
}

// handleGetSettings reports the current ECU settings. A missing fetcher or
// an inv-driver fetch failure degrades to an error field, never a 5xx.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SettingsGet == nil {
		writeJSON(w, http.StatusOK, settingsDTO{Error: "settings unavailable"})
		return
	}
	resp, err := s.cfg.SettingsGet(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, settingsDTO{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, settingsToDTO(resp.GetSettings()))
}

// handleSetSettings writes ECU settings and returns the values in effect
// afterwards. A rejected write (resp.Ok==false) returns HTTP 400.
func (s *Server) handleSetSettings(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SettingsSet == nil {
		http.Error(w, "settings unavailable", http.StatusServiceUnavailable)
		return
	}
	var body settingsDTO
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	resp, err := s.cfg.SettingsSet(r.Context(), &wire.Settings{
		EcuId:         body.EcuID,
		Mac:           body.Mac,
		PanOverride:   body.PanOverride,
		ZigbeeType:    body.ZigbeeType,
		InverterNames: body.InverterNames,
	})
	if err != nil {
		msg := err.Error()
		if resp != nil && resp.GetError() != "" {
			msg = resp.GetError()
		}
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, settingsToDTO(resp.GetSettings()))
}

// systemStatus returns a cached SystemStatus (<=sysCacheTTL old) or
// fetches a fresh one. Returns an error if no fetcher is configured or
// the fetch fails.
func (s *Server) systemStatus(ctx context.Context) (*wire.SystemStatusResponse, error) {
	s.sysMu.Lock()
	defer s.sysMu.Unlock()
	if s.sysResp != nil && time.Since(s.sysAt) < sysCacheTTL {
		return s.sysResp, nil
	}
	if s.cfg.SysStatus == nil {
		return nil, errors.New("system status unavailable")
	}
	resp, err := s.cfg.SysStatus(ctx)
	if err != nil {
		return nil, err
	}
	s.sysResp = resp
	s.sysAt = time.Now()
	return resp, nil
}

// --- static SPA serving ---

// staticHandler serves the embedded SPA with history-API fallback: any
// GET that isn't an existing asset and doesn't look like an API call
// returns index.html so client-side routing works on deep links.
func (s *Server) staticHandler() http.Handler {
	fileServer := http.FileServer(http.FS(s.cfg.Assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if upath == "" {
			upath = "index.html"
		}
		if _, err := fs.Stat(s.cfg.Assets, upath); err != nil {
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				serveIndex(w, r2, s.cfg.Assets)
				return
			}
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	b, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		http.Error(w, "index missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// readPassword extracts {"password": "..."} from a JSON body.
func readPassword(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return "", false
	}
	if body.Password == "" {
		http.Error(w, "password required", http.StatusBadRequest)
		return "", false
	}
	return body.Password, true
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SSE connections are long-lived; don't spam a line per stream tick.
		if r.URL.Path != "/api/stream" {
			log.Printf("http: %s %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}

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
	"fmt"
	"io/fs"
	"log"
	"math"
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
	"github.com/bolke/inv-driver/codec"
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
	// GridProfileFn issues one grid-profile management op to inv-driver.
	// Nil degrades GET /api/gridprofile to an error field and makes
	// POST /api/gridprofile/select return a 503.
	GridProfileFn func(context.Context, *wire.GridProfileRequest) (*wire.GridProfileResponse, error)
	// SendFrame injects one L2 frame to a single inverter (by peer_uid) via
	// inv-driver. Nil makes POST /api/power return a 503. Used to apply
	// set-power (output-cap) commands the server encodes from the request.
	SendFrame func(context.Context, string, []byte) error
	// PairingFn issues one pairing op (scan/add/replace/rekey/abort/status)
	// to inv-driver. Nil makes the POST /api/pairing/* endpoints return a
	// 503 and GET /api/pairing/status report unavailable.
	PairingFn func(context.Context, *wire.PairingRequest) (*wire.PairingResponse, error)
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
	mux.HandleFunc("POST /api/auth/recover", s.handleAuthRecover)
	// Account management (authenticated).
	mux.Handle("POST /api/auth/change-password", s.cfg.Auth.Require(http.HandlerFunc(s.handleChangePassword)))
	mux.Handle("POST /api/auth/recovery", s.cfg.Auth.Require(http.HandlerFunc(s.handleRegenerateRecovery)))
	// Step-up: re-verify the operator password without minting a new
	// session. Used to confirm destructive UI actions like a MAC change
	// that would re-PAN the radio.
	mux.Handle("POST /api/auth/verify", s.cfg.Auth.Require(http.HandlerFunc(s.handleAuthVerify)))

	// Protected data endpoints.
	mux.Handle("GET /api/fleet", s.cfg.Auth.Require(http.HandlerFunc(s.handleFleet)))
	mux.Handle("GET /api/system", s.cfg.Auth.Require(http.HandlerFunc(s.handleSystem)))
	mux.Handle("GET /api/events", s.cfg.Auth.Require(http.HandlerFunc(s.handleEvents)))
	mux.Handle("GET /api/settings", s.cfg.Auth.Require(http.HandlerFunc(s.handleGetSettings)))
	mux.Handle("PUT /api/settings", s.cfg.Auth.Require(http.HandlerFunc(s.handleSetSettings)))
	mux.Handle("GET /api/profiles", s.cfg.Auth.Require(http.HandlerFunc(s.handleGetProfiles)))
	mux.Handle("GET /api/overlays", s.cfg.Auth.Require(http.HandlerFunc(s.handleGetOverlays)))
	mux.Handle("POST /api/profiles/base", s.cfg.Auth.Require(http.HandlerFunc(s.handleSelectBase)))
	mux.Handle("PUT /api/profiles/overlay", s.cfg.Auth.Require(http.HandlerFunc(s.handlePutOverlay)))
	mux.Handle("DELETE /api/profiles/overlay", s.cfg.Auth.Require(http.HandlerFunc(s.handleDeleteOverlay)))
	mux.Handle("GET /api/history", s.cfg.Auth.Require(http.HandlerFunc(s.handleHistory)))
	mux.Handle("POST /api/power", s.cfg.Auth.Require(http.HandlerFunc(s.handleSetPower)))
	mux.Handle("POST /api/pairing/scan", s.cfg.Auth.Require(http.HandlerFunc(s.handlePairingScan)))
	mux.Handle("POST /api/pairing/add", s.cfg.Auth.Require(http.HandlerFunc(s.handlePairingAdd)))
	mux.Handle("POST /api/pairing/replace", s.cfg.Auth.Require(http.HandlerFunc(s.handlePairingReplace)))
	mux.Handle("POST /api/pairing/rekey", s.cfg.Auth.Require(http.HandlerFunc(s.handlePairingRekey)))
	mux.Handle("POST /api/pairing/change-channel", s.cfg.Auth.Require(http.HandlerFunc(s.handlePairingChangeChannel)))
	mux.Handle("POST /api/pairing/abort", s.cfg.Auth.Require(http.HandlerFunc(s.handlePairingAbort)))
	mux.Handle("GET /api/pairing/status", s.cfg.Auth.Require(http.HandlerFunc(s.handlePairingStatus)))
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
		"recovery_set":  s.cfg.Auth.RecoveryConfigured(),
	})
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	pw, ok := readPassword(w, r)
	if !ok {
		return
	}
	// Setup is atomic: it returns ErrAlreadyConfigured rather than relying on a
	// separate Configured() check (which races on this public endpoint).
	code, err := s.cfg.Auth.Setup(pw)
	if err != nil {
		if errors.Is(err, auth.ErrAlreadyConfigured) {
			http.Error(w, "already configured", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tok, err := s.cfg.Auth.NewSession()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	auth.SetCookie(w, tok, true)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "recovery_code": code})
}

// handleAuthRecover resets the password from the recovery code (public, no
// session required) and returns the rotated recovery code.
func (s *Server) handleAuthRecover(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RecoveryCode string `json:"recovery_code"`
		Password     string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.RecoveryCode == "" || body.Password == "" {
		http.Error(w, "recovery code and new password required", http.StatusBadRequest)
		return
	}
	newCode, err := s.cfg.Auth.Recover(body.RecoveryCode, body.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrNotConfigured), errors.Is(err, auth.ErrNoRecovery):
			http.Error(w, "recovery unavailable", http.StatusConflict)
		case errors.Is(err, auth.ErrInvalidRecovery):
			http.Error(w, "invalid recovery code", http.StatusUnauthorized)
		default:
			http.Error(w, err.Error(), http.StatusBadRequest) // e.g. password too short
		}
		return
	}
	tok, err := s.cfg.Auth.NewSession()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	auth.SetCookie(w, tok, true)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "recovery_code": newCode})
}

// handleChangePassword replaces the password after verifying the current one.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.CurrentPassword == "" || body.NewPassword == "" {
		http.Error(w, "current and new password required", http.StatusBadRequest)
		return
	}
	if err := s.cfg.Auth.ChangePassword(body.CurrentPassword, body.NewPassword); err != nil {
		if errors.Is(err, auth.ErrInvalidPassword) {
			http.Error(w, "current password incorrect", http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleAuthVerify re-verifies the operator password without minting a new
// session. Used by the UI as a step-up confirm for destructive actions; on
// success it MARKS the session as stepped-up so subsequent sensitive writes
// (MAC / PAN-override changes on PUT /api/settings) within stepUpTTL pass
// the server-side gate. 200 = correct, 401 = wrong, 400 = missing.
func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	pw, ok := readPassword(w, r)
	if !ok {
		return
	}
	match, err := s.cfg.Auth.Verify(pw)
	if err != nil {
		// ErrNotConfigured is the only expected error here; treat it as
		// a 401 so the client doesn't leak the configured/unconfigured
		// distinction past the auth gate.
		http.Error(w, "invalid password", http.StatusUnauthorized)
		return
	}
	if !match {
		http.Error(w, "invalid password", http.StatusUnauthorized)
		return
	}
	s.cfg.Auth.MarkStepUp(r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleRegenerateRecovery issues a fresh recovery code for the logged-in
// operator and returns it once.
func (s *Server) handleRegenerateRecovery(w http.ResponseWriter, r *http.Request) {
	code, err := s.cfg.Auth.RegenerateRecovery()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"recovery_code": code})
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

// powerReqDTO is the POST /api/power body. Exactly one target selects the
// scope: a non-empty uid caps that one inverter; array=true caps the whole
// fleet, where watts is the array-wide ceiling distributed across online
// inverters proportionally to each one's nameplate. watts is the target AC
// output in watts.
type powerReqDTO struct {
	UID   string `json:"uid"`
	Array bool   `json:"array"`
	Watts int    `json:"watts"`
}

// powerResultDTO reports the outcome for one inverter. AppliedWatts is what
// was actually commanded after clamping to the hardware envelope (the per-
// panel floor/ceiling), which may differ from the request.
type powerResultDTO struct {
	UID          string `json:"uid"`
	OK           bool   `json:"ok"`
	AppliedWatts int    `json:"applied_watts"`
	Error        string `json:"error,omitempty"`
}

type powerRespDTO struct {
	Results []powerResultDTO `json:"results"`
	Error   string           `json:"error,omitempty"`
}

// handleSetPower applies an output cap. Set-power is per-panel watts at the
// codec; this handler takes whole-inverter (or whole-array) watts, splits by
// panel count, clamps to the codec envelope, encodes per family, and sends
// one unicast per inverter (never a broadcast — mixed families and per-panel
// floors need per-inverter values).
func (s *Server) handleSetPower(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SendFrame == nil {
		http.Error(w, "power control unavailable", http.StatusServiceUnavailable)
		return
	}
	var body powerReqDTO
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Watts <= 0 {
		http.Error(w, "watts must be positive", http.StatusBadRequest)
		return
	}

	fleet := s.cfg.Snap.Fleet(time.Now())

	// targets maps each inverter to its individual target watts.
	type target struct {
		inv   snapshot.InverterDTO
		watts int
	}
	var targets []target

	if body.Array {
		var online []snapshot.InverterDTO
		var totalNameplate int
		for _, inv := range fleet.Inverters {
			if inv.Online && inv.NameplateW > 0 && len(inv.Panels) > 0 {
				online = append(online, inv)
				totalNameplate += int(inv.NameplateW)
			}
		}
		if len(online) == 0 || totalNameplate == 0 {
			http.Error(w, "no online inverters to cap", http.StatusConflict)
			return
		}
		pct := float64(body.Watts) / float64(totalNameplate)
		if pct > 1 {
			pct = 1
		}
		for _, inv := range online {
			targets = append(targets, target{inv: inv, watts: int(pct * float64(inv.NameplateW))})
		}
	} else {
		var found *snapshot.InverterDTO
		for i := range fleet.Inverters {
			if fleet.Inverters[i].UID == body.UID {
				found = &fleet.Inverters[i]
				break
			}
		}
		if found == nil {
			http.Error(w, "unknown inverter", http.StatusBadRequest)
			return
		}
		targets = append(targets, target{inv: *found, watts: body.Watts})
	}

	out := powerRespDTO{Results: make([]powerResultDTO, 0, len(targets))}
	for _, t := range targets {
		frame, applied, err := capFrame(t.inv, t.watts)
		res := powerResultDTO{UID: t.inv.UID, AppliedWatts: applied}
		if err != nil {
			res.Error = err.Error()
			out.Results = append(out.Results, res)
			continue
		}
		if err := s.cfg.SendFrame(r.Context(), t.inv.UID, frame); err != nil {
			res.Error = err.Error()
			out.Results = append(out.Results, res)
			continue
		}
		res.OK = true
		out.Results = append(out.Results, res)
	}
	writeJSON(w, http.StatusOK, out)
}

// capFrame builds the set-power L2 frame for one inverter from a whole-
// inverter watt target, treated as a fraction of nameplate: per-panel =
// round((targetW / nameplate) × 500), where per-panel 500 = full output. This
// always reaches nameplate (→ 500 = uncapped) and doesn't depend on the
// reported panel count. Returns the encoded frame plus the watts actually
// commanded (the fraction × nameplate, after clamping to the codec envelope).
func capFrame(inv snapshot.InverterDTO, targetW int) ([]byte, int, error) {
	nameplate := int(inv.NameplateW)
	if nameplate <= 0 {
		return nil, 0, fmt.Errorf("no nameplate for inverter")
	}
	frac := float64(targetW) / float64(nameplate)
	if frac > 1 {
		frac = 1
	}
	perPanel := int(math.Round(frac * float64(codec.MaxPanelLimitW)))
	if perPanel < int(codec.MinPanelLimitW) {
		perPanel = int(codec.MinPanelLimitW)
	}
	if perPanel > int(codec.MaxPanelLimitW) {
		perPanel = int(codec.MaxPanelLimitW)
	}
	applied := int(math.Round(float64(perPanel) / float64(codec.MaxPanelLimitW) * float64(nameplate)))
	frame, err := codec.EncodeSetPower(inv.ModelCode, uint16(perPanel), false)
	if err != nil {
		return nil, applied, err
	}
	return frame, applied, nil
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
	By          string `json:"by,omitempty"` // originating backend (Hello name)
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
			By:          e.GetBy(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// effectiveDTO carries the values inv-driver is actually using right now
// (operator inputs with empty fields resolved against the live system).
type effectiveDTO struct {
	Mac        string `json:"mac"`
	Pan        string `json:"pan"`
	ZigbeeType string `json:"zigbee_type"`
	Channel    uint32 `json:"channel"`
}

// settingsDTO shapes the /api/settings request and response bodies.
type settingsDTO struct {
	EcuID         string            `json:"ecu_id"`
	Mac           string            `json:"mac"`
	PanOverride   string            `json:"pan_override"`
	ZigbeeType    string            `json:"zigbee_type"`
	Channel       uint32            `json:"channel"`
	InverterNames map[string]string `json:"inverter_names,omitempty"`
	Effective     *effectiveDTO     `json:"effective,omitempty"`
	Error         string            `json:"error,omitempty"`
}

func settingsToDTO(resp *wire.SettingsResponse) settingsDTO {
	s := resp.GetSettings()
	out := settingsDTO{
		EcuID:         s.GetEcuId(),
		Mac:           s.GetMac(),
		PanOverride:   s.GetPanOverride(),
		ZigbeeType:    s.GetZigbeeType(),
		Channel:       s.GetChannel(),
		InverterNames: s.GetInverterNames(),
	}
	if eff := resp.GetEffective(); eff != nil {
		out.Effective = &effectiveDTO{
			Mac:        eff.GetMac(),
			Pan:        eff.GetPan(),
			ZigbeeType: eff.GetZigbeeType(),
			Channel:    eff.GetChannel(),
		}
	}
	return out
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
	writeJSON(w, http.StatusOK, settingsToDTO(resp))
}

// handleSetSettings writes ECU settings and returns the values in effect
// afterwards. A rejected write (resp.Ok==false) returns HTTP 400.
//
// Sensitive fields (mac, pan_override) are gated by a per-session step-up:
// the operator must POST /api/auth/verify successfully within stepUpTTL
// before a request that changes either field is accepted. A hand-rolled
// client that skips the UI confirm dialog hits a 403 here.
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
	sensitive := s.sensitiveChange(r.Context(), body)
	if sensitive && !s.cfg.Auth.StepUpValid(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "step-up required"})
		return
	}
	resp, err := s.cfg.SettingsSet(r.Context(), &wire.Settings{
		EcuId:         body.EcuID,
		Mac:           body.Mac,
		PanOverride:   body.PanOverride,
		ZigbeeType:    body.ZigbeeType,
		Channel:       body.Channel,
		InverterNames: body.InverterNames,
	})
	if err != nil {
		msg := err.Error()
		if resp != nil && resp.GetError() != "" {
			msg = resp.GetError()
		}
		// Do NOT consume step-up on failure: the operator may retry the
		// same sensitive write without re-typing the password.
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	// Single-use step-up: a successful sensitive write consumes the flag
	// so any subsequent sensitive write inside stepUpTTL requires a fresh
	// /api/auth/verify. Non-sensitive writes do not touch step-up.
	if sensitive {
		s.cfg.Auth.ConsumeStepUp(r)
	}
	writeJSON(w, http.StatusOK, settingsToDTO(resp))
}

// sensitiveChange reports whether body changes a step-up-gated field (mac
// or pan_override) relative to the currently-persisted settings. If the
// current value can't be read (no fetcher / fetch failure), the change is
// treated as sensitive — fail-closed.
func (s *Server) sensitiveChange(ctx context.Context, body settingsDTO) bool {
	if s.cfg.SettingsGet == nil {
		return body.Mac != "" || body.PanOverride != ""
	}
	cur, err := s.cfg.SettingsGet(ctx)
	if err != nil || cur == nil {
		return body.Mac != "" || body.PanOverride != ""
	}
	prev := cur.GetSettings()
	return body.Mac != prev.GetMac() || body.PanOverride != prev.GetPanOverride()
}

// gridProfileSummaryDTO is one selectable base profile (shared by the
// Profiles screen's base selector).
type gridProfileSummaryDTO struct {
	ID         string `json:"id"`
	VNomV      int    `json:"vnom_v"`
	SourceRef  string `json:"source_ref,omitempty"`
	PointCount int    `json:"point_count"`
}

// ipcStatus / ipcProfileSummary mirror the JSON inv-driver's grid-profile
// manager returns for get_status and list_profiles.
type ipcStatus struct {
	ActiveBase      string `json:"active_base"`
	ReconcilerReady bool   `json:"reconciler_ready"`
}

type ipcProfileSummary struct {
	ID     string `json:"id"`
	VNomV  int    `json:"vnom_v"`
	Source struct {
		Ref string `json:"ref"`
	} `json:"source"`
	PointCount int `json:"point_count"`
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
		// The SPA bundle has a stable name (assets/main.js), so without this a
		// browser keeps serving a cached bundle after a redeploy. no-cache makes
		// it revalidate, so a deploy is picked up on the next load.
		w.Header().Set("Cache-Control", "no-cache")
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

// decodeJSON decodes a small JSON request body into v, writing a 400 and
// returning false on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(v); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return false
	}
	return true
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

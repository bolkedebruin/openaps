package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/bolke/inv-driver/wire"
)

// pairingErrMsg picks the most informative error string out of a
// (resp, err) pair returned by PairingFn: prefer inv-driver's structured
// error, else the transport error.
func pairingErrMsg(resp *wire.PairingResponse, err error) string {
	if resp != nil && resp.GetError() != "" {
		return resp.GetError()
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

// pairingStatusResp is the body returned by every pairing endpoint. It wraps
// the raw PairingStatus JSON that inv-driver maintains in memory (passed
// through verbatim so the Go and TS sides need not re-derive the shape) plus
// a flat ok/error for transport-level failures.
type pairingStatusResp struct {
	OK     bool            `json:"ok"`
	Error  string          `json:"error,omitempty"`
	Status json.RawMessage `json:"status,omitempty"`
}

// writePairingResp renders a PairingResponse as the standard wrapper. statusOK
// is the HTTP status used when the op was accepted.
func (s *Server) writePairingResp(w http.ResponseWriter, resp *wire.PairingResponse, err error, statusOK int) {
	if err != nil || resp == nil || !resp.GetOk() {
		out := pairingStatusResp{OK: false, Error: pairingErrMsg(resp, err)}
		if resp != nil {
			out.Status = json.RawMessage(resp.GetStatusJson())
		}
		writeJSON(w, http.StatusBadRequest, out)
		return
	}
	writeJSON(w, statusOK, pairingStatusResp{OK: true, Status: json.RawMessage(resp.GetStatusJson())})
}

// handlePairingScan starts a discovery scan. fast (slow=false) solicits on
// the current channel parked on PAN 0xFFFF; slow sweeps the channel range and
// blacks out telemetry — the UI warns. Found serials are bound after the scan.
func (s *Server) handlePairingScan(w http.ResponseWriter, r *http.Request) {
	if s.cfg.PairingFn == nil {
		http.Error(w, "pairing unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Slow    bool   `json:"slow"`
		ChanLo  uint32 `json:"chan_lo"`
		ChanHi  uint32 `json:"chan_hi"`
		DwellMs uint32 `json:"dwell_ms"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	resp, err := s.cfg.PairingFn(r.Context(), &wire.PairingRequest{
		Op: &wire.PairingRequest_Scan{Scan: &wire.ScanStart{
			Slow: body.Slow, ChanLo: body.ChanLo, ChanHi: body.ChanHi, DwellMs: body.DwellMs}}})
	s.writePairingResp(w, resp, err, http.StatusAccepted)
}

// handlePairingAdd inserts a known 12-digit serial then runs bind (+migrate).
func (s *Server) handlePairingAdd(w http.ResponseWriter, r *http.Request) {
	if s.cfg.PairingFn == nil {
		http.Error(w, "pairing unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Serial string `json:"serial"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Serial == "" {
		http.Error(w, "serial is required", http.StatusBadRequest)
		return
	}
	resp, err := s.cfg.PairingFn(r.Context(), &wire.PairingRequest{
		Op: &wire.PairingRequest_AddById{AddById: &wire.AddById{Serial: body.Serial}}})
	s.writePairingResp(w, resp, err, http.StatusAccepted)
}

// handlePairingReplace swaps a dead unit for a new one, inheriting the old
// unit's grid profile + power cap + array slot.
func (s *Server) handlePairingReplace(w http.ResponseWriter, r *http.Request) {
	if s.cfg.PairingFn == nil {
		http.Error(w, "pairing unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		OldUID    string `json:"old_uid"`
		NewSerial string `json:"new_serial"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.OldUID == "" {
		http.Error(w, "old_uid is required", http.StatusBadRequest)
		return
	}
	resp, err := s.cfg.PairingFn(r.Context(), &wire.PairingRequest{
		Op: &wire.PairingRequest_Replace{Replace: &wire.ReplaceInverter{
			OldUid: body.OldUID, NewSerial: body.NewSerial}}})
	s.writePairingResp(w, resp, err, http.StatusAccepted)
}

// handlePairingRekey moves the whole fleet to a new PAN (broadcast 0x22).
// Gated by step-up like the other PAN-changing path: the operator must POST
// /api/auth/verify within stepUpTTL first, since this re-PANs the radio and
// blacks out telemetry while it runs.
func (s *Server) handlePairingRekey(w http.ResponseWriter, r *http.Request) {
	if s.cfg.PairingFn == nil {
		http.Error(w, "pairing unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		NewPan  string `json:"new_pan"`
		Channel uint32 `json:"channel"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !s.cfg.Auth.StepUpValid(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "step-up required"})
		return
	}
	resp, err := s.cfg.PairingFn(r.Context(), &wire.PairingRequest{
		Op: &wire.PairingRequest_FleetRekey{FleetRekey: &wire.FleetRekey{
			NewPan: body.NewPan, Channel: body.Channel}}})
	if resp != nil && resp.GetOk() {
		// Single-use: a successful broadcast consumes the step-up flag.
		s.cfg.Auth.ConsumeStepUp(r)
	}
	s.writePairingResp(w, resp, err, http.StatusAccepted)
}

// handlePairingAbort requests a safe abort of the active op.
func (s *Server) handlePairingAbort(w http.ResponseWriter, r *http.Request) {
	if s.cfg.PairingFn == nil {
		http.Error(w, "pairing unavailable", http.StatusServiceUnavailable)
		return
	}
	resp, err := s.cfg.PairingFn(r.Context(), &wire.PairingRequest{
		Op: &wire.PairingRequest_Abort{Abort: &wire.Empty{}}})
	s.writePairingResp(w, resp, err, http.StatusOK)
}

// handlePairingStatus returns the live in-memory PairingStatus. The progress
// drawer polls this ~1s while an op is active. A fetch failure degrades to an
// ok=false body, never a 5xx, so a transient inv-driver round-trip error
// doesn't tear down the drawer's poll loop.
func (s *Server) handlePairingStatus(w http.ResponseWriter, r *http.Request) {
	if s.cfg.PairingFn == nil {
		writeJSON(w, http.StatusOK, pairingStatusResp{OK: false, Error: "pairing unavailable"})
		return
	}
	resp, err := s.cfg.PairingFn(r.Context(), &wire.PairingRequest{
		Op: &wire.PairingRequest_GetStatus{GetStatus: &wire.Empty{}}})
	if err != nil {
		out := pairingStatusResp{OK: false, Error: pairingErrMsg(resp, err)}
		if resp != nil {
			out.Status = json.RawMessage(resp.GetStatusJson())
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	writeJSON(w, http.StatusOK, pairingStatusResp{OK: true, Status: json.RawMessage(resp.GetStatusJson())})
}

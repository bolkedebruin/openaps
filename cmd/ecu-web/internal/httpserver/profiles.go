package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bolkedebruin/openaps/internal/gridprofile"
	"github.com/bolkedebruin/openaps/wire"
)

// gridProfileErrMsg picks the most informative error string out of a
// (resp, err) pair returned by GridProfileFn: if inv-driver attached a
// structured error message, prefer that; otherwise fall back to the raw
// transport error. Shared across handleSelectBase, handlePutOverlay, and
// handleDeleteOverlay so an empty/missing inv-driver error never erases the
// transport error.
func gridProfileErrMsg(resp *wire.GridProfileResponse, err error) string {
	if resp != nil && resp.GetError() != "" {
		return resp.GetError()
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

// profilesDTO is the GET /api/profiles payload: the fleet-wide base profile,
// the named Local Site profiles (overlays), the inverters available as targets
// (with their writable parameter codes), and the editable parameter catalog.
type profilesDTO struct {
	Base          profileBaseDTO             `json:"base"`
	BaseDefaults  map[string]defaultDTO      `json:"base_defaults"`
	Overlays      []localSiteDTO             `json:"overlays"`
	Inverters     []profileInvDTO            `json:"inverters"`
	Params        []gridprofile.ParamInfo    `json:"params"`
	ConflictRules []gridprofile.ConflictRule `json:"conflict_rules"`
	Error         string                     `json:"error,omitempty"`
}

// defaultDTO is the active base profile's value (and allowed range, if any)
// for one parameter.
type defaultDTO struct {
	Value float64  `json:"value"`
	Unit  string   `json:"unit"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
}

type profileBaseDTO struct {
	ActiveBase      string                  `json:"active_base"`
	ReconcilerReady bool                    `json:"reconciler_ready"`
	Profiles        []gridProfileSummaryDTO `json:"profiles"`
}

// localSiteDTO is one named Local Site profile (an overlay) and its targets.
type localSiteDTO struct {
	ID     string              `json:"id"`
	UIDs   []string            `json:"uids"`
	Points []localSitePointDTO `json:"points"`
}

type localSitePointDTO struct {
	ApsCode string  `json:"aps_code"`
	Value   float64 `json:"value"`
	Unit    string  `json:"unit,omitempty"`
}

type profileInvDTO struct {
	UID           string             `json:"uid"`
	Model         string             `json:"model"`
	ModelCode     uint8              `json:"model_code"`
	WritableCodes []string           `json:"writable_codes"`
	Current       map[string]float64 `json:"current"` // aps_code -> the inverter's current value
}

// currentValues returns the inverter's protection readback as aps-code -> value
// (the Protection.values map verbatim — single source of truth). The one editor
// nicety: the over-frequency curtailment start is read back as DC (the read
// alias) but the editor shows it under the writable code CA, so alias DC -> CA.
func currentValues(p *wire.Protection) map[string]float64 {
	m := map[string]float64{}
	if p == nil {
		return m
	}
	for k, v := range p.GetValues() {
		m[k] = v
	}
	if v, ok := m["DC"]; ok {
		m["CA"] = v
	}
	return m
}

// handleGetProfiles aggregates everything the Profiles screen needs. The
// inverters and parameter catalog are local (always returned); the base and
// overlays come from inv-driver and degrade to an error field, never a 5xx.
func (s *Server) handleGetProfiles(w http.ResponseWriter, r *http.Request) {
	out := profilesDTO{
		Inverters:     s.fleetTargets(),
		Params:        gridprofile.ParamCatalog(),
		Overlays:      []localSiteDTO{},
		BaseDefaults:  map[string]defaultDTO{},
		ConflictRules: gridprofile.ConflictRules(),
	}
	if s.cfg.GridProfileFn == nil {
		out.Error = "grid profile unavailable"
		writeJSON(w, http.StatusOK, out)
		return
	}

	// Base: status (active + reconciler) + list (rich summaries).
	statusResp, err := s.cfg.GridProfileFn(r.Context(), &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_GetStatus{GetStatus: &wire.Empty{}}})
	if err != nil {
		out.Error = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	var st ipcStatus
	if err := json.Unmarshal(statusResp.GetJson(), &st); err != nil {
		out.Error = "parse status: " + err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	out.Base.ActiveBase = st.ActiveBase
	out.Base.ReconcilerReady = st.ReconcilerReady

	listResp, err := s.cfg.GridProfileFn(r.Context(), &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_ListProfiles{ListProfiles: &wire.Empty{}}})
	if err != nil {
		out.Error = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	var summaries []ipcProfileSummary
	if err := json.Unmarshal(listResp.GetJson(), &summaries); err == nil {
		out.Base.Profiles = make([]gridProfileSummaryDTO, 0, len(summaries))
		for _, p := range summaries {
			out.Base.Profiles = append(out.Base.Profiles, gridProfileSummaryDTO{
				ID: p.ID, VNomV: p.VNomV, SourceRef: p.Source.Ref, PointCount: p.PointCount})
		}
	}

	// Base defaults: the active base profile's per-code values.
	baseResp, err := s.cfg.GridProfileFn(r.Context(), &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_GetBase{GetBase: &wire.Empty{}}})
	if err != nil {
		out.Error = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	if defs := map[string]defaultDTO{}; json.Unmarshal(baseResp.GetJson(), &defs) == nil {
		out.BaseDefaults = defs
	}

	// Overlays (Local Site profiles).
	overlays, err := s.overlaysList(r.Context())
	if err != nil {
		out.Error = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	out.Overlays = overlays
	writeJSON(w, http.StatusOK, out)
}

// overlaysList fetches the stored Local Site profiles (overlays) and reduces
// each point to {aps_code, value, unit}.
func (s *Server) overlaysList(ctx context.Context) ([]localSiteDTO, error) {
	resp, err := s.cfg.GridProfileFn(ctx, &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_ListOverlays{ListOverlays: &wire.Empty{}}})
	if err != nil {
		return nil, err
	}
	var overlays []gridprofile.Overlay
	if err := json.Unmarshal(resp.GetJson(), &overlays); err != nil {
		return nil, err
	}
	out := make([]localSiteDTO, 0, len(overlays))
	for _, o := range overlays {
		ls := localSiteDTO{ID: o.ID, UIDs: o.UIDs, Points: []localSitePointDTO{}}
		for _, p := range o.Points {
			ls.Points = append(ls.Points, localSitePointDTO{
				ApsCode: p.Apply.ApsCode, Value: p.Native.Value, Unit: p.Native.Unit})
		}
		out = append(out, ls)
	}
	return out, nil
}

// handleGetOverlays returns just the Local Site profiles (lighter than
// /api/profiles): the dashboard uses it to flag inverters with an active
// custom profile. Degrades to an empty list, never a 5xx.
func (s *Server) handleGetOverlays(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GridProfileFn == nil {
		writeJSON(w, http.StatusOK, []localSiteDTO{})
		return
	}
	overlays, err := s.overlaysList(r.Context())
	if err != nil {
		overlays = []localSiteDTO{}
	}
	writeJSON(w, http.StatusOK, overlays)
}

// fleetTargets lists the inverters as overlay targets, each annotated with the
// parameter codes writable on that model (capability filter).
func (s *Server) fleetTargets() []profileInvDTO {
	catalog := gridprofile.ParamCatalog()
	fleet := s.cfg.Snap.Fleet(time.Now())
	out := make([]profileInvDTO, 0, len(fleet.Inverters))
	for _, inv := range fleet.Inverters {
		writable := make([]string, 0, len(catalog))
		for _, p := range catalog {
			if gridprofile.Writeable(inv.ModelCode, p.ApsCode) {
				writable = append(writable, p.ApsCode)
			}
		}
		out = append(out, profileInvDTO{
			UID: inv.UID, Model: inv.Model, ModelCode: inv.ModelCode, WritableCodes: writable,
			Current: currentValues(s.cfg.Snap.Protection(inv.UID))})
	}
	return out
}

// handleSelectBase sets the active fleet-wide base profile (moved here from the
// Settings screen). inv-driver persists the active base synchronously and runs
// the fleet-wide reconcile asynchronously, returning at once; progress lands as
// audit events under by="inv-driver" (profile_apply_started / per-uid
// overlay_apply_* / profile_apply_complete). This handler returns 202 Accepted
// once the reconcile is queued. Validation / round-trip errors return 400.
func (s *Server) handleSelectBase(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GridProfileFn == nil {
		http.Error(w, "grid profile unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	resp, err := s.cfg.GridProfileFn(r.Context(), &wire.GridProfileRequest{
		Op: &wire.GridProfileRequest_SelectBase{SelectBase: &wire.SelectBase{Id: body.ID}}})
	if err != nil || !resp.GetOk() {
		http.Error(w, gridProfileErrMsg(resp, err), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"active_base": body.ID, "status": "reconciling"})
}

// overlayWriteReq is the PUT /api/profiles/overlay body.
type overlayWriteReq struct {
	ID     string   `json:"id"`
	UIDs   []string `json:"uids"`
	Points []struct {
		ApsCode string  `json:"aps_code"`
		Value   float64 `json:"value"`
	} `json:"points"`
}

// overlayQueuedResp is the 202 Accepted body of PUT /api/profiles/overlay.
// inv-driver persists the overlay synchronously and runs the per-UID
// reconcile asynchronously; progress lands as audit events under
// by="inv-driver" (overlay_apply_started / overlay_param_written /
// overlay_param_failed / overlay_apply_complete).
//
// failed[] carries any UIDs whose persist-and-queue step itself failed (e.g.
// inv-driver round-trip error, validation reject). If at least one UID
// queued, the response is 202; if all failed, 400 — body shape unchanged.
type overlayQueuedResp struct {
	ID     string             `json:"id"`
	Status string             `json:"status"`
	UIDs   []string           `json:"uids"`
	Failed []overlayFailedUID `json:"failed,omitempty"`
}

// overlayFailedUID is one entry of the failed[] list returned by PUT
// /api/profiles/overlay.
type overlayFailedUID struct {
	UID   string `json:"uid"`
	Error string `json:"error"`
}

// handlePutOverlay creates or updates a Local Site profile: it builds the
// overlay document and asks inv-driver to apply it to each target inverter.
// inv-driver returns immediately once the overlay is persisted and the
// per-UID reconcile is queued; this handler returns 202 Accepted with the
// queued uids and the operator watches the events log for outcomes. Build/
// validation errors still return 400.
func (s *Server) handlePutOverlay(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GridProfileFn == nil {
		http.Error(w, "grid profile unavailable", http.StatusServiceUnavailable)
		return
	}
	var body overlayWriteReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	if len(body.UIDs) == 0 {
		http.Error(w, "at least one target inverter is required", http.StatusBadRequest)
		return
	}
	if len(body.Points) == 0 {
		http.Error(w, "at least one parameter is required", http.StatusBadRequest)
		return
	}

	points := make([]gridprofile.PointEntry, 0, len(body.Points))
	for _, p := range body.Points {
		pe, err := gridprofile.BuildOverlayPoint(p.ApsCode, p.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		points = append(points, pe)
	}
	overlay := gridprofile.Overlay{
		Schema: gridprofile.SchemaVersion,
		ID:     body.ID,
		UIDs:   body.UIDs,
		Points: points,
	}
	raw, err := json.Marshal(overlay)
	if err != nil {
		http.Error(w, "encode overlay: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Each SetOverlay roundtrip persists-and-queues and returns immediately, so
	// the request context (bounded by the controller read deadline) is the only
	// timeout needed — same as handleDeleteOverlay. A per-uid error is recorded
	// in failed[] so one bad uid never bails the others out of the response.
	queued := make([]string, 0, len(body.UIDs))
	failed := make([]overlayFailedUID, 0)
	for _, uid := range body.UIDs {
		resp, err := s.cfg.GridProfileFn(r.Context(), &wire.GridProfileRequest{
			Op: &wire.GridProfileRequest_SetOverlay{SetOverlay: &wire.OverlaySet{Uid: uid, OverlayJson: raw}}})
		if err != nil {
			failed = append(failed, overlayFailedUID{UID: uid, Error: gridProfileErrMsg(resp, err)})
			continue
		}
		if !resp.GetOk() {
			failed = append(failed, overlayFailedUID{UID: uid, Error: resp.GetError()})
			continue
		}
		queued = append(queued, uid)
	}
	body202 := overlayQueuedResp{ID: body.ID, Status: "queued", UIDs: queued, Failed: failed}
	// At least one queued → 202. Nothing queued → 400 (same body shape so the
	// frontend can render per-uid errors either way).
	status := http.StatusAccepted
	if len(queued) == 0 {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, body202)
}

// handleDeleteOverlay removes a Local Site profile from each of its targets.
// inv-driver clears each overlay row synchronously and queues the per-UID
// reconcile-to-base asynchronously, returning at once; progress lands as audit
// events (overlay_apply_*). This handler returns 202 Accepted with the queued
// uids; the operator watches the events log for outcomes. A per-uid persist/
// queue failure is reported in failed[]; if every uid fails, 400.
func (s *Server) handleDeleteOverlay(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GridProfileFn == nil {
		http.Error(w, "grid profile unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		ID   string   `json:"id"`
		UIDs []string `json:"uids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(body.UIDs) == 0 {
		http.Error(w, "uids are required", http.StatusBadRequest)
		return
	}
	queued := make([]string, 0, len(body.UIDs))
	failed := make([]overlayFailedUID, 0)
	for _, uid := range body.UIDs {
		resp, err := s.cfg.GridProfileFn(r.Context(), &wire.GridProfileRequest{
			Op: &wire.GridProfileRequest_ClearOverlay{ClearOverlay: &wire.ClearOverlay{Uid: uid}}})
		if err != nil || !resp.GetOk() {
			failed = append(failed, overlayFailedUID{UID: uid, Error: gridProfileErrMsg(resp, err)})
			continue
		}
		queued = append(queued, uid)
	}
	status := http.StatusAccepted
	if len(queued) == 0 {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, overlayQueuedResp{ID: body.ID, Status: "reconciling", UIDs: queued, Failed: failed})
}

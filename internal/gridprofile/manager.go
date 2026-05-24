package gridprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/bolke/inv-driver/wire"
)

// uidRe validates IPC uid values: exactly 12 hex chars (case-insensitive).
var uidRe = regexp.MustCompile(`^[0-9A-Fa-f]{12}$`)

// Manager composes a Store and a Reconciler (component B) to serve
// grid-profile IPC operations. The Reconciler field may be nil before
// component B wires it in; callers that require reconciliation will
// receive an error.
//
// All methods are safe for concurrent use.
type Manager struct {
	Store      *Store
	Reconciler *Reconciler
	// ProfilesDir is the on-disk directory from which RefreshProfiles
	// reloads base profiles. May be empty if the startup load is sufficient.
	ProfilesDir string
	// OverlaysDir is the on-disk directory for overlay files.
	OverlaysDir string

	// BroadcastEnabled gates the broadcast whole-profile apply path in
	// selectBase. Default false: selectBase uses unicast ReconcileAll.
	// Set to true only after on-wire validation of the broadcast protection
	// opcode (see GRID_PROFILE_BROADCAST_GROUNDTRUTH.md).
	BroadcastEnabled bool
	// BroadcastSender delivers broadcast L2 frames to the bus. Required when
	// BroadcastEnabled is true; ignored otherwise.
	BroadcastSender Broadcaster
	// BroadcastModelCodes lists the model families targeted by selectBase
	// broadcast. Required when BroadcastEnabled is true.
	BroadcastModelCodes []uint8
}

// Handle dispatches a GridProfileRequest to the appropriate operation
// and returns a GridProfileResponse. Always returns a non-nil response.
func (m *Manager) Handle(ctx context.Context, req *wire.GridProfileRequest) *wire.GridProfileResponse {
	if req == nil {
		return errResp("nil GridProfileRequest")
	}
	switch req.GetOp().(type) {
	case *wire.GridProfileRequest_ListProfiles:
		return m.listProfiles(ctx)
	case *wire.GridProfileRequest_RefreshProfiles:
		return m.refreshProfiles(ctx)
	case *wire.GridProfileRequest_SelectBase:
		return m.selectBase(ctx, req.GetSelectBase().GetId())
	case *wire.GridProfileRequest_SetOverlay:
		return m.setOverlay(ctx, req.GetSetOverlay().GetUid(), req.GetSetOverlay().GetOverlayJson())
	case *wire.GridProfileRequest_ClearOverlay:
		return m.clearOverlay(ctx, req.GetClearOverlay().GetUid())
	case *wire.GridProfileRequest_GetEffective:
		return m.getEffective(ctx, req.GetGetEffective().GetUid())
	case *wire.GridProfileRequest_GetStatus:
		return m.getStatus(ctx)
	case *wire.GridProfileRequest_ListOverlays:
		return m.listOverlays(ctx)
	default:
		return errResp(fmt.Sprintf("unknown GridProfileRequest op %T", req.GetOp()))
	}
}

// profileSummary is the lightweight wire shape for list/refresh responses.
// It carries only the fields needed for display; the full point list is
// omitted to keep 66 profiles well under wire.MaxFrameBytes (64 KiB).
type profileSummary struct {
	ID         string  `json:"id"`
	VNomV      float64 `json:"vnom_v,omitempty"`
	Source     Source  `json:"source"`
	PointCount int     `json:"point_count"`
}

// listProfiles returns lightweight summaries of all stored base profiles.
func (m *Manager) listProfiles(ctx context.Context) *wire.GridProfileResponse {
	profiles, err := m.Store.ListProfiles(ctx)
	if err != nil {
		return errResp(fmt.Sprintf("ListProfiles: %v", err))
	}
	summaries := make([]profileSummary, len(profiles))
	for i, p := range profiles {
		summaries[i] = profileSummary{
			ID:         p.ID,
			VNomV:      p.VNomV,
			Source:     p.Source,
			PointCount: len(p.Points),
		}
	}
	raw, err := json.Marshal(summaries)
	if err != nil {
		return errResp(fmt.Sprintf("marshal profiles: %v", err))
	}
	return okResp(raw)
}

// refreshProfiles reloads profiles from the configured on-disk directory.
func (m *Manager) refreshProfiles(ctx context.Context) *wire.GridProfileResponse {
	if m.ProfilesDir == "" {
		return errResp("profiles dir not configured")
	}
	if err := m.Store.LoadProfilesFromDir(ctx, m.ProfilesDir); err != nil {
		return errResp(fmt.Sprintf("LoadProfilesFromDir: %v", err))
	}
	if m.OverlaysDir != "" {
		if err := m.Store.LoadOverlaysFromDir(ctx, m.OverlaysDir); err != nil {
			return errResp(fmt.Sprintf("LoadOverlaysFromDir: %v", err))
		}
	}
	return m.listProfiles(ctx)
}

// selectBase sets the active base profile and triggers reconciliation of all
// known inverters.
//
// When BroadcastEnabled is true and BroadcastSender is configured, it applies
// the base profile via ApplyBaseBroadcast (broadcast whole-profile push). Points
// that return "unsupported_broadcast" are then reconciled via unicast ReconcileAll.
// When BroadcastEnabled is false (default), unicast ReconcileAll is used directly.
//
// Either way apply-then-read confirmation is performed; encode errors are
// fail-closed.
func (m *Manager) selectBase(ctx context.Context, id string) *wire.GridProfileResponse {
	if id == "" {
		return errResp("SelectBase: id is empty")
	}
	if _, err := m.Store.GetProfile(ctx, id); err != nil {
		return errResp(fmt.Sprintf("SelectBase: profile %q not found: %v", id, err))
	}
	if err := m.Store.SetActiveBase(ctx, id); err != nil {
		return errResp(fmt.Sprintf("SelectBase: %v", err))
	}
	if m.Reconciler == nil {
		return errResp("SelectBase: reconciler not available; base stored but not applied")
	}

	if m.BroadcastEnabled && m.BroadcastSender != nil && len(m.BroadcastModelCodes) > 0 {
		return m.selectBaseViaBroadcast(ctx)
	}

	// Default: unicast ReconcileAll.
	reports, err := m.Reconciler.ReconcileAll(ctx)
	if err != nil {
		return errResp(fmt.Sprintf("SelectBase: ReconcileAll: %v", err))
	}
	raw, err := json.Marshal(reports)
	if err != nil {
		return errResp(fmt.Sprintf("SelectBase: marshal reports: %v", err))
	}
	if summary := reconcileFailedSummary(reports); summary != "" {
		return &wire.GridProfileResponse{Ok: false, Json: raw,
			Error: "SelectBase: one or more points not confirmed: " + summary}
	}
	return okResp(raw)
}

// selectBaseViaBroadcast applies the active base profile via broadcast, then
// falls back to unicast ReconcileAll for any points marked "unsupported_broadcast".
func (m *Manager) selectBaseViaBroadcast(ctx context.Context) *wire.GridProfileResponse {
	bcastRpt, err := m.Reconciler.ApplyBaseBroadcast(ctx, m.BroadcastSender, m.BroadcastModelCodes...)
	if err != nil {
		return errResp(fmt.Sprintf("SelectBase broadcast: %v", err))
	}

	// Also run unicast ReconcileAll to cover unsupported-broadcast points and
	// confirm the full effective state.
	uniReports, err := m.Reconciler.ReconcileAll(ctx)
	if err != nil {
		return errResp(fmt.Sprintf("SelectBase broadcast (unicast follow-up): %v", err))
	}

	combined := struct {
		Broadcast interface{} `json:"broadcast"`
		Unicast   interface{} `json:"unicast"`
	}{
		Broadcast: bcastRpt,
		Unicast:   uniReports,
	}
	raw, err := json.Marshal(combined)
	if err != nil {
		return errResp(fmt.Sprintf("SelectBase: marshal combined report: %v", err))
	}
	return okResp(raw)
}

// setOverlay validates and upserts a per-inverter overlay, then triggers
// reconciliation for that inverter.
func (m *Manager) setOverlay(ctx context.Context, uid string, overlayJSON []byte) *wire.GridProfileResponse {
	if uid == "" {
		return errResp("SetOverlay: uid is empty")
	}
	if !uidRe.MatchString(uid) {
		return errResp(fmt.Sprintf("SetOverlay: uid %q is not a valid 12-hex-char inverter uid", uid))
	}
	if len(overlayJSON) == 0 {
		return errResp("SetOverlay: overlay_json is empty")
	}
	if err := ValidateOverlay(overlayJSON); err != nil {
		return errResp(fmt.Sprintf("SetOverlay: validate: %v", err))
	}
	var o Overlay
	if err := json.Unmarshal(overlayJSON, &o); err != nil {
		return errResp(fmt.Sprintf("SetOverlay: unmarshal: %v", err))
	}
	// Require the IPC uid to appear in the overlay's declared uids list.
	if !overlayContainsUID(o, uid) {
		return errResp(fmt.Sprintf("SetOverlay: uid %q not listed in overlay body uids[]", uid))
	}
	if err := m.Store.UpsertOverlay(ctx, uid, o); err != nil {
		return errResp(fmt.Sprintf("SetOverlay: upsert: %v", err))
	}
	if m.Reconciler == nil {
		return errResp("SetOverlay: reconciler not available; overlay stored but not applied")
	}
	report, err := m.Reconciler.ReconcileUID(ctx, uid)
	if err != nil {
		return errResp(fmt.Sprintf("SetOverlay: ReconcileUID: %v", err))
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return errResp(fmt.Sprintf("SetOverlay: marshal report: %v", err))
	}
	if summary := pointFailedSummary(report.Points); summary != "" {
		return &wire.GridProfileResponse{Ok: false, Json: raw,
			Error: "SetOverlay: one or more points not confirmed: " + summary}
	}
	return okResp(raw)
}

// clearOverlay removes the overlay for an inverter and triggers reconciliation.
func (m *Manager) clearOverlay(ctx context.Context, uid string) *wire.GridProfileResponse {
	if uid == "" {
		return errResp("ClearOverlay: uid is empty")
	}
	if !uidRe.MatchString(uid) {
		return errResp(fmt.Sprintf("ClearOverlay: uid %q is not a valid 12-hex-char inverter uid", uid))
	}
	if err := m.Store.ClearOverlay(ctx, uid); err != nil {
		return errResp(fmt.Sprintf("ClearOverlay: %v", err))
	}
	if m.Reconciler == nil {
		// Overlay cleared; reconciler absent is non-fatal here since the
		// overlay is the write — the next startup will reconcile.
		return okResp(nil)
	}
	report, err := m.Reconciler.ReconcileUID(ctx, uid)
	if err != nil {
		return errResp(fmt.Sprintf("ClearOverlay: ReconcileUID: %v", err))
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return errResp(fmt.Sprintf("ClearOverlay: marshal report: %v", err))
	}
	if summary := pointFailedSummary(report.Points); summary != "" {
		return &wire.GridProfileResponse{Ok: false, Json: raw,
			Error: "ClearOverlay: one or more points not confirmed: " + summary}
	}
	return okResp(raw)
}

// getEffective computes and returns the effective profile for one inverter.
// The effective profile is base⊕overlay, range-intersected, clamped, and
// capability-filtered — identical to what buildDesired applies before any send.
// The Reconciler is required (it exposes buildDesired).
func (m *Manager) getEffective(ctx context.Context, uid string) *wire.GridProfileResponse {
	if uid == "" {
		return errResp("GetEffective: uid is empty")
	}
	if m.Reconciler == nil {
		return errResp("GetEffective: reconciler not available")
	}
	mc, ok := m.Reconciler.modelOf(uid)
	if !ok {
		return errResp(fmt.Sprintf("GetEffective: model code for %q not known", uid))
	}
	baseID, err := m.Store.ActiveBase(ctx)
	if err != nil {
		return errResp(fmt.Sprintf("GetEffective: no active base: %v", err))
	}

	// Reuse buildDesired so GetEffective returns what is actually applied.
	eff, err := m.Reconciler.buildDesired(ctx, uid, mc)
	if err != nil {
		return errResp(fmt.Sprintf("GetEffective: buildDesired: %v", err))
	}
	if eff == nil {
		return errResp("GetEffective: no desired state (no active base)")
	}

	result := struct {
		InverterUID string           `json:"inverter_uid"`
		BaseID      string           `json:"base_id"`
		ModelCode   uint8            `json:"model_code"`
		Points      []EffectivePoint `json:"points"`
	}{
		InverterUID: uid,
		BaseID:      baseID,
		ModelCode:   mc,
		Points:      eff.Points,
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return errResp(fmt.Sprintf("GetEffective: marshal: %v", err))
	}
	return okResp(raw)
}

// getStatus returns latest apply-log entries and active base profile as JSON.
func (m *Manager) getStatus(ctx context.Context) *wire.GridProfileResponse {
	baseID, _ := m.Store.ActiveBase(ctx)
	profiles, _ := m.Store.ListProfiles(ctx)
	profileIDs := make([]string, 0, len(profiles))
	for _, p := range profiles {
		profileIDs = append(profileIDs, p.ID)
	}
	status := struct {
		ActiveBase      string   `json:"active_base"`
		StoredProfiles  []string `json:"stored_profiles"`
		ReconcilerReady bool     `json:"reconciler_ready"`
	}{
		ActiveBase:      baseID,
		StoredProfiles:  profileIDs,
		ReconcilerReady: m.Reconciler != nil,
	}
	raw, err := json.Marshal(status)
	if err != nil {
		return errResp(fmt.Sprintf("GetStatus: marshal: %v", err))
	}
	return okResp(raw)
}

// listOverlays returns all stored overlays — the named "Local Site profiles" —
// one per id with its full target uids and points, serialised as JSON.
func (m *Manager) listOverlays(ctx context.Context) *wire.GridProfileResponse {
	overlays, err := m.Store.ListOverlays(ctx)
	if err != nil {
		return errResp(fmt.Sprintf("ListOverlays: %v", err))
	}
	if overlays == nil {
		overlays = []Overlay{}
	}
	raw, err := json.Marshal(overlays)
	if err != nil {
		return errResp(fmt.Sprintf("ListOverlays: marshal: %v", err))
	}
	return okResp(raw)
}

// okResp constructs a success response with an optional JSON payload.
func okResp(jsonBytes []byte) *wire.GridProfileResponse {
	return &wire.GridProfileResponse{Ok: true, Json: jsonBytes}
}

// errResp constructs a failure response with the given error message.
func errResp(msg string) *wire.GridProfileResponse {
	return &wire.GridProfileResponse{Ok: false, Error: msg}
}

// pointFailedSummary returns a non-empty summary string when any point
// in the list is not in a confirmed state (in_sync or applied).
// An empty string means all points are confirmed.
func pointFailedSummary(points []PointResult) string {
	var failed []string
	for _, pr := range points {
		if pr.Result != "in_sync" && pr.Result != "applied" && pr.Result != "unknown" {
			failed = append(failed, fmt.Sprintf("%s=%s", pr.ApsCode, pr.Result))
		}
	}
	if len(failed) == 0 {
		return ""
	}
	s := ""
	for i, f := range failed {
		if i > 0 {
			s += ", "
		}
		s += f
	}
	return s
}

// reconcileFailedSummary aggregates failed points across all ReconcileReports.
func reconcileFailedSummary(reports []ReconcileReport) string {
	var all []PointResult
	for _, r := range reports {
		all = append(all, r.Points...)
	}
	return pointFailedSummary(all)
}

// overlayContainsUID reports whether the overlay's uids[] list includes uid.
func overlayContainsUID(o Overlay, uid string) bool {
	for _, u := range o.UIDs {
		if u == uid {
			return true
		}
	}
	return false
}

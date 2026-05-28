package gridprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	"github.com/bolke/inv-driver/internal/buslock"
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

	// BusLock, when non-nil, serialises the broadcast-apply path against the
	// pairing state machine (the two are mutually exclusive — both disrupt
	// the fleet bus). MUST be the same *buslock.Lock the pairing manager
	// uses. Nil disables the guard (unicast paths don't need it).
	BusLock *buslock.Lock

	// Events, when non-nil, receives audit events emitted by the async
	// overlay applier (overlay_apply_started / overlay_param_written /
	// overlay_param_failed / overlay_apply_complete). A nil sink disables
	// event emission; the apply still runs.
	Events EventSink

	// ApplyParent is the cancellation root for async overlay applies. Production
	// callers MUST set it to the daemon's root context so shutdown cancels all
	// in-flight applies; nil falls back to context.Background which only suits
	// in-process tests that block until done channels close.
	ApplyParent context.Context

	applierOnce sync.Once
	applierInst *applier
}

// asyncApplier lazily constructs the per-Manager applier instance bound to
// the Reconciler + Events sink.
func (m *Manager) asyncApplier() *applier {
	m.applierOnce.Do(func() {
		m.applierInst = newApplier(m.Reconciler, m.Events)
	})
	return m.applierInst
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
	case *wire.GridProfileRequest_GetBase:
		return m.getBase(ctx)
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
		// Broadcast disrupts the fleet bus; serialise against pairing.
		if m.BusLock != nil {
			if ok, owner := m.BusLock.TryAcquire("gridprofile-broadcast"); !ok {
				return errResp(fmt.Sprintf("bus busy: held by %q (grid-profile broadcast and pairing are mutually exclusive)", owner))
			}
			defer m.BusLock.Release()
		}
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

// setOverlay validates and upserts a per-inverter overlay, then queues an
// async reconcile for that inverter.
//
// Fleet-membership policy: every uid in the overlay body's uids[] list (and
// the IPC uid itself) MUST be a current fleet member as reported by the
// reconciler's modelOf(). An uid that's never been seen — typo or stale
// device — is rejected up-front so gp_overlays can't grow dead rows. An uid
// that's a real fleet member but currently offline is ALSO rejected here:
// modelOf returns false for any uid the store has no recent telemetry for,
// and we prefer "operator re-saves when the inverter is back" over silently
// persisting an overlay that the reconcile-on-reappear path will pick up
// eventually anyway. Consistent with [overlay-apply-is-queue]: persist only
// what we know we can apply.
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
	// Fleet-membership check (see method comment). Verify BEFORE persisting so
	// a typo or stale uid can never create an orphan gp_overlays row.
	if m.Reconciler != nil {
		if _, ok := m.Reconciler.modelOf(uid); !ok {
			return errResp(fmt.Sprintf("SetOverlay: uid %q not in fleet", uid))
		}
		for _, u := range o.UIDs {
			if _, ok := m.Reconciler.modelOf(u); !ok {
				return errResp(fmt.Sprintf("SetOverlay: uid %q not in fleet", u))
			}
		}
	}
	// Reject a conflicting overlay (base ⊕ overlay) before persisting it, so an
	// incoherent profile never enters the store (the reconcile path is the
	// ultimate backstop, but it would store-then-fail). Same rules as the editor.
	if msgs := m.candidateConflicts(ctx, o); len(msgs) > 0 {
		return errResp("SetOverlay: " + conflictError(msgs).Error())
	}
	if err := m.Store.UpsertOverlay(ctx, uid, o); err != nil {
		return errResp(fmt.Sprintf("SetOverlay: upsert: %v", err))
	}
	if m.Reconciler == nil {
		return errResp("SetOverlay: reconciler not available; overlay stored but not applied")
	}

	// Apply asynchronously: spawn one goroutine per uid (bounded), emit
	// per-point events to the audit log, return immediately. The caller
	// surfaces progress via the events stream.
	parent := m.ApplyParent
	if parent == nil {
		parent = context.Background()
	}
	m.asyncApplier().enqueueWithPoints(parent, uid, o.ID, overlayCodes(o.Points))

	raw, err := json.Marshal(overlayQueuedResp{Status: "queued", ID: o.ID, UIDs: []string{uid}})
	if err != nil {
		return errResp(fmt.Sprintf("SetOverlay: marshal queued: %v", err))
	}
	return okResp(raw)
}

// overlayQueuedResp is the JSON payload of a SetOverlay response under the
// async apply path: the overlay is persisted, the reconcile loop runs in the
// background, and progress is reported via the events log.
type overlayQueuedResp struct {
	Status string   `json:"status"`
	ID     string   `json:"id"`
	UIDs   []string `json:"uids"`
}

// ReconcileOverlaysForUID enqueues an async overlay reconcile for one uid
// IFF an overlay is persisted for it AND the inverter is in the fleet.
// Intended to be called from the ingest OnFirstSeen hook: the protection
// cache is warm by the time first-seen fires (the startup protection read
// has completed), so the reconciler can compare intended-vs-readback
// without spurious no_readback rows. Per-uid hook also re-arms on radio
// rejoin (protLastSeen gap > protReseenGap), so an inverter that goes
// dark and returns re-triggers the overlay reconcile automatically.
//
// No-op (no error) when there is no overlay, no reconciler, or the uid
// isn't in the fleet — those are all expected during normal boot races.
func (m *Manager) ReconcileOverlaysForUID(ctx context.Context, uid string) error {
	if m.Store == nil || m.Reconciler == nil {
		return nil
	}
	if uid == "" || !uidRe.MatchString(uid) {
		return nil
	}
	if _, ok := m.Reconciler.modelOf(uid); !ok {
		return nil
	}
	o, err := m.Store.GetOverlay(ctx, uid)
	if err != nil {
		// sql.ErrNoRows is the common "no overlay for this uid" case — not an
		// error from the caller's perspective.
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("ReconcileOverlaysForUID %q: %w", uid, err)
	}
	parent := m.ApplyParent
	if parent == nil {
		parent = context.Background()
	}
	m.asyncApplier().enqueueWithPoints(parent, uid, o.ID, overlayCodes(o.Points))
	return nil
}

// overlayCodes extracts the APS code list from overlay points, used to scope
// per-point event emission to overlay-owned codes only.
func overlayCodes(points []PointEntry) []string {
	out := make([]string, 0, len(points))
	for _, p := range points {
		out = append(out, p.Apply.ApsCode)
	}
	return out
}

// ReconcileAllOverlays enqueues an async reconcile for every (uid, overlay)
// pair currently persisted in the store. It is idempotent: a uid whose state
// already matches the overlay produces only in_sync rows.
//
// NOTE: This is NOT called at boot — the protection cache is empty until
// the per-uid startup read completes, and the reconciler would treat every
// param as no_readback and emit spurious overlay_param_failed rows.
// Per-uid reconcile is hooked into ingest.OnFirstSeen via
// ReconcileOverlaysForUID, which fires AFTER the protection read settles
// and also re-arms on radio rejoin. ReconcileAllOverlays is retained for
// admin/IPC-driven full-fleet sweeps where the caller has independent
// evidence the cache is warm.
//
// uids absent from the current fleet (modelOf returns false) are skipped —
// they'll converge when the inverter is next seen, same policy as setOverlay
// itself (see its method comment for the fleet-membership rationale).
//
// Best-effort: errors loading the overlay list are logged via the returned
// error but never block boot; the daemon still comes up.
func (m *Manager) ReconcileAllOverlays(ctx context.Context) error {
	if m.Store == nil {
		return nil
	}
	overlays, err := m.Store.ListOverlays(ctx)
	if err != nil {
		return fmt.Errorf("ReconcileAllOverlays: list: %w", err)
	}
	if m.Reconciler == nil {
		// No reconciler yet: nothing to enqueue against. Not an error — boot
		// order may legitimately race the first telemetry.
		return nil
	}
	parent := m.ApplyParent
	if parent == nil {
		parent = context.Background()
	}
	app := m.asyncApplier()
	for _, o := range overlays {
		codes := overlayCodes(o.Points)
		for _, uid := range o.UIDs {
			if _, ok := m.Reconciler.modelOf(uid); !ok {
				continue
			}
			app.enqueueWithPoints(parent, uid, o.ID, codes)
		}
	}
	return nil
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

// baseDefault is one parameter's value (and allowed range, if the base
// profile declares one) in the active base profile.
type baseDefault struct {
	Value float64  `json:"value"`
	Unit  string   `json:"unit"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
}

// getBase returns the active base profile's per-aps-code values and ranges as
// a JSON object {aps_code: {value, unit, min?, max?}} — the defaults an overlay
// overrides, plus the certified envelope to flag out-of-range overrides. An
// empty object means no base is selected.
func (m *Manager) getBase(ctx context.Context) *wire.GridProfileResponse {
	id, _ := m.Store.ActiveBase(ctx)
	defaults := map[string]baseDefault{}
	if id != "" {
		p, err := m.Store.GetProfile(ctx, id)
		if err != nil {
			return errResp(fmt.Sprintf("GetBase: %v", err))
		}
		for _, pt := range p.Points {
			bd := baseDefault{Value: pt.Native.Value, Unit: pt.Native.Unit}
			if pt.Range != nil {
				lo, hi := pt.Range.Min, pt.Range.Max
				bd.Min, bd.Max = &lo, &hi
			}
			defaults[pt.Apply.ApsCode] = bd
		}
	}
	raw, err := json.Marshal(defaults)
	if err != nil {
		return errResp(fmt.Sprintf("GetBase: marshal: %v", err))
	}
	return okResp(raw)
}

// InheritProfile copies any per-inverter overlay from oldUID onto newUID and
// reconciles newUID against the active base, then clears the old overlay. It
// implements the inv-driver-scoped half of the pairing Replace "inherit
// config" step (the grid profile + Local Site overlay). Power cap and
// array-slot layout live outside inv-driver's DB and are re-applied by the
// controller after Replace completes.
//
// Returns a short human-readable summary for the pairing status. A missing
// old overlay is not an error: the replacement simply inherits the active
// base via the reconcile, with no per-inverter overrides.
func (m *Manager) InheritProfile(ctx context.Context, oldUID, newUID string) (string, error) {
	if m.Store == nil {
		return "", fmt.Errorf("InheritProfile: store not available")
	}
	if !uidRe.MatchString(newUID) {
		return "", fmt.Errorf("InheritProfile: new uid %q invalid", newUID)
	}

	o, err := m.Store.GetOverlay(ctx, oldUID)
	switch {
	case err == sql.ErrNoRows:
		// No overlay to inherit — just reconcile the new unit to the base.
		if m.Reconciler != nil {
			if _, rErr := m.Reconciler.ReconcileUID(ctx, newUID); rErr != nil {
				return "", fmt.Errorf("InheritProfile: reconcile %q: %w", newUID, rErr)
			}
		}
		return "no overlay to inherit; applied active base", nil
	case err != nil:
		return "", fmt.Errorf("InheritProfile: get overlay %q: %w", oldUID, err)
	}

	// Remap the overlay's uids[] from old to new so it targets the
	// replacement (the per-uid row key is newUID; the body's uids[] must
	// agree, mirroring setOverlay's membership check).
	remapped := make([]string, 0, len(o.UIDs))
	hasNew := false
	for _, u := range o.UIDs {
		if u == oldUID {
			u = newUID
		}
		if u == newUID {
			hasNew = true
		}
		remapped = append(remapped, u)
	}
	if !hasNew {
		remapped = append(remapped, newUID)
	}
	o.UIDs = remapped

	if err := m.Store.UpsertOverlay(ctx, newUID, o); err != nil {
		return "", fmt.Errorf("InheritProfile: upsert overlay %q: %w", newUID, err)
	}
	_ = m.Store.ClearOverlay(ctx, oldUID)

	if m.Reconciler != nil {
		if _, rErr := m.Reconciler.ReconcileUID(ctx, newUID); rErr != nil {
			return "", fmt.Errorf("InheritProfile: reconcile %q: %w", newUID, rErr)
		}
	}
	return fmt.Sprintf("inherited overlay %q (%d points)", o.ID, len(o.Points)), nil
}

// candidateConflicts evaluates the conflict rules against base ⊕ candidate
// overlay (effective native values), without persisting anything.
func (m *Manager) candidateConflicts(ctx context.Context, ov Overlay) []string {
	eff := map[string]float64{}
	if id, err := m.Store.ActiveBase(ctx); err == nil && id != "" {
		if base, err := m.Store.GetProfile(ctx, id); err == nil {
			for _, p := range base.Points {
				eff[p.Apply.ApsCode] = p.Native.Value
			}
		}
	}
	for _, p := range ov.Points {
		eff[p.Apply.ApsCode] = p.Native.Value
	}
	return CheckConflicts(eff)
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

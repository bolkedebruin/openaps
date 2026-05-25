package gridprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Sender delivers a single L2 frame to one inverter identified by uid.
// Frame bytes are L2 only; the bus-manager wraps the L1 envelope.
type Sender interface {
	Send(ctx context.Context, uid string, frame []byte) error
}

// ReadbackSource provides access to the decoded protection read-back cache
// maintained by the ingest layer.
type ReadbackSource interface {
	// ReadbackNative returns the latest decoded protection values for uid,
	// keyed by APsystems 2-letter code in native units (V, Hz, s).
	// ok is false if no read-back has arrived yet for this inverter.
	ReadbackNative(uid string) (map[string]float64, bool)
	// TriggerRead asks the read pipeline to issue a fresh protection query
	// for uid.  The result arrives asynchronously via ReadbackNative.
	TriggerRead(uid string)
}

// Options configures reconciler behaviour.
type Options struct {
	// AutoReassert re-applies drifted points automatically via unicast when
	// a desired state exists.  Defaults to true.
	AutoReassert bool
	// PeriodicSweep sets the interval for background reconciliation of all
	// known inverters.  Zero disables the background sweep.
	PeriodicSweep time.Duration
	// ReadSettle is the pause between sending a frame and reading back the
	// result.  The protection-read pipeline is asynchronous; ~4 s covers the
	// on-demand read latency.
	ReadSettle time.Duration
}

// defaultOptions returns the recommended defaults.
func defaultOptions() Options {
	return Options{
		AutoReassert: true,
		ReadSettle:   4 * time.Second,
	}
}

// PointResult records the outcome of one apply-or-verify step for a single
// APsystems code.
type PointResult struct {
	ApsCode  string
	Intended float64
	Readback float64 // 0 if not yet read or unknown
	// Result is one of: "in_sync", "applied", "unconfirmed", "unsupported",
	// "no_readback", "unknown" (QS1A page-A), or an error description.
	Result string
}

// ReconcileReport summarises the outcome of one ReconcileUID call.
type ReconcileReport struct {
	UID    string
	Points []PointResult
	// Err holds any top-level error (e.g. no model, encode failure).
	Err error
}

// VerifyReport summarises the outcome of one VerifyStartup call.
type VerifyReport struct {
	UID string
	// IdentifiedID is the best-matching profile id from IdentifyProfile, or ""
	// if no profiles are stored.
	IdentifiedID    string
	IdentifiedScore float64
	// HasDesiredState is true when an active base profile exists.
	HasDesiredState bool
	// Points reports the per-code status (only populated when HasDesiredState).
	Points []PointResult
	// ReconciledOnStartup is true if AutoReassert triggered a ReconcileUID.
	ReconciledOnStartup bool
	Err                 error
}

// toleranceFor returns the per-code native-unit tolerance used in the
// apply-then-read comparison.
//
// The table is sized to absorb ~1 LSB of commanded-vs-readback quantization
// (e.g. DS3 DD commands 16.7 %Pref/Hz but reads back 16.57).
// Codes not explicitly listed use a small default (0.10 native units).
var toleranceFor = map[string]float64{
	// Frequency thresholds (Hz): ~0.05 Hz covers ~1 LSB at int(Hz*100) scale
	"AE": 0.06, "AJ": 0.06, "AF": 0.06, "AK": 0.06,
	"BP": 0.06, "BQ": 0.06,
	// Frequency-watt curve (Hz)
	"DH": 0.06, "DI": 0.06, "CB": 0.06, "CC": 0.06,
	"CA": 0.06, "DC": 0.06,
	// Droop slope (%Pref/Hz): DS3 quantises ~0.13 per step; QS1 is tighter
	"DD": 0.20,
	// Voltage thresholds in %VNom (absolute V / VNom * 100); 1 V at 230 V ≈ 0.43%
	"AC": 0.50, "AQ": 0.50, "AD": 0.50, "AY": 0.50, "AB": 0.50,
	"BN": 0.50, "BO": 0.50,
	// Times (s): ~0.06 s covers integer-×0.01 or ×0.02 encodings
	"BB": 0.06, "BD": 0.06, "BC": 0.06, "BE": 0.06,
	"BJ": 0.06, "BH": 0.06, "BK": 0.06, "BI": 0.06,
	"AG": 0.06, "AS": 0.06,
	// Delay / response time (s)
	"CG": 0.10,
}

// codeTolerance returns the tolerance for an APsystems code.
func codeTolerance(code string) float64 {
	if t, ok := toleranceFor[code]; ok {
		return t
	}
	return 0.10
}

// withinTolerance reports whether a and b are within the per-code tolerance.
func withinTolerance(code string, a, b float64) bool {
	return math.Abs(a-b) <= codeTolerance(code)
}

// Reconciler drives the desired-vs-actual grid-protection state machine.
// Use NewReconciler to construct one.
type Reconciler struct {
	st      *Store
	snd     Sender
	rb      ReadbackSource
	modelOf func(uid string) (uint8, bool)
	opts    Options

	// KnownUIDs, when set, lists every inverter the daemon knows about so a
	// fleet-wide base change reconciles inverters that have no overlay row too
	// (not just UIDs present in gp_overlays). Nil = overlay UIDs only.
	KnownUIDs func(ctx context.Context) []string
}

// NewReconciler constructs a Reconciler.
//
// modelOf must return the APsystems model code (e.g. codec.ModelDS3) for a
// given inverter UID, and false if the model is not yet known.
//
// If opts is a zero value (all fields at their zero values), safe defaults are
// applied: AutoReassert=true, ReadSettle=4s.  An explicit Options{AutoReassert:
// false} is honoured as-is; use it to disable auto-apply.
func NewReconciler(st *Store, snd Sender, rb ReadbackSource, modelOf func(uid string) (uint8, bool), opts Options) *Reconciler {
	zero := Options{}
	if opts == zero {
		opts = defaultOptions()
	}
	return &Reconciler{st: st, snd: snd, rb: rb, modelOf: modelOf, opts: opts}
}

// buildDesired computes the desired EffectiveProfile for uid + modelCode:
// base ⊕ overlay, range-clamped with range intersection, capability-filtered.
// Returns (nil, nil) when no active base is set.
func (r *Reconciler) buildDesired(ctx context.Context, uid string, modelCode uint8) (*EffectiveProfile, error) {
	baseID, err := r.st.ActiveBase(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ActiveBase: %w", err)
	}

	base, err := r.st.GetProfile(ctx, baseID)
	if err != nil {
		return nil, fmt.Errorf("GetProfile %q: %w", baseID, err)
	}

	// Re-validate the stored profile JSON before any apply.
	if rawJSON, marshalErr := json.Marshal(base); marshalErr == nil {
		if valErr := ValidateProfile(rawJSON); valErr != nil {
			return nil, fmt.Errorf("profile %q failed schema validation: %w", baseID, valErr)
		}
	}

	// Index base points by code.
	basePoints := make(map[string]PointEntry, len(base.Points))
	for _, p := range base.Points {
		basePoints[p.Apply.ApsCode] = p
	}

	// Start with base points.
	merged := make(map[string]PointEntry, len(base.Points))
	for k, v := range basePoints {
		merged[k] = v
	}

	// Apply overlay (sparse overrides) with range intersection.
	// The effective range is intersect(base.range, overlay.range) so an
	// overlay can never widen the certified base envelope.
	ov, err := r.st.GetOverlay(ctx, uid)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("GetOverlay %q: %w", uid, err)
	}
	if err == nil {
		for _, ovPt := range ov.Points {
			code := ovPt.Apply.ApsCode
			effective := ovPt // copy

			// Effective range = intersect(physical default, base range, overlay
			// range). Including the per-code physical default means a code the
			// base profile doesn't define is still clamped, so an overlay can't
			// drive an out-of-envelope value to the inverter.
			rng := defaultRange(code)
			if basePt, hasBase := basePoints[code]; hasBase {
				rng = intersectRanges(rng, basePt.Range)
			}
			rng = intersectRanges(rng, ovPt.Range)
			effective.Range = rng
			merged[code] = effective
		}
	}

	// Fail closed on cross-parameter conflicts (e.g. slope start past end,
	// curtailment start above the over-frequency trip). Checked on the merged
	// effective native values so no apply path can bypass the invariant.
	effVals := make(map[string]float64, len(merged))
	for code, entry := range merged {
		v := entry.Native.Value
		if entry.Range != nil && entry.Range.Min <= entry.Range.Max {
			if v < entry.Range.Min {
				v = entry.Range.Min
			} else if v > entry.Range.Max {
				v = entry.Range.Max
			}
		}
		effVals[code] = v
	}
	if err := conflictError(CheckConflicts(effVals)); err != nil {
		return nil, err
	}

	// Build EffectiveProfile: clamp to effective range + capability-filter.
	var points []EffectivePoint
	for code, entry := range merged {
		v, ok := effectivePointValue(modelCode, code, entry)
		if !ok {
			continue
		}
		points = append(points, EffectivePoint{
			ApsCode:     code,
			NativeValue: v,
			Unit:        entry.Native.Unit,
			Range:       entry.Range,
			SF:          entry.SF,
		})
	}

	return &EffectiveProfile{
		InverterUID: uid,
		ModelCode:   modelCode,
		Points:      points,
	}, nil
}

// intersectRanges returns the intersection of two Ranges: the tightest
// envelope that is within both.  If either is nil the other is returned
// as-is.  If the intersection is empty (min > max after intersect) the
// returned range is nil, signalling that the overlay value is out of the
// base envelope and the point should be skipped.
func intersectRanges(base, overlay *Range) *Range {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}
	lo := base.Min
	if overlay.Min > lo {
		lo = overlay.Min
	}
	hi := base.Max
	if overlay.Max < hi {
		hi = overlay.Max
	}
	if lo > hi {
		// Empty intersection: overlay range exceeds base envelope.
		return &Range{Min: lo, Max: lo} // clamp-to-single-point sentinel
	}
	return &Range{Min: lo, Max: hi}
}

// ReconcileUID diffs the desired effective state against the latest read-back
// for uid, and if AutoReassert is enabled, re-applies any drifted points via
// unicast.
//
// Returns a ReconcileReport; the returned error is non-nil only for
// infrastructure failures (model unknown, store error, etc.) — per-point
// encode failures are reported inside the report's Points slice.
func (r *Reconciler) ReconcileUID(ctx context.Context, uid string) (ReconcileReport, error) {
	rpt := ReconcileReport{UID: uid}

	modelCode, ok := r.modelOf(uid)
	if !ok {
		rpt.Err = fmt.Errorf("model code for %q not known", uid)
		return rpt, rpt.Err
	}

	desired, err := r.buildDesired(ctx, uid, modelCode)
	if err != nil {
		rpt.Err = err
		return rpt, err
	}
	if desired == nil {
		// No desired state — nothing to reconcile.
		return rpt, nil
	}

	readback, hasRB := r.rb.ReadbackNative(uid)

	for _, ep := range desired.Points {
		pr := PointResult{ApsCode: ep.ApsCode, Intended: ep.NativeValue}

		if isQS1APageACode(ep.ApsCode) && isQS1Family(modelCode) {
			pr.Result = "unknown"
			rpt.Points = append(rpt.Points, pr)
			continue
		}

		if !hasRB {
			pr.Result = "no_readback"
			rpt.Points = append(rpt.Points, pr)
			continue
		}

		rbVal, codePresent := readback[ep.ApsCode]
		if !codePresent {
			pr.Result = "no_readback"
			rpt.Points = append(rpt.Points, pr)
			continue
		}
		pr.Readback = rbVal

		if withinTolerance(ep.ApsCode, ep.NativeValue, rbVal) {
			pr.Result = "in_sync"
			rpt.Points = append(rpt.Points, pr)
			_ = r.st.AppendApplyLog(ctx, uid, ep.ApsCode, ep.NativeValue, rbVal, "in_sync")
			continue
		}

		// Drifted point.
		if !r.opts.AutoReassert {
			pr.Result = "drift"
			rpt.Points = append(rpt.Points, pr)
			continue
		}

		// Encode + send.
		frames, encErr := EncodeEffectivePoint(modelCode, ep)
		if encErr != nil {
			pr.Result = fmt.Sprintf("encode_error: %v", encErr)
			rpt.Points = append(rpt.Points, pr)
			_ = r.st.AppendApplyLog(ctx, uid, ep.ApsCode, ep.NativeValue, rbVal, pr.Result)
			continue
		}

		var sendErr error
		for _, frame := range frames {
			if sendErr = r.snd.Send(ctx, uid, frame); sendErr != nil {
				break
			}
		}
		if sendErr != nil {
			pr.Result = fmt.Sprintf("send_error: %v", sendErr)
			rpt.Points = append(rpt.Points, pr)
			_ = r.st.AppendApplyLog(ctx, uid, ep.ApsCode, ep.NativeValue, rbVal, pr.Result)
			continue
		}

		// Wait for the read-back pipeline to settle.
		select {
		case <-ctx.Done():
			pr.Result = "context_cancelled"
			rpt.Points = append(rpt.Points, pr)
			return rpt, ctx.Err()
		case <-time.After(r.opts.ReadSettle):
		}

		// Trigger a fresh read and wait again.
		r.rb.TriggerRead(uid)
		select {
		case <-ctx.Done():
			pr.Result = "context_cancelled"
			rpt.Points = append(rpt.Points, pr)
			return rpt, ctx.Err()
		case <-time.After(r.opts.ReadSettle):
		}

		// Read confirmation.
		newRB, newOK := r.rb.ReadbackNative(uid)
		if newOK {
			if v, ok2 := newRB[ep.ApsCode]; ok2 {
				pr.Readback = v
				if withinTolerance(ep.ApsCode, ep.NativeValue, v) {
					pr.Result = "applied"
				} else {
					pr.Result = "unconfirmed"
				}
			} else {
				pr.Result = "unconfirmed"
			}
		} else {
			pr.Result = "unconfirmed"
		}

		rpt.Points = append(rpt.Points, pr)
		_ = r.st.AppendApplyLog(ctx, uid, ep.ApsCode, ep.NativeValue, pr.Readback, pr.Result)
	}

	return rpt, nil
}

// ReconcileAll calls ReconcileUID for every inverter that has a model code
// registered.  Inverter UIDs are discovered by looking at the set of
// overlays plus all inverters known to modelOf.  The caller-supplied
// uidList determines the set of UIDs to reconcile.
//
// Passing nil uses a best-effort scan of the overlay table.
func (r *Reconciler) ReconcileAll(ctx context.Context) ([]ReconcileReport, error) {
	// Enumerate all UIDs with overlays stored.
	rows, err := r.st.db.QueryContext(ctx, `SELECT uid FROM gp_overlays`)
	if err != nil {
		return nil, fmt.Errorf("ReconcileAll: query overlays: %w", err)
	}
	seen := map[string]bool{}
	var uids []string
	add := func(uid string) {
		if uid != "" && !seen[uid] {
			seen[uid] = true
			uids = append(uids, uid)
		}
	}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return nil, fmt.Errorf("ReconcileAll: scan uid: %w", err)
		}
		add(uid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ReconcileAll: rows.Err: %w", err)
	}
	// Include every known inverter so a base change reaches inverters without
	// an overlay (ReconcileUID no-ops where there is no desired state).
	if r.KnownUIDs != nil {
		for _, uid := range r.KnownUIDs(ctx) {
			add(uid)
		}
	}

	var reports []ReconcileReport
	for _, uid := range uids {
		rpt, err := r.ReconcileUID(ctx, uid)
		if err != nil && ctx.Err() != nil {
			return reports, ctx.Err()
		}
		reports = append(reports, rpt)
	}
	return reports, nil
}

// IdentifyProfile scores each stored base profile against the supplied
// read-back map and returns the id of the best match together with its
// fractional match score (0.0–1.0).
//
// Scoring: fraction of the profile's capability-relevant, readable native
// points that match the reading within per-code tolerance.  Only codes
// present in both the profile and the reading contribute to the denominator.
// Returns ("", 0) if no profiles are stored or the reading is empty.
func (r *Reconciler) IdentifyProfile(reading map[string]float64, modelCode uint8) (id string, score float64) {
	ctx := context.Background()
	profiles, err := r.st.ListProfiles(ctx)
	if err != nil || len(profiles) == 0 {
		return "", 0
	}
	if len(reading) == 0 {
		return "", 0
	}

	var bestID string
	bestScore := -1.0 // sentinel so the first scorable profile always wins

	for _, p := range profiles {
		var matched, total int
		for _, entry := range p.Points {
			code := entry.Apply.ApsCode
			if !Writeable(modelCode, code) {
				continue
			}
			if isQS1APageACode(code) && isQS1Family(modelCode) {
				continue // not readable on QS1A
			}
			rbVal, ok := reading[code]
			if !ok {
				continue
			}
			total++
			if withinTolerance(code, entry.Native.Value, rbVal) {
				matched++
			}
		}
		if total == 0 {
			continue
		}
		s := float64(matched) / float64(total)
		if s > bestScore {
			bestScore = s
			bestID = p.ID
		}
	}
	if bestScore < 0 {
		return "", 0
	}
	return bestID, bestScore
}

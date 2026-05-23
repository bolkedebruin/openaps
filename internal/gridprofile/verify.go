package gridprofile

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bolke/inv-driver/codec"
)

// qs1aPageACodes is the set of APsystems 2-letter codes QS1A firmware does
// NOT return on a protection read-back.  QS1/QS1A answers the 0xDD page-A
// query with a 0xDB-tagged reply that the codec now decodes
// (codec.qs1ReadPageA), so the volt/freq trips, recovery/start times, and
// clearance times ARE read-back-verifiable.  No QS1A code is currently
// unreadable, so this set is empty; it stays as the seam for any code later
// found genuinely absent.
var qs1aPageACodes = map[string]bool{}

// isQS1APageACode returns true only for codes that QS1A firmware genuinely
// does not return on its protection read-back.  The page-A codes are now
// decoded from the 0xDB reply, so they are no longer in this set.
func isQS1APageACode(code string) bool {
	return qs1aPageACodes[code]
}

// VerifyStartup reads the current protection state of uid, identifies the
// running profile, and — when a desired state exists and AutoReassert is
// enabled — triggers reconciliation.
//
// The function is designed to be called once per inverter on first-seen,
// after the protection read pipeline has had a chance to deliver the initial
// pages.  Callers should call TriggerRead before VerifyStartup and wait for
// the read-back to arrive, or accept that the report may show no_readback.
func (r *Reconciler) VerifyStartup(ctx context.Context, uid string, modelCode uint8) (VerifyReport, error) {
	report := VerifyReport{UID: uid}

	// Read the current protection state.
	r.rb.TriggerRead(uid)

	reading, hasRB := r.rb.ReadbackNative(uid)
	if !hasRB {
		reading = map[string]float64{}
	}

	// Identify what profile is currently running.
	report.IdentifiedID, report.IdentifiedScore = r.IdentifyProfile(reading, modelCode)

	// Check whether a desired state (active base) exists.
	baseID, err := r.st.ActiveBase(ctx)
	if err == sql.ErrNoRows {
		// No desired state: report identified profile and baseline only.
		report.HasDesiredState = false
		return report, nil
	}
	if err != nil {
		report.Err = fmt.Errorf("VerifyStartup: ActiveBase: %w", err)
		return report, report.Err
	}
	_ = baseID
	report.HasDesiredState = true

	if !hasRB {
		// Desired state exists but the ingest layer has not delivered any
		// read-back yet (inverter not yet polled).
		report.Err = fmt.Errorf("VerifyStartup %q: no read-back available yet", uid)
		return report, report.Err
	}

	// Build desired effective profile.
	desired, err := r.buildDesired(ctx, uid, modelCode)
	if err != nil {
		report.Err = fmt.Errorf("VerifyStartup: buildDesired: %w", err)
		return report, report.Err
	}
	if desired == nil {
		report.HasDesiredState = false
		return report, nil
	}

	// Compare desired vs read-back per point.
	hasDrift := false
	for _, ep := range desired.Points {
		pr := PointResult{ApsCode: ep.ApsCode, Intended: ep.NativeValue}

		if isQS1APageACode(ep.ApsCode) && isQS1Family(modelCode) {
			pr.Result = "unknown"
			report.Points = append(report.Points, pr)
			continue
		}

		rbVal, ok := reading[ep.ApsCode]
		if !ok {
			pr.Result = "no_readback"
			report.Points = append(report.Points, pr)
			continue
		}
		pr.Readback = rbVal

		if withinTolerance(ep.ApsCode, ep.NativeValue, rbVal) {
			pr.Result = "in_sync"
		} else {
			pr.Result = "drift"
			hasDrift = true
		}
		report.Points = append(report.Points, pr)
	}

	if hasDrift && r.opts.AutoReassert {
		rpt, reconcileErr := r.ReconcileUID(ctx, uid)
		if reconcileErr == nil {
			report.Points = rpt.Points
			report.ReconciledOnStartup = true
		}
	}

	return report, nil
}

// isQS1Family is defined in capability.go and is package-internal; exposed
// here for clarity.  modelQS1 and modelQS1A are the codec constants.
var _ = codec.ModelQS1  // codec.ModelQS1 = 0x08
var _ = codec.ModelQS1A // codec.ModelQS1A = 0x18

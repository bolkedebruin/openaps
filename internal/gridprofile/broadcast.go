package gridprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/bolkedebruin/openaps/codec"
)

// broadcastProtoValidated controls whether broadcast protection apply is
// permitted at all.  While false the broadcast apply path FAILS CLOSED
// regardless of the operator -gridprofile-broadcast flag.  Flip this
// constant only after the broadcast protection opcode has been confirmed
// on-wire against live hardware (see GRID_PROFILE_BROADCAST_GROUNDTRUTH.md).
//
// The ecu-zb bus-manager carries an independent gate at the (cmd,sub)
// level that must be flipped in coordination with this constant.
//
// Validated on-wire 2026-05-23: DS3 (0xAB) against a captured
// iSetPvGrid broadcast, and QS1A (0x2C) by a direct apply-then-read test
// (CB 50.2->50.3->50.2 confirmed on both QS1A units). The SelectBase
// broadcast path stays opt-in via the operator -gridprofile-broadcast flag.
const broadcastProtoValidated = true

// Broadcaster delivers a single broadcast L2 frame to all enrolled inverters
// of a given family. Frame bytes are L2 only; ecu-zb's bus-mgr wraps the
// L1 envelope with SA=0.
type Broadcaster interface {
	Broadcast(ctx context.Context, frame []byte) error
}

// BroadcastReport summarises the outcome of one ApplyBaseBroadcast call.
type BroadcastReport struct {
	ProfileID string
	Points    []PointResult
	// Err holds any top-level error (e.g. no active base, store error).
	Err error
}

// effectivePointValue applies the same clamp + capability filter that
// buildDesired and EncodeEffectivePoint use.  It returns the clamped
// native value and whether the code is writable for modelCode.
//
// This is the single shared helper reused by buildDesired,
// EncodeEffectivePoint (via clamp), getEffective, and broadcast so that
// no path sends raw unclamped values.
func effectivePointValue(modelCode uint8, code string, entry PointEntry) (native float64, ok bool) {
	if !Writeable(modelCode, code) {
		return 0, false
	}
	v := entry.Native.Value
	if entry.Range != nil && entry.Range.Min <= entry.Range.Max {
		if v < entry.Range.Min {
			v = entry.Range.Min
		} else if v > entry.Range.Max {
			v = entry.Range.Max
		}
	}
	return v, true
}

// ApplyBaseBroadcast broadcasts the active base profile to all inverters of
// the given model families.  For each writable point in the profile it:
//
//  1. Applies the shared clamp + capability filter.
//  2. Encodes the broadcast L2 frame via codec.EncodeSetProtectionBroadcast.
//  3. Sends the frame via b.Broadcast (SA=0 addressed by ecu-zb).
//  4. Waits ReadSettle, then triggers a fresh read, waits again.
//  5. Reads back each affected inverter via rb.ReadbackNative and confirms
//     within per-code tolerance.
//
// The broadcast apply path FAILS CLOSED when broadcastProtoValidated is false,
// returning a clear error regardless of the operator flag.
//
// Points whose broadcast encoding is not supported (ErrUnsupportedBroadcastParam)
// are recorded in the report as "unsupported_broadcast".
//
// modelCodes lists the model families to target. At least one must be provided.
func (r *Reconciler) ApplyBaseBroadcast(ctx context.Context, b Broadcaster, modelCodes ...uint8) (BroadcastReport, error) {
	rpt := BroadcastReport{}

	// Fail-closed: broadcast protection is not yet on-wire validated.
	if !broadcastProtoValidated {
		err := errors.New("broadcast protection apply not yet on-wire-validated; broadcastProtoValidated=false")
		rpt.Err = err
		return rpt, err
	}

	if b == nil {
		return rpt, errors.New("ApplyBaseBroadcast: Broadcaster is nil")
	}
	if len(modelCodes) == 0 {
		return rpt, errors.New("ApplyBaseBroadcast: no model codes supplied")
	}

	baseID, err := r.st.ActiveBase(ctx)
	if err == sql.ErrNoRows {
		return rpt, errors.New("ApplyBaseBroadcast: no active base profile")
	}
	if err != nil {
		return rpt, fmt.Errorf("ApplyBaseBroadcast: ActiveBase: %w", err)
	}
	rpt.ProfileID = baseID

	base, err := r.st.GetProfile(ctx, baseID)
	if err != nil {
		return rpt, fmt.Errorf("ApplyBaseBroadcast: GetProfile %q: %w", baseID, err)
	}

	// Re-validate the stored profile JSON before sending anything.
	if raw, marshalErr := marshalProfile(base); marshalErr == nil {
		if valErr := ValidateProfile(raw); valErr != nil {
			return rpt, fmt.Errorf("ApplyBaseBroadcast: stored profile %q failed schema validation: %w", baseID, valErr)
		}
	}

	// Collect target UIDs from the live inverter inventory, not overlays.
	// Overlays are sparse; fleets without overlays would never be confirmed.
	uids, err := r.inventoryUIDs(ctx, modelCodes)
	if err != nil {
		return rpt, fmt.Errorf("ApplyBaseBroadcast: enumerate UIDs: %w", err)
	}

	for _, entry := range base.Points {
		code := entry.Apply.ApsCode
		if code == "" {
			continue
		}

		// Apply shared clamp + capability filter across all targeted model codes.
		// Use the first model code that accepts the point for encoding.
		var filteredValue float64
		var filteredMC uint8
		var anyWriteable bool
		for _, mc := range modelCodes {
			v, ok := effectivePointValue(mc, code, entry)
			if ok {
				filteredValue = v
				filteredMC = mc
				anyWriteable = true
				break
			}
		}
		if !anyWriteable {
			pr := PointResult{ApsCode: code, Intended: entry.Native.Value, Result: "unsupported"}
			rpt.Points = append(rpt.Points, pr)
			_ = r.st.AppendApplyLog(ctx, "", code, entry.Native.Value, 0, "unsupported")
			continue
		}

		result, fatalErr := r.broadcastPoint(ctx, b, uids, modelCodes, code, filteredValue, filteredMC)
		pr := PointResult{ApsCode: code, Intended: filteredValue, Result: result}
		_ = r.st.AppendApplyLog(ctx, "", code, filteredValue, 0, result)
		rpt.Points = append(rpt.Points, pr)
		if fatalErr != nil {
			rpt.Err = fatalErr
			return rpt, fatalErr
		}
		if result == "context_cancelled" {
			return rpt, ctx.Err()
		}
	}

	return rpt, nil
}

// inventoryUIDs returns the UIDs of all inverters in the store whose
// model_code is one of the requested model families.
func (r *Reconciler) inventoryUIDs(ctx context.Context, modelCodes []uint8) ([]string, error) {
	rows, err := r.st.db.QueryContext(ctx,
		`SELECT uid, model_code FROM inverters WHERE model_code IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("inventoryUIDs: query: %w", err)
	}
	defer rows.Close()

	mcSet := make(map[uint8]bool, len(modelCodes))
	for _, mc := range modelCodes {
		mcSet[mc] = true
	}

	var uids []string
	for rows.Next() {
		var uid string
		var mc int64
		if err := rows.Scan(&uid, &mc); err != nil {
			return nil, fmt.Errorf("inventoryUIDs: scan: %w", err)
		}
		if mcSet[uint8(mc)] {
			uids = append(uids, uid)
		}
	}
	return uids, rows.Err()
}

// marshalProfile serialises a Profile to JSON for schema re-validation.
func marshalProfile(p Profile) ([]byte, error) {
	return json.Marshal(p)
}

// broadcastPoint encodes + broadcasts one param. Returns (result string, fatal error).
// A fatal error is non-nil only for unexpected encode failures (fail-closed);
// unsupported/broadcast-unsupported/send errors are surfaced in the result string only.
// nativeValue is already clamp+capability-filtered by the caller.
func (r *Reconciler) broadcastPoint(
	ctx context.Context,
	b Broadcaster,
	uids []string,
	modelCodes []uint8,
	code string,
	nativeValue float64,
	_ uint8, // primaryMC reserved; per-mc iteration below selects the encoder
) (string, error) {
	var lastEncErr error
	for _, mc := range modelCodes {
		if !Writeable(mc, code) {
			continue
		}
		longName, ok := LongName(code)
		if !ok {
			continue
		}
		frames, encErr := codec.EncodeSetProtectionBroadcast(mc, longName, nativeValue)
		if errors.Is(encErr, codec.ErrUnsupportedBroadcastParam) ||
			errors.Is(encErr, codec.ErrUnsupportedProtectionParam) ||
			errors.Is(encErr, codec.ErrUnsupportedProtectionFamily) {
			lastEncErr = encErr
			continue
		}
		if encErr != nil {
			// Unexpected encode error — fail-closed.
			return "encode_error", fmt.Errorf("ApplyBaseBroadcast: encode %q model 0x%02X: %w", code, mc, encErr)
		}

		// Send each frame via broadcast.
		for _, frame := range frames {
			if sendErr := b.Broadcast(ctx, frame); sendErr != nil {
				return fmt.Sprintf("broadcast_error: %v", sendErr), nil
			}
		}

		// Apply-then-read confirmation.
		result := r.waitAndConfirmBroadcast(ctx, uids, modelCodes, code, nativeValue)
		return result, nil
	}

	// No model family could broadcast this code.
	if errors.Is(lastEncErr, codec.ErrUnsupportedBroadcastParam) {
		return "unsupported_broadcast", nil
	}
	if lastEncErr != nil {
		return fmt.Sprintf("unsupported: %v", lastEncErr), nil
	}
	return "unsupported", nil
}

// waitAndConfirmBroadcast waits for the read-back pipeline to settle, triggers
// fresh reads, waits again, then scores confirmation across all targeted UIDs.
func (r *Reconciler) waitAndConfirmBroadcast(
	ctx context.Context,
	uids []string,
	modelCodes []uint8,
	code string,
	nativeValue float64,
) string {
	select {
	case <-ctx.Done():
		return "context_cancelled"
	case <-time.After(r.opts.ReadSettle):
	}

	for _, uid := range uids {
		r.rb.TriggerRead(uid)
	}

	select {
	case <-ctx.Done():
		return "context_cancelled"
	case <-time.After(r.opts.ReadSettle):
	}

	return r.confirmBroadcastPoint(uids, modelCodes, code, nativeValue)
}

// confirmBroadcastPoint scores the read-back confirmation for a broadcast point
// across all UIDs targeted by the broadcast.
func (r *Reconciler) confirmBroadcastPoint(
	uids []string,
	modelCodes []uint8,
	code string,
	nativeValue float64,
) string {
	confirmed, total := 0, 0
	for _, uid := range uids {
		mc, ok := r.modelOf(uid)
		if !ok {
			continue
		}
		targeted := false
		for _, tmc := range modelCodes {
			if mc == tmc {
				targeted = true
				break
			}
		}
		if !targeted {
			continue
		}
		rb, hasRB := r.rb.ReadbackNative(uid)
		if !hasRB {
			continue
		}
		rbVal, ok := rb[code]
		if !ok {
			continue
		}
		total++
		if math.Abs(nativeValue-rbVal) <= codeTolerance(code) {
			confirmed++
		}
	}

	if total == 0 {
		return "no_readback"
	}
	if confirmed == total {
		return "applied"
	}
	return fmt.Sprintf("unconfirmed (%d/%d confirmed)", confirmed, total)
}

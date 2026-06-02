package server

import (
	"math"
	"sync"
	"time"

	"github.com/bolke/ecu-sunspec/internal/source"
)

// WriteTracker tracks the lifecycle of writes that go through ZigBee
// dispatch, so SunSpec models with an `AdptCtlRslt` field (Model 711, and
// future 700-series writable models) can publish IN_PROGRESS / COMPLETED /
// FAILED faithfully — instead of hardcoding COMPLETED.
//
// Lifecycle:
//
//  1. Caller (e.g. FreqDroopWriter.Apply) queues writes via Writer.SetProtectionParam,
//     then calls Track(uid, modelID, expected).
//  2. The next refresh cycle calls Reconcile(snap) which compares expected
//     values against snap.Protection[uid]. Match → COMPLETED. Past
//     deadline without match → FAILED.
//  3. Get(uid, modelID) returns the current Rslt for emit. Default for an
//     un-tracked (uid, modelID) is COMPLETED — i.e. nothing in flight.
//
// The tracker is goroutine-safe.
type WriteTracker struct {
	mu      sync.Mutex
	entries map[trackerKey]*trackerEntry
}

type trackerKey struct {
	uid   uint8
	model uint16
}

type trackerEntry struct {
	// reqCounter increments each time a write is tracked for this key.
	// Clients can use this to correlate "the request I just made".
	reqCounter uint16
	// rslt is the last known async result enum (model-specific values;
	// caller maps 0/1/2 → IN_PROGRESS/COMPLETED/FAILED per spec).
	rslt uint16
	// expected holds the field values we expect to see in protection_parameters60code
	// for this UID once the write lands. Keyed by 2-letter code (so the
	// reconcile path can read snap.Protection[uid] without long-name lookup).
	expected map[string]float64
	// deadline is when we give up on IN_PROGRESS and flip to FAILED.
	deadline time.Time
}

// Result enum constants — use the same numeric values that SunSpec's
// AdptCtlRslt / AdptCrvRslt enums use across the 700-series.
const (
	WriteRsltInProgress uint16 = 0
	WriteRsltCompleted  uint16 = 1
	WriteRsltFailed     uint16 = 2

	// Default deadline for write reconciliation. Three ZigBee cycles
	// (~5 min default poll × 3) is generous; firmware typically reports
	// back within one cycle when accepting, never when rejecting.
	defaultWriteDeadline = 15 * time.Minute

	// Float comparison tolerance for matching expected vs. observed values.
	// Inverter firmware sometimes rounds to 0.1 Hz / 0.01 V resolution.
	writeMatchTolerance = 0.05
)

// NewWriteTracker returns a fresh tracker.
func NewWriteTracker() *WriteTracker {
	return &WriteTracker{entries: make(map[trackerKey]*trackerEntry)}
}

// Track records that a write was issued for (uid, modelID) and is expected
// to settle to the supplied 2-letter-coded values. expected may be nil if
// the caller doesn't have a way to verify (e.g. parameters that don't
// surface in protection_parameters60code).
func (wt *WriteTracker) Track(uid uint8, modelID uint16, expected map[string]float64) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	key := trackerKey{uid: uid, model: modelID}
	e, ok := wt.entries[key]
	if !ok {
		e = &trackerEntry{}
		wt.entries[key] = e
	}
	e.reqCounter++
	e.rslt = WriteRsltInProgress
	e.expected = expected
	e.deadline = time.Now().Add(defaultWriteDeadline)
}

// Get returns the current (rslt, reqCounter) for (uid, modelID). If no
// entry exists, returns (COMPLETED, 0) — the right default for "nothing
// in flight, server is fine".
func (wt *WriteTracker) Get(uid uint8, modelID uint16) (uint16, uint16) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if e, ok := wt.entries[trackerKey{uid: uid, model: modelID}]; ok {
		return e.rslt, e.reqCounter
	}
	return WriteRsltCompleted, 0
}

// Reconcile walks all pending entries and advances Rslt based on the new
// snapshot. Match (within tolerance) → COMPLETED. Past deadline without
// match → FAILED. Otherwise stays IN_PROGRESS for the next refresh.
//
// Pending entries that have transitioned past IN_PROGRESS are kept in
// the map so a Get within the same emit cycle returns the correct value.
// Callers that want to clean up after one full publish cycle can call
// PruneSettled.
func (wt *WriteTracker) Reconcile(snap source.Snapshot) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	now := time.Now()
	for key, e := range wt.entries {
		if e.rslt != WriteRsltInProgress {
			continue
		}
		// Map unit ID → inverter UID via snap.Inverters order.
		uid, ok := unitIDToInverterUID(snap, key.uid)
		if !ok {
			// Inverter has dropped from the snapshot. Conservative: fail.
			if now.After(e.deadline) {
				e.rslt = WriteRsltFailed
			}
			continue
		}
		prot := snap.Protection[uid]
		if matchesAll(prot, e.expected) {
			e.rslt = WriteRsltCompleted
			continue
		}
		if now.After(e.deadline) {
			e.rslt = WriteRsltFailed
		}
	}
}

// PruneSettled drops entries that have been in COMPLETED or FAILED state
// for at least the given grace period since the deadline. Called by the
// server after each refresh to keep the map bounded.
func (wt *WriteTracker) PruneSettled(grace time.Duration) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	now := time.Now()
	for key, e := range wt.entries {
		if e.rslt == WriteRsltInProgress {
			continue
		}
		if now.After(e.deadline.Add(grace)) {
			delete(wt.entries, key)
		}
	}
}

// matchesAll returns true if every (code, expected_value) in want matches
// the corresponding field in prot within writeMatchTolerance. A code that
// the inverter hasn't reported yet (Has[code] == false) counts as no-match.
func matchesAll(prot source.ProtectionParams, want map[string]float64) bool {
	if len(want) == 0 {
		return false // no expectations means we can't conclude COMPLETED
	}
	for code, expected := range want {
		actual, ok := protGet(prot, code)
		if !ok {
			return false
		}
		if math.Abs(actual-expected) > writeMatchTolerance {
			return false
		}
	}
	return true
}

// protGet returns the float value for a 2-letter code from prot, plus
// whether the inverter has populated it (via prot.Has).
func protGet(prot source.ProtectionParams, code string) (float64, bool) {
	if !prot.Has[code] {
		return 0, false
	}
	switch code {
	case "AB":
		return prot.AvgOV, true
	case "AC":
		return prot.UVStg2, true
	case "AD":
		return prot.OVStg2, true
	case "AE":
		return prot.UFSlow, true
	case "AF":
		return prot.OFSlow, true
	case "AG":
		return prot.ReconnectS, true
	case "AH":
		return prot.VWinLow, true
	case "AI":
		return prot.VWinHi, true
	case "AJ":
		return prot.UFFast, true
	case "AK":
		return prot.OFFast, true
	case "AQ":
		return prot.UVFast, true
	case "AS":
		return prot.StartS, true
	case "AY":
		return prot.OVStg3, true
	case "BB":
		return prot.UV2ClrS, true
	case "BC":
		return prot.OV2ClrS, true
	case "BD":
		return prot.UV3ClrS, true
	case "BE":
		return prot.OV3ClrS, true
	case "BH":
		return prot.UF1ClrS, true
	case "BI":
		return prot.OF1ClrS, true
	case "BJ":
		return prot.UF2ClrS, true
	case "BK":
		return prot.OF2ClrS, true
	case "BN":
		return prot.ReconnVLow, true
	case "BO":
		return prot.ReconnVHi, true
	case "BP":
		return prot.ReconnFLow, true
	case "BQ":
		return prot.ReconnFHi, true
	case "CC":
		return prot.OFDroopEnd, true
	case "CV":
		return prot.OFDroopMode, true
	case "DC":
		return prot.OFDroopStart, true
	case "DD":
		return prot.OFDroopSlope, true
	}
	return 0, false
}

// unitIDToInverterUID maps a Modbus unit ID to the inverter UID for
// the snapshot's per-inverter banks (uid 2..N+1 → snap.Inverters[uid-2]).
// uid 1 is the aggregate; tracking is only meaningful per-inverter, so
// we return false for aggregate writes.
func unitIDToInverterUID(snap source.Snapshot, unitID uint8) (string, bool) {
	if unitID < 2 {
		return "", false
	}
	idx := int(unitID) - 2
	if idx >= len(snap.Inverters) {
		return "", false
	}
	return snap.Inverters[idx].UID, true
}

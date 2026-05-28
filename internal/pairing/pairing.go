// Package pairing implements the inv-driver side of OTA inverter pairing:
// the Add/Replace and FleetRekey state machines that drive ecu-zb radio
// primitives (PairingCmd) and persist the result. Policy lives here; ecu-zb
// holds none.
//
// Layering: the controller (ecu-web) sends an Envelope.pairing_req over the
// UDS; the IPC server (controller-gated, exactly like grid-profile) calls
// Handle. start/abort/get_status return immediately; the actual flow runs on
// a background goroutine and reports progress via the in-memory
// PairingStatus (polled via get_status). Only milestones go to the events
// log.
package pairing

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/bolke/inv-driver/internal/buslock"
	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
)

// serialRe validates a 12-digit decimal inverter serial (the label id; the
// 6-byte BCD radio address packs 2 decimal digits per byte). All-decimal so
// the %02x round-trip back to the same string holds.
var serialRe = regexp.MustCompile(`^[0-9]{12}$`)

// uidRe validates the 12-hex-char inverter uid used as the store key. A
// serial (12 decimal digits) is also a valid uid string.
var uidRe = regexp.MustCompile(`^[0-9A-Fa-f]{12}$`)

// lockOwner is the buslock owner label for pairing operations.
const lockOwner = "pairing"

// Default sweep / dwell parameters (contract §Channel 1 server defaults).
const (
	defaultChanLo  uint32 = 11
	defaultChanHi  uint32 = 26
	defaultDwellMs uint32 = 2000
	// fastScanWindowMs is the report-id collection window for a fast scan
	// (single channel, no sweep).
	fastScanWindowMs uint32 = 3000
	// maxDwellMs caps the per-channel report-id window so a scan cannot park
	// the radio off-PAN (away from telemetry) for an absurd duration.
	maxDwellMs uint32 = 10000
)

// clampScanWindow bounds a requested dwell/window: a zero (unset) request
// uses the contract default, and any value is capped at maxDwellMs so a scan
// cannot park the radio off-PAN for an absurd duration.
func clampScanWindow(dwell uint32) uint32 {
	if dwell == 0 {
		return defaultDwellMs
	}
	if dwell > maxDwellMs {
		return maxDwellMs
	}
	return dwell
}

// EventSink records a pairing milestone to the audit log. uid may be empty
// for fleet-level milestones. by is the originating controller backend.
type EventSink interface {
	AppendEvent(ctx context.Context, tsMs int64, inverterUID, kind, severity, by, detail string) error
}

// ProfileInheritor moves the grid-profile overlay (Local Site profile) from
// a dead inverter to its replacement during a Replace op. It is the
// inv-driver-scoped part of "inherit config": the grid profile + per-inverter
// overlay. Power cap is NOT inherited (a Replace installs new hardware whose
// cap may legitimately differ); the status message prompts the operator to
// re-apply it if needed. The ECU has no array-slot/layout concept, so the
// operator label is the position identity carried over — see NameInheritor.
// A nil ProfileInheritor skips the grid-profile step (logged, non-fatal).
type ProfileInheritor interface {
	// InheritProfile copies any persisted overlay from oldUID to newUID and
	// triggers a reconcile of newUID against the active base. Returns a
	// human-readable summary for the status message, or an error.
	InheritProfile(ctx context.Context, oldUID, newUID string) (summary string, err error)
}

// NameInheritor moves the operator label (Settings.inverter_names entry) from
// a dead inverter to its replacement during Replace. The ECU has no array
// slot/layout, so the friendly name is the per-position identity an operator
// recognises ("Roof South #3") and is the right thing to carry to the new
// serial. A nil NameInheritor skips it (logged, non-fatal).
type NameInheritor interface {
	// InheritName moves any operator label from oldUID to newUID. Returns
	// whether a label was moved, or an error.
	InheritName(ctx context.Context, oldUID, newUID string) (moved bool, err error)
}

// Settings reads/writes the pan_override field. FleetRekey persists the new
// PAN here and NEVER touches ecu_eth0_mac.conf (macapp race).
type Settings interface {
	PANOverride() string
	SetPANOverride(ctx context.Context, panHex string) error
}

// Manager owns the pairing state machines, the single-op lock, the
// in-memory status, and the correlation transport. One per process.
type Manager struct {
	Store     *store.Store
	Transport *Transport
	// Lock is the shared bus guard. MUST be the same *buslock.Lock the
	// grid-profile broadcast path uses so pairing and broadcast are mutually
	// exclusive. A nil lock disables mutual exclusion (test-only).
	Lock *buslock.Lock
	// Events records milestones; nil disables audit logging.
	Events EventSink
	// Profiles performs Replace config inheritance; may be nil.
	Profiles ProfileInheritor
	// Names moves the operator label from the dead unit to its replacement
	// during Replace ("inherit slot"); may be nil.
	Names NameInheritor
	// Settings persists the rekey PAN via pan_override; required for rekey.
	Settings Settings
	// CurrentChannel returns the radio channel to fall back to when a
	// request omits it (0). Required for scan/rekey; nil → channel 0 used
	// and the op fails fast with a clear error.
	CurrentChannel func() uint32
	// Parent is the cancellation root for running ops; nil → Background.
	Parent context.Context
	// CommitSettle overrides the post-0x22-commit quiet window. Zero uses
	// the production default (commitSettle). Tests set a short value.
	CommitSettle time.Duration

	status statusTracker

	// runMu guards the single in-flight op's lifecycle handle (cancel +
	// done) so abort and a finishing op don't race.
	runMu  sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// Handle dispatches a PairingRequest. start/abort return immediately;
// get_status returns the live snapshot. by attributes any milestone events.
// Always returns a non-nil response.
func (m *Manager) Handle(ctx context.Context, by string, req *wire.PairingRequest) *wire.PairingResponse {
	if req == nil {
		return errResp("nil PairingRequest")
	}
	switch req.GetOp().(type) {
	case *wire.PairingRequest_Scan:
		return m.startScan(by, req.GetScan())
	case *wire.PairingRequest_AddById:
		return m.startAdd(by, req.GetAddById())
	case *wire.PairingRequest_Replace:
		return m.startReplace(by, req.GetReplace())
	case *wire.PairingRequest_FleetRekey:
		return m.startRekey(by, req.GetFleetRekey())
	case *wire.PairingRequest_Abort:
		return m.abort()
	case *wire.PairingRequest_GetStatus:
		return m.getStatus()
	default:
		return errResp(fmt.Sprintf("unknown PairingRequest op %T", req.GetOp()))
	}
}

// getStatus returns the live in-memory PairingStatus as JSON.
func (m *Manager) getStatus() *wire.PairingResponse {
	raw, err := m.status.snapshotJSON()
	if err != nil {
		return errResp(fmt.Sprintf("get_status: marshal: %v", err))
	}
	return &wire.PairingResponse{Ok: true, StatusJson: raw}
}

// abort cancels the running op (Safe-Abort). It is a no-op (still ok) when
// nothing is running.
func (m *Manager) abort() *wire.PairingResponse {
	m.runMu.Lock()
	cancel := m.cancel
	m.runMu.Unlock()
	if cancel == nil {
		return &wire.PairingResponse{Ok: true, Error: "no pairing op running"}
	}
	cancel()
	raw, _ := m.status.snapshotJSON()
	return &wire.PairingResponse{Ok: true, StatusJson: raw}
}

// startOp acquires the bus lock, resets status, and launches fn on a
// background goroutine. It returns the immediate start response (with the
// initial status snapshot) or an error if the lock is held / a config gate
// fails. fn owns lock release + the milestone/finish transitions.
func (m *Manager) startOp(by, op, stage string, total int, fn func(ctx context.Context)) *wire.PairingResponse {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	if m.done != nil {
		select {
		case <-m.done:
			// Previous op finished; fall through and start a new one.
		default:
			return errResp("a pairing op is already running")
		}
	}

	if m.Lock != nil {
		if ok, owner := m.Lock.TryAcquire(lockOwner); !ok {
			return errResp(fmt.Sprintf("bus busy: held by %q (pairing and grid-profile broadcast are mutually exclusive)", owner))
		}
	}

	parent := m.Parent
	if parent == nil {
		parent = context.Background()
	}
	opCtx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	m.cancel = cancel
	m.done = done

	m.status.begin(op, stage, total, by)

	go func() {
		defer func() {
			if m.Lock != nil {
				m.Lock.Release()
			}
			cancel()
			close(done)
		}()
		fn(opCtx)
	}()

	raw, _ := m.status.snapshotJSON()
	return &wire.PairingResponse{Ok: true, StatusJson: raw}
}

// milestone records a pairing milestone to the audit log (best-effort).
func (m *Manager) milestone(ctx context.Context, uid, kind, severity, detail string) {
	if m.Events == nil {
		return
	}
	_ = m.Events.AppendEvent(ctx, time.Now().UnixMilli(), uid, kind, severity, m.status.attribution(), detail)
}

// settleDur is the post-commit quiet window, overridable for tests.
func (m *Manager) settleDur() time.Duration {
	if m.CommitSettle > 0 {
		return m.CommitSettle
	}
	return commitSettle
}

// ZigBee 2.4 GHz channel bounds (IEEE 802.15.4 channels 11-26). A channel
// outside this range must never reach the wire (the byte() casts downstream
// would truncate it).
const (
	minChannel uint32 = 11
	maxChannel uint32 = 26
)

// channelOrCurrent resolves a request channel (0 → current radio channel)
// and validates the result is a usable 2.4 GHz channel (11-26). It returns
// an error rather than letting an out-of-range value reach a wire primitive
// (where a uint32→byte cast would silently truncate it).
func (m *Manager) channelOrCurrent(reqChannel uint32) (uint32, error) {
	ch := reqChannel
	if ch == 0 {
		if m.CurrentChannel != nil {
			ch = m.CurrentChannel()
		}
	}
	if ch < minChannel || ch > maxChannel {
		return 0, fmt.Errorf("channel %d out of range (want %d-%d)", ch, minChannel, maxChannel)
	}
	return ch, nil
}

// abortedOr returns true (and records the aborted milestone+status) when ctx
// is cancelled, so flow steps can early-return cleanly.
func (m *Manager) abortedOr(ctx context.Context) bool {
	if ctx.Err() == nil {
		return false
	}
	m.status.finishAborted()
	m.milestone(context.Background(), "", "pairing_aborted", "warn", "")
	return true
}

// fail records an error milestone + status and is the common terminal for
// any flow step that returns an error.
func (m *Manager) fail(stepErr error) {
	if errors.Is(stepErr, context.Canceled) {
		m.status.finishAborted()
		m.milestone(context.Background(), "", "pairing_aborted", "warn", "")
		return
	}
	m.status.finishError(stepErr.Error())
	m.milestone(context.Background(), "", "pairing_error", "error", stepErr.Error())
}

// errResp constructs a failure PairingResponse.
func errResp(msg string) *wire.PairingResponse {
	return &wire.PairingResponse{Ok: false, Error: msg}
}

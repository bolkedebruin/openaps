package pairing

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bolke/inv-driver/wire"
)

// commitRounds is the number of 0x22 broadcast commits per main.exe
// (send22order sends it ×3). ecu-zb may itself repeat internally; we issue
// the commit primitive commitRounds times for parity.
const commitRounds = 3

// startRekey moves the whole fleet to a new PAN. PREP snapshots the old PAN,
// rendezvous on 0xFFFF, prime each known inverter with the new PAN, commit
// ×3, re-arm the module on the new PAN (persisted via pan_override),
// re-query short addresses, VERIFY, DONE. On commit/verify failure the old
// PAN is restored on the module and the op reports an error.
func (m *Manager) startRekey(by string, req *wire.FleetRekey) *wire.PairingResponse {
	if m.Transport == nil {
		return errResp("rekey: transport not configured")
	}
	if m.Settings == nil {
		return errResp("rekey: settings (pan_override) not configured")
	}
	newPan, err := parsePAN(req.GetNewPan())
	if err != nil {
		return errResp("rekey: " + err.Error())
	}
	// The current (old) PAN is whatever ecu-zb is bonded to — queried from the
	// radio owner, not re-derived from settings.
	rootCtx := m.Parent
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	qctx, qcancel := context.WithTimeout(rootCtx, defaultCmdTimeout)
	oldPan, err := m.Transport.getModulePan(qctx)
	qcancel()
	if err != nil {
		return errResp("rekey: current/operating PAN unknown: " + err.Error())
	}
	if newPan == oldPan {
		return errResp(fmt.Sprintf("rekey: new PAN 0x%04X equals current PAN", newPan))
	}
	ch, err := m.channelOrCurrent(req.GetChannel())
	if err != nil {
		return errResp("rekey: " + err.Error())
	}

	// Snapshot the fleet at start time so a unit that drops mid-rekey is
	// still primed/verified against the list we committed to.
	serials, err := m.fleetSerials()
	if err != nil {
		return errResp("rekey: " + err.Error())
	}
	if len(serials) == 0 {
		return errResp("rekey: no inverters in inventory to migrate")
	}

	return m.startOp(by, OpRekey, StageRekey, len(serials), func(ctx context.Context) {
		m.runRekey(ctx, serials, oldPan, newPan, ch)
	})
}

// runRekey is the rekey body (PREP→prime→commit→rearm→rebind→verify).
func (m *Manager) runRekey(ctx context.Context, serials []string, oldPan, newPan uint32, ch uint32) {
	m.status.setStage(StageRekey, fmt.Sprintf("prep: 0x%04X -> 0x%04X ch=%d", oldPan, newPan, ch))
	for _, s := range serials {
		m.status.upsertInverter(PerInverter{Serial: s, State: "pending"})
	}

	// RENDEZVOUS: park module on 0xFFFF.
	m.status.setStage(StageRekey, "rendezvous on PAN 0xFFFF")
	if err := m.Transport.setModulePan(ctx, 0xFFFF, ch); err != nil {
		m.rekeyRollback(oldPan, ch, fmt.Errorf("module to 0xFFFF: %w", err))
		return
	}

	// PRIME each inverter with the new PAN.
	for i, s := range serials {
		if ctx.Err() != nil {
			// Aborted before commit: the fleet has NOT moved. Restore the
			// module to the old PAN so telemetry resumes, then report.
			m.rekeyRollback(oldPan, ch, ctx.Err())
			return
		}
		m.status.update(func(st *PairingStatus) { st.CurrentSerial = s; st.Substep = "prime " + s })
		m.status.setInverterState(s, "priming")
		if err := m.Transport.primeInv(ctx, s, newPan, ch); err != nil {
			m.rekeyRollback(oldPan, ch, fmt.Errorf("prime %s: %w", s, err))
			return
		}
		m.status.setInverterState(s, "primed")
		m.status.update(func(st *PairingStatus) { st.Done = i + 1 })
	}

	// COMMIT ×3 (broadcast).
	m.status.update(func(st *PairingStatus) { st.Done = 0; st.Substep = "commit broadcast" })
	for r := 0; r < commitRounds; r++ {
		if ctx.Err() != nil {
			m.rekeyRollback(oldPan, ch, ctx.Err())
			return
		}
		if err := m.Transport.commitPan(ctx, newPan, ch); err != nil {
			m.rekeyRollback(oldPan, ch, fmt.Errorf("commit round %d: %w", r+1, err))
			return
		}
	}
	// Post-commit settle.
	select {
	case <-ctx.Done():
		m.rekeyRollback(oldPan, ch, ctx.Err())
		return
	case <-time.After(m.settleDur()):
	}

	// REARM: module to the new PAN, persist via pan_override.
	m.status.setStage(StageRekey, "rearm module on new PAN")
	if err := m.Transport.setModulePan(ctx, newPan, ch); err != nil {
		m.rekeyRollback(oldPan, ch, fmt.Errorf("rearm module on new PAN: %w", err))
		return
	}
	if err := m.Settings.SetPANOverride(ctx, fmt.Sprintf("%04X", newPan)); err != nil {
		m.rekeyRollback(oldPan, ch, fmt.Errorf("persist pan_override: %w", err))
		return
	}
	m.milestone(ctx, "", "rekey_committed", "info", fmt.Sprintf("0x%04X -> 0x%04X ch=%d", oldPan, newPan, ch))

	// REBIND + VERIFY: re-query each short address on the new PAN.
	m.status.setStage(StageRekey, "verify")
	verified := 0
	for i, s := range serials {
		if ctx.Err() != nil {
			// Already committed + persisted; abort here means partial verify,
			// not a rollback — the fleet is on the new PAN now.
			break
		}
		m.status.update(func(st *PairingStatus) { st.CurrentSerial = s; st.Substep = "verify " + s; st.Done = i })
		sa, err := m.Transport.getShortAddr(ctx, s)
		if err != nil || sa == 0 {
			m.status.setInverterState(s, "unverified")
			continue
		}
		if m.Store != nil {
			_ = m.Store.SetInverterShortAddr(ctx, s, sa)
			_ = m.Store.SetInverterPairingState(ctx, s, "bound", 0)
		}
		m.status.upsertInverter(PerInverter{Serial: s, ShortAddr: uint32(sa), State: "verified"})
		verified++
	}

	if verified < len(serials) {
		// Commit already persisted the new PAN; we do NOT roll back (the
		// fleet has moved). Report the partial result so the operator can
		// re-run verify / investigate the stragglers.
		m.status.update(func(st *PairingStatus) {
			st.Stage = StageError
			st.Error = fmt.Sprintf("rekey committed to 0x%04X but only %d/%d inverters verified", newPan, verified, len(serials))
			st.Done = verified
		})
		m.milestone(context.Background(), "", "pairing_error", "warn",
			fmt.Sprintf("rekey partial verify %d/%d", verified, len(serials)))
		return
	}
	m.status.finishDone(fmt.Sprintf("fleet rekeyed to 0x%04X (%d/%d verified)", newPan, verified, len(serials)))
}

// rekeyRollback restores the module to the old PAN and records the error.
// Used on any pre-commit failure (the fleet has NOT moved yet, so returning
// the module to the old PAN keeps telemetry flowing).
func (m *Manager) rekeyRollback(oldPan, ch uint32, cause error) {
	rbCtx, cancel := context.WithTimeout(context.Background(), defaultCmdTimeout)
	defer cancel()
	rbErr := m.Transport.setModulePan(rbCtx, oldPan, ch)
	msg := cause.Error()
	if rbErr != nil {
		msg = fmt.Sprintf("%v; rollback to PAN 0x%04X also failed: %v", cause, oldPan, rbErr)
	} else {
		msg = fmt.Sprintf("%v; rolled back module to PAN 0x%04X", cause, oldPan)
	}
	m.status.finishError(msg)
	m.milestone(context.Background(), "", "pairing_error", "error", msg)
}

// fleetSerials returns the 12-digit decimal serials of every inverter in
// inventory (uid is the serial as a 12-char string).
func (m *Manager) fleetSerials() ([]string, error) {
	if m.Store == nil {
		return nil, fmt.Errorf("store not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	refs, err := m.Store.ListInvertersForPoll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list inverters: %w", err)
	}
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if serialRe.MatchString(r.UID) {
			out = append(out, r.UID)
		}
	}
	return out, nil
}

// parsePAN parses a 1-4 hex digit PAN string into a uint16-range value.
func parsePAN(s string) (uint32, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if s == "" {
		return 0, fmt.Errorf("empty PAN")
	}
	if len(s) > 4 {
		return 0, fmt.Errorf("PAN %q has more than 4 hex digits", s)
	}
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, fmt.Errorf("PAN %q not valid hex: %w", s, err)
	}
	return uint32(v), nil
}

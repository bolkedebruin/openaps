package pairing

import (
	"context"
	"fmt"

	"github.com/bolke/inv-driver/wire"
)

// startChangeChannel moves the whole fleet to a new RF channel while keeping
// the PAN unchanged. Unlike a rekey (which uses the rendezvous/prime/commit
// primitives to change the PAN), a channel migration uses the directed
// setInvPan (0x0F) primitive to hop each inverter to the new channel on the
// CURRENT PAN, then moves the module (setModulePan, 0x05) so ecu-zb adopts
// the new opChannel, and finally re-queries short addresses on the new
// channel.
//
// inv-driver does NOT validate the channel range — ecu-zb's checkChannel is
// the authority and rejects an out-of-range value at the wire, the error
// propagating back via PairingCmdResult.
func (m *Manager) startChangeChannel(by string, req *wire.FleetChangeChannel) *wire.PairingResponse {
	if m.Transport == nil {
		return errResp("change_channel: transport not configured")
	}
	if m.Settings == nil {
		return errResp("change_channel: settings (channel) not configured")
	}
	newChannel := req.GetChannel()
	if newChannel == 0 {
		return errResp("change_channel: target channel required")
	}

	// The current PAN AND channel are whatever ecu-zb is bonded to — queried
	// from the radio owner, not re-derived from settings. This keeps the
	// rollback target (the old channel) honest even if settings drifted.
	rootCtx := m.Parent
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	qctx, qcancel := context.WithTimeout(rootCtx, defaultCmdTimeout)
	currentPan, currentChan, err := m.Transport.getModuleConfig(qctx)
	qcancel()
	if err != nil {
		return errResp("change_channel: current/operating PAN unknown: " + err.Error())
	}

	// Old channel comes from the radio owner. Fall back to CurrentChannel only
	// if the backend didn't report one (older backend). No range check here —
	// ecu-zb owns that.
	oldChannel := currentChan
	if oldChannel == 0 && m.CurrentChannel != nil {
		oldChannel = m.CurrentChannel()
	}
	if oldChannel == 0 {
		return errResp("change_channel: current channel unknown")
	}
	if newChannel == oldChannel {
		return errResp(fmt.Sprintf("change_channel: new channel %d equals current channel", newChannel))
	}

	// Snapshot the fleet at start time so a unit that drops mid-migration is
	// still moved/verified against the list we committed to.
	serials, err := m.fleetSerials()
	if err != nil {
		return errResp("change_channel: " + err.Error())
	}
	if len(serials) == 0 {
		return errResp("change_channel: no inverters in inventory to migrate")
	}

	return m.startOp(by, OpChannel, StageChannel, len(serials), func(ctx context.Context) {
		m.runChangeChannel(ctx, serials, currentPan, oldChannel, newChannel)
	})
}

// runChangeChannel is the channel-migration body: directed channel hop per
// inverter (0x0F on the current PAN) → move the module to the new channel →
// re-query short addresses on the new channel → persist. On any failure
// BEFORE the module move completes, the module is restored to the old
// channel so telemetry resumes — reversible like rekey.
func (m *Manager) runChangeChannel(ctx context.Context, serials []string, pan, oldChannel, newChannel uint32) {
	m.status.setStage(StageChannel, fmt.Sprintf("prep: ch %d -> %d (PAN 0x%04X)", oldChannel, newChannel, pan))
	m.status.update(func(st *PairingStatus) {
		st.Sweep = Sweep{Chan: newChannel, ChanLo: oldChannel, ChanHi: newChannel}
	})
	for _, s := range serials {
		m.status.upsertInverter(PerInverter{Serial: s, State: "pending"})
	}
	m.milestone(ctx, "", "channel_change_started", "info",
		fmt.Sprintf("ch %d -> %d (PAN 0x%04X)", oldChannel, newChannel, pan))

	// HOP each inverter to the new channel via the directed 0x0F primitive,
	// sent while the module is still on the current channel so the inverter
	// is reachable. PAN is kept unchanged.
	for i, s := range serials {
		if ctx.Err() != nil {
			// Aborted mid-loop: any inverter already hopped this round is now
			// on the NEW channel and unreachable from the old one — a 0x0F hop
			// takes effect the instant the inverter receives it, so the
			// module-only rollback below CANNOT un-hop them. (Only an abort
			// before the first hop leaves the whole fleet still on the old
			// channel.) Restore the module to the old channel so the
			// not-yet-hopped units keep reporting; recover the split fleet by
			// re-running the change-channel toward the new channel.
			m.changeChannelRollback(pan, oldChannel, newChannel, ctx.Err())
			return
		}
		m.status.update(func(st *PairingStatus) { st.CurrentSerial = s; st.Substep = "hop " + s })
		m.status.setInverterState(s, "hopping")
		if err := m.Transport.setInvPan(ctx, s, pan, newChannel); err != nil {
			m.changeChannelRollback(pan, oldChannel, newChannel, fmt.Errorf("set_inv_pan %s: %w", s, err))
			return
		}
		m.status.setInverterState(s, "hopped")
		m.status.update(func(st *PairingStatus) { st.Done = i + 1 })
	}

	// MOVE the module to the new channel (real set: pan != 0, != 0xFFFF) so
	// ecu-zb adopts newChannel as its opChannel. This is the point of no
	// return — past here the fleet is on the new channel.
	m.status.setStage(StageChannel, "move module to new channel")
	if err := m.Transport.setModulePan(ctx, pan, newChannel); err != nil {
		m.changeChannelRollback(pan, oldChannel, newChannel, fmt.Errorf("move module to ch %d: %w", newChannel, err))
		return
	}
	if err := m.Settings.SetChannel(ctx, newChannel); err != nil {
		// The module has moved but persistence failed. Roll the module back
		// to the old channel so the running config matches what's persisted.
		m.changeChannelRollback(pan, oldChannel, newChannel, fmt.Errorf("persist channel: %w", err))
		return
	}
	m.milestone(ctx, "", "channel_change_committed", "info",
		fmt.Sprintf("ch %d -> %d (PAN 0x%04X)", oldChannel, newChannel, pan))

	// REBIND + VERIFY: re-query each short address on the new channel. The
	// fleet has already moved (the module is on the new channel), so a partial
	// result is reported, not rolled back. Unverified units may simply not have
	// answered the re-query yet, or may be stragglers left behind on the old
	// channel — recover by re-running the change-channel toward newChannel (the
	// module hops back to old, re-hops the stragglers reachable there, then
	// moves to new and re-verifies the units that already moved).
	m.rebindVerify(ctx, serials, rebindTemplates{
		stage:      StageChannel,
		errKind:    "channel_change_error",
		partialErr: fmt.Sprintf("channel migrated to %d but only %%d/%%d inverters verified; re-run the change-channel to %d to converge the fleet", newChannel, newChannel),
		partialMsg: fmt.Sprintf("channel partial verify %%d/%%d; re-run change-channel to %d to converge", newChannel),
		done:       fmt.Sprintf("fleet moved to channel %d (%%d/%%d verified)", newChannel),
	})
}

// changeChannelRollback restores the MODULE to the old channel and records
// the error. Used on any failure up to and including the persist step.
//
// Channel migration is non-atomic: each inverter hops the instant it receives
// the directed 0x0F, so on a mid-loop failure the units already hopped are now
// on the NEW channel and this module-only rollback cannot un-hop them — the
// fleet is split across two channels. Returning the module to the old channel
// keeps the not-yet-hopped units reporting; the clean recovery for the
// already-hopped ones is to RE-RUN the change-channel toward newChannel (the
// module rolls back to old, re-hops the stragglers reachable there, then moves
// to new and picks up the units that already hopped). Do NOT expect recovery
// on the old channel.
func (m *Manager) changeChannelRollback(pan, oldChannel, newChannel uint32, cause error) {
	rbCtx, cancel := context.WithTimeout(context.Background(), defaultCmdTimeout)
	defer cancel()
	rbErr := m.Transport.setModulePan(rbCtx, pan, oldChannel)
	var msg string
	if rbErr != nil {
		msg = fmt.Sprintf("%v; rollback to channel %d also failed: %v; any already-hopped inverters are on channel %d — re-run the change-channel to %d to converge the fleet", cause, oldChannel, rbErr, newChannel, newChannel)
	} else {
		msg = fmt.Sprintf("%v; module rolled back to channel %d, but any already-hopped inverters are on channel %d — re-run the change-channel to %d to converge the fleet", cause, oldChannel, newChannel, newChannel)
	}
	m.status.finishError(msg)
	m.milestone(context.Background(), "", "channel_change_error", "error", msg)
}

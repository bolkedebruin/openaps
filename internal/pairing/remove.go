package pairing

import (
	"context"
	"fmt"

	"github.com/bolkedebruin/openaps/wire"
)

// startRemove removes one inverter from the fleet. Two cases:
//
//   - force=true: a mistyped / never-live entry. No radio op is attempted; the
//     DB rows are simply deleted.
//   - force=false: a live decommission. A best-effort EVICT re-PANs the unit to
//     the rendezvous PAN 0xFFFF on the current operating channel (0x0F), so it
//     drops off the operating network. The DB rows are deleted whether the
//     evict succeeds or fails.
//
// In BOTH cases the DB rows are ALWAYS deleted. There is no tombstone and no
// ingest suppression: if a failed-evict unit calls in again it will reappear,
// and the operator can re-run the removal. The terminal status carries the
// outcome — Evicted=true on a clean evict, else Message warns that the unit may
// reappear (failed evict) or describes the plain delete (force).
//
// The op runs through startOp so it acquires the PRIORITY bus lock (the
// telemetry pause) exactly like the other operator ops; a concurrent telemetry
// poll yields to it.
func (m *Manager) startRemove(by string, req *wire.RemoveById) *wire.PairingResponse {
	serial := req.GetSerial()
	if !serialRe.MatchString(serial) {
		return errResp(fmt.Sprintf("remove: serial %q is not 12 decimal digits", serial))
	}
	if m.Store == nil {
		return errResp("remove: store not configured")
	}
	force := req.GetForce()
	// A best-effort evict needs the transport and a resolvable channel; fail
	// fast on a non-force request that can't even attempt the evict so the
	// operator isn't told "removed" when nothing happened on the radio. The
	// plain (force) delete needs neither.
	if !force {
		if m.Transport == nil {
			return errResp("remove: transport not configured (use force to delete the entry without evicting)")
		}
	}

	// In this codebase the store key (uid) is the 12-digit serial as a string;
	// the same value is the pairing/evict identifier. serialRe guarantees it is
	// also a valid uid, so no conversion is needed.
	uid := serial

	return m.startOp(by, OpRemove, StageRemove, 1, func(ctx context.Context) {
		m.status.update(func(s *PairingStatus) { s.CurrentSerial = serial })

		evicted := false
		message := ""

		if force {
			message = fmt.Sprintf("removed entry %s (no evict; force delete)", serial)
		} else {
			m.status.setStage(StageEvict, "re-PAN to rendezvous 0xFFFF")
			m.status.setInverterState(serial, "evicting")
			// Resolve the current operating channel from the radio owner. A
			// failure here does NOT abort the removal — it is folded into the
			// best-effort evict result.
			if err := m.evictInverter(ctx, serial); err != nil {
				message = fmt.Sprintf("evict of %s failed (%v); inventory entry deleted — the unit may reappear if it calls in again, re-run remove to evict it", serial, err)
				m.status.setInverterState(serial, "evict-failed")
				m.milestone(ctx, uid, "inverter_evict_failed", "warn", err.Error())
			} else {
				evicted = true
				message = fmt.Sprintf("evicted %s to rendezvous PAN 0xFFFF; inventory entry deleted", serial)
				m.status.setInverterState(serial, "evicted")
			}
		}

		// ALWAYS delete the DB rows, regardless of the evict outcome.
		m.status.setStage(StageDelete, "delete inventory rows")
		if err := m.Store.DeleteInverter(ctx, uid); err != nil {
			m.fail(fmt.Errorf("remove %s: delete inventory: %w", serial, err))
			return
		}

		m.milestone(ctx, uid, "inverter_removed", "warn", fmt.Sprintf("evicted=%v force=%v", evicted, force))
		m.status.update(func(s *PairingStatus) {
			s.Evicted = evicted
			s.Done = 1
		})
		m.status.finishDone(message)
	})
}

// evictInverter performs the best-effort radio evict: it reads the current
// operating channel from the radio owner (ecu-zb) and directs the inverter
// (0x0F) onto the rendezvous PAN 0xFFFF on that channel, so it leaves the
// operating network. Any failure (channel read or the directed set) is returned
// to the caller, which treats it as a non-fatal best-effort outcome.
func (m *Manager) evictInverter(ctx context.Context, serial string) error {
	_, channel, err := m.Transport.getModuleConfig(ctx)
	if err != nil {
		return fmt.Errorf("read operating channel: %w", err)
	}
	if channel == 0 {
		// Older backend that doesn't report a channel: fall back to the
		// configured current channel so the evict still targets a real one.
		ch, cerr := m.channelOrCurrent(0)
		if cerr != nil {
			return fmt.Errorf("operating channel unavailable: %w", cerr)
		}
		channel = ch
	}
	if err := m.Transport.setInvPan(ctx, serial, 0xFFFF, channel); err != nil {
		return fmt.Errorf("set inverter PAN 0xFFFF ch=%d: %w", channel, err)
	}
	return nil
}

package pairing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// commitSettle is the post-commit quiet window — the migration radio
// latency is on the order of seconds; do not race a read across it.
const commitSettle = 10 * time.Second

// startScan runs a discovery scan (fast = current channel on PAN 0xFFFF;
// slow = sweep chan_lo..chan_hi). Found serials are recorded to the status
// and persisted as pairing rows; the operator chooses what to bind next via
// Add. Scan does NOT auto-bind (the contract's Add/Replace flows bind).
func (m *Manager) startScan(by string, req *wire.ScanStart) *wire.PairingResponse {
	if m.Transport == nil {
		return errResp("scan: transport not configured")
	}
	chLo, chHi, dwell := req.GetChanLo(), req.GetChanHi(), req.GetDwellMs()
	if chLo == 0 {
		chLo = defaultChanLo
	}
	if chHi == 0 {
		chHi = defaultChanHi
	}
	// Clamp the dwell window to a sane floor/ceiling so a scan cannot park
	// the radio off-PAN for an absurd duration.
	dwell = clampScanWindow(dwell)
	slow := req.GetSlow()

	// Resolve the effective channel (0 → current) before launching the op so
	// a missing current channel fails fast. The range itself is not validated
	// here — ecu-zb's checkChannel is the authority and rejects an
	// out-of-range value at the wire.
	ch, err := m.channelOrCurrent(0)
	if err != nil {
		return errResp("scan: " + err.Error())
	}

	return m.startOp(by, OpScan, StageScan, 0, func(ctx context.Context) {
		m.milestone(ctx, "", "scan_started", "info", scanDetail(slow, chLo, chHi))
		var found []*wire.FoundInverter
		var err error
		if slow {
			found, err = m.sweepScan(ctx, ch, chLo, chHi, dwell)
		} else {
			found, err = m.fastScan(ctx, ch)
		}
		if m.abortedOr(ctx) {
			return
		}
		if err != nil {
			m.fail(fmt.Errorf("scan: %w", err))
			return
		}
		m.recordFound(ctx, found)
		m.status.finishDone(fmt.Sprintf("scan complete: %d inverter(s) found", len(found)))
	})
}

// fastScan parks the module on PAN 0xFFFF (current channel) and runs one
// report-id window. Even "fast" leaves the operating PAN — telemetry pauses
// while parked on 0xFFFF.
func (m *Manager) fastScan(ctx context.Context, ch uint32) ([]*wire.FoundInverter, error) {
	m.status.setStage(StageScan, "park on rendezvous PAN")
	m.status.update(func(s *PairingStatus) {
		s.Sweep = Sweep{Chan: ch, ChanLo: ch, ChanHi: ch}
		s.Substep = "report-id scan"
	})
	found, err := m.scanChannel(ctx, ch, fastScanWindowMs)
	// Restore the module to the operating PAN regardless of scan outcome.
	m.restoreModule(ch)
	return found, err
}

// sweepScan walks channels chLo..chHi on PAN 0xFFFF, dwelling dwell per
// channel and accumulating distinct serials.
func (m *Manager) sweepScan(ctx context.Context, restoreCh, chLo, chHi, dwell uint32) ([]*wire.FoundInverter, error) {
	// Restore the radio to the operating PAN on EVERY exit path — a failed
	// channel step or a mid-sweep abort must not leave it parked on 0xFFFF.
	defer m.restoreModule(restoreCh)
	byID := map[string]*wire.FoundInverter{}
	order := []string{}
	for ch := chLo; ch <= chHi; ch++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		m.status.update(func(s *PairingStatus) {
			s.Sweep = Sweep{Chan: ch, ChanLo: chLo, ChanHi: chHi}
			s.Substep = fmt.Sprintf("sweep channel %d", ch)
		})
		found, err := m.scanChannel(ctx, ch, dwell)
		if err != nil {
			return nil, err
		}
		for _, fi := range found {
			if _, ok := byID[fi.GetSerial()]; !ok {
				order = append(order, fi.GetSerial())
			}
			byID[fi.GetSerial()] = fi
		}
	}
	out := make([]*wire.FoundInverter, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out, nil
}

// scanChannel parks the module on the rendezvous PAN at ch and runs one
// report-id window. It records the channel of everything that answers. Every
// path that listens on a channel goes through here: the fast scan, the
// sweep, the locate, and the pre-migration confirmation.
func (m *Manager) scanChannel(ctx context.Context, ch, dwell uint32) ([]*wire.FoundInverter, error) {
	if err := m.Transport.setModulePan(ctx, 0xFFFF, ch); err != nil {
		return nil, fmt.Errorf("set module pan ch=%d: %w", ch, err)
	}
	found, err := m.Transport.reportScan(ctx, dwell)
	if err != nil {
		return nil, fmt.Errorf("report scan ch=%d: %w", ch, err)
	}
	m.recordFoundChannel(ctx, found, ch)
	return found, nil
}

// answered reports whether serial is among the units that announced.
func answered(found []*wire.FoundInverter, serial string) bool {
	for _, fi := range found {
		if fi.GetSerial() == serial {
			return true
		}
	}
	return false
}

// restoreModule returns the radio to ecu-zb's operating PAN by sending the
// pan=0 sentinel — ecu-zb (the radio owner) resolves the actual PAN+channel
// it bonded to, so inv-driver never re-derives it. Best-effort: a restore
// failure is logged via status but not fatal (the next telemetry cycle /
// ecu-zb restart re-parks it). The ch argument is unused for pan=0 (ecu-zb
// uses its own operating channel) but kept for call-site symmetry.
func (m *Manager) restoreModule(_ uint32) {
	// Use a fresh short-lived context so an aborted op still restores the
	// radio rather than leaving it parked on 0xFFFF.
	ctx, cancel := context.WithTimeout(context.Background(), defaultCmdTimeout)
	defer cancel()
	if err := m.Transport.setModulePan(ctx, 0, 0); err != nil {
		m.status.update(func(s *PairingStatus) { s.Message = "warning: module not restored to operating PAN: " + err.Error() })
	}
}

// recordFoundChannel persists the RF channel on which each unit answered.
// Only this code knows the channel: the scan commanded the park, and the
// announcement itself carries no channel. Without the record, a later add
// can only guess the ECU's own channel. That is the one channel on which a
// unit bound to some other ECU is least likely to listen.
//
// Best effort: a persist failure costs the add a hint. It does not fail the
// scan.
func (m *Manager) recordFoundChannel(ctx context.Context, found []*wire.FoundInverter, ch uint32) {
	if m.Store == nil {
		return
	}
	for _, fi := range found {
		serial := fi.GetSerial()
		if !serialRe.MatchString(serial) {
			continue
		}
		_ = m.Store.SetInverterFoundChannel(ctx, serial, ch)
	}
}

// recordFound persists each discovered inverter (pairing_state=found,
// encrypted, last_announce_ms) and surfaces it in the status, emitting one
// inverter_found milestone per new serial.
func (m *Manager) recordFound(ctx context.Context, found []*wire.FoundInverter) {
	now := time.Now().UnixMilli()
	for _, fi := range found {
		serial := fi.GetSerial()
		if !serialRe.MatchString(serial) {
			continue
		}
		m.status.upsertInverter(PerInverter{
			Serial:    serial,
			ShortAddr: fi.GetShortAddr(),
			State:     "found",
			Encrypted: fi.GetEncrypted(),
		})
		if m.Store != nil {
			_ = m.Store.SetInverterPairingState(ctx, serial, "found", now)
			_ = m.Store.SetInverterEncrypted(ctx, serial, fi.GetEncrypted())
		}
		m.milestone(ctx, serial, "inverter_found", "info", fmt.Sprintf("encrypted=%v", fi.GetEncrypted()))
	}
}

// startAdd inserts a known serial and runs bind (+ migrate if off-PAN).
func (m *Manager) startAdd(by string, req *wire.AddById) *wire.PairingResponse {
	serial := req.GetSerial()
	if !serialRe.MatchString(serial) {
		return errResp(fmt.Sprintf("add: serial %q is not 12 decimal digits", serial))
	}
	if m.Transport == nil {
		return errResp("add: transport not configured")
	}
	if _, err := m.channelOrCurrent(0); err != nil {
		return errResp("add: " + err.Error())
	}
	return m.startOp(by, OpAdd, StageBind, 1, func(ctx context.Context) {
		m.status.update(func(s *PairingStatus) { s.CurrentSerial = serial })
		m.status.upsertInverter(PerInverter{Serial: serial, State: "binding"})
		if err := m.bindAndMigrate(ctx, serial); err != nil {
			if m.abortedOr(ctx) {
				return
			}
			m.fail(fmt.Errorf("add %s: %w", serial, err))
			return
		}
		m.status.update(func(s *PairingStatus) { s.Done = 1 })
		m.status.finishDone("inverter added: " + serial)
	})
}

// startReplace swaps a dead unit for a new serial, inheriting config + slot.
func (m *Manager) startReplace(by string, req *wire.ReplaceInverter) *wire.PairingResponse {
	oldUID := req.GetOldUid()
	newSerial := req.GetNewSerial()
	if !uidRe.MatchString(oldUID) {
		return errResp(fmt.Sprintf("replace: old_uid %q invalid", oldUID))
	}
	if newSerial == "" {
		return errResp("replace: new_serial is required (scan-to-find not yet implemented)")
	}
	if !serialRe.MatchString(newSerial) {
		return errResp(fmt.Sprintf("replace: new_serial %q is not 12 decimal digits", newSerial))
	}
	if m.Transport == nil {
		return errResp("replace: transport not configured")
	}
	if _, err := m.channelOrCurrent(0); err != nil {
		return errResp("replace: " + err.Error())
	}
	return m.startOp(by, OpReplace, StageBind, 1, func(ctx context.Context) {
		m.status.update(func(s *PairingStatus) { s.CurrentSerial = newSerial })
		m.status.upsertInverter(PerInverter{Serial: newSerial, State: "binding"})

		// Bind + migrate the replacement onto our PAN.
		if err := m.bindAndMigrate(ctx, newSerial); err != nil {
			if m.abortedOr(ctx) {
				return
			}
			m.fail(fmt.Errorf("replace %s: %w", newSerial, err))
			return
		}

		// CONFIGURE: inherit the dead unit's grid profile, then retire it.
		m.status.setStage(StageConfigure, "inherit grid profile")
		m.status.setInverterState(newSerial, "configuring")
		summary := "config inherited"
		if m.Profiles != nil {
			s, err := m.Profiles.InheritProfile(ctx, oldUID, newSerial)
			if err != nil {
				m.fail(fmt.Errorf("replace %s: inherit profile: %w", newSerial, err))
				return
			}
			if s != "" {
				summary = s
			}
		}
		// Inherit the operator label (the ECU's stand-in for "slot") from the
		// dead unit to the replacement. Best-effort: a missing label is not a
		// failure.
		if m.Names != nil {
			if moved, err := m.Names.InheritName(ctx, oldUID, newSerial); err != nil {
				m.fail(fmt.Errorf("replace %s: inherit label: %w", newSerial, err))
				return
			} else if moved {
				summary += "; label inherited"
			}
		}
		// Retire the dead unit from the inventory.
		if m.Store != nil {
			if err := m.Store.DeleteInverter(ctx, oldUID); err != nil {
				m.fail(fmt.Errorf("replace: retire old unit %s: %w", oldUID, err))
				return
			}
		}
		m.status.setInverterState(newSerial, "configured")
		m.status.update(func(s *PairingStatus) { s.Done = 1 })
		// Power cap is not inherited (new hardware may warrant a different
		// cap); prompt the operator to re-apply it if needed.
		m.status.finishDone(fmt.Sprintf("replaced %s with %s (%s); re-apply power cap if needed", oldUID, newSerial, summary))
	})
}

// bindAndMigrate is the per-serial BIND (+ single MIGRATE when off-PAN)
// flow shared by Add and Replace. It queries the short address; if the unit
// is not reachable on our PAN it performs a single rendezvous migration
// (0xFFFF → prime → commit → operating PAN) and re-queries. On success the
// short address is persisted and the unit is bound quiet.
func (m *Manager) bindAndMigrate(ctx context.Context, serial string) error {
	// The migrate target PAN is ecu-zb's operating PAN, queried from the radio
	// owner rather than re-derived here.
	pan, err := m.Transport.getModulePan(ctx)
	if err != nil {
		return fmt.Errorf("operating PAN: %w", err)
	}
	ch, err := m.channelOrCurrent(0)
	if err != nil {
		return err
	}

	m.status.setStage(StageBind, "query short address")
	sa, err := m.Transport.getShortAddr(ctx, serial)
	if err != nil {
		// The unit is not reachable on our PAN. Find the channel it IS on,
		// then run a single rendezvous migration from there.
		srcCh, migErr := m.migrateFromRecalledChannel(ctx, serial, pan, ch)
		if errors.Is(migErr, errUnitNotOnChannel) || srcCh == 0 {
			// There is no usable record, or the unit moved after the
			// record. Sweep for where it is now, then migrate from there.
			located, locErr := m.locateChannel(ctx, serial, ch)
			if locErr != nil {
				return locErr
			}
			srcCh = located
			migErr = m.migrateOne(ctx, serial, pan, ch, srcCh)
		}
		if migErr != nil {
			return fmt.Errorf("migrate from channel %d: %w", srcCh, migErr)
		}
		// Re-query after migration.
		m.status.setStage(StageBind, "re-query short address")
		sa, err = m.Transport.getShortAddr(ctx, serial)
		if err != nil {
			return fmt.Errorf("get short addr after migrate from channel %d: %w", srcCh, err)
		}
	}
	if sa == 0 {
		return fmt.Errorf("inverter %s returned short_addr 0 (not joined)", serial)
	}

	m.status.upsertInverter(PerInverter{Serial: serial, ShortAddr: uint32(sa), State: "bound"})
	if m.Store != nil {
		if err := m.Store.SetInverterShortAddr(ctx, serial, sa); err != nil {
			return fmt.Errorf("persist short addr: %w", err)
		}
		_ = m.Store.SetInverterPairingState(ctx, serial, "bound", 0)
	}

	// Bind quiet: turn report-id off so the unit goes silent in steady state.
	m.status.setStage(StageBind, "bind quiet")
	if err := m.Transport.bindQuiet(ctx, sa); err != nil {
		return fmt.Errorf("bind quiet: %w", err)
	}
	m.milestone(ctx, serial, "bind_ok", "info", fmt.Sprintf("short_addr=0x%04X", sa))
	return nil
}

// errUnitNotOnChannel reports that the inverter did not announce itself on
// the channel where the rendezvous parked. The runner sent nothing at it.
var errUnitNotOnChannel = errors.New("inverter did not answer on that channel")

// migrateFromRecalledChannel tries the channel on which the last scan heard
// this unit. It returns that channel and the migration outcome. It returns
// channel 0 when there is no record to try.
//
// The record is a hint, not a licence to send. It can be months old, and the
// unit may have moved to another ECU since. migrateOne confirms that the
// unit is really there before anything goes out. If the unit is not there,
// migrateOne reports errUnitNotOnChannel. A stale record therefore costs one
// scan window and then falls back to the sweep.
func (m *Manager) migrateFromRecalledChannel(ctx context.Context, serial string, pan, opChannel uint32) (uint32, error) {
	if m.Store == nil {
		return 0, nil
	}
	row, err := m.Store.GetInverterPairing(ctx, serial)
	// A record of our own channel is no help. The direct query on it
	// already failed. That is how we got here.
	if err != nil || row.FoundChannel == 0 || row.FoundChannel == opChannel {
		return 0, nil
	}
	m.milestone(ctx, serial, "channel_recalled", "info",
		fmt.Sprintf("last heard on channel %d", row.FoundChannel))
	return row.FoundChannel, m.migrateOne(ctx, serial, pan, opChannel, row.FoundChannel)
}

// locateChannel sweeps the rendezvous PAN across the usable channels for one
// serial. It stops at the channel on which the serial answers, and it
// records that channel, so a later add skips the sweep. It restores the
// module to the operating PAN on every exit path.
//
// A sweep parks the radio off the operating PAN, and telemetry stops for the
// whole fleet while it runs. It therefore happens only after a direct query
// failed and after the add tried any recorded channel.
func (m *Manager) locateChannel(ctx context.Context, serial string, restoreCh uint32) (uint32, error) {
	m.status.setStage(StageMigrate, fmt.Sprintf("locating %s across channels %d-%d", serial, defaultChanLo, defaultChanHi))
	m.milestone(ctx, serial, "channel_sweep_started", "info",
		fmt.Sprintf("channels %d-%d, telemetry paused", defaultChanLo, defaultChanHi))
	defer m.restoreModule(restoreCh)

	for ch := defaultChanLo; ch <= defaultChanHi; ch++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		m.status.update(func(s *PairingStatus) {
			s.Sweep = Sweep{Chan: ch, ChanLo: defaultChanLo, ChanHi: defaultChanHi}
			s.Substep = fmt.Sprintf("listening for %s on channel %d", serial, ch)
		})
		found, err := m.scanChannel(ctx, ch, defaultDwellMs)
		if err != nil {
			return 0, fmt.Errorf("locate %s: %w", serial, err)
		}
		if answered(found, serial) {
			m.milestone(ctx, serial, "channel_located", "info", fmt.Sprintf("answered on channel %d", ch))
			return ch, nil
		}
	}
	return 0, fmt.Errorf("inverter %s did not answer on any channel %d-%d: it is powered down, out of range, or not an inverter on this site",
		serial, defaultChanLo, defaultChanHi)
}

// migrateOne runs a single-inverter rendezvous migration onto pan/ch. The
// rendezvous runs on srcChannel, the channel on which the inverter listens
// now. The prime and the commit carry the target pan/ch. One prime+commit
// therefore moves the unit across channels and across PANs. migrateOne
// always restores the module to the operating PAN, also on the abort and
// error paths. It never leaves the rest of the fleet behind.
//
// migrateOne sends nothing at the inverter until the inverter announces
// itself on srcChannel. The prime is a directed frame with no
// acknowledgement. The commit is a bare broadcast with no destination at
// all. Neither can report a wrong channel. Without the confirmation, a stale
// record would put a PAN-change broadcast on a channel that this site does
// not own. The mistake would only appear later, as a failed re-query.
func (m *Manager) migrateOne(ctx context.Context, serial string, pan, ch, srcChannel uint32) (err error) {
	m.status.setStage(StageMigrate, fmt.Sprintf("rendezvous on PAN 0xFFFF channel %d", srcChannel))
	m.status.setInverterState(serial, "migrating")
	defer m.restoreModule(ch)

	m.status.update(func(s *PairingStatus) {
		s.Substep = fmt.Sprintf("confirming %s is on channel %d", serial, srcChannel)
	})
	found, err := m.scanChannel(ctx, srcChannel, defaultDwellMs)
	if err != nil {
		return fmt.Errorf("module to 0xFFFF ch=%d: %w", srcChannel, err)
	}
	if !answered(found, serial) {
		return fmt.Errorf("%w: %s silent on channel %d", errUnitNotOnChannel, serial, srcChannel)
	}
	m.status.setStage(StageMigrate, "prime inverter")
	if err := m.Transport.primeInv(ctx, serial, pan, ch); err != nil {
		return fmt.Errorf("prime: %w", err)
	}
	m.status.setStage(StageMigrate, "commit PAN")
	if err := m.Transport.commitPan(ctx, pan, ch); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	// Post-commit settle before the module hops back and we re-query.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(m.settleDur()):
	}
	m.milestone(ctx, serial, "migrate_ok", "info", fmt.Sprintf("pan=0x%04X ch=%d (from channel %d)", pan, ch, srcChannel))
	return nil
}

// scanDetail builds the scan_started milestone detail string.
func scanDetail(slow bool, chLo, chHi uint32) string {
	if slow {
		return fmt.Sprintf("slow sweep ch %d-%d on PAN 0xFFFF", chLo, chHi)
	}
	return "fast scan on PAN 0xFFFF"
}

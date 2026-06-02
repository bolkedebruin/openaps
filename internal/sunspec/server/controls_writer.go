package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
	"github.com/bolkedebruin/openaps/internal/sunspec/sunspec"
	"github.com/bolkedebruin/openaps/codec"
)

// frameSender dispatches one L2 frame to an inverter by UID. Satisfied by
// *invdriver.Client; an interface here keeps the Model 123 write path
// testable without a live UDS daemon.
type frameSender interface {
	Send(ctx context.Context, uid string, frame []byte) error
}

// sendPanelWatts encodes a per-panel watt cap for one inverter and hands
// the frame to inv-driver for radio dispatch. modelCode selects the
// family encoder (DS3 / QS1A / YC600-C3). Shared by ControlsWriter and
// Reverter so both speak the same path to the wire.
func sendPanelWatts(ctx context.Context, sender frameSender, uid string, modelCode uint8, panelW int) error {
	frame, err := codec.EncodeSetPower(modelCode, uint16(panelW), false)
	if err != nil {
		return err
	}
	return sender.Send(ctx, uid, frame)
}

// ControlsWriter applies Modbus writes that target SunSpec Model 123
// (Inverter Controls). It encodes the register-level operation as an L2
// radio frame (set-power for WMaxLimPct, on/off for Conn) and dispatches
// it through inv-driver.
//
// uid identifies which bank the write came in on:
//
//	uid 1     → applies to ALL inverters in snap.Inverters (aggregate)
//	uid 2..N+1 → applies to snap.Inverters[uid-2] only
type ControlsWriter struct {
	uid      uint8
	snap     source.Snapshot
	sender   frameSender
	reverter *Reverter
	limits   *source.PowerLimitCache
	logger   *log.Logger
}

// Apply takes a slice of registers freshly written by a client (offsets
// relative to the Model 123 body, length up to ControlsBodyLen) and dispatches
// the resulting SQL operations.
//
// The write semantics mirror the standard SunSpec definitions:
//
//	WMaxLim_Pct + WMaxLim_Ena=1 → cap each affected inverter's per-panel
//	                              watts to (raw/WMaxLimPctRawFull) ×
//	                              (NameplateW / PanelCount), bounded by
//	                              [MinPanelLimitW, MaxPanelLimitW].
//	WMaxLim_Ena=0               → restore to MaxPanelLimitW (full output).
//	WMaxLim_Pct_RvrtTms > 0     → schedule auto-revert in N seconds. If the
//	                              controller fails to refresh within that
//	                              window, the cap is lifted (pre-2018
//	                              Model 123 reversion semantics).
//	Conn=0                      → turn the inverter(s) off.
//	Conn=1                      → turn the inverter(s) back on.
func (cw *ControlsWriter) Apply(ctx context.Context, addrOffset uint16, regs []uint16) error {
	if cw == nil || cw.sender == nil {
		return fmt.Errorf("writes disabled or inv-driver not configured")
	}

	targets := cw.targetUIDs()
	if len(targets) == 0 {
		return fmt.Errorf("no inverters mapped to unit ID %d", cw.uid)
	}

	enaWritten := writeTouches(addrOffset, regs, sunspec.OffControlsWMaxLimEna)
	pctWritten := writeTouches(addrOffset, regs, sunspec.OffControlsWMaxLimPct)
	rvrtWritten := writeTouches(addrOffset, regs, sunspec.OffControlsWMaxLimPctRvrtTms)

	// WMaxLim_Ena=0 written explicitly → restore full output and cancel any
	// pending reversion. WMaxLimPct written (with or without Ena) → apply
	// that cap; if RvrtTms is non-zero, arm the reverter. Ena=1 written
	// without Pct → no DB action (existing cap stays).
	if enaWritten && readField(addrOffset, regs, sunspec.OffControlsWMaxLimEna) == 0 {
		cw.reverter.Cancel(cw.uid)
		if err := cw.restoreFull(ctx, targets); err != nil {
			return err
		}
	} else if pctWritten {
		pct := readField(addrOffset, regs, sunspec.OffControlsWMaxLimPct)
		if err := cw.applyCap(ctx, targets, pct); err != nil {
			return err
		}
		if rvrtWritten {
			rvrt := readField(addrOffset, regs, sunspec.OffControlsWMaxLimPctRvrtTms)
			cw.armOrCancelReversion(rvrt, targets)
		}
	}

	// Did the write touch the Conn (connect/disconnect) field? Apply on/off.
	if writeTouches(addrOffset, regs, sunspec.OffControlsConn) {
		conn := readField(addrOffset, regs, sunspec.OffControlsConn)
		if err := cw.applyConn(ctx, targets, conn); err != nil {
			return err
		}
	}

	return nil
}

// targetUIDs returns the inverter UIDs the write should affect.
func (cw *ControlsWriter) targetUIDs() []string {
	if cw.uid <= 1 {
		// aggregate
		out := make([]string, 0, len(cw.snap.Inverters))
		for _, inv := range cw.snap.Inverters {
			out = append(out, inv.UID)
		}
		return out
	}
	idx := int(cw.uid) - 2
	if idx < 0 || idx >= len(cw.snap.Inverters) {
		return nil
	}
	return []string{cw.snap.Inverters[idx].UID}
}

func (cw *ControlsWriter) restoreFull(ctx context.Context, uids []string) error {
	for _, uid := range uids {
		inv, ok := findInverter(cw.snap.Inverters, uid)
		if !ok {
			return fmt.Errorf("restore %s: not in snapshot", uid)
		}
		if err := sendPanelWatts(ctx, cw.sender, uid, uint8(inv.Model), source.MaxPanelLimitW); err != nil {
			return fmt.Errorf("restore %s: %w", uid, err)
		}
		cw.limits.Set(uid, source.MaxPanelLimitW)
	}
	return nil
}

func (cw *ControlsWriter) applyCap(ctx context.Context, uids []string, rawPct uint16) error {
	if rawPct > sunspec.WMaxLimPctRawFull {
		rawPct = sunspec.WMaxLimPctRawFull
	}
	full := int64(sunspec.WMaxLimPctRawFull)
	for _, uid := range uids {
		inv, ok := findInverter(cw.snap.Inverters, uid)
		if !ok {
			return fmt.Errorf("setmax %s: not in snapshot", uid)
		}
		panels := inv.PanelCount()
		if panels <= 0 {
			return fmt.Errorf("setmax %s: panel count is zero", uid)
		}
		nameplatePerPanel := inv.NameplateW() / panels
		if nameplatePerPanel <= 0 {
			return fmt.Errorf("setmax %s: nameplate per panel is zero", uid)
		}
		target := int(int64(rawPct) * int64(nameplatePerPanel) / full)
		// Below MinPanelLimitW (20 W/panel) the inverter shuts off, so an
		// aggressive curtailment must never reach the firmware. Clamp up.
		if target < source.MinPanelLimitW {
			cw.warnf("setmax %s: WMaxLimPct=%d → %d W/panel below floor, clamped to %d W",
				uid, rawPct, target, source.MinPanelLimitW)
			target = source.MinPanelLimitW
		}
		if target > source.MaxPanelLimitW {
			target = source.MaxPanelLimitW
		}
		if err := sendPanelWatts(ctx, cw.sender, uid, uint8(inv.Model), target); err != nil {
			return fmt.Errorf("setmax %s: %w", uid, err)
		}
		cw.limits.Set(uid, target)
	}
	return nil
}

func (cw *ControlsWriter) warnf(format string, args ...interface{}) {
	if cw.logger != nil {
		cw.logger.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// findInverter looks up an inverter in the snapshot by UID.
func findInverter(invs []source.Inverter, uid string) (source.Inverter, bool) {
	for _, inv := range invs {
		if inv.UID == uid {
			return inv, true
		}
	}
	return source.Inverter{}, false
}

// armOrCancelReversion translates the wire-format RvrtTms value into a
// reverter Schedule call. The SunSpec uint16 not-implemented sentinel
// (0xFFFF) and zero are both treated as "no auto-revert."
func (cw *ControlsWriter) armOrCancelReversion(rvrtTmsSec uint16, targets []string) {
	if rvrtTmsSec == 0 || rvrtTmsSec == 0xFFFF {
		cw.reverter.Cancel(cw.uid)
		return
	}
	cw.reverter.Schedule(cw.uid, time.Duration(rvrtTmsSec)*time.Second, cw.revertTargets(targets))
}

// revertTargets resolves uids to {uid, modelCode} pairs from the current
// snapshot so the reverter can restore full power later without holding a
// snapshot reference. uids absent from the snapshot are skipped.
func (cw *ControlsWriter) revertTargets(uids []string) []revertTarget {
	out := make([]revertTarget, 0, len(uids))
	for _, uid := range uids {
		if inv, ok := findInverter(cw.snap.Inverters, uid); ok {
			out = append(out, revertTarget{uid: uid, modelCode: uint8(inv.Model)})
		}
	}
	return out
}

func (cw *ControlsWriter) applyConn(ctx context.Context, uids []string, conn uint16) error {
	on := conn != 0
	// On/off is family-independent, so the frame is identical for every
	// target — build it once.
	frame := codec.EncodeOnOff(on, false)
	for _, uid := range uids {
		if err := cw.sender.Send(ctx, uid, frame); err != nil {
			return fmt.Errorf("turn %s: %w", uid, err)
		}
	}
	return nil
}

// writeTouches reports whether the FC16 write at addrOffset (relative to the
// Model 123 body) covers a particular field offset.
func writeTouches(addrOffset uint16, regs []uint16, fieldOff uint16) bool {
	if fieldOff < addrOffset {
		return false
	}
	if uint16(len(regs)) <= fieldOff-addrOffset {
		return false
	}
	return true
}

// readField extracts a field value from a partial write. Returns 0 if the
// write didn't include this field.
func readField(addrOffset uint16, regs []uint16, fieldOff uint16) uint16 {
	if !writeTouches(addrOffset, regs, fieldOff) {
		return 0
	}
	return regs[fieldOff-addrOffset]
}

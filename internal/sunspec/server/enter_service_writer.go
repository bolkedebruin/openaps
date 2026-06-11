package server

import (
	"context"
	"fmt"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
)

// EnterServiceWriter applies Modbus writes that target SunSpec Model 703
// (Enter Service). Of the model's writable fields, ONLY ESDlyTms (the
// reconnect delay) is exposed — the V/Hz boundaries (ESVHi/ESVLo/ESHzHi/
// ESHzLo) are regulatory grid-protection values and stay read-only.
//
// ESDlyTms maps to APsystems' `grid_recovery_time` (2-letter code AG).
// Per-parameter dispatch via inv-driver, which translates to ZigBee on
// the next poll cycle.
//
// Writer targets a single inverter when uid is 2..N+1; aggregate (uid 1) is
// rejected — pushing reconnect-time across a heterogeneous fleet has no
// useful semantics, and the gridProfile dispatch path already covers
// fleet-wide profile changes.
type EnterServiceWriter struct {
	uid    uint8
	snap   source.Snapshot
	sender frameSender
}

// EnterServiceMinReconnectS / MaxReconnectS are the firmware's documented
// range for grid_recovery_time across regulatory profiles. Outside this
// range the inverter silently rejects; we reject sync with exception 03.
const (
	EnterServiceMinReconnectS = 10
	EnterServiceMaxReconnectS = 610
)

// Apply handles a write to the Model 703 register window. addrOffset is
// relative to the model body (i.e. body[0] = ES, body[1] = ESVHi, …),
// matching the layout in `internal/sunspec/enter_service.go`.
//
//	body[0]   ES enum16            read-only
//	body[1]   ESVHi  uint16 % VRef read-only (regulatory)
//	body[2]   ESVLo  uint16 % VRef read-only (regulatory)
//	body[3..4]   ESHzHi uint32 Hz   read-only (regulatory)
//	body[5..6]   ESHzLo uint32 Hz   read-only (regulatory)
//	body[7..8]   ESDlyTms uint32 s  WRITABLE — maps to grid_recovery_time
//	body[9..10]  ESRndTms uint32 s  read-only (not exposed by APsystems)
//	body[11..12] ESRmpTms uint32 s  read-only (not exposed by APsystems)
//	body[13..14] ESDlyRemTms uint32 read-only (not implemented)
//	body[15]   V_SF                 read-only (scale factor)
//	body[16]   Hz_SF                read-only (scale factor)
func (esw *EnterServiceWriter) Apply(ctx context.Context, addrOffset uint16, regs []uint16) error {
	if esw == nil || esw.sender == nil {
		return fmt.Errorf("writes disabled or inv-driver not configured")
	}
	if esw.uid <= 1 {
		// Aggregate writes don't make sense for ESDlyTms — refuse rather
		// than silently broadcast.
		return fmt.Errorf("model 703 writes only valid on per-inverter unit IDs (2..N+1)")
	}
	idx := int(esw.uid) - 2
	if idx < 0 || idx >= len(esw.snap.Inverters) {
		return fmt.Errorf("unit ID %d not mapped to any inverter", esw.uid)
	}
	inv := esw.snap.Inverters[idx]
	uid := inv.UID

	// Only one writable register window: ESDlyTms at body offset 7..8 (uint32,
	// 2 regs). Require the write to cover EXACTLY this 2-register window —
	// partial writes (just the high word) would assemble a bogus value, and
	// writes that extend into adjacent fields hit regulatory boundaries.
	const esDlyOffset = 7
	const esDlyLen = 2
	if addrOffset != esDlyOffset || len(regs) != esDlyLen {
		return fmt.Errorf("model 703 only ESDlyTms is writable; expect write at offset=%d len=%d, got offset=%d len=%d",
			esDlyOffset, esDlyLen, addrOffset, len(regs))
	}

	hi := regs[0]
	lo := regs[1]
	val := uint32(hi)<<16 | uint32(lo)
	// ESDlyTms has no scale factor (uint32 in seconds, per the SunSpec spec).
	seconds := int64(val)
	if seconds < EnterServiceMinReconnectS || seconds > EnterServiceMaxReconnectS {
		return fmt.Errorf("ESDlyTms=%d outside [%d,%d] s",
			seconds, EnterServiceMinReconnectS, EnterServiceMaxReconnectS)
	}

	// Cross-parameter sanity: validate against the inverter's currently
	// loaded profile if we have it. The inverter's protection_parameters60code
	// row tells us what the firmware will accept. We don't currently store
	// per-parameter range_max from the menu; rely on the universal [10, 610].
	// (Recoded to use snapshot.Protection if it carries the loaded spec_type's
	//  range later — for now, the universal range is conservative.)
	prot, ok := esw.snap.Protection[uid]
	if ok && prot.Has["AG"] {
		// No structural cross-param check at this point; the value lives
		// alone in the firmware's reconnect-window logic. Just ensure we're
		// not writing a value that would silently regress on the next
		// daily 9 AM gridProfile push if AG/CG in singlegridPfile.conf differ.
		_ = prot
	}

	if err := dispatchProtection(ctx, esw.sender, inv, "grid_recovery_time", float64(seconds)); err != nil {
		return fmt.Errorf("queue grid_recovery_time=%d for %s: %w", seconds, uid, err)
	}
	return nil
}

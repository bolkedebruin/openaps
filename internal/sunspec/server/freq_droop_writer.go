package server

import (
	"context"
	"fmt"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
	"github.com/bolkedebruin/openaps/internal/sunspec/sunspec"
)

// FreqDroopWriter handles Modbus writes to SunSpec Model 711 (DERFreqDroop).
// Exposes the over-frequency droop curve:
//
//	body[0]    Ena                 WRITABLE (0/1 — drives Over_frequency_Watt_set enum)
//	body[1]    AdptCtlReq          WRITABLE (request counter, no-op server-side)
//	body[2]    AdptCtlRslt         read-only (server-published async result)
//	body[3]    NCtl                read-only
//	body[4..7] RvrtTms / RvrtRem   read-only
//	body[8]    RvrtCtl             read-only
//	body[9..11] Db/K/RspTms_SF     read-only
//	body[12..13] DbOf  uint32      WRITABLE — OF deadband from 50 Hz (cHz)
//	body[14..15] DbUf  uint32      read-only (UF-side not yet mapped)
//	body[16]   KOf                 WRITABLE — OF droop slope (%Pref/Hz, sf=-1)
//	body[17]   KUf                 read-only
//	body[18..19] RspTms uint32     WRITABLE — OF response delay (sec, sf=0)
//	body[20]   PMin                read-only (Model 711 doesn't define an
//	                               endpoint — droop bounded by inverter PMin)
//	body[21]   ReadOnly            read-only
//
// Long-name dispatch via Writer.SetProtectionParam:
//
//	Ena    → Over_frequency_Watt_set (CV: 13 AS/NZS, 14 other, 15 disabled)
//	DbOf   → Over_frequency_Watt_Start_set (Hz = 50.0 + DbOf/100)
//	KOf    → Over_Frequency_Watt_Slope_set (%Pref/Hz, written as DD/10)
//	RspTms → Over_frequency_Watt_Delay_Time_set (CG, whole seconds; the ECU
//	         writer applies the per-family scale: DS3 stores s+0.5, QS1
//	         multiplies by DAT_6dc28)
//
// Per EN 50549-1, the droop gradient (16.7..100 %P/Hz, corresponding to
// 12%..2% droop) is the standard parameter; APsystems' DD code holds it
// directly. We allow 10..150 %P/Hz to cover EU + AU/NZ markets (SMA's
// installer range is 10..100; AS/NZS-4777.2 typical ≈ 50; IEEE 1547-2018
// default ≈ 150 on 60 Hz). VDE-AR-N 4105 default is 40 (5% droop).
//
// Cross-parameter check: HzStart must sit ≥ 0.5 Hz below the inverter's
// currently-loaded OF1 trip (AK / OFFast) — guarantees the curve has
// frequency room to actuate before trip-disconnect. Validates against
// snapshot.Protection[uid].OFFast (firmware truth, not host cache).
//
// Note on the curve endpoint: Model 711 does NOT define a "HzEnd"
// parameter — the curve is parametric (deadband + slope), bounded by the
// inverter's PMin/PMax envelope, not by an end frequency. APsystems' CC
// code (Over_frequency_Watt_High_set, e.g. 52.0 Hz) belongs to the trip /
// ride-through model (Model 710), not the droop model. We do NOT touch
// CC from this writer — Model 710 stays the source of truth for trip
// thresholds.
type FreqDroopWriter struct {
	uid     uint8
	snap    source.Snapshot
	sender  frameSender
	tracker *WriteTracker
}

const (
	// Body offsets within Model 711 (excluding ID + L).
	freqDroopBodyEna      = 0
	freqDroopBodyAdptReq  = 1
	freqDroopBodyAdptRslt = 2
	freqDroopBodyDbOfHi   = 12
	freqDroopBodyDbOfLo   = 13
	freqDroopBodyKOf      = 16
	freqDroopBodyRspTmsHi = 18
	freqDroopBodyRspTmsLo = 19

	// OF-trip safety margin (Hz). HzStart (where curtailment begins) must
	// sit at least this far below the inverter's OF1 trip — guarantees the
	// curve has frequency room to actuate before trip-disconnect.
	freqDroopOFMarginHz = 0.5

	// KOf bounds in deci-percent (%P/Hz × 10). 100..1500 covers 10..150
	// %P/Hz — encompasses EN 50549-1 [16.7, 100], VDE-AR-N 4105 default 40,
	// SMA installer range [10, 100], and IEEE 1547-2018 / AS-NZS-4777.2
	// up to ~150.
	freqDroopMinKOf uint16 = 100
	freqDroopMaxKOf uint16 = 1500

	// Mode enum values for Over_frequency_Watt_set.
	freqDroopModeASNZS    = 13
	freqDroopModeOther    = 14
	freqDroopModeDisabled = 15
)

func (fdw *FreqDroopWriter) Apply(ctx context.Context, addrOffset uint16, regs []uint16) error {
	if fdw == nil || fdw.sender == nil {
		return fmt.Errorf("writes disabled or inv-driver not configured")
	}
	if fdw.uid <= 1 {
		return fmt.Errorf("Model 711 writes only valid on per-inverter unit IDs (2..N+1)")
	}
	idx := int(fdw.uid) - 2
	if idx < 0 || idx >= len(fdw.snap.Inverters) {
		return fmt.Errorf("unit ID %d not mapped to any inverter", fdw.uid)
	}
	inv := fdw.snap.Inverters[idx]
	uid := inv.UID
	prot := fdw.snap.Protection[uid]

	// dispatch routes a single (paramName, value) write: frequency
	// thresholds encode to an L2 frame and go through inv-driver,
	// env-gated / multi-frame params fall back to SQLite. Keeps each
	// writable field below a one-liner.
	dispatch := func(paramName string, value float64) error {
		return dispatchProtection(ctx, fdw.sender, inv, paramName, value)
	}

	// Determine which fields the write touches.
	wantsEna := writeTouches(addrOffset, regs, freqDroopBodyEna)
	wantsReq := writeTouches(addrOffset, regs, freqDroopBodyAdptReq)
	wantsDbOf := writeTouches(addrOffset, regs, freqDroopBodyDbOfHi) &&
		writeTouches(addrOffset, regs, freqDroopBodyDbOfLo)
	wantsKOf := writeTouches(addrOffset, regs, freqDroopBodyKOf)
	wantsRspTms := writeTouches(addrOffset, regs, freqDroopBodyRspTmsHi) &&
		writeTouches(addrOffset, regs, freqDroopBodyRspTmsLo)

	// Reject writes that touch any read-only register in the model body.
	for off := uint16(0); off < 22; off++ {
		if !writeTouches(addrOffset, regs, off) {
			continue
		}
		switch off {
		case freqDroopBodyEna, freqDroopBodyAdptReq,
			freqDroopBodyDbOfHi, freqDroopBodyDbOfLo, freqDroopBodyKOf,
			freqDroopBodyRspTmsHi, freqDroopBodyRspTmsLo:
			// writable
		case freqDroopBodyAdptRslt:
			return fmt.Errorf("AdptCtlRslt is server-published (read-only)")
		default:
			return fmt.Errorf("Model 711 body offset %d is read-only "+
				"(only Ena, AdptCtlReq, DbOf, KOf, RspTms are writable)", off)
		}
	}
	if !(wantsEna || wantsDbOf || wantsKOf || wantsRspTms) {
		if wantsReq {
			return nil // no-op counter bump
		}
		return fmt.Errorf("Model 711 write must touch at least one writable field (Ena/DbOf/KOf/RspTms)")
	}

	// Cross-parameter sanity uses the inverter's currently-loaded OF1.
	if !prot.Has["AK"] {
		return fmt.Errorf("cannot validate against inverter's OF1 trip (AK not yet read from %s)", uid)
	}

	if wantsDbOf {
		hi := readField(addrOffset, regs, freqDroopBodyDbOfHi)
		lo := readField(addrOffset, regs, freqDroopBodyDbOfLo)
		dbOf := uint32(hi)<<16 | uint32(lo)
		hzStart := 50.0 + float64(dbOf)/100.0
		if hzStart <= 50.0 {
			return fmt.Errorf("HzStart %.2f Hz must be > 50 Hz (above nominal)", hzStart)
		}
		if hzStart > prot.OFFast-freqDroopOFMarginHz {
			return fmt.Errorf("HzStart %.2f Hz must sit ≥ %.1f Hz below OF1 trip %.2f Hz (active on %s)",
				hzStart, freqDroopOFMarginHz, prot.OFFast, uid)
		}
		if err := dispatch("Over_frequency_Watt_Start_set", hzStart); err != nil {
			return fmt.Errorf("set Over_frequency_Watt_Start_set: %w", err)
		}
	}

	if wantsKOf {
		kOf := readField(addrOffset, regs, freqDroopBodyKOf)
		if kOf < freqDroopMinKOf || kOf > freqDroopMaxKOf {
			return fmt.Errorf("KOf=%d (%.1f %%P/Hz) outside [%d,%d] (%.1f-%.1f %%P/Hz)",
				kOf, float64(kOf)/10.0,
				freqDroopMinKOf, freqDroopMaxKOf,
				float64(freqDroopMinKOf)/10.0, float64(freqDroopMaxKOf)/10.0)
		}
		// SunSpec KOf is %Pref/Hz with sf=-1, so deci-percent. APsystems
		// DD is the same gradient in %Pref/Hz directly. KOf/10 → DD.
		slope := float64(kOf) / 10.0
		if err := dispatch("Over_Frequency_Watt_Slope_set", slope); err != nil {
			return fmt.Errorf("set Over_Frequency_Watt_Slope_set: %w", err)
		}
	}

	if wantsRspTms {
		hi := readField(addrOffset, regs, freqDroopBodyRspTmsHi)
		lo := readField(addrOffset, regs, freqDroopBodyRspTmsLo)
		// RspTms_SF=0 → wire value is whole seconds. The ECU-side writer
		// applies the per-family scale (DS3 stores s+0.5; QS1 multiplies by
		// DAT_6dc28); we just enqueue by long-form name.
		secs := float64(uint32(hi)<<16 | uint32(lo))
		if err := dispatch("Over_frequency_Watt_Delay_Time_set", secs); err != nil {
			return fmt.Errorf("set Over_frequency_Watt_Delay_Time_set: %w", err)
		}
	}

	// Track expected post-write state (keyed by 2-letter codes for
	// reconciliation against snap.Protection later) before we actually
	// translate the writes — this way Track sees the full set even if a
	// later SetProtectionParam fails partway through.
	expected := map[string]float64{}

	if wantsEna {
		ena := readField(addrOffset, regs, freqDroopBodyEna)
		// Map SunSpec Ena (0/1) to APsystems' enum (13 = AS/NZS, 14 = other, 15 = disabled).
		modeVal := freqDroopModeOther
		if ena == 0 {
			modeVal = freqDroopModeDisabled
		} else if int(prot.OFDroopMode) == freqDroopModeASNZS {
			modeVal = freqDroopModeASNZS // preserve current AS/NZS mode
		}
		expected["CV"] = float64(modeVal)
		if err := dispatch("Over_frequency_Watt_set", float64(modeVal)); err != nil {
			return fmt.Errorf("set Over_frequency_Watt_set: %w", err)
		}
	}

	// Re-derive the expected DC / DD values that were already written
	// above (we want them in `expected` for tracking, not just dispatched).
	if wantsDbOf {
		hi := readField(addrOffset, regs, freqDroopBodyDbOfHi)
		lo := readField(addrOffset, regs, freqDroopBodyDbOfLo)
		expected["DC"] = 50.0 + float64(uint32(hi)<<16|uint32(lo))/100.0
	}
	if wantsKOf {
		expected["DD"] = float64(readField(addrOffset, regs, freqDroopBodyKOf)) / 10.0
	}

	if fdw.tracker != nil && len(expected) > 0 {
		fdw.tracker.Track(fdw.uid, sunspec.FreqDroopModelID, expected)
	}
	return nil
}

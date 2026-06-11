package codec

import (
	"fmt"
	"math"
)

// DS3 protection-param wire scales. All produce a 16-bit big-endian
// value, truncated toward zero (the firmware's (int) cast).
const (
	ds3ProtFreqScale  = 100.0                // freq thresholds CB/CC/DH/DI: int(Hz*100).
	ds3ProtSlopeScale = 1023.0 * 16.0 * 0.01 // CF slope: int(v*163.68). NOT env-gated on DS3.
	// AG grid_recovery: int(s * line-cycles-per-sec). Use ×60 on 60 Hz
	// sites, else ×50. Encoded for 50 Hz here; a 60 Hz site would need ×60.
	ds3ProtRecoveryScale = 50.0
)

// isDS3ProtectionWrite reports whether a DS3 L2 (cmd,sub) is a protection
// write: the 0xAA opcode is shared with set-power, so any sub other than
// the set-power sub (0x27) is a protection param.
func isDS3ProtectionWrite(cmd, sub byte) bool {
	return cmd == CmdSetPowerDS3Unicast && sub != SubMaxPowerDS3
}

// ds3ProtParam binds a long-form param to its DS3 0xAA sub-byte and the
// real→wire transform.
type ds3ProtParam struct {
	sub    byte
	toWire func(float64) int64
}

func ds3ScaleHz(v float64) int64     { return int64(v * ds3ProtFreqScale) }
func ds3ScaleSlope(v float64) int64  { return int64(v * ds3ProtSlopeScale) }
func ds3RoundSec(v float64) int64    { return int64(v + 0.5) } // CG: firmware adds +0.5 before the cast
func ds3RecoverySec(v float64) int64 { return int64(v * ds3ProtRecoveryScale) }

// ds3ModeEnum maps the over-frequency-watt mode (Over_frequency_Watt_set;
// SunSpec sends 13=AS/NZS, 14=other, 15=disabled) to the DS3 sub-0x28
// value: 0xF→0, {3,4,0xD}→1, else→2.
func ds3ModeEnum(v float64) int64 {
	switch int(v) {
	case 0xF:
		return 0
	case 3, 4, 0xD:
		return 1
	default:
		return 2
	}
}

// ds3VoltScale is the DS3 voltage trip write scale: raw = int(V × ds3VoltWriteScale).
// Inversion of the read scale ds3GridVScale = 0.268 (1/0.268 = 3.73134...).
// Confirmed by golden capture: 264 V → 985 (int(264/0.268)=985); 253 V → 944; etc.
const ds3VoltWriteScale = 1.0 / ds3GridVScale // 3.73134...

// ds3ClearTimeScale is the DS3 trip-clearance time write scale: raw = int(s × 100).
// Inversion of dsSec read scale (raw × 0.01). Captures confirm:
// BC raw=10 (0.10 s), BB raw=2 (0.02 s), BE raw=5 (0.05 s), BD raw=120 (1.2 s),
// BI raw=30 (0.30 s), BH raw=16 (0.16 s), BK raw=30 (0.30 s), BJ raw=30 (0.30 s).
const ds3ClearTimeScale = 100.0

func ds3ScaleVolt(v float64) int64  { return int64(v * ds3VoltWriteScale) }
func ds3ScaleClear(v float64) int64 { return int64(v * ds3ClearTimeScale) }

// ds3ProtParams maps each writable DS3 protection param to its sub-byte
// and scaling. Over_frequency_Watt_Start_set (CA) has no DS3 branch in
// the firmware and so is intentionally absent.
//
// Voltage scale is int(V/0.268). Freq scale is int(Hz×100). Clear-time
// scale is int(s×100). All golden captures confirm 16-bit values
// (max captured: BD=120, well within 0xFFFF).
var ds3ProtParams = map[string]ds3ProtParam{
	// Freq-watt curve + droop (existing)
	"Over_frequency_Watt_Low_set":        {0x2B, ds3ScaleHz},     // CB
	"Over_frequency_Watt_High_set":       {0x2A, ds3ScaleHz},     // CC
	"Under_Frequency_Watt_Low_set":       {0x56, ds3ScaleHz},     // DH
	"Under_Frequency_Watt_High_set":      {0x55, ds3ScaleHz},     // DI
	"Over_Frequency_Watt_Slope_set":      {0x2D, ds3ScaleSlope},  // CF
	"Over_frequency_Watt_Delay_Time_set": {0x2E, ds3RoundSec},    // CG
	"Over_frequency_Watt_set":            {0x28, ds3ModeEnum},    // CV (mode enum)
	"grid_recovery_time":                 {0x25, ds3RecoverySec}, // AG

	// Voltage trip thresholds (over/under_voltage_fast/slow/slow_90/stage_2_90)
	// Scale: int(V/0.268), 16-bit.
	"over_voltage_fast":        {0x01, ds3ScaleVolt}, // AI — golden: 264 V → 985
	"under_voltage_fast":       {0x02, ds3ScaleVolt}, // AH — golden: 196 V → 731
	"over_voltage_slow":        {0x03, ds3ScaleVolt}, // AD — golden: 264 V → 985
	"under_voltage_stage_2":    {0x04, ds3ScaleVolt}, // AQ — golden: 184 V → 686
	"over_voltage_slow_90":     {0x05, ds3ScaleVolt}, // AY — golden: 253 V → 944
	"under_voltage_stage_2_90": {0x06, ds3ScaleVolt}, // AC — golden: 196 V → 731

	// Frequency trip thresholds (over/under_frequency_fast/slow)
	// Scale: int(Hz × 100), 16-bit.
	"over_frequency_fast":  {0x09, ds3ScaleHz}, // AK — golden: 52.0 Hz → 5200
	"under_frequency_fast": {0x0a, ds3ScaleHz}, // AJ — golden: 47.0 Hz → 4700
	"over_frequency_slow":  {0x0b, ds3ScaleHz}, // AF — golden: 52.0 Hz → 5200
	"under_frequency_slow": {0x0c, ds3ScaleHz}, // AE — golden: 47.5 Hz → 4750

	// Voltage trip clearance times (Over/Under_Voltage[1/2]_clearance_time)
	// Scale: int(s × 100), centiseconds, 16-bit.
	"Over_Voltage1_clearance_time":  {0x0f, ds3ScaleClear}, // BC — golden: 0.10 s → 10
	"Under_Voltage1_clearance_time": {0x10, ds3ScaleClear}, // BB — golden: 0.02 s → 2
	"Over_Voltage2_clearance_time":  {0x11, ds3ScaleClear}, // BE — golden: 0.05 s → 5
	"Under_Voltage2_clearance_time": {0x12, ds3ScaleClear}, // BD — golden: 1.2 s → 120

	// Frequency trip clearance times (Over/Under_Frequency[1/2]_clearance_time)
	// Scale: int(s × 100), centiseconds, 16-bit.
	"Over_Frequency1_clearance_time":  {0x15, ds3ScaleClear}, // BI — golden: 0.30 s → 30
	"Under_Frequency1_clearance_time": {0x16, ds3ScaleClear}, // BH — golden: 0.16 s → 16
	"Over_Frequency2_clearance_time":  {0x17, ds3ScaleClear}, // BK — golden: 0.30 s → 30
	"Under_Frequency2_clearance_time": {0x18, ds3ScaleClear}, // BJ — golden: 0.30 s → 30

	// 10-min average over-voltage (min10_Over_average_voltage / ac_600s)
	// Scale: int(V/ds3GridVScale), truncates, 16-bit. Golden capture: 253 V → 944.
	"min10_Over_average_voltage": {0x42, ds3ScaleVolt}, // AB

	// Reconnect-frequency thresholds (Reconnection_over/under_frequency).
	// DS3 dispatch: sub 0x23 (BQ) / 0x24 (BP), scale int(Hz × 100), byte_count 2.
	"Reconnection_over_frequency":  {0x23, ds3ScaleHz}, // BQ
	"Reconnection_under_frequency": {0x24, ds3ScaleHz}, // BP
}

// encodeProtectionDS3 builds a DS3 protection frame for one param using
// the supplied opcode (CmdSetPowerDS3Unicast for unicast, CmdSetPowerDS3Broadcast
// for broadcast). The value is right-aligned big-endian ending at byte [8]
// (16-bit max); the sub-byte at [4] distinguishes the param from set-power
// on the same opcode family. Always single-frame.
func encodeProtectionDS3(paramName string, value float64, opcode byte) ([][]byte, error) {
	pp, ok := ds3ProtParams[paramName]
	if !ok {
		return nil, fmt.Errorf("%w: %q on DS3", ErrUnsupportedProtectionParam, paramName)
	}
	wire := pp.toWire(value)
	if wire < 0 || wire > 0xFFFF {
		return nil, fmt.Errorf("set-protection: %q=%g → wire %d out of 16-bit range", paramName, value, wire)
	}
	return [][]byte{BuildL2Frame(opcode, []byte{pp.sub, 0, 0, byte(wire >> 8), byte(wire)})}, nil
}

// DS3 protection-READ scales + page field maps. Offsets are relative to
// the page-tag byte (reply[4]); the reply cmd is always 0xDD and the page
// is selected by reply[4]. Validated on live captures vs 60code (page A
// 16/16; page B freq-watt) — see security/protection-param-read.md. Volts
// ×0.268 rounded (truncation under-reads by 1 LSB); freq ×0.01; AG
// reconnect = raw×0.02 (50 Hz line-cycle count); AS identity.
func dsVolt(raw int) float64  { return float64(int(float64(raw)*ds3GridVScale + 0.5)) }
func dsFreq(raw int) float64  { return float64(raw) * ds3FreqScale }
func dsAG(raw int) float64    { return float64(raw) / ds3ProtRecoveryScale } // read = inverse of write ×50
func dsIdent(raw int) float64 { return float64(raw) }
func dsSec(raw int) float64   { return float64(raw) * ds3FreqScale } // clearance time, raw×0.01 s

// dsVoltAvg is the 10-min average over-voltage (AB): (int)(raw×0.268),
// truncated (the firmware drops the +0.5 the other volt reads use).
func dsVoltAvg(raw int) float64 { return float64(int(float64(raw) * ds3GridVScale)) }

// dsDroop is the over-frequency-watt droop slope (DD): ((raw×6000)/2208898)×100,
// the same factor the firmware applies on page A for AG. ~16.7 = EN 50549-1 max.
func dsDroop(raw int) float64 { return float64(raw) * ds3DroopNum / ds3DroopDen * 100.0 }

// dsMaxPower is the rated/limited output cap (DA), per panel: raw16 × 0.06027 W
// — the exact inverse of EncodeSetPowerDS3's round(W / 0.06027).
func dsMaxPower(raw int) float64 { return float64(raw) * setPowerScaleDS3Inv }

// dsMode is the over-frequency-watt mode (CV): a 1-byte enum the firmware
// remaps 0→0xF, 1→0xD, 2→0xE (else 0xF) into the 60code column.
func dsMode(raw int) float64 {
	switch raw {
	case 1:
		return 0xD
	case 2:
		return 0xE
	default:
		return 0xF
	}
}

const (
	ds3DroopNum = 6000.0    // DAT_5b3f0
	ds3DroopDen = 2208898.0 // DAT_5b3f8
)

var (
	ds3ReadPageA = []protReadField{
		{code: "AI", off: 0x01, width: 2, scale: dsVolt}, {code: "AH", off: 0x03, width: 2, scale: dsVolt}, {code: "AD", off: 0x05, width: 2, scale: dsVolt},
		{code: "AQ", off: 0x07, width: 2, scale: dsVolt}, {code: "AY", off: 0x09, width: 2, scale: dsVolt}, {code: "AC", off: 0x0b, width: 2, scale: dsVolt},
		{code: "AK", off: 0x11, width: 2, scale: dsFreq}, {code: "AJ", off: 0x13, width: 2, scale: dsFreq}, {code: "AF", off: 0x15, width: 2, scale: dsFreq}, {code: "AE", off: 0x17, width: 2, scale: dsFreq},
		// clearance times (seconds, raw×0.01): 16-bit volt-clear + 24-bit freq/stage-clear.
		{code: "BC", off: 0x1d, width: 2, scale: dsSec}, {code: "BB", off: 0x1f, width: 2, scale: dsSec},
		{code: "BE", off: 0x21, width: 2, scale: dsSec}, {code: "BD", off: 0x23, width: 2, scale: dsSec},
		{code: "BI", off: 0x29, width: 3, scale: dsSec}, {code: "BH", off: 0x2c, width: 3, scale: dsSec},
		{code: "BK", off: 0x4c, width: 3, scale: dsSec}, {code: "BJ", off: 0x4f, width: 3, scale: dsSec},
		{code: "AS", off: 0x39, width: 2, scale: dsIdent}, {code: "BO", off: 0x3d, width: 2, scale: dsVolt}, {code: "BN", off: 0x3f, width: 2, scale: dsVolt},
		{code: "BQ", off: 0x41, width: 2, scale: dsFreq}, {code: "BP", off: 0x43, width: 2, scale: dsFreq}, {code: "AG", off: 0x45, width: 2, scale: dsAG},
	}
	ds3ReadPageB = []protReadField{
		{code: "DA", off: 0x03, width: 2, scale: dsMaxPower}, // output cap, W/panel
		{code: "CV", off: 0x05, width: 1, scale: dsMode},
		{code: "DC", off: 0x06, width: 2, scale: dsFreq}, {code: "CC", off: 0x08, width: 2, scale: dsFreq}, {code: "CB", off: 0x0a, width: 2, scale: dsFreq},
		{code: "DD", off: 0x0c, width: 2, scale: dsDroop},
		{code: "AB", off: 0x31, width: 2, scale: dsVoltAvg},
		{code: "DI", off: 0x3f, width: 2, scale: dsFreq}, {code: "DH", off: 0x41, width: 2, scale: dsFreq},
	}
)

func init() {
	protReadTables = append(protReadTables, ds3ReadPageA, ds3ReadPageB)
}

// ds3ProtectionPage resolves a DS3 protection reply (cmd always 0xDD,
// page tag at byte[4]) to its field table; data base = byte[4].
func ds3ProtectionPage(f []byte, _ byte) ([]protReadField, int, bool) {
	if len(f) < 5 {
		return nil, 0, false
	}
	switch f[4] {
	case CmdProtReadPageA:
		return ds3ReadPageA, 4, true
	case CmdProtReadPageB:
		return ds3ReadPageB, 4, true
	}
	return nil, 0, false
}

// DS3 set-power scaling. Watts → on-wire register value is
// round(watts / setPowerScaleDS3Inv), saturated at setPowerRegMaxDS3.
const (
	setPowerScaleDS3Inv float64 = 0.06027
	setPowerRegMaxDS3   uint32  = 0x474C
)

// EncodeSetPowerDS3 builds the L2 frame to set max-power on the DS3
// family. broadcast=true emits opcode CmdSetPowerDS3Broadcast;
// broadcast=false emits CmdSetPowerDS3Unicast. SubMaxPowerDS3 selects
// the rated-power parameter; the 16-bit BE value occupies the last two
// body bytes.
func EncodeSetPowerDS3(panelWatts uint16, broadcast bool) ([]byte, error) {
	if err := checkPanelWatts(panelWatts); err != nil {
		return nil, err
	}
	val := uint32(math.Round(float64(panelWatts) / setPowerScaleDS3Inv))
	if val > setPowerRegMaxDS3 {
		val = setPowerRegMaxDS3
	}
	body := []byte{
		SubMaxPowerDS3,
		0x00,
		0x00,
		byte(val >> 8),
		byte(val & 0xFF),
	}
	cmd := CmdSetPowerDS3Unicast
	if broadcast {
		cmd = CmdSetPowerDS3Broadcast
	}
	return BuildL2Frame(cmd, body), nil
}

// DS3 reply payload layout, body offset 0 = byte right after the L2
// cmd 0xBB.
//
// Wire scale constants:
//
//	panelVScale  = 0.020   (panel V = raw16 × 0.020)
//	panelIScale  = 0.01172 (panel I = raw16 × 0.01172)
//	gridVScale   = 0.268   (grid V = raw16 × 0.268)
//	freqScale    = 0.010   (freq Hz = raw16 × 0.010)
//	tempScale    = 0.321   (some temperature, optional)
//	lifetime     = 1.674e-08 (per-byte → kWh, hard-coded)
const (
	ds3PanelVScale = 0.020
	ds3PanelIScale = 0.01172
	ds3GridVScale  = 0.268
	ds3FreqScale   = 0.010
	ds3Lifetime    = 1.674e-08
)

// DS3 reply payload layout (from resolvedata_DS3 @ 0x1f45c):
//
//	[0x0b..0x0f]  status / fault bitfield — see DS3Status
//	[0x10..0x11]  panel A V raw       × 0.020
//	[0x12..0x13]  panel B V raw       × 0.020
//	[0x14..0x15]  panel A I raw       × 0.01172
//	[0x16..0x17]  panel B I raw       × 0.01172
//	[0x18..0x19]  grid V raw          × 0.268
//	[0x1a..0x1b]  freq raw            × 0.010
//	[0x1c..0x1d]  inverter report counter (16-bit BE)
//	[0x1e..0x1f]  active power (u16, W) — stored at inv+0x1f4
//	[0x20..0x21]  reactive power (s16, VAR) — stored at inv+0x1e8
//	[0x28..0x2b]  panel A lifetime accumulator (4-byte BE)
//	[0x2c..0x2f]  panel B lifetime
func decodeDS3(body []byte, r *Reply) {
	if len(body) < 0x30 {
		return
	}
	r.GridV = fround(float64(be16(body, 0x18)) * ds3GridVScale)
	r.FreqHz = fround(float64(be16(body, 0x1a)) * ds3FreqScale)
	r.ReportSec = uint32(be16(body, 0x1c))

	for _, pp := range []struct {
		idx, vOff, iOff int
	}{
		{0, 0x10, 0x14}, // A
		{1, 0x12, 0x16}, // B
	} {
		v := float64(be16(body, pp.vOff)) * ds3PanelVScale
		i := float64(be16(body, pp.iOff)) * ds3PanelIScale
		r.Panels = append(r.Panels, Panel{
			Index: pp.idx,
			DCV:   fround(v),
			DCI:   fround(i),
			W:     fround(v * i),
		})
	}

	r.ActivePowerW = float64(be16(body, 0x1e))
	r.ReactivePower = float64(int16(be16(body, 0x20)))

	r.LifetimeRaw = []uint64{
		charsToUint(body, 0x28, 4),
		charsToUint(body, 0x2c, 4),
	}
	r.LifetimeScale = ds3Lifetime

	ds := decodeDS3Status(body)
	r.ExtendedStatus = ds
	r.Status = ds.Faults().InverterStatus()
}

// DS3Status holds the five raw status/fault bytes a DS3 reply ships
// at body[0x0b..0x10]. The firmware unpacks the same bytes into ~32
// individual slots but only derives semantic signals from a subset of
// those — those land in InverterStatus on the parent Reply, and the
// named-bit Faults() decoder below surfaces the per-flag semantics
// for the bits whose meaning is pinned by the Modbus packing.
type DS3Status struct {
	// Raw is body[0x0b..0x10] verbatim. Bit numbering: bit 0 = LSB.
	Raw [5]byte
}

// DS3Faults names the DS3 status bits whose meaning is confirmed by
// the firmware's Modbus packing (Modbus/SunSpec landing) or by
// debounce/aggregator placement. All bits are fault-active: '1' on
// the wire means an event/fault is present; '0' means healthy.
// Warning-bucket and legacy-slot bits are not surfaced here; consumers
// needing them read DS3Status.Raw.
type DS3Faults struct {
	GridRelayFault   bool // body[0xc] bit 1 → inv+0x29a (1=event; firmware also maps St 4↔6 and Evt1 bit 1)
	DCContactorFault bool // body[0xd] bit 7 → inv+0x299 → Evt1 bit 7
	DCBusFault       bool // body[0xd] bit 4 → inv+0x29e (1=event; counter inv+0x3f0 tracks fault-poll count, inv+0x3ec the healthy streak)
	DCGroundFault    bool // body[0xd] bit 3 → inv+0x298 → Evt1 bit 0

	IsoFaultA bool // body[0xe] bit 4 → inv+0x290 → Evt1 bit 6
	IsoFaultB bool // body[0xe] bit 6 → inv+0x292 → Evt1 bit 6

	ACOverVoltStage1  bool // body[0xe] bit 0 → inv+0x2a5 → Evt1 bit 15
	ACOverVoltStage2  bool // body[0xe] bit 2 → inv+0x2a7 → Evt1 bit 15
	ACUnderVoltStage1 bool // body[0xe] bit 1 → inv+0x2a6 → Evt1 bit 14
	ACUnderVoltStage2 bool // body[0xe] bit 3 → inv+0x2a8 → Evt1 bit 14

	OverFreqStage1  bool // body[0xf] bit 2 → inv+0x2a9 → Evt1 bit 13
	OverFreqStage2  bool // body[0xf] bit 4 → inv+0x2ab → Evt1 bit 13
	OverFreqAux     bool // body[0xf] bit 6 → inv+0x2ad → Evt1 bit 13
	OverFreqExtra   bool // body[0xf] bit 0 → inv+0x2b4 → Evt1 bit 13
	UnderFreqStage1 bool // body[0xf] bit 3 → inv+0x2aa → Evt1 bit 12
	UnderFreqStage2 bool // body[0xf] bit 5 → inv+0x2ac → Evt1 bit 12
	UnderFreqAux    bool // body[0xf] bit 7 → inv+0x2ae → Evt1 bit 12
	UnderFreqExtra  bool // body[0xf] bit 1 → inv+0x2b5 → Evt1 bit 12
}

// Faults decodes the named-meaning bits from the raw status bytes.
// Bits whose meaning is unclear (warning bucket, legacy slots) stay
// in Raw; consumers that need them read Raw directly.
func (s DS3Status) Faults() DS3Faults {
	bC, bD, bE, bF := s.Raw[1], s.Raw[2], s.Raw[3], s.Raw[4]
	return DS3Faults{
		GridRelayFault:    bC&(1<<1) != 0,
		DCContactorFault:  bD&(1<<7) != 0,
		DCBusFault:        bD&(1<<4) != 0,
		DCGroundFault:     bD&(1<<3) != 0,
		IsoFaultA:         bE&(1<<4) != 0,
		IsoFaultB:         bE&(1<<6) != 0,
		ACOverVoltStage1:  bE&(1<<0) != 0,
		ACOverVoltStage2:  bE&(1<<2) != 0,
		ACUnderVoltStage1: bE&(1<<1) != 0,
		ACUnderVoltStage2: bE&(1<<3) != 0,
		OverFreqStage1:    bF&(1<<2) != 0,
		OverFreqStage2:    bF&(1<<4) != 0,
		OverFreqAux:       bF&(1<<6) != 0,
		OverFreqExtra:     bF&(1<<0) != 0,
		UnderFreqStage1:   bF&(1<<3) != 0,
		UnderFreqStage2:   bF&(1<<5) != 0,
		UnderFreqAux:      bF&(1<<7) != 0,
		UnderFreqExtra:    bF&(1<<1) != 0,
	}
}

// ModbusStatus returns the SunSpec Model 103 (St, Evt1) register pair
// for a DS3 reply via the shared packModbusStatus helper. Comm, ZB-link,
// and over-temperature bits stay zero — DS3 has no body source for
// those.
//
// NOTE: the legacy St mapping (slot 0x29a=='1' → St=6) is suspected
// dead code; ecu-sunspec computes its own St via
// aggregateOperatingState. This helper is kept as a faithful firmware
// emulation for reference.
func (s DS3Status) ModbusStatus() (st, evt1 uint16) {
	f := s.Faults()
	return packModbusStatus(modbusStatusInputs{
		GridRelay:      f.GridRelayFault,
		DCContactor:    f.DCContactorFault,
		DCGround:       f.DCGroundFault,
		IsoAny:         f.IsoFaultA || f.IsoFaultB,
		ACOverVoltAny:  f.ACOverVoltStage1 || f.ACOverVoltStage2,
		ACUnderVoltAny: f.ACUnderVoltStage1 || f.ACUnderVoltStage2,
		OverFreqAny:    f.OverFreqStage1 || f.OverFreqStage2 || f.OverFreqAux || f.OverFreqExtra,
		UnderFreqAny:   f.UnderFreqStage1 || f.UnderFreqStage2 || f.UnderFreqAux || f.UnderFreqExtra,
	})
}

// decodeDS3Status returns the raw 5-byte block. The semantic
// aggregator booleans (InverterStatus) are derived from
// DS3Status.Faults().InverterStatus() rather than computed here so the
// bit→meaning mapping has one source of truth.
func decodeDS3Status(body []byte) DS3Status {
	var ds DS3Status
	if len(body) < 0x10 {
		return ds
	}
	copy(ds.Raw[:], body[0x0b:0x10])
	return ds
}

// InverterStatus reduces named DS3 fault bits to the firmware's five
// aggregator signals.
func (f DS3Faults) InverterStatus() InverterStatus {
	return InverterStatus{
		DCBusFault: f.DCBusFault,
		FaultA:     f.OverFreqStage1 || f.OverFreqStage2 || f.OverFreqAux || f.OverFreqExtra,
		FaultB:     f.UnderFreqStage1 || f.UnderFreqStage2 || f.UnderFreqAux || f.UnderFreqExtra,
		WarningA:   f.ACOverVoltStage1 || f.ACOverVoltStage2,
		WarningB:   f.ACUnderVoltStage1 || f.ACUnderVoltStage2,
	}
}

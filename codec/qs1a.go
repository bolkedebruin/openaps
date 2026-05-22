package codec

import "fmt"

// isQS1ProtectionWrite reports whether a QS1 L2 (cmd,sub) is a protection
// write: the 0x1C opcode is shared with set-power (any sub other than the
// set-power sub 0x8C is a protection param), and 0x5D (grid_recovery via
// the YC600 builder) is always protection.
func isQS1ProtectionWrite(cmd, sub byte) bool {
	return (cmd == CmdSetPowerQS1Unicast && sub != SubMaxPowerQS1) || cmd == CmdGridRecoveryQS1
}

// qs1ProtFreqSubs maps the long-form param name to its QS1 0x1C sub-byte
// for the single-frame frequency-threshold protection params.
var qs1ProtFreqSubs = map[string]byte{
	"Over_frequency_Watt_Start_set": 0x62, // CA
	"Over_frequency_Watt_Low_set":   0x68, // CB
	"Over_frequency_Watt_High_set":  0x65, // CC
	"Under_Frequency_Watt_Low_set":  0xA1, // DH
	"Under_Frequency_Watt_High_set": 0xA4, // DI
}

// protRecoveryScaleQS1 is the QS1 grid_recovery_time wire scale:
// int(s*100), 16-bit. Uses the YC600 builder (opcode 0x5D), not 0x1C.
const protRecoveryScaleQS1 = 100.0

// QS1 protection-READ scales + page field maps. Offsets are relative to
// the cmd byte (reply[3]); the cmd byte itself is the page (0xDE/0xD9).
// QS1A returns NO 0xDD page-A reply, so the volt/freq trips aren't
// retrievable this way — only reconnect (0xDE) + DH/DI (0xD9), validated
// on live captures vs 60code. Volts = (raw/1.332)/4 rounded; freq =
// 50_000_000/raw (24-bit BE).
// qsProtVoltDenom is the QS1 protection-read voltage divisor (DAT_4fff0):
// volts = (raw/1.332)/4. Distinct from the telemetry grid-V denom (1.32).
const qsProtVoltDenom = 1.332

func qsVolt(raw int) float64 {
	if raw == 0 {
		return 0
	}
	return float64(int((float64(raw)/qsProtVoltDenom)/4 + 0.5))
}

func qsFreq(raw int) float64 {
	if raw == 0 {
		return 0
	}
	return qs1aFreqDivBy / float64(raw)
}

var (
	qs1ReadPageB = []protReadField{
		{"BN", 0x01, 2, qsVolt}, {"BO", 0x03, 2, qsVolt}, {"BP", 0x05, 3, qsFreq}, {"BQ", 0x08, 3, qsFreq},
	}
	qs1ReadPageC = []protReadField{
		{"DH", 0x0c, 3, qsFreq}, {"DI", 0x0f, 3, qsFreq},
	}
)

// qs1ProtectionPage resolves a QS1 protection reply to its field table;
// the cmd byte is the page and data base = byte[3].
func qs1ProtectionPage(_ []byte, cmd byte) ([]protReadField, int, bool) {
	switch cmd {
	case CmdProtReadPageB:
		return qs1ReadPageB, 3, true
	case CmdProtReadPageC:
		return qs1ReadPageC, 3, true
	}
	return nil, 0, false
}

// encodeProtectionQS1A builds the QS1 (0x1C) unicast frame(s) for one
// protection param. Frequency thresholds are single 0x1C frames with
// byte_count 3 (24-bit value left-aligned at [6..8]); the over-frequency
// mode enum is a 2-3 frame 0x1C sequence; grid_recovery_time uses the
// YC600 builder (opcode 0x5D). The opcode shares the family SET byte with
// set-power; the sub-byte at [4] distinguishes.
func encodeProtectionQS1A(paramName string, value float64) ([][]byte, error) {
	if sub, ok := qs1ProtFreqSubs[paramName]; ok {
		wire := int64(qs1aFreqDivBy / value) // wire = int(50e6/Hz), truncate toward zero
		if wire < 0 || wire > 0xFFFFFF {
			return nil, fmt.Errorf("set-protection: %q=%g Hz → wire %d out of 24-bit range", paramName, value, wire)
		}
		return [][]byte{BuildL2Frame(CmdSetPowerQS1Unicast, []byte{sub, 3, byte(wire >> 16), byte(wire >> 8), byte(wire)})}, nil
	}
	switch paramName {
	case "Over_frequency_Watt_set": // CV — multi-frame mode enum
		return qs1ModeFrames(value), nil
	case "grid_recovery_time": // AG — YC600 0x5D builder, int(s*100)
		wire := int64(value * protRecoveryScaleQS1)
		if wire < 0 || wire > 0xFFFF {
			return nil, fmt.Errorf("set-protection: grid_recovery_time=%g → wire %d out of 16-bit range", value, wire)
		}
		// 0x5D body: value left-aligned big-endian at [4..5], rest zero.
		return [][]byte{BuildL2Frame(CmdGridRecoveryQS1, []byte{byte(wire >> 8), byte(wire), 0, 0, 0})}, nil
	}
	return nil, fmt.Errorf("%w: %q on QS1A", ErrUnsupportedProtectionParam, paramName)
}

// qs1ModeFrames builds the QS1 over-frequency-watt mode enum sequence
// (Over_frequency_Watt_set, sub 0x5F + 0x99), replicating main.exe's
// branch+remap: 0xF (disabled) → 5F=0x30, 5F=0x0F, 99=0; {3,4,0xD}
// (AS/NZS, 0xD→4) → 5F=0x10, 5F=v, 99=1; else (e.g. 0xE→1) → 5F=0x20,
// 5F=v. Each is a 0x1C frame with byte_count 1 (value at [6]).
func qs1ModeFrames(value float64) [][]byte {
	mode := int(value)
	frame := func(sub, v byte) []byte {
		return BuildL2Frame(CmdSetPowerQS1Unicast, []byte{sub, 1, v, 0, 0})
	}
	switch mode {
	case 0xF:
		return [][]byte{frame(0x5F, 0x30), frame(0x5F, 0x0F), frame(0x99, 0x00)}
	case 3, 4, 0xD:
		v := byte(mode)
		if mode == 0xD {
			v = 4
		}
		return [][]byte{frame(0x5F, 0x10), frame(0x5F, v), frame(0x99, 0x01)}
	default:
		v := byte(mode)
		if mode == 0xE {
			v = 1
		}
		return [][]byte{frame(0x5F, 0x20), frame(0x5F, v)}
	}
}

// setPowerScaleDSP is the QS1 / QS1A / YC600-family watts-to-register
// multiplier used by EncodeSetPowerQS1A and EncodeSetPowerC3.
//
//	reg = (panelWatts * setPowerScaleDSP) >> setPowerScaleDSPShift
const (
	setPowerScaleDSP      uint32 = 0x1CE3
	setPowerScaleDSPShift        = 8
	// setPowerQS1ValueLen is the body byte that announces a 2-byte value
	// follows under sub-code SubMaxPowerQS1.
	setPowerQS1ValueLen byte = 0x02
)

// EncodeSetPowerQS1A builds the L2 frame to set max-power on the
// QS1 / QS1A family (model codes 0x08 / 0x18). broadcast=true emits
// CmdSetPowerQS1Broadcast; broadcast=false emits CmdSetPowerQS1Unicast.
// SubMaxPowerQS1 selects the parameter; value length is 2.
func EncodeSetPowerQS1A(panelWatts uint16, broadcast bool) ([]byte, error) {
	if err := checkPanelWatts(panelWatts); err != nil {
		return nil, err
	}
	val := (uint32(panelWatts) * setPowerScaleDSP) >> setPowerScaleDSPShift
	body := []byte{
		SubMaxPowerQS1,
		setPowerQS1ValueLen,
		byte(val >> 8),
		byte(val & 0xFF),
		0x00,
	}
	cmd := CmdSetPowerQS1Unicast
	if broadcast {
		cmd = CmdSetPowerQS1Broadcast
	}
	return BuildL2Frame(cmd, body), nil
}

// QS1A reply payload layout, body offset 0 = byte right after the
// L2 cmd 0xB1 (i.e. main.exe's `param_1` to resolvedata_60_1200,
// which is l2_frame[4..]).
//
// Constants from main.exe .rodata (Ghidra dumps at 0x1dda8..0x1de8
// and 0x1ea00):
//
//	busPFBase    = 2.85   @ 0x1dda8  (bus V denom)
//	busScale     = 3300.0 @ 0x1ddb0  (bus V numer)
//	busDenom     = 4092.0 @ 0x1ddb8  (bus V denom inner)
//	busOffset    = 757.0  @ 0x1ddc0  (bus V offset)
//	freqDivBy    = 5e7    @ 0x1ddc8  (Hz = 5e7 / raw_3byte)
//	panelVMax    = 82.5   @ 0x1ddd0  (panel V scale, 12-bit ADC)
//	panelIMax    = 27.5   @ 0x1ddd8  (panel I scale, 12-bit ADC)
//	adc12bit     = 4096.0 @ 0x1dde0
//	gridVDenom   = 1.32   @ 0x1dde8  (V_grid = raw16 / 1.32 / 4)
//	efficiency   = 0.95   @ 0x1ea00  (active power scale)
//	lifetime     = 3.756e-11           (per-byte → kWh, in main.exe code)
const (
	qs1aBusScale  = 3300.0
	qs1aBusDenom  = 4092.0
	qs1aBusOffset = 757.0
	qs1aBusPFBase = 2.85
	qs1aFreqDivBy = 50_000_000.0 // QS1 freq raw↔Hz dividend; also the protection-encode dividend (DAT_6dbf8/6f5e8)
	qs1aPanelVMax = 82.5
	qs1aPanelIMax = 27.5
	qs1aADC12bit  = 4096.0
	qs1aGridDenom = 1.32
	qs1aEfficien  = 0.95
	qs1aLifetime  = 3.756e-11
)

// decodeQS1A populates r with QS1A telemetry from body. Body length
// is typically 0x52 = 82 bytes for QS1A (88-byte total reply minus
// 4 L2 SOF/type/cmd minus 2 chk minus 2 FE FE).
//
// Field map (offsets into body):
//
//	[0..1]   internal DC bus voltage (16-bit ADC) → V via the busPF formula.
//	         Stored at inv+0x4c/0x1d0 by main.exe; printed as "bus_vol".
//	[2..4]   grid frequency (24-bit) → Hz = 5e7 / raw
//	[6..8]   panel D 3-byte packed (12-bit V + 12-bit I)
//	[9..11]  panel C 3-byte packed
//	[12..14] panel B 3-byte packed
//	[15..17] panel A 3-byte packed
//	[0x12..0x13]  AC grid voltage (16-bit) → V = raw / 1.32 / 4
//	[0x14..0x15]  inverter report counter (16-bit; cumulative — diff
//	              between polls gives sample window seconds)
//	[0x17..0x1a]  status bit flags
//	[0x1b..0x1f]  panel A lifetime accumulator (5-byte BE)
//	[0x20..0x24]  panel B lifetime
//	[0x25..0x29]  panel C lifetime
//	[0x2a..0x2e]  panel D lifetime
//	[0x2f]   power-factor encoding (resolvedata sets cos/sin from this)
//
// The 3-byte panel packing:
//
//	byte[off+0]    = current[7:0]
//	byte[off+1]    = (voltage[3:0]<<4) | current[11:8]
//	byte[off+2]    = voltage[11:4]
//	V = packed × 82.5 / 4096,  I = packed × 27.5 / 4096
func decodeQS1A(body []byte, r *Reply) {
	r.BusV = fround(qs1aBusV(be16(body, 0)))
	r.GridV = fround(qs1aGridV(be16(body, 0x12)))
	r.FreqHz = fround(qs1aFreqHz(be24(body, 2)))
	r.ReportSec = uint32(be16(body, 0x14))

	// Panels: payload byte order is D, C, B, A — invert so r.Panels
	// is indexed A=0, B=1, C=2, D=3 to match the 12-character UID
	// suffix labels users see (704000006835A/B/C/D).
	for _, pp := range []struct {
		idx, off int
	}{
		{0, 15}, // A — bytes 15..17
		{1, 12}, // B — bytes 12..14
		{2, 9},  // C — bytes 9..11
		{3, 6},  // D — bytes 6..8
	} {
		v := qs1aPanelV(body, pp.off)
		i := qs1aPanelI(body, pp.off)
		r.Panels = append(r.Panels, Panel{
			Index: pp.idx,
			DCV:   fround(v),
			DCI:   fround(i),
			W:     fround(v * i),
		})
	}

	// Active power total = sum of panel watts × 0.95 efficiency.
	// main.exe applies the same 0.95 (DAT_0001ea00) before storing
	// at param_2 + 0x1f4. Bypassing main.exe's MAXOP and DC-vs-energy
	// tie-break logic (those are for ECU power-limiting, not raw
	// telemetry).
	var w float64
	for _, p := range r.Panels {
		w += p.W
	}
	r.ActivePowerW = fround(w * qs1aEfficien)

	// Lifetime accumulators per panel (5 bytes BE each), payload
	// offset order is A, B, C, D — matches r.Panels order above.
	r.LifetimeRaw = []uint64{
		charsToUint(body, 0x1b, 5),
		charsToUint(body, 0x20, 5),
		charsToUint(body, 0x25, 5),
		charsToUint(body, 0x2a, 5),
	}
	r.LifetimeScale = qs1aLifetime

	r.QS1AStatus = decodeQS1AStatus(body)
	r.Status = r.QS1AStatus.Faults().InverterStatus()
}

// QS1AStatus holds the raw status bytes a QS1A reply spreads across
// the payload. main.exe (resolvedata_60_1200 @ 0x1dbec) unpacks them
// into the same inverter-struct slots DS3 uses (0x288..0x2db) and an
// additional grid-side block at inv+0x3cc..0x3d3 driven by Grid.
//
// Source layout (in ascending body offset order):
//
//	Main  = body[0x17..0x1a]  — primary fault/warning bits
//	Extra = body[0x34]        — single byte copied verbatim to
//	                            inv+0x3d9; semantics undocumented.
//	Grid  = body[0x38..0x39]  — grid-side flag table; main.exe also
//	                            steps per-bit "absence" counters at
//	                            inv+0x3dc/0x3e0/0x3e4/0x3e8.
type QS1AStatus struct {
	Main  [4]byte
	Grid  [2]byte
	Extra byte
}

// decodeQS1AStatus returns the raw status bytes plus the
// family-agnostic aggregator booleans. Source-byte mapping (verified
// against resolvedata_60_1200):
//
//	DCBusFault  = body[0x18] bit 4                          (inv+0x29e)
//	WarningA    = body[0x1a] bits 0,2                       (inv+0x2a5,0x2a7)
//	WarningB    = body[0x1a] bits 1,3                       (inv+0x2a6,0x2a8)
//	FaultA      = body[0x1a] bits 4,6 | body[0x19] bit 0    (inv+0x2a9,0x2ab,0x2ad,
//	              | body[0x17] bit 5                              0x2b4)
//	FaultB      = body[0x1a] bits 5,7 | body[0x19] bit 1    (inv+0x2aa,0x2ac,0x2ae,
//	              | body[0x17] bit 6                              0x2b5)
//
// decodeQS1AStatus returns the raw status bytes. The semantic
// aggregator booleans (InverterStatus) are derived from
// QS1AStatus.Faults().InverterStatus() so the bit→meaning mapping has
// one source of truth.
func decodeQS1AStatus(body []byte) QS1AStatus {
	var qs QS1AStatus
	if len(body) < 0x3a {
		return qs
	}
	copy(qs.Main[:], body[0x17:0x1b])
	copy(qs.Grid[:], body[0x38:0x3a])
	qs.Extra = body[0x34]
	return qs
}

// InverterStatus reduces named QS1A fault bits to the firmware's five
// aggregator signals (resolvedata_60_1200's slot-walking aggregator
// at inv+0x3ec / 0x3f0 / 0x3f4 / 0x3f8 / 0x3fc / 0x400).
func (f QS1AFaults) InverterStatus() InverterStatus {
	return InverterStatus{
		DCBusFault: f.DCBusFault,
		FaultA:     f.OverFreqFast || f.OverFreqSlow || f.OverFreqExtra || f.OverFreqRMS,
		FaultB:     f.UnderFreqFast || f.UnderFreqSlow || f.UnderFreqExtra || f.UnderFreqRMS,
		WarningA:   f.ACOverVoltFast || f.ACOverVoltSlow,
		WarningB:   f.ACUnderVoltFast || f.ACUnderVoltSlow,
	}
}

// QS1AFaults names the QS1A status bits whose meaning is pinned by
// main.exe's Modbus packing (update_modbus_status @ 0x96570) or by
// debounce/aggregator placement. All bits are fault-active: '1' on
// the wire means an event/fault is present; '0' means healthy.
// Verified via qs1200_60_status @ 0x297d8 (gates the cloud-upload
// alarm-pending flag on these slots being '1'). Bits with no named
// semantic stay reachable via QS1AStatus.Main/Grid/Extra.
//
// QS1A reports a strictly richer Evt1 surface than DS3: isolation is
// split per-channel (A/B/C/D), and over-temperature has a body bit
// where DS3 has none. Conversely, QS1A's OF/UF cluster reads fewer
// off-body slots, so Evt1 OF/UF here reflects only what body[0x17,
// 0x19, 0x1a] supplies.
type QS1AFaults struct {
	GridRelayFault   bool // body[0x17] bit 2 → inv+0x29a (1=event; firmware also maps St 4↔6 and Evt1 bit 1)
	DCContactorFault bool // body[0x17] bit 1 → inv+0x299 → Evt1 bit 7
	DCGroundFault    bool // body[0x17] bit 3 → inv+0x298 → Evt1 bit 0
	DCBusFault       bool // body[0x18] bit 4 → inv+0x29e (1=event; counter inv+0x3f0 tracks fault-poll count, inv+0x3ec the healthy streak)
	CommFault        bool // body[0x17] bit 4 → inv+0x29b → Evt1 bits 5 + 3
	OverTemperature  bool // body[0x18] bit 2 → inv+0x2b3 → Evt1 bit 1

	IsoFaultA bool // body[0x19] bit 2 → inv+0x290 → Evt1 bit 6
	IsoFaultB bool // body[0x19] bit 4 → inv+0x292 → Evt1 bit 6
	IsoFaultC bool // body[0x18] bit 0 → inv+0x2b1 → Evt1 bit 6
	IsoFaultD bool // body[0x19] bit 6 → inv+0x2af → Evt1 bit 6

	ACOverVoltFast  bool // body[0x1a] bit 2 → inv+0x2a7 → Evt1 bit 15
	ACOverVoltSlow  bool // body[0x1a] bit 0 → inv+0x2a5 → Evt1 bit 15
	ACUnderVoltFast bool // body[0x1a] bit 3 → inv+0x2a8 → Evt1 bit 14
	ACUnderVoltSlow bool // body[0x1a] bit 1 → inv+0x2a6 → Evt1 bit 14

	OverFreqFast   bool // body[0x1a] bit 6 → inv+0x2ab → Evt1 bit 13
	OverFreqSlow   bool // body[0x1a] bit 4 → inv+0x2a9 → Evt1 bit 13
	OverFreqExtra  bool // body[0x19] bit 0 → inv+0x2ad → Evt1 bit 13
	OverFreqRMS    bool // body[0x17] bit 5 → inv+0x2b4 → Evt1 bit 13 (10-min RMS)
	UnderFreqFast  bool // body[0x1a] bit 7 → inv+0x2ac → Evt1 bit 12
	UnderFreqSlow  bool // body[0x1a] bit 5 → inv+0x2aa → Evt1 bit 12
	UnderFreqExtra bool // body[0x19] bit 1 → inv+0x2ae → Evt1 bit 12
	UnderFreqRMS   bool // body[0x17] bit 6 → inv+0x2b5 → Evt1 bit 12 (10-min RMS)

	// ZBLink{A,B} feed Evt1 bit 4 via inv+0x3cc/0x3cd. Polarity
	// reflects the firmware verbatim — main.exe ORs the slot value
	// into Evt1 bit 4 regardless of SunSpec's GRID_DISCONNECT
	// convention. Same slots also drive the per-channel absence
	// counters at inv+0x3dc/0x3e0.
	ZBLinkA bool // body[0x39] bit 0 → inv+0x3cc → Evt1 bit 4
	ZBLinkB bool // body[0x39] bit 1 → inv+0x3cd → Evt1 bit 4
}

// Faults decodes the named-meaning bits from the QS1A status bytes.
// Bits without a confirmed semantic (warning bucket, reserved, raw
// "Extra" byte) stay in Main/Grid/Extra.
func (s QS1AStatus) Faults() QS1AFaults {
	b17, b18, b19, b1a := s.Main[0], s.Main[1], s.Main[2], s.Main[3]
	b39 := s.Grid[1]
	return QS1AFaults{
		GridRelayFault:   b17&(1<<2) != 0,
		DCContactorFault: b17&(1<<1) != 0,
		DCGroundFault:    b17&(1<<3) != 0,
		DCBusFault:       b18&(1<<4) != 0,
		CommFault:        b17&(1<<4) != 0,
		OverTemperature:  b18&(1<<2) != 0,
		IsoFaultA:        b19&(1<<2) != 0,
		IsoFaultB:        b19&(1<<4) != 0,
		IsoFaultC:        b18&(1<<0) != 0,
		IsoFaultD:        b19&(1<<6) != 0,
		ACOverVoltFast:   b1a&(1<<2) != 0,
		ACOverVoltSlow:   b1a&(1<<0) != 0,
		ACUnderVoltFast:  b1a&(1<<3) != 0,
		ACUnderVoltSlow:  b1a&(1<<1) != 0,
		OverFreqFast:     b1a&(1<<6) != 0,
		OverFreqSlow:     b1a&(1<<4) != 0,
		OverFreqExtra:    b19&(1<<0) != 0,
		OverFreqRMS:      b17&(1<<5) != 0,
		UnderFreqFast:    b1a&(1<<7) != 0,
		UnderFreqSlow:    b1a&(1<<5) != 0,
		UnderFreqExtra:   b19&(1<<1) != 0,
		UnderFreqRMS:     b17&(1<<6) != 0,
		ZBLinkA:          b39&(1<<0) != 0,
		ZBLinkB:          b39&(1<<1) != 0,
	}
}

// ModbusStatus reproduces main.exe's update_modbus_status @ 0x96570
// for a QS1A reply via the shared packModbusStatus helper. QS1A
// populates strictly more Evt1 bits than DS3 — isolation (bit 14,
// per-channel A/B/C/D), comm (bits 13 + 11), ZB-link (bit 12), and
// over-temperature (bit 1) all have body sources for QS1A and zero
// body sources for DS3.
func (s QS1AStatus) ModbusStatus() (st, evt1 uint16) {
	f := s.Faults()
	return packModbusStatus(modbusStatusInputs{
		GridRelay:       f.GridRelayFault,
		DCContactor:     f.DCContactorFault,
		DCGround:        f.DCGroundFault,
		IsoAny:          f.IsoFaultA || f.IsoFaultB || f.IsoFaultC || f.IsoFaultD,
		CommFault:       f.CommFault,
		ZBLink:          f.ZBLinkA || f.ZBLinkB,
		OverTemperature: f.OverTemperature,
		ACOverVoltAny:   f.ACOverVoltFast || f.ACOverVoltSlow,
		ACUnderVoltAny:  f.ACUnderVoltFast || f.ACUnderVoltSlow,
		OverFreqAny:     f.OverFreqFast || f.OverFreqSlow || f.OverFreqExtra || f.OverFreqRMS,
		UnderFreqAny:    f.UnderFreqFast || f.UnderFreqSlow || f.UnderFreqExtra || f.UnderFreqRMS,
	})
}

// qs1aBusV applies main.exe's internal-DC-link formula:
//
//	V = (raw * 3300/4092 - 757) / 2.85
//
// This is the value main.exe prints as "bus_vol".
func qs1aBusV(raw uint16) float64 {
	return (float64(raw)*qs1aBusScale/qs1aBusDenom - qs1aBusOffset) / qs1aBusPFBase
}

// qs1aGridV applies main.exe's grid voltage formula:
//
//	V = (raw_16bit / 1.32) / 4
func qs1aGridV(raw uint16) float64 {
	return (float64(raw) / qs1aGridDenom) / 4.0
}

// qs1aFreqHz applies main.exe's formula:
//
//	Hz = 5e7 / raw3byte
func qs1aFreqHz(raw uint32) float64 {
	if raw == 0 {
		return 0
	}
	return qs1aFreqDivBy / float64(raw)
}

// qs1aPanelV / qs1aPanelI unpack a 12-bit ADC value out of the
// 3-byte packed format used per panel:
//
//	byte[off+0]    = current[7:0]
//	byte[off+1]    = (voltage[3:0]<<4) | current[11:8]
//	byte[off+2]    = voltage[11:4]
//
// Then voltage = packed * 82.5 / 4096, current = packed * 27.5 / 4096.
func qs1aPanelV(body []byte, off int) float64 {
	if off+2 >= len(body) {
		return 0
	}
	raw := uint16(body[off+2])<<4 | uint16(body[off+1]>>4)
	return float64(raw) * qs1aPanelVMax / qs1aADC12bit
}

func qs1aPanelI(body []byte, off int) float64 {
	if off+1 >= len(body) {
		return 0
	}
	raw := (uint16(body[off+1]&0x0F) << 8) | uint16(body[off])
	return float64(raw) * qs1aPanelIMax / qs1aADC12bit
}

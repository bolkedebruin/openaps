package codec

import (
	"fmt"
	"math"
)

// protFreqScaleDS3 is the DS3 frequency-threshold wire scale (main.exe
// .rodata DAT_6dc28 etc.): wire = int(Hz * 100), 16-bit big-endian,
// truncated toward zero (no +0.5).
const protFreqScaleDS3 = 100.0

// ds3ProtFreqSubs maps the long-form param name to its DS3 0xAA sub-byte
// for the single-frame frequency-threshold protection params.
// Over_frequency_Watt_Start_set (CA) has no DS3 branch in main.exe.
var ds3ProtFreqSubs = map[string]byte{
	"Over_frequency_Watt_Low_set":   0x2B, // CB
	"Over_frequency_Watt_High_set":  0x2A, // CC
	"Under_Frequency_Watt_Low_set":  0x56, // DH
	"Under_Frequency_Watt_High_set": 0x55, // DI
}

// encodeProtectionDS3 builds the DS3 (0xAA) unicast frame for one
// frequency-threshold param. The 16-bit value is right-aligned at bytes
// [7..8] (byte_count 2); the opcode is the DS3 family SET opcode, shared
// with set-power and distinguished by the sub-byte at [4].
func encodeProtectionDS3(paramName string, hz float64) ([]byte, error) {
	sub, ok := ds3ProtFreqSubs[paramName]
	if !ok {
		return nil, fmt.Errorf("%w: %q on DS3", ErrUnsupportedProtectionParam, paramName)
	}
	wire := int64(hz * protFreqScaleDS3) // truncate toward zero
	if wire < 0 || wire > 0xFFFF {
		return nil, fmt.Errorf("set-protection: %q=%g Hz → wire %d out of 16-bit range", paramName, hz, wire)
	}
	return BuildL2Frame(CmdSetPowerDS3Unicast, []byte{sub, 0, 0, byte(wire >> 8), byte(wire)}), nil
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
// cmd 0xBB (= main.exe `param_1` for resolvedata_DS3, which is
// l2_frame[4..]).
//
// Magic constants from main.exe .rodata (read via Ghidra at
// 0x1f770..0x1f790):
//
//	panelVScale  = 0.020   @ 0x1f770  (panel V = raw16 × 0.020)
//	panelIScale  = 0.01172 @ 0x1f778  (panel I = raw16 × 0.01172)
//	gridVScale   = 0.268   @ 0x1f780  (grid V = raw16 × 0.268)
//	freqScale    = 0.010   @ 0x1f788  (freq Hz = raw16 × 0.010)
//	tempScale    = 0.321   @ 0x1f790  (some temperature, optional)
//	lifetime     = 1.674e-08            (per-byte → kWh, hard-coded)
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

	r.DS3Status = decodeDS3Status(body)
	r.Status = r.DS3Status.Faults().InverterStatus()
}

// DS3Status holds the five raw status/fault bytes a DS3 reply ships
// at body[0x0b..0x10]. main.exe (resolvedata_DS3 @ 0x1f45c) unpacks
// the same bytes into ~32 individual ASCII '0'/'1' slots in its
// inverter struct (0x288..0x2db, 0x3d0..0x3d8); the firmware only
// derives semantic signals from a subset of those slots — those
// land in InverterStatus on the parent Reply, and the named-bit
// Faults() decoder below surfaces the per-flag semantics for the
// bits whose meaning is pinned by main.exe's Modbus packing
// (update_modbus_status @ 0x96570).
type DS3Status struct {
	// Raw is body[0x0b..0x10] verbatim. Bit numbering: bit 0 = LSB.
	Raw [5]byte
}

// DS3Faults names the DS3 status bits whose meaning is confirmed by
// main.exe's update_modbus_status (Modbus/SunSpec landing) or by
// debounce/aggregator placement. All bits are fault-active: '1' on
// the wire means an event/fault is present; '0' means healthy.
// Verified via DS3_DS3D_status @ 0x290f8 (gates the cloud-upload
// alarm-pending flag on these slots being '1'). Warning-bucket and
// legacy-slot bits are not surfaced here; consumers needing them
// read DS3Status.Raw.
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

// ModbusStatus reproduces main.exe's update_modbus_status @ 0x96570
// for a DS3 reply. Returns the SunSpec Model 103 (St, Evt1) register
// pair via the shared packModbusStatus helper. Comm, ZB-link, and
// over-temperature bits stay zero — DS3 has no body source for those.
//
// NOTE: the firmware's St mapping (slot 0x29a=='1' → St=6) is
// suspected dead code; ecu-sunspec computes its own St via
// aggregateOperatingState. This helper is kept as a faithful
// firmware emulation for reference.
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
// aggregator signals. Mirrors main.exe's slot-walking aggregator in
// resolvedata_DS3 (inv+0x3ec / 0x3f0 / 0x3f4 / 0x3f8 / 0x3fc / 0x400).
func (f DS3Faults) InverterStatus() InverterStatus {
	return InverterStatus{
		DCBusFault: f.DCBusFault,
		FaultA:     f.OverFreqStage1 || f.OverFreqStage2 || f.OverFreqAux || f.OverFreqExtra,
		FaultB:     f.UnderFreqStage1 || f.UnderFreqStage2 || f.UnderFreqAux || f.UnderFreqExtra,
		WarningA:   f.ACOverVoltStage1 || f.ACOverVoltStage2,
		WarningB:   f.ACUnderVoltStage1 || f.ACUnderVoltStage2,
	}
}

package codec

import "testing"

func TestDS3Status_Faults_BitMap(t *testing.T) {
	cases := []struct {
		name  string
		raw   [5]byte
		check func(f DS3Faults) bool
	}{
		{"grid relay fault", ds3Set(1, 1<<1), func(f DS3Faults) bool { return f.GridRelayFault }},
		{"DC contactor fault", ds3Set(2, 1<<7), func(f DS3Faults) bool { return f.DCContactorFault }},
		{"DC bus fault", ds3Set(2, 1<<4), func(f DS3Faults) bool { return f.DCBusFault }},
		{"DC ground fault", ds3Set(2, 1<<3), func(f DS3Faults) bool { return f.DCGroundFault }},
		{"iso fault A", ds3Set(3, 1<<4), func(f DS3Faults) bool { return f.IsoFaultA }},
		{"iso fault B", ds3Set(3, 1<<6), func(f DS3Faults) bool { return f.IsoFaultB }},
		{"AC OV stage 1", ds3Set(3, 1<<0), func(f DS3Faults) bool { return f.ACOverVoltStage1 }},
		{"AC OV stage 2", ds3Set(3, 1<<2), func(f DS3Faults) bool { return f.ACOverVoltStage2 }},
		{"AC UV stage 1", ds3Set(3, 1<<1), func(f DS3Faults) bool { return f.ACUnderVoltStage1 }},
		{"AC UV stage 2", ds3Set(3, 1<<3), func(f DS3Faults) bool { return f.ACUnderVoltStage2 }},
		{"OF stage 1", ds3Set(4, 1<<2), func(f DS3Faults) bool { return f.OverFreqStage1 }},
		{"OF stage 2", ds3Set(4, 1<<4), func(f DS3Faults) bool { return f.OverFreqStage2 }},
		{"OF aux", ds3Set(4, 1<<6), func(f DS3Faults) bool { return f.OverFreqAux }},
		{"OF extra", ds3Set(4, 1<<0), func(f DS3Faults) bool { return f.OverFreqExtra }},
		{"UF stage 1", ds3Set(4, 1<<3), func(f DS3Faults) bool { return f.UnderFreqStage1 }},
		{"UF stage 2", ds3Set(4, 1<<5), func(f DS3Faults) bool { return f.UnderFreqStage2 }},
		{"UF aux", ds3Set(4, 1<<7), func(f DS3Faults) bool { return f.UnderFreqAux }},
		{"UF extra", ds3Set(4, 1<<1), func(f DS3Faults) bool { return f.UnderFreqExtra }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := DS3Status{Raw: tc.raw}.Faults()
			if !tc.check(f) {
				t.Errorf("expected target field true for raw %v, got %+v", tc.raw, f)
			}
		})
	}
}

func TestDS3Status_Faults_ZeroIsZero(t *testing.T) {
	if (DS3Status{}.Faults() != DS3Faults{}) {
		t.Errorf("zero status produced non-zero faults")
	}
}

// Byte 0x0b (Raw[0]) holds only warning-bucket and legacy slots —
// no named DS3Faults field reads it, so setting it to 0xff must
// leave Faults at zero.
func TestDS3Status_Faults_Byte0bUnusedByNamedFields(t *testing.T) {
	if (DS3Status{Raw: ds3Set(0, 0xff)}.Faults() != DS3Faults{}) {
		t.Errorf("byte 0x0b bits unexpectedly populated named fields")
	}
}

func TestDecodeDS3Status_ShortBodyReturnsZero(t *testing.T) {
	body := make([]byte, 0x0f) // one byte short of body[0x10]
	got := decodeDS3Status(body)
	if got != (DS3Status{}) {
		t.Errorf("short body should yield zero DS3Status, got %v", got.Raw)
	}
}

func TestDS3Faults_InverterStatusReduction(t *testing.T) {
	// FaultA fires when any byte 0xf even bit is set.
	is := (DS3Faults{OverFreqStage1: true}).InverterStatus()
	if !is.FaultA || is.FaultB || is.WarningA || is.WarningB {
		t.Errorf("OverFreqStage1 should set only FaultA, got %+v", is)
	}
	// WarningB fires only on AC under-volt bits.
	is = (DS3Faults{ACUnderVoltStage2: true}).InverterStatus()
	if is.FaultA || is.FaultB || is.WarningA || !is.WarningB {
		t.Errorf("ACUnderVoltStage2 should set only WarningB, got %+v", is)
	}
	if (DS3Faults{}.InverterStatus() != InverterStatus{}) {
		t.Errorf("zero faults should yield zero InverterStatus")
	}
}

func TestDS3Status_ModbusStatus_StRegister(t *testing.T) {
	// Grid relay open → St = 4 (standby).
	st, _ := DS3Status{}.ModbusStatus()
	if st != 4 {
		t.Errorf("relay-open St: got %d want 4", st)
	}
	// Grid relay closed (body[0xc] bit 1) → St = 6.
	st, _ = DS3Status{Raw: ds3Set(1, 1<<1)}.ModbusStatus()
	if st != 6 {
		t.Errorf("relay-closed St: got %d want 6", st)
	}
}

func TestDS3Status_ModbusStatus_Evt1Bits(t *testing.T) {
	cases := []struct {
		name    string
		raw     [5]byte
		evt1Has uint16
	}{
		{"DC contactor → bit 15", ds3Set(2, 1<<7), 0x8000},
		{"iso fault A → bit 14", ds3Set(3, 1<<4), 0x4000},
		{"iso fault B → bit 14", ds3Set(3, 1<<6), 0x4000},
		{"grid relay → bit 9", ds3Set(1, 1<<1), 0x0200},
		{"DC ground → bit 8", ds3Set(2, 1<<3), 0x0100},
		{"AC OV1 → bit 7", ds3Set(3, 1<<0), 0x0080},
		{"AC OV2 → bit 7", ds3Set(3, 1<<2), 0x0080},
		{"AC UV1 → bit 6", ds3Set(3, 1<<1), 0x0040},
		{"AC UV2 → bit 6", ds3Set(3, 1<<3), 0x0040},
		{"OF stage1 → bit 5", ds3Set(4, 1<<2), 0x0020},
		{"OF stage2 → bit 5", ds3Set(4, 1<<4), 0x0020},
		{"OF aux → bit 5", ds3Set(4, 1<<6), 0x0020},
		{"OF extra → bit 5", ds3Set(4, 1<<0), 0x0020},
		{"UF stage1 → bit 4", ds3Set(4, 1<<3), 0x0010},
		{"UF stage2 → bit 4", ds3Set(4, 1<<5), 0x0010},
		{"UF aux → bit 4", ds3Set(4, 1<<7), 0x0010},
		{"UF extra → bit 4", ds3Set(4, 1<<1), 0x0010},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, evt1 := DS3Status{Raw: tc.raw}.ModbusStatus()
			if evt1&tc.evt1Has == 0 {
				t.Errorf("Evt1=0x%04x missing mask 0x%04x", evt1, tc.evt1Has)
			}
		})
	}
}

func TestDS3Status_ModbusStatus_AllBitsAllSet(t *testing.T) {
	// Every status bit set → all bits this DS3-only path can supply
	// should be high. Bits sourced from non-body slots (comm @ 0x29b,
	// ZB-link @ 0x3cc/0x3cd, over-temp @ 0x2b3) stay zero.
	st, evt1 := DS3Status{Raw: [5]byte{0xff, 0xff, 0xff, 0xff, 0xff}}.ModbusStatus()
	if st != 6 {
		t.Errorf("St: got %d want 6", st)
	}
	want := uint16(0x8000 | 0x4000 | 0x0200 | 0x0100 | 0x0080 | 0x0040 | 0x0020 | 0x0010)
	if evt1 != want {
		t.Errorf("Evt1: got 0x%04x want 0x%04x", evt1, want)
	}
}

// ds3Set returns a 5-byte array with one byte set to v at index idx.
func ds3Set(idx int, v byte) [5]byte {
	var b [5]byte
	b[idx] = v
	return b
}

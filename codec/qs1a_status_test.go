package codec

import "testing"

// mainSet returns a 4-byte Main array with one byte set to v at index idx.
func mainSet(idx int, v byte) [4]byte {
	var b [4]byte
	b[idx] = v
	return b
}

// gridSet returns a 2-byte Grid array with one byte set to v at index idx.
func gridSet(idx int, v byte) [2]byte {
	var b [2]byte
	b[idx] = v
	return b
}

func TestQS1AStatus_Faults_MainByteBits(t *testing.T) {
	cases := []struct {
		name  string
		main  [4]byte
		check func(f QS1AFaults) bool
	}{
		// byte 0x17
		{"DC contactor fault", mainSet(0, 1<<1), func(f QS1AFaults) bool { return f.DCContactorFault }},
		{"grid relay fault", mainSet(0, 1<<2), func(f QS1AFaults) bool { return f.GridRelayFault }},
		{"DC ground fault", mainSet(0, 1<<3), func(f QS1AFaults) bool { return f.DCGroundFault }},
		{"comm fault", mainSet(0, 1<<4), func(f QS1AFaults) bool { return f.CommFault }},
		{"OF RMS", mainSet(0, 1<<5), func(f QS1AFaults) bool { return f.OverFreqRMS }},
		{"UF RMS", mainSet(0, 1<<6), func(f QS1AFaults) bool { return f.UnderFreqRMS }},
		// byte 0x18
		{"iso fault C", mainSet(1, 1<<0), func(f QS1AFaults) bool { return f.IsoFaultC }},
		{"over-temperature", mainSet(1, 1<<2), func(f QS1AFaults) bool { return f.OverTemperature }},
		{"DC bus fault", mainSet(1, 1<<4), func(f QS1AFaults) bool { return f.DCBusFault }},
		// byte 0x19
		{"OF extra", mainSet(2, 1<<0), func(f QS1AFaults) bool { return f.OverFreqExtra }},
		{"UF extra", mainSet(2, 1<<1), func(f QS1AFaults) bool { return f.UnderFreqExtra }},
		{"iso fault A", mainSet(2, 1<<2), func(f QS1AFaults) bool { return f.IsoFaultA }},
		{"iso fault B", mainSet(2, 1<<4), func(f QS1AFaults) bool { return f.IsoFaultB }},
		{"iso fault D", mainSet(2, 1<<6), func(f QS1AFaults) bool { return f.IsoFaultD }},
		// byte 0x1a
		{"AC OV slow", mainSet(3, 1<<0), func(f QS1AFaults) bool { return f.ACOverVoltSlow }},
		{"AC UV slow", mainSet(3, 1<<1), func(f QS1AFaults) bool { return f.ACUnderVoltSlow }},
		{"AC OV fast", mainSet(3, 1<<2), func(f QS1AFaults) bool { return f.ACOverVoltFast }},
		{"AC UV fast", mainSet(3, 1<<3), func(f QS1AFaults) bool { return f.ACUnderVoltFast }},
		{"OF slow", mainSet(3, 1<<4), func(f QS1AFaults) bool { return f.OverFreqSlow }},
		{"UF slow", mainSet(3, 1<<5), func(f QS1AFaults) bool { return f.UnderFreqSlow }},
		{"OF fast", mainSet(3, 1<<6), func(f QS1AFaults) bool { return f.OverFreqFast }},
		{"UF fast", mainSet(3, 1<<7), func(f QS1AFaults) bool { return f.UnderFreqFast }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := QS1AStatus{Main: tc.main}.Faults()
			if !tc.check(f) {
				t.Errorf("expected target field true for Main %v, got %+v", tc.main, f)
			}
		})
	}
}

func TestQS1AStatus_Faults_ZeroIsZero(t *testing.T) {
	if (QS1AStatus{}.Faults() != QS1AFaults{}) {
		t.Errorf("zero status produced non-zero faults")
	}
}

// body[0x38] (Grid[0]) only feeds slot 0x3d8, which is unmapped to
// any named QS1AFaults field — setting Grid[0] to 0xff must not
// pollute the Faults struct.
func TestQS1AStatus_Faults_Grid0Byte0xffStaysZero(t *testing.T) {
	if (QS1AStatus{Grid: gridSet(0, 0xff)}.Faults() != QS1AFaults{}) {
		t.Errorf("Grid[0] bits unexpectedly populated named fields")
	}
}

func TestDecodeQS1AStatus_ShortBodyReturnsZero(t *testing.T) {
	body := make([]byte, 0x39) // one byte short of body[0x3a]
	got := decodeQS1AStatus(body)
	if got != (QS1AStatus{}) {
		t.Errorf("short body should yield zero QS1AStatus, got %+v", got)
	}
}

func TestQS1AFaults_InverterStatusReduction(t *testing.T) {
	is := (QS1AFaults{OverFreqFast: true}).InverterStatus()
	if !is.FaultA || is.FaultB || is.WarningA || is.WarningB {
		t.Errorf("OverFreqFast should set only FaultA, got %+v", is)
	}
	is = (QS1AFaults{ACUnderVoltSlow: true}).InverterStatus()
	if is.FaultA || is.FaultB || is.WarningA || !is.WarningB {
		t.Errorf("ACUnderVoltSlow should set only WarningB, got %+v", is)
	}
	if (QS1AFaults{}.InverterStatus() != InverterStatus{}) {
		t.Errorf("zero faults should yield zero InverterStatus")
	}
}

func TestQS1AStatus_Faults_GridByteBits(t *testing.T) {
	cases := []struct {
		name  string
		grid  [2]byte
		check func(f QS1AFaults) bool
	}{
		{"ZB link A (body[0x39] bit 0)", gridSet(1, 1<<0), func(f QS1AFaults) bool { return f.ZBLinkA }},
		{"ZB link B (body[0x39] bit 1)", gridSet(1, 1<<1), func(f QS1AFaults) bool { return f.ZBLinkB }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := QS1AStatus{Grid: tc.grid}.Faults()
			if !tc.check(f) {
				t.Errorf("expected target field true for Grid %v, got %+v", tc.grid, f)
			}
		})
	}
}

func TestQS1AStatus_ModbusStatus_StRegister(t *testing.T) {
	st, _ := QS1AStatus{}.ModbusStatus()
	if st != 4 {
		t.Errorf("relay-open St: got %d want 4", st)
	}
	st, _ = QS1AStatus{Main: mainSet(0, 1<<2)}.ModbusStatus()
	if st != 6 {
		t.Errorf("relay-closed St: got %d want 6", st)
	}
}

func TestQS1AStatus_ModbusStatus_Evt1Bits(t *testing.T) {
	cases := []struct {
		name    string
		main    [4]byte
		grid    [2]byte
		evt1Has uint16
	}{
		{"DC contactor → bit 15", mainSet(0, 1<<1), [2]byte{}, 0x8000},
		{"iso C → bit 14", mainSet(1, 1<<0), [2]byte{}, 0x4000},
		{"iso A → bit 14", mainSet(2, 1<<2), [2]byte{}, 0x4000},
		{"iso B → bit 14", mainSet(2, 1<<4), [2]byte{}, 0x4000},
		{"iso D → bit 14", mainSet(2, 1<<6), [2]byte{}, 0x4000},
		{"comm → bits 13 and 11", mainSet(0, 1<<4), [2]byte{}, 0x2800},
		{"ZB link A → bit 12", [4]byte{}, gridSet(1, 1<<0), 0x1000},
		{"ZB link B → bit 12", [4]byte{}, gridSet(1, 1<<1), 0x1000},
		{"grid relay → bit 9", mainSet(0, 1<<2), [2]byte{}, 0x0200},
		{"DC ground → bit 8", mainSet(0, 1<<3), [2]byte{}, 0x0100},
		{"AC OV fast → bit 7", mainSet(3, 1<<2), [2]byte{}, 0x0080},
		{"AC OV slow → bit 7", mainSet(3, 1<<0), [2]byte{}, 0x0080},
		{"AC UV fast → bit 6", mainSet(3, 1<<3), [2]byte{}, 0x0040},
		{"AC UV slow → bit 6", mainSet(3, 1<<1), [2]byte{}, 0x0040},
		{"OF fast → bit 5", mainSet(3, 1<<6), [2]byte{}, 0x0020},
		{"OF extra → bit 5", mainSet(2, 1<<0), [2]byte{}, 0x0020},
		{"OF RMS → bit 5", mainSet(0, 1<<5), [2]byte{}, 0x0020},
		{"UF fast → bit 4", mainSet(3, 1<<7), [2]byte{}, 0x0010},
		{"UF extra → bit 4", mainSet(2, 1<<1), [2]byte{}, 0x0010},
		{"UF RMS → bit 4", mainSet(0, 1<<6), [2]byte{}, 0x0010},
		{"over-temperature → bit 1", mainSet(1, 1<<2), [2]byte{}, 0x0002},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, evt1 := QS1AStatus{Main: tc.main, Grid: tc.grid}.ModbusStatus()
			if evt1&tc.evt1Has != tc.evt1Has {
				t.Errorf("Evt1=0x%04x missing mask 0x%04x", evt1, tc.evt1Has)
			}
		})
	}
}

func TestQS1AStatus_ModbusStatus_AllBitsAllSet(t *testing.T) {
	// All Main + all Grid bytes 0xff → every Evt1 bit QS1A supplies
	// should fire. SunSpec Model 103 bits not driven by any QS1A
	// body source stay zero (Stop, Memory, hardware fault).
	st, evt1 := QS1AStatus{
		Main: [4]byte{0xff, 0xff, 0xff, 0xff},
		Grid: [2]byte{0xff, 0xff},
	}.ModbusStatus()
	if st != 6 {
		t.Errorf("St: got %d want 6", st)
	}
	want := uint16(0x8000 | 0x4000 | 0x2000 | 0x1000 | 0x0800 |
		0x0200 | 0x0100 | 0x0080 | 0x0040 | 0x0020 | 0x0010 | 0x0002)
	if evt1 != want {
		t.Errorf("Evt1: got 0x%04x want 0x%04x", evt1, want)
	}
}

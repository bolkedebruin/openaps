package codec

import "testing"

// TestExtendedStatus_InterfaceSatisfied is a compile-time + runtime
// check that both per-family status types implement ExtendedStatus.
// The compile-time _ = ExtendedStatus(...) checks already live in
// extended_status.go; this test pins the projection behaviour through
// the interface so a future implementation breaks loudly.
func TestExtendedStatus_InterfaceSatisfied(t *testing.T) {
	cases := []struct {
		name string
		es   ExtendedStatus
	}{
		{"DS3Status zero", DS3Status{}},
		{"QS1AStatus zero", QS1AStatus{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if (c.es.InverterStatus() != InverterStatus{}) {
				t.Errorf("InverterStatus: zero status should yield zero InverterStatus, got %+v", c.es.InverterStatus())
			}
			st, evt1 := c.es.ModbusStatus()
			// Zero status → GridRelay false → St=4 (StandBy), Evt1=0.
			if st != 4 || evt1 != 0 {
				t.Errorf("ModbusStatus zero: got (st=%d evt1=0x%04x) want (4,0)", st, evt1)
			}
		})
	}
}

// TestExtendedStatus_DS3RoundTrip pins the DS3 projection through the
// ExtendedStatus interface against the per-family Faults() projection,
// so future refactors of either path stay consistent.
func TestExtendedStatus_DS3RoundTrip(t *testing.T) {
	raw := DS3Status{Raw: [5]byte{0, 1 << 1, 1 << 4, 1 << 0, 1 << 2}}
	want := raw.Faults().InverterStatus()
	var es ExtendedStatus = raw
	if es.InverterStatus() != want {
		t.Errorf("InverterStatus via interface diverges: got %+v want %+v", es.InverterStatus(), want)
	}
	wantSt, wantEvt1 := raw.ModbusStatus()
	gotSt, gotEvt1 := es.ModbusStatus()
	if gotSt != wantSt || gotEvt1 != wantEvt1 {
		t.Errorf("ModbusStatus via interface diverges: got (%d,0x%04x) want (%d,0x%04x)", gotSt, gotEvt1, wantSt, wantEvt1)
	}
}

// TestExtendedStatus_QS1ARoundTrip mirrors the DS3 check on QS1A.
func TestExtendedStatus_QS1ARoundTrip(t *testing.T) {
	raw := QS1AStatus{
		Main: [4]byte{1 << 2, 1 << 4, 1 << 0, 1 << 6},
		Grid: [2]byte{0, 1 << 1},
	}
	want := raw.Faults().InverterStatus()
	var es ExtendedStatus = raw
	if es.InverterStatus() != want {
		t.Errorf("InverterStatus via interface diverges: got %+v want %+v", es.InverterStatus(), want)
	}
	wantSt, wantEvt1 := raw.ModbusStatus()
	gotSt, gotEvt1 := es.ModbusStatus()
	if gotSt != wantSt || gotEvt1 != wantEvt1 {
		t.Errorf("ModbusStatus via interface diverges: got (%d,0x%04x) want (%d,0x%04x)", gotSt, gotEvt1, wantSt, wantEvt1)
	}
}

// TestReply_ExtendedStatus_NilForUnknownFamily verifies that a reply
// the codec failed to classify (unknown cmd byte) leaves ExtendedStatus
// nil — callers MUST check the interface for nil before projecting.
func TestReply_ExtendedStatus_NilForUnknownFamily(t *testing.T) {
	// Same shape as TestDecodeReply_UnknownCmdLeavesBodyForInspection
	// but focused on the new ExtendedStatus contract.
	frame := hx(t, "fcfc 5011 00ff 999900000003 fbfb 06 aa 00 00 00 00 00 00 c1 fefe")
	r, err := DecodeReply(frame)
	if err == nil {
		t.Fatal("unknown cmd should return an error")
	}
	if r.Family != FamilyUnknown {
		t.Errorf("Family: got %v want FamilyUnknown", r.Family)
	}
	if r.ExtendedStatus != nil {
		t.Errorf("ExtendedStatus: got %+v want nil for unknown family", r.ExtendedStatus)
	}
}

package source

import (
	"testing"

	"github.com/bolke/inv-driver/wire"
)

func TestProtectionFromWire(t *testing.T) {
	p := &wire.Protection{
		PeerUid: "999900000001",
		Values: map[string]float64{
			"CB": 50.2, // over-freq curve low
			"AK": 52.0, // over-freq fast trip
			"CV": 13,   // droop mode
			"AG": 60,   // grid recovery
			// CH not reported.
		},
	}
	// A code already in the base that the wire envelope doesn't carry (BB)
	// must survive the merge, and OFFast must be overridden by the wire value.
	base := ProtectionParams{Has: map[string]bool{"BB": true, "AK": true}, UV2ClrS: 0.2, OFFast: 99}
	pp := protectionFromWire(base, p)
	if pp.UV2ClrS != 0.2 || !pp.Has["BB"] {
		t.Errorf("BB base not preserved: %v has=%v", pp.UV2ClrS, pp.Has["BB"])
	}
	if pp.OFCurveOFLow != 50.2 || !pp.Has["CB"] {
		t.Errorf("CB=%v has=%v", pp.OFCurveOFLow, pp.Has["CB"])
	}
	if pp.OFFast != 52.0 || !pp.Has["AK"] {
		t.Errorf("AK=%v has=%v", pp.OFFast, pp.Has["AK"])
	}
	if pp.OFDroopMode != 13 || !pp.Has["CV"] {
		t.Errorf("CV=%v has=%v", pp.OFDroopMode, pp.Has["CV"])
	}
	if pp.ReconnectS != 60 || !pp.Has["AG"] {
		t.Errorf("AG=%v has=%v", pp.ReconnectS, pp.Has["AG"])
	}
	if pp.Has["CH"] {
		t.Error("CH unset on the wire must be absent in Has")
	}
}

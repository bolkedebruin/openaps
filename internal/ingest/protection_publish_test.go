package ingest

import (
	"testing"

	"github.com/bolke/inv-driver/codec"
)

// TestReadingToProto_CarriesEveryCode guards the publish boundary: every code
// the codec can decode is carried in Protection.values — readingToProto copies
// the whole aps_code→value map, so no decoded value can vanish before its
// consumers (ecu-web / ecu-sunspec) silently.
func TestReadingToProto_CarriesEveryCode(t *testing.T) {
	all := codec.AllProtectionCodes()
	if len(all) == 0 {
		t.Fatal("codec.AllProtectionCodes() returned nothing")
	}
	vals := make(map[string]float64, len(all))
	for i, code := range all {
		vals[code] = float64(i) + 0.5
	}
	p := readingToProto("aabbccddeeff", 1, &codec.ProtectionReading{Model: "DS3", Values: vals})
	got := p.GetValues()
	for _, code := range all {
		if _, ok := got[code]; !ok {
			t.Errorf("code %s decoded by codec but not carried in Protection.values (silent drop)", code)
		}
	}
	if len(got) != len(all) {
		t.Errorf("Protection.values has %d entries, want %d", len(got), len(all))
	}
}

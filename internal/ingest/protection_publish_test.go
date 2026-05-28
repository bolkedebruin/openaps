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

// TestCacheProtectionValues_MergeKeepsDroppedPage guards the read-back cap fix:
// a QS1A read spans 3 pages, and a re-read whose page B reply drops must NOT
// erase a previously-known code (the DA output cap). cacheProtectionValues
// merges per-UID, so DA survives a partial read and a real change still wins.
func TestCacheProtectionValues_MergeKeepsDroppedPage(t *testing.T) {
	in := &Ingestor{}
	uid := "806000042582"

	// Full read: page A + B + C (DA + DD live on page B).
	m1 := in.cacheProtectionValues(uid, map[string]float64{"AC": 196, "DA": 375, "DD": 16.56, "DH": 47})
	if m1["DA"] != 375 {
		t.Fatalf("after full read DA=%v, want 375", m1["DA"])
	}

	// Partial re-read: page B dropped (no DA/DD) — must keep the prior values.
	m2 := in.cacheProtectionValues(uid, map[string]float64{"AC": 196, "DH": 47})
	if m2["DA"] != 375 {
		t.Errorf("after partial re-read DA=%v, want 375 (merge must keep the dropped page)", m2["DA"])
	}
	if m2["DD"] != 16.56 {
		t.Errorf("DD=%v, want 16.56 (kept)", m2["DD"])
	}

	// A real change still overwrites on the next read that carries the code.
	m3 := in.cacheProtectionValues(uid, map[string]float64{"DA": 500})
	if m3["DA"] != 500 {
		t.Errorf("after change DA=%v, want 500", m3["DA"])
	}
	if rb, ok := in.ReadbackNative(uid); !ok || rb["DA"] != 500 || rb["AC"] != 196 {
		t.Errorf("ReadbackNative = %v (ok=%v), want cumulative with DA=500", rb, ok)
	}
}

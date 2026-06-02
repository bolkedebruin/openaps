package inventory

import "testing"

func TestMap_RecordAndLookup(t *testing.T) {
	t.Parallel()
	m := NewMap()
	if _, ok := m.LookupSA("999900000003"); ok {
		t.Fatal("empty map: lookup should miss")
	}
	m.Record("999900000003", 0xC459)
	sa, ok := m.LookupSA("999900000003")
	if !ok || sa != 0xC459 {
		t.Fatalf("lookup: got (0x%04X, %v) want (0xC459, true)", sa, ok)
	}
}

func TestMap_CaseInsensitive(t *testing.T) {
	t.Parallel()
	m := NewMap()
	m.Record("80600004ABCD", 0x1234)
	sa, ok := m.LookupSA("80600004abcd")
	if !ok || sa != 0x1234 {
		t.Fatalf("lower-case lookup: got (0x%04X, %v)", sa, ok)
	}
}

func TestMap_NilSafe(t *testing.T) {
	t.Parallel()
	var m *Map
	m.Record("aabbccddeeff", 1)
	if _, ok := m.LookupSA("aabbccddeeff"); ok {
		t.Fatal("nil map: lookup should miss")
	}
}

func TestMap_EmptyUIDIgnored(t *testing.T) {
	t.Parallel()
	m := NewMap()
	m.Record("", 0x1111)
	if _, ok := m.LookupSA(""); ok {
		t.Fatal("empty uid should not be recorded")
	}
}

func TestMap_OverwriteLatest(t *testing.T) {
	t.Parallel()
	m := NewMap()
	m.Record("abc", 0x0001)
	m.Record("abc", 0x0002)
	sa, _ := m.LookupSA("abc")
	if sa != 0x0002 {
		t.Fatalf("got 0x%04X want 0x0002", sa)
	}
}

package source

import "testing"

func TestPowerLimitCache(t *testing.T) {
	c := NewPowerLimitCache()
	if _, ok := c.Get("X"); ok {
		t.Error("empty cache should miss")
	}
	c.Set("X", 137)
	if w, ok := c.Get("X"); !ok || w != 137 {
		t.Errorf("got %d,%v want 137,true", w, ok)
	}
	// nil-safe
	var n *PowerLimitCache
	n.Set("Y", 1)
	if _, ok := n.Get("Y"); ok {
		t.Error("nil cache Get must be false")
	}
}

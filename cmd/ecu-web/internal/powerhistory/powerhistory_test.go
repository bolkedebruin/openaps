package powerhistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const (
	tsYear2000 = 946684800000  // 2000-01-01, below the plausibility floor
	tsValid    = 1700000000000 // 2023-11-14, above the floor
)

func TestAddDropsImplausibleTimestamps(t *testing.T) {
	r := New(10, "")
	r.Add(tsYear2000, 100)       // unset/glitched clock — dropped
	r.Add(tsValid, 200)          // good
	r.Add(tsYear2000+60000, 300) // still year 2000 — dropped

	pts := r.Points()
	if len(pts) != 1 {
		t.Fatalf("points = %d, want 1 (only the valid sample)", len(pts))
	}
	if pts[0].T != tsValid || pts[0].W != 200 {
		t.Errorf("kept point = %+v, want {T:%d W:200}", pts[0], int64(tsValid))
	}
}

func TestNewDropsPoisonedPointsOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "power-history.json")
	seed := []Point{
		{T: tsYear2000, W: 1},
		{T: tsYear2000 + 60000, W: 2},
		{T: tsValid, W: 3},
		{T: tsValid + 60000, W: 4},
	}
	b, _ := json.Marshal(seed)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	pts := New(10, path).Points()
	if len(pts) != 2 {
		t.Fatalf("loaded points = %d, want 2 (year-2000 filtered)", len(pts))
	}
	for _, p := range pts {
		if !plausible(p.T) {
			t.Errorf("kept implausible point %+v", p)
		}
	}
	if pts[0].W != 3 || pts[1].W != 4 {
		t.Errorf("kept = %+v, want W 3 then 4 in order", pts)
	}
}

func TestNewCapacityTrimAfterFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "power-history.json")
	seed := []Point{
		{T: tsYear2000, W: 0}, // dropped first
		{T: tsValid, W: 1},
		{T: tsValid + 60000, W: 2},
		{T: tsValid + 120000, W: 3},
	}
	b, _ := json.Marshal(seed)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	pts := New(2, path).Points() // 3 valid survive filter, then trim to last 2
	if len(pts) != 2 {
		t.Fatalf("points = %d, want 2", len(pts))
	}
	if pts[0].W != 2 || pts[1].W != 3 {
		t.Errorf("kept = %+v, want last two valid (W 2,3)", pts)
	}
}

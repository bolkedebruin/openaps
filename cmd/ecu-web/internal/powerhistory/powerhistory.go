// Package powerhistory keeps a downsampled ring of fleet output power so
// the dashboard can chart W across a multi-day window. ecu-web samples
// it from the live snapshot; it is presentation history (a convenience),
// not the authoritative energy record. The ring is persisted to a file
// in the state dir so it survives restarts.
package powerhistory

import (
	"encoding/json"
	"os"
	"sync"
)

// Point is one sample: wall-clock unix ms and fleet active power (W).
type Point struct {
	T int64   `json:"t"`
	W float64 `json:"w"`
}

// Ring is a fixed-capacity, time-ordered buffer of Points.
type Ring struct {
	mu   sync.Mutex
	pts  []Point
	cap  int
	path string
}

// New returns a Ring holding up to capacity points, persisted at path
// (empty disables persistence). Existing points at path are loaded.
func New(capacity int, path string) *Ring {
	r := &Ring{cap: capacity, path: path}
	if path != "" {
		if b, err := os.ReadFile(path); err == nil {
			var pts []Point
			if json.Unmarshal(b, &pts) == nil {
				if len(pts) > capacity {
					pts = pts[len(pts)-capacity:]
				}
				r.pts = pts
			}
		}
	}
	return r
}

// Add appends a sample, evicting the oldest when over capacity.
func (r *Ring) Add(tMs int64, w float64) {
	r.mu.Lock()
	r.pts = append(r.pts, Point{T: tMs, W: w})
	if len(r.pts) > r.cap {
		r.pts = r.pts[len(r.pts)-r.cap:]
	}
	r.mu.Unlock()
}

// Points returns a copy, oldest→newest.
func (r *Ring) Points() []Point {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Point, len(r.pts))
	copy(out, r.pts)
	return out
}

// Save persists the ring to its file (atomic write). No-op when path is
// empty.
func (r *Ring) Save() error {
	if r.path == "" {
		return nil
	}
	b, err := json.Marshal(r.Points())
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

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

// minPlausibleMs is the earliest timestamp the ring accepts: 2020-01-01
// UTC. A correct ECU clock is well past this; an unset or glitched clock
// (e.g. 2000-01-01 after an RTC loss) lands before it. The chart spans
// its x-axis from the first to the last point, so a single year-2000
// sample stretches the axis across decades and collapses the real data
// into an invisible sliver. Dropping sub-epoch points on load and on Add
// keeps a clock glitch from poisoning the chart — and lets it self-heal
// once the clock is corrected, with no manual file cleanup.
const minPlausibleMs = 1577836800000 // 2020-01-01T00:00:00Z

// plausible reports whether a timestamp is recent enough to chart.
func plausible(tMs int64) bool { return tMs >= minPlausibleMs }

// keepPlausible returns the points with implausible timestamps removed,
// preserving order.
func keepPlausible(in []Point) []Point {
	out := make([]Point, 0, len(in))
	for _, p := range in {
		if plausible(p.T) {
			out = append(out, p)
		}
	}
	return out
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
				pts = keepPlausible(pts)
				if len(pts) > capacity {
					pts = pts[len(pts)-capacity:]
				}
				r.pts = pts
			}
		}
	}
	return r
}

// Add appends a sample, evicting the oldest when over capacity. Samples
// with an implausible timestamp (clock unset/glitched) are dropped so
// they can't poison the chart's time axis.
func (r *Ring) Add(tMs int64, w float64) {
	if !plausible(tMs) {
		return
	}
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

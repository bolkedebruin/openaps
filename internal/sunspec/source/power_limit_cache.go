package source

import "sync"

// PowerLimitCache holds the per-inverter set-power cap (watts/panel) that
// ecu-sunspec itself last commanded. Since inv-driver is the sole
// set-power writer, ecu-sunspec's own last write is the authoritative
// current cap for the WMaxLimPct read-back. An inverter with no recorded
// write is uncapped (the Builder defaults to MaxPanelLimitW).
type PowerLimitCache struct {
	mu sync.RWMutex
	w  map[string]int
}

// NewPowerLimitCache returns an empty cache.
func NewPowerLimitCache() *PowerLimitCache {
	return &PowerLimitCache{w: make(map[string]int)}
}

// Set records the per-panel watt cap last commanded for uid.
func (c *PowerLimitCache) Set(uid string, watts int) {
	if c == nil || uid == "" {
		return
	}
	c.mu.Lock()
	c.w[uid] = watts
	c.mu.Unlock()
}

// Get returns the recorded cap for uid, ok=false if none.
func (c *PowerLimitCache) Get(uid string) (int, bool) {
	if c == nil {
		return 0, false
	}
	c.mu.RLock()
	w, ok := c.w[uid]
	c.mu.RUnlock()
	return w, ok
}

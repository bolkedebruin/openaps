package source

import "github.com/bolke/inv-driver/codec"

// MinPanelLimitW and MaxPanelLimitW are the per-panel watt bounds the
// SunSpec WMaxLimPct clamp uses. The canonical source of truth is the
// inv-driver codec, which mirrors the ECU's stock set_maxpower() endpoint
// (rejects values < 20 or > 500). int-typed here for the SunSpec
// register math; set-power itself is dispatched through inv-driver.
const (
	MinPanelLimitW = int(codec.MinPanelLimitW)
	MaxPanelLimitW = int(codec.MaxPanelLimitW)
)

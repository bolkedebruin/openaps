package codec

// MinPanelLimitW and MaxPanelLimitW bound the per-panel watt setpoint
// every set-power encoder accepts. The bounds mirror the ECU's
// stock PHP set_maxpower() endpoint (rejects values < 20 or > 500), so
// writes through this codec see the same error envelope as writes
// through the local web UI.
const (
	MinPanelLimitW uint16 = 20
	MaxPanelLimitW uint16 = 500
)

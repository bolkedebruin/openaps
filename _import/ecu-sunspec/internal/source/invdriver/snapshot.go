package invdriver

import (
	"math"
	"time"

	"github.com/bolke/inv-driver/wire"
)

// Snapshot is the cached form of one inverter's stream. Telemetry
// fields come from wire.Telemetry envelopes; identity fields (model,
// software version, per-leg phase, pair flags) come from
// wire.InverterInfo envelopes. Identity pointers are nil until the
// first Info envelope arrives, distinguishing "not yet seen" from
// "explicitly zero". FetchedAt tracks telemetry freshness;
// InfoFetchedAt tracks identity freshness.
type Snapshot struct {
	TsMs       int64
	Cmd        uint32
	Model      string
	ACWatts    float64
	ACVolts    float64
	ACFreq     float64
	BusV       float64
	ReportSec  uint32
	Panels     []Panel
	LifetimeWh float64
	RSSI       byte // L1 envelope byte 4 — primary RSSI
	LQI        byte // L1 envelope byte 5 — secondary signal metric
	FetchedAt  time.Time

	// Faults carries the family-specific named-bit decode the codec
	// produces. Nil when the inverter has not yet streamed a telemetry
	// frame this session, or when the codec can't family-classify the
	// reply (unknown cmd byte).
	Faults *wire.InverterFaults

	ModelCode       *uint32
	SoftwareVersion *uint32
	Phase           *uint32 // per-leg assignment (1/2/3); nil for unset
	ZigbeeBound     *bool
	TurnedOffRpt    *bool
	InfoFetchedAt   time.Time
}

// Panel is the per-DC-channel slice of a Snapshot.
type Panel struct {
	Index int
	DCV   float64
	DCI   float64
	W     float64
}

// fromTelemetry converts a wire.Telemetry into a Snapshot. The
// lifetime sum uses the inverter-reported per-channel counters
// multiplied by the per-frame scale; no host-side integration is done.
// Non-finite or negative scales are dropped to keep a hostile or
// malformed frame from poisoning the downstream uint64 cast.
func fromTelemetry(t *wire.Telemetry) Snapshot {
	panels := t.GetPanels()
	out := Snapshot{
		TsMs:      t.GetTsMs(),
		Cmd:       t.GetCmd(),
		Model:     t.GetModel(),
		ACWatts:   t.GetActivePowerW(),
		ACVolts:   t.GetGridV(),
		ACFreq:    t.GetFreqHz(),
		BusV:      t.GetBusV(),
		ReportSec: t.GetReportSec(),
		Faults:    t.GetFaults(),
		RSSI:      byte(t.GetRssi()),
		LQI:       byte(t.GetLqi()),
		FetchedAt: time.Now(),
	}
	if len(panels) > 0 {
		out.Panels = make([]Panel, 0, len(panels))
		for _, p := range panels {
			out.Panels = append(out.Panels, Panel{
				Index: int(p.GetIndex()),
				DCV:   p.GetDcV(),
				DCI:   p.GetDcI(),
				W:     p.GetW(),
			})
		}
	}
	scale := t.GetLifetimeScale()
	if !math.IsNaN(scale) && !math.IsInf(scale, 0) && scale >= 0 {
		for _, r := range t.GetLifetimeRaw() {
			out.LifetimeWh += float64(r) * scale
		}
	}
	if math.IsNaN(out.LifetimeWh) || math.IsInf(out.LifetimeWh, 0) || out.LifetimeWh < 0 {
		out.LifetimeWh = 0
	}
	return out
}

// mergeInfo overlays identity fields from a wire.InverterInfo onto an
// existing Snapshot. Telemetry fields are left untouched. Unset
// optional fields on the envelope leave the previous identity value
// alone — the wire schema treats unset as "no update for this column".
func mergeInfo(snap Snapshot, info *wire.InverterInfo) Snapshot {
	if info == nil {
		return snap
	}
	if info.ModelCode != nil {
		v := *info.ModelCode
		snap.ModelCode = &v
	}
	if info.SoftwareVersion != nil {
		v := *info.SoftwareVersion
		snap.SoftwareVersion = &v
	}
	if info.Phase != nil {
		v := *info.Phase
		snap.Phase = &v
	}
	if info.ZigbeeBound != nil {
		v := *info.ZigbeeBound
		snap.ZigbeeBound = &v
	}
	if info.TurnedOffRpt != nil {
		v := *info.TurnedOffRpt
		snap.TurnedOffRpt = &v
	}
	snap.InfoFetchedAt = time.Now()
	return snap
}

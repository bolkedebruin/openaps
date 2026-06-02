package source

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// AggregateEventBits returns the bitwise-OR of every inverter's EventBits.
// Used by the aggregate SunSpec bank's Inverter Model so an alarm on any one
// microinverter shows up at the system level.
func (s Snapshot) AggregateEventBits() [4]uint32 {
	var out [4]uint32
	for _, inv := range s.Inverters {
		for i := range out {
			out[i] |= inv.EventBits[i]
		}
	}
	return out
}

// AnyInverterHasFaults returns true when at least one inverter has
// inv-driver-sourced typed faults available. The encoder uses this
// to decide whether to emit the new EvtVnd1 layout (typed) or fall
// back to the legacy EventBits projection.
func (s Snapshot) AnyInverterHasFaults() bool {
	for _, inv := range s.Inverters {
		if inv.Faults != nil {
			return true
		}
	}
	return false
}

// Snapshot is the unified view of ECU state used by the SunSpec encoder.
// All values are best-effort: missing fields are zero, freshness varies per source.
type Snapshot struct {
	Captured time.Time

	ECUID           string
	Firmware        string
	Model           string
	PollingInterval int // seconds; 0 = unknown / default

	SystemPowerW        int32   // each_system_power latest sample
	SystemMaxPowerW     int32   // inv-driver FleetSummary.NameplateTotalW — fleet capacity derived from codec.NameplateWattsForModel applied to each inverter's model_code (0 when invdriver hasn't published a fleet frame yet; encoder falls through to TotalNameplateW())
	LifetimeEnergyWh    uint64  // lifetime_energy * 1000
	TodayEnergyWh       uint64  // daily_energy of today * 1000
	MonthEnergyWh       uint64  // monthly_energy of current month * 1000
	YearEnergyWh        uint64  // yearly_energy of current year * 1000
	GridFrequencyHz     float64 // averaged over online inverters; 0 if none
	GridVoltageV        float64 // averaged over inverters' first AC-voltage column; 0 if none
	MaxTemperatureC     int     // max(per-inverter raw temp - 100)
	InverterCount       int
	InverterOnlineCount int
	Inverters           []Inverter

	// Protection holds per-inverter active grid-protection thresholds keyed
	// by inverter UID. Empty when the firmware doesn't populate
	// protection_parameters60code (older revisions). Used by SunSpec models
	// 707/708/709/710 (DER trip) and 703 (Enter Service).
	Protection map[string]ProtectionParams
}

// TotalNameplateW returns the sum of per-inverter rated AC output, regardless
// of online state. Used for SunSpec Model 120 (Nameplate Ratings).
func (s Snapshot) TotalNameplateW() int {
	sum := 0
	for _, inv := range s.Inverters {
		sum += inv.NameplateW()
	}
	return sum
}

// TotalPanelCount returns the number of PV input channels across all online
// inverters. Used to size SunSpec Model 160 (per-panel granularity).
func (s Snapshot) TotalPanelCount() int {
	n := 0
	for _, inv := range s.Inverters {
		if inv.Online {
			n += inv.PanelCount()
		}
	}
	return n
}

// NominalVRefV returns the nominal AC voltage at the Point of Common
// Coupling, derived from the active grid-protection thresholds the ECU
// has pushed to the inverters. SunSpec Model 121 VRef wants this static
// commissioned value, not a live measurement.
//
// The 10-minute-average over-voltage threshold (AB,
// `min10_Over_average_voltage`) sits at 110% of nominal under
// EN 50549-1 / VDE-AR-N 4105 / similar. We back out nominal as
// AB / 1.10, rounded to the nearest 1 V, taking the first inverter's
// values (the fleet is on a single grid profile, so all entries
// agree).
//
// Returns 0 when no inverter has populated AB — the caller should
// emit the SunSpec uint16 not-implemented sentinel.
func (s Snapshot) NominalVRefV() int {
	for _, p := range s.Protection {
		if p.Has != nil && p.Has["AB"] && p.AvgOV > 0 {
			return int(p.AvgOV/1.10 + 0.5)
		}
	}
	return 0
}

// TotalLimitedW returns the user-configured fleet maximum active power
// — what SunSpec Model 121 calls WMax.
//
// Per inverter, the effective AC ceiling is min(LimitedPowerW × panel_count,
// NameplateW): the per-panel DC cap can exceed the AC inverter's nameplate
// (e.g. 4 × 500 W = 2000 W DC sum vs 1600 W AC on a QS1A), but the
// inverter throttles below the DC sum. The effective ceiling is the
// smaller of the two.
//
// When the inverter is uncapped (LimitedPowerW = MaxPanelLimitW) the
// per-inverter contribution equals NameplateW, so the fleet total
// matches WRtg. Curtailing below nameplate scales the contribution
// down per the user's setpoint.
//
// When NameplateW is unknown (model_int not in the lookup table and no
// TypeCode default), this falls back to LimitedPowerW × panel_count
// — the historical behavior, kept so the model works on hardware
// without a catalogued nameplate.
func (s Snapshot) TotalLimitedW() int {
	sum := 0
	for _, inv := range s.Inverters {
		dc := inv.LimitedPowerW * inv.PanelCount()
		ac := inv.NameplateW()
		if ac > 0 && ac < dc {
			sum += ac
		} else {
			sum += dc
		}
	}
	return sum
}

// Inverter holds per-inverter values, populated from inv-driver telemetry.
// (Field comments noting "params col N" describe the legacy on-wire
// layout; the values now come from inv-driver.)
type Inverter struct {
	UID            string
	Online         bool
	TypeCode       string  // wire-protocol type code; see per_inverter.go inverterModelLabel
	FrequencyHz    float64 // params col 3
	TemperatureC   int     // params col 4 - 100
	ACVoltageV     int     // best-guess phase voltage (params col 6 for type 01 or 03)
	ACPowerW       int     // best-guess phase power (sum of channel powers when knowable)
	SignalStrength int     // 0..255 from signal_strength table
	Phase          int     // from id.phase (1/2/3 or 0)
	Model          int     // from id.model
	SoftwareVer    int     // from id.software_version
	LimitedPowerW  int     // per-panel curtailment cap from power.limitedpower (W)

	// PanelWatts carries decoded per-panel watts from inv-driver; length
	// should equal PanelCount(). It is the source PanelPowers() uses.
	PanelWatts []int

	// EventBits holds the 86-bit event bitstring from database.db.Event,
	// packed LSB-first into uint32 slots: [Evt1, Evt2, EvtVnd1, EvtVnd2].
	// The semantics of each bit are not publicly documented by APsystems —
	// we surface the raw bits and let downstream consumers decode them.
	//
	// Source-of-truth precedence: when Faults is non-nil, the SunSpec
	// encoder uses it for Evt1 + EvtVnd1 and OR's the EventBits-derived
	// Evt1 in for completeness during the hybrid window. EvtVnd1 then
	// reflects the new typed-faults layout, not the legacy PHP-UI bit
	// positions. See docs/EVENTS.md.
	EventBits [4]uint32

	// Faults is the inv-driver-sourced typed fault decode. Nil when
	// this inverter hasn't reported a telemetry frame this session
	// (then encoder falls back to EventBits for alarm output).
	Faults *wire.InverterFaults
}

// PanelCount is the number of PV input channels per the inverter type. We
// avoid hardcoding the fleet shape — every count derives from this single
// switch keyed on the wire-protocol type code.
//
// Type codes follow the upstream HA homeassistant-apsystems_ecu_reader
// integration:
//
//	"01" — YC600 / DS3       (2-channel single-phase)
//	"02" — YC1000            (4-channel three-phase)
//	"03" — QS1               (4-channel single-phase)
//	"04" — DS3-H / DS3D-L    (2-channel single-phase)
//	"05" — QT2               (4-channel three-phase)
func (inv Inverter) PanelCount() int {
	switch inv.TypeCode {
	case "01", "04":
		return 2
	case "03":
		return 4
	}
	return 0
}

// PanelPowers returns one watts value per panel from inv-driver's decoded
// PanelWatts; len(result)==PanelCount(). Returns nil when offline or when
// no per-panel data is present. The slice is truncated/padded to
// PanelCount so the encoder always sees a stable length.
func (inv Inverter) PanelPowers() []int {
	if !inv.Online || inv.PanelWatts == nil {
		return nil
	}
	n := inv.PanelCount()
	if n == 0 {
		return nil
	}
	out := make([]int, n)
	for i := 0; i < n && i < len(inv.PanelWatts); i++ {
		out[i] = inv.PanelWatts[i]
	}
	return out
}

// nameplateByModel maps the per-inverter `model` integer (from `id.model`
// in the ECU's database.db) to its rated AC output in watts.
//
// Populated at startup from a JSON file (see LoadNameplateTable). The
// table is a lookup file — not hardcoded — so adding an entry for a new
// submodel doesn't require rebuilding the binary.
//
// Empty by default; lookups fall through to TypeCode-based defaults.
var nameplateByModel = map[int]int{}

// LoadNameplateTable replaces the model→watts table with the contents of
// a JSON file. Format:
//
//	{
//	  "models": {
//	    "24": {"nameplate_w": 1600, "label": "QS1A T24 (EU)"},
//	    "32": {"nameplate_w": 750,  "label": "DS3 EU"}
//	  }
//	}
//
// Returns the number of entries loaded. A missing file is not an error
// (returns 0, nil); only malformed JSON or unreadable file content errors.
//
// Derive entries by inspecting the per-inverter nameplate values that
// inv-driver publishes (codec.NameplateWattsForModel) and overriding
// them here when the site needs finer-grained per-submodel resolution.
func LoadNameplateTable(path string) (int, error) {
	if path == "" {
		return 0, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var doc struct {
		Models map[string]struct {
			NameplateW int    `json:"nameplate_w"`
			Label      string `json:"label"`
		} `json:"models"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return 0, fmt.Errorf("parse %s: %w", path, err)
	}
	tbl := make(map[int]int, len(doc.Models))
	for k, v := range doc.Models {
		mid, err := strconv.Atoi(k)
		if err != nil {
			return 0, fmt.Errorf("parse %s: model key %q is not an integer", path, k)
		}
		if v.NameplateW > 0 {
			tbl[mid] = v.NameplateW
		}
	}
	nameplateByModel = tbl
	return len(tbl), nil
}

// NameplateW is the rated AC output watts for this inverter.
//
// Lookup priority:
//  1. The model_int → watts table loaded from JSON via LoadNameplateTable
//  2. TypeCode-based default — coarser fallback when the table doesn't
//     cover this submodel (or no table file is configured)
//
// The Snapshot.SystemMaxPowerW value (from inv-driver's FleetSummary)
// is the authoritative fleet total when available; the per-inverter
// value here is what we surface in per-inverter SunSpec banks
// (UID 2..N+1) where a per-device value is required.
func (inv Inverter) NameplateW() int {
	if w, ok := nameplateByModel[inv.Model]; ok {
		return w
	}
	switch inv.TypeCode {
	case "01":
		return 730
	case "03":
		return 1460
	case "04":
		return 880
	}
	return 0
}

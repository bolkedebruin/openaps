package source

import (
	"context"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bolkedebruin/openaps/internal/sunspec/source/invdriver"
	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/wire"
)

// Builder composes a Snapshot from inv-driver telemetry plus the ECU's
// own /etc/yuneng identity files. Per-inverter data comes solely from
// inv-driver.
type Builder struct {
	YunengDir string

	ECUID    string
	Firmware string
	Model    string

	// InvDriverClient is the per-inverter telemetry source. It is the only
	// source; when nil, Build produces a snapshot with no inverters.
	InvDriverClient *invdriver.Client

	// InvDriverOnlineMaxAge is the staleness threshold for marking an
	// inv-driver-sourced inverter Online. Zero means 10 seconds.
	InvDriverOnlineMaxAge time.Duration

	// LimitCache supplies the per-inverter set-power cap (WMaxLimPct
	// read-back) from ecu-sunspec's own writes. nil → every inverter
	// reads as uncapped.
	LimitCache *PowerLimitCache
}

// NewBuilder helper that loads metadata from /etc/yuneng/*.conf if available.
// Missing files are tolerated — they're surfaced as empty strings.
func NewBuilder(yunengDir string) *Builder {
	b := &Builder{
		ECUID:     readTrim(yunengDir, "ecuid.conf"),
		Firmware:  readTrim(yunengDir, "version.conf"),
		Model:     readTrim(yunengDir, "model.conf"),
		YunengDir: yunengDir,
	}
	return b
}

func readTrim(dir, name string) string {
	if dir == "" {
		return ""
	}
	b, err := os.ReadFile(dir + "/" + name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// Build produces a fresh snapshot. Errors from optional sources are not fatal:
// they log into the snapshot's ECUID-prefixed missing fields as zeros and the
// caller decides what to surface upstream.
func (b *Builder) Build(ctx context.Context) (Snapshot, error) {
	s := Snapshot{
		Captured: time.Now(),
		ECUID:    b.ECUID,
		Firmware: b.Firmware,
		Model:    b.Model,
	}
	if v := readTrim(b.YunengDir, "polling_interval.conf"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.PollingInterval = n
		}
	}

	// Fleet aggregates: nameplate sum + lifetime/today/month/year
	// energy. All sourced from inv-driver's FleetSummary envelope —
	// inv-driver owns its own energy_lifetime + energy_period_anchor
	// tables. Zero values until the subscriber receives its first
	// FleetSummary (typically within one ZB cycle of attach).
	if b.InvDriverClient != nil {
		if f := b.InvDriverClient.Fleet(); f != nil {
			s.SystemMaxPowerW = int32(f.GetNameplateTotalW())
			s.LifetimeEnergyWh = f.GetLifetimeWh()
			s.TodayEnergyWh = f.GetTodayWh()
			s.MonthEnergyWh = f.GetMonthWh()
			s.YearEnergyWh = f.GetYearWh()
		}
	}

	// Per-inverter telemetry comes solely from inv-driver. Without a
	// client there is no source, so the snapshot carries no inverters.
	var invs []Inverter
	if b.InvDriverClient != nil {
		invs = b.invertersFromInvDriver()
	}

	// Grid-protection thresholds come solely from inv-driver's protection
	// read (it queries the inverter directly). Each UID's wire.Protection
	// overlays a zero base, so only codes inv-driver actually decoded are
	// present; the rest are dropped by SanitizeProtection below. Empty
	// until the first protection poll lands.
	if b.InvDriverClient != nil {
		for uid, p := range b.InvDriverClient.Protection() {
			if s.Protection == nil {
				s.Protection = make(map[string]ProtectionParams)
			}
			s.Protection[uid] = protectionFromWire(s.Protection[uid], p)
		}
	}

	// Report only read-back-verified protection: drop QS1A's unreadable
	// page-A trips (their 60code values are phantom — set blind and never
	// read back) and any physically out-of-range value. Dropped codes
	// become not-available in the SunSpec encoder.
	if s.Protection != nil {
		for _, inv := range invs {
			if pp, ok := s.Protection[inv.UID]; ok {
				s.Protection[inv.UID] = SanitizeProtection(pp, isQS1Family(inv))
			}
		}
	}

	var (
		freqSum float64
		freqN   int
		vSum    int
		vN      int
		tempMax int
		hasTemp bool
		online  int
	)
	for i := range invs {
		inv := &invs[i]
		if c := b.LimitCache; c != nil {
			if w, ok := c.Get(inv.UID); ok {
				inv.LimitedPowerW = w
			}
		}
		if inv.LimitedPowerW == 0 {
			inv.LimitedPowerW = MaxPanelLimitW // no recorded cap → uncapped
		}
		if inv.Online {
			online++
			if inv.FrequencyHz > 0 {
				freqSum += inv.FrequencyHz
				freqN++
			}
			if inv.ACVoltageV > 0 {
				vSum += inv.ACVoltageV
				vN++
			}
			if !hasTemp || inv.TemperatureC > tempMax {
				tempMax = inv.TemperatureC
				hasTemp = true
			}
		}
	}

	s.Inverters = invs
	s.InverterCount = len(invs)
	s.InverterOnlineCount = online
	if freqN > 0 {
		s.GridFrequencyHz = freqSum / float64(freqN)
	}
	if vN > 0 {
		s.GridVoltageV = float64(vSum) / float64(vN)
	}
	if hasTemp {
		s.MaxTemperatureC = tempMax
	}

	// SystemPowerW is the live sum of online inverters' AC power. This
	// is the ONLY source for the field — historical row tables freeze
	// at the pre-sunset value when inverters go quiet, so using the
	// live sum (even when zero) keeps the state and the W field
	// honest through dusk.
	s.SystemPowerW = liveSystemPowerW(invs)

	return s, nil
}

// liveSystemPowerW returns the current sum of online inverters' AC
// power, used as the authoritative source for Snapshot.SystemPowerW.
// Offline inverters never contribute, regardless of any stale
// ACPowerW reading their last params row may carry.
func liveSystemPowerW(invs []Inverter) int32 {
	var sum int
	for _, inv := range invs {
		if inv.Online {
			sum += inv.ACPowerW
		}
	}
	return int32(sum)
}

// invertersFromInvDriver materialises the per-inverter slice from the
// cached invdriver telemetry. Sorted by UID for stable bank ordering.
//
// TODO(temp): inv-driver does not carry per-inverter temperature
// yet — codec offset has not been pinned. See
// .wolf/codec_temperature_extraction.md for the probe recipe.
func (b *Builder) invertersFromInvDriver() []Inverter {
	maxAge := b.InvDriverOnlineMaxAge
	if maxAge <= 0 {
		maxAge = 10 * time.Second
	}
	all := b.InvDriverClient.All()
	uids := make([]string, 0, len(all))
	for uid := range all {
		uids = append(uids, uid)
	}
	sort.Strings(uids)
	now := time.Now()
	invs := make([]Inverter, 0, len(uids))
	for _, uid := range uids {
		snap := all[uid]
		tc := wire.TypeCodeForModel(snap.Model)
		if snap.ModelCode != nil {
			if mc := typeCodeFromModelCode(*snap.ModelCode); mc != "" {
				tc = mc
			}
		}
		inv := Inverter{
			UID:         uid,
			Online:      now.Sub(snap.FetchedAt) < maxAge,
			TypeCode:    tc,
			FrequencyHz: snap.ACFreq,
			ACVoltageV:  intRound(snap.ACVolts),
			ACPowerW:    intRound(snap.ACWatts),
		}
		if snap.ModelCode != nil {
			inv.Model = int(*snap.ModelCode)
		}
		if snap.SoftwareVersion != nil {
			inv.SoftwareVer = int(*snap.SoftwareVersion)
		}
		if snap.Phase != nil {
			inv.Phase = int(*snap.Phase)
		}
		if len(snap.Panels) > 0 {
			pw := make([]int, len(snap.Panels))
			for i, p := range snap.Panels {
				pw[i] = intRound(p.W)
			}
			inv.PanelWatts = pw
		}
		inv.Faults = snap.Faults
		// SignalStrength applies the clamp: a raw L1 byte < 0x10 is taken
		// as a sentinel for "no/unknown signal" and reported as 0xff.
		// Otherwise pass the byte through.
		if snap.RSSI > 0 {
			rssi := int(snap.RSSI)
			if rssi < 0x10 {
				rssi = 0xff
			}
			inv.SignalStrength = rssi
		}
		invs = append(invs, inv)
	}
	return invs
}

// typeCodeFromModelCode maps the inverter's numeric model_code (as
// reported in wire.InverterInfo) to the parameters_app.conf "type"
// column used elsewhere in the SunSpec layer. Unknown codes return ""
// so the caller falls back to a model-string-based lookup.
func typeCodeFromModelCode(model uint32) string {
	switch uint8(model) {
	case codec.ModelYC600, codec.ModelDS3, codec.ModelDS3H, codec.ModelDS3L:
		return "01"
	case codec.ModelQS1, codec.ModelQS1A:
		return "03"
	case codec.ModelQT2:
		return "05"
	}
	return ""
}

func intRound(f float64) int {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return int(math.Round(f))
}

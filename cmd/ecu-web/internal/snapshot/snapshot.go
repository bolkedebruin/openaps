// Package snapshot holds the in-memory projection of the inverter fleet
// that ecu-web serves to the browser. It is fed exclusively by the
// inv-driver UDS subscriber stream (Telemetry / InverterInfo /
// FleetSummary / Protection envelopes) and exposes JSON-friendly DTOs
// with derived fields (per-inverter nameplate cap, load %, online
// freshness). It owns no business logic: every authoritative value
// originates in inv-driver.
package snapshot

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/wire"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// onlineWindow is how long after the last telemetry frame an inverter is
// still considered online. The poller cadence is ~1s and a full poll
// cycle is ~15-20s; three missed cycles is a generous "offline" mark.
const onlineWindow = 60 * time.Second

// Snapshot is the concurrency-safe fleet model. The zero value is not
// usable; call New.
type Snapshot struct {
	mu        sync.RWMutex
	inverters map[string]*invState // keyed by peer_uid
	fleet     fleetAgg

	// onChange, when set, is invoked after every applied envelope so a
	// consumer (the SSE hub) can push a fresh view. Never called under
	// the lock.
	onChange func()
}

// invState is the merged per-inverter record. Telemetry and InverterInfo
// arrive on separate envelopes and are merged here by peer_uid.
type invState struct {
	uid         string
	shortAddr   uint32
	modelCode   uint8
	modelName   string
	phase       uint32
	swVersion   uint32
	zigbeeBound *bool
	turnedOff   *bool
	encrypted   *bool // last-observed L1 frame type: true=AES, false=plaintext, nil=unknown

	haveTelemetry bool
	lastSeenMs    int64
	gridV         float64
	busV          float64
	freqHz        float64
	activePowerW  float64
	reactiveVar   float64
	rssi          uint32
	lqi           uint32
	panels        []*wire.Panel
	faults        *wire.InverterFaults
	protection    *wire.Protection
}

// fleetAgg mirrors the inv-driver FleetSummary (nameplate + energy
// accumulators). Active power is summed from live telemetry here.
type fleetAgg struct {
	tsMs            int64
	nameplateTotalW uint32
	inverterCount   uint32
	lifetimeWh      uint64
	todayWh         uint64
	monthWh         uint64
	yearWh          uint64
	haveSummary     bool
}

// New returns an empty Snapshot. onChange may be nil.
func New(onChange func()) *Snapshot {
	return &Snapshot{
		inverters: make(map[string]*invState),
		onChange:  onChange,
	}
}

// SetOnChange installs the change callback after construction, so a
// consumer (e.g. the SSE hub) that needs the snapshot at build time can
// still register for updates. Not safe to call concurrently with Apply*.
func (s *Snapshot) SetOnChange(f func()) { s.onChange = f }

func (s *Snapshot) notify() {
	if s.onChange != nil {
		s.onChange()
	}
}

// inv returns the state for uid, creating it if absent. Caller holds mu.
func (s *Snapshot) inv(uid string) *invState {
	st := s.inverters[uid]
	if st == nil {
		st = &invState{uid: uid}
		s.inverters[uid] = st
	}
	return st
}

// ApplyTelemetry merges one decoded inverter reply.
func (s *Snapshot) ApplyTelemetry(t *wire.Telemetry) {
	if t == nil || t.GetPeerUid() == "" {
		return
	}
	s.mu.Lock()
	st := s.inv(t.GetPeerUid())
	st.haveTelemetry = true
	st.lastSeenMs = t.GetTsMs()
	st.shortAddr = t.GetShortAddr()
	if m := t.GetModel(); m != "" {
		st.modelName = m
	}
	st.gridV = t.GetGridV()
	st.busV = t.GetBusV()
	st.freqHz = t.GetFreqHz()
	st.activePowerW = t.GetActivePowerW()
	st.reactiveVar = t.GetReactiveVar()
	st.rssi = t.GetRssi()
	st.lqi = t.GetLqi()
	st.panels = t.GetPanels()
	if f := t.GetFaults(); f != nil {
		st.faults = f
	}
	if t.Encrypted != nil {
		v := t.GetEncrypted()
		st.encrypted = &v
	}
	s.mu.Unlock()
	s.notify()
}

// ApplyInfo merges identity / pair-state for one inverter.
func (s *Snapshot) ApplyInfo(in *wire.InverterInfo) {
	if in == nil || in.GetPeerUid() == "" {
		return
	}
	s.mu.Lock()
	st := s.inv(in.GetPeerUid())
	if in.GetShortAddr() != 0 {
		st.shortAddr = in.GetShortAddr()
	}
	if in.ModelCode != nil {
		st.modelCode = uint8(in.GetModelCode())
		if st.modelName == "" {
			st.modelName = modelName(st.modelCode)
		}
	}
	if in.SoftwareVersion != nil {
		st.swVersion = in.GetSoftwareVersion()
	}
	if in.Phase != nil {
		st.phase = in.GetPhase()
	}
	if in.ZigbeeBound != nil {
		v := in.GetZigbeeBound()
		st.zigbeeBound = &v
	}
	if in.TurnedOffRpt != nil {
		v := in.GetTurnedOffRpt()
		st.turnedOff = &v
	}
	if in.GetTsMs() > st.lastSeenMs {
		st.lastSeenMs = in.GetTsMs()
	}
	s.mu.Unlock()
	s.notify()
}

// ApplyFleet stores the inv-driver fleet aggregates.
func (s *Snapshot) ApplyFleet(f *wire.FleetSummary) {
	if f == nil {
		return
	}
	s.mu.Lock()
	s.fleet = fleetAgg{
		tsMs:            f.GetTsMs(),
		nameplateTotalW: f.GetNameplateTotalW(),
		inverterCount:   f.GetInverterCount(),
		lifetimeWh:      f.GetLifetimeWh(),
		todayWh:         f.GetTodayWh(),
		monthWh:         f.GetMonthWh(),
		yearWh:          f.GetYearWh(),
		haveSummary:     true,
	}
	s.mu.Unlock()
	s.notify()
}

// ApplyProtection stores the latest grid-protection readback for one
// inverter.
func (s *Snapshot) ApplyProtection(p *wire.Protection) {
	if p == nil || p.GetPeerUid() == "" {
		return
	}
	s.mu.Lock()
	st := s.inv(p.GetPeerUid())
	st.protection = p
	if st.modelName == "" {
		st.modelName = p.GetModel()
	}
	s.mu.Unlock()
	s.notify()
}

// Protection returns the last grid-protection readback for an inverter, or
// nil if none has arrived. Used to surface each inverter's current parameter
// values as overlay-editor defaults.
func (s *Snapshot) Protection(uid string) *wire.Protection {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st := s.inverters[uid]; st != nil {
		return st.protection
	}
	return nil
}

// --- read side: JSON DTOs ---

// PanelDTO mirrors one DC channel.
type PanelDTO struct {
	Index int32   `json:"index"`
	DCV   float64 `json:"dc_v"`
	DCA   float64 `json:"dc_a"`
	W     float64 `json:"w"`
}

// InverterDTO is the per-inverter view the dashboard renders.
type InverterDTO struct {
	UID         string  `json:"uid"`
	ShortAddr   uint32  `json:"short_addr"`
	Model       string  `json:"model"`
	ModelCode   uint8   `json:"model_code"`
	Phase       uint32  `json:"phase"`
	SWVersion   uint32  `json:"sw_version"`
	ZigbeeBound *bool   `json:"zigbee_bound,omitempty"`
	TurnedOff   *bool   `json:"turned_off,omitempty"`
	Encrypted   *bool   `json:"encrypted,omitempty"`
	Online      bool    `json:"online"`
	LastSeenMs  int64   `json:"last_seen_ms"`
	AgeS        float64 `json:"age_s"`

	ActivePowerW float64 `json:"active_power_w"`
	NameplateW   uint32  `json:"nameplate_w"`
	LoadPct      float64 `json:"load_pct"`
	GridV        float64 `json:"grid_v"`
	BusV         float64 `json:"bus_v"`
	FreqHz       float64 `json:"freq_hz"`
	ReactiveVar  float64 `json:"reactive_var"`
	RSSI         uint32  `json:"rssi"`
	LQI          uint32  `json:"lqi"`

	Panels     []PanelDTO      `json:"panels"`
	Faults     json.RawMessage `json:"faults,omitempty"`     // only-active bits
	Protection json.RawMessage `json:"protection,omitempty"` // present thresholds
}

// FleetDTO is the top-level dashboard payload.
type FleetDTO struct {
	TsMs            int64         `json:"ts_ms"`
	NameplateTotalW uint32        `json:"nameplate_total_w"`
	InverterCount   int           `json:"inverter_count"`
	OnlineCount     int           `json:"online_count"`
	ActivePowerW    float64       `json:"active_power_w"`
	LifetimeWh      uint64        `json:"lifetime_wh"`
	TodayWh         uint64        `json:"today_wh"`
	MonthWh         uint64        `json:"month_wh"`
	YearWh          uint64        `json:"year_wh"`
	Inverters       []InverterDTO `json:"inverters"`
}

var protoJSON = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: false}

// Fleet renders the current snapshot as the dashboard DTO. now is passed
// in for deterministic tests.
func (s *Snapshot) Fleet(now time.Time) FleetDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := FleetDTO{
		TsMs:            s.fleet.tsMs,
		NameplateTotalW: s.fleet.nameplateTotalW,
		LifetimeWh:      s.fleet.lifetimeWh,
		TodayWh:         s.fleet.todayWh,
		MonthWh:         s.fleet.monthWh,
		YearWh:          s.fleet.yearWh,
		Inverters:       make([]InverterDTO, 0, len(s.inverters)),
	}
	nowMs := now.UnixMilli()
	var sumActive float64
	online := 0
	for _, st := range s.inverters {
		d := st.toDTO(nowMs)
		if d.Online {
			online++
			sumActive += d.ActivePowerW
		}
		out.Inverters = append(out.Inverters, d)
	}
	// Stable order by UID so the UI doesn't reshuffle between frames
	// (map iteration order is randomised).
	sort.Slice(out.Inverters, func(i, j int) bool { return out.Inverters[i].UID < out.Inverters[j].UID })
	out.InverterCount = len(s.inverters)
	out.OnlineCount = online
	out.ActivePowerW = sumActive
	// Fall back to a locally-summed nameplate if the daemon hasn't sent a
	// FleetSummary yet.
	if !s.fleet.haveSummary {
		var sum uint32
		for _, st := range s.inverters {
			sum += codec.NameplateWattsForModel(st.modelCode)
		}
		out.NameplateTotalW = sum
	}
	return out
}

func (st *invState) toDTO(nowMs int64) InverterDTO {
	npW := codec.NameplateWattsForModel(st.modelCode)
	ageMs := nowMs - st.lastSeenMs
	if st.lastSeenMs == 0 {
		ageMs = 0
	}
	online := st.haveTelemetry && st.lastSeenMs != 0 &&
		ageMs <= onlineWindow.Milliseconds()

	d := InverterDTO{
		UID:          st.uid,
		ShortAddr:    st.shortAddr,
		Model:        st.modelName,
		ModelCode:    st.modelCode,
		Phase:        st.phase,
		SWVersion:    st.swVersion,
		ZigbeeBound:  st.zigbeeBound,
		TurnedOff:    st.turnedOff,
		Encrypted:    st.encrypted,
		Online:       online,
		LastSeenMs:   st.lastSeenMs,
		AgeS:         float64(ageMs) / 1000.0,
		ActivePowerW: st.activePowerW,
		NameplateW:   npW,
		GridV:        st.gridV,
		BusV:         st.busV,
		FreqHz:       st.freqHz,
		ReactiveVar:  st.reactiveVar,
		RSSI:         st.rssi,
		LQI:          st.lqi,
		Panels:       make([]PanelDTO, 0, len(st.panels)),
	}
	if d.Model == "" {
		d.Model = modelName(st.modelCode)
	}
	if npW > 0 {
		d.LoadPct = st.activePowerW / float64(npW) * 100.0
	}
	for _, p := range st.panels {
		d.Panels = append(d.Panels, PanelDTO{
			Index: p.GetIndex(), DCV: p.GetDcV(), DCA: p.GetDcI(), W: p.GetW(),
		})
	}
	// faults: emit only the active (true) named bits of the family
	// variant by protojson-marshalling the inner message.
	if inner := faultsInner(st.faults); inner != nil {
		if b, err := protoJSON.Marshal(inner); err == nil && string(b) != "{}" {
			d.Faults = b
		}
	}
	if st.protection != nil {
		// Emit the flat aps_code -> value map (Protection.values).
		if b, err := json.Marshal(st.protection.GetValues()); err == nil && string(b) != "null" {
			d.Protection = b
		}
	}
	return d
}

// faultsInner unwraps the family oneof to the concrete message so the
// emitted JSON is a flat object of active bits rather than nested under
// "ds3"/"qs1a". Returns nil when there are no faults to report.
func faultsInner(f *wire.InverterFaults) proto.Message {
	if f == nil {
		return nil
	}
	if ds3 := f.GetDs3(); ds3 != nil {
		return ds3
	}
	if qs1a := f.GetQs1A(); qs1a != nil {
		return qs1a
	}
	return nil
}

// modelName maps a raw model code to a human family name, matching
// codec's Telemetry.Model strings where they exist.
func modelName(code uint8) string {
	switch code {
	case codec.ModelYC600Old, codec.ModelYC600, codec.ModelYC600B:
		return "YC600"
	case codec.ModelYC1000:
		return "YC1000"
	case codec.ModelQS1:
		return "QS1"
	case codec.ModelQS1A:
		return "QS1A"
	case codec.ModelDS3:
		return "DS3"
	case codec.ModelDS3H:
		return "DS3-H"
	case codec.ModelDS3L:
		return "DS3-L"
	case codec.ModelQT2:
		return "QT2"
	case 0:
		return ""
	default:
		return "unknown"
	}
}

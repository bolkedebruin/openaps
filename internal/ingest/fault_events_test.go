package ingest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/store"
	"github.com/bolkedebruin/openaps/wire"
)

func ds3Frame(setters map[string]bool) *wire.InverterFaults {
	d := &wire.DS3Faults{}
	for name, v := range setters {
		switch name {
		case "grid_relay_fault":
			d.GridRelayFault = v
		case "dc_contactor_fault":
			d.DcContactorFault = v
		case "dc_bus_fault":
			d.DcBusFault = v
		case "dc_ground_fault":
			d.DcGroundFault = v
		case "iso_fault_a":
			d.IsoFaultA = v
		case "iso_fault_b":
			d.IsoFaultB = v
		case "ac_over_volt_stage1":
			d.AcOverVoltStage1 = v
		case "ac_over_volt_stage2":
			d.AcOverVoltStage2 = v
		case "ac_under_volt_stage1":
			d.AcUnderVoltStage1 = v
		case "ac_under_volt_stage2":
			d.AcUnderVoltStage2 = v
		case "over_freq_stage1":
			d.OverFreqStage1 = v
		case "over_freq_stage2":
			d.OverFreqStage2 = v
		case "over_freq_aux":
			d.OverFreqAux = v
		case "over_freq_extra":
			d.OverFreqExtra = v
		case "under_freq_stage1":
			d.UnderFreqStage1 = v
		case "under_freq_stage2":
			d.UnderFreqStage2 = v
		case "under_freq_aux":
			d.UnderFreqAux = v
		case "under_freq_extra":
			d.UnderFreqExtra = v
		}
	}
	return &wire.InverterFaults{Family: &wire.InverterFaults_Ds3{Ds3: d}}
}

func qs1aFrame(setters map[string]bool) *wire.InverterFaults {
	q := &wire.QS1AFaults{}
	for name, v := range setters {
		switch name {
		case "grid_relay_fault":
			q.GridRelayFault = v
		case "dc_contactor_fault":
			q.DcContactorFault = v
		case "dc_ground_fault":
			q.DcGroundFault = v
		case "dc_bus_fault":
			q.DcBusFault = v
		case "comm_fault":
			q.CommFault = v
		case "over_temperature":
			q.OverTemperature = v
		case "iso_fault_a":
			q.IsoFaultA = v
		case "ac_over_volt_fast":
			q.AcOverVoltFast = v
		case "over_freq_fast":
			q.OverFreqFast = v
		case "zb_link_a":
			q.ZbLinkA = v
		}
	}
	return &wire.InverterFaults{Family: &wire.InverterFaults_Qs1A{Qs1A: q}}
}

func loadEvents(t *testing.T, in *Ingestor) []store.Event {
	t.Helper()
	evs, err := in.S.QueryEvents(context.Background(), store.EventFilter{Limit: 1000})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	return evs
}

// fakeClock is a controllable wall clock for faultTracker tests. The
// returned now() function reads the latest set value, so tests can
// advance time between markFaults calls.
type fakeClock struct {
	t time.Time
}

func (c *fakeClock) now() time.Time { return c.t }

// setupFaultClock installs a fakeClock on the ingestor's fault tracker.
// markFaults is a no-op until called, so we trigger the lazy init by
// poking initFaults explicitly.
func setupFaultClock(in *Ingestor, start time.Time) *fakeClock {
	in.initFaults()
	c := &fakeClock{t: start}
	in.faults.now = c.now
	return c
}

// TestFaults_FirstFrameSeedsNoEmit pins design choice 1: the very first
// telemetry frame per uid silently seeds per-bit state. A persistent
// pre-existing raise must not replay as a fresh event on restart.
// Subsequent transitions (clear, then re-raise) DO emit normally.
func TestFaults_FirstFrameSeedsNoEmit(t *testing.T) {
	in := newIngestor(t)
	const uid = "AAAAAAAAAAAA"
	clk := setupFaultClock(in, time.Unix(1_000, 0))

	// Frame 1: bit already set. This is the seed — no event.
	in.markFaults(uid, 1_000_000, ds3Frame(map[string]bool{"over_freq_stage1": true}), 50.4, 248.1, 0)
	if evs := loadEvents(t, in); len(evs) != 0 {
		t.Fatalf("first frame should emit nothing (silent seed), got %d events: %v", len(evs), evs)
	}

	// Frame 2: bit clears. The seed recorded raised=true, so the clear
	// emits.
	clk.t = clk.t.Add(1 * time.Second)
	in.markFaults(uid, 1_001_000, ds3Frame(map[string]bool{}), 50.0, 230.0, 1200)
	evs := loadEvents(t, in)
	if len(evs) != 1 || evs[0].Kind != "fault_cleared" || evs[0].Severity != "info" {
		t.Fatalf("expected one fault_cleared(info) after seed-then-clear, got %v", evs)
	}

	// Frame 3: bit re-raises after the coalesce window has elapsed — emit.
	clk.t = clk.t.Add(faultCoalesceWindow + time.Second)
	in.markFaults(uid, 1_002_000, ds3Frame(map[string]bool{"over_freq_stage1": true}), 50.5, 248.0, 0)
	evs = loadEvents(t, in)
	if len(evs) != 2 {
		t.Fatalf("expected 2 events (clear + raise) after re-raise, got %d: %v", len(evs), evs)
	}
	if evs[1].Kind != "fault_raised" || evs[1].Severity != "warn" {
		t.Fatalf("re-raise should be fault_raised(warn), got %+v", evs[1])
	}
}

// TestFaults_SeededThenRaiseEmits is the post-seed contract: once the
// uid has been seeded with the bit OFF, a 0→1 transition emits a normal
// fault_raised, and the same bit still set on the next frame emits
// nothing.
func TestFaults_SeededThenRaiseEmits(t *testing.T) {
	in := newIngestor(t)
	const uid = "AAAAAAAAAAAA"
	ts := int64(1_000_000)

	// Frame 1 seeds with the bit OFF.
	in.markFaults(uid, ts, ds3Frame(map[string]bool{}), 50.0, 230.0, 0)
	if evs := loadEvents(t, in); len(evs) != 0 {
		t.Fatalf("seed frame must emit nothing, got %v", evs)
	}

	// Frame 2: bit goes 0→1 — emit.
	in.markFaults(uid, ts+1000, ds3Frame(map[string]bool{"over_freq_stage1": true}), 50.42, 248.1, 0)
	evs := loadEvents(t, in)
	if len(evs) != 1 {
		t.Fatalf("want 1 event after 0→1, got %d (%v)", len(evs), evs)
	}
	e := evs[0]
	if e.Kind != "fault_raised" || e.Severity != "warn" || e.By != "inv-driver" || e.InverterUID != uid {
		t.Fatalf("unexpected event header: %+v", e)
	}
	for _, want := range []string{"over_freq_stage1", "freq=50.4", "v=248.1", "w=0.0"} {
		if !strings.Contains(e.Detail, want) {
			t.Fatalf("detail %q missing %q", e.Detail, want)
		}
	}

	// Frame 3: bit still set — emit nothing.
	in.markFaults(uid, ts+2000, ds3Frame(map[string]bool{"over_freq_stage1": true}), 50.5, 248.0, 0)
	if got := loadEvents(t, in); len(got) != 1 {
		t.Fatalf("stable bit produced %d events, want 1: %v", len(got), got)
	}
}

// TestFaults_RaiseThenClear covers the clear path: a raised bit going
// 1→0 emits fault_cleared with severity=info.
func TestFaults_RaiseThenClear(t *testing.T) {
	in := newIngestor(t)
	const uid = "BBBBBBBBBBBB"
	ts := int64(2_000_000)

	// Seed with the bit OFF so the next 0→1 actually emits.
	in.markFaults(uid, ts, ds3Frame(map[string]bool{}), 50.0, 230.0, 0)
	in.markFaults(uid, ts+1000, ds3Frame(map[string]bool{"ac_over_volt_stage1": true}), 50.0, 253.0, 0)
	in.markFaults(uid, ts+6000, ds3Frame(map[string]bool{}), 50.0, 230.0, 1200)

	evs := loadEvents(t, in)
	if len(evs) != 2 {
		t.Fatalf("want 2 events, got %d: %v", len(evs), evs)
	}
	if evs[0].Kind != "fault_raised" || evs[0].Severity != "warn" {
		t.Fatalf("first event: %+v", evs[0])
	}
	if evs[1].Kind != "fault_cleared" || evs[1].Severity != "info" {
		t.Fatalf("clear event: %+v", evs[1])
	}
	if !strings.Contains(evs[1].Detail, "ac_over_volt_stage1") {
		t.Fatalf("clear detail %q lacks bit name", evs[1].Detail)
	}
	if !strings.Contains(evs[1].Detail, "w=1200.0") {
		t.Fatalf("clear detail %q lacks live W", evs[1].Detail)
	}
}

// TestFaults_FlapCoalescedWithin60s covers the flap-suppression rule:
// raise→clear→raise within 60s emits only the first two events, NOT a
// second raise. Coalesce is on the WALL CLOCK so the test drives an
// injected clock rather than telemetry tsMs.
func TestFaults_FlapCoalescedWithin60s(t *testing.T) {
	in := newIngestor(t)
	const uid = "CCCCCCCCCCCC"
	clk := setupFaultClock(in, time.Unix(3_000, 0))

	// Seed with bit OFF so we can later observe a real 0→1 raise.
	in.markFaults(uid, 3_000_000, ds3Frame(map[string]bool{}), 50.0, 230.0, 0)

	// t+0: raise.
	in.markFaults(uid, 3_000_001, ds3Frame(map[string]bool{"over_freq_stage1": true}), 50.4, 247.0, 0)
	// t+2s: clear.
	clk.t = clk.t.Add(2 * time.Second)
	in.markFaults(uid, 3_000_002, ds3Frame(map[string]bool{}), 50.0, 230.0, 500)
	// t+30s: re-raise. Wall-clock window is still <60s → suppressed.
	clk.t = clk.t.Add(30 * time.Second)
	in.markFaults(uid, 3_000_003, ds3Frame(map[string]bool{"over_freq_stage1": true}), 50.4, 247.0, 0)

	evs := loadEvents(t, in)
	raises := 0
	for _, e := range evs {
		if e.Kind == "fault_raised" {
			raises++
		}
	}
	if raises != 1 {
		t.Fatalf("want exactly 1 fault_raised in coalesce window, got %d: %v", raises, evs)
	}
}

// TestFaults_RaiseAfterCoalesceWindowEmits covers the recovery from a
// coalesce window: once 60s+ have passed since the clear (wall clock),
// the next raise fires normally.
func TestFaults_RaiseAfterCoalesceWindowEmits(t *testing.T) {
	in := newIngestor(t)
	const uid = "DDDDDDDDDDDD"
	clk := setupFaultClock(in, time.Unix(4_000, 0))

	// Seed OFF, then raise, then clear.
	in.markFaults(uid, 4_000_000, ds3Frame(map[string]bool{}), 50.0, 230.0, 0)
	in.markFaults(uid, 4_000_001, ds3Frame(map[string]bool{"over_freq_stage1": true}), 50.4, 247.0, 0)
	clk.t = clk.t.Add(1 * time.Second)
	in.markFaults(uid, 4_000_002, ds3Frame(map[string]bool{}), 50.0, 230.0, 500)
	// 61s after the clear: window has elapsed → emit.
	clk.t = clk.t.Add(faultCoalesceWindow + 100*time.Millisecond)
	in.markFaults(uid, 4_000_003, ds3Frame(map[string]bool{"over_freq_stage1": true}), 50.5, 248.0, 0)

	evs := loadEvents(t, in)
	raises := 0
	for _, e := range evs {
		if e.Kind == "fault_raised" {
			raises++
		}
	}
	if raises != 2 {
		t.Fatalf("want 2 raises (one before, one after window), got %d: %v", raises, evs)
	}
}

// TestFaults_TimeSourcePoisoningResistance pins design choice 2: the
// coalesce window uses wall clock, NOT telemetry tsMs. A
// forward-stamped clear timestamp must not suppress a subsequent real
// raise.
func TestFaults_TimeSourcePoisoningResistance(t *testing.T) {
	in := newIngestor(t)
	const uid = "PPPPPPPPPPPP"
	clk := setupFaultClock(in, time.Unix(10_000, 0))

	// Seed OFF, raise, clear — clear telemetry carries a far-future tsMs.
	in.markFaults(uid, 10_000_000, ds3Frame(map[string]bool{}), 50.0, 230.0, 0)
	in.markFaults(uid, 10_000_001, ds3Frame(map[string]bool{"over_freq_stage1": true}), 50.4, 247.0, 0)
	const year2099Ms int64 = 4_070_908_800_000
	in.markFaults(uid, year2099Ms, ds3Frame(map[string]bool{}), 50.0, 230.0, 0)

	// Wall-clock advances past the 60s coalesce window, then re-raise.
	// Under a tsMs-based coalesce, lastClearedMs would be the
	// year-2099 stamp, and the diff (nowMs - lastClearedMs) would be a
	// large negative number — < 60000 — so the raise would be
	// suppressed FOREVER. Under wall-clock coalesce, lastClearedMs
	// reflects real time, the window elapses, and the raise emits.
	clk.t = clk.t.Add(faultCoalesceWindow + time.Second)
	in.markFaults(uid, 10_000_002, ds3Frame(map[string]bool{"over_freq_stage1": true}), 50.4, 247.0, 0)

	evs := loadEvents(t, in)
	raises := 0
	for _, e := range evs {
		if e.Kind == "fault_raised" {
			raises++
		}
	}
	// One raise pre-poisoning + one raise post-poisoning ≥ 2 expected.
	if raises < 2 {
		t.Fatalf("wall-clock coalesce must ignore poisoned tsMs; want ≥2 raises, got %d: %v", raises, evs)
	}
}

// TestFaults_MultipleBitsInOneFrame covers the multi-bit case: more
// than one transition in a single frame yields one event per bit.
func TestFaults_MultipleBitsInOneFrame(t *testing.T) {
	in := newIngestor(t)
	const uid = "EEEEEEEEEEEE"
	ts := int64(5_000_000)

	// Seed OFF so the next frame's three 0→1 transitions all emit.
	in.markFaults(uid, ts, ds3Frame(map[string]bool{}), 50.0, 230.0, 0)
	in.markFaults(uid, ts+1000, ds3Frame(map[string]bool{
		"over_freq_stage1":    true,
		"ac_over_volt_stage1": true,
		"iso_fault_a":         true,
	}), 50.4, 252.0, 0)

	evs := loadEvents(t, in)
	if len(evs) != 3 {
		t.Fatalf("want 3 raise events, got %d: %v", len(evs), evs)
	}
	seen := map[string]string{}
	for _, e := range evs {
		if e.Kind != "fault_raised" || e.By != "inv-driver" {
			t.Fatalf("unexpected event: %+v", e)
		}
		bit := strings.SplitN(e.Detail, " ", 2)[0]
		seen[bit] = e.Severity
	}
	wantSev := map[string]string{
		"over_freq_stage1":    "warn",
		"ac_over_volt_stage1": "warn",
		"iso_fault_a":         "error",
	}
	for bit, sev := range wantSev {
		got, ok := seen[bit]
		if !ok {
			t.Fatalf("missing event for bit %q (got %v)", bit, seen)
		}
		if got != sev {
			t.Fatalf("severity for %q: got %q want %q", bit, got, sev)
		}
	}
}

// TestFaults_QS1AFamilyTracked covers the QS1A oneof path so we know
// both family decoders are wired.
func TestFaults_QS1AFamilyTracked(t *testing.T) {
	in := newIngestor(t)
	const uid = "FFFFFFFFFFFF"
	ts := int64(6_000_000)

	// Seed with both bits OFF so the next-frame raises actually emit.
	in.markFaults(uid, ts, qs1aFrame(map[string]bool{}), 50.0, 230.0, 0)
	in.markFaults(uid, ts+1000, qs1aFrame(map[string]bool{"over_freq_fast": true, "zb_link_a": true}), 50.3, 249.5, 200)
	in.markFaults(uid, ts+3000, qs1aFrame(map[string]bool{"zb_link_a": true}), 50.1, 248.0, 220)

	evs := loadEvents(t, in)
	if len(evs) != 3 {
		t.Fatalf("want 3 events (2 raises + 1 clear), got %d: %v", len(evs), evs)
	}
	// Find the clear, verify it's the over_freq_fast bit (zb_link_a still set).
	var clear *store.Event
	for i := range evs {
		if evs[i].Kind == "fault_cleared" {
			clear = &evs[i]
		}
	}
	if clear == nil {
		t.Fatalf("expected a fault_cleared event in %v", evs)
	}
	if !strings.Contains(clear.Detail, "over_freq_fast") {
		t.Fatalf("clear detail %q should reference over_freq_fast", clear.Detail)
	}
	if clear.Severity != "info" || clear.By != "inv-driver" {
		t.Fatalf("clear header: %+v", clear)
	}
}

// TestFaults_NilFamilyIsNoop covers the safety case where the codec
// couldn't classify a frame: no panics, no events.
func TestFaults_NilFamilyIsNoop(t *testing.T) {
	in := newIngestor(t)
	in.markFaults("GGGGGGGGGGGG", 7_000_000, nil, 50.0, 230.0, 0)
	in.markFaults("GGGGGGGGGGGG", 7_001_000, &wire.InverterFaults{}, 50.0, 230.0, 0)
	if evs := loadEvents(t, in); len(evs) != 0 {
		t.Fatalf("nil/empty family should emit nothing, got %v", evs)
	}
}

// TestFaults_SeverityMapping pins design choice 4: hardware-protective
// bits (DC bus / contactor / ground / relay / ISO) raise with
// severity=error; soft-grid bits raise with severity=warn; any clear
// is severity=info regardless of bit.
func TestFaults_SeverityMapping(t *testing.T) {
	in := newIngestor(t)
	ts := int64(8_000_000)

	cases := []struct {
		name    string
		uid     string
		frame   func(set map[string]bool) *wire.InverterFaults
		bit     string
		wantSev string
	}{
		{"DS3 dc_bus_fault → error", "U1DCBUSFAULT", ds3Frame, "dc_bus_fault", "error"},
		{"DS3 grid_relay_fault → error", "U1GRIDRELAY1", ds3Frame, "grid_relay_fault", "error"},
		{"DS3 dc_ground_fault → error", "U1DCGROUND12", ds3Frame, "dc_ground_fault", "error"},
		{"DS3 iso_fault_a → error", "U1ISOFAULTA12", ds3Frame, "iso_fault_a", "error"},
		{"DS3 over_freq_stage1 → warn", "U1OVERFREQ1A", ds3Frame, "over_freq_stage1", "warn"},
		{"DS3 ac_over_volt_stage1 → warn", "U1OVERVOLT1A", ds3Frame, "ac_over_volt_stage1", "warn"},
		{"QS1A iso_fault_a → error", "U2QSISOFAULT", qs1aFrame, "iso_fault_a", "error"},
		{"QS1A comm_fault → warn", "U2QSCOMMFAULT", qs1aFrame, "comm_fault", "warn"},
		{"QS1A over_temperature → warn", "U2QSOVERTEMP", qs1aFrame, "over_temperature", "warn"},
		{"QS1A zb_link_a → warn", "U2QSZBLINKA1", qs1aFrame, "zb_link_a", "warn"},
	}

	for _, tc := range cases {
		// Seed OFF, then raise the bit.
		in.markFaults(tc.uid, ts, tc.frame(map[string]bool{}), 50.0, 230.0, 0)
		in.markFaults(tc.uid, ts+1000, tc.frame(map[string]bool{tc.bit: true}), 50.0, 230.0, 0)
		// Then clear → severity=info regardless of bit.
		in.markFaults(tc.uid, ts+2000, tc.frame(map[string]bool{}), 50.0, 230.0, 0)
		ts += 10_000
	}

	evs := loadEvents(t, in)
	gotRaise := map[string]string{} // uid → severity of the raise event
	gotClear := map[string]string{} // uid → severity of the clear event
	for _, e := range evs {
		if e.Kind == "fault_raised" {
			gotRaise[e.InverterUID] = e.Severity
		}
		if e.Kind == "fault_cleared" {
			gotClear[e.InverterUID] = e.Severity
		}
	}

	for _, tc := range cases {
		if got := gotRaise[tc.uid]; got != tc.wantSev {
			t.Errorf("%s: raise severity for uid=%s bit=%s: got %q want %q", tc.name, tc.uid, tc.bit, got, tc.wantSev)
		}
		if got := gotClear[tc.uid]; got != "info" {
			t.Errorf("%s: clear severity for uid=%s: got %q want \"info\"", tc.name, tc.uid, got)
		}
	}
}

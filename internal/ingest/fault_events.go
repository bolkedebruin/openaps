package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bolkedebruin/openaps/wire"
)

// Fault-bit transition auditing.
//
// Each telemetry frame carries an InverterFaults oneof (DS3Faults or
// QS1AFaults). A bit flipping 0→1 is logged as a fault_raised event;
// 1→0 is logged as fault_cleared. The detail string captures the live
// grid context (freq / V / W) at the transition so an operator can
// reconstruct what the inverter was seeing when it tripped.
//
// Design choices locked in for this tracker:
//
//  1. First-frame seeding. The very first telemetry frame per uid is a
//     silent seed: per-uid state is recorded, but no fault_raised events
//     are emitted for bits already set. A persistent bit that was raised
//     before inv-driver was watching does NOT replay as a fresh event on
//     restart (or on first contact with a freshly-online inverter). The
//     subsequent clear will emit normally — the operator still sees the
//     recovery — and any later re-raise emits normally too.
//
//  2. Wall-clock coalesce. The 60s coalesce window uses wall clock
//     (now()), NOT the telemetry tsMs, so a forward- or backward-stamped
//     telemetry frame cannot suppress real raises indefinitely. Event
//     rows still carry the telemetry tsMs so the recorded timeline
//     reflects what the inverter saw.
//
//  3. Asymmetric coalesce. Re-raises within faultCoalesceWindow of a
//     clear are suppressed; clears themselves are NEVER coalesced. An
//     operator wants to see the bit cleared, even mid-flap.
//
//  4. Severity grain. fault_raised severity is "error" for
//     hardware-protective bits (DC bus / contactor / ground / relay /
//     ISO) and "warn" for soft-grid bits (over/under voltage stages,
//     over/under frequency stages, comm, temperature, zb_link).
//     fault_cleared is always "info".
const faultCoalesceWindow = 60 * time.Second

type faultTracker struct {
	mu    sync.Mutex
	state map[string]*uidFaults // uid → per-uid state

	// now is the wall-clock source for the coalesce window. Defaults to
	// time.Now; tests may override before any markFaults call.
	now func() time.Time
}

type uidFaults struct {
	// seeded is true once the first telemetry frame for this uid has
	// been observed. The first frame initialises the per-bit raised
	// flags but emits nothing — see design choice 1.
	seeded bool
	bits   map[string]*faultState
}

type faultState struct {
	raised        bool
	lastClearedMs int64 // wall-clock unix-ms of the most recent clear
}

type faultEmit struct {
	uid    string
	kind   string // "fault_raised" | "fault_cleared"
	bit    string
	detail string
}

// initFaults builds the per-Ingestor fault tracker exactly once. Safe to
// call from any goroutine; subsequent calls are no-ops. Used by tests to
// inject a custom wall clock by setting in.faults.now afterwards.
func (in *Ingestor) initFaults() {
	in.faultsOnce.Do(func() {
		in.faults = &faultTracker{
			state: make(map[string]*uidFaults),
			now:   time.Now,
		}
	})
}

// markFaults diffs the active bit set on the frame against the prior
// state for uid, emits raise/clear events for transitions, and writes
// them to the events log. Called from handleTelemetry; the lock is
// held only across the in-memory diff, never across the store write.
func (in *Ingestor) markFaults(uid string, tsMs int64, f *wire.InverterFaults, freqHz, gridV, activeW float64) {
	if uid == "" || tsMs <= 0 || f == nil {
		return
	}
	active := activeBitSet(f)
	if active == nil {
		// Family not classified — nothing to track.
		return
	}
	in.initFaults()

	nowMs := in.faults.now().UnixMilli()

	in.faults.mu.Lock()
	per := in.faults.state[uid]
	if per == nil {
		per = &uidFaults{bits: make(map[string]*faultState)}
		in.faults.state[uid] = per
	}
	// Make sure every known bit for this family has a state entry so a
	// 1→0 transition on a bit we've never seen still registers.
	for name := range active {
		if per.bits[name] == nil {
			per.bits[name] = &faultState{}
		}
	}

	var emits []faultEmit
	if !per.seeded {
		// First frame for this uid: seed every bit silently. A
		// persistent pre-existing raise must not replay on restart or on
		// first contact.
		for name, st := range per.bits {
			st.raised = active[name]
		}
		per.seeded = true
	} else {
		for name, st := range per.bits {
			now := active[name]
			switch {
			case now && !st.raised:
				// Coalesce: if this bit cleared recently (wall clock), swallow
				// the new raise. The bit stays "raised" internally so the
				// next clear re-arms the cycle.
				if st.lastClearedMs > 0 && nowMs-st.lastClearedMs < faultCoalesceWindow.Milliseconds() {
					st.raised = true
					continue
				}
				st.raised = true
				emits = append(emits, faultEmit{
					uid:    uid,
					kind:   "fault_raised",
					bit:    name,
					detail: formatFaultDetail(name, freqHz, gridV, activeW),
				})
			case !now && st.raised:
				st.raised = false
				st.lastClearedMs = nowMs
				emits = append(emits, faultEmit{
					uid:    uid,
					kind:   "fault_cleared",
					bit:    name,
					detail: formatFaultDetail(name, freqHz, gridV, activeW),
				})
			}
		}
	}
	in.faults.mu.Unlock()

	if len(emits) == 0 || in.S == nil {
		return
	}
	ctx := context.Background()
	for _, e := range emits {
		sev := "info"
		if e.kind == "fault_raised" {
			sev = faultSeverity(e.bit)
		}
		if err := in.S.AppendEvent(ctx, tsMs, e.uid, e.kind, sev, "inv-driver", e.detail); err != nil {
			slog.Error("ingest fault event", "kind", e.kind, "uid", e.uid, "bit", e.bit, "err", err)
		}
	}
}

// formatFaultDetail renders the per-event detail string. Hz/V use one
// decimal; W uses one decimal as well (telemetry already reports
// whole-watt granularity but the format stays uniform).
func formatFaultDetail(bit string, freqHz, gridV, activeW float64) string {
	return fmt.Sprintf("%s freq=%.1f v=%.1f w=%.1f", bit, freqHz, gridV, activeW)
}

// faultSeverity classifies a fault bit name into an event severity.
//
// "error" covers the hardware-protective bits — DC bus / contactor /
// ground, AC grid relay, and isolation (ISO) faults — where the
// inverter took itself offline to protect itself or the grid. These
// indicate an internal abnormality that an operator should look at.
//
// "warn" covers the soft-grid bits — voltage / frequency stage trips,
// comm, over-temperature, ZigBee link — where the inverter responded
// to an external condition (grid out of envelope, radio loss). These
// usually self-clear and don't need an operator. fault_cleared is
// always "info" regardless of bit and is handled by the caller.
func faultSeverity(bit string) string {
	switch bit {
	case
		"grid_relay_fault",
		"dc_contactor_fault",
		"dc_bus_fault",
		"dc_ground_fault",
		"iso_fault_a",
		"iso_fault_b",
		"iso_fault_c",
		"iso_fault_d":
		return "error"
	default:
		return "warn"
	}
}

// activeBitSet returns bitName→bool for the family carried by f. The
// bit-name strings match the proto json names (snake_case) so the
// frontend / events log are consistent with the wire schema. Returns
// nil if the family oneof is unset (codec couldn't classify the
// frame).
//
// Bit lists kept in sync with proto/busmgr.proto messages DS3Faults
// (18 bits) and QS1AFaults (24 bits).
func activeBitSet(f *wire.InverterFaults) map[string]bool {
	switch x := f.GetFamily().(type) {
	case *wire.InverterFaults_Ds3:
		return ds3BitSet(x.Ds3)
	case *wire.InverterFaults_Qs1A:
		return qs1aBitSet(x.Qs1A)
	default:
		return nil
	}
}

func ds3BitSet(d *wire.DS3Faults) map[string]bool {
	if d == nil {
		return nil
	}
	return map[string]bool{
		"grid_relay_fault":     d.GetGridRelayFault(),
		"dc_contactor_fault":   d.GetDcContactorFault(),
		"dc_bus_fault":         d.GetDcBusFault(),
		"dc_ground_fault":      d.GetDcGroundFault(),
		"iso_fault_a":          d.GetIsoFaultA(),
		"iso_fault_b":          d.GetIsoFaultB(),
		"ac_over_volt_stage1":  d.GetAcOverVoltStage1(),
		"ac_over_volt_stage2":  d.GetAcOverVoltStage2(),
		"ac_under_volt_stage1": d.GetAcUnderVoltStage1(),
		"ac_under_volt_stage2": d.GetAcUnderVoltStage2(),
		"over_freq_stage1":     d.GetOverFreqStage1(),
		"over_freq_stage2":     d.GetOverFreqStage2(),
		"over_freq_aux":        d.GetOverFreqAux(),
		"over_freq_extra":      d.GetOverFreqExtra(),
		"under_freq_stage1":    d.GetUnderFreqStage1(),
		"under_freq_stage2":    d.GetUnderFreqStage2(),
		"under_freq_aux":       d.GetUnderFreqAux(),
		"under_freq_extra":     d.GetUnderFreqExtra(),
	}
}

func qs1aBitSet(q *wire.QS1AFaults) map[string]bool {
	if q == nil {
		return nil
	}
	return map[string]bool{
		"grid_relay_fault":   q.GetGridRelayFault(),
		"dc_contactor_fault": q.GetDcContactorFault(),
		"dc_ground_fault":    q.GetDcGroundFault(),
		"dc_bus_fault":       q.GetDcBusFault(),
		"comm_fault":         q.GetCommFault(),
		"over_temperature":   q.GetOverTemperature(),
		"iso_fault_a":        q.GetIsoFaultA(),
		"iso_fault_b":        q.GetIsoFaultB(),
		"iso_fault_c":        q.GetIsoFaultC(),
		"iso_fault_d":        q.GetIsoFaultD(),
		"ac_over_volt_fast":  q.GetAcOverVoltFast(),
		"ac_over_volt_slow":  q.GetAcOverVoltSlow(),
		"ac_under_volt_fast": q.GetAcUnderVoltFast(),
		"ac_under_volt_slow": q.GetAcUnderVoltSlow(),
		"over_freq_fast":     q.GetOverFreqFast(),
		"over_freq_slow":     q.GetOverFreqSlow(),
		"over_freq_extra":    q.GetOverFreqExtra(),
		"over_freq_rms":      q.GetOverFreqRms(),
		"under_freq_fast":    q.GetUnderFreqFast(),
		"under_freq_slow":    q.GetUnderFreqSlow(),
		"under_freq_extra":   q.GetUnderFreqExtra(),
		"under_freq_rms":     q.GetUnderFreqRms(),
		"zb_link_a":          q.GetZbLinkA(),
		"zb_link_b":          q.GetZbLinkB(),
	}
}

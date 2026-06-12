package modem

import (
	"context"
	"log"
	"sync"
	"time"
)

// Default watchdog timings. The cadence is deliberately in minutes: a probe
// reads through the pairing redirect sink, so a telemetry frame that lands
// mid-probe is consumed by the ack scan and lost. At a few-minute interval a
// probe eats at most ~one frame and the next 1s poll re-fetches it. While the
// bus stays silent the watchdog re-probes once per interval, leaving telemetry
// a clear window in between to stamp inbound activity and end the silence.
const (
	defaultWatchdogSilence  = 5 * time.Minute
	defaultWatchdogInterval = 2 * time.Minute
	defaultWatchdogCooldown = 3 * time.Minute
)

// WatchdogClock returns the current time; injectable so tests drive logic
// against a manual clock instead of the wall clock.
type WatchdogClock func() time.Time

// Watchdog re-arms the modem when it wedges at runtime. The splice owns the
// single modem reader and the bring-up runs only once at startup, so a module
// or UART that hangs after the fleet sleeps overnight stays dead until reboot.
// The watchdog watches inbound activity and, when the bus has been silent long
// enough, probes the module with a 0x0D ping. A healthy module acks even when
// every inverter is off — that ack is the night-vs-wedge discriminator: an
// alive probe means the silence is just sleeping inverters and nothing is
// reset; a dead probe means the module is wedged and Recover (HW reset +
// Set-PANID re-arm) runs.
//
// Probe, Recover, Now and Log are injected so the core is hardware-free and
// fully testable. Zero-valued duration/func fields take the documented
// defaults at Run.
type Watchdog struct {
	// Probe pings the module (0x0D). alive is true iff the module acked; a
	// dead module reports alive=false with a nil err (that is the signal, not
	// a failure). err is a real transport/write error only.
	Probe func(context.Context) (alive bool, err error)
	// Recover hardware-resets the radio and re-sends Set-PANID+Channel.
	Recover func(context.Context) error
	// Silence is the inbound-silence required before a probe is allowed.
	Silence time.Duration
	// Interval is the check/probe cadence.
	Interval time.Duration
	// Cooldown is the quiet window after a recovery during which no probe
	// runs, letting the fleet rejoin before we judge the bus again.
	Cooldown time.Duration
	// Now sources the current time; defaults to time.Now.
	Now WatchdogClock
	// Log records watchdog activity; defaults to log.Printf.
	Log func(string, ...any)

	mu          sync.Mutex
	lastInbound time.Time
	lastRecover time.Time
	started     bool
}

// NoteInbound stamps the most recent inbound modem activity. It runs on the
// telemetry hot path (RawReplySink), so it does only a clock read under the
// mutex and is safe to call concurrently with Run.
func (w *Watchdog) NoteInbound() {
	now := w.now()
	w.mu.Lock()
	w.lastInbound = now
	w.mu.Unlock()
}

// now returns the injected clock, defaulting to time.Now.
func (w *Watchdog) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

// logf logs via the injected logger, defaulting to log.Printf.
func (w *Watchdog) logf(format string, args ...any) {
	if w.Log != nil {
		w.Log(format, args...)
		return
	}
	log.Printf(format, args...)
}

// defaults fills zero-valued timings with their documented values. Called once
// at Run start under the mutex.
func (w *Watchdog) defaults() {
	if w.Silence <= 0 {
		w.Silence = defaultWatchdogSilence
	}
	if w.Interval <= 0 {
		w.Interval = defaultWatchdogInterval
	}
	if w.Cooldown <= 0 {
		w.Cooldown = defaultWatchdogCooldown
	}
}

// action is the decision the watchdog makes on a tick, kept separate from the
// side effects so the decision is a pure function of the clock and the
// recorded windows.
type action int

const (
	// actionSkip means leave the module alone this tick (in cooldown, or the
	// bus is still active).
	actionSkip action = iota
	// actionProbe means the bus has been silent long enough to ping the
	// module and find out whether it is asleep or wedged.
	actionProbe
)

// tickDecision is the pure tick logic: given the current time it decides
// whether to skip or probe, reading only the recorded windows. It takes the
// mutex to read lastInbound/lastRecover consistently with NoteInbound but has
// no side effects, so a test can drive it directly without timers.
//
// Order matters: cooldown first (don't judge the bus while the fleet is
// rejoining after a reset), then activity (a recently active bus is healthy by
// definition), else probe.
func (w *Watchdog) tickDecision(now time.Time) action {
	w.mu.Lock()
	lastInbound := w.lastInbound
	lastRecover := w.lastRecover
	w.mu.Unlock()

	if !lastRecover.IsZero() && now.Sub(lastRecover) < w.Cooldown {
		return actionSkip
	}
	if now.Sub(lastInbound) < w.Silence {
		return actionSkip
	}
	return actionProbe
}

// Run drives the watchdog until ctx is cancelled. It stamps inbound activity
// up front so the first probe can't fire before the fleet has had a chance to
// report, then on each Interval tick decides (tickDecision) whether to probe.
// An alive probe does nothing — the silence is just sleeping inverters. A
// transport error is inconclusive and is retried next tick. A dead probe runs
// Recover; on success both windows reset, and on failure only the recover
// window resets so we back off instead of reset-storming. Probes never
// overlap, and ctx cancellation returns promptly even while waiting on the
// ticker.
func (w *Watchdog) Run(ctx context.Context) {
	w.mu.Lock()
	w.defaults()
	w.lastInbound = w.now()
	w.started = true
	interval := w.Interval
	w.mu.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.tickDecision(w.now()) != actionProbe {
				continue
			}
			w.runProbe(ctx)
		}
	}
}

// runProbe pings the module and acts on the result. Separated from Run so the
// single sequential probe path is testable on its own.
func (w *Watchdog) runProbe(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	alive, err := w.Probe(ctx)
	if err != nil {
		// Inconclusive — a transport error tells us nothing about whether the
		// module is wedged, so never reset on it; try again next tick.
		w.logf("watchdog: probe error (inconclusive, not resetting): %v", err)
		return
	}
	if alive {
		// Module acks 0x0D: it is fine, the inverters are simply asleep or
		// genuinely offline. No reset.
		return
	}

	if ctx.Err() != nil {
		// Shutting down: don't start a hardware reset we can't finish cleanly.
		return
	}
	w.logf("watchdog: module unresponsive to 0x0D; recovering")
	rerr := w.Recover(ctx)
	now := w.now()
	w.mu.Lock()
	// Always stamp lastRecover so the cooldown backs us off even on a failed
	// recovery, preventing a reset storm.
	w.lastRecover = now
	if rerr == nil {
		// A clean re-arm: treat the bus as fresh so we give the fleet a chance
		// to rejoin before probing again.
		w.lastInbound = now
	}
	w.mu.Unlock()
	if rerr != nil {
		w.logf("watchdog: recovery failed (backing off): %v", rerr)
	}
}

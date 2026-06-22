package ingest

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Online/offline + sunrise/sundown auditing.
//
// An inverter is "raw online" when telemetry has arrived within onlineWindow
// (matches ecu-web's snapshot window). A change to "online" emits immediately
// (so a sunrise lands within one telemetry frame of dawn). A change to
// "offline" is debounced by offlineDebounce so a transient radio dropout
// doesn't spam the log. Fleet-level "sunrise" / "sundown" track the debounced
// "any inverter online" flag.
const (
	onlineWindow     = 60 * time.Second
	offlineDebounce  = 5 * time.Minute
	onlineSweepEvery = 30 * time.Second
)

type onlineTracker struct {
	mu          sync.Mutex
	lastSeen    map[string]int64 // uid → ms
	state       map[string]*invOnline
	fleetOnline bool
}

type invOnline struct {
	online         bool  // last emitted state
	offlineSinceMs int64 // when raw went offline; 0 when raw is online
}

// markSeen records a fresh telemetry frame for uid. Called from
// handleTelemetry; very hot path, so it's a single map write under a small
// mutex.
func (in *Ingestor) markSeen(uid string, tsMs int64) {
	if uid == "" || tsMs <= 0 {
		return
	}
	in.ensureOnline()
	in.online.mu.Lock()
	in.online.lastSeen[uid] = tsMs
	in.online.mu.Unlock()
}

func (in *Ingestor) ensureOnline() {
	in.onlineOnce.Do(func() {
		in.online = &onlineTracker{
			lastSeen: make(map[string]int64),
			state:    make(map[string]*invOnline),
		}
	})
}

// StartOnlineSweep launches the periodic sweep that emits online/offline
// transitions (debounced) and fleet sunrise/sundown. Blocks until ctx is done.
// Safe to call once at boot from a goroutine.
func (in *Ingestor) StartOnlineSweep(ctx context.Context) {
	in.ensureOnline()
	t := time.NewTicker(onlineSweepEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			in.onlineSweep(ctx, now.UnixMilli())
		}
	}
}

type onlineEmit struct {
	uid  string
	kind string
}

// onlineSweep evaluates each known uid against the window/debounce and emits
// any state-change events to the store. Emits are computed under the lock and
// written without it so a slow store doesn't stall the tracker.
func (in *Ingestor) onlineSweep(ctx context.Context, now int64) {
	if in.S == nil {
		return
	}
	in.ensureOnline()
	in.online.mu.Lock()
	var emits []onlineEmit
	anyOnline := false
	for uid, last := range in.online.lastSeen {
		rawOnline := (now - last) <= onlineWindow.Milliseconds()
		st := in.online.state[uid]
		if st == nil {
			st = &invOnline{} // online=false, offlineSinceMs=0
			in.online.state[uid] = st
		}
		if rawOnline {
			st.offlineSinceMs = 0
			if !st.online {
				st.online = true
				emits = append(emits, onlineEmit{uid, "online"})
			}
		} else {
			if st.online {
				// Anchor offlineSince at the moment telemetry actually stopped
				// (lastSeen + onlineWindow), not at sweep-detection time, so the
				// debounce reflects the real outage duration rather than sweep
				// latency.
				start := last + onlineWindow.Milliseconds()
				if st.offlineSinceMs == 0 || start < st.offlineSinceMs {
					st.offlineSinceMs = start
				}
				if now-st.offlineSinceMs >= offlineDebounce.Milliseconds() {
					st.online = false
					emits = append(emits, onlineEmit{uid, "offline"})
				}
			}
		}
		if st.online {
			anyOnline = true
		}
	}
	if anyOnline != in.online.fleetOnline {
		kind := "sundown"
		if anyOnline {
			kind = "sunrise"
		}
		in.online.fleetOnline = anyOnline
		emits = append(emits, onlineEmit{"", kind})
	}
	in.online.mu.Unlock()

	for _, e := range emits {
		if err := in.S.AppendEvent(ctx, now, e.uid, e.kind, "info", "inv-driver", ""); err != nil {
			slog.Error("ingest online event", "kind", e.kind, "uid", e.uid, "err", err)
		}
	}
}

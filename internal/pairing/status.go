package pairing

import (
	"encoding/json"
	"sync"
	"time"
)

// Stage values for PairingStatus.Stage. They match the contract §Channel 1
// enum exactly so the ecu-web TS mirror can switch on them.
const (
	StageScan      = "scan"
	StageBind      = "bind"
	StageMigrate   = "migrate"
	StageConfigure = "configure"
	StageRekey     = "rekey"
	StageDone      = "done"
	StageAborted   = "aborted"
	StageError     = "error"
)

// Op values for PairingStatus.Op.
const (
	OpScan    = "scan"
	OpAdd     = "add"
	OpReplace = "replace"
	OpRekey   = "rekey"
)

// PerInverter is one inverter's progress within the current op. State is a
// free-text per-inverter stage (e.g. "found", "bound", "migrated",
// "configured", "error"). Encrypted mirrors the last-observed frame type.
type PerInverter struct {
	Serial    string `json:"serial"`
	ShortAddr uint32 `json:"short_addr"`
	State     string `json:"state"`
	Encrypted bool   `json:"encrypted"`
}

// Sweep describes the channel-sweep position during a slow scan / rekey.
type Sweep struct {
	Chan   uint32 `json:"chan"`
	ChanLo uint32 `json:"chan_lo"`
	ChanHi uint32 `json:"chan_hi"`
}

// PairingStatus is the in-memory snapshot of the running pairing op,
// returned verbatim (as JSON) by get_status. It is the source of truth for
// the UI progress drawer; fine-grained progress lives here and is NOT
// event-logged (see contract §Channel 1).
type PairingStatus struct {
	Op            string        `json:"op"`
	Stage         string        `json:"stage"`
	Total         int           `json:"total"`
	Done          int           `json:"done"`
	CurrentSerial string        `json:"current_serial"`
	Substep       string        `json:"substep"`
	Sweep         Sweep         `json:"sweep"`
	PerInverter   []PerInverter `json:"per_inverter"`
	Message       string        `json:"message"`
	Error         string        `json:"error"`
	StartedMs     int64         `json:"started_ms"`
	UpdatedMs     int64         `json:"updated_ms"`
}

// statusTracker is the concurrency-safe holder of the current PairingStatus.
// The state machine (one goroutine) mutates it; get_status reads a JSON
// snapshot from the IPC goroutine.
type statusTracker struct {
	mu sync.RWMutex
	st PairingStatus
	// by attributes the current op's milestone events to the controller
	// backend that started it. Captured at begin (under mu) and read by
	// milestone (under mu) so a concurrent get_status/abort can't race it.
	by string
}

// begin resets the tracker for a fresh op, captures the milestone
// attribution, and stamps the start time.
func (t *statusTracker) begin(op, stage string, total int, by string) {
	now := time.Now().UnixMilli()
	t.mu.Lock()
	t.st = PairingStatus{
		Op:        op,
		Stage:     stage,
		Total:     total,
		StartedMs: now,
		UpdatedMs: now,
	}
	t.by = by
	t.mu.Unlock()
}

// attribution returns the captured milestone "by" attribution.
func (t *statusTracker) attribution() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.by
}

// update applies a mutation under the lock and bumps UpdatedMs.
func (t *statusTracker) update(fn func(s *PairingStatus)) {
	t.mu.Lock()
	fn(&t.st)
	t.st.UpdatedMs = time.Now().UnixMilli()
	t.mu.Unlock()
}

// setStage is a convenience for the common stage+substep transition.
func (t *statusTracker) setStage(stage, substep string) {
	t.update(func(s *PairingStatus) {
		s.Stage = stage
		s.Substep = substep
	})
}

// finishError marks the op failed with a message.
func (t *statusTracker) finishError(msg string) {
	t.update(func(s *PairingStatus) {
		s.Stage = StageError
		s.Error = msg
	})
}

// finishAborted marks the op aborted.
func (t *statusTracker) finishAborted() {
	t.update(func(s *PairingStatus) {
		s.Stage = StageAborted
		s.Message = "aborted by operator"
	})
}

// finishDone marks the op complete.
func (t *statusTracker) finishDone(msg string) {
	t.update(func(s *PairingStatus) {
		s.Stage = StageDone
		s.Done = s.Total
		s.Message = msg
	})
}

// snapshot returns a deep copy of the current status.
func (t *statusTracker) snapshot() PairingStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	cp := t.st
	if t.st.PerInverter != nil {
		cp.PerInverter = make([]PerInverter, len(t.st.PerInverter))
		copy(cp.PerInverter, t.st.PerInverter)
	}
	return cp
}

// snapshotJSON returns the current status serialised as JSON.
func (t *statusTracker) snapshotJSON() ([]byte, error) {
	return json.Marshal(t.snapshot())
}

// upsertInverter sets/updates a per-inverter progress row by serial.
func (t *statusTracker) upsertInverter(pi PerInverter) {
	t.update(func(s *PairingStatus) {
		for i := range s.PerInverter {
			if s.PerInverter[i].Serial == pi.Serial {
				if pi.ShortAddr != 0 {
					s.PerInverter[i].ShortAddr = pi.ShortAddr
				}
				if pi.State != "" {
					s.PerInverter[i].State = pi.State
				}
				s.PerInverter[i].Encrypted = pi.Encrypted
				return
			}
		}
		s.PerInverter = append(s.PerInverter, pi)
	})
}

// setInverterState updates only the State field for a serial.
func (t *statusTracker) setInverterState(serial, state string) {
	t.upsertInverter(PerInverter{Serial: serial, State: state})
}

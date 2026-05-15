// Package paramsfile writes /tmp/parameters_app.conf in the format
// main.exe uses, so ecu-sunspec (and any other consumer that parses
// that file) sees inv-driver's 5s-cadence telemetry instead of waiting
// for main.exe's ~9-minute poll cycle.
//
// The file format, observed on firmware 2.1.29D and parsed by
// ecu-sunspec/internal/source/params.go (ParseParamsApp):
//
//	01,<count>,<yyyymmddhhmmss>          ; line 1: proto, count, local-tz timestamp
//	<UID>,<online>,<type>,<freq>,<temp+100>,<col5..colN>
//	...                                  ; one line per inverter
//
// TypeCode mapping (from main.exe protocol_APS18 @ 0x32eb4):
//
//	"01"  →  DS3 / DS3D / YC600 (2-channel)
//	"03"  →  QS1 / QS1A (4-channel)
//	"04"  →  DS3-H / DS3D-L
//
// Per-inverter tail layouts (from ecu-sunspec/internal/source/params.go
// guessACFromTail; verified against historical_data.db.each_system_power):
//
//	type 01: <UID>,1,01,<freq>,<temp+100>,<P0>,<Vac>,<P1>,<Vac>
//	type 03: <UID>,1,03,<freq>,<temp+100>,<P0>,<Vac>,<P1>,<P2>,<P3>
//
// Offline inverter rows are 3 fields: <UID>,0,<type>. We emit those
// when an inverter hasn't replied within OfflineAfter.
//
// We do NOT lock against main.exe's writes — both processes use
// atomic temp+rename, so the worst case is that main.exe's
// 9-minute snapshot briefly overwrites our 5s data and ecu-sunspec
// reads the older sample for one cycle. Our next reply restores
// fresh telemetry.
package paramsfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bolke/inv-driver/wire"
)

// Updater accumulates the latest decoded telemetry per inverter UID
// and rewrites the params file atomically on each Update().
type Updater struct {
	// Path is the target file, e.g. /tmp/parameters_app.conf.
	Path string
	// OfflineAfter: an inverter is reported as offline when its last
	// successful reply is older than this. Default 30s.
	OfflineAfter time.Duration
	// Now is overridable for tests.
	Now func() time.Time

	mu sync.Mutex
	by map[string]*entry
}

type entry struct {
	UID      string
	TypeCode string
	Last     *wire.Telemetry
	LastAt   time.Time
}

// New returns an Updater ready to use.
func New(path string) *Updater {
	return &Updater{
		Path:         path,
		OfflineAfter: 30 * time.Second,
		Now:          time.Now,
		by:           make(map[string]*entry),
	}
}

// Update records a fresh telemetry frame for t.PeerUid and rewrites
// the file. Errors come back from the file write only — the in-memory
// state is updated unconditionally.
func (u *Updater) Update(t *wire.Telemetry) error {
	if t == nil {
		return errors.New("paramsfile: nil telemetry")
	}
	if t.GetPeerUid() == "" {
		return errors.New("paramsfile: telemetry has no peer UID")
	}
	tc := wire.TypeCodeForModel(t.GetModel())
	if tc == "" {
		return fmt.Errorf("paramsfile: no type code for model %q", t.GetModel())
	}
	u.mu.Lock()
	e, ok := u.by[t.GetPeerUid()]
	if !ok {
		e = &entry{UID: t.GetPeerUid(), TypeCode: tc}
		u.by[t.GetPeerUid()] = e
	}
	e.TypeCode = tc
	e.Last = t
	e.LastAt = u.now()
	u.mu.Unlock()
	return u.writeFile()
}

func (u *Updater) now() time.Time {
	if u.Now != nil {
		return u.Now()
	}
	return time.Now()
}

// writeFile renders the current state and atomically replaces u.Path.
func (u *Updater) writeFile() error {
	body := u.Render()

	dir := filepath.Dir(u.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// CreateTemp guarantees a unique path so two Update() calls
	// can't collide. We then rename atomically over u.Path.
	tmp, err := os.CreateTemp(dir, ".params-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleaned := false
	defer func() {
		if !cleaned {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, u.Path); err != nil {
		return err
	}
	cleaned = true
	return nil
}

// Render produces the file content as a string. Exposed for tests.
func (u *Updater) Render() string {
	u.mu.Lock()
	defer u.mu.Unlock()

	now := u.now()

	// Stable iteration order so the file is reproducible (and tests
	// don't flake). Sort by UID.
	uids := make([]string, 0, len(u.by))
	for uid := range u.by {
		uids = append(uids, uid)
	}
	sort.Strings(uids)

	var sb strings.Builder
	fmt.Fprintf(&sb, "01,%d,%s\n", len(uids), now.Format("20060102150405"))

	for _, uid := range uids {
		e := u.by[uid]
		if now.Sub(e.LastAt) > u.OfflineAfter {
			fmt.Fprintf(&sb, "%s,0,%s\n", e.UID, e.TypeCode)
			continue
		}
		writeInverterLine(&sb, e)
	}

	return sb.String()
}

func writeInverterLine(sb *strings.Builder, e *entry) {
	t := e.Last
	freq := truncFreq(t.GetFreqHz())
	tempCol := tempRawFor(e.TypeCode, t) // already includes the +100 offset
	gridV := intRound(t.GetGridV())
	panels := t.GetPanels()

	switch e.TypeCode {
	case "01": // DS3 / DS3D / YC600
		var p0, p1 int
		if len(panels) > 0 {
			p0 = intRound(panels[0].GetW())
		}
		if len(panels) > 1 {
			p1 = intRound(panels[1].GetW())
		}
		// tail = [tempRaw, P0, Vac, P1, Vac]
		fmt.Fprintf(sb, "%s,1,01,%s,%d,%d,%d,%d,%d\n",
			e.UID, freq, tempCol, p0, gridV, p1, gridV)

	case "03": // QS1 / QS1A
		var p [4]int
		for i := 0; i < 4 && i < len(panels); i++ {
			p[i] = intRound(panels[i].GetW())
		}
		// tail = [tempRaw, P0, Vac, P1, P2, P3]
		fmt.Fprintf(sb, "%s,1,03,%s,%d,%d,%d,%d,%d,%d\n",
			e.UID, freq, tempCol,
			p[0], gridV, p[1], p[2], p[3])
	}
}

// tempRawFor returns the integer to put in the temp-raw column.
// QS1A uses inv+0x4c which holds the bus voltage formula's output —
// in practice that scales to ~25 (degrees C-ish) and matches what
// main.exe writes. DS3 uses an NTC reading via a Steinhart-Hart
// formula we haven't ported yet; until we do, fall back to a
// neutral 25°C placeholder. The +100 offset is applied here.
func tempRawFor(typeCode string, t *wire.Telemetry) int {
	switch typeCode {
	case "03":
		return intRound(t.GetBusV()) + 100
	default:
		return 125
	}
}

// truncFreq formats freq the way main.exe does: one decimal,
// truncated (NOT rounded) since main.exe goes via int(freq*10).
func truncFreq(hz float64) string {
	if hz < 0 {
		hz = 0
	}
	deci := int64(hz * 10) // truncates toward zero
	return fmt.Sprintf("%d.%d", deci/10, deci%10)
}

// intRound rounds-half-away-from-zero to int.
func intRound(x float64) int {
	if x >= 0 {
		return int(x + 0.5)
	}
	return int(x - 0.5)
}

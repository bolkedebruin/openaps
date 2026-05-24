// Command inv-driver is the v0 daemon and CLI for ingesting backend
// telemetry (from ecu-zb / bus-mgr) over a Unix-domain socket into a
// SQLite state-store, plus two read-only dump subcommands for ops
// inspection.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bolke/inv-driver/internal/eventlog"
	"github.com/bolke/inv-driver/internal/events"
	"github.com/bolke/inv-driver/internal/gridprofile"
	"github.com/bolke/inv-driver/internal/ingest"
	"github.com/bolke/inv-driver/internal/ipc"
	"github.com/bolke/inv-driver/internal/probe"
	"github.com/bolke/inv-driver/internal/settings"
	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/internal/systemstatus"
	"github.com/bolke/inv-driver/internal/telemetrypoll"
	"github.com/bolke/inv-driver/wire"
)

const (
	defaultSocket = "/var/run/inv-driver.sock"
	defaultDB     = "/var/lib/inv-driver/state.db"
	// defaultSocketMode is restrictive by default; operators must opt
	// in to wider modes via -socket-mode if a sibling unprivileged
	// user needs access.
	defaultSocketMode = "0600"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	sub := os.Args[1]
	args := os.Args[2:]
	var err error
	switch sub {
	case "serve":
		err = runServe(args)
	case "dump-telemetry":
		err = runDumpTelemetry(args)
	case "dump-events":
		err = runDumpEvents(args)
	case "set-power":
		err = runSetPower(args)
	case "-h", "--help", "help":
		usage(os.Stdout)
		return
	default:
		fmt.Fprintf(os.Stderr, "inv-driver: unknown subcommand %q\n\n", sub)
		usage(os.Stderr)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "inv-driver %s: %v\n", sub, err)
		os.Exit(1)
	}
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `usage: inv-driver <subcommand> [flags]

subcommands:
  serve            Run the UDS ingestion daemon.
  dump-telemetry   Print the latest sample per inverter as JSON lines.
  dump-events      Print rows from the events table as JSON lines.
  set-power        Dispatch an inverter max-power command (per-UID or broadcast).

Run 'inv-driver <subcommand> -h' for subcommand-specific flags.
`)
}

const (
	defaultGridProfilesDir = "/var/lib/inv-driver/gridprofiles"
	// defaultSettings is inv-driver's own config file.
	defaultSettings = "/etc/inv-driver/settings.json"
)

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	defSock := defaultSocket
	if s := os.Getenv("INV_DRIVER_SOCK"); s != "" {
		defSock = s
	}
	socket := fs.String("socket", defSock, "UDS path to listen on (env INV_DRIVER_SOCK overrides default)")
	dbPath := fs.String("db", defaultDB, "SQLite state-store path")
	modeStr := fs.String("socket-mode", defaultSocketMode, "socket file mode (octal); default 0600")
	retainTelemetry := fs.Duration("retain-telemetry", 24*time.Hour, "max age of telemetry events before pruning (0 disables)")
	retainOther := fs.Duration("retain-other", 7*24*time.Hour, "max age of non-telemetry events before pruning (0 disables)")
	pruneInterval := fs.Duration("prune-interval", time.Hour, "how often to run event pruning (0 disables periodic prune)")
	probeBackend := fs.String("probe-backend", "apsystems-stock-zb", "backend name that handles cold-start info-query probes (empty disables)")
	probeInterval := fs.Duration("probe-interval", 200*time.Millisecond, "per-inverter delay between probe Sends")
	controllerBackends := fs.String("controller-backends", "inv-driver-cli",
		"comma-separated list of peer backends permitted to inject Envelope_Send / Envelope_Broadcast frames")
	controllerUIDs := fs.String("controller-uids", "0",
		"comma-separated list of OS user-ids whose UDS connections may inject Envelope_Send / Envelope_Broadcast frames (Linux SO_PEERCRED; empty disables UID gate)")
	gridProfilesDir := fs.String("gridprofiles-dir", defaultGridProfilesDir,
		"directory containing grid-profile JSON files (profiles/ and overlays/ subdirs)")
	settingsPath := fs.String("settings", defaultSettings,
		"inv-driver's settings file (ecu-id, mac, pan-override, zigbee-type)")
	gridProfileBroadcast := fs.Bool("gridprofile-broadcast", false,
		"enable broadcast whole-profile apply path in SelectBase (default false; requires on-wire validation)")
	telemetryPoll := fs.Bool("telemetry-poll", true,
		"enable periodic 0xBB telemetry queries originated by inv-driver (default true; disable only for diagnostics)")
	telemetryPollInterval := fs.Duration("telemetry-poll-interval", 1*time.Second,
		"cadence between 0xBB poll rounds; per-inverter Sends are spaced by -probe-interval within the round")
	if err := fs.Parse(args); err != nil {
		return err
	}
	controllers := splitTrim(*controllerBackends)
	uids, err := parseUIDList(*controllerUIDs)
	if err != nil {
		return fmt.Errorf("-controller-uids: %w", err)
	}

	mode, err := parseOctalMode(*modeStr)
	if err != nil {
		return fmt.Errorf("-socket-mode: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		return fmt.Errorf("mkdir db dir: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, *dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	// Apply gridprofile schema migrations (idempotent).
	if err := gridprofile.ApplySchema(ctx, st.DB()); err != nil {
		return fmt.Errorf("gridprofile schema: %w", err)
	}

	// Build the gridprofile store and load on-disk profiles/overlays.
	gpStore := gridprofile.NewStore(st.DB())
	profilesDir := filepath.Join(*gridProfilesDir, "profiles")
	overlaysDir := filepath.Join(*gridProfilesDir, "overlays")
	if err := gpStore.LoadProfilesFromDir(ctx, profilesDir); err != nil {
		// Non-fatal: missing dir on first install is expected.
		log.Printf("gridprofile: LoadProfilesFromDir %q: %v", profilesDir, err)
	}
	if err := gpStore.LoadOverlaysFromDir(ctx, overlaysDir); err != nil {
		log.Printf("gridprofile: LoadOverlaysFromDir %q: %v", overlaysDir, err)
	}

	pruneOnce(ctx, st, *retainTelemetry, *retainOther)
	if *pruneInterval > 0 {
		go pruneLoop(ctx, st, *retainTelemetry, *retainOther, *pruneInterval)
	}

	pub := events.New()
	// Buffered cap 1: multiple triggers during one probe Run coalesce
	// into a single re-run.
	trigger := make(chan struct{}, 1)
	srv := &ipc.Server{
		SocketPath: *socket,
		SocketMode: mode,
		Publisher:  pub,
		Store:      st,
	}
	ingestor := &ingest.Ingestor{
		S:                  st,
		Pub:                pub,
		Probe:              trigger,
		Router:             srv,
		RouteBackend:       *probeBackend,
		ControllerBackends: controllers,
		ControllerUIDs:     uids,
	}
	srv.Ingestor = ingestor

	// Wire the gridprofile VerifyStartup hook (non-blocking; fires once per
	// first-seen inverter after the protection read is enqueued).
	// The hook is set after gpReconciler is constructed below; declare a
	// pointer-capture so the closure always sees the final reconciler value.
	var gpReconcilerPtr *gridprofile.Reconciler

	// Wire the gridprofile manager. The Reconciler is component B; when it
	// is available it will be wired here. Until then, ops that require
	// reconciliation return an error from the manager.
	gpSender := &gridprofile.IPCSender{Router: srv, Backend: *probeBackend}
	gpReadback := &gridprofile.IngestReadback{Ingestor: ingestor}
	modelOf := func(uid string) (uint8, bool) {
		return modelCodeFromStore(ctx, st, uid)
	}
	gpReconciler := gridprofile.NewReconciler(gpStore, gpSender, gpReadback, modelOf, gridprofile.Options{
		AutoReassert:  true,
		ReadSettle:    4 * time.Second,
		PeriodicSweep: 0, // disabled until user selects a base
	})
	gpReconcilerPtr = gpReconciler

	// Wire the OnFirstSeen hook: on first telemetry from an inverter, run
	// VerifyStartup non-blocking after the protection read settles.
	ingestor.OnFirstSeen = func(uid string) {
		rec := gpReconcilerPtr
		if rec == nil {
			return
		}
		mc, ok := modelCodeFromStore(ctx, st, uid)
		if !ok {
			return
		}
		go func() {
			// Wait for the protection read pipeline to deliver the first pages.
			time.Sleep(2 * 4 * time.Second) // 2 × ReadSettle
			rpt, err := rec.VerifyStartup(ctx, uid, mc)
			if err != nil {
				log.Printf("gridprofile VerifyStartup uid=%s: %v", uid, err)
				return
			}
			log.Printf("gridprofile VerifyStartup uid=%s identified=%q score=%.2f desiredState=%v reconciledOnStartup=%v",
				uid, rpt.IdentifiedID, rpt.IdentifiedScore, rpt.HasDesiredState, rpt.ReconciledOnStartup)
		}()
	}
	gpBroadcaster := &gridprofile.IPCBroadcaster{Router: srv, Backend: *probeBackend}
	gpManager := &gridprofile.Manager{
		Store:       gpStore,
		Reconciler:  gpReconciler,
		ProfilesDir: profilesDir,
		OverlaysDir: overlaysDir,
		// Broadcast path defaults off; enable with -gridprofile-broadcast.
		// Targets DS3 and QS1A families (the two families the codec handles).
		BroadcastEnabled:    *gridProfileBroadcast,
		BroadcastSender:     gpBroadcaster,
		BroadcastModelCodes: []uint8{0x20, 0x18}, // ModelDS3=0x20, ModelQS1A=0x18
	}
	srv.GridProfile = gpManager

	// inv-driver's settings file (ecu-id, mac, pan, zigbee-type).
	settingsStore, err := settings.Open(*settingsPath)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}

	// SystemStatus: peer registry (fed by every Hello) + ECU identity
	// from inv-driver's settings.
	hostname, _ := os.Hostname()
	ssManager := systemstatus.NewManager(hostname, controllers)
	ssManager.Identity = func() *wire.EcuIdentity {
		s := settingsStore.Get()
		return &wire.EcuIdentity{EcuId: s.EcuID}
	}
	srv.SystemStatus = ssManager
	srv.Peers = ssManager

	// Events read API over the append-only events store.
	srv.Events = eventlog.NewHandler(st)

	// Settings read/write API over inv-driver's settings file.
	srv.Settings = settings.NewHandler(settingsStore)

	if *probeBackend != "" {
		p := &probe.Probe{
			Store:        st,
			Server:       srv,
			Backend:      *probeBackend,
			SendInterval: *probeInterval,
			Trigger:      trigger,
		}
		go p.RunLoop(ctx)
		srv.OnPublisherAttach = func(backend string) {
			if backend != *probeBackend {
				return
			}
			select {
			case trigger <- struct{}{}:
			default:
			}
		}
	}

	if *telemetryPoll {
		if *probeBackend == "" {
			log.Printf("telemetrypoll: -probe-backend is empty; telemetry polling disabled")
		} else {
			tp := &telemetrypoll.Poller{
				Store:        telemetrypoll.NewStoreAdapter(st),
				Server:       srv,
				Backend:      *probeBackend,
				Interval:     *telemetryPollInterval,
				SendInterval: *probeInterval,
			}
			go tp.Run(ctx)
			log.Printf("telemetrypoll: started (interval=%s per-inverter-spacing=%s backend=%q)",
				*telemetryPollInterval, *probeInterval, *probeBackend)
		}
	}

	return srv.Serve(ctx)
}

// modelCodeFromStore looks up the model code for a given inverter UID from
// the state store. Returns (0, false) when the UID is not known or has no
// model code yet.
func modelCodeFromStore(ctx context.Context, st *store.Store, uid string) (uint8, bool) {
	var modelCode sql.NullInt64
	err := st.DB().QueryRowContext(ctx,
		`SELECT model_code FROM inverters WHERE uid = ?`, uid).Scan(&modelCode)
	if err != nil || !modelCode.Valid {
		return 0, false
	}
	return uint8(modelCode.Int64), true
}

// pruneOnce runs both retention tiers once. Telemetry events rotate
// aggressively (dominant volume); other kinds get the longer retention.
func pruneOnce(ctx context.Context, st *store.Store, retainTelemetry, retainOther time.Duration) {
	now := time.Now().UnixMilli()
	if retainTelemetry > 0 {
		cutoff := now - retainTelemetry.Milliseconds()
		n, err := st.PruneEvents(ctx, cutoff, []string{"telemetry"})
		if err != nil {
			log.Printf("prune: telemetry: %v", err)
		} else if n > 0 {
			log.Printf("prune: removed %d telemetry events older than %s", n, retainTelemetry)
		}
	}
	if retainOther > 0 {
		cutoff := now - retainOther.Milliseconds()
		// Empty kinds = every kind eligible; rows already deleted by
		// the telemetry call above just won't match.
		n, err := st.PruneEvents(ctx, cutoff, nil)
		if err != nil {
			log.Printf("prune: other: %v", err)
		} else if n > 0 {
			log.Printf("prune: removed %d events older than %s", n, retainOther)
		}
	}
}

func pruneLoop(ctx context.Context, st *store.Store, retainTelemetry, retainOther, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pruneOnce(ctx, st, retainTelemetry, retainOther)
		}
	}
}

// parseUIDList parses a comma-separated list of decimal UIDs.
// An empty / blank input returns nil (disables the gate).
func parseUIDList(csv string) ([]int, error) {
	parts := splitTrim(csv)
	if len(parts) == 0 {
		return nil, nil
	}
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("bad uid %q: %w", p, err)
		}
		if v < 0 {
			return nil, fmt.Errorf("uid %d must be non-negative", v)
		}
		out = append(out, v)
	}
	return out, nil
}

// splitTrim splits csv on ',', trims surrounding whitespace from each
// element, and drops empty entries. An all-blank input returns nil.
func splitTrim(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseOctalMode(s string) (os.FileMode, error) {
	v, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(v), nil
}

// telemetryRow is one inverter's latest sample assembled from joined
// rows. Tagged so json.Marshal emits the operator-friendly shape.
type telemetryRow struct {
	InverterUID     string          `json:"inverter_uid"`
	TsMs            int64           `json:"ts_ms"`
	Cmd             int64           `json:"cmd"`
	Model           any             `json:"model"`
	Family          any             `json:"family"`
	ShortAddr       any             `json:"short_addr"`
	ModelCode       any             `json:"model_code"`
	SoftwareVersion any             `json:"software_version"`
	Phase           any             `json:"phase"`
	ZigbeeBound     any             `json:"zigbee_bound"`
	TurnedOffRpt    any             `json:"turned_off_rpt"`
	ACW             any             `json:"ac_w"`
	ACV             any             `json:"ac_v"`
	ACFreq          any             `json:"ac_freq"`
	BusV            any             `json:"bus_v"`
	ReportSec       any             `json:"report_sec"`
	Panels          []telemetryPan  `json:"panels"`
	Lifetime        []telemetryEner `json:"lifetime"`
	LastIntervalMs  any             `json:"last_interval_ms"` // gap to previous frame for this UID
	AvgIntervalMs   any             `json:"avg_interval_ms"`  // EWMA over recent frames
	IntervalSamples any             `json:"interval_samples"` // EWMA sample count
}

type telemetryPan struct {
	ChannelIdx int `json:"i"`
	DCV        any `json:"dc_v"`
	DCI        any `json:"dc_i"`
	W          any `json:"w"`
}

type telemetryEner struct {
	ChannelIdx   int     `json:"i"`
	Raw          int64   `json:"prev_raw"`
	Scale        float64 `json:"scale"`
	CumulativeWh float64 `json:"cumulative_wh"`
}

func runDumpTelemetry(args []string) error {
	fs := flag.NewFlagSet("dump-telemetry", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDB, "SQLite state-store path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	st, err := store.Open(ctx, *dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	const liveQ = `
SELECT t.inverter_uid, t.ts_ms, t.cmd, t.ac_w, t.ac_v, t.ac_freq, t.bus_v, t.report_sec,
       i.short_addr, i.family, i.model,
       i.model_code, i.software_version, i.phase, i.zigbee_bound, i.turned_off_rpt
FROM   telemetry_live t
LEFT   JOIN inverters i ON i.uid = t.inverter_uid
ORDER  BY t.inverter_uid ASC
`
	rows, err := st.DB().QueryContext(ctx, liveQ)
	if err != nil {
		return err
	}
	defer rows.Close()

	enc := json.NewEncoder(os.Stdout)
	for rows.Next() {
		var (
			uid       string
			tsMs      int64
			cmd       int64
			acW       sqlNullFloat
			acV       sqlNullFloat
			acFreq    sqlNullFloat
			busV      sqlNullFloat
			reportSec sqlNullInt
			shortAddr sqlNullInt
			family    sqlNullString
			model     sqlNullString
			modelCode sqlNullInt
			swVersion sqlNullInt
			phase     sqlNullInt
			zbBound   sqlNullInt
			rptOff    sqlNullInt
		)
		if err := rows.Scan(&uid, &tsMs, &cmd, &acW, &acV, &acFreq, &busV, &reportSec,
			&shortAddr, &family, &model,
			&modelCode, &swVersion, &phase, &zbBound, &rptOff); err != nil {
			return err
		}
		panels, err := loadPanels(ctx, st.DB(), uid)
		if err != nil {
			return err
		}
		lifetime, err := loadLifetime(ctx, st.DB(), uid)
		if err != nil {
			return err
		}
		var lastIntervalMs sqlNullInt
		var avgIntervalMs sql.NullFloat64
		var intervalSamples sqlNullInt
		_ = st.DB().QueryRowContext(ctx,
			`SELECT last_interval_ms, avg_interval_ms, sample_count FROM inverter_metrics WHERE inverter_uid=?`,
			uid).Scan(&lastIntervalMs, &avgIntervalMs, &intervalSamples)
		var avgAny any
		if avgIntervalMs.Valid {
			avgAny = avgIntervalMs.Float64
		}
		row := telemetryRow{
			InverterUID:     uid,
			TsMs:            tsMs,
			Cmd:             cmd,
			Model:           model.toAny(),
			Family:          family.toAny(),
			ShortAddr:       shortAddr.toAny(),
			ModelCode:       modelCode.toAny(),
			SoftwareVersion: swVersion.toAny(),
			Phase:           phase.toAny(),
			ZigbeeBound:     boolFromNullInt(zbBound),
			TurnedOffRpt:    boolFromNullInt(rptOff),
			ACW:             acW.toAny(),
			ACV:             acV.toAny(),
			ACFreq:          acFreq.toAny(),
			BusV:            busV.toAny(),
			ReportSec:       reportSec.toAny(),
			Panels:          panels,
			Lifetime:        lifetime,
			LastIntervalMs:  lastIntervalMs.toAny(),
			AvgIntervalMs:   avgAny,
			IntervalSamples: intervalSamples.toAny(),
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return rows.Err()
}

func loadPanels(ctx context.Context, db *sql.DB, uid string) ([]telemetryPan, error) {
	const q = `
SELECT channel_idx, dc_v, dc_i, w
FROM   inverter_panels
WHERE  inverter_uid = ?
ORDER  BY channel_idx ASC
`
	rows, err := db.QueryContext(ctx, q, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []telemetryPan
	for rows.Next() {
		var (
			idx int
			dcV sqlNullFloat
			dcI sqlNullFloat
			w   sqlNullFloat
		)
		if err := rows.Scan(&idx, &dcV, &dcI, &w); err != nil {
			return nil, err
		}
		out = append(out, telemetryPan{
			ChannelIdx: idx,
			DCV:        dcV.toAny(),
			DCI:        dcI.toAny(),
			W:          w.toAny(),
		})
	}
	return out, rows.Err()
}

func loadLifetime(ctx context.Context, db *sql.DB, uid string) ([]telemetryEner, error) {
	const q = `
SELECT channel_idx, prev_raw, cumulative_wh, scale
FROM   energy_lifetime
WHERE  inverter_uid = ?
ORDER  BY channel_idx ASC
`
	rows, err := db.QueryContext(ctx, q, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []telemetryEner
	for rows.Next() {
		var (
			idx          int
			prevRaw      int64
			cumulativeWh float64
			scale        float64
		)
		if err := rows.Scan(&idx, &prevRaw, &cumulativeWh, &scale); err != nil {
			return nil, err
		}
		out = append(out, telemetryEner{
			ChannelIdx:   idx,
			Raw:          prevRaw,
			Scale:        scale,
			CumulativeWh: cumulativeWh,
		})
	}
	return out, rows.Err()
}

func runDumpEvents(args []string) error {
	fs := flag.NewFlagSet("dump-events", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDB, "SQLite state-store path")
	sinceMs := fs.Int64("since-ms", 0, "only rows with ts_ms >= this value")
	kind := fs.String("kind", "", "filter by kind (empty = any)")
	limit := fs.Int("limit", 100, "max rows to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *limit <= 0 {
		return errors.New("-limit must be > 0")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, *dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	evs, err := st.QueryEvents(ctx, store.EventFilter{
		SinceMs: *sinceMs,
		Kind:    *kind,
		Limit:   *limit,
	})
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	for _, e := range evs {
		row := map[string]any{
			"id":           e.ID,
			"ts_ms":        e.TsMs,
			"inverter_uid": orNil(e.InverterUID),
			"kind":         e.Kind,
			"severity":     e.Severity,
			"short_addr":   orNilU32(e.ShortAddr),
			"error":        orNil(e.Detail),
			"raw_hex":      orNil(e.RawHex),
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

// orNil maps an empty string to nil so JSON output shows null for unset
// optional columns (matching the previous sql.Null* behaviour).
func orNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func orNilU32(v uint32) any {
	if v == 0 {
		return nil
	}
	return v
}

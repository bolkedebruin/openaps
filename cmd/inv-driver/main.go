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

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/internal/buslock"
	"github.com/bolkedebruin/openaps/internal/eventlog"
	"github.com/bolkedebruin/openaps/internal/events"
	"github.com/bolkedebruin/openaps/internal/gridprofile"
	"github.com/bolkedebruin/openaps/internal/ingest"
	"github.com/bolkedebruin/openaps/internal/ipc"
	"github.com/bolkedebruin/openaps/internal/pairing"
	"github.com/bolkedebruin/openaps/internal/probe"
	"github.com/bolkedebruin/openaps/internal/settings"
	"github.com/bolkedebruin/openaps/internal/store"
	"github.com/bolkedebruin/openaps/internal/systemstatus"
	"github.com/bolkedebruin/openaps/internal/telemetrypoll"
	"github.com/bolkedebruin/openaps/wire"
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
	case "import-stock":
		err = runImportStock(args)
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
  import-stock     Seed the inverter inventory from the stock APsystems DB.

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
	enableAESL1 := fs.Bool("enable-aes-l1", false,
		"EXPERIMENTAL: enable L1 OTA AES-128 decrypt of inbound encrypted frames (per-frame key derivation per docs/AES-DESIGN.md; no on-wire test vectors)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *enableAESL1 {
		codec.SetAESEnabled(true)
		log.Printf("L1 AES enabled (EXPERIMENTAL: no on-wire test vectors)")
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

	// Per-inverter online/offline (offline debounced) + fleet sunrise/sundown
	// audit events. Runs until ctx is cancelled.
	go ingestor.StartOnlineSweep(ctx)

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
	// Let a fleet-wide base change reach inverters that have no overlay row.
	gpReconciler.KnownUIDs = func(ctx context.Context) []string {
		refs, err := st.ListInvertersForPoll(ctx)
		if err != nil {
			return nil
		}
		out := make([]string, 0, len(refs))
		for _, ref := range refs {
			out = append(out, ref.UID)
		}
		return out
	}

	// gpManagerPtr is captured by the OnFirstSeen hook so it can also
	// reconcile any persisted overlay for the uid AFTER the protection
	// cache is warm (VerifyStartup's read settle gives us that). Set
	// below right after gpManager is constructed; the hook nil-checks it.
	var gpManagerPtr *gridprofile.Manager

	// Wire the OnFirstSeen hook: on first telemetry from an inverter, run
	// VerifyStartup non-blocking after the protection read settles, and ALSO
	// reconcile any persisted overlay for that uid (it would otherwise wait
	// for an IPC SetOverlay to converge after a daemon restart). Per the
	// ingest layer, OnFirstSeen re-arms on radio rejoin (protLastSeen gap),
	// so a dark-then-back inverter re-triggers both paths automatically.
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
			// Now that the protection cache is warm for this uid, enqueue an
			// overlay reconcile if one is persisted. This replaces the
			// boot-time ReconcileAllOverlays sweep — running it before the
			// startup protection read produced spurious "no_readback" failures
			// for every overlay point on every restart.
			if mp := gpManagerPtr; mp != nil {
				if err := mp.ReconcileOverlaysForUID(ctx, uid); err != nil {
					log.Printf("gridprofile ReconcileOverlaysForUID uid=%s: %v", uid, err)
				}
			}
		}()
	}
	// Single process-wide bus guard: pairing ops and grid-profile broadcast
	// are mutually exclusive (both disrupt the fleet bus).
	busLock := buslock.New()

	gpBroadcaster := &gridprofile.IPCBroadcaster{Router: srv, Backend: *probeBackend}
	gpManager := &gridprofile.Manager{
		Store:       gpStore,
		Reconciler:  gpReconciler,
		ProfilesDir: profilesDir,
		OverlaysDir: overlaysDir,
		BusLock:     busLock,
		// Broadcast path defaults off; enable with -gridprofile-broadcast.
		// Targets every model code with a broadcast protection encoder
		// (codec.BroadcastModelCodes is the authority).
		BroadcastEnabled:    *gridProfileBroadcast,
		BroadcastSender:     gpBroadcaster,
		BroadcastModelCodes: codec.BroadcastModelCodes(),
		// Async overlay applier: events route to the audit log under
		// by="inv-driver"; the apply parent context is the daemon lifecycle
		// so applies survive the IPC request but stop at shutdown.
		Events:      storeEventSink{S: st},
		ApplyParent: ctx,
	}
	srv.GridProfile = gpManager
	gpManagerPtr = gpManager

	// NOTE: We intentionally do NOT call gpManager.ReconcileAllOverlays here
	// at boot. The protection cache is empty until each inverter's first
	// startup-protection-read completes, so a fleet-wide sweep at this point
	// would treat every overlay param as no_readback and emit spurious
	// overlay_param_failed events. Instead, per-uid overlay reconcile is
	// hooked into ingest.OnFirstSeen above (after the read-settle wait), so
	// each uid converges right after its protection cache warms — and the
	// hook re-arms on radio rejoin too.

	// inv-driver's settings file (ecu-id, mac, pan, zigbee-type).
	settingsStore, err := settings.Open(*settingsPath)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}

	// Boot-time MAC apply: macapp may have overwritten eth0 with the
	// firmware-provisioned MAC. If the operator persisted an override,
	// reassert it now (before the UDS is published, so consumers see
	// the corrected value when they dial in).
	if mac := settingsStore.Get().MAC; mac != "" {
		live := settings.ReadEth0MAC()
		if !strings.EqualFold(strings.TrimSpace(live), mac) {
			log.Printf("settings: applying mac override %s (live=%s)", mac, live)
			bootCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			applyErr := settings.ApplyMAC(bootCtx, mac)
			cancel()
			if applyErr != nil {
				log.Printf("settings: mac apply at boot failed: %v (continuing with live MAC)", applyErr)
			}
			// Record the outcome to the events store so an operator can
			// reconstruct the boot-time history later, even if they only
			// reach the ECU via the recovery path.
			kind, sev, detail := settings.ApplyEventFields(applyErr == nil, live, mac, applyErr)
			if err := st.AppendEvent(ctx, time.Now().UnixMilli(), "", kind, sev, "inv-driver", "boot: "+detail); err != nil {
				log.Printf("settings: append boot apply event: %v", err)
			}
		}
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

	// Settings read/write API over inv-driver's settings file. A SET that
	// applies a new MAC also emits a mac_apply_ok / mac_apply_failed audit
	// row attributed to the controller that drove the write (e.g.
	// "ecu-web"). The "by" attribution is filled in by the IPC layer; the
	// handler-level default here is "ecu-web" because all set-rpc traffic
	// today comes from the operator console.
	settingsHandler := settings.NewHandler(settingsStore)
	settingsHandler.AppendEvent = st.AppendEvent
	settingsHandler.AppendEventBy = "ecu-web"
	srv.Settings = settingsHandler

	// Pairing state machine: drives ecu-zb radio primitives via the
	// correlation transport, persists results, and shares busLock with the
	// grid-profile broadcast path. Controller-gated in the IPC layer.
	pairingTransport := pairing.NewTransport(srv, *probeBackend)
	ingestor.PairingResults = pairingTransport
	pairingManager := &pairing.Manager{
		Store:     st,
		Transport: pairingTransport,
		Lock:      busLock,
		Events:    st,
		Profiles:  gpManager,
		Names:     pairingSettings{store: settingsStore},
		Settings:  pairingSettings{store: settingsStore},
		// Resolve the operating RF channel from settings, falling back to
		// the built-in default when unset. A rekey request may override via
		// FleetRekey.channel.
		CurrentChannel: func() uint32 {
			if ch := settingsStore.Get().Channel; ch != 0 {
				return ch
			}
			return settings.DefaultChannel
		},
		Parent: ctx,
	}
	srv.Pairing = pairingManager

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
				// Share the single process-wide bus guard so a pairing
				// op or gridprofile broadcast in flight pauses polling
				// for its duration — a 0xBB query interleaved with a
				// pairing 0x05/0x0E/0x22 on the modem fd corrupts the
				// modem's view of the next config-op ack.
				BusLock: busLock,
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

// storeEventSink adapts *store.Store to gridprofile.EventSink so the async
// overlay applier can emit audit rows under by="inv-driver". Errors are
// logged but never propagated — event emission must not block or fail a
// background apply.
type storeEventSink struct {
	S *store.Store
}

func (s storeEventSink) AppendEvent(ctx context.Context, tsMs int64, uid, kind, severity, detail string) {
	if s.S == nil {
		return
	}
	if err := s.S.AppendEvent(ctx, tsMs, uid, kind, severity, "inv-driver", detail); err != nil {
		log.Printf("gridprofile: append %s event uid=%s: %v", kind, uid, err)
	}
}

// pairingSettings adapts *settings.Store to the pairing.Settings interface so
// FleetRekey can read/persist pan_override (never ecu_eth0_mac.conf — the
// macapp race that bricked the fleet).
type pairingSettings struct {
	store *settings.Store
}

func (p pairingSettings) PANOverride() string {
	return p.store.Get().PANOverride
}

func (p pairingSettings) SetPANOverride(_ context.Context, panHex string) error {
	s := p.store.Get()
	s.PANOverride = panHex
	return p.store.Save(s)
}

// SetChannel persists the operating RF channel after a successful channel
// migration. inv-driver does not validate the range — ecu-zb is the radio
// authority — so the value is stored verbatim.
func (p pairingSettings) SetChannel(_ context.Context, channel uint32) error {
	s := p.store.Get()
	s.Channel = channel
	return p.store.Save(s)
}

// InheritName moves the operator label from oldUID to newUID in the settings
// inverter_names map (the ECU's stand-in for an array slot). Best-effort: if
// the dead unit had no label there is nothing to move.
func (p pairingSettings) InheritName(_ context.Context, oldUID, newUID string) (bool, error) {
	s := p.store.Get()
	name, ok := s.InverterNames[oldUID]
	if !ok || name == "" {
		return false, nil
	}
	// Get() shares the underlying map; copy it before mutating so we never
	// touch live state in place (avoids a race and a partial update on a
	// failed Save).
	names := make(map[string]string, len(s.InverterNames))
	for k, v := range s.InverterNames {
		names[k] = v
	}
	delete(names, oldUID)
	names[newUID] = name
	s.InverterNames = names
	return true, p.store.Save(s)
}

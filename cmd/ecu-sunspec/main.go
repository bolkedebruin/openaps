// Command ecu-sunspec exposes an APsystems ECU as a SunSpec inverter over
// Modbus TCP. Per-inverter telemetry and grid-protection state come from
// inv-driver over a UDS socket.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bolkedebruin/openaps/internal/sunspec/config"
	"github.com/bolkedebruin/openaps/internal/sunspec/server"
	"github.com/bolkedebruin/openaps/internal/sunspec/source"
	"github.com/bolkedebruin/openaps/internal/sunspec/source/invdriver"
	"github.com/bolkedebruin/openaps/internal/sunspec/sunspec"
	"gopkg.in/natefinch/lumberjack.v2"
)

// version is overridable at build time via -ldflags '-X main.version=...'.
var version = "dev"

func main() {
	// Intercept operator subcommands before flag parsing so they can
	// define their own flag sets. "gridprofile" must be the first
	// non-flag argument (i.e. before any -flag).
	if len(os.Args) > 1 && os.Args[1] == "gridprofile" {
		runGridprofile(os.Args[2:])
		return
	}

	var (
		bind           = flag.String("bind", "tcp://0.0.0.0:502", "modbus TCP listen URL")
		yunengDir      = flag.String("yuneng-dir", "/etc/yuneng", "directory containing ecuid.conf, version.conf, model.conf (\"\" to skip)")
		manufacturer   = flag.String("manufacturer", "", "SunSpec Mn field; empty = "+sunspec.DefaultManufacturer)
		modelName      = flag.String("model-name", "", "SunSpec Md field; empty = read /etc/yuneng/model.conf, then fall back to "+sunspec.DefaultModelName)
		phase          = flag.String("phase-mode", "auto", "SunSpec inverter model: auto|single|split|three (101|102|103)")
		serialOverride = flag.String("serial-override", "", "force this SN regardless of ecuid.conf (use to re-spawn under a new device id)")
		serialFallback = flag.String("serial-fallback", "", "SN to use when ecuid.conf is unavailable")
		refresh        = flag.Duration("refresh-interval", 5*time.Second, "snapshot refresh cadence")

		logFile       = flag.String("log-file", "", "rotated log file path; empty means stderr")
		logMaxSizeMB  = flag.Int("log-max-size", 5, "max log size MB before rotation")
		logMaxBackups = flag.Int("log-max-backups", 3, "rotated log files retained")
		logMaxAgeDays = flag.Int("log-max-age", 7, "rotated log retention days")

		configPath    = flag.String("config", config.DefaultPath, "path to JSON config file (writes.enabled / writes.allow_list); missing file = writes enabled for loopback + local LAN")
		nameplatePath = flag.String("nameplate-file", "/home/sunspec-nameplate.json", "path to JSON model_int→watts table; missing file = TypeCode-based fallback")

		invDriverSock       = flag.String("invdriver-sock", defaultInvDriverSock(), "UDS path to inv-driver, the sole telemetry source (env INV_DRIVER_SOCK overrides default; empty is a fatal misconfiguration)")
		invDriverOnlineMaxA = flag.Duration("invdriver-online-max-age", 10*time.Second, "max age of an inv-driver telemetry frame before the inverter is marked offline")
	)
	flag.Parse()

	if n, err := source.LoadNameplateTable(*nameplatePath); err != nil {
		fmt.Fprintf(os.Stderr, "ecu-sunspec: nameplate-file %s: %v\n", *nameplatePath, err)
	} else if n > 0 {
		fmt.Fprintf(os.Stderr, "ecu-sunspec: loaded %d nameplate entries from %s\n", n, *nameplatePath)
	}

	var logSink io.Writer = os.Stderr
	if *logFile != "" {
		logSink = &lumberjack.Logger{
			Filename:   *logFile,
			MaxSize:    *logMaxSizeMB,
			MaxBackups: *logMaxBackups,
			MaxAge:     *logMaxAgeDays,
			Compress:   true,
		}
	}
	logger := log.New(logSink, "ecu-sunspec ", log.LstdFlags|log.Lmsgprefix)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatalf("load config %s: %v", *configPath, err)
	}
	if cfg.Writes.IsEnabled() {
		logger.Printf("writes enabled; allow_list=%v", cfg.Writes.AllowList)
	} else {
		logger.Printf("writes explicitly disabled")
	}

	// invClient is the inv-driver subscriber AND the sole write owner:
	// when non-nil it carries every inverter write (Model 123 set-power /
	// on-off and the grid-protection params) over Send → ecu-zb → radio.
	// Telemetry, energy, and grid-protection all come from inv-driver.
	var invClient *invdriver.Client

	builder := source.NewBuilder(*yunengDir)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if *invDriverSock == "" {
		logger.Fatalf("--invdriver-sock is empty: inv-driver is the only telemetry source, nothing to serve")
	}

	// Sub-second staleness floors are almost never what the operator
	// intends — clamp to 1s so a typo can't mark every fresh frame
	// offline. Zero / negative is already handled in the builder
	// (falls back to the 10s default).
	if *invDriverOnlineMaxA > 0 && *invDriverOnlineMaxA < time.Second {
		logger.Printf("invdriver-online-max-age=%s below 1s; clamping to 1s",
			*invDriverOnlineMaxA)
		*invDriverOnlineMaxA = time.Second
	}
	ic := invdriver.New(*invDriverSock, version)
	ic.Logger = logger
	if err := ic.Start(ctx); err != nil {
		logger.Fatalf("invdriver start (%s): %v", *invDriverSock, err)
	}
	defer ic.Stop()
	invClient = ic
	builder.InvDriverClient = ic
	builder.InvDriverOnlineMaxAge = *invDriverOnlineMaxA
	logger.Printf("invdriver subscriber: socket=%s online-max-age=%s", *invDriverSock, *invDriverOnlineMaxA)

	phaseMode, err := parsePhase(*phase)
	if err != nil {
		logger.Fatalf("phase-mode: %v", err)
	}

	// Shared between the snapshot builder (WMaxLimPct read-back) and the
	// write path (records each set-power).
	limitCache := source.NewPowerLimitCache()
	builder.LimitCache = limitCache

	scfg := server.Config{
		URL:             *bind,
		RefreshInterval: *refresh,
		Encoder: sunspec.Options{
			Manufacturer:   *manufacturer,
			ModelName:      *modelName,
			SerialOverride: *serialOverride,
			SerialFallback: *serialFallback,
			Phase:          phaseMode,
		},
		Writes:     cfg,
		LimitCache: limitCache,
		Logger:     logger,
	}
	// Assign only when present so Config.InvDriver stays an untyped nil
	// interface (not a typed-nil *invdriver.Client) when disabled.
	if invClient != nil {
		scfg.InvDriver = invClient
	}
	srv := server.New(builder, scfg)

	if err := srv.Start(ctx); err != nil {
		logger.Fatalf("server start: %v", err)
	}
	<-ctx.Done()
	logger.Println("shutting down")
	if err := srv.Stop(); err != nil {
		logger.Printf("stop: %v", err)
	}
}

// defaultInvDriverSock returns the per-environment default for the
// inv-driver UDS path. INV_DRIVER_SOCK overrides the ECU default.
func defaultInvDriverSock() string {
	if s := os.Getenv("INV_DRIVER_SOCK"); s != "" {
		return s
	}
	return "/var/run/inv-driver.sock"
}

func parsePhase(s string) (sunspec.PhaseMode, error) {
	switch s {
	case "auto", "":
		return sunspec.PhaseAuto, nil
	case "single", "1", "101":
		return sunspec.PhaseSingle, nil
	case "split", "2", "102":
		return sunspec.PhaseSplit, nil
	case "three", "3", "103":
		return sunspec.PhaseThree, nil
	default:
		return sunspec.PhaseAuto, fmt.Errorf("unknown phase mode %q", s)
	}
}

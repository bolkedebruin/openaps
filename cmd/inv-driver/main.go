// Command inv-driver is the v0 daemon and CLI for ingesting backend
// telemetry (from ecu-zb / bus-mgr) over a Unix-domain socket into a
// SQLite state-store, plus two read-only dump subcommands for ops
// inspection. See INV_DRIVER_DESIGN.md for the long-form design.
package main

import (
	"context"
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
	"syscall"
	"time"

	"github.com/bolke/inv-driver/internal/ingest"
	"github.com/bolke/inv-driver/internal/ipc"
	"github.com/bolke/inv-driver/internal/store"
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

Run 'inv-driver <subcommand> -h' for subcommand-specific flags.
`)
}

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
	if err := fs.Parse(args); err != nil {
		return err
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

	pruneOnce(ctx, st, *retainTelemetry, *retainOther)
	if *pruneInterval > 0 {
		go pruneLoop(ctx, st, *retainTelemetry, *retainOther, *pruneInterval)
	}

	srv := &ipc.Server{
		SocketPath: *socket,
		SocketMode: mode,
		Ingestor:   &ingest.Ingestor{S: st},
	}
	return srv.Serve(ctx)
}

// pruneOnce runs both retention tiers once, logging summary lines.
// Telemetry events rotate aggressively (1 Hz × N inverters is the
// dominant volume); other kinds — backend_hello, decode_failed,
// future state-change events — get the longer retention.
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

func parseOctalMode(s string) (os.FileMode, error) {
	v, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(v), nil
}

// decodePayload parses a payload column as JSON. On parse failure it
// returns the raw string so the row isn't dropped from the dump.
// Empty string returns nil.
func decodePayload(s string) any {
	if s == "" {
		return nil
	}
	var p any
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return s
	}
	return p
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

	const q = `
SELECT inverter_uid, ts_ms, ac_w, ac_v, ac_freq, bus_v, report_sec, payload_json
FROM   telemetry_live
ORDER  BY inverter_uid ASC
`
	rows, err := st.DB().QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	enc := json.NewEncoder(os.Stdout)
	for rows.Next() {
		var (
			uid       string
			tsMs      int64
			acW       sqlNullFloat
			acV       sqlNullFloat
			acFreq    sqlNullFloat
			busV      sqlNullFloat
			reportSec sqlNullInt
			payload   string
		)
		if err := rows.Scan(&uid, &tsMs, &acW, &acV, &acFreq, &busV, &reportSec, &payload); err != nil {
			return err
		}
		row := map[string]any{
			"inverter_uid": uid,
			"ts_ms":        tsMs,
			"ac_w":         acW.toAny(),
			"ac_v":         acV.toAny(),
			"ac_freq":      acFreq.toAny(),
			"bus_v":        busV.toAny(),
			"report_sec":   reportSec.toAny(),
			"payload":      decodePayload(payload),
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return rows.Err()
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

	q := `
SELECT id, ts_ms, inverter_uid, kind, severity, payload_json
FROM   events
WHERE  ts_ms >= ?
`
	qargs := []any{*sinceMs}
	if *kind != "" {
		q += " AND kind = ?"
		qargs = append(qargs, *kind)
	}
	q += " ORDER BY ts_ms ASC LIMIT ?"
	qargs = append(qargs, *limit)

	rows, err := st.DB().QueryContext(ctx, q, qargs...)
	if err != nil {
		return err
	}
	defer rows.Close()

	enc := json.NewEncoder(os.Stdout)
	for rows.Next() {
		var (
			id       int64
			tsMs     int64
			uid      sqlNullString
			rowKind  string
			severity string
			payload  sqlNullString
		)
		if err := rows.Scan(&id, &tsMs, &uid, &rowKind, &severity, &payload); err != nil {
			return err
		}
		var p any
		if payload.Valid {
			p = decodePayload(payload.String)
		}
		row := map[string]any{
			"id":           id,
			"ts_ms":        tsMs,
			"inverter_uid": uid.toAny(),
			"kind":         rowKind,
			"severity":     severity,
			"payload":      p,
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return rows.Err()
}

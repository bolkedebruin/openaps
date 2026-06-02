// Grid-profile operator subcommand. Invoked when the first argument is
// "gridprofile". Talks to inv-driver via the IPC proxy methods on
// invdriver.Client and prints JSON results to stdout.
//
// Usage:
//
//	ecu-sunspec [--invdriver-sock <sock>] gridprofile <op> [args...]
//
// Operations (read-only):
//
//	list                       — list stored base profiles
//	refresh                    — reload profiles from server directory, list
//	status                     — show active base, stored profiles, reconciler state
//	effective <uid>            — compute effective profile for one inverter
//
// Mutating operations (require explicit subcommand):
//
//	select <id>                — set active base profile; reconcile all inverters
//	overlay <uid> <file.json>  — upsert per-inverter overlay from file; reconcile
//	clear-overlay <uid>        — remove per-inverter overlay; reconcile
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/bolkedebruin/openaps/internal/sunspec/source/invdriver"
)

// runGridprofile is the entry-point for the "gridprofile" subcommand.
// It parses its own flags/args from args (everything after "gridprofile"),
// dials inv-driver via a temporary Client, and prints the result as JSON.
func runGridprofile(args []string) {
	fs := flag.NewFlagSet("gridprofile", flag.ExitOnError)
	sock := fs.String("invdriver-sock", defaultInvDriverSock(), "UDS path to inv-driver")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ecu-sunspec gridprofile [--invdriver-sock <path>] <op> [args...]

Operations:
  list                       list stored base profiles
  refresh                    reload profiles from server directory
  status                     show active base and reconciler state
  effective <uid>            effective profile for one inverter UID

  select <id>                set active base profile (mutating)
  overlay <uid> <file.json>  upsert per-inverter overlay from file (mutating)
  clear-overlay <uid>        remove per-inverter overlay (mutating)

`)
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fs.Usage()
		os.Exit(2)
	}

	if *sock == "" {
		fmt.Fprintln(os.Stderr, "ecu-sunspec gridprofile: --invdriver-sock is empty")
		os.Exit(1)
	}

	c := invdriver.New(*sock, version)
	ctx := context.Background()
	op := rest[0]

	switch op {
	case "list":
		profiles, err := c.ListProfiles(ctx)
		mustOK("list", err)
		printJSON(profiles)

	case "refresh":
		profiles, err := c.RefreshProfiles(ctx)
		mustOK("refresh", err)
		printJSON(profiles)

	case "status":
		raw, err := c.GetStatus(ctx)
		mustOK("status", err)
		printRaw(raw)

	case "effective":
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "usage: gridprofile effective <uid>")
			os.Exit(2)
		}
		raw, err := c.GetEffective(ctx, rest[1])
		mustOK("effective", err)
		printRaw(raw)

	case "select":
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "usage: gridprofile select <id>")
			os.Exit(2)
		}
		mustOK("select", c.SelectBase(ctx, rest[1]))
		fmt.Println("ok")

	case "overlay":
		if len(rest) < 3 {
			fmt.Fprintln(os.Stderr, "usage: gridprofile overlay <uid> <file.json>")
			os.Exit(2)
		}
		uid := rest[1]
		data, err := os.ReadFile(rest[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "gridprofile overlay: read %s: %v\n", rest[2], err)
			os.Exit(1)
		}
		mustOK("overlay", c.SetOverlay(ctx, uid, data))
		fmt.Println("ok")

	case "clear-overlay":
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "usage: gridprofile clear-overlay <uid>")
			os.Exit(2)
		}
		mustOK("clear-overlay", c.ClearOverlay(ctx, rest[1]))
		fmt.Println("ok")

	default:
		fmt.Fprintf(os.Stderr, "gridprofile: unknown operation %q\n", op)
		fs.Usage()
		os.Exit(2)
	}
}

// mustOK prints op: err to stderr and exits 1 if err != nil.
func mustOK(op string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "gridprofile %s: %v\n", op, err)
		os.Exit(1)
	}
}

// printJSON marshals v with indentation and writes it to stdout.
func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}
}

// printRaw pretty-prints a json.RawMessage to stdout.
func printRaw(raw json.RawMessage) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		// Not valid JSON — print as-is.
		_, _ = os.Stdout.Write(raw)
		_ = os.Stdout.Sync()
		return
	}
	printJSON(v)
}

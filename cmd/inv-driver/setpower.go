package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
)

func runSetPower(args []string) error {
	fs := flag.NewFlagSet("set-power", flag.ContinueOnError)
	defSock := defaultSocket
	if s := os.Getenv("INV_DRIVER_SOCK"); s != "" {
		defSock = s
	}
	socket := fs.String("socket", defSock, "UDS path of the running daemon (env INV_DRIVER_SOCK overrides default)")
	dbPath := fs.String("db", defaultDB, "SQLite state-store path")
	uid := fs.String("uid", "", "inverter UID (12 hex chars); required unless --broadcast is set")
	broadcast := fs.Bool("broadcast", false, "broadcast to every inverter on the bus (SA=0)")
	watts := fs.Int("watts", -1, "max-power per panel in watts, 20..500")
	family := fs.String("family", "dsp", "family for --broadcast: 'dsp' (QS1 / QS1A), 'ds3' (DS3 family), or 'c3' (YC600 / YC1000)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage:")
		fmt.Fprintln(fs.Output(), "  inv-driver set-power --uid <12-hex> --watts <20-500>")
		fmt.Fprintln(fs.Output(), "  inv-driver set-power --broadcast --family dsp|c3|ds3 --watts <20-500>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *watts < 20 || *watts > 500 {
		return errors.New("--watts must be in [20, 500]")
	}
	if *broadcast == (*uid != "") {
		return errors.New("exactly one of --uid or --broadcast must be set")
	}
	if !*broadcast && !isValidUID(*uid) {
		return fmt.Errorf("--uid %q: expected 12 hex characters", *uid)
	}

	frame, peerUID, err := buildSetPowerFrame(*dbPath, *uid, *broadcast, uint16(*watts), *family)
	if err != nil {
		return err
	}

	fmt.Printf("frame: % X\n", frame)

	env := &wire.Envelope{}
	if *broadcast {
		env.Body = &wire.Envelope_Broadcast{Broadcast: &wire.Broadcast{Frame: frame}}
	} else {
		env.Body = &wire.Envelope_Send{Send: &wire.Send{PeerUid: peerUID, Frame: frame}}
	}

	if err := sendEnvelopeOverUDS(*socket, env); err != nil {
		fmt.Fprintf(os.Stderr, "status: not delivered (%v)\n", err)
		return fmt.Errorf("send envelope: %w", err)
	}
	fmt.Println("status: delivered to daemon")
	return nil
}

// isValidUID accepts the 12-hex UID form codec.ParseL1 emits. Case
// insensitive — operators commonly paste these uppercase.
func isValidUID(s string) bool {
	if len(s) != 12 {
		return false
	}
	for i := 0; i < 12; i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func buildSetPowerFrame(dbPath, uid string, broadcast bool, panelWatts uint16, family string) ([]byte, string, error) {
	if broadcast {
		var modelCode uint8
		switch strings.ToLower(family) {
		case "dsp":
			modelCode = 0x18
		case "ds3":
			modelCode = 0x20
		case "c3":
			modelCode = 0x06
		default:
			return nil, "", fmt.Errorf("unknown --family %q (want 'dsp', 'ds3', or 'c3')", family)
		}
		l2, err := codec.EncodeSetPower(modelCode, panelWatts, true)
		if err != nil {
			return nil, "", err
		}
		return codec.BuildL1Frame(0, l2), "", nil
	}

	sa, modelCode, err := lookupInverter(dbPath, uid)
	if err != nil {
		return nil, "", err
	}
	l2, err := codec.EncodeSetPower(modelCode, panelWatts, false)
	if err != nil {
		return nil, "", err
	}
	return codec.BuildL1Frame(sa, l2), uid, nil
}

func lookupInverter(dbPath, uid string) (sa uint16, modelCode uint8, err error) {
	ctx := context.Background()
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		return 0, 0, fmt.Errorf("open db: %w", err)
	}
	defer st.Close()

	var (
		saNI sql.NullInt64
		mcNI sql.NullInt64
	)
	row := st.DB().QueryRowContext(ctx,
		`SELECT short_addr, model_code FROM inverters WHERE uid = ?`, uid)
	if err := row.Scan(&saNI, &mcNI); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, fmt.Errorf("uid %q: not in inverters table", uid)
		}
		return 0, 0, fmt.Errorf("uid %q: %w", uid, err)
	}
	if !saNI.Valid {
		return 0, 0, fmt.Errorf("uid %q: short_addr is NULL (inverter never paired)", uid)
	}
	if !mcNI.Valid {
		return 0, 0, fmt.Errorf("uid %q: model_code is NULL (probe has not completed)", uid)
	}
	return uint16(saNI.Int64), uint8(mcNI.Int64), nil
}

// sendEnvelopeOverUDS dials the daemon, says Hello as a publisher with
// a sentinel backend name, writes one envelope, and closes. Any dial,
// hello, or write error is surfaced to the caller, who treats it as
// "not delivered" but not as a fatal CLI error.
func sendEnvelopeOverUDS(socketPath string, env *wire.Envelope) error {
	d := net.Dialer{Timeout: 1500 * time.Millisecond}
	conn, err := d.Dial("unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(1500 * time.Millisecond))
	hostname, _ := os.Hostname()
	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend:     "inv-driver-cli",
		Version:     "set-power",
		Hostname:    hostname,
		StartedAtMs: time.Now().UnixMilli(),
	}}}
	if err := wire.WriteFrame(conn, hello); err != nil {
		return fmt.Errorf("hello: %w", err)
	}
	if err := wire.WriteFrame(conn, env); err != nil {
		return fmt.Errorf("envelope: %w", err)
	}
	return nil
}


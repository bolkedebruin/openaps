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

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/internal/store"
	"github.com/bolkedebruin/openaps/wire"
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
	family := fs.String("family", "", "family for --broadcast (required when --broadcast is set); see -h for the list of accepted names")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage:")
		fmt.Fprintln(fs.Output(), "  inv-driver set-power --uid <12-hex> --watts <20-500>")
		fmt.Fprintf(fs.Output(), "  inv-driver set-power --broadcast --family %s --watts <20-500>\n", setPowerFamilyChoices())
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if uint16(*watts) < codec.MinPanelLimitW || uint16(*watts) > codec.MaxPanelLimitW || *watts < 0 {
		return fmt.Errorf("--watts must be in [%d, %d]", codec.MinPanelLimitW, codec.MaxPanelLimitW)
	}
	if *broadcast == (*uid != "") {
		return errors.New("exactly one of --uid or --broadcast must be set")
	}
	if *broadcast && *family == "" {
		return fmt.Errorf("--family is required with --broadcast (choices: %s)", setPowerFamilyChoices())
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
		modelCode, err := setPowerFamilyToModel(family)
		if err != nil {
			return nil, "", err
		}
		l2, err := codec.EncodeSetPower(modelCode, panelWatts, true)
		if err != nil {
			return nil, "", err
		}
		return l2, "", nil
	}

	modelCode, err := lookupInverter(dbPath, uid)
	if err != nil {
		return nil, "", err
	}
	l2, err := codec.EncodeSetPower(modelCode, panelWatts, false)
	if err != nil {
		return nil, "", err
	}
	return l2, uid, nil
}

// setPowerFamilies pairs a flag name with a representative model code
// that EncodeSetPower accepts. Listed in family-key order (codec.Family
// strings) plus the legacy "c3" name for the YC600 / YC1000 group, so
// callers and help output stay in sync with codec without enumerating
// model codes here.
var setPowerFamilies = []struct {
	name      string
	modelCode uint8
}{
	{"ds3", codec.ModelDS3},
	{"qs1", codec.ModelQS1A},
	{"qs1a", codec.ModelQS1A}, // alias for qs1
	{"dsp", codec.ModelQS1A},  // alias for qs1
	{"yc600", codec.ModelYC600},
	{"yc1000", codec.ModelYC1000},
	{"c3", codec.ModelYC1000}, // alias for the YC600 / YC1000 group
}

// setPowerFamilyToModel maps a -family flag value to a representative
// model code. Unknown names return an error listing the accepted set.
func setPowerFamilyToModel(name string) (uint8, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, f := range setPowerFamilies {
		if f.name == n {
			return f.modelCode, nil
		}
	}
	return 0, fmt.Errorf("unknown --family %q (choices: %s)", name, setPowerFamilyChoices())
}

// setPowerFamilyChoices renders the accepted -family values as a
// pipe-separated list for the usage string.
func setPowerFamilyChoices() string {
	names := make([]string, 0, len(setPowerFamilies))
	for _, f := range setPowerFamilies {
		names = append(names, f.name)
	}
	return strings.Join(names, "|")
}

// lookupInverter returns the model_code for uid. short_addr is checked
// for validity (rows that never paired are rejected) but the wrapping
// outer envelope is built by the bus-mgr backend, so the SA value is
// not needed here.
func lookupInverter(dbPath, uid string) (modelCode uint8, err error) {
	ctx := context.Background()
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		return 0, fmt.Errorf("open db: %w", err)
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
			return 0, fmt.Errorf("uid %q: not in inverters table", uid)
		}
		return 0, fmt.Errorf("uid %q: %w", uid, err)
	}
	if !saNI.Valid {
		return 0, fmt.Errorf("uid %q: short_addr is NULL (inverter never paired)", uid)
	}
	if !mcNI.Valid {
		return 0, fmt.Errorf("uid %q: model_code is NULL (probe has not completed)", uid)
	}
	return uint8(mcNI.Int64), nil
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

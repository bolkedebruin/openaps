// Command protbcast is a one-off test/ops tool for sending a single
// grid-protection parameter frame (broadcast or unicast) to live
// hardware via the running inv-driver daemon. It is NOT a production
// path — use it to validate broadcast protection frames on-wire before
// enabling them in higher-level flows.
//
// Usage examples:
//
//	# Broadcast Over_frequency_Watt_Low_set = 50.3 Hz to all QS1A inverters:
//	protbcast -model qs1a -param Over_frequency_Watt_Low_set -value 50.3
//
//	# Unicast the same param to one DS3 inverter:
//	protbcast -model ds3 -param Over_frequency_Watt_Low_set -value 50.3 \
//	          -broadcast=false -uid <uid>
//
//	# Use a non-default socket:
//	protbcast -socket /tmp/inv-driver.sock -model qs1a \
//	          -param Under_Frequency_Watt_Low_set -value 47.0
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/wire"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "protbcast: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("protbcast", flag.ContinueOnError)
	socket := fs.String("socket", "/var/run/inv-driver.sock",
		"UDS path of the running inv-driver daemon")
	modelStr := fs.String("model", "",
		"inverter family (required); see -h for the list of accepted names")
	param := fs.String("param", "",
		`firmware long param name, e.g. "Over_frequency_Watt_Low_set"`)
	value := fs.Float64("value", 0,
		"param value in native units (Hz for frequency thresholds)")
	broadcast := fs.Bool("broadcast", true,
		"broadcast to all inverters of the family (SA=0); set false for unicast")
	uid := fs.String("uid", "",
		"12-hex inverter UID; required when -broadcast=false")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: protbcast [flags]")
		fmt.Fprintln(fs.Output(), "")
		fmt.Fprintln(fs.Output(), "flags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *modelStr == "" {
		return fmt.Errorf("-model is required")
	}
	if *param == "" {
		return fmt.Errorf("-param is required")
	}
	if *value <= 0 {
		return fmt.Errorf("-value must be > 0")
	}

	modelCode, err := parseModel(*modelStr)
	if err != nil {
		return err
	}

	if !*broadcast && !isValidUID(*uid) {
		return fmt.Errorf("-uid %q: must be exactly 12 hex characters when -broadcast=false", *uid)
	}

	var frames [][]byte
	if *broadcast {
		frames, err = codec.EncodeSetProtectionBroadcast(modelCode, *param, *value)
	} else {
		frames, err = codec.EncodeSetProtection(modelCode, *param, *value)
	}
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	fmt.Printf("model:     0x%02X (%s)\n", modelCode, *modelStr)
	fmt.Printf("param:     %s\n", *param)
	fmt.Printf("value:     %g\n", *value)
	modeStr := "broadcast (SA=0)"
	if !*broadcast {
		modeStr = fmt.Sprintf("unicast uid=%s", *uid)
	}
	fmt.Printf("mode:      %s\n", modeStr)
	fmt.Printf("frames:    %d\n", len(frames))
	for i, f := range frames {
		fmt.Printf("  [%d] % X\n", i, f)
	}
	fmt.Println()

	conn, err := dialDaemon(*socket)
	if err != nil {
		return fmt.Errorf("dial %s: %w", *socket, err)
	}
	defer conn.Close()

	if err := sendHello(conn); err != nil {
		return err
	}

	for i, f := range frames {
		var env *wire.Envelope
		if *broadcast {
			env = &wire.Envelope{Body: &wire.Envelope_Broadcast{
				Broadcast: &wire.Broadcast{Frame: f},
			}}
		} else {
			env = &wire.Envelope{Body: &wire.Envelope_Send{
				Send: &wire.Send{PeerUid: strings.ToLower(*uid), Frame: f},
			}}
		}
		_ = conn.SetWriteDeadline(time.Now().Add(1500 * time.Millisecond))
		if err := wire.WriteFrame(conn, env); err != nil {
			return fmt.Errorf("write frame %d: %w", i, err)
		}
		fmt.Printf("frame %d delivered to daemon\n", i)
	}

	return nil
}

// parseModel maps a user-supplied family string to a representative
// codec model code. Names are matched against the broadcast-capable
// model codes that codec.BroadcastModelCodes returns — so adding a new
// broadcast encoder in codec automatically widens the CLI's accepted
// set without a parallel list to keep in sync. The lookup accepts both
// the family key (codec.Family.String, e.g. "qs1") and the exact model
// label codec.FamilyForModelString understands (e.g. "QS1A", "DS3-L").
func parseModel(s string) (uint8, error) {
	n := strings.ToLower(strings.TrimSpace(s))
	if n == "" {
		return 0, fmt.Errorf("-model %q: empty", s)
	}
	wantFam := codec.FamilyForModelString(n)
	for _, code := range codec.BroadcastModelCodes() {
		fam := codec.FamilyOf(code)
		if wantFam == fam && wantFam != codec.FamilyUnknown {
			return code, nil
		}
	}
	return 0, fmt.Errorf("-model %q: must be one of %s", s, protbcastFamilyChoices())
}

// protbcastFamilyChoices renders the accepted family names — one per
// broadcast-capable family in codec.BroadcastModelCodes — for error /
// help messages. The list is built from codec at call time so it stays
// in sync with the broadcast encoder coverage.
func protbcastFamilyChoices() string {
	seen := make(map[codec.Family]bool)
	var names []string
	for _, code := range codec.BroadcastModelCodes() {
		fam := codec.FamilyOf(code)
		if fam == codec.FamilyUnknown || seen[fam] {
			continue
		}
		seen[fam] = true
		names = append(names, fam.String())
	}
	return strings.Join(names, ", ")
}

// isValidUID accepts the 12-hex UID form that codec.ParseL1 emits.
// Case-insensitive.
func isValidUID(s string) bool {
	if len(s) != 12 {
		return false
	}
	for _, c := range s {
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

// dialDaemon opens a Unix-domain socket connection to the inv-driver daemon.
func dialDaemon(socketPath string) (net.Conn, error) {
	d := net.Dialer{Timeout: 1500 * time.Millisecond}
	return d.Dial("unix", socketPath)
}

// sendHello writes the Hello envelope that identifies this client as a
// controller backend (inv-driver-cli is the default controller-backends
// value in the daemon's -controller-backends flag).
func sendHello(conn net.Conn) error {
	_ = conn.SetWriteDeadline(time.Now().Add(1500 * time.Millisecond))
	hostname, _ := os.Hostname()
	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend:     "inv-driver-cli",
		Version:     "protbcast",
		Hostname:    hostname,
		StartedAtMs: time.Now().UnixMilli(),
	}}}
	if err := wire.WriteFrame(conn, hello); err != nil {
		return fmt.Errorf("hello: %w", err)
	}
	return nil
}

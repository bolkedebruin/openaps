// Command legacy-bridge is an inv-driver UDS subscriber that
// materialises decoded telemetry into the legacy main.exe-style
// /tmp/parameters_app.conf file so existing consumers (ecu-sunspec
// etc.) keep working alongside inv-driver.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bolke/inv-driver/cmd/legacy-bridge/internal/paramsfile"
	"github.com/bolke/inv-driver/wire"
)

// helloFrame builds the SUBSCRIBER Hello the bridge sends on every
// reconnect.
func helloFrame() *wire.Hello {
	hostname, _ := os.Hostname()
	return &wire.Hello{
		Backend:     "legacy-bridge",
		Version:     version,
		Hostname:    hostname,
		StartedAtMs: time.Now().UnixMilli(),
		Role:        wire.Role_SUBSCRIBER,
	}
}

const (
	defaultSocket     = "/var/run/inv-driver.sock"
	defaultParamsFile = "/tmp/parameters_app.conf"
)

// version is overridable at build time via -ldflags '-X main.version=...'.
var version = "dev"

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	fs := flag.NewFlagSet("legacy-bridge", flag.ContinueOnError)
	defSock := defaultSocket
	if s := os.Getenv("INV_DRIVER_SOCK"); s != "" {
		defSock = s
	}
	socket := fs.String("socket", defSock, "inv-driver UDS path (env INV_DRIVER_SOCK overrides default)")
	paramsPath := fs.String("params-file", defaultParamsFile, "destination file for legacy params snapshot")
	showVer := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *showVer {
		fmt.Println(version)
		return
	}
	if err := validateParamsPath(*paramsPath); err != nil {
		log.Printf("legacy-bridge: %v", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	u := paramsfile.New(*paramsPath)
	log.Printf("legacy-bridge: version=%s socket=%s params-file=%s", version, *socket, *paramsPath)

	if err := runLoop(ctx, *socket, u); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("legacy-bridge: exit: %v", err)
		os.Exit(1)
	}
}

// validateParamsPath enforces that the destination is under /tmp/.
// Operators wanting a different path need to argue for the change;
// /tmp/ is the only directory main.exe writes into today.
func validateParamsPath(p string) error {
	if !strings.HasPrefix(p, "/tmp/") {
		return errors.New("--params-file must be under /tmp/")
	}
	return nil
}

// runLoop attaches as a SUBSCRIBER and feeds Telemetry envelopes into
// the paramsfile updater, reconnecting with backoff on failure.
func runLoop(ctx context.Context, socket string, u *paramsfile.Updater) error {
	onTelemetry := func(t *wire.Telemetry) {
		if t == nil {
			return
		}
		if err := u.Update(t); err != nil {
			log.Printf("legacy-bridge: update %s: %v", t.GetPeerUid(), err)
		}
	}
	return wire.DialLoop(ctx, wire.DialLoopConfig{
		SocketPath: socket,
		OnConnect: func() {
			log.Printf("legacy-bridge: connected %s", socket)
		},
		OnDialError: func(err error, retryIn time.Duration) {
			log.Printf("legacy-bridge: dial/session %s: %v (retry in %s)", socket, err, retryIn)
		},
		Session: func(ctx context.Context, conn net.Conn) error {
			return wire.SubscriberSession(ctx, conn, helloFrame(), onTelemetry)
		},
	})
}

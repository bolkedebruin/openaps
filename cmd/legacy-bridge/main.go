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

	"github.com/bolke/inv-driver/internal/paramsfile"
	"github.com/bolke/inv-driver/wire"
)

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
	return wire.DialLoop(ctx, wire.DialLoopConfig{
		SocketPath: socket,
		OnConnect: func() {
			log.Printf("legacy-bridge: connected %s", socket)
		},
		OnDialError: func(err error, retryIn time.Duration) {
			log.Printf("legacy-bridge: dial/session %s: %v (retry in %s)", socket, err, retryIn)
		},
		Session: func(ctx context.Context, conn net.Conn) error {
			return session(ctx, conn, u)
		},
	})
}

// session sends Hello(SUBSCRIBER) and loops reading envelopes until
// the conn errors out or ctx is cancelled.
func session(ctx context.Context, conn net.Conn, u *paramsfile.Updater) error {
	hostname, _ := os.Hostname()
	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend:     "legacy-bridge",
		Version:     version,
		Hostname:    hostname,
		StartedAtMs: time.Now().UnixMilli(),
		Role:        wire.Role_SUBSCRIBER,
	}}}
	if err := wire.WriteFrame(conn, hello); err != nil {
		return fmt.Errorf("hello: %w", err)
	}

	// Closing the conn on ctx cancel unblocks the read loop with
	// net.ErrClosed.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	for {
		var env wire.Envelope
		if err := wire.ReadFrame(conn, &env); err != nil {
			return err
		}
		switch b := env.GetBody().(type) {
		case *wire.Envelope_Telemetry:
			if b.Telemetry == nil {
				continue
			}
			if err := u.Update(b.Telemetry); err != nil {
				log.Printf("legacy-bridge: update %s: %v", b.Telemetry.GetPeerUid(), err)
			}
		default:
			// Ignore envelopes we don't act on.
		}
	}
}

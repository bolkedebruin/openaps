// Command ecu-web serves the on-ECU operator console: an HTTP/2 web UI
// (Victron-VRM-style) backed by inv-driver over its Unix-domain socket.
// It attaches as a read-only SUBSCRIBER for live telemetry and as a
// controller for control operations.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bolkedebruin/openaps/cmd/ecu-web/internal/auth"
	"github.com/bolkedebruin/openaps/cmd/ecu-web/internal/httpserver"
	"github.com/bolkedebruin/openaps/cmd/ecu-web/internal/snapshot"
	"github.com/bolkedebruin/openaps/cmd/ecu-web/internal/uds"
	"github.com/bolkedebruin/openaps/cmd/ecu-web/web"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	listen := flag.String("listen", ":443", "HTTPS listen address")
	sock := flag.String("invdriver-sock", "/var/run/inv-driver.sock", "inv-driver UDS path")
	stateDir := flag.String("state-dir", "/var/lib/ecu-web", "dir for TLS cert/key and auth credentials")
	backend := flag.String("backend", "ecu-web", "Hello backend identity reported to inv-driver")
	pushRate := flag.Duration("push-rate", time.Second, "minimum interval between SSE fleet frames")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("ecu-web: ")
	log.Printf("starting version=%s listen=%s sock=%s state-dir=%s", version, *listen, *sock, *stateDir)

	hostname, _ := os.Hostname()

	authMgr, err := auth.NewManager(filepath.Join(*stateDir, "auth.json"))
	if err != nil {
		log.Fatalf("auth init: %v", err)
	}
	if !authMgr.Configured() {
		log.Printf("no operator password set — first browser visit must complete setup")
	}

	assets, err := web.Assets()
	if err != nil {
		log.Fatalf("embed assets: %v", err)
	}

	snap := snapshot.New(nil)
	sub := &uds.Subscriber{
		SocketPath: *sock,
		Backend:    *backend,
		Version:    version,
		Hostname:   hostname,
		Snap:       snap,
	}
	ctrl := &uds.Controller{
		SocketPath: *sock,
		Backend:    *backend,
		Version:    version,
		Hostname:   hostname,
	}

	srv := httpserver.New(httpserver.Config{
		Listen:        *listen,
		StateDir:      *stateDir,
		Assets:        assets,
		Snap:          snap,
		Auth:          authMgr,
		Conn:          sub.Connected,
		PushRate:      *pushRate,
		SysStatus:     ctrl.SystemStatus,
		EventsFn:      ctrl.Events,
		SettingsGet:   ctrl.GetSettings,
		SettingsSet:   ctrl.SetSettings,
		GridProfileFn: ctrl.GridProfile,
		SendFrame:     ctrl.Send,
		PairingFn:     ctrl.Pairing,
	})
	snap.SetOnChange(srv.MarkDirty)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := sub.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("uds subscriber exited: %v", err)
		}
	}()

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("http server: %v", err)
	}
	log.Printf("shutdown complete")
}

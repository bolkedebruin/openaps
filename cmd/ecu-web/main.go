// Command ecu-web serves the on-ECU operator console: an HTTP/2 web UI
// (Victron-VRM-style) backed by inv-driver over its Unix-domain socket.
// It attaches as a read-only SUBSCRIBER for live telemetry and as a
// controller for control operations.
package main

import (
	"context"
	"flag"
	"log/slog"
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
	"github.com/bolkedebruin/openaps/internal/logx"
)

// version and gitHash are overridden at build time via
// -ldflags "-X main.version=... -X main.gitHash=...".
var version = "dev"
var gitHash = ""

func main() {
	listen := flag.String("listen", ":443", "HTTPS listen address")
	sock := flag.String("invdriver-sock", "/var/run/inv-driver.sock", "inv-driver UDS path")
	recoverySock := flag.String("recoveryd-sock", "/var/run/recoveryd.sock", "recoveryd UDS path (SSH access plane)")
	stateDir := flag.String("state-dir", "/var/lib/ecu-web", "dir for TLS cert/key and auth credentials")
	backend := flag.String("backend", "ecu-web", "Hello backend identity reported to inv-driver")
	pushRate := flag.Duration("push-rate", time.Second, "minimum interval between SSE fleet frames")
	lc := logx.Bind(flag.CommandLine, "ecu-web", "/var/log/ecu-web.log")
	flag.Parse()
	lc.Init()

	slog.Info("starting", "version", version, "listen", *listen, "sock", *sock, "state_dir", *stateDir)

	hostname, _ := os.Hostname()

	authMgr, err := auth.NewManager(filepath.Join(*stateDir, "auth.json"))
	if err != nil {
		logx.Fatal("auth init", "err", err)
	}
	if !authMgr.Configured() {
		slog.Info("no operator password set — first browser visit must complete setup")
	}

	assets, err := web.Assets()
	if err != nil {
		logx.Fatal("embed assets", "err", err)
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
	rec := &uds.Recovery{SocketPath: *recoverySock}

	srv := httpserver.New(httpserver.Config{
		Listen:        *listen,
		Version:       version,
		Commit:        gitHash,
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
		SSHKeysList:   rec.ListKeys,
		SSHKeyAdd:     rec.AddKey,
		SSHKeyRemove:  rec.RemoveKey,
	})
	snap.SetOnChange(srv.MarkDirty)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := sub.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("uds subscriber exited", "err", err)
		}
	}()

	if err := srv.Run(ctx); err != nil {
		logx.Fatal("http server", "err", err)
	}
	slog.Info("shutdown complete")
}

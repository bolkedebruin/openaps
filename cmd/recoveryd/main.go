// Command recoveryd owns the box's SSH access plane: it is the single
// writer of authorized_keys. It loads the operator's authorized-key list
// from /etc/recoveryd/access.json, renders it into the active provider's
// authorized_keys at startup (the anti-brick boot guarantee), and serves
// a local protobuf UDS so the web console can add/remove keys. It is
// independent of inv-driver and ecu-web.
//
// Subcommands:
//
//	(default) / serve   run the daemon (boot-render + UDS server)
//	seed                ingest an authorized_keys file into access.json
//	                    (used by the brownfield installer; no daemon)
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bolkedebruin/openaps/internal/recoveryd"
)

// version is set via -ldflags at build time.
var version = "dev"

const (
	defaultAccess = "/etc/recoveryd/access.json"
	defaultSocket = "/var/run/recoveryd.sock"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("recoveryd: ")

	// First non-flag arg selects a subcommand; bare invocation serves.
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "seed":
			runSeed(args[1:])
			return
		case "serve":
			args = args[1:]
		}
	}
	runServe(args)
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	accessPath := fs.String("access", defaultAccess, "path to access.json (key list source of truth)")
	socketPath := fs.String("socket", defaultSocket, "local UDS path (root-only, mode 0600)")
	dropbearKey := fs.String("dropbear-host-key", recoveryd.DefaultDropbearHostKey, "dropbear RSA host key ensured under the openaps provider")
	showVersion := fs.Bool("version", false, "print version and exit")
	_ = fs.Parse(args)

	if *showVersion {
		log.Printf("version %s", version)
		return
	}

	store, err := recoveryd.Open(*accessPath)
	if err != nil {
		log.Fatalf("load access file: %v", err)
	}
	rend := &recoveryd.Renderer{DropbearKeyPath: *dropbearKey}
	mgr := recoveryd.NewManager(store, rend)

	// Boot-render BEFORE serving so authorized_keys + the dropbear host
	// key are in place even if the UDS never gets a client. A render
	// failure is fatal — it means the box may be unreachable.
	if err := mgr.BootRender(); err != nil {
		log.Fatalf("boot-render authorized_keys: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &recoveryd.Server{Manager: mgr, SocketPath: *socketPath}
	if err := srv.Serve(ctx); err != nil {
		log.Fatalf("serve: %v", err)
	}
	log.Printf("shutdown")
}

// runSeed ingests an authorized_keys file into access.json and exits. It
// does NOT render — the daemon renders at boot. Used by the installer to
// seed the operator's bundled key under recoveryd's ownership instead of
// writing /root/.ssh/authorized_keys directly.
func runSeed(args []string) {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	accessPath := fs.String("access", defaultAccess, "path to access.json to seed/merge into")
	from := fs.String("from", "", "authorized_keys-format file to ingest (required)")
	provider := fs.String("provider", string(recoveryd.ProviderOpenAPS), "provider: openaps|host|off")
	hostUser := fs.String("host-user", "", "host user for the host provider")
	_ = fs.Parse(args)

	if *from == "" {
		log.Fatalf("seed: -from is required")
	}
	store, err := recoveryd.Open(*accessPath)
	if err != nil {
		log.Fatalf("seed: load access file: %v", err)
	}
	n, err := recoveryd.Seed(store, recoveryd.Provider(*provider), *hostUser, *from)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("seed: added %d key(s) -> %s (provider=%s)", n, *accessPath, *provider)
}

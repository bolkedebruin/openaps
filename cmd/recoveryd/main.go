// Command recoveryd owns the box's SSH access plane: it is the single
// writer of authorized_keys, and authorized_keys is the source of truth.
// It reads and rewrites the real authorized_keys file directly (no separate
// key-list store) and serves a local protobuf UDS so the web console can
// list/add/remove keys. It is independent of inv-driver and ecu-web.
//
// At startup it ensures the .ssh dir exists and, when managing dropbear,
// the dropbear RSA host key — then serves. There is nothing to render:
// authorized_keys already is the truth.
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

const defaultSocket = "/var/run/recoveryd.sock"

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("recoveryd: ")

	fs := flag.NewFlagSet("recoveryd", flag.ExitOnError)
	authorizedKeys := fs.String("authorized-keys", "", "authorized_keys file recoveryd owns (source of truth); empty resolves the managed user's ~/.ssh/authorized_keys (root by default)")
	chownUser := fs.String("chown-user", "", "host user to own .ssh + authorized_keys (empty = root; set for the host provider)")
	manageDropbear := fs.Bool("manage-dropbear", true, "ensure the dropbear RSA host key (set false for a host sshd / Pi)")
	socketPath := fs.String("socket", defaultSocket, "local UDS path (root-only, mode 0600)")
	dropbearKey := fs.String("dropbear-host-key", recoveryd.DefaultDropbearHostKey, "dropbear RSA host key ensured when -manage-dropbear")
	showVersion := fs.Bool("version", false, "print version and exit")
	_ = fs.Parse(os.Args[1:])

	if *showVersion {
		log.Printf("version %s", version)
		return
	}

	prov := &recoveryd.Provider{
		AuthorizedKeysPath: *authorizedKeys,
		ChownUser:          *chownUser,
		ManageDropbear:     *manageDropbear,
		DropbearKeyPath:    *dropbearKey,
	}
	mgr := recoveryd.NewManager(prov)

	// Ensure the .ssh dir + (if managing dropbear) the host key BEFORE
	// serving so the recovery path is intact even if the UDS never gets a
	// client. A failure here is fatal — it means the box may be unreachable.
	if err := mgr.Boot(); err != nil {
		log.Fatalf("boot: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &recoveryd.Server{Manager: mgr, SocketPath: *socketPath}
	if err := srv.Serve(ctx); err != nil {
		log.Fatalf("serve: %v", err)
	}
	log.Printf("shutdown")
}

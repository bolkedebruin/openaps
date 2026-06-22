// Command openaps-tls-proxy is a loopback HTTP front-end for the ECU's ancient
// opkg. opkg fetches a GitHub-hosted package feed over plain HTTP from
// 127.0.0.1; this proxy upstreams to the feed over TLS 1.2+ with a vendored CA
// bundle, verifies the Packages index against the OpenAPS release key, and
// authenticates each .ipk against the SHA-256 the signed index pins for it.
//
// It binds loopback only: the proxy hands unverified-by-opkg bytes to a client
// that cannot itself check signatures, so it must never be reachable off-box.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/bolkedebruin/openaps/internal/logx"
	"github.com/bolkedebruin/openaps/internal/releasekey"
	"github.com/bolkedebruin/openaps/internal/tlsproxy"
)

// version is set via -ldflags at build time.
var version = "dev"

func main() {
	fs := flag.NewFlagSet("openaps-tls-proxy", flag.ExitOnError)
	lc := logx.Bind(fs, "openaps-tls-proxy", "/var/log/openaps-tls-proxy.log")
	listen := fs.String("listen", "127.0.0.1:8071", "loopback address opkg points at (must be a loopback address)")
	upstream := fs.String("upstream", "", "HTTPS feed base, e.g. https://USER.github.io/openaps-feed/stable (required)")
	mount := fs.String("mount", "/", "request path prefix opkg uses")
	index := fs.String("index", "Packages.gz", "signed index file opkg fetches")
	cacheDir := fs.String("cache-dir", "/home/openaps/opkg-cache", "staging dir for .ipk downloads (a real filesystem, never the /tmp tmpfs)")
	pubkey := fs.String("pubkey", "/etc/openaps/release.pub", "release public key the index is verified against")
	caFile := fs.String("ca-file", "", "extra PEM roots for a private/self-signed feed")
	idleTimeout := fs.Duration("idle-timeout", 0, "self-exit after this much inactivity (0 = run until signal; >0 = on-demand)")
	showVersion := fs.Bool("version", false, "print version and exit")
	_ = fs.Parse(os.Args[1:])
	logger := lc.Init()

	if *showVersion {
		logger.Info("version", "version", version)
		return
	}
	if *upstream == "" {
		logx.Fatal("-upstream is required")
	}

	// Enforce loopback-only. This proxy serves bytes opkg cannot itself verify;
	// exposing it off-box would let a network peer feed an unauthenticated
	// client, so the bind address must resolve to a loopback address.
	if err := requireLoopback(*listen); err != nil {
		logx.Fatal("listen", "err", err)
	}

	verifier, err := releasekey.LoadFile(*pubkey)
	if err != nil {
		logx.Fatal("release key", "err", err)
	}

	srv, err := tlsproxy.New(tlsproxy.Config{
		UpstreamBase: *upstream,
		MountPrefix:  *mount,
		IndexName:    *index,
		CacheDir:     *cacheDir,
		ExtraCAFile:  *caFile,
		Verifier:     verifier,
		Logger:       logger,
	})
	if err != nil {
		logx.Fatal("init", "err", err)
	}

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		logx.Fatal("listen", "addr", *listen, "err", err)
	}
	logger.Info("listening", "addr", ln.Addr(), "upstream", *upstream)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := srv.Serve(ctx, ln, *idleTimeout); err != nil {
		logx.Fatal("serve", "err", err)
	}
	logger.Info("shutdown")
}

// requireLoopback resolves the host part of addr and returns an error unless
// every resolved IP is a loopback address (127.0.0.0/8 or ::1).
func requireLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ips, err := net.DefaultResolver.LookupIP(context.Background(), "ip", host)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return fmt.Errorf("refusing non-loopback bind %s (this proxy must never be exposed off-box)", host)
	}
	for _, ip := range ips {
		if !ip.IsLoopback() {
			return fmt.Errorf("refusing non-loopback bind %s (this proxy must never be exposed off-box)", host)
		}
	}
	return nil
}

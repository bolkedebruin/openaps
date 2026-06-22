// Package tlsproxy is a loopback HTTP front-end that lets the ECU's ancient
// opkg (0.1.8, no modern TLS, no CA store) pull a package feed from GitHub.
// opkg talks plain HTTP to 127.0.0.1; the proxy upstreams to the feed over
// TLS 1.2+ with a vendored CA bundle, verifies the Packages index against the
// OpenAPS release key, and authenticates every .ipk against the SHA-256 the
// signed index pins for it. Bytes are never handed to opkg until verified, so
// opkg's missing signature support does not weaken the chain.
package tlsproxy

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	maxIndexBytes   = 32 << 20  // 32 MiB cap on a Packages index
	maxPackageBytes = 128 << 20 // 128 MiB cap on a single .ipk
)

// Verifier is the subset of releasekey.Verifier the proxy needs. It is an
// interface so tests can inject a verifier built from a throwaway key.
type Verifier interface {
	Verify(message, sig []byte) error
}

// Config configures a Server. UpstreamBase and Verifier are required.
type Config struct {
	// UpstreamBase is the HTTPS feed root, e.g.
	// https://user.github.io/openaps-feed/stable . Must be https.
	UpstreamBase string
	// MountPrefix is the request path prefix opkg uses; the remainder is
	// appended to UpstreamBase. Defaults to "/".
	MountPrefix string
	// IndexName is the signed index file opkg fetches. Defaults to
	// "Packages.gz". Its detached signature is IndexName + ".sig".
	IndexName string
	// CacheDir is where .ipk downloads are staged and hashed. It MUST be on
	// a real filesystem with room (e.g. /home), never the 16 MiB /tmp tmpfs.
	CacheDir string
	// ExtraCAFile optionally adds PEM roots for a private/self-signed feed.
	ExtraCAFile string
	// Verifier checks the index signature against the release key.
	Verifier Verifier
	// Client is the upstream HTTP client; nil builds a hardened default.
	Client *http.Client
	// Logger is used for request/error logging; nil discards.
	Logger *slog.Logger
}

// Server is the loopback opkg proxy.
type Server struct {
	upstream    *url.URL
	mountPrefix string
	indexName   string
	cacheDir    string
	verifier    Verifier
	client      *http.Client
	log         *slog.Logger

	mu       sync.Mutex
	manifest *manifest // cached after the index is fetched + verified

	activity chan struct{} // pinged on each handled request (for idle exit)
}

// New validates cfg and constructs a Server.
func New(cfg Config) (*Server, error) {
	if cfg.Verifier == nil {
		return nil, errors.New("tlsproxy: Verifier is required")
	}
	if cfg.CacheDir == "" {
		return nil, errors.New("tlsproxy: CacheDir is required")
	}
	u, err := url.Parse(strings.TrimRight(cfg.UpstreamBase, "/"))
	if err != nil {
		return nil, fmt.Errorf("tlsproxy: parse upstream %q: %w", cfg.UpstreamBase, err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("tlsproxy: upstream must be https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("tlsproxy: upstream %q has no host", cfg.UpstreamBase)
	}
	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("tlsproxy: cache dir: %w", err)
	}
	// A download killed mid-stream (the on-demand wrapper backgrounds the proxy
	// and it self-exits on idle; opkg or an operator may also kill it) leaves an
	// ipk-*.part file behind. Sweep them on startup so partials never accrete in
	// the persistent cache on a space-constrained box.
	if leftovers, _ := filepath.Glob(filepath.Join(cfg.CacheDir, "ipk-*.part")); len(leftovers) > 0 {
		for _, p := range leftovers {
			_ = os.Remove(p)
		}
	}

	client := cfg.Client
	if client == nil {
		pool, err := certPool(cfg.ExtraCAFile)
		if err != nil {
			return nil, err
		}
		upstreamHost := u.Hostname()
		client = &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool},
				ForceAttemptHTTP2:   true,
				MaxIdleConns:        4,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 30 * time.Second,
			},
			// Follow redirects only within the upstream host and only over
			// https, so a hijacked redirect cannot exfiltrate the request or
			// downgrade the transport.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("tlsproxy: too many redirects")
				}
				if req.URL.Scheme != "https" {
					return fmt.Errorf("tlsproxy: refusing non-https redirect to %s", req.URL)
				}
				if !strings.EqualFold(req.URL.Hostname(), upstreamHost) {
					return fmt.Errorf("tlsproxy: refusing cross-host redirect to %s", req.URL.Host)
				}
				return nil
			},
		}
	}

	mount := cfg.MountPrefix
	if mount == "" {
		mount = "/"
	}
	if !strings.HasPrefix(mount, "/") {
		mount = "/" + mount
	}
	idx := cfg.IndexName
	if idx == "" {
		idx = "Packages.gz"
	}
	lg := cfg.Logger
	if lg == nil {
		lg = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Server{
		upstream:    u,
		mountPrefix: strings.TrimRight(mount, "/"),
		indexName:   idx,
		cacheDir:    cfg.CacheDir,
		verifier:    cfg.Verifier,
		client:      client,
		log:         lg,
		activity:    make(chan struct{}, 1),
	}, nil
}

// Handler returns the proxy's http.Handler. It is the bare handler rather than
// an http.ServeMux on purpose: opkg's "src .../" config yields request paths
// like "//Packages.gz", and ServeMux would answer those with a 301/307 path-
// clean redirect that BusyBox wget does not follow. handle does its own
// path.Clean, so it accepts the doubled slash directly.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.handle)
}

// relPath validates the request path against the mount prefix and returns the
// feed-relative path. It rejects anything that escapes the feed root.
func (s *Server) relPath(reqPath string) (string, error) {
	clean := path.Clean(reqPath)
	if s.mountPrefix != "" && !strings.HasPrefix(clean, s.mountPrefix) {
		return "", errors.New("outside mount prefix")
	}
	rel := strings.TrimPrefix(clean, s.mountPrefix)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." {
		return "", errors.New("empty path")
	}
	// path.Clean has collapsed any traversal; a leading ".." remaining here
	// means the request resolved above the mount, which must never map upstream.
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", errors.New("path traversal")
	}
	return rel, nil
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	select {
	case s.activity <- struct{}{}:
	default:
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel, err := s.relPath(r.URL.Path)
	if err != nil {
		s.log.Warn("reject", "path", r.URL.Path, "err", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if path.Base(rel) == s.indexName {
		s.serveIndex(w, r, rel)
		return
	}
	s.serveFile(w, r, rel)
}

// serveIndex fetches the index and its detached signature, verifies the
// signature with the release key, caches the parsed manifest, then serves the
// verified index bytes.
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request, rel string) {
	body, err := s.loadIndex(r.Context(), rel)
	if err != nil {
		s.log.Error("index", "rel", rel, "err", err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(body)
}

// loadIndex fetches rel and rel.sig, verifies, parses the manifest, caches it,
// and returns the verified index bytes.
func (s *Server) loadIndex(ctx context.Context, rel string) ([]byte, error) {
	body, err := s.get(ctx, rel, maxIndexBytes)
	if err != nil {
		return nil, fmt.Errorf("fetch index: %w", err)
	}
	sig, err := s.get(ctx, rel+".sig", maxIndexBytes)
	if err != nil {
		return nil, fmt.Errorf("fetch signature: %w", err)
	}
	if err := s.verifier.Verify(body, sig); err != nil {
		return nil, fmt.Errorf("verify index: %w", err)
	}
	m, err := parsePackages(body)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.manifest = m
	s.mu.Unlock()
	return body, nil
}

// serveFile authenticates a .ipk against the verified manifest and serves it.
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, rel string) {
	s.mu.Lock()
	m := s.manifest
	s.mu.Unlock()
	if m == nil {
		// opkg normally fetches the index first, but be robust if a file is
		// requested cold: load and verify the index now.
		if _, err := s.loadIndex(r.Context(), s.indexName); err != nil {
			s.log.Error("file: index load", "rel", rel, "err", err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		s.mu.Lock()
		m = s.manifest
		s.mu.Unlock()
	}
	want, ok := m.digestFor(rel)
	if !ok {
		s.log.Warn("file: not in verified index", "rel", rel)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	tmp, gotDigest, err := s.download(r.Context(), rel)
	if err != nil {
		s.log.Error("file: download", "rel", rel, "err", err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	defer os.Remove(tmp)
	if gotDigest != want {
		s.log.Error("file: digest mismatch", "rel", rel, "want", want, "got", gotDigest)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}

	f, err := os.Open(tmp)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = io.Copy(w, f)
}

// doGet issues a bounded GET for a feed-relative path and returns the live
// response on HTTP 200. On any error (including non-200) it returns a non-nil
// error with the body already closed, so callers never leak a connection.
func (s *Server) doGet(ctx context.Context, rel string) (*http.Response, error) {
	u, err := s.upstreamURL(rel)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("upstream status %d for %s", resp.StatusCode, rel)
	}
	return resp, nil
}

// download streams rel from the upstream into a temp file in the cache dir,
// computing its SHA-256 in the same pass, and returns the temp path and the
// lowercase hex digest. The caller verifies the digest and removes the file.
func (s *Server) download(ctx context.Context, rel string) (string, string, error) {
	resp, err := s.doGet(ctx, rel)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	f, err := os.CreateTemp(s.cacheDir, "ipk-*.part")
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxPackageBytes+1))
	closeErr := f.Close()
	if err != nil {
		os.Remove(f.Name())
		return "", "", err
	}
	if closeErr != nil {
		os.Remove(f.Name())
		return "", "", closeErr
	}
	if n > maxPackageBytes {
		os.Remove(f.Name())
		return "", "", fmt.Errorf("package exceeds %d bytes", maxPackageBytes)
	}
	return f.Name(), hex.EncodeToString(h.Sum(nil)), nil
}

// get fetches rel from the upstream into memory, bounded by limit bytes.
func (s *Server) get(ctx context.Context, rel string, limit int64) ([]byte, error) {
	resp, err := s.doGet(ctx, rel)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response for %s exceeds %d bytes", rel, limit)
	}
	return body, nil
}

// upstreamURL joins the upstream base with a validated feed-relative path,
// re-checking that the result stays under the base path.
func (s *Server) upstreamURL(rel string) (string, error) {
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("illegal path %q", rel)
	}
	u := *s.upstream
	u.Path = path.Join(s.upstream.Path, rel)
	if !strings.HasPrefix(u.Path, s.upstream.Path) {
		return "", fmt.Errorf("path %q escapes base", rel)
	}
	return u.String(), nil
}

// Serve runs the HTTP server on ln until ctx is cancelled. If idleTimeout > 0
// it also returns once no request has been handled for that duration, so an
// on-demand launcher (openaps-update starts the proxy, runs opkg, lets it
// exit) does not leave a daemon resident. idleTimeout <= 0 runs until ctx.
func (s *Server) Serve(ctx context.Context, ln net.Listener, idleTimeout time.Duration) error {
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}

	idleCtx, cancelIdle := context.WithCancel(ctx)
	defer cancelIdle()
	if idleTimeout > 0 {
		go s.idleWatch(idleCtx, idleTimeout, cancelIdle)
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case <-idleCtx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		<-errc
		return nil
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) idleWatch(ctx context.Context, timeout time.Duration, fire context.CancelFunc) {
	t := time.NewTimer(timeout)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.activity:
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			t.Reset(timeout)
		case <-t.C:
			s.log.Info("idle, shutting down", "timeout", timeout)
			fire()
			return
		}
	}
}

package tlsproxy

import (
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bolkedebruin/openaps/internal/releasekey"
)

// feed builds an in-memory package feed: a set of .ipk bodies keyed by their
// feed-relative filename, a gzipped Packages index pinning each one's real
// SHA-256, and a detached signature over the served (gzipped) index bytes.
type feed struct {
	priv     *rsa.PrivateKey
	ipks     map[string][]byte // filename -> body
	indexGz  []byte
	indexSig []byte
}

// newFeed signs over the gzipped index bytes, matching what serveIndex hands
// to Verifier.Verify (the raw served body).
func newFeed(t *testing.T, ipks map[string][]byte) *feed {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	var sb bytes.Buffer
	for name, body := range ipks {
		sum := sha256.Sum256(body)
		fmt.Fprintf(&sb, "Package: %s\nVersion: 1.0.0\nFilename: %s\nSHA256sum: %s\n\n",
			name, name, hex.EncodeToString(sum[:]))
	}
	var gzbuf bytes.Buffer
	zw := gzip.NewWriter(&gzbuf)
	if _, err := zw.Write(sb.Bytes()); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	indexGz := gzbuf.Bytes()

	sum := sha256.Sum256(indexGz)
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return &feed{priv: priv, ipks: ipks, indexGz: indexGz, indexSig: sig}
}

func (f *feed) verifier(t *testing.T) Verifier {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&f.priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	v, err := releasekey.Load(pemBytes)
	if err != nil {
		t.Fatalf("load verifier: %v", err)
	}
	return v
}

// upstream serves the feed over TLS. tamperIndex, when set, mutates the served
// index bytes after signing so the signature no longer matches.
func (f *feed) upstream(t *testing.T, tamperIndex bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/Packages.gz", func(w http.ResponseWriter, r *http.Request) {
		body := f.indexGz
		if tamperIndex {
			body = append([]byte(nil), f.indexGz...)
			body[len(body)-1] ^= 0xff
		}
		w.Write(body)
	})
	mux.HandleFunc("/Packages.gz.sig", func(w http.ResponseWriter, r *http.Request) {
		w.Write(f.indexSig)
	})
	for name, body := range f.ipks {
		b := body
		mux.HandleFunc("/"+name, func(w http.ResponseWriter, r *http.Request) {
			w.Write(b)
		})
	}
	ts := httptest.NewTLSServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// newServer wires a Server to the test upstream, injecting ts.Client() so TLS
// to the throwaway cert is trusted.
func newServer(t *testing.T, f *feed, ts *httptest.Server) *Server {
	t.Helper()
	srv, err := New(Config{
		UpstreamBase: ts.URL,
		CacheDir:     t.TempDir(),
		Verifier:     f.verifier(t),
		Client:       ts.Client(),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv
}

// New rejects http upstreams; the test upstream is https via NewTLSServer, but
// guard against accidental scheme drift.
func TestNewRequiresHTTPS(t *testing.T) {
	f := newFeed(t, map[string][]byte{"a.ipk": []byte("x")})
	_, err := New(Config{
		UpstreamBase: "http://example.com/feed",
		CacheDir:     t.TempDir(),
		Verifier:     f.verifier(t),
	})
	if err == nil {
		t.Fatal("expected rejection of non-https upstream")
	}
}

func TestNewRequiresVerifierAndCacheDir(t *testing.T) {
	if _, err := New(Config{UpstreamBase: "https://x/", CacheDir: "x"}); err == nil {
		t.Fatal("expected error without verifier")
	}
	f := newFeed(t, map[string][]byte{"a.ipk": []byte("x")})
	if _, err := New(Config{UpstreamBase: "https://x/", Verifier: f.verifier(t)}); err == nil {
		t.Fatal("expected error without cache dir")
	}
}

func TestServeIndexHappy(t *testing.T) {
	f := newFeed(t, map[string][]byte{"openaps_1.0.0_arm.ipk": []byte("ipk-body-bytes")})
	ts := f.upstream(t, false)
	srv := newServer(t, f, ts)

	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	resp, err := http.Get(front.URL + "/Packages.gz")
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("index status = %d, want 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, f.indexGz) {
		t.Fatal("served index bytes do not match verified upstream bytes")
	}
	// Manifest must be cached after a successful index fetch.
	srv.mu.Lock()
	cached := srv.manifest
	srv.mu.Unlock()
	if cached == nil {
		t.Fatal("manifest not cached after index fetch")
	}
}

func TestServeIPKHappy(t *testing.T) {
	body := []byte("the real .ipk payload")
	f := newFeed(t, map[string][]byte{"openaps_1.0.0_arm.ipk": body})
	ts := f.upstream(t, false)
	srv := newServer(t, f, ts)

	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	// Prime the index first.
	if _, err := http.Get(front.URL + "/Packages.gz"); err != nil {
		t.Fatalf("prime index: %v", err)
	}

	resp, err := http.Get(front.URL + "/openaps_1.0.0_arm.ipk")
	if err != nil {
		t.Fatalf("get ipk: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ipk status = %d, want 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, body) {
		t.Fatal("served .ipk bytes differ from upstream")
	}
}

// A cold .ipk request with no prior index fetch must still load+verify the
// index before serving.
func TestServeIPKColdLoadsIndex(t *testing.T) {
	body := []byte("cold payload")
	f := newFeed(t, map[string][]byte{"cold_1.0.0_arm.ipk": body})
	ts := f.upstream(t, false)
	srv := newServer(t, f, ts)

	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	resp, err := http.Get(front.URL + "/cold_1.0.0_arm.ipk")
	if err != nil {
		t.Fatalf("get cold ipk: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cold ipk status = %d, want 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, body) {
		t.Fatal("cold .ipk bytes differ from upstream")
	}
	srv.mu.Lock()
	cached := srv.manifest
	srv.mu.Unlock()
	if cached == nil {
		t.Fatal("cold request did not cache manifest")
	}
}

// A tampered index body fails the signature check -> 502, no bytes served.
func TestServeIndexTamperedFailsClosed(t *testing.T) {
	f := newFeed(t, map[string][]byte{"openaps_1.0.0_arm.ipk": []byte("x")})
	ts := f.upstream(t, true) // tamper after signing
	srv := newServer(t, f, ts)

	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	resp, err := http.Get(front.URL + "/Packages.gz")
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("tampered index status = %d, want 502", resp.StatusCode)
	}
	srv.mu.Lock()
	cached := srv.manifest
	srv.mu.Unlock()
	if cached != nil {
		t.Fatal("tampered index must not populate the cached manifest")
	}
}

// An .ipk whose served bytes do not hash to the pinned digest -> 502 and the
// bytes are not served.
func TestServeIPKDigestMismatchFailsClosed(t *testing.T) {
	pinned := []byte("the bytes the index pins")
	f := newFeed(t, map[string][]byte{"openaps_1.0.0_arm.ipk": pinned})
	// Override the served body with different bytes after the index is signed
	// against the original hash.
	ts := f.upstream(t, false)
	srv := newServer(t, f, ts)

	// Re-point the upstream to serve mismatching .ipk bytes while keeping the
	// original signed index, so the digest pin no longer matches the body.
	mux := http.NewServeMux()
	mux.HandleFunc("/Packages.gz", func(w http.ResponseWriter, r *http.Request) { w.Write(f.indexGz) })
	mux.HandleFunc("/Packages.gz.sig", func(w http.ResponseWriter, r *http.Request) { w.Write(f.indexSig) })
	mux.HandleFunc("/openaps_1.0.0_arm.ipk", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ATTACKER-SUBSTITUTED-BYTES"))
	})
	ts.Config.Handler = mux

	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	resp, err := http.Get(front.URL + "/openaps_1.0.0_arm.ipk")
	if err != nil {
		t.Fatalf("get ipk: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("mismatch status = %d, want 502", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if bytes.Contains(got, []byte("ATTACKER-SUBSTITUTED-BYTES")) {
		t.Fatal("mismatching .ipk bytes were served to the client")
	}
}

// An .ipk not present in the verified index -> 404.
func TestServeIPKNotInIndex(t *testing.T) {
	f := newFeed(t, map[string][]byte{"listed_1.0.0_arm.ipk": []byte("x")})
	ts := f.upstream(t, false)
	srv := newServer(t, f, ts)

	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	// Prime the index.
	if _, err := http.Get(front.URL + "/Packages.gz"); err != nil {
		t.Fatalf("prime index: %v", err)
	}

	resp, err := http.Get(front.URL + "/unlisted_9.9.9_arm.ipk")
	if err != nil {
		t.Fatalf("get unlisted: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unlisted status = %d, want 404", resp.StatusCode)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	f := newFeed(t, map[string][]byte{"a.ipk": []byte("x")})
	ts := f.upstream(t, false)
	srv := newServer(t, f, ts)

	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	req, _ := http.NewRequest(http.MethodPost, front.URL+"/Packages.gz", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", resp.StatusCode)
	}
}

func TestHEADReturns200NoBody(t *testing.T) {
	f := newFeed(t, map[string][]byte{"openaps_1.0.0_arm.ipk": []byte("ipk-body")})
	ts := f.upstream(t, false)
	srv := newServer(t, f, ts)

	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	// HEAD on the index.
	req, _ := http.NewRequest(http.MethodHead, front.URL+"/Packages.gz", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("head index: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HEAD index status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) != 0 {
		t.Fatalf("HEAD index returned %d body bytes, want 0", len(body))
	}

	// HEAD on a listed .ipk (verifies digest then returns no body).
	req, _ = http.NewRequest(http.MethodHead, front.URL+"/openaps_1.0.0_arm.ipk", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("head ipk: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HEAD ipk status = %d, want 200", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) != 0 {
		t.Fatalf("HEAD ipk returned %d body bytes, want 0", len(body))
	}
}

// Path traversal must never escape the feed root. Both an encoded ..%2f and a
// literal ../ are rejected (404) and never reach the upstream as a parent path.
func TestPathTraversalRejected(t *testing.T) {
	f := newFeed(t, map[string][]byte{"a.ipk": []byte("x")})
	ts := f.upstream(t, false)
	srv := newServer(t, f, ts)

	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	for _, p := range []string{
		"/../etc/passwd",
		"/sub/../../etc/passwd",
		"/..%2f..%2fetc%2fpasswd",
	} {
		// Build the request URL by raw concatenation so the encoded form is
		// preserved on the wire rather than normalized by url.Parse.
		resp, err := http.Get(front.URL + p)
		if err != nil {
			t.Fatalf("get %q: %v", p, err)
		}
		sc := resp.StatusCode
		resp.Body.Close()
		if sc == http.StatusOK {
			t.Fatalf("traversal %q returned 200, want rejection", p)
		}
	}
}

// A signed index whose signature is over different bytes (wrong key/sig) fails
// closed even though the bytes themselves are well-formed gzip.
func TestServeIndexWrongSignatureFailsClosed(t *testing.T) {
	f := newFeed(t, map[string][]byte{"a.ipk": []byte("x")})
	// Corrupt the detached signature.
	f.indexSig = append([]byte(nil), f.indexSig...)
	f.indexSig[0] ^= 0xff
	ts := f.upstream(t, false)
	srv := newServer(t, f, ts)

	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	resp, err := http.Get(front.URL + "/Packages.gz")
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("bad-signature index status = %d, want 502", resp.StatusCode)
	}
}

// TestDoubledSlashPathServesIndex guards against http.ServeMux's path-clean
// redirect: opkg's "src .../" yields "//Packages.gz", which must be served
// directly (BusyBox wget does not follow the 301 a ServeMux would issue).
func TestDoubledSlashPathServesIndex(t *testing.T) {
	f := newFeed(t, map[string][]byte{"openaps_1.0.0_arm.ipk": []byte("ipk-body-bytes")})
	ts := f.upstream(t, false)
	srv := newServer(t, f, ts)
	front := httptest.NewServer(srv.Handler())
	t.Cleanup(front.Close)

	req, err := http.NewRequest(http.MethodGet, front.URL+"//Packages.gz", nil)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	// Do not follow redirects: a ServeMux would 301 here, which must NOT happen.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("doubled-slash index: status %d, want 200 (no redirect)", resp.StatusCode)
	}
}

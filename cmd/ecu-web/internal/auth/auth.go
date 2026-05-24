// Package auth gates the ecu-web control surface behind a single
// operator password. The password is never stored in clear: only a
// PBKDF2-HMAC-SHA256 hash + random salt live on disk (ecu-web's own
// state dir, not an ECU device-config file). Sessions are random
// opaque tokens held in memory and carried in a Secure, HttpOnly
// cookie. No password set yet => the UI forces first-run setup.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	cookieName = "ecuweb_session"
	// pbkdf2Iter is tuned for the ECU's ARMv7 CPU (~0.7s per derive);
	// the hash is stored locally and reachable only with root.
	pbkdf2Iter  = 10_000
	pbkdf2KeyLn = 32
	saltLen     = 16
	tokenLen    = 32
	sessionTTL  = 12 * time.Hour
	minPassword = 8
)

// ErrNotConfigured is returned by Login when no password has been set.
var ErrNotConfigured = errors.New("auth: no password configured")

// creds is the on-disk credential record.
type creds struct {
	Salt string `json:"salt"` // base64
	Hash string `json:"hash"` // base64 PBKDF2 output
	Iter int    `json:"iter"`
}

// Manager owns credentials and live sessions. Safe for concurrent use.
type Manager struct {
	path string // credentials file path

	mu       sync.RWMutex
	c        *creds // nil => not configured
	sessions map[string]time.Time
}

// NewManager loads credentials from path (if present) and returns a
// ready Manager. A missing file is not an error — it means first-run.
func NewManager(path string) (*Manager, error) {
	m := &Manager{path: path, sessions: make(map[string]time.Time)}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return m, nil
		}
		return nil, fmt.Errorf("auth: read %s: %w", path, err)
	}
	var c creds
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("auth: parse %s: %w", path, err)
	}
	m.c = &c
	return m, nil
}

// Configured reports whether a password has been set.
func (m *Manager) Configured() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.c != nil
}

// SetPassword installs (or replaces) the operator password, persisting
// the hash to disk. Replacing an existing password is allowed only when
// the caller is already authenticated; the HTTP layer enforces that.
func (m *Manager) SetPassword(pw string) error {
	if len(pw) < minPassword {
		return fmt.Errorf("auth: password must be at least %d characters", minPassword)
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hash, err := pbkdf2.Key(sha256.New, pw, salt, pbkdf2Iter, pbkdf2KeyLn)
	if err != nil {
		return err
	}
	c := &creds{
		Salt: base64.StdEncoding.EncodeToString(salt),
		Hash: base64.StdEncoding.EncodeToString(hash),
		Iter: pbkdf2Iter,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(m.path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	if err := os.WriteFile(m.path, b, 0o600); err != nil {
		return fmt.Errorf("auth: write %s: %w", m.path, err)
	}
	m.mu.Lock()
	m.c = c
	m.mu.Unlock()
	return nil
}

// verify checks pw against the stored hash in constant time.
func (m *Manager) verify(pw string) (bool, error) {
	m.mu.RLock()
	c := m.c
	m.mu.RUnlock()
	if c == nil {
		return false, ErrNotConfigured
	}
	salt, err := base64.StdEncoding.DecodeString(c.Salt)
	if err != nil {
		return false, err
	}
	want, err := base64.StdEncoding.DecodeString(c.Hash)
	if err != nil {
		return false, err
	}
	iter := c.Iter
	if iter <= 0 {
		iter = pbkdf2Iter
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// Login verifies the password and, on success, returns a new session
// token. Returns ErrNotConfigured if no password is set.
func (m *Manager) Login(pw string) (string, error) {
	ok, err := m.verify(pw)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("auth: invalid password")
	}
	return m.newSession()
}

// NewSession mints a session token without deriving the password. Used
// right after SetPassword so first-run setup pays one PBKDF2, not two.
func (m *Manager) NewSession() (string, error) { return m.newSession() }

func (m *Manager) newSession() (string, error) {
	raw := make([]byte, tokenLen)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	tok := base64.RawURLEncoding.EncodeToString(raw)
	m.mu.Lock()
	m.sessions[tok] = time.Now().Add(sessionTTL)
	m.mu.Unlock()
	return tok, nil
}

// validToken reports whether tok names a live, unexpired session.
func (m *Manager) validToken(tok string) bool {
	if tok == "" {
		return false
	}
	m.mu.RLock()
	exp, ok := m.sessions[tok]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		m.mu.Lock()
		delete(m.sessions, tok)
		m.mu.Unlock()
		return false
	}
	return true
}

// Logout invalidates the session carried by r, if any.
func (m *Manager) Logout(r *http.Request) {
	ck, err := r.Cookie(cookieName)
	if err != nil {
		return
	}
	m.mu.Lock()
	delete(m.sessions, ck.Value)
	m.mu.Unlock()
}

// Authenticated reports whether r carries a live session.
func (m *Manager) Authenticated(r *http.Request) bool {
	ck, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return m.validToken(ck.Value)
}

// SetCookie writes the session cookie onto w. secure controls the
// Secure attribute (true under TLS).
func SetCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

// ClearCookie expires the session cookie on w.
func ClearCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// Require wraps next so requests without a live session get 401.
func (m *Manager) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.Authenticated(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

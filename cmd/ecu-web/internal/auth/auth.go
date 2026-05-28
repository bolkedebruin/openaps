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
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	maxSessions = 64 // cap the in-memory session map (single operator)
	// Login backoff: consecutive failures beyond this add a growing delay,
	// throttling brute force without a lockout (which an attacker could weaponise
	// to deny the operator). PBKDF2 already costs ~0.7s/attempt.
	failGrace   = 3
	failStep    = 500 * time.Millisecond
	failMaxWait = 5 * time.Second
	// recoveryBytes is the entropy of the operator recovery code (80 bits,
	// rendered as 16 base32 chars). It's a system-generated, high-entropy
	// secret, so a single SHA-256 (not slow PBKDF2) guards it adequately.
	recoveryBytes = 10
	// stepUpTTL bounds how long a successful /api/auth/verify marks the
	// session as "stepped up" for sensitive writes. Time-bounded (not
	// single-use) so one Verify can cover a brief save retry.
	stepUpTTL = 60 * time.Second
)

// ErrNotConfigured is returned by Login when no password has been set.
var ErrNotConfigured = errors.New("auth: no password configured")

// ErrAlreadyConfigured is returned by Setup when a password already exists.
var ErrAlreadyConfigured = errors.New("auth: already configured")

// ErrInvalidPassword is returned when a supplied password doesn't match.
var ErrInvalidPassword = errors.New("auth: invalid password")

// ErrInvalidRecovery is returned when a supplied recovery code doesn't match.
var ErrInvalidRecovery = errors.New("auth: invalid recovery code")

// ErrNoRecovery is returned when recovery is attempted but no recovery code
// has been set (e.g. an install created before recovery codes existed).
var ErrNoRecovery = errors.New("auth: no recovery code configured")

// creds is the on-disk credential record.
type creds struct {
	Salt         string `json:"salt"` // base64
	Hash         string `json:"hash"` // base64 PBKDF2 output
	Iter         int    `json:"iter"`
	RecoveryHash string `json:"recovery_hash,omitempty"` // base64 SHA-256 of the recovery code
}

// session carries the per-token bookkeeping. expires gates the cookie
// lifetime; stepUpAt is the wall-clock timestamp of the most recent
// successful step-up Verify (zero = none), used by StepUpValid to gate
// sensitive writes for stepUpTTL.
type session struct {
	expires  time.Time
	stepUpAt time.Time
}

// Manager owns credentials and live sessions. Safe for concurrent use.
type Manager struct {
	path string // credentials file path

	mu        sync.RWMutex
	c         *creds // nil => not configured
	sessions  map[string]*session
	failCount int // consecutive failed logins (for backoff)

	// now is the wall-clock source. Defaults to time.Now; tests override
	// it to drive step-up expiry without real-time sleeps.
	now func() time.Time
}

// NewManager loads credentials from path (if present) and returns a
// ready Manager. A missing file is not an error — it means first-run.
func NewManager(path string) (*Manager, error) {
	m := &Manager{path: path, sessions: make(map[string]*session), now: time.Now}
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

// derivePassword derives a fresh salt + PBKDF2 hash for pw (slow; call outside
// any lock).
func derivePassword(pw string) (saltB64, hashB64 string, err error) {
	if len(pw) < minPassword {
		return "", "", fmt.Errorf("auth: password must be at least %d characters", minPassword)
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", "", err
	}
	hash, err := pbkdf2.Key(sha256.New, pw, salt, pbkdf2Iter, pbkdf2KeyLn)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(hash), nil
}

var recoveryEnc = base32.StdEncoding.WithPadding(base32.NoPadding)

// generateRecoveryCode returns a fresh recovery code as XXXX-XXXX-XXXX-XXXX.
func generateRecoveryCode() (string, error) {
	raw := make([]byte, recoveryBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	s := recoveryEnc.EncodeToString(raw)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return b.String(), nil
}

// normalizeRecovery makes recovery-code entry forgiving: upper-cased, with
// spaces and dashes stripped, so "abcd-efgh" and "ABCDEFGH" hash the same.
func normalizeRecovery(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if r == '-' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// hashRecovery returns the base64 SHA-256 of the normalized recovery code.
func hashRecovery(code string) string {
	sum := sha256.Sum256([]byte(normalizeRecovery(code)))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// persistCreds writes the credential record to disk (0600 in a 0700 dir).
func (m *Manager) persistCreds(c *creds) error {
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
	return nil
}

// SetPassword installs (or replaces) the operator password, persisting the
// hash to disk. Any existing recovery code is preserved. Replacing an existing
// password is allowed only when the caller is already authenticated; the HTTP
// layer enforces that (see ChangePassword for the verified variant).
func (m *Manager) SetPassword(pw string) error {
	salt, hash, err := derivePassword(pw)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c := &creds{Salt: salt, Hash: hash, Iter: pbkdf2Iter}
	if m.c != nil {
		c.RecoveryHash = m.c.RecoveryHash
	}
	if err := m.persistCreds(c); err != nil {
		return err
	}
	m.c = c
	return nil
}

// Setup installs the FIRST operator password plus a one-time recovery code,
// atomically: if a password is already configured it returns
// ErrAlreadyConfigured. This closes the check-then-act race on the public
// first-run endpoint (two concurrent setups, or a second setup after one
// already won). The returned recovery code is shown to the operator once;
// only its hash is persisted.
func (m *Manager) Setup(pw string) (string, error) {
	salt, hash, err := derivePassword(pw) // derive before taking the lock
	if err != nil {
		return "", err
	}
	code, err := generateRecoveryCode()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.c != nil {
		return "", ErrAlreadyConfigured
	}
	c := &creds{Salt: salt, Hash: hash, Iter: pbkdf2Iter, RecoveryHash: hashRecovery(code)}
	if err := m.persistCreds(c); err != nil {
		return "", err
	}
	m.c = c
	return code, nil
}

// ChangePassword replaces the operator password after verifying the current
// one. The recovery code is left unchanged. Returns ErrInvalidPassword if the
// current password is wrong.
func (m *Manager) ChangePassword(current, next string) error {
	ok, err := m.verify(current)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidPassword
	}
	return m.SetPassword(next)
}

// Recover resets the password using the recovery code, then rotates the
// recovery code (single-use) and invalidates all live sessions. It returns the
// NEW recovery code (shown once). Throttled like Login to slow brute force.
func (m *Manager) Recover(code, newPw string) (string, error) {
	m.mu.RLock()
	c := m.c
	m.mu.RUnlock()
	if c == nil {
		return "", ErrNotConfigured
	}
	if c.RecoveryHash == "" {
		return "", ErrNoRecovery
	}
	if subtle.ConstantTimeCompare([]byte(hashRecovery(code)), []byte(c.RecoveryHash)) != 1 {
		m.mu.Lock()
		m.failCount++
		n := m.failCount
		m.mu.Unlock()
		if d := loginBackoff(n); d > 0 {
			time.Sleep(d)
		}
		return "", ErrInvalidRecovery
	}
	salt, hash, err := derivePassword(newPw)
	if err != nil {
		return "", err
	}
	newCode, err := generateRecoveryCode()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failCount = 0
	nc := &creds{Salt: salt, Hash: hash, Iter: pbkdf2Iter, RecoveryHash: hashRecovery(newCode)}
	if err := m.persistCreds(nc); err != nil {
		return "", err
	}
	m.c = nc
	// A successful recovery may be us locking out whoever knew the old
	// password — drop every existing session.
	m.sessions = make(map[string]*session)
	return newCode, nil
}

// RegenerateRecovery issues a fresh recovery code (invalidating the old one)
// without changing the password. Returns the new code, shown once.
func (m *Manager) RegenerateRecovery() (string, error) {
	code, err := generateRecoveryCode()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.c == nil {
		return "", ErrNotConfigured
	}
	nc := *m.c
	nc.RecoveryHash = hashRecovery(code)
	if err := m.persistCreds(&nc); err != nil {
		return "", err
	}
	m.c = &nc
	return code, nil
}

// RecoveryConfigured reports whether a recovery code has been set.
func (m *Manager) RecoveryConfigured() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.c != nil && m.c.RecoveryHash != ""
}

// Verify reports whether pw matches the stored password (constant-time).
// Returns ErrNotConfigured when no password is set. Used by the step-up
// auth endpoint to gate sensitive UI confirmations (e.g. a MAC change
// that would re-PAN the radio) without minting a new session.
//
// Verify shares Login's failure counter and backoff: a wrong Verify is
// not cheaper than a wrong Login, so an authenticated attacker can't use
// the step-up endpoint to brute-force the password unthrottled.
func (m *Manager) Verify(pw string) (bool, error) {
	ok, err := m.verify(pw)
	if err != nil {
		return false, err
	}
	if !ok {
		m.mu.Lock()
		m.failCount++
		n := m.failCount
		m.mu.Unlock()
		if d := loginBackoff(n); d > 0 {
			time.Sleep(d)
		}
		return false, nil
	}
	m.mu.Lock()
	m.failCount = 0
	m.mu.Unlock()
	return true, nil
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
		// Throttle brute force: grow a delay with consecutive failures (no
		// lockout, which an attacker could use to deny the operator).
		m.mu.Lock()
		m.failCount++
		n := m.failCount
		m.mu.Unlock()
		if d := loginBackoff(n); d > 0 {
			time.Sleep(d)
		}
		return "", ErrInvalidPassword
	}
	m.mu.Lock()
	m.failCount = 0
	m.mu.Unlock()
	return m.newSession()
}

// loginBackoff returns the delay to apply after the nth consecutive failure.
func loginBackoff(n int) time.Duration {
	if n <= failGrace {
		return 0
	}
	d := time.Duration(n-failGrace) * failStep
	if d > failMaxWait {
		d = failMaxWait
	}
	return d
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
	now := m.clock()
	m.mu.Lock()
	// Drop expired sessions, then bound the map so it can't grow without limit.
	for t, s := range m.sessions {
		if now.After(s.expires) {
			delete(m.sessions, t)
		}
	}
	if len(m.sessions) >= maxSessions {
		// Evict the soonest-to-expire session to make room.
		var oldest string
		var oldestExp time.Time
		for t, s := range m.sessions {
			if oldest == "" || s.expires.Before(oldestExp) {
				oldest, oldestExp = t, s.expires
			}
		}
		delete(m.sessions, oldest)
	}
	m.sessions[tok] = &session{expires: now.Add(sessionTTL)}
	m.mu.Unlock()
	return tok, nil
}

// clock returns the configured wall-clock, defaulting to time.Now if a
// caller built a Manager without it.
func (m *Manager) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

// SetClock overrides the wall-clock source. Tests use this to drive
// step-up TTL expiry without real-time sleeps; production callers leave
// the default time.Now.
func (m *Manager) SetClock(now func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
}

// validToken reports whether tok names a live, unexpired session.
func (m *Manager) validToken(tok string) bool {
	if tok == "" {
		return false
	}
	m.mu.RLock()
	s, ok := m.sessions[tok]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	if m.clock().After(s.expires) {
		m.mu.Lock()
		delete(m.sessions, tok)
		m.mu.Unlock()
		return false
	}
	return true
}

// MarkStepUp records that the session carried by r has just completed a
// successful step-up Verify; subsequent calls to StepUpValid within
// stepUpTTL will report true. No-op if r has no live session.
func (m *Manager) MarkStepUp(r *http.Request) {
	ck, err := r.Cookie(cookieName)
	if err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[ck.Value]; ok {
		s.stepUpAt = m.clock()
	}
}

// ConsumeStepUp clears the stepped-up flag on the session carried by r,
// if any. The HTTP layer calls it after each successful sensitive write
// so the next sensitive write requires a fresh Verify even within the
// stepUpTTL window — an authed/compromised session cannot batch multiple
// sensitive operations off one password entry. No-op if r has no live
// session or no step-up was set.
func (m *Manager) ConsumeStepUp(r *http.Request) {
	ck, err := r.Cookie(cookieName)
	if err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[ck.Value]; ok {
		s.stepUpAt = time.Time{}
	}
}

// StepUpValid reports whether the session carried by r is currently
// stepped-up (a successful Verify within the last stepUpTTL). The flag
// is time-bounded, not single-use: one Verify covers a brief save retry,
// but after stepUpTTL the next sensitive write requires a fresh Verify.
func (m *Manager) StepUpValid(r *http.Request) bool {
	ck, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[ck.Value]
	if !ok {
		return false
	}
	if s.stepUpAt.IsZero() {
		return false
	}
	return m.clock().Sub(s.stepUpAt) <= stepUpTTL
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

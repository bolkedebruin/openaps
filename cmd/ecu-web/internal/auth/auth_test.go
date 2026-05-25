package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestSetupLoginLogoutLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	m, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Configured() {
		t.Fatal("fresh manager should be unconfigured")
	}

	if err := m.SetPassword("short"); err == nil {
		t.Error("expected rejection of too-short password")
	}
	if err := m.SetPassword("correct-horse"); err != nil {
		t.Fatal(err)
	}
	if !m.Configured() {
		t.Fatal("should be configured after SetPassword")
	}

	// Persisted: a fresh manager loads the same credential.
	m2, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	if !m2.Configured() {
		t.Fatal("credential did not persist")
	}
	if _, err := m2.Login("wrong"); err == nil {
		t.Error("login with wrong password should fail")
	}
	tok, err := m2.Login("correct-horse")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !m2.validToken(tok) {
		t.Error("token should be valid right after login")
	}
}

func TestLoginNotConfigured(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if _, err := m.Login("anything"); err != ErrNotConfigured {
		t.Errorf("err = %v, want ErrNotConfigured", err)
	}
}

func TestAuthenticatedViaCookie(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if err := m.SetPassword("password123"); err != nil {
		t.Fatal(err)
	}
	tok, _ := m.Login("password123")

	rec := httptest.NewRecorder()
	SetCookie(rec, tok, true)
	ck := rec.Result().Cookies()[0]
	if !ck.HttpOnly || !ck.Secure || ck.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie hardening missing: %+v", ck)
	}

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(ck)
	if !m.Authenticated(r) {
		t.Error("request with valid cookie should authenticate")
	}

	m.Logout(r)
	if m.Authenticated(r) {
		t.Error("after logout the session should be gone")
	}
}

func TestRequireMiddleware(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	_ = m.SetPassword("password123")
	tok, _ := m.Login("password123")

	guarded := m.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	// no cookie -> 401
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no cookie => %d, want 401", rec.Code)
	}

	// valid cookie -> passes through
	rec = httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/x", nil)
	SetCookie(rec, tok, true)
	r.AddCookie(rec.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	guarded.ServeHTTP(rec, r)
	if rec.Code != http.StatusTeapot {
		t.Errorf("valid cookie => %d, want 418", rec.Code)
	}
}

func TestSetup_AtomicRejectsSecond(t *testing.T) {
	m, err := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Setup("password123"); err != nil {
		t.Fatalf("first Setup: %v", err)
	}
	if err := m.Setup("different456"); !errors.Is(err, ErrAlreadyConfigured) {
		t.Fatalf("second Setup = %v, want ErrAlreadyConfigured", err)
	}
	if _, err := m.Login("password123"); err != nil {
		t.Errorf("original password should still work after a rejected re-setup: %v", err)
	}
}

func TestLoginBackoff(t *testing.T) {
	if loginBackoff(failGrace) != 0 {
		t.Error("no delay within the grace count")
	}
	if loginBackoff(failGrace+1) != failStep {
		t.Errorf("first delay after grace = %v, want %v", loginBackoff(failGrace+1), failStep)
	}
	if loginBackoff(1_000_000) != failMaxWait {
		t.Errorf("delay not capped at %v", failMaxWait)
	}
}

func TestSessionCapBounded(t *testing.T) {
	m, err := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxSessions+25; i++ {
		if _, err := m.NewSession(); err != nil {
			t.Fatal(err)
		}
	}
	m.mu.RLock()
	n := len(m.sessions)
	m.mu.RUnlock()
	if n > maxSessions {
		t.Errorf("session map has %d entries, exceeds cap %d", n, maxSessions)
	}
}

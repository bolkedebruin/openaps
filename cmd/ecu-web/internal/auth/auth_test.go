package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// sessionCookie builds a request carrying the given session token in the
// auth cookie. Tests use this to drive MarkStepUp / StepUpValid without
// going through Login (which costs PBKDF2).
func sessionCookie(t *testing.T, tok string) *http.Request {
	t.Helper()
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	return r
}

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
	code, err := m.Setup("password123")
	if err != nil {
		t.Fatalf("first Setup: %v", err)
	}
	if code == "" {
		t.Fatal("Setup should return a recovery code")
	}
	if !m.RecoveryConfigured() {
		t.Fatal("recovery should be configured after Setup")
	}
	if _, err := m.Setup("different456"); !errors.Is(err, ErrAlreadyConfigured) {
		t.Fatalf("second Setup = %v, want ErrAlreadyConfigured", err)
	}
	if _, err := m.Login("password123"); err != nil {
		t.Errorf("original password should still work after a rejected re-setup: %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if _, err := m.Setup("password123"); err != nil {
		t.Fatal(err)
	}
	if err := m.ChangePassword("wrong", "newpassword"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("change with wrong current = %v, want ErrInvalidPassword", err)
	}
	if err := m.ChangePassword("password123", "short"); err == nil {
		t.Error("expected rejection of too-short new password")
	}
	if err := m.ChangePassword("password123", "newpassword"); err != nil {
		t.Fatalf("change: %v", err)
	}
	if _, err := m.Login("newpassword"); err != nil {
		t.Errorf("login with new password failed: %v", err)
	}
	if _, err := m.Login("password123"); err == nil {
		t.Error("old password should no longer work")
	}
}

func TestChangePasswordKeepsRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	m, _ := NewManager(path)
	code, _ := m.Setup("password123")
	if err := m.ChangePassword("password123", "newpassword"); err != nil {
		t.Fatal(err)
	}
	// The original recovery code must still reset the (now changed) password.
	m2, _ := NewManager(path)
	if _, err := m2.Recover(code, "thirdpassword"); err != nil {
		t.Fatalf("recovery code should survive a password change: %v", err)
	}
}

func TestRecoverResetsAndRotates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	m, _ := NewManager(path)
	code, _ := m.Setup("password123")

	// A live session should be invalidated by recovery.
	tok, _ := m.Login("password123")
	if !m.validToken(tok) {
		t.Fatal("token should be valid before recovery")
	}

	if _, err := m.Recover("WRONG-CODE-HERE-XXXX", "newpassword"); !errors.Is(err, ErrInvalidRecovery) {
		t.Fatalf("recover with wrong code = %v, want ErrInvalidRecovery", err)
	}

	newCode, err := m.Recover(code, "newpassword")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if newCode == "" || newCode == code {
		t.Errorf("recovery code should rotate: old=%q new=%q", code, newCode)
	}
	if m.validToken(tok) {
		t.Error("recovery should invalidate existing sessions")
	}
	if _, err := m.Login("newpassword"); err != nil {
		t.Errorf("login with recovered password failed: %v", err)
	}
	// The used code is single-use: it must no longer work.
	if _, err := m.Recover(code, "another"); !errors.Is(err, ErrInvalidRecovery) {
		t.Errorf("old recovery code reused = %v, want ErrInvalidRecovery", err)
	}
	// Forgiving input: lower-case and spaced variants of the new code work.
	if _, err := m.Recover(" "+newCode+" ", "spacedpass"); err != nil {
		t.Errorf("spaced recovery code rejected: %v", err)
	}
}

func TestRecoverWithoutRecoveryCode(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	// SetPassword (not Setup) installs a password with no recovery code.
	if err := m.SetPassword("password123"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Recover("anything", "newpassword"); !errors.Is(err, ErrNoRecovery) {
		t.Fatalf("recover without a recovery code = %v, want ErrNoRecovery", err)
	}
}

func TestRegenerateRecovery(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	code, _ := m.Setup("password123")
	newCode, err := m.RegenerateRecovery()
	if err != nil {
		t.Fatal(err)
	}
	if newCode == code {
		t.Error("RegenerateRecovery should produce a different code")
	}
	if _, err := m.Recover(code, "newpassword"); !errors.Is(err, ErrInvalidRecovery) {
		t.Error("old code should be invalid after regeneration")
	}
	if _, err := m.Recover(newCode, "newpassword"); err != nil {
		t.Errorf("new code should work: %v", err)
	}
}

func TestVerify(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	// Unconfigured: returns ErrNotConfigured.
	if _, err := m.Verify("anything"); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Verify unconfigured = %v, want ErrNotConfigured", err)
	}
	if err := m.SetPassword("password123"); err != nil {
		t.Fatal(err)
	}
	ok, err := m.Verify("password123")
	if err != nil {
		t.Fatalf("Verify correct: %v", err)
	}
	if !ok {
		t.Error("Verify correct = false, want true")
	}
	ok, err = m.Verify("wrong")
	if err != nil {
		t.Fatalf("Verify wrong returned err: %v", err)
	}
	if ok {
		t.Error("Verify wrong = true, want false")
	}

	// Throttling: same failure counter as Login. Burn through the grace
	// window with wrong Verify calls; the next one must take >= failStep.
	// (Reset by re-using the just-failed counter from above.)
	for i := 0; i < failGrace; i++ {
		_, _ = m.Verify("wrong")
	}
	start := time.Now()
	_, _ = m.Verify("wrong")
	if d := time.Since(start); d < failStep {
		t.Errorf("Verify after failGrace failures took %v, want >= %v", d, failStep)
	}
	// A correct Verify resets the counter.
	if _, err := m.Verify("password123"); err != nil {
		t.Fatalf("Verify resets: %v", err)
	}
	start = time.Now()
	_, _ = m.Verify("wrong")
	if d := time.Since(start); d >= failStep {
		t.Errorf("first failure after reset took %v, want < %v", d, failStep)
	}
}

func TestStepUpFlow(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if err := m.SetPassword("password123"); err != nil {
		t.Fatal(err)
	}
	// Inject a clock so we can advance past stepUpTTL without sleeps.
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var nowVal atomic.Int64
	nowVal.Store(t0.UnixNano())
	m.SetClock(func() time.Time { return time.Unix(0, nowVal.Load()) })

	tok, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	r := sessionCookie(t, tok)

	// No step-up yet.
	if m.StepUpValid(r) {
		t.Fatal("StepUpValid before any Verify = true, want false")
	}

	// Wrong Verify must NOT step the session up. The handler only calls
	// MarkStepUp on a successful Verify, so simulate that contract here.
	if ok, _ := m.Verify("wrong"); ok {
		t.Fatal("wrong Verify returned true")
	}
	// (No MarkStepUp call on the wrong path.)
	if m.StepUpValid(r) {
		t.Error("StepUpValid after a wrong Verify = true, want false")
	}

	// Correct Verify + MarkStepUp -> valid.
	if ok, _ := m.Verify("password123"); !ok {
		t.Fatal("correct Verify returned false")
	}
	m.MarkStepUp(r)
	if !m.StepUpValid(r) {
		t.Error("StepUpValid right after MarkStepUp = false, want true")
	}

	// Advance past stepUpTTL: must expire.
	nowVal.Store(t0.Add(stepUpTTL + time.Second).UnixNano())
	if m.StepUpValid(r) {
		t.Error("StepUpValid after TTL = true, want false")
	}

	// A fresh successful Verify re-arms.
	if ok, _ := m.Verify("password123"); !ok {
		t.Fatal("re-verify failed")
	}
	m.MarkStepUp(r)
	if !m.StepUpValid(r) {
		t.Error("StepUpValid after re-Verify = false")
	}
}

func TestConsumeStepUp_ClearsFlag(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	if err := m.SetPassword("password123"); err != nil {
		t.Fatal(err)
	}
	tok, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	r := sessionCookie(t, tok)

	m.MarkStepUp(r)
	if !m.StepUpValid(r) {
		t.Fatal("StepUpValid right after MarkStepUp = false, want true")
	}
	m.ConsumeStepUp(r)
	if m.StepUpValid(r) {
		t.Error("StepUpValid after ConsumeStepUp = true, want false (single-use)")
	}
	// Idempotent: a second ConsumeStepUp on an already-cleared session is a no-op.
	m.ConsumeStepUp(r)
	if m.StepUpValid(r) {
		t.Error("StepUpValid after second ConsumeStepUp = true, want false")
	}
}

func TestConsumeStepUp_NoCookieIsNoop(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	_ = m.SetPassword("password123")
	r := httptest.NewRequest("GET", "/", nil) // no cookie
	// Must not panic.
	m.ConsumeStepUp(r)
}

func TestStepUpScopedToSession(t *testing.T) {
	m, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"))
	_ = m.SetPassword("password123")

	tokA, _ := m.NewSession()
	tokB, _ := m.NewSession()
	rA := sessionCookie(t, tokA)
	rB := sessionCookie(t, tokB)

	m.MarkStepUp(rA)
	if !m.StepUpValid(rA) {
		t.Error("session A should be stepped up")
	}
	if m.StepUpValid(rB) {
		t.Error("session B should NOT be stepped up — step-up must be per-session")
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

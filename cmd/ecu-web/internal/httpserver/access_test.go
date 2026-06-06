package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bolkedebruin/openaps/wire"
)

func TestListSSHKeysEndpoint(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		SSHKeysList: func(context.Context) (*wire.AccessResponse, error) {
			return &wire.AccessResponse{
				Ok:       true,
				Provider: "openaps",
				KeyCount: 1,
				Keys: []*wire.SshKey{
					{Pubkey: "ssh-ed25519 AAAA", Comment: "op", Fingerprint: "SHA256:abc", AddedMs: 100},
				},
			}, nil
		},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(httptest.NewRequest("GET", "/api/access/ssh-keys", nil), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("list => %d (%s)", rec.Code, rec.Body)
	}
	var out accessDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Provider != "openaps" || len(out.Keys) != 1 || out.Keys[0].Fingerprint != "SHA256:abc" {
		t.Errorf("unexpected DTO: %+v", out)
	}
}

func TestListSSHKeysDegradesOnError(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		SSHKeysList: func(context.Context) (*wire.AccessResponse, error) {
			return nil, errors.New("recoveryd down")
		},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(httptest.NewRequest("GET", "/api/access/ssh-keys", nil), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("should degrade to 200, got %d", rec.Code)
	}
	var out accessDTO
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Error == "" {
		t.Errorf("expected error field, got %+v", out)
	}
}

func TestListSSHKeysUnavailable(t *testing.T) {
	// No SSHKeysList wired -> 503.
	h, cookies := newSettingsServer(t, Config{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(httptest.NewRequest("GET", "/api/access/ssh-keys", nil), cookies))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestAddSSHKeyEndpoint(t *testing.T) {
	var gotPub, gotComment string
	h, cookies := newSettingsServer(t, Config{
		SSHKeyAdd: func(_ context.Context, pubkey, comment string) (*wire.AccessResponse, error) {
			gotPub, gotComment = pubkey, comment
			return &wire.AccessResponse{Ok: true, Provider: "openaps", KeyCount: 1,
				Keys: []*wire.SshKey{{Pubkey: pubkey, Comment: comment, Fingerprint: "SHA256:new"}}}, nil
		},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/access/ssh-keys",
		`{"pubkey":"ssh-ed25519 AAAA","comment":"laptop"}`), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("add => %d (%s)", rec.Code, rec.Body)
	}
	if gotPub != "ssh-ed25519 AAAA" || gotComment != "laptop" {
		t.Errorf("add passed wrong args: %q %q", gotPub, gotComment)
	}
}

func TestAddSSHKeyRejected(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		SSHKeyAdd: func(context.Context, string, string) (*wire.AccessResponse, error) {
			return &wire.AccessResponse{Ok: false, Error: "malformed pubkey"}, errors.New("recoveryd: malformed pubkey")
		},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/access/ssh-keys", `{"pubkey":"junk"}`), cookies))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body)
	}
}

func TestAddSSHKeyRequiresPubkey(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		SSHKeyAdd: func(context.Context, string, string) (*wire.AccessResponse, error) {
			t.Fatal("SSHKeyAdd should not be called for an empty pubkey")
			return nil, nil
		},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/access/ssh-keys", `{"pubkey":""}`), cookies))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestRemoveSSHKeyStepUpRequired verifies DELETE is gated by the single-use
// step-up: without a prior /api/auth/verify it is 403 and recoveryd is never
// called.
func TestRemoveSSHKeyStepUpRequired(t *testing.T) {
	called := false
	h, cookies := newSettingsServer(t, Config{
		SSHKeyRemove: func(context.Context, string) (*wire.AccessResponse, error) {
			called = true
			return &wire.AccessResponse{Ok: true}, nil
		},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("DELETE", "/api/access/ssh-keys",
		`{"fingerprint":"SHA256:abc"}`), cookies))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without step-up, got %d", rec.Code)
	}
	if called {
		t.Error("SSHKeyRemove called despite missing step-up")
	}
}

// TestRemoveSSHKeyWithStepUp verifies a verified session can delete a key,
// and that the step-up is consumed (a second delete is 403 again).
func TestRemoveSSHKeyWithStepUp(t *testing.T) {
	var removed string
	h, cookies := newSettingsServer(t, Config{
		SSHKeyRemove: func(_ context.Context, fp string) (*wire.AccessResponse, error) {
			removed = fp
			return &wire.AccessResponse{Ok: true, Provider: "openaps", KeyCount: 0}, nil
		},
	})

	// Step up.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/auth/verify", `{"password":"password123"}`), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("step-up verify => %d", rec.Code)
	}

	// First delete succeeds.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("DELETE", "/api/access/ssh-keys",
		`{"fingerprint":"SHA256:abc"}`), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete with step-up => %d (%s)", rec.Code, rec.Body)
	}
	if removed != "SHA256:abc" {
		t.Errorf("removed = %q, want SHA256:abc", removed)
	}

	// Second delete is rejected: step-up was single-use.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("DELETE", "/api/access/ssh-keys",
		`{"fingerprint":"SHA256:def"}`), cookies))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("second delete should be 403 (step-up consumed), got %d", rec.Code)
	}
}

func TestRemoveSSHKeyRequiresFingerprint(t *testing.T) {
	h, cookies := newSettingsServer(t, Config{
		SSHKeyRemove: func(context.Context, string) (*wire.AccessResponse, error) {
			t.Fatal("SSHKeyRemove should not be called for an empty fingerprint")
			return nil, nil
		},
	})
	// Step up so we get past auth into the missing-fingerprint check.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("POST", "/api/auth/verify", `{"password":"password123"}`), cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("step-up verify => %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookies(jsonReq("DELETE", "/api/access/ssh-keys", `{"fingerprint":""}`), cookies))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSSHKeysRequireAuth(t *testing.T) {
	_, h := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/access/ssh-keys", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /api/access/ssh-keys => %d, want 401", rec.Code)
	}
}

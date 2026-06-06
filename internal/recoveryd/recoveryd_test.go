package recoveryd

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// a couple of valid keys generated once and pinned here; the exact bytes
// don't matter, only that they parse as OpenSSH authorized-key lines.
const (
	keyA = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAG3oZkfNp0gi7/VMbmIhW7wccXwvKguyY3BaQmiE/pf alice@host"
	keyB = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHHvfqipwfn6y4DfenyaB3ft6lk0VIIN8YK+62tZacvn bob@host"
)

func fpOf(t *testing.T, line string) string {
	t.Helper()
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		t.Fatalf("parse pinned key: %v", err)
	}
	return ssh.FingerprintSHA256(pub)
}

func TestParseKeyValidatesAndFingerprints(t *testing.T) {
	k, err := ParseKey(keyA, "")
	if err != nil {
		t.Fatalf("ParseKey valid: %v", err)
	}
	if k.Fingerprint != fpOf(t, keyA) {
		t.Errorf("fingerprint mismatch: %s", k.Fingerprint)
	}
	if !strings.HasPrefix(k.Fingerprint, "SHA256:") {
		t.Errorf("fingerprint not SHA256 form: %s", k.Fingerprint)
	}
	if k.Comment != "alice@host" {
		t.Errorf("comment from line = %q, want alice@host", k.Comment)
	}
	// canonical pubkey drops the trailing comment.
	if strings.Contains(k.Pubkey, "alice@host") {
		t.Errorf("canonical pubkey should not contain comment: %q", k.Pubkey)
	}
}

func TestParseKeyCommentOverride(t *testing.T) {
	k, err := ParseKey(keyA, "operator override")
	if err != nil {
		t.Fatal(err)
	}
	if k.Comment != "operator override" {
		t.Errorf("comment = %q, want override", k.Comment)
	}
}

func TestParseKeyRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "   ", "not a key", "ssh-ed25519 @@@notbase64@@@"} {
		if _, err := ParseKey(bad, ""); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestParseKeyRejectsCommentNewlineInjection(t *testing.T) {
	// A comment carrying an embedded newline would inject a SECOND
	// authorized_keys line. It must be rejected outright.
	for _, bad := range []string{
		"ok\nssh-rsa AAAA attacker",
		"ok\r\ncommand=\"evil\" key",
		"tab\there",
		"null\x00byte",
	} {
		if _, err := ParseKey(keyA, bad); err == nil {
			t.Errorf("expected rejection of comment %q", bad)
		}
	}
}

// newTestManager builds a Manager over a provider whose authorized_keys
// lives under a temp dir, with a no-op dropbear key generator.
func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	prov := &Provider{
		AuthorizedKeysPath: filepath.Join(dir, ".ssh", "authorized_keys"),
		ManageDropbear:     true,
		DropbearKeyPath:    filepath.Join(dir, "hk"),
		dropbearKeyGen:     noopDropbear,
	}
	return NewManager(prov), dir
}

// noopDropbear avoids invoking the real dropbearkey binary in tests.
func noopDropbear(path string) error { return os.WriteFile(path, []byte("hostkey"), 0o600) }

func mustKeys(t *testing.T, m *Manager) []Key {
	t.Helper()
	keys, err := m.ListKeys()
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	return keys
}

func TestListParsesMultiLineFile(t *testing.T) {
	m, dir := newTestManager(t)
	akPath := filepath.Join(dir, ".ssh", "authorized_keys")
	if err := os.MkdirAll(filepath.Dir(akPath), 0o700); err != nil {
		t.Fatal(err)
	}
	// blank lines and a # comment must be tolerated/skipped.
	content := "# operator bundle\n\n" + keyA + "\n" + keyB + "\n"
	if err := os.WriteFile(akPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	keys := mustKeys(t, m)
	if len(keys) != 2 {
		t.Fatalf("parsed %d keys, want 2", len(keys))
	}
	if keys[0].Fingerprint != fpOf(t, keyA) || keys[1].Fingerprint != fpOf(t, keyB) {
		t.Errorf("fingerprints mismatch: %+v", keys)
	}
}

func TestListMissingFileIsEmpty(t *testing.T) {
	m, _ := newTestManager(t)
	if got := len(mustKeys(t, m)); got != 0 {
		t.Errorf("missing authorized_keys should list empty, got %d", got)
	}
}

func TestAddDedupesAppendsAtomic(t *testing.T) {
	m, dir := newTestManager(t)
	akPath := filepath.Join(dir, ".ssh", "authorized_keys")

	if _, err := m.AddKey(keyA, "alice"); err != nil {
		t.Fatal(err)
	}
	// adding the same key (even with a different comment line) is a no-op.
	if _, err := m.AddKey(keyA, "alice-again"); err != nil {
		t.Fatal(err)
	}
	if got := len(mustKeys(t, m)); got != 1 {
		t.Fatalf("dedupe failed: %d keys", got)
	}
	// second distinct key appends.
	if _, err := m.AddKey(keyB, "bob"); err != nil {
		t.Fatal(err)
	}
	if got := len(mustKeys(t, m)); got != 2 {
		t.Fatalf("append failed: %d keys", got)
	}
	// file is mode 0600 and dir 0700.
	fi, err := os.Stat(akPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("authorized_keys mode = %o, want 600", fi.Mode().Perm())
	}
	di, _ := os.Stat(filepath.Dir(akPath))
	if di.Mode().Perm() != 0o700 {
		t.Errorf(".ssh mode = %o, want 700", di.Mode().Perm())
	}
}

func TestAddRejectsMalformed(t *testing.T) {
	m, _ := newTestManager(t)
	if _, err := m.AddKey("garbage", ""); err == nil {
		t.Error("expected error adding malformed key")
	}
	if len(mustKeys(t, m)) != 0 {
		t.Error("malformed key was persisted")
	}
}

func TestManagerRejectsInjectedComment(t *testing.T) {
	m, dir := newTestManager(t)
	if _, err := m.AddKey(keyA, "ok\nssh-ed25519 AAAA injected"); err == nil {
		t.Fatal("expected AddKey to reject injected comment")
	}
	if len(mustKeys(t, m)) != 0 {
		t.Error("injected-comment key was persisted")
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".ssh", "authorized_keys"))
	if strings.Contains(string(b), "injected") {
		t.Errorf("injected line reached the file: %q", string(b))
	}
}

func TestOptionsSurviveRewrite(t *testing.T) {
	// A restricted key (forced command + no-pty) must keep its options
	// across the whole-file rewrite an unrelated AddKey triggers — dropping
	// them would silently widen it to a full shell.
	m, dir := newTestManager(t)
	akPath := filepath.Join(dir, ".ssh", "authorized_keys")
	if err := os.MkdirAll(filepath.Dir(akPath), 0o700); err != nil {
		t.Fatal(err)
	}
	restricted := `command="/usr/local/bin/recover",no-pty,no-port-forwarding ` + keyA + " ops"
	if err := os.WriteFile(akPath, []byte(restricted+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Adding an unrelated key rewrites the file.
	if _, err := m.AddKey(keyB, "bob"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(akPath)
	got := string(b)
	if !strings.Contains(got, `command="/usr/local/bin/recover"`) ||
		!strings.Contains(got, "no-pty") || !strings.Contains(got, "no-port-forwarding") {
		t.Errorf("restricted key options were stripped on rewrite:\n%s", got)
	}
}

func TestOptionsControlCharRejected(t *testing.T) {
	// An option carrying an embedded newline is the same injection vector
	// as a comment newline and must be rejected.
	bad := "command=\"a\nb\" " + keyA
	if _, err := ParseKey(bad, ""); err == nil {
		t.Error("expected rejection of option with embedded newline")
	}
}

func TestWriteFixesStaleTempMode(t *testing.T) {
	// A stale <path>.tmp left by a prior crash must not leak its (looser)
	// permissions into the published authorized_keys: open(2) only honours
	// the mode when creating, so writeSync chmods explicitly.
	m, dir := newTestManager(t)
	akPath := filepath.Join(dir, ".ssh", "authorized_keys")
	if err := os.MkdirAll(filepath.Dir(akPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(akPath+".tmp", []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.AddKey(keyA, "alice"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(akPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("authorized_keys mode = %o after stale-tmp write, want 600", fi.Mode().Perm())
	}
}

func TestRemoveDropsOne(t *testing.T) {
	m, dir := newTestManager(t)
	ka, _ := m.AddKey(keyA, "alice")
	if _, err := m.AddKey(keyB, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveKey(ka.Fingerprint); err != nil {
		t.Fatal(err)
	}
	if len(mustKeys(t, m)) != 1 {
		t.Fatalf("remove failed")
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".ssh", "authorized_keys"))
	if strings.Contains(string(b), "alice") {
		t.Errorf("removed key still in authorized_keys: %q", string(b))
	}
	// removing an absent key errors.
	if err := m.RemoveKey(ka.Fingerprint); err == nil {
		t.Error("expected error removing absent key")
	}
}

func TestRemoveLastKeyRefused(t *testing.T) {
	m, dir := newTestManager(t)
	ka, _ := m.AddKey(keyA, "alice")
	if err := m.RemoveKey(ka.Fingerprint); err == nil {
		t.Fatal("expected refusal removing the only key")
	}
	// the key must still be present — the refusal is the lockout guard.
	if len(mustKeys(t, m)) != 1 {
		t.Error("the only key was removed despite the guard")
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".ssh", "authorized_keys"))
	if !strings.Contains(string(b), "alice") {
		t.Errorf("only-key guard truncated the file: %q", string(b))
	}
}

func TestBootEnsuresDirAndHostKey(t *testing.T) {
	m, dir := newTestManager(t)
	if err := m.Boot(); err != nil {
		t.Fatal(err)
	}
	if di, err := os.Stat(filepath.Join(dir, ".ssh")); err != nil || di.Mode().Perm() != 0o700 {
		t.Errorf(".ssh not created 0700: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "hk")); err != nil {
		t.Errorf("dropbear host key not ensured: %v", err)
	}
}

func TestBootNoDropbearSkipsHostKey(t *testing.T) {
	dir := t.TempDir()
	prov := &Provider{
		AuthorizedKeysPath: filepath.Join(dir, ".ssh", "authorized_keys"),
		ManageDropbear:     false,
		DropbearKeyPath:    filepath.Join(dir, "hk"),
		dropbearKeyGen:     noopDropbear,
	}
	m := NewManager(prov)
	if err := m.Boot(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "hk")); !os.IsNotExist(err) {
		t.Errorf("host key was created despite manage-dropbear=false: %v", err)
	}
}

func TestHostProviderChowns(t *testing.T) {
	dir := t.TempDir()
	me, err := user.Current()
	if err != nil {
		t.Skip("no current user")
	}
	prov := &Provider{
		AuthorizedKeysPath: filepath.Join(dir, ".ssh", "authorized_keys"),
		ChownUser:          "ops",
		ManageDropbear:     false,
		lookupUser: func(name string) (*user.User, error) {
			return &user.User{Uid: me.Uid, Gid: me.Gid, HomeDir: dir, Username: name}, nil
		},
	}
	m := NewManager(prov)
	if _, err := m.AddKey(keyA, ""); err != nil {
		t.Fatalf("host AddKey: %v", err)
	}
	akPath := filepath.Join(dir, ".ssh", "authorized_keys")
	if _, err := os.Stat(akPath); err != nil {
		t.Errorf("host authorized_keys not written: %v", err)
	}
	// chown to the (current-uid) host user succeeds; a real uid mismatch
	// would surface as an error from writeKeys, which AddKey propagates.
	if di, err := os.Stat(filepath.Join(dir, ".ssh")); err != nil || di.Mode().Perm() != 0o700 {
		t.Errorf("host .ssh dir not 0700: err=%v", err)
	}
}

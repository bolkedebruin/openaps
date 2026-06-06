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

func TestManagerRejectsInjectedCommentAndNeverRenders(t *testing.T) {
	m, root := newTestManager(t)
	if _, err := m.AddKey(keyA, "ok\nssh-ed25519 AAAA injected"); err == nil {
		t.Fatal("expected AddKey to reject injected comment")
	}
	if len(m.List().SSHKeys) != 0 {
		t.Error("injected-comment key was persisted")
	}
	// The rendered authorized_keys must never gain the injected line. The
	// file may not exist at all (nothing rendered); if it does, it is empty.
	b, _ := os.ReadFile(filepath.Join(root, ".ssh", "authorized_keys"))
	if strings.Contains(string(b), "injected") {
		t.Errorf("injected line reached rendered file: %q", string(b))
	}
}

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "access.json")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Get().Provider != ProviderOpenAPS {
		t.Errorf("default provider = %q, want openaps", st.Get().Provider)
	}
	want := Access{
		Provider: ProviderHost,
		HostUser: "ops",
		SSHKeys:  []Key{{Pubkey: "x", Comment: "c", AddedMs: 42, Fingerprint: "SHA256:zz"}},
	}
	if err := st.Save(want); err != nil {
		t.Fatal(err)
	}
	// mode 0600
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("access.json mode = %o, want 600", fi.Mode().Perm())
	}
	// reopen and confirm round-trip
	st2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := st2.Get()
	if got.Provider != want.Provider || got.HostUser != want.HostUser || len(got.SSHKeys) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.SSHKeys[0].Fingerprint != "SHA256:zz" {
		t.Errorf("key fingerprint mismatch: %+v", got.SSHKeys[0])
	}
}

func TestStoreCloneIsolation(t *testing.T) {
	st, _ := Open(filepath.Join(t.TempDir(), "access.json"))
	_ = st.Save(Access{Provider: ProviderOpenAPS, SSHKeys: []Key{{Fingerprint: "a"}}})
	g := st.Get()
	g.SSHKeys[0].Fingerprint = "mutated"
	if st.Get().SSHKeys[0].Fingerprint != "a" {
		t.Error("Get() did not return an isolated copy")
	}
}

// noopDropbear avoids invoking the real dropbearkey binary in tests.
func noopDropbear(path string) error { return os.WriteFile(path, []byte("hostkey"), 0o600) }

func TestRenderOpenAPS(t *testing.T) {
	root := t.TempDir()
	dbkey := filepath.Join(t.TempDir(), "dropbear", "host_key")
	r := &Renderer{rootHome: root, DropbearKeyPath: dbkey, dropbearKeyGen: noopDropbear}

	ka, _ := ParseKey(keyA, "alice")
	kb, _ := ParseKey(keyB, "bob")
	if err := r.Render(Access{Provider: ProviderOpenAPS, SSHKeys: []Key{ka, kb}}); err != nil {
		t.Fatal(err)
	}

	akPath := filepath.Join(root, ".ssh", "authorized_keys")
	b, err := os.ReadFile(akPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), string(b))
	}
	if !strings.HasSuffix(lines[0], " alice") || !strings.HasSuffix(lines[1], " bob") {
		t.Errorf("comments not rendered: %q", lines)
	}
	// .ssh dir 0700, file 0600
	di, _ := os.Stat(filepath.Join(root, ".ssh"))
	if di.Mode().Perm() != 0o700 {
		t.Errorf(".ssh mode = %o, want 700", di.Mode().Perm())
	}
	fi, _ := os.Stat(akPath)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("authorized_keys mode = %o, want 600", fi.Mode().Perm())
	}
	// dropbear host key ensured
	if _, err := os.Stat(dbkey); err != nil {
		t.Errorf("dropbear host key not created: %v", err)
	}
}

func TestRenderIsFullRewrite(t *testing.T) {
	root := t.TempDir()
	r := &Renderer{rootHome: root, dropbearKeyGen: noopDropbear, DropbearKeyPath: filepath.Join(t.TempDir(), "hk")}
	ka, _ := ParseKey(keyA, "")
	kb, _ := ParseKey(keyB, "")
	if err := r.Render(Access{Provider: ProviderOpenAPS, SSHKeys: []Key{ka, kb}}); err != nil {
		t.Fatal(err)
	}
	// now render with only one key — file must shrink (not append).
	if err := r.Render(Access{Provider: ProviderOpenAPS, SSHKeys: []Key{kb}}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, ".ssh", "authorized_keys"))
	if strings.Contains(string(b), "AAAAIAG3oZkfNp0gi") {
		t.Errorf("removed key still present after re-render: %q", string(b))
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("want 1 line after rewrite, got %d", len(lines))
	}
}

func TestRenderOffRevokesRenderedKeys(t *testing.T) {
	root := t.TempDir()
	r := &Renderer{rootHome: root, dropbearKeyGen: noopDropbear, DropbearKeyPath: filepath.Join(t.TempDir(), "hk")}
	// First render keys under openaps, then flip to off.
	ka, _ := ParseKey(keyA, "alice")
	if err := r.Render(Access{Provider: ProviderOpenAPS, SSHKeys: []Key{ka}}); err != nil {
		t.Fatal(err)
	}
	akPath := filepath.Join(root, ".ssh", "authorized_keys")
	if !fileNonEmpty(akPath) {
		t.Fatalf("openaps render did not write keys")
	}
	if err := r.Render(Access{Provider: ProviderOff}); err != nil {
		t.Fatalf("off render: %v", err)
	}
	// off must truncate the previously-rendered authorized_keys, not leave
	// the stale root keys honored by dropbear.
	if fileNonEmpty(akPath) {
		b, _ := os.ReadFile(akPath)
		t.Errorf("off did not revoke rendered keys: %q", string(b))
	}
}

func TestRenderOpenAPSRefusesToEmptyExisting(t *testing.T) {
	root := t.TempDir()
	r := &Renderer{rootHome: root, dropbearKeyGen: noopDropbear, DropbearKeyPath: filepath.Join(t.TempDir(), "hk")}
	ka, _ := ParseKey(keyA, "alice")
	if err := r.Render(Access{Provider: ProviderOpenAPS, SSHKeys: []Key{ka}}); err != nil {
		t.Fatal(err)
	}
	akPath := filepath.Join(root, ".ssh", "authorized_keys")
	// Now render zero keys under openaps (simulating a blanked access.json);
	// the existing non-empty file must be preserved, not truncated.
	if err := r.Render(Access{Provider: ProviderOpenAPS, SSHKeys: nil}); err != nil {
		t.Fatal(err)
	}
	if !fileNonEmpty(akPath) {
		t.Errorf("openaps with zero keys truncated a non-empty authorized_keys")
	}
}

func TestRenderHostUsesUserHome(t *testing.T) {
	home := t.TempDir()
	me, err := user.Current()
	if err != nil {
		t.Skip("no current user")
	}
	r := &Renderer{
		lookupUser: func(name string) (*user.User, error) {
			return &user.User{Uid: me.Uid, Gid: me.Gid, HomeDir: home, Username: name}, nil
		},
	}
	ka, _ := ParseKey(keyA, "")
	if err := r.Render(Access{Provider: ProviderHost, HostUser: "ops", SSHKeys: []Key{ka}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "authorized_keys")); err != nil {
		t.Errorf("host authorized_keys not written: %v", err)
	}
	// host provider must NOT manage a dropbear host key — the render
	// succeeds with no DropbearKeyPath set, which proves it isn't touched.
}

func TestRenderHostRequiresUser(t *testing.T) {
	r := &Renderer{}
	if err := r.Render(Access{Provider: ProviderHost}); err == nil {
		t.Error("host provider without host_user should error")
	}
}

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "access.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := &Renderer{rootHome: dir, dropbearKeyGen: noopDropbear, DropbearKeyPath: filepath.Join(dir, "hk")}
	return NewManager(st, r), dir
}

func TestManagerAddDedupe(t *testing.T) {
	m, _ := newTestManager(t)
	k1, err := m.AddKey(keyA, "alice")
	if err != nil {
		t.Fatal(err)
	}
	// adding the same key (even with a different comment line) is a no-op.
	if _, err := m.AddKey(keyA, "alice-again"); err != nil {
		t.Fatal(err)
	}
	if got := len(m.List().SSHKeys); got != 1 {
		t.Fatalf("dedupe failed: %d keys", got)
	}
	if k1.Fingerprint != fpOf(t, keyA) {
		t.Errorf("fingerprint mismatch")
	}
}

func TestManagerAddRejectsMalformed(t *testing.T) {
	m, _ := newTestManager(t)
	if _, err := m.AddKey("garbage", ""); err == nil {
		t.Error("expected error adding malformed key")
	}
	if len(m.List().SSHKeys) != 0 {
		t.Error("malformed key was persisted")
	}
}

func TestManagerRemove(t *testing.T) {
	m, root := newTestManager(t)
	ka, _ := m.AddKey(keyA, "alice")
	if _, err := m.AddKey(keyB, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveKey(ka.Fingerprint); err != nil {
		t.Fatal(err)
	}
	if len(m.List().SSHKeys) != 1 {
		t.Fatalf("remove failed")
	}
	// authorized_keys re-rendered without the removed key.
	b, _ := os.ReadFile(filepath.Join(root, ".ssh", "authorized_keys"))
	if strings.Contains(string(b), "alice") {
		t.Errorf("removed key still in authorized_keys: %q", string(b))
	}
	// removing again errors (absent).
	if err := m.RemoveKey(ka.Fingerprint); err == nil {
		t.Error("expected error removing absent key")
	}
}

func TestManagerBootRenderFromPersisted(t *testing.T) {
	dir := t.TempDir()
	st, _ := Open(filepath.Join(dir, "access.json"))
	ka, _ := ParseKey(keyA, "alice")
	_ = st.Save(Access{Provider: ProviderOpenAPS, SSHKeys: []Key{ka}})

	// fresh manager (simulating a reboot) must render from disk.
	st2, _ := Open(filepath.Join(dir, "access.json"))
	r := &Renderer{rootHome: dir, dropbearKeyGen: noopDropbear, DropbearKeyPath: filepath.Join(dir, "hk")}
	m := NewManager(st2, r)
	if err := m.BootRender(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".ssh", "authorized_keys"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "alice") {
		t.Errorf("boot-render did not write persisted key: %q", string(b))
	}
}

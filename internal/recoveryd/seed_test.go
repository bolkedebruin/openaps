package recoveryd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeedMergesAndDedupes(t *testing.T) {
	dir := t.TempDir()
	akFile := filepath.Join(dir, "authorized_keys")
	// two distinct keys plus a comment and a blank line, and keyA again.
	content := "# operator bundle\n" + keyA + "\n\n" + keyB + "\n" + keyA + "\n"
	if err := os.WriteFile(akFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	st, _ := Open(filepath.Join(dir, "access.json"))
	n, err := Seed(st, ProviderOpenAPS, "", akFile)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("seed added %d, want 2 (deduped)", n)
	}
	a := st.Get()
	if a.Provider != ProviderOpenAPS || len(a.SSHKeys) != 2 {
		t.Fatalf("unexpected state after seed: %+v", a)
	}
	for _, k := range a.SSHKeys {
		if k.Fingerprint == "" {
			t.Errorf("seeded key missing fingerprint: %+v", k)
		}
	}

	// re-seeding the same file is idempotent (0 added).
	n2, err := Seed(st, ProviderOpenAPS, "", akFile)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Errorf("re-seed added %d, want 0", n2)
	}
}

func TestSeedRejectsMalformed(t *testing.T) {
	dir := t.TempDir()
	akFile := filepath.Join(dir, "bad")
	_ = os.WriteFile(akFile, []byte("not a real key\n"), 0o600)
	st, _ := Open(filepath.Join(dir, "access.json"))
	if _, err := Seed(st, ProviderOpenAPS, "", akFile); err == nil {
		t.Error("expected error seeding malformed key")
	}
}

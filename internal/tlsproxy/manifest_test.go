package tlsproxy

import (
	"bytes"
	"compress/gzip"
	"testing"
)

// gz gzips b for feeding parsePackages's compressed path.
func gz(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

const sampleIndex = `Package: openaps-inv-driver
Version: 1.0.0
Filename: openaps-inv-driver_1.0.0_arm.ipk
SHA256sum: ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789

Package: openaps-recoveryd
Version: 1.0.0
Filename: openaps-recoveryd_1.0.0_arm.ipk
SHA256sum: 0011223344556677889900112233445566778899001122334455667788990011
`

func TestParsePackagesPlain(t *testing.T) {
	m, err := parsePackages([]byte(sampleIndex))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	d, ok := m.digestFor("openaps-inv-driver_1.0.0_arm.ipk")
	if !ok {
		t.Fatal("expected first package present")
	}
	// SHA256sum must be lowercased.
	if d != "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789" {
		t.Fatalf("digest not lowercased: %q", d)
	}
	if _, ok := m.digestFor("openaps-recoveryd_1.0.0_arm.ipk"); !ok {
		t.Fatal("expected second package present")
	}
}

func TestParsePackagesGzip(t *testing.T) {
	m, err := parsePackages(gz(t, []byte(sampleIndex)))
	if err != nil {
		t.Fatalf("parse gzipped: %v", err)
	}
	if _, ok := m.digestFor("openaps-inv-driver_1.0.0_arm.ipk"); !ok {
		t.Fatal("expected package present after gunzip")
	}
}

func TestDigestForMiss(t *testing.T) {
	m, err := parsePackages([]byte(sampleIndex))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := m.digestFor("not-a-real-package.ipk"); ok {
		t.Fatal("expected miss for unlisted filename")
	}
}

func TestParsePackagesRejectsMissingFilename(t *testing.T) {
	idx := `Package: openaps-x
Version: 1.0.0
SHA256sum: 0011223344556677889900112233445566778899001122334455667788990011
`
	if _, err := parsePackages([]byte(idx)); err == nil {
		t.Fatal("expected rejection of stanza missing Filename")
	}
}

func TestParsePackagesRejectsMissingSHA(t *testing.T) {
	idx := `Package: openaps-x
Version: 1.0.0
Filename: openaps-x_1.0.0_arm.ipk
`
	if _, err := parsePackages([]byte(idx)); err == nil {
		t.Fatal("expected rejection of stanza missing SHA256sum")
	}
}

func TestParsePackagesRejectsEmpty(t *testing.T) {
	if _, err := parsePackages(nil); err == nil {
		t.Fatal("expected rejection of empty index")
	}
	if _, err := parsePackages([]byte("\n\n   \n")); err == nil {
		t.Fatal("expected rejection of whitespace-only index")
	}
}

// A missing-field stanza must be rejected even when valid stanzas precede it,
// so a single unpinnable package fails the whole index closed.
func TestParsePackagesRejectsMixedMissingField(t *testing.T) {
	idx := sampleIndex + `
Package: openaps-broken
Version: 2.0.0
Filename: openaps-broken_2.0.0_arm.ipk
`
	if _, err := parsePackages([]byte(idx)); err == nil {
		t.Fatal("expected rejection when any stanza lacks SHA256sum")
	}
}

// TestParseNormalizesDotSlashFilename guards the fix for "./"-prefixed feed
// Filename fields: opkg requests the bare path, so the stored key must match.
func TestParseNormalizesDotSlashFilename(t *testing.T) {
	idx := "Package: openaps-inv-driver\n" +
		"Filename: ./openaps-inv-driver_1.0_arm.ipk\n" +
		"SHA256sum: ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789\n\n"
	m, err := parsePackages([]byte(idx))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	d, ok := m.digestFor("openaps-inv-driver_1.0_arm.ipk")
	if !ok {
		t.Fatal("bare request path did not match a ./-prefixed Filename")
	}
	if d != "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789" {
		t.Fatalf("digest not lowercased: %s", d)
	}
}

// TestParseRejectsTraversalFilename ensures a feed cannot pin a digest under a
// path that escapes the feed root.
func TestParseRejectsTraversalFilename(t *testing.T) {
	idx := "Package: evil\nFilename: ../../etc/passwd\n" +
		"SHA256sum: ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789\n\n"
	if _, err := parsePackages([]byte(idx)); err == nil {
		t.Fatal("expected rejection of traversal Filename")
	}
}

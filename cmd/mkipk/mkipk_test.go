package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// arMember is a parsed member of an ar archive.
type arMember struct {
	name string
	body []byte
}

// parseAr parses a "common" ar archive into ordered members.
func parseAr(t *testing.T, b []byte) []arMember {
	t.Helper()
	if !bytes.HasPrefix(b, []byte("!<arch>\n")) {
		t.Fatalf("missing ar magic")
	}
	p := b[8:]
	var members []arMember
	for len(p) >= 60 {
		hdr := p[:60]
		if hdr[58] != '`' || hdr[59] != '\n' {
			t.Fatalf("bad ar header terminator: %q", hdr[58:60])
		}
		name := string(bytes.TrimRight(hdr[0:16], " "))
		var size int
		_, err := fmtSscan(string(bytes.TrimRight(hdr[48:58], " ")), &size)
		if err != nil {
			t.Fatalf("bad size field: %v", err)
		}
		p = p[60:]
		if len(p) < size {
			t.Fatalf("truncated member %q", name)
		}
		members = append(members, arMember{name: name, body: append([]byte(nil), p[:size]...)})
		p = p[size:]
		if size%2 == 1 {
			if len(p) == 0 || p[0] != '\n' {
				t.Fatalf("missing pad byte after %q", name)
			}
			p = p[1:]
		}
	}
	return members
}

func fmtSscan(s string, out *int) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	*out = n
	return 1, nil
}

// readTarGz unpacks a gzip+tar into name->entry maps.
func readTarGz(t *testing.T, b []byte) (map[string][]byte, map[string]byte) {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	tr := tar.NewReader(gz)
	contents := map[string][]byte{}
	types := map[string]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		types[hdr.Name] = hdr.Typeflag
		body, _ := io.ReadAll(tr)
		contents[hdr.Name] = body
	}
	return contents, types
}

func TestBuildIpk(t *testing.T) {
	dir := t.TempDir()

	// Control stanza.
	controlFile := filepath.Join(dir, "control")
	if err := os.WriteFile(controlFile, []byte("Package: openaps-test\nVersion: 1.0\nArchitecture: all\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Maintainer scripts.
	scriptsDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "postinst"), []byte("#!/bin/sh\nexit 0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "conffiles"), []byte("/etc/openaps/x.conf\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Data tree: a nested leaf file.
	dataDir := filepath.Join(dir, "data")
	leafDir := filepath.Join(dataDir, "usr", "bin")
	if err := os.MkdirAll(leafDir, 0755); err != nil {
		t.Fatal(err)
	}
	leafBody := []byte("ELFPLACEHOLDER")
	if err := os.WriteFile(filepath.Join(leafDir, "openaps-test"), leafBody, 0755); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(dir, "openaps-test_1.0_all.ipk")
	if err := build(controlFile, dataDir, scriptsDir, outFile); err != nil {
		t.Fatalf("build: %v", err)
	}

	raw, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}

	// 1. ar members present and in order.
	members := parseAr(t, raw)
	if len(members) != 3 {
		t.Fatalf("want 3 ar members, got %d", len(members))
	}
	wantOrder := []string{"debian-binary", "control.tar.gz", "data.tar.gz"}
	for i, m := range members {
		if m.name != wantOrder[i] {
			t.Fatalf("member %d = %q, want %q", i, m.name, wantOrder[i])
		}
	}
	if string(members[0].body) != "2.0\n" {
		t.Fatalf("debian-binary = %q, want %q", members[0].body, "2.0\n")
	}

	// 2. control.tar.gz contains ./control + ./postinst (0755) + ./conffiles.
	cContents, cTypes := readTarGz(t, members[1].body)
	if _, ok := cContents["./control"]; !ok {
		t.Fatalf("control.tar.gz missing ./control; have %v", keys(cContents))
	}
	if _, ok := cContents["./postinst"]; !ok {
		t.Fatalf("control.tar.gz missing ./postinst")
	}
	if _, ok := cContents["./conffiles"]; !ok {
		t.Fatalf("control.tar.gz missing ./conffiles")
	}
	_ = cTypes

	// 3. data.tar.gz has explicit dir entries for every ancestor + the leaf.
	dContents, dTypes := readTarGz(t, members[2].body)
	for _, dir := range []string{"./usr/", "./usr/bin/"} {
		if dTypes[dir] != tar.TypeDir {
			t.Fatalf("data.tar.gz missing TypeDir entry %q; have %v", dir, keys(dTypes))
		}
	}
	leaf := "./usr/bin/openaps-test"
	if dTypes[leaf] != tar.TypeReg {
		t.Fatalf("data.tar.gz missing leaf %q", leaf)
	}
	if !bytes.Equal(dContents[leaf], leafBody) {
		t.Fatalf("leaf body = %q, want %q", dContents[leaf], leafBody)
	}
	// Dir entries must precede the leaf so busybox tar can mkdir first.
	assertOrderTarGz(t, members[2].body, "./usr/", "./usr/bin/", leaf)

	// 4. Round-trip: rebuilding the same inputs yields identical bytes.
	outFile2 := filepath.Join(dir, "again.ipk")
	if err := build(controlFile, dataDir, scriptsDir, outFile2); err != nil {
		t.Fatal(err)
	}
	raw2, err := os.ReadFile(outFile2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, raw2) {
		t.Fatalf("build is not reproducible: byte mismatch")
	}
}

func TestBuildIpkNoScripts(t *testing.T) {
	dir := t.TempDir()
	controlFile := filepath.Join(dir, "control")
	if err := os.WriteFile(controlFile, []byte("Package: x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "etc"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "etc", "f"), []byte("z"), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "x.ipk")
	if err := build(controlFile, dataDir, "", out); err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, _ := os.ReadFile(out)
	members := parseAr(t, raw)
	if len(members) != 3 {
		t.Fatalf("want 3 members, got %d", len(members))
	}
	cContents, _ := readTarGz(t, members[1].body)
	if _, ok := cContents["./control"]; !ok {
		t.Fatalf("missing ./control")
	}
	if len(cContents) != 1 {
		t.Fatalf("control.tar.gz should hold only ./control, got %v", keys(cContents))
	}
}

// assertOrderTarGz checks the named entries appear in the given relative order.
func assertOrderTarGz(t *testing.T, b []byte, names ...string) {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var seq []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seq = append(seq, hdr.Name)
	}
	idx := func(name string) int {
		for i, s := range seq {
			if s == name {
				return i
			}
		}
		return -1
	}
	prev := -1
	for _, n := range names {
		i := idx(n)
		if i < 0 {
			t.Fatalf("entry %q not found in %v", n, seq)
		}
		if i < prev {
			t.Fatalf("entry %q out of order in %v", n, seq)
		}
		prev = i
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

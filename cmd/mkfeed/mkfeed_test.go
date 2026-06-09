package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"testing"
)

// buildIpk assembles a minimal .ipk (ar of debian-binary, control.tar.gz,
// data.tar.gz) for exercising the readers and fingerprint.
func buildIpk(t *testing.T, control string, postinst string, dataFiles map[string]string) []byte {
	t.Helper()
	ctlFiles := map[string]string{"./control": control}
	if postinst != "" {
		ctlFiles["./postinst"] = postinst
	}
	members := []struct {
		name string
		data []byte
	}{
		{"debian-binary", []byte("2.0\n")},
		{"control.tar.gz", tgz(t, ctlFiles)},
		{"data.tar.gz", tgz(t, dataFiles)},
	}
	var b bytes.Buffer
	b.WriteString("!<arch>\n")
	for _, m := range members {
		fmt.Fprintf(&b, "%-16s%-12d%-6d%-6d%-8s%-10d`\n", m.name, 0, 0, 0, "100644", len(m.data))
		b.Write(m.data)
		if len(m.data)%2 == 1 {
			b.WriteByte('\n')
		}
	}
	return b.Bytes()
}

func tgz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	tw.Close()
	zw.Close()
	return buf.Bytes()
}

func TestArMembersAndControl(t *testing.T) {
	ctl := "Package: openaps-base\nVersion: 1.0.0\nArchitecture: all\n"
	ipk := buildIpk(t, ctl, "#!/bin/sh\nexit 0\n", map[string]string{"./usr/x": "y"})
	control, dataGz, ctlGz, err := readIpkBytes(ipk)
	if err != nil {
		t.Fatalf("readIpkBytes: %v", err)
	}
	if field(control, "Package") != "openaps-base" || field(control, "Architecture") != "all" {
		t.Fatalf("bad control fields: %q", control)
	}
	if len(dataGz) == 0 || len(ctlGz) == 0 {
		t.Fatal("empty data/control gz")
	}
	if _, ok := tarMember(ctlGz, "./postinst"); !ok {
		t.Fatal("postinst not found in control.tar.gz")
	}
}

func TestFingerprintIgnoresVersionButTracksPayload(t *testing.T) {
	mk := func(version, payload, postinst string) string {
		ctl := "Package: p\nVersion: " + version + "\nArchitecture: all\nDepends: openaps-base\n"
		ipk := buildIpk(t, ctl, postinst, map[string]string{"./usr/x": payload})
		c, d, cg, err := readIpkBytes(ipk)
		if err != nil {
			t.Fatal(err)
		}
		return fingerprint(c, cg, d)
	}
	base := mk("1.0.0", "hello", "#post1")
	// Same payload + scripts, only the version differs -> same fingerprint.
	if got := mk("9.9.9", "hello", "#post1"); got != base {
		t.Fatalf("version bump changed fingerprint: %s vs %s", base, got)
	}
	// Different data payload -> different fingerprint.
	if got := mk("1.0.0", "HELLO", "#post1"); got == base {
		t.Fatal("payload change did not change fingerprint")
	}
	// Different postinst -> different fingerprint.
	if got := mk("1.0.0", "hello", "#post2"); got == base {
		t.Fatal("postinst change did not change fingerprint")
	}
}

func TestCanonicalControlDropsVersion(t *testing.T) {
	c := canonicalControl([]byte("Package: p\nVersion: 1.2.3\nArchitecture: all\n"))
	if bytes.Contains(c, []byte("1.2.3")) || bytes.Contains(c, []byte("Version")) {
		t.Fatalf("version not stripped: %q", c)
	}
}

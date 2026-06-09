// Command mkfeed assembles an opkg feed with content-hash "only changed"
// republishing. It compares each freshly built .ipk against the previously
// published feed (via a versions.json manifest): a package whose payload is
// unchanged keeps its published .ipk and version, so opkg sees no upgrade for
// it; a changed or new package is republished at the release version. It then
// writes the merged feed directory, the Packages index, and the updated
// versions.json. Signing the gzipped index is left to the caller.
//
// The "fingerprint" deliberately excludes the Version field, so a release that
// only bumps the tag does not look like a change. Builds are reproducible
// (mkipk zeroes mtimes), so identical payloads hash identically across runs.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type pkgEntry struct {
	Version     string `json:"version"`
	Fingerprint string `json:"fingerprint"`
	Filename    string `json:"filename"`
}

type manifest struct {
	Packages map[string]pkgEntry `json:"packages"`
}

func main() {
	newDir := flag.String("new", "", "directory of freshly built .ipk files (required)")
	prevDir := flag.String("prev", "", "previously published feed directory (may be empty/absent on first run)")
	outDir := flag.String("out", "", "output feed directory to assemble (required)")
	version := flag.String("version", "", "release version stamped onto changed/new packages (required)")
	flag.Parse()
	if *newDir == "" || *outDir == "" || *version == "" {
		fatal("flags -new, -out and -version are required")
	}

	prev := loadManifest(*prevDir)
	out := &manifest{Packages: map[string]pkgEntry{}}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal("mkdir out: %v", err)
	}

	built, err := filepath.Glob(filepath.Join(*newDir, "*.ipk"))
	if err != nil || len(built) == 0 {
		fatal("no .ipk files in %s", *newDir)
	}
	sort.Strings(built)

	var changed, kept []string
	seen := map[string]string{}
	for _, ipk := range built {
		ctl, dataGz, ctlGz, err := readIpk(ipk)
		if err != nil {
			fatal("%s: %v", ipk, err)
		}
		name := field(ctl, "Package")
		arch := field(ctl, "Architecture")
		if name == "" || arch == "" {
			fatal("%s: control missing Package/Architecture", ipk)
		}
		// The -new dir must hold exactly one freshly built .ipk per package. A
		// duplicate means a stale build (e.g. an old version left in the dir);
		// fail loudly rather than silently let the last one win the feed.
		if other, dup := seen[name]; dup {
			fatal("package %q appears twice in -new: %s and %s — clean the build dir", name, filepath.Base(other), filepath.Base(ipk))
		}
		seen[name] = ipk
		fp := fingerprint(ctl, ctlGz, dataGz)

		if p, ok := prev.Packages[name]; ok && p.Fingerprint == fp {
			// Unchanged: keep the already-published .ipk and its version.
			src := filepath.Join(*prevDir, p.Filename)
			if err := copyFile(src, filepath.Join(*outDir, p.Filename)); err != nil {
				fatal("keep %s: %v", name, err)
			}
			out.Packages[name] = p
			kept = append(kept, fmt.Sprintf("%s=%s", name, p.Version))
			continue
		}
		// Changed or new: publish the freshly built .ipk at the release version.
		filename := fmt.Sprintf("%s_%s_%s.ipk", name, *version, arch)
		if err := copyFile(ipk, filepath.Join(*outDir, filename)); err != nil {
			fatal("publish %s: %v", name, err)
		}
		out.Packages[name] = pkgEntry{Version: *version, Fingerprint: fp, Filename: filename}
		changed = append(changed, name)
	}

	writePackages(*outDir, out)
	writeManifest(*outDir, out)

	sort.Strings(changed)
	sort.Strings(kept)
	fmt.Printf("feed assembled in %s\n  changed/new (%d): %s\n  kept     (%d): %s\n",
		*outDir, len(changed), strings.Join(changed, " "), len(kept), strings.Join(kept, " "))
}

// fingerprint hashes the package's payload and metadata EXCEPT its Version, so
// a version-only bump is not treated as a change.
func fingerprint(control, controlGz, dataGz []byte) string {
	h := sha256.New()
	h.Write(canonicalControl(control))
	// Maintainer scripts live in control.tar.gz alongside ./control; fold the
	// whole control.tar.gz in but with the version-bearing ./control replaced
	// by its canonical (version-stripped) form already hashed above. Simplest
	// stable proxy: hash the postinst/conffiles members explicitly.
	for _, member := range []string{"./postinst", "./preinst", "./prerm", "./postrm", "./conffiles"} {
		if b, ok := tarMember(controlGz, member); ok {
			h.Write([]byte(member))
			h.Write(b)
		}
	}
	h.Write(dataGz)
	return hex.EncodeToString(h.Sum(nil))
}

// canonicalControl returns the control stanza with the Version line removed and
// the remaining fields sorted, so field reordering or a version bump alone does
// not change the fingerprint.
func canonicalControl(control []byte) []byte {
	var lines []string
	for _, ln := range strings.Split(string(control), "\n") {
		if strings.HasPrefix(strings.ToLower(ln), "version:") {
			continue
		}
		if strings.TrimSpace(ln) == "" {
			continue
		}
		lines = append(lines, ln)
	}
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n"))
}

// writePackages emits the opkg Packages index over every .ipk in the feed dir.
func writePackages(dir string, m *manifest) {
	names := make([]string, 0, len(m.Packages))
	for n := range m.Packages {
		names = append(names, n)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	for _, name := range names {
		p := m.Packages[name]
		path := filepath.Join(dir, p.Filename)
		raw, err := os.ReadFile(path)
		if err != nil {
			fatal("index %s: %v", p.Filename, err)
		}
		ctl, _, _, err := readIpkBytes(raw)
		if err != nil {
			fatal("index %s: %v", p.Filename, err)
		}
		md := md5.Sum(raw)
		sum := sha256.Sum256(raw)
		// Take the control stanza, drop any Filename/Size/checksum lines, append
		// the computed ones.
		for _, ln := range strings.Split(strings.TrimRight(string(ctl), "\n"), "\n") {
			switch strings.ToLower(strings.SplitN(ln, ":", 2)[0]) {
			case "filename", "size", "md5sum", "sha256sum":
				continue
			}
			if strings.TrimSpace(ln) != "" {
				buf.WriteString(ln + "\n")
			}
		}
		fmt.Fprintf(&buf, "Filename: %s\n", p.Filename)
		fmt.Fprintf(&buf, "Size: %d\n", len(raw))
		fmt.Fprintf(&buf, "MD5Sum: %s\n", hex.EncodeToString(md[:]))
		fmt.Fprintf(&buf, "SHA256sum: %s\n\n", hex.EncodeToString(sum[:]))
	}
	if err := os.WriteFile(filepath.Join(dir, "Packages"), buf.Bytes(), 0o644); err != nil {
		fatal("write Packages: %v", err)
	}
}

func writeManifest(dir string, m *manifest) {
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "versions.json"), append(b, '\n'), 0o644); err != nil {
		fatal("write versions.json: %v", err)
	}
}

func loadManifest(dir string) *manifest {
	m := &manifest{Packages: map[string]pkgEntry{}}
	if dir == "" {
		return m
	}
	b, err := os.ReadFile(filepath.Join(dir, "versions.json"))
	if err != nil {
		return m // first run / no prior feed
	}
	if err := json.Unmarshal(b, m); err != nil {
		fatal("parse prev versions.json: %v", err)
	}
	if m.Packages == nil {
		m.Packages = map[string]pkgEntry{}
	}
	return m
}

// ---- .ipk (ar) reading ----

func readIpk(path string) (control, dataGz, controlGz []byte, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}
	return readIpkBytes(raw)
}

func readIpkBytes(raw []byte) (control, dataGz, controlGz []byte, err error) {
	members, err := arMembers(raw)
	if err != nil {
		return nil, nil, nil, err
	}
	controlGz = members["control.tar.gz"]
	dataGz = members["data.tar.gz"]
	if controlGz == nil || dataGz == nil {
		return nil, nil, nil, fmt.Errorf("ipk missing control.tar.gz or data.tar.gz")
	}
	control, ok := tarMember(controlGz, "./control")
	if !ok {
		if control, ok = tarMember(controlGz, "control"); !ok {
			return nil, nil, nil, fmt.Errorf("control.tar.gz has no ./control")
		}
	}
	return control, dataGz, controlGz, nil
}

// arMembers parses a (BSD/GNU-common) ar archive into name->data.
func arMembers(raw []byte) (map[string][]byte, error) {
	const magic = "!<arch>\n"
	if len(raw) < len(magic) || string(raw[:len(magic)]) != magic {
		return nil, fmt.Errorf("not an ar archive")
	}
	out := map[string][]byte{}
	off := len(magic)
	for off+60 <= len(raw) {
		hdr := raw[off : off+60]
		name := strings.TrimRight(string(hdr[0:16]), " ")
		name = strings.TrimSuffix(name, "/") // GNU appends '/'
		var size int
		if _, err := fmt.Sscanf(strings.TrimSpace(string(hdr[48:58])), "%d", &size); err != nil {
			return nil, fmt.Errorf("ar header size: %w", err)
		}
		start := off + 60
		if start+size > len(raw) {
			return nil, fmt.Errorf("ar member %q overruns archive", name)
		}
		out[name] = raw[start : start+size]
		off = start + size
		if size%2 == 1 {
			off++ // members are 2-byte aligned
		}
	}
	return out, nil
}

// tarMember returns the contents of a member in a gzipped tar, by name.
func tarMember(gz []byte, name string) ([]byte, bool) {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, false
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, false
		}
		if h.Name == name {
			b, err := io.ReadAll(tr)
			if err != nil {
				return nil, false
			}
			return b, true
		}
	}
}

func field(control []byte, key string) string {
	for _, ln := range strings.Split(string(control), "\n") {
		k, v, ok := strings.Cut(ln, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), key) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "mkfeed: "+format+"\n", a...)
	os.Exit(1)
}

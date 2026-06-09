package tlsproxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"strings"
)

// normalizeFilename canonicalizes a feed-relative path so the digest stored
// from a Packages "Filename:" field and the path opkg later requests compare
// equal. opkg/dpkg indices commonly emit "./pkg.ipk" while opkg then requests
// "/pkg.ipk"; stripping the "./" and cleaning both sides keys them the same.
// It returns ("", false) for anything that escapes the feed root.
func normalizeFilename(p string) (string, bool) {
	p = path.Clean(strings.TrimPrefix(strings.TrimSpace(p), "./"))
	if p == "" || p == "." || p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") {
		return "", false
	}
	return p, true
}

// manifest maps each package's feed-relative Filename to its lowercase hex
// SHA-256, parsed from a verified Packages index. It is the hash chain that
// lets the proxy authenticate individual .ipk downloads: the index itself is
// signed with the release key, and every .ipk is checked against the digest
// the signed index pins for it.
type manifest struct {
	bySHA256 map[string]string // Filename -> sha256 hex
}

// parsePackages reads an opkg/dpkg Packages index (optionally gzipped) and
// extracts the Filename and SHA256sum of each stanza. Stanzas are separated
// by blank lines; fields are "Key: value". A stanza missing either field is
// rejected, because an unpinnable package must never be served.
func parsePackages(raw []byte) (*manifest, error) {
	data := raw
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("tlsproxy: gunzip Packages: %w", err)
		}
		data, err = io.ReadAll(io.LimitReader(zr, maxIndexBytes))
		if err != nil {
			return nil, fmt.Errorf("tlsproxy: read Packages: %w", err)
		}
	}

	m := &manifest{bySHA256: make(map[string]string)}
	var filename, sha string
	flush := func() error {
		if filename == "" && sha == "" {
			return nil // blank trailing stanza
		}
		if filename == "" || sha == "" {
			return fmt.Errorf("tlsproxy: Packages stanza missing Filename or SHA256sum (filename=%q)", filename)
		}
		key, ok := normalizeFilename(filename)
		if !ok {
			return fmt.Errorf("tlsproxy: Packages stanza has illegal Filename %q", filename)
		}
		m.bySHA256[key] = strings.ToLower(sha)
		filename, sha = "", ""
		return nil
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue // continuation or malformed line; not a field we need
		}
		val = strings.TrimSpace(val)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "filename":
			filename = val
		case "sha256sum":
			sha = val
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("tlsproxy: scan Packages: %w", err)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(m.bySHA256) == 0 {
		return nil, fmt.Errorf("tlsproxy: Packages index has no packages")
	}
	return m, nil
}

// digestFor returns the pinned SHA-256 hex for a feed-relative filename and
// whether it is present in the index. The lookup is normalized the same way
// the keys were stored, so a "./"-prefixed index entry matches the bare path
// opkg requests.
func (m *manifest) digestFor(filename string) (string, bool) {
	key, ok := normalizeFilename(filename)
	if !ok {
		return "", false
	}
	d, ok := m.bySHA256[key]
	return d, ok
}

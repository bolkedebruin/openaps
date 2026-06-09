// Command mkipk builds an opkg-installable .ipk package.
//
// An .ipk is an ar archive holding exactly three members, in order:
//
//	debian-binary    — the bytes "2.0\n"
//	control.tar.gz   — ./control (+ optional postinst/preinst/prerm/postrm/conffiles)
//	data.tar.gz      — the staged file tree, ./usr/... style, with explicit
//	                   directory entries for every ancestor
//
// The build is reproducible: all tar/ar timestamps and ownership are zeroed.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	control := flag.String("control", "", "path to the control stanza file (required)")
	data := flag.String("data", "", "staged data directory tree -> data.tar.gz (required)")
	scripts := flag.String("scripts", "", "optional dir of maintainer scripts/conffiles for control.tar.gz")
	out := flag.String("out", "", "output .ipk path (required)")
	flag.Parse()

	if *control == "" || *data == "" || *out == "" {
		flag.Usage()
		fmt.Fprintln(os.Stderr, "\nmkipk -control <file> -data <dir> [-scripts <dir>] -out <file.ipk>")
		os.Exit(2)
	}

	if err := build(*control, *data, *scripts, *out); err != nil {
		fmt.Fprintf(os.Stderr, "mkipk: %v\n", err)
		os.Exit(1)
	}
}

func build(controlFile, dataDir, scriptsDir, outFile string) error {
	controlGz, err := buildControlTarGz(controlFile, scriptsDir)
	if err != nil {
		return fmt.Errorf("control.tar.gz: %w", err)
	}
	dataGz, err := buildDataTarGz(dataDir)
	if err != nil {
		return fmt.Errorf("data.tar.gz: %w", err)
	}

	var ipk bytes.Buffer
	aw := newArWriter(&ipk)
	if err := aw.add("debian-binary", 0644, []byte("2.0\n")); err != nil {
		return err
	}
	if err := aw.add("control.tar.gz", 0644, controlGz); err != nil {
		return err
	}
	if err := aw.add("data.tar.gz", 0644, dataGz); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outFile), 0755); err != nil {
		return err
	}
	return os.WriteFile(outFile, ipk.Bytes(), 0644)
}

// buildControlTarGz assembles control.tar.gz: ./control (mode 0644) plus any
// maintainer scripts / conffiles from scriptsDir. Scripts are mode 0755.
func buildControlTarGz(controlFile, scriptsDir string) ([]byte, error) {
	control, err := os.ReadFile(controlFile)
	if err != nil {
		return nil, err
	}

	type entry struct {
		name string
		mode int64
		body []byte
	}
	entries := []entry{{name: "control", mode: 0644, body: control}}

	if scriptsDir != "" {
		ents, err := os.ReadDir(scriptsDir)
		if err != nil {
			return nil, err
		}
		// Scripts get 0755; conffiles is a plain manifest at 0644.
		scriptNames := map[string]bool{
			"postinst": true, "preinst": true, "prerm": true, "postrm": true,
		}
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			body, err := os.ReadFile(filepath.Join(scriptsDir, name))
			if err != nil {
				return nil, err
			}
			mode := int64(0644)
			if scriptNames[name] {
				mode = 0755
			}
			entries = append(entries, entry{name: name, mode: mode, body: body})
		}
	}

	// Stable order for reproducibility; ./control first.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].name == "control" {
			return true
		}
		if entries[j].name == "control" {
			return false
		}
		return entries[i].name < entries[j].name
	})

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     "./" + e.name,
			Mode:     e.mode,
			Size:     int64(len(e.body)),
			Typeflag: tar.TypeReg,
			Format:   tar.FormatGNU,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(e.body); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// buildDataTarGz walks dataDir and emits a gzip+tar with "./usr/..." paths.
// Every ancestor directory gets an explicit TypeDir entry (busybox tar needs
// the parent before it can create a leaf file). Output is deterministic.
func buildDataTarGz(dataDir string) ([]byte, error) {
	type fileEnt struct {
		rel  string // forward-slash relative path
		mode int64
		body []byte
		link string
		typ  byte
	}
	var files []fileEnt
	dirSet := map[string]bool{}

	addAncestors := func(rel string) {
		dir := path.Dir(rel)
		for dir != "." && dir != "/" && dir != "" {
			dirSet[dir] = true
			dir = path.Dir(dir)
		}
	}

	err := filepath.Walk(dataDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dataDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		switch {
		case info.IsDir():
			dirSet[rel] = true
			addAncestors(rel)
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			addAncestors(rel)
			files = append(files, fileEnt{rel: rel, mode: 0777, link: target, typ: tar.TypeSymlink})
		default:
			body, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			addAncestors(rel)
			files = append(files, fileEnt{rel: rel, mode: int64(info.Mode().Perm()), body: body, typ: tar.TypeReg})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	dirs := make([]string, 0, len(dirSet))
	for d := range dirSet {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// Directories first (sorted shallow->deep by lexical order), then files.
	for _, d := range dirs {
		hdr := &tar.Header{
			Name:     "./" + ensureSlash(d),
			Mode:     0755,
			Typeflag: tar.TypeDir,
			Format:   tar.FormatGNU,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
	}
	for _, f := range files {
		hdr := &tar.Header{
			Name:     "./" + f.rel,
			Mode:     f.mode,
			Typeflag: f.typ,
			Format:   tar.FormatGNU,
		}
		if f.typ == tar.TypeSymlink {
			hdr.Linkname = f.link
		} else {
			hdr.Size = int64(len(f.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if f.typ == tar.TypeReg {
			if _, err := tw.Write(f.body); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ensureSlash(p string) string {
	if strings.HasSuffix(p, "/") {
		return p
	}
	return p + "/"
}

// ---------------- ar writer ----------------

// arWriter emits a System V / GNU "common" ar archive. Each member header is
// 60 bytes: name[16] mtime[12] uid[6] gid[6] mode[8] size[10] then the magic
// "`\n" terminator. Members are 2-byte aligned (a '\n' pad byte follows any
// odd-length member body).
type arWriter struct {
	w       io.Writer
	started bool
}

func newArWriter(w io.Writer) *arWriter { return &arWriter{w: w} }

func (a *arWriter) add(name string, mode int64, body []byte) error {
	if !a.started {
		if _, err := io.WriteString(a.w, "!<arch>\n"); err != nil {
			return err
		}
		a.started = true
	}
	if len(name) > 16 {
		return fmt.Errorf("ar member name too long: %q", name)
	}
	hdr := make([]byte, 0, 60)
	hdr = appendField(hdr, name, 16)                         // name
	hdr = appendField(hdr, "0", 12)                          // mtime (zeroed for reproducibility)
	hdr = appendField(hdr, "0", 6)                           // uid
	hdr = appendField(hdr, "0", 6)                           // gid
	hdr = appendField(hdr, fmt.Sprintf("%o", mode), 8)       // mode (octal)
	hdr = appendField(hdr, fmt.Sprintf("%d", len(body)), 10) // size (decimal)
	hdr = append(hdr, '`', '\n')                             // magic terminator
	if _, err := a.w.Write(hdr); err != nil {
		return err
	}
	if _, err := a.w.Write(body); err != nil {
		return err
	}
	if len(body)%2 == 1 {
		if _, err := a.w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}

// appendField writes s left-justified, space-padded to width bytes.
func appendField(dst []byte, s string, width int) []byte {
	if len(s) > width {
		s = s[:width]
	}
	dst = append(dst, s...)
	for i := len(s); i < width; i++ {
		dst = append(dst, ' ')
	}
	return dst
}

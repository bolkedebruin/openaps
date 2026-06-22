package logx

import (
	"flag"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func initTo(t *testing.T, args ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.log")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := Bind(fs, "ecu-test", path)
	if err := fs.Parse(append([]string{"-log-file", path}, args...)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	c.Init()
	return path
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestBridgesStdlibAndSlog(t *testing.T) {
	path := initTo(t)
	log.Printf("via stdlib %d", 7)
	slog.Info("via slog", "port", 502)

	out := read(t, path)
	for _, want := range []string{"via stdlib 7", "via slog", "port=502", "component=ecu-test"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestTimestampsAreUTC(t *testing.T) {
	path := initTo(t)
	slog.Info("tick")

	out := read(t, path)
	// TextHandler renders the time attr as time=<RFC3339>. Forcing UTC means
	// the offset is the Z designator, never a local +hh:mm / -hh:mm.
	field := ""
	for _, f := range strings.Fields(out) {
		if strings.HasPrefix(f, "time=") {
			field = f
			break
		}
	}
	if field == "" {
		t.Fatalf("no time= field in output\n%s", out)
	}
	if !strings.HasSuffix(field, "Z") {
		t.Errorf("timestamp not UTC (want trailing Z): %s", field)
	}
}

func TestLevelFiltering(t *testing.T) {
	path := initTo(t, "-log-level", "warn")
	slog.Debug("debug-line")
	slog.Warn("warn-line")
	log.Printf("stdlib-line") // bridged at info, below warn → dropped

	out := read(t, path)
	if strings.Contains(out, "debug-line") {
		t.Errorf("debug should be filtered at warn level\n%s", out)
	}
	if strings.Contains(out, "stdlib-line") {
		t.Errorf("bridged stdlib info should be filtered at warn level\n%s", out)
	}
	if !strings.Contains(out, "warn-line") {
		t.Errorf("warn should pass\n%s", out)
	}
}

func TestUnwritableFileFallsBackToStderr(t *testing.T) {
	// A -log-file under a path component that is a regular file (not a dir)
	// cannot be created; Init must not panic and must still return a usable
	// logger rather than silently swallowing everything.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(blocker, "nested", "test.log") // blocker is a file, so MkdirAll/Open fail
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := Bind(fs, "ecu-test", bad)
	if err := fs.Parse([]string{"-log-file", bad}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	logger := c.Init()
	if logger == nil {
		t.Fatal("Init returned nil logger on unwritable file")
	}
	if _, err := os.Stat(bad); err == nil {
		t.Errorf("expected no log file at %s", bad)
	}
	logger.Info("still works") // must not panic
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"":      slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"ERROR": slog.LevelError,
	}
	for in, want := range cases {
		got, err := parseLevel(in)
		if err != nil || got != want {
			t.Errorf("parseLevel(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseLevel("nonsense"); err == nil {
		t.Error("parseLevel(nonsense) should error")
	}
}

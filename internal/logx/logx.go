// Package logx is the shared logging setup for the OpenAPS daemons. It
// gives every component one uniform logger: a leveled slog logger writing
// structured records (text by default, JSON optional) to stderr or a
// size-rotated file, with the standard library log package bridged onto the
// same handler so the existing log.Printf call sites across the codebase
// share one destination, level, and format.
//
// Usage in a command's main:
//
//	lc := logx.Bind(fs, "ecu-zb", "/var/log/ecu-zb.log")
//	fs.Parse(args)
//	lc.Init()
//	log.Printf("starting")            // bridged through slog
//	slog.Info("ready", "port", 502)   // native structured record
package logx

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config holds the resolved logging options. Bind registers the flags that
// populate it; Init applies them. Normally constructed via Bind; a zero
// value Init()s to info-level text on stderr with no component tag.
type Config struct {
	component string

	Level      string
	Format     string
	File       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// Bind registers the -log-* flags on fs and returns the Config they write
// to. component tags every record with a "component" attribute;
// defaultFile is the rotated log path used unless -log-file overrides it
// (an empty default means log to stderr).
func Bind(fs *flag.FlagSet, component, defaultFile string) *Config {
	c := &Config{component: component}
	fs.StringVar(&c.Level, "log-level", "info", "log level: debug|info|warn|error")
	fs.StringVar(&c.Format, "log-format", "text", "log format: text|json")
	fs.StringVar(&c.File, "log-file", defaultFile, "rotated log file path; empty = stderr")
	fs.IntVar(&c.MaxSizeMB, "log-max-size", 5, "max log file size in MB before rotation")
	fs.IntVar(&c.MaxBackups, "log-max-backups", 3, "rotated log files retained")
	fs.IntVar(&c.MaxAgeDays, "log-max-age", 7, "days to retain rotated logs")
	fs.BoolVar(&c.Compress, "log-compress", true, "gzip rotated log files")
	return c
}

// Init builds the logger from the resolved flags, installs it as the slog
// default, and routes the standard library log package onto the same
// handler. It returns the logger for callers that want to emit structured
// records directly. Call it once, after flag parsing.
func (c *Config) Init() *slog.Logger {
	level, err := parseLevel(c.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logx: %v; defaulting to info\n", err)
		level = slog.LevelInfo
	}

	// Select the sink. lumberjack opens the file lazily on first write, and
	// slog discards a handler's write error — so an unwritable -log-file
	// would silently swallow every log line. Probe it up front: create the
	// dir and the file (0600, matching lumberjack's new-file mode) so a
	// permission / read-only-fs problem falls back to stderr (captured by
	// the init script's boot log) with a visible warning instead.
	var sink io.Writer = os.Stderr
	var fileErr error
	if c.File != "" {
		if dir := filepath.Dir(c.File); dir != "" {
			_ = os.MkdirAll(dir, 0o755)
		}
		f, err := os.OpenFile(c.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			fileErr = err
		} else {
			_ = f.Close()
			sink = &lumberjack.Logger{
				Filename:   c.File,
				MaxSize:    c.MaxSizeMB,
				MaxBackups: c.MaxBackups,
				MaxAge:     c.MaxAgeDays,
				Compress:   c.Compress,
			}
		}
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if strings.EqualFold(c.Format, "json") {
		h = slog.NewJSONHandler(sink, opts)
	} else {
		h = slog.NewTextHandler(sink, opts)
	}
	if c.component != "" {
		h = h.WithAttrs([]slog.Attr{slog.String("component", c.component)})
	}
	logger := slog.New(h)
	slog.SetDefault(logger)

	// Bridge the standard library log package onto the same handler: the
	// codebase logs mostly via log.Printf, and routing those through slog
	// gives them the shared destination, level, and format without touching
	// every call site. Flags are cleared because the handler supplies the
	// timestamp. log.Fatalf still writes its line, then exits.
	log.SetFlags(0)
	log.SetOutput(&slogWriter{logger: logger, level: slog.LevelInfo})

	if fileErr != nil {
		slog.Warn("log file not writable; logging to stderr", "file", c.File, "err", fileErr)
	}
	return logger
}

// Fatal logs at error level via the default slog logger, then exits with
// status 1. It is the slog-native replacement for log.Fatalf, which slog
// has no equivalent of. Pass structured key/value pairs like slog.Error:
//
//	logx.Fatal("listen", "addr", addr, "err", err)
func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

// slogWriter adapts the line-oriented standard log package onto a structured
// slog logger: each Write becomes one record at the configured level, with
// the trailing newline stripped so it does not double up in the output.
type slogWriter struct {
	logger *slog.Logger
	level  slog.Level
}

func (w *slogWriter) Write(p []byte) (int, error) {
	w.logger.Log(context.Background(), w.level, strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q", s)
	}
}

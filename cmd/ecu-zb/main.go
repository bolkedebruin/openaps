// Command ecu-zb is a transparent splice between the host process and
// the CC2530 ZigBee modem on /dev/ttyO2, with a pcapng tap fanout.
//
// v1 is byte-oriented passthrough only; the hook interface exists for
// v2 but NoOpHook is the only wired implementation when no inv-driver
// socket is configured.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/bolkedebruin/openaps/internal/logx"
	"github.com/bolkedebruin/openaps/internal/zigbee/busmgr"
	"github.com/bolkedebruin/openaps/internal/zigbee/inventory"
	"github.com/bolkedebruin/openaps/internal/zigbee/modem"
	"github.com/bolkedebruin/openaps/internal/zigbee/proxy"
	"github.com/bolkedebruin/openaps/internal/zigbee/tap"
	"github.com/bolkedebruin/openaps/internal/zigbee/uart"
)

const version = "0.1.0"

func main() {
	cfg := parseFlags()
	cfg.log.Init()
	slog.Info("ecu-zb starting", "version", version, "proxy", cfg.proxy, "tty", cfg.tty)

	if !cfg.proxy {
		logx.Fatal("v1 requires --proxy (transparent splice mode)")
	}

	if err := run(cfg); err != nil {
		logx.Fatal("fatal", "err", err)
	}
}

type config struct {
	proxy         bool
	tty           string
	pty           string
	tapSock       string
	tapTCP        string
	bufSize       int
	invDriverSock string
	log           *logx.Config
}

func parseFlags() config {
	var c config
	c.log = logx.Bind(flag.CommandLine, "ecu-zb", "/var/log/ecu-zb.log")
	flag.BoolVar(&c.proxy, "proxy", false, "v1 transparent splice mode (required)")
	flag.StringVar(&c.tty, "tty", "/dev/ttyO2",
		"UART device the host process opens; ecu-zb renames it to <tty>.real and symlinks the pty slave at <tty>")
	flag.StringVar(&c.pty, "pty", "",
		"optional additional symlink for the pty slave (e.g. /home/applications/ecu-zb/run/zb-pty)")
	flag.StringVar(&c.tapSock, "tap-sock", "",
		"Unix STREAM socket path for pcapng fanout (empty disables)")
	flag.StringVar(&c.tapTCP, "tap-tcp", "",
		"TCP listen address for pcapng fanout, e.g. 0.0.0.0:19999 (empty disables)")
	flag.IntVar(&c.bufSize, "bufsize", 64, "max tty read size; ≤16-byte chunks expected, 64 is a safe ceiling")
	flag.StringVar(&c.invDriverSock, "invdriver-sock", "",
		"UDS path of inv-driver daemon to publish reassembled L1 reply frames to (e.g. /var/run/inv-driver.sock); empty disables — without it, ecu-zb runs as splice + pcap tap only")
	flag.Parse()
	return c
}

func run(cfg config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Take ownership of the real UART. After this, /dev/ttyO2 is
	//    free for us to symlink at the pty slave.
	realPath, restoreUART, err := captureRealUART(cfg.tty)
	if err != nil {
		return fmt.Errorf("capture %s: %w", cfg.tty, err)
	}
	defer restoreUART()
	slog.Info("real UART available", "path", realPath)

	modemPort, err := uart.OpenSerial(realPath)
	if err != nil {
		return fmt.Errorf("open real uart: %w", err)
	}
	defer modemPort.Close()
	slog.Info("opened modem UART @ 57600 8N1 raw", "path", realPath)

	// 2. Allocate the pty pair and publish its slave at the original
	//    path (and optionally at --pty).
	if cfg.pty != "" {
		if err := os.MkdirAll(dirOf(cfg.pty), 0o755); err != nil {
			return fmt.Errorf("mkdir for --pty: %w", err)
		}
	}

	var (
		ptyMu  sync.Mutex
		curPty *uart.PTY
	)
	openAndPublishPTY := func() (*uart.PTY, error) {
		p, err := uart.OpenPTY()
		if err != nil {
			return nil, fmt.Errorf("open pty: %w", err)
		}
		_ = os.Remove(cfg.tty)
		if err := os.Symlink(p.SlavePath, cfg.tty); err != nil {
			_ = p.Close()
			return nil, fmt.Errorf("symlink %s -> %s: %w", cfg.tty, p.SlavePath, err)
		}
		if cfg.pty != "" {
			_ = os.Remove(cfg.pty)
			if err := os.Symlink(p.SlavePath, cfg.pty); err != nil {
				_ = os.Remove(cfg.tty)
				_ = p.Close()
				return nil, fmt.Errorf("symlink %s -> %s: %w", cfg.pty, p.SlavePath, err)
			}
		}
		return p, nil
	}

	pty, err := openAndPublishPTY()
	if err != nil {
		return err
	}
	curPty = pty
	defer func() {
		ptyMu.Lock()
		if curPty != nil {
			_ = curPty.Close()
			curPty = nil
		}
		ptyMu.Unlock()
		_ = os.Remove(cfg.tty)
		if cfg.pty != "" {
			_ = os.Remove(cfg.pty)
		}
	}()
	slog.Info("pty slave ready", "path", pty.SlavePath)
	slog.Info("symlinked tty to pty slave", "tty", cfg.tty, "slave", pty.SlavePath)
	if cfg.pty != "" {
		slog.Info("symlinked extra pty to pty slave", "pty", cfg.pty, "slave", pty.SlavePath)
	}

	// 2b. Bring the radio up at boot: a one-time coordinator config
	// (0x0D liveness ping, hardware-resetting an unresponsive radio,
	// then 0x05 Set-PANID+Channel). The module retains this, so we do
	// not re-assert it every poll cycle. It runs after the pty is published
	// (so the pty appears promptly regardless of bring-up duration) but
	// before the tap/busmgr/splice start, while nothing else touches the
	// modem fd. Best-effort: a warning on failure, since an already-
	// configured module keeps working and the splice does not depend on it.
	opPAN, opChannel := bringupModem(ctx, int(modemPort.Fd()), cfg.invDriverSock)

	// 3. Pcapng tap fanout.
	br := tap.NewBroadcaster(tap.SectionInfo{
		Hardware: "APsystems ECU-R-Pro",
		OS:       unameString(),
		UserAppl: "ecu-zb v" + version,
	})
	srv := tap.NewServer(br, cfg.tapSock, cfg.tapTCP)
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("tap start: %w", err)
	}
	defer srv.Stop()

	// 4. Splice hook + optional inv-driver busmgr client.
	//
	// When an inv-driver socket is configured, BusTrackerHook is always
	// active: it suppresses replies to inv-driver-originated queries
	// from the host stream and publishes every reassembled L1 frame
	// upstream. Polling is driven entirely by inv-driver's Send path
	// (InjectTracked). When no inv-driver socket is configured,
	// NoOpHook is used instead.
	var hook proxy.Hook = proxy.NoOpHook{}
	var tracker *proxy.BusTrackerHook
	var busClient *busmgr.Client
	// wd is the runtime radio watchdog. It is allocated up front (not where its
	// Probe/Recover are wired) so the RawReplySink closure below captures a
	// stable non-nil pointer: NoteInbound runs on the dispatch goroutine and the
	// wiring runs on this one, so a late assignment would be an unsynchronised
	// read. NoteInbound on a watchdog whose Run never starts is a harmless
	// timestamp nobody reads. Run is started later, only under watchdogEnabled.
	var wd *modem.Watchdog

	if cfg.invDriverSock != "" {
		tracker = proxy.NewBusTrackerHook()
		hook = tracker
		wd = &modem.Watchdog{}

		inv := inventory.NewMap()
		busClient = busmgr.New(cfg.invDriverSock, "apsystems-stock-zb", version)
		busClient.Inventory = inv

		pub := &busmgr.Publisher{C: busClient, Inventory: inv}
		tracker.RawReplySink = func(sa uint16, l1 []byte) {
			wd.NoteInbound()
			pub.RawFrame(sa, l1)
		}
		busClient.OnSend = tracker.InjectTracked

		if err := busClient.Start(ctx); err != nil {
			return fmt.Errorf("busmgr start: %w", err)
		}
		slog.Info("busmgr sink configured", "sock", cfg.invDriverSock)
	}
	if busClient != nil {
		defer busClient.Stop()
	}

	sp := &proxy.Splice{
		Modem:   modemPort,
		Host:    pty.Master,
		Hook:    hook,
		Tap:     br,
		BufSize: cfg.bufSize,
		HostReopener: func(prev io.ReadWriter) (io.ReadWriter, error) {
			ptyMu.Lock()
			defer ptyMu.Unlock()
			// Sanity check: prev should be the master we currently
			// know about. If not, somebody already swapped it.
			if curPty != nil && curPty.Master != prev {
				return curPty.Master, nil
			}
			old := curPty
			next, err := openAndPublishPTY()
			if err != nil {
				return nil, err
			}
			curPty = next
			if old != nil {
				_ = old.Close()
			}
			slog.Warn("pty reopened", "slave", next.SlavePath)
			return next.Master, nil
		},
	}

	if tracker != nil {
		if err := tracker.Start(sp); err != nil {
			return fmt.Errorf("tracker start: %w", err)
		}
		defer tracker.Stop()
	}

	// Pairing executor: OTA (re)pairing primitives run on the modem fd with
	// the splice paused for exclusivity. Wired only when inv-driver is
	// connected (it drives PairingCmd); without it pairing has no client.
	// The same adapter instance backs the radio watchdog so probe/pairing
	// share the pairing lock and never race.
	var pairer *pairingAdapter
	if busClient != nil {
		pairer = newPairingAdapter(sp, int(modemPort.Fd()), opPAN, opChannel)
		busClient.Pairing = pairer
		if watchdogEnabled(true, opPAN) {
			wd.Probe = pairer.Probe
			wd.Recover = pairer.Recover
		} else {
			slog.Warn("radio watchdog disabled: no operating PAN")
		}
	}

	// 5. Splice + signal handling.
	//
	// Shutdown is tricky because *os.File.Close on a /dev/pts/N or
	// /dev/ttyO2.real character device does not always unblock an
	// in-flight syscall.Read on Linux (the runtime poller doesn't
	// manage tty fds). So on signal we:
	//   - cancel ctx so any non-blocked code paths bail out
	//   - close the fds (which lets the kernel deliver EBADF on the
	//     next read, even if the current syscall hasn't returned)
	//   - release the symlinks and rename .real back NOW, not via
	//     deferred cleanup — defers won't run if we have to os.Exit
	//   - give the splice 1s to drain naturally; force-exit otherwise
	releaseSymlinks := func() {
		_ = os.Remove(cfg.tty)
		if cfg.pty != "" {
			_ = os.Remove(cfg.pty)
		}
	}

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	spDone := make(chan error, 1)
	go func() { spDone <- sp.Run(ctx) }()

	// Runtime radio recovery: once the splice owns the modem reader, the
	// watchdog probes the radio through the pairing redirect when the bus goes
	// silent and re-arms it if it has wedged. Shares ctx so it stops on
	// shutdown. Started only when its Probe/Recover were wired above (an
	// inv-driver-backed pairing adapter and a known operating PAN).
	if wd != nil && wd.Probe != nil {
		go wd.Run(ctx)
		slog.Info("radio watchdog running")
	}

	slog.Info("splice running",
		"goroutines", runtime.NumGoroutine(), "gomaxprocs", runtime.GOMAXPROCS(0))

	select {
	case err := <-spDone:
		if err != nil && !isClosedAfterShutdown(ctx, err) {
			return fmt.Errorf("splice: %w", err)
		}
		slog.Info("splice ended cleanly")
		return nil
	case s := <-sigs:
		slog.Info("signal received, shutting down", "signal", s)
		cancel()
		if tracker != nil {
			tracker.Stop()
		}
		releaseSymlinks()
		_ = modemPort.Close()
		ptyMu.Lock()
		if curPty != nil {
			_ = curPty.Close()
			curPty = nil
		}
		ptyMu.Unlock()
		srv.Stop()
		select {
		case <-spDone:
			slog.Info("splice drained naturally")
		case <-time.After(1 * time.Second):
			slog.Warn("splice did not drain within 1s, forcing exit")
		}
		// Restore /dev/ttyO2 from /dev/ttyO2.real.
		restoreUART()
		slog.Info("shutdown complete")
		os.Exit(0)
		return nil
	}
}

// captureRealUART takes ownership of the device file at ttyPath. If
// ttyPath.real already exists (a previous ecu-zb run left it staged),
// we resume by opening that. Otherwise we rename ttyPath -> ttyPath.real.
// The returned restore() reverses whichever operation we did.
func captureRealUART(ttyPath string) (realPath string, restore func(), err error) {
	realPath = ttyPath + ".real"

	stagedExists := pathExists(realPath)
	originalExists := pathExists(ttyPath)
	originalIsLink, _ := isSymlink(ttyPath)

	switch {
	case stagedExists && originalExists && !originalIsLink:
		return "", noop, fmt.Errorf("inconsistent: both %s and %s exist; refusing to clobber", ttyPath, realPath)
	case stagedExists:
		// Resume mode: somebody else (or a prior crashed ecu-zb) already
		// moved the device. Don't undo that on shutdown — leave it as
		// they staged it.
		return realPath, noop, nil
	case originalExists && !originalIsLink:
		if err := os.Rename(ttyPath, realPath); err != nil {
			return "", noop, fmt.Errorf("rename %s -> %s: %w", ttyPath, realPath, err)
		}
		return realPath, func() {
			_ = os.Remove(ttyPath) // remove our pty symlink first
			if err := os.Rename(realPath, ttyPath); err != nil {
				slog.Error("restore rename failed", "from", realPath, "to", ttyPath, "err", err)
			} else {
				slog.Info("restored tty", "tty", ttyPath)
			}
		}, nil
	default:
		return "", noop, fmt.Errorf("neither %s nor %s present; nothing to capture", ttyPath, realPath)
	}
}

func noop() {}

// bringupModem configures the radio at startup. Both the PAN and the
// channel come from inv-driver settings
// when available (single source of truth): pan_override and channel are
// fetched in one round-trip. The PAN falls back to ReadPAN autodetect
// (provisioned MAC config if present, else the live eth0 MAC); the channel
// falls back to channel.conf / DefaultChannel. Every step is best-effort: if
// the PAN is underivable or the module doesn't ack, we log and carry on — a
// module configured by a prior run keeps working, and the splice/poll path
// does not depend on this succeeding. Returns the resolved operating PAN and
// channel (0,0 if the PAN could not be derived) so the pairing executor can
// own/restore them.
func bringupModem(ctx context.Context, fd int, invDriverSock string) (uint16, byte) {
	pan, channel, ok := resolveRadioConfig(ctx, invDriverSock)
	if !ok {
		slog.Warn("modem skipping bring-up, cannot derive PAN")
		return 0, 0
	}
	if err := modem.BringupAPsystems(fd, pan, channel); err != nil {
		slog.Warn("modem bring-up incomplete", "err", err)
	}
	return pan, channel
}

// resolveRadioConfig resolves the operating PAN and channel, preferring
// inv-driver settings (pan_override + channel) over the stock confs. Returns
// ok=false only when no PAN can be derived at all.
func resolveRadioConfig(ctx context.Context, invDriverSock string) (pan uint16, channel byte, ok bool) {
	var panHex string
	var chanSetting uint32
	if invDriverSock != "" {
		panHex, chanSetting, _ = modem.FetchSettings(ctx, invDriverSock, "apsystems-stock-zb")
	}

	if p, parsed := modem.ParsePANHex(panHex); parsed {
		pan = p
		slog.Info("modem using inv-driver pan_override", "pan", fmt.Sprintf("0x%04X", pan))
	} else {
		if panHex != "" {
			slog.Warn("modem ignoring unparseable pan_override, using autodetect", "pan_override", panHex)
		}
		p, err := modem.ReadPAN(modem.EthInterface)
		if err != nil {
			slog.Error("modem cannot derive PAN", "err", err)
			return 0, 0, false
		}
		pan = p
	}

	channel = modem.ResolveChannel(chanSetting)
	if chanSetting >= 11 && chanSetting <= 26 {
		slog.Info("modem using inv-driver channel", "channel", channel)
	} else {
		slog.Info("modem using channel.conf channel", "channel", channel)
	}
	return pan, channel, true
}

func pathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func isSymlink(p string) (bool, error) {
	fi, err := os.Lstat(p)
	if err != nil {
		return false, err
	}
	return fi.Mode()&os.ModeSymlink != 0, nil
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}

func isClosedAfterShutdown(ctx context.Context, err error) bool {
	if ctx.Err() == nil {
		return false
	}
	// After cancellation we close fds, so reads return io.EOF or
	// "file already closed". Treat those as clean.
	if err == io.EOF {
		return true
	}
	if errStr := err.Error(); errStr != "" {
		// Best-effort string match; the standard library does not
		// expose os.ErrClosed for *os.File reads in a typed way.
		for _, s := range []string{"file already closed", "use of closed", "context canceled"} {
			if contains(errStr, s) {
				return true
			}
		}
	}
	return false
}

func contains(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

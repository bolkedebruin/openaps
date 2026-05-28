package settings

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Security invariants:
//   1. mac is matched against macPattern (6 colon-hex octets) before
//      any exec. The handler validates; this function re-validates.
//   2. All three commands run via exec.CommandContext with an explicit
//      argv slice. No shell is spawned. The mac is passed as a single
//      argv element; no string concatenation reaches a shell.
//   3. The interface name is a package constant, not derived from
//      operator input.
//   4. The binary path is a package constant ("/bin/ip"); $PATH is
//      not consulted.

// applyMACBin and applyMACInterface are package-level vars so tests
// can stub them; production defaults are "/bin/ip" and "eth0".
var (
	applyMACBin       = "/bin/ip"
	applyMACInterface = "eth0"
)

// applyMACStepTimeout bounds each individual `ip link` invocation.
const applyMACStepTimeout = 5 * time.Second

// applyMACVerifyDelay is the settle delay before reading back the
// applied MAC from sysfs; applyMACVerifyTimeout bounds the readback
// poll itself.
const (
	applyMACVerifyDelay   = 200 * time.Millisecond
	applyMACVerifyTimeout = 2 * time.Second
	applyMACVerifyPoll    = 100 * time.Millisecond
)

// runCmd is a stubbable wrapper around exec.CommandContext(...).CombinedOutput()
// so unit tests can capture the exact argv without touching the system.
var runCmd = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// readMACFile returns the lower-case eth0 MAC from sysfs (the verify
// readback path). Overridable for tests.
var readMACFile = func() string {
	b, err := os.ReadFile("/sys/class/net/" + applyMACInterface + "/address")
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(string(b)))
}

// ApplyMAC reconfigures eth0 to use mac.
// mac MUST be pre-validated against macPattern; ApplyMAC re-validates
// defense-in-depth.
//
// The command sequence is:
//
//	ip link set dev eth0 down
//	ip link set dev eth0 address <mac>
//	ip link set dev eth0 up
//
// Each step runs via exec.CommandContext with an explicit argv — no
// shell is invoked — so even an unsanitised mac could not escape.
//
// Returns nil only when all three steps succeed AND a sysfs readback
// confirms the new value. On any step's failure the function returns
// that step's error wrapped with context. A failure AFTER `down`
// succeeded would leave eth0 administratively down (remote brick), so
// ApplyMAC always issues a best-effort recovery `up` on the error
// return path. The success path leaves eth0 already up — the recovery
// `up` becomes a redundant no-op.
func ApplyMAC(ctx context.Context, mac string) error {
	if !macPattern.MatchString(mac) {
		return fmt.Errorf("applymac: mac %q invalid: want 6 colon-separated hex octets", mac)
	}

	var retErr error
	defer func() {
		if retErr == nil {
			return
		}
		// Best-effort link-up recovery so a mid-sequence failure does not
		// strand eth0 in the DOWN state. Use a fresh short timeout so a
		// caller-cancelled ctx still gets a recovery attempt.
		recCtx, cancel := context.WithTimeout(context.Background(), applyMACStepTimeout)
		defer cancel()
		_ = runStep(recCtx, "up-recovery", []string{"link", "set", "dev", applyMACInterface, "up"})
	}()

	steps := []struct {
		label string
		args  []string
	}{
		{"down", []string{"link", "set", "dev", applyMACInterface, "down"}},
		{"address", []string{"link", "set", "dev", applyMACInterface, "address", mac}},
		{"up", []string{"link", "set", "dev", applyMACInterface, "up"}},
	}
	for _, step := range steps {
		if err := runStep(ctx, step.label, step.args); err != nil {
			retErr = err
			return retErr
		}
	}
	if err := verifyMAC(ctx, mac); err != nil {
		retErr = err
		return retErr
	}
	return nil
}

// runStep executes one `ip link` invocation under its own short timeout.
func runStep(ctx context.Context, label string, args []string) error {
	stepCtx, cancel := context.WithTimeout(ctx, applyMACStepTimeout)
	defer cancel()
	out, err := runCmd(stepCtx, applyMACBin, args...)
	if err != nil {
		return fmt.Errorf("applymac: step %s failed: %w (output: %s)", label, err, truncOutput(out))
	}
	return nil
}

// verifyMAC polls the sysfs address attribute until it matches the
// requested MAC (lower-case) or the verify timeout elapses.
func verifyMAC(ctx context.Context, mac string) error {
	want := strings.ToLower(mac)

	select {
	case <-time.After(applyMACVerifyDelay):
	case <-ctx.Done():
		return fmt.Errorf("applymac: verify cancelled before settle: %w", ctx.Err())
	}

	deadline := time.Now().Add(applyMACVerifyTimeout)
	var got string
	for {
		got = readMACFile()
		if got == want {
			return nil
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-time.After(applyMACVerifyPoll):
		case <-ctx.Done():
			return fmt.Errorf("applymac: verify cancelled: %w", ctx.Err())
		}
	}
	return fmt.Errorf("applymac: verify failed: eth0 address is %q, want %q", got, want)
}

// truncOutput trims a command's combined output for inclusion in an
// error message — keeps the error compact in logs.
func truncOutput(b []byte) string {
	const max = 200
	s := strings.TrimSpace(string(b))
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

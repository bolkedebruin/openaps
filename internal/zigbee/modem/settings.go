package modem

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bolkedebruin/openaps/internal/udsutil"
	"github.com/bolkedebruin/openaps/wire"
)

// settingsTimeout bounds the whole fetch (dial + hello + request +
// response). Bring-up cannot stall on a slow or absent inv-driver.
const settingsTimeout = 3 * time.Second

// FetchSettings opens a short-lived connection to the inv-driver UDS at
// sockPath, says Hello (role unspecified, which runs inv-driver's publisher
// loop that serves get requests), asks for the current settings, and returns
// the pan_override and channel fields so the radio config has a single source
// of truth (inv-driver) rather than the stock /etc/yuneng confs. ok is true
// only when a SettingsResponse arrives; on any dial/write/read error or
// timeout it returns ("", 0, false) so the caller falls back to autodetect /
// channel.conf.
func FetchSettings(ctx context.Context, sockPath, backend string) (panHex string, channel uint32, ok bool) {
	ctx, cancel := context.WithTimeout(ctx, settingsTimeout)
	defer cancel()

	host, _ := os.Hostname()
	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend:     backend,
		Hostname:    host,
		StartedAtMs: time.Now().UnixMilli(),
	}}}
	req := &wire.Envelope{Body: &wire.Envelope_SettingsReq{SettingsReq: &wire.SettingsRequest{
		Op: &wire.SettingsRequest_Get{Get: &wire.Empty{}},
	}}}
	env, err := udsutil.Roundtrip(ctx, sockPath, hello,
		func(e *wire.Envelope) bool { return e.GetSettingsResp() != nil }, req)
	if err != nil {
		log.Printf("modem: settings fetch skipped, %s: %v", sockPath, err)
		return "", 0, false
	}
	s := env.GetSettingsResp().GetSettings()
	return s.GetPanOverride(), s.GetChannel(), true
}

// ParsePANHex parses a 16-bit PAN from a hex string, tolerating an
// optional "0x" prefix and surrounding whitespace and accepting 1-4 hex
// digits. ok is false for empty or non-hex input.
func ParsePANHex(s string) (pan uint16, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if s == "" || len(s) > 4 {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(v), true
}

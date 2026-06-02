// Package config loads ecu-sunspec runtime configuration from a JSON file.
//
// Settings that are sensitive (writes, IP allowlist) live in this file
// rather than in CLI flags so they're not exposed in `ps`, can be set up
// once at deploy time, and ship cleanly inside the install package.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
)

// DefaultPath is where the binary looks if --config isn't given.
const DefaultPath = "/home/sunspec.json"

// Config is the on-disk schema. All fields are optional; the zero value of
// each field is a safe default (writes off, no allowlist, etc.).
type Config struct {
	Writes WritesConfig `json:"writes"`
}

// WritesConfig gates Modbus write-class function codes (FC06, FC16) and
// SunSpec Model 123 control actions.
type WritesConfig struct {
	// Enabled toggles whether the server accepts any writes.
	//
	// JSON tristate: omitted = nil = default (true); explicit false disables.
	// The default is on so off-the-shelf ESS tooling (Victron Cerbo,
	// dbus-fronius, Home Assistant) can drive Model 123 curtailment
	// without operator intervention. Combine with AllowList /
	// AllowLocalNetwork for source-address gating.
	Enabled *bool `json:"enabled,omitempty"`

	// AllowList is the list of additional remote-host IP addresses (or
	// CIDRs) that may issue writes, beyond the loopback and local-subnet
	// defaults.
	//
	// Format: dotted-quad IP addresses ("10.25.1.29") or CIDRs ("10.25.1.0/24").
	AllowList []string `json:"allow_list"`

	// AllowLocalNetwork, when true, auto-allows writes from any address on
	// the same subnet as one of the server's network interfaces. Default
	// true — most home setups want their LAN devices (HA, Victron GX,
	// laptop, etc.) to write without per-host enumeration. Set to false
	// for stricter deployments where only AllowList entries should write.
	//
	// JSON tristate: omitted = nil = default (true); explicit false disables.
	AllowLocalNetwork *bool `json:"allow_local_network,omitempty"`
}

// IsEnabled reports whether writes are enabled. Default (nil) is true.
func (w WritesConfig) IsEnabled() bool {
	if w.Enabled == nil {
		return true
	}
	return *w.Enabled
}

// BoolPtr returns a pointer to b. Convenience for programmatic config
// construction (tests, sidecar config, etc.):
//
//	cfg.Writes.Enabled = config.BoolPtr(false)
func BoolPtr(b bool) *bool { return &b }

// LocalAllowed reports the effective value of AllowLocalNetwork — defaulting
// to true when the field is omitted.
func (w WritesConfig) LocalAllowed() bool {
	if w.AllowLocalNetwork == nil {
		return true
	}
	return *w.AllowLocalNetwork
}

// Load reads and validates a Config JSON file. Returns the zero value with no
// error if the file does not exist (writes default to disabled in that case).
func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Missing file is the expected case for installs that haven't
			// opted into writes yet. Return the zero-value Config (writes
			// disabled, empty allowlist).
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := c.validate(); err != nil {
		return Config{}, fmt.Errorf("invalid %s: %w", path, err)
	}
	return c, nil
}

func (c Config) validate() error {
	for _, item := range c.Writes.AllowList {
		if _, _, err := net.ParseCIDR(item); err == nil {
			continue
		}
		if ip := net.ParseIP(item); ip != nil {
			continue
		}
		return fmt.Errorf("writes.allow_list: %q is neither an IP nor a CIDR", item)
	}
	return nil
}

// AllowsWrite reports whether the given remote address (typically
// req.ClientAddr from simonvetter/modbus, format "host:port") is permitted
// to issue writes.
//
// Decision order:
//  1. Reject if writes are explicitly disabled.
//  2. Always allow loopback.
//  3. Allow if the IP is in any local interface's subnet AND
//     AllowLocalNetwork is on (default).
//  4. Allow if the IP is listed in AllowList (exact match or CIDR).
//  5. Reject otherwise.
func (c Config) AllowsWrite(remoteAddr string) bool {
	if !c.Writes.IsEnabled() {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	if c.Writes.LocalAllowed() && ipInLocalNetwork(ip) {
		return true
	}
	for _, item := range c.Writes.AllowList {
		if cidrIP := net.ParseIP(item); cidrIP != nil {
			if cidrIP.Equal(ip) {
				return true
			}
			continue
		}
		if _, cidr, err := net.ParseCIDR(item); err == nil && cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// ipInLocalNetwork reports whether ip lies inside any of the host's own
// network interfaces' subnets (e.g. the ECU's eth0 is 10.25.1.33/24, so
// 10.25.1.29 returns true).
//
// Indirected through a package-level var so tests can stub it out.
var ipInLocalNetwork = func(ip net.IP) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		if cidr, ok := a.(*net.IPNet); ok && cidr.Contains(ip) {
			// Skip the loopback /8 — already handled separately, and we
			// don't want to over-match 127.0.0.0/8 to 10.x via stray addrs.
			if cidr.IP.IsLoopback() {
				continue
			}
			return true
		}
	}
	return false
}

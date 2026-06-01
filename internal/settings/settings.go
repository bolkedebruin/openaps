// Package settings owns inv-driver's single configuration file (ecu-id,
// mac, pan-override, zigbee-type). inv-driver is the sole writer; other
// Go processes obtain these values from inv-driver over the UDS.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultChannel is the ZigBee channel used when Settings.Channel is unset.
// inv-driver no longer interprets the channel beyond this 0->default
// fallback; ecu-zb owns range validation.
const DefaultChannel uint32 = 16

// Settings is the operator-owned configuration persisted as one JSON
// file. Zero values mean "unset".
type Settings struct {
	// EcuID is the operator's identifier for this ECU (shown in the web
	// console). Free-form; not tied to any APsystems cloud id.
	EcuID string `json:"ecu_id"`
	// MAC overrides the network MAC the Go stack reports/uses. Empty =
	// use the live interface MAC.
	MAC string `json:"mac"`
	// PANOverride, when non-empty (hex, e.g. "0DCE"), forces the ZigBee
	// PAN id instead of deriving it from the MAC.
	PANOverride string `json:"pan_override"`
	// ZigbeeType selects the radio backend: "apsystems" (the built-in
	// module) or "general" (a plain/USB ZigBee modem). Empty defaults to
	// apsystems.
	ZigbeeType string `json:"zigbee_type"`
	// InverterNames maps an inverter UID to an operator-chosen label.
	InverterNames map[string]string `json:"inverter_names,omitempty"`
	// Channel is the ZigBee radio channel (11-26). Zero means unset, in
	// which case the built-in default channel is used.
	Channel uint32 `json:"channel,omitempty"`
}

// Store is the concurrency-safe owner of the settings file.
type Store struct {
	path string
	mu   sync.RWMutex
	s    Settings
}

// Open loads the settings file at path. A missing file is not an error —
// it yields zero-value Settings (first run).
func Open(path string) (*Store, error) {
	st := &Store{path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return st, nil
		}
		return nil, fmt.Errorf("settings: read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, &st.s); err != nil {
		return nil, fmt.Errorf("settings: parse %s: %w", path, err)
	}
	return st, nil
}

// Get returns a copy of the current settings.
func (st *Store) Get() Settings {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.s
}

// Save replaces the settings and persists them atomically (write temp +
// rename).
func (st *Store) Save(s Settings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(st.path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	tmp := st.path + ".tmp"
	// 0o600: the settings file holds operator identifiers and the MAC
	// override; only root needs to read it.
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("settings: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, st.path); err != nil {
		return fmt.Errorf("settings: rename %s: %w", st.path, err)
	}
	st.mu.Lock()
	st.s = s
	st.mu.Unlock()
	return nil
}

// ReadEth0MAC returns the live eth0 MAC as a lower-case colon-separated
// string, or "" on any error. It first reads the sysfs attribute (cheap,
// no syscall overhead) and falls back to net.Interfaces for portability
// on hosts without /sys (the developer's macOS). It is used to log the
// live MAC alongside a MAC-apply event; inv-driver no longer derives a PAN
// from it.
func ReadEth0MAC() string {
	if b, err := os.ReadFile("/sys/class/net/eth0/address"); err == nil {
		s := strings.TrimSpace(string(b))
		if s != "" {
			return strings.ToLower(s)
		}
	}
	ifi, err := net.InterfaceByName("eth0")
	if err == nil && len(ifi.HardwareAddr) > 0 {
		return strings.ToLower(ifi.HardwareAddr.String())
	}
	return ""
}

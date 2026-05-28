package settings

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// Effective is the resolved settings the Go stack actually uses: each
// operator-set field falls back to the live system value when empty.
type Effective struct {
	MAC        string
	PAN        string
	ZigbeeType string
	Channel    uint32
}

// defaultZigbeeType is the radio backend used when ZigbeeType is unset.
const defaultZigbeeType = "apsystems"

// DefaultChannel is the ZigBee channel used when Settings.Channel is unset.
const DefaultChannel uint32 = 16

// Resolve computes the effective values for s. readLiveMAC supplies the
// network MAC when the operator hasn't overridden it; pass ReadEth0MAC
// in production, a stub in tests. A nil reader is treated as no live MAC.
func Resolve(s Settings, readLiveMAC func() string) Effective {
	mac := s.MAC
	if mac == "" && readLiveMAC != nil {
		mac = readLiveMAC()
	}

	pan := strings.ToUpper(s.PANOverride)
	switch {
	case pan != "":
		// Operator override wins. Left-pad to 4 hex digits so the UI
		// displays a consistent 16-bit value.
		if n := len(pan); n < 4 {
			pan = strings.Repeat("0", 4-n) + pan
		}
	case mac != "":
		if p, ok := lower16FromMAC(mac); ok {
			pan = fmt.Sprintf("%04X", p)
		}
	}

	zt := s.ZigbeeType
	if zt == "" {
		zt = defaultZigbeeType
	}

	ch := s.Channel
	if ch == 0 {
		ch = DefaultChannel
	}

	return Effective{MAC: mac, PAN: pan, ZigbeeType: zt, Channel: ch}
}

// lower16FromMAC returns the lower 16 bits of a colon-separated MAC as
// uint16. Returns ok=false on a malformed MAC.
func lower16FromMAC(mac string) (uint16, bool) {
	hw, err := net.ParseMAC(mac)
	if err != nil || len(hw) < 2 {
		return 0, false
	}
	return uint16(hw[len(hw)-2])<<8 | uint16(hw[len(hw)-1]), true
}

// ReadEth0MAC returns the live eth0 MAC as a lower-case colon-separated
// string, or "" on any error. It first reads the sysfs attribute (cheap,
// no syscall overhead) and falls back to net.Interfaces for portability
// on hosts without /sys (the developer's macOS).
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

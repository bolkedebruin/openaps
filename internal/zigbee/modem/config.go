package modem

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// EthInterface is the network interface whose MAC seeds the ZigBee
	// coordinator PAN ID when no MAC config file is present.
	EthInterface = "eth0"

	// MACConfPath is the APsystems provisioned eth0 MAC config. When it
	// exists it is the authoritative source for the PAN.
	MACConfPath = "/etc/yuneng/ecu_eth0_mac.conf"

	// ChannelConfPath is the RF channel config (APsystems init_ecu layout).
	ChannelConfPath = "/etc/yuneng/channel.conf"

	// DefaultChannel is the fallback when channel.conf is absent or
	// unparseable — 0x10 (decimal 16, ~2435 MHz), matching init_ecu.
	DefaultChannel byte = 0x10
)

// ReadPAN derives the ZigBee coordinator PAN ID (low 16 bits of the eth0
// MAC). It autodetects the source:
//
//   - If the APsystems MAC config file exists, it is used. It holds the
//     provisioned MAC the fleet's PAN is bonded to and is readable from
//     early boot, so the PAN is correct regardless of boot ordering and of
//     when the live interface MAC is programmed.
//   - Otherwise (e.g. a future image without the legacy config) it falls
//     back to the live hardware MAC from the kernel.
func ReadPAN(iface string) (uint16, error) {
	if data, err := os.ReadFile(MACConfPath); err == nil {
		if pan, err := parsePAN(string(data)); err == nil {
			return pan, nil
		}
	}
	path := "/sys/class/net/" + iface + "/address"
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("no MAC config and read %s: %w", path, err)
	}
	return parsePAN(string(data))
}

// parsePAN returns the low 16 bits (last two octets) of a MAC string.
func parsePAN(s string) (uint16, error) {
	oct := macOctets(s)
	if len(oct) < 2 {
		return 0, fmt.Errorf("MAC %q has fewer than 2 octets", strings.TrimSpace(s))
	}
	return uint16(oct[len(oct)-2])<<8 | uint16(oct[len(oct)-1]), nil
}

// macOctets parses the hex octets from a MAC string in either colon-
// separated ("00:11:22:33:44:55") or bare ("001122334455") form,
// case-insensitive, ignoring any non-hex characters.
func macOctets(s string) []byte {
	var hex []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			hex = append(hex, c)
		}
	}
	if len(hex) < 2 {
		return nil
	}
	out := make([]byte, len(hex)/2)
	for i := range out {
		v, _ := strconv.ParseUint(string(hex[i*2:i*2+2]), 16, 8)
		out[i] = byte(v)
	}
	return out
}

// ReadChannel reads the RF channel from channel.conf, returning
// DefaultChannel when the file is missing or unparseable. Values may be
// hex ("0x10") or decimal ("16"), matching init_ecu.
func ReadChannel(path string) byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultChannel
	}
	s := strings.TrimSpace(string(data))
	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
		base = 16
	}
	v, err := strconv.ParseUint(s, base, 8)
	if err != nil {
		return DefaultChannel
	}
	return byte(v)
}

// ResolveChannel picks the operating RF channel: the inv-driver settings
// channel when it is a usable 2.4 GHz value (11-26), else the stock
// channel.conf (which itself falls back to DefaultChannel). settingChannel is
// 0 when unset in inv-driver. This keeps inv-driver as the source of truth and
// only reads the stock conf as a fallback.
func ResolveChannel(settingChannel uint32) byte {
	if settingChannel >= 11 && settingChannel <= 26 {
		return byte(settingChannel)
	}
	return ReadChannel(ChannelConfPath)
}

// Package tap encodes pcapng blocks for the ecu-zb wire tap and fans
// the live stream out to multiple consumers.
//
// Format: little-endian, link type DLT_USER0 (147), per-packet payload
// is a single direction byte (0 = host→CC2530, 1 = CC2530→host)
// followed by the raw chunk bytes — identical to what
// zigbee-tap/socat_to_pcap.py emitted, so the existing aps_zigbee.lua
// dissector decodes our output unchanged.
package tap

import (
	"bytes"
	"encoding/binary"
	"time"
)

const (
	dlTUser0 = 147

	blockTypeSHB = 0x0A0D0D0A
	blockTypeIDB = 0x00000001
	blockTypeEPB = 0x00000006

	shbByteOrderMagic = 0x1A2B3C4D

	optEndOfOpt = 0

	optShbHardware = 2
	optShbOS       = 3
	optShbUserAppl = 4

	optIfName    = 2
	optIfDescr   = 3
	optIfTsresol = 9
)

// SectionInfo carries the SHB option strings.
type SectionInfo struct {
	Hardware string
	OS       string
	UserAppl string
}

// Interface IDs used by the broadcaster. The header always emits two
// IDBs in this order so consumers see distinct interfaces in
// Wireshark's "Interface" column. Filter with
// `frame.interface_id == 1` to see only ecu-zb-injected traffic.
const (
	IfaceWire   uint32 = 0 // host ↔ CC2530 wire traffic
	IfaceInject uint32 = 1 // ecu-zb's own injected queries + their responses
)

// EncodeHeader builds the section preamble every consumer receives:
// one SHB followed by two IDBs (zb0 = wire, zb-inj0 = injected).
func EncodeHeader(s SectionInfo) []byte {
	var b bytes.Buffer
	writeSHB(&b, s)
	writeIDB(&b, "zb0", "APsystems CC2530 wire (host ↔ modem)")
	writeIDB(&b, "zb-inj0", "ecu-zb injected queries + their responses (filtered out of host stream)")
	return b.Bytes()
}

// EncodeEPB returns one Enhanced Packet Block for a chunk observed at
// ts. The first payload byte is the direction (DirToModem / DirToHost);
// the rest is the raw chunk. ifaceID selects the IDB.
func EncodeEPB(ifaceID uint32, direction byte, chunk []byte, ts time.Time) []byte {
	payload := make([]byte, 1+len(chunk))
	payload[0] = direction
	copy(payload[1:], chunk)

	micros := ts.UnixMicro()
	tsHi := uint32(uint64(micros) >> 32)
	tsLo := uint32(uint64(micros) & 0xFFFFFFFF)

	body := make([]byte, 0, 20+len(payload)+padTo4(len(payload)))
	body = appendU32(body, ifaceID)              // interface id
	body = appendU32(body, tsHi)                 // timestamp upper
	body = appendU32(body, tsLo)                 // timestamp lower
	body = appendU32(body, uint32(len(payload))) // captured len
	body = appendU32(body, uint32(len(payload))) // original len
	body = append(body, payload...)
	body = padBody(body, len(payload))
	// no options — terminate is implicit when option list is absent

	return wrapBlock(blockTypeEPB, body)
}

func writeSHB(b *bytes.Buffer, s SectionInfo) {
	body := make([]byte, 0, 64)
	body = appendU32(body, shbByteOrderMagic)
	body = appendU16(body, 1) // major
	body = appendU16(body, 0) // minor
	// section length: -1 (unknown / streaming)
	body = appendU64(body, 0xFFFFFFFFFFFFFFFF)

	body = appendStringOpt(body, optShbHardware, s.Hardware)
	body = appendStringOpt(body, optShbOS, s.OS)
	body = appendStringOpt(body, optShbUserAppl, s.UserAppl)
	body = appendEndOfOpt(body)

	b.Write(wrapBlock(blockTypeSHB, body))
}

func writeIDB(b *bytes.Buffer, name, descr string) {
	body := make([]byte, 0, 32)
	body = appendU16(body, dlTUser0) // link type
	body = appendU16(body, 0)        // reserved
	body = appendU32(body, 65535)    // snap len

	body = appendStringOpt(body, optIfName, name)
	body = appendStringOpt(body, optIfDescr, descr)
	// if_tsresol = 6 (microseconds, default but explicit doesn't hurt)
	body = appendOpt(body, optIfTsresol, []byte{6})
	body = appendEndOfOpt(body)

	b.Write(wrapBlock(blockTypeIDB, body))
}

// wrapBlock prepends the block-type / total-length header and appends
// the trailing total-length per pcapng layout.
func wrapBlock(blockType uint32, body []byte) []byte {
	total := uint32(12 + len(body))
	out := make([]byte, 0, total)
	out = appendU32(out, blockType)
	out = appendU32(out, total)
	out = append(out, body...)
	out = appendU32(out, total)
	return out
}

func appendOpt(buf []byte, code uint16, value []byte) []byte {
	buf = appendU16(buf, code)
	buf = appendU16(buf, uint16(len(value)))
	buf = append(buf, value...)
	return padBody(buf, len(value))
}

func appendStringOpt(buf []byte, code uint16, s string) []byte {
	if s == "" {
		return buf
	}
	return appendOpt(buf, code, []byte(s))
}

func appendEndOfOpt(buf []byte) []byte {
	buf = appendU16(buf, optEndOfOpt)
	buf = appendU16(buf, 0)
	return buf
}

func padBody(buf []byte, n int) []byte {
	pad := padTo4(n)
	for i := 0; i < pad; i++ {
		buf = append(buf, 0)
	}
	return buf
}

func padTo4(n int) int {
	r := n % 4
	if r == 0 {
		return 0
	}
	return 4 - r
}

func appendU16(buf []byte, v uint16) []byte {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	return append(buf, b[:]...)
}

func appendU32(buf []byte, v uint32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return append(buf, b[:]...)
}

func appendU64(buf []byte, v uint64) []byte {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	return append(buf, b[:]...)
}

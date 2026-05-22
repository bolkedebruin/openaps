package codec

import "fmt"

// ProtectionQueryFrames returns the three paged protection-param read
// queries to send to one inverter, in order (page A=0xDD, B=0xDE, C=0xD9).
// Each is the all-zero-body L2 frame; the inverter replies per page with
// the loaded grid-protection thresholds. The L1 envelope (target short
// address) is added downstream by ecu-zb.
//
// Mirrors main.exe's get_parameters_from_inverter @ 0x6462c: send the
// pages, collect the replies, then DecodeProtectionReply over them.
func ProtectionQueryFrames() [][]byte {
	return [][]byte{
		BuildL2Frame(CmdProtReadPageA, make([]byte, protQueryBodyLen)),
		BuildL2Frame(CmdProtReadPageB, make([]byte, protQueryBodyLen)),
		BuildL2Frame(CmdProtReadPageC, make([]byte, protQueryBodyLen)),
	}
}

// protQueryBodyLen is the 5-byte zero body of a read query (inner_len
// 0x06 = 1 cmd + 5 body), matching the firmware's query immediates.
const protQueryBodyLen = 5

// protReadSaneMax is the upper plausibility bound for any decoded
// protection value (volts ≤ ~600, Hz ≤ ~70, seconds ≤ ~1000, enums small);
// anything beyond it is garbage and dropped rather than published.
const protReadSaneMax = 2000

// ProtectionReading is one inverter's decoded grid-protection thresholds,
// keyed by APsystems 2-letter code in native units (V, Hz, seconds; the
// mode/PF enums as their raw index). Only codes the reply actually
// carried are present — mirroring main.exe's per-column 60code presence.
type ProtectionReading struct {
	Model  string
	Values map[string]float64
}

// protReadScale transforms a raw register to a native value.
type protReadScale func(raw int) float64

// protReadField binds a code to its byte offset (relative to the page's
// data base) and scale. The per-family field tables live in ds3.go /
// qs1a.go so offsets stay with their family.
type protReadField struct {
	code  string
	off   int
	width int // 2 or 3 bytes, big-endian
	scale protReadScale
}

// protectionPager resolves one reply frame to its field table + data
// base (the byte the offsets are relative to), or ok=false if the frame
// isn't a recognised page. Implemented per family (ds3ProtectionPage /
// qs1ProtectionPage) to keep offset knowledge in the family files.
type protectionPager func(frame []byte, cmd byte) (fields []protReadField, base int, ok bool)

// DecodeProtectionReply decodes the paged protection read replies for one
// inverter into native-unit values. frames are the per-page L2 frames
// (FB FB len cmd … FE FE) in any order. Page resolution + offsets are
// family-specific (see ds3ProtectionPage / qs1ProtectionPage). Unknown or
// short pages are skipped; returns an error only if nothing decoded.
func DecodeProtectionReply(modelCode uint8, frames [][]byte) (*ProtectionReading, error) {
	r := &ProtectionReading{Values: make(map[string]float64, 24)}
	var pager protectionPager
	switch modelCode {
	case ModelDS3, ModelDS3H, ModelDS3L, ModelExt36:
		r.Model, pager = "DS3", ds3ProtectionPage
	case ModelQS1, ModelQS1A:
		r.Model, pager = "QS1A", qs1ProtectionPage
	default:
		return nil, fmt.Errorf("%w: protection read model 0x%02X", ErrUnsupportedProtectionFamily, modelCode)
	}

	for _, f := range frames {
		l2, err := ParseL2(f)
		if err != nil {
			continue
		}
		fields, base, ok := pager(f, l2.Cmd)
		if !ok {
			continue
		}
		for _, fld := range fields {
			i := base + fld.off
			if i+fld.width > len(f) {
				continue
			}
			v := fld.scale(readBE(f, i, fld.width))
			// Drop implausible values: every protection threshold is a
			// small non-negative quantity (V/Hz/s/enum, all < ~600).
			// Guards against garbage from a misclassified, noisy, or
			// hostile reply (and a future-firmware offset mismap) reaching
			// SunSpec as a real threshold.
			if v < 0 || v > protReadSaneMax {
				continue
			}
			r.Values[fld.code] = v
		}
	}
	if len(r.Values) == 0 {
		return nil, fmt.Errorf("protection read: no fields decoded for model 0x%02X", modelCode)
	}
	return r, nil
}

// readBE reads a 2- or 3-byte big-endian register; the 2-byte form is
// signed (the firmware's VectorSignedToFloat).
func readBE(f []byte, i, width int) int {
	if width == 3 {
		return int(f[i])<<16 | int(f[i+1])<<8 | int(f[i+2])
	}
	v := int(f[i])<<8 | int(f[i+1])
	if v >= 0x8000 {
		v -= 0x10000
	}
	return v
}

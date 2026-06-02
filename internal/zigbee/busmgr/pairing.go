package busmgr

import (
	"fmt"
	"log"
	"time"

	"github.com/bolkedebruin/openaps/internal/zigbee/modem"
	"github.com/bolkedebruin/openaps/wire"
)

// PairingExecutor runs one OTA pairing primitive to completion. It owns the
// modem mutex and pauses/resumes the splice interception internally when a
// primitive parks the radio off the operating PAN (set_module_pan to a
// non-operating PAN, report_scan, commit). Implemented in main.go by an
// adapter holding the modem fd and the splice; kept as an interface so the
// busmgr package does not depend on proxy/modem wiring and stays testable.
type PairingExecutor interface {
	// SetModulePan parks the local module on pan/channel (op 0x05). The
	// caller-side decision of whether this is "off-PAN" (and thus needs the
	// splice paused) lives in the implementation.
	SetModulePan(pan uint16, channel byte) error
	// GetShortAddr returns an inverter's assigned short address (op 0x0E).
	GetShortAddr(serial string) (uint16, error)
	// SetInvPan forces one inverter onto pan/channel (op 0x0F).
	SetInvPan(serial string, pan uint16, channel byte) error
	// PrimeInv primes one inverter with the new pan/channel (op 0x11).
	PrimeInv(serial string, pan uint16, channel byte) error
	// CommitPan broadcasts the PAN commit (op 0x22, sent 3x). Parks off-PAN.
	CommitPan(pan uint16, channel byte) error
	// BindQuiet binds a short address and turns report-id off (op 0x08).
	BindQuiet(shortAddr uint16) error
	// ReportScan turns report-id on, collects announcements for the window,
	// then turns report-id off. Parks off-PAN.
	ReportScan(window time.Duration) ([]modem.FoundUnit, error)
	// GetOperatingPAN returns the PAN the radio is bonded to (set at bring-up,
	// updated on a real SetModulePan). No modem I/O. Lets inv-driver prime/
	// migrate onto the live PAN without re-deriving it from settings.
	GetOperatingPAN() uint16
	// GetOperatingChannel returns the RF channel the radio is on. Lets
	// inv-driver source the channel-migration old-channel from the radio owner
	// rather than from settings.
	GetOperatingChannel() byte
}

// defaultScanWindow bounds a report_scan that arrives with no timeout.
const defaultScanWindow = 8 * time.Second

// ZigBee 2.4 GHz channel bounds (IEEE 802.15.4 channels 11-26). inv-driver
// already validates the channel, but this is a defensive last line before
// the uint32->byte cast so a truncated/out-of-range channel can never reach
// the wire.
const (
	minChannel uint32 = 11
	maxChannel uint32 = 26
)

// checkChannel rejects a nonzero channel outside the usable 2.4 GHz range
// (below 11 or above 26). It guards the byte() casts in handlePairingCmd
// against uint32 truncation reaching the wire.
func checkChannel(ch uint32) error {
	if ch != 0 && (ch < minChannel || ch > maxChannel) {
		return fmt.Errorf("channel %d out of range (want %d-%d)", ch, minChannel, maxChannel)
	}
	return nil
}

// handlePairingCmd runs one PairingCmd primitive and enqueues a
// PairingCmdResult upstream correlated by req_id. Each primitive runs to
// completion in this goroutine (the reader loop) — the primitives are short
// and serialised by inv-driver's global pairing lock, and running inline
// keeps the modem exchange ordered with respect to other inbound commands.
func (c *Client) handlePairingCmd(cmd *wire.PairingCmd) {
	reqID := cmd.GetReqId()
	if c.Pairing == nil {
		c.replyPairingErr(reqID, "pairing executor not configured on this bus backend")
		return
	}

	res := &wire.PairingCmdResult{ReqId: reqID, Ok: true}

	switch op := cmd.GetOp().(type) {
	case *wire.PairingCmd_SetModulePan:
		if err := checkChannel(op.SetModulePan.GetChannel()); err != nil {
			c.finishPairing(res, err)
			return
		}
		err := c.Pairing.SetModulePan(uint16(op.SetModulePan.GetPan()), byte(op.SetModulePan.GetChannel()))
		c.finishPairing(res, err)

	case *wire.PairingCmd_ReportScan:
		window := time.Duration(op.ReportScan.GetTimeoutMs()) * time.Millisecond
		if window <= 0 {
			window = defaultScanWindow
		}
		found, err := c.Pairing.ReportScan(window)
		if err == nil {
			for _, u := range found {
				res.Found = append(res.Found, &wire.FoundInverter{
					Serial:    u.Serial,
					Encrypted: u.Encrypted,
				})
			}
		}
		c.finishPairing(res, err)

	case *wire.PairingCmd_GetShortAddr:
		sa, err := c.Pairing.GetShortAddr(op.GetShortAddr.GetSerial())
		if err == nil {
			res.ShortAddr = uint32(sa)
		}
		c.finishPairing(res, err)

	case *wire.PairingCmd_SetInvPan:
		m := op.SetInvPan
		if err := checkChannel(m.GetChannel()); err != nil {
			c.finishPairing(res, err)
			return
		}
		err := c.Pairing.SetInvPan(m.GetSerial(), uint16(m.GetPan()), byte(m.GetChannel()))
		c.finishPairing(res, err)

	case *wire.PairingCmd_PrimeInv:
		m := op.PrimeInv
		if err := checkChannel(m.GetChannel()); err != nil {
			c.finishPairing(res, err)
			return
		}
		err := c.Pairing.PrimeInv(m.GetSerial(), uint16(m.GetPan()), byte(m.GetChannel()))
		c.finishPairing(res, err)

	case *wire.PairingCmd_CommitPan:
		m := op.CommitPan
		if err := checkChannel(m.GetChannel()); err != nil {
			c.finishPairing(res, err)
			return
		}
		err := c.Pairing.CommitPan(uint16(m.GetPan()), byte(m.GetChannel()))
		c.finishPairing(res, err)

	case *wire.PairingCmd_BindQuiet:
		err := c.Pairing.BindQuiet(uint16(op.BindQuiet.GetShortAddr()))
		c.finishPairing(res, err)

	case *wire.PairingCmd_GetModulePan:
		res.Pan = uint32(c.Pairing.GetOperatingPAN())
		res.Channel = uint32(c.Pairing.GetOperatingChannel())
		c.finishPairing(res, nil)

	default:
		c.replyPairingErr(reqID, fmt.Sprintf("unknown or empty pairing op (req_id=%d)", reqID))
		return
	}
}

// finishPairing stamps an error (if any) onto res and enqueues it.
func (c *Client) finishPairing(res *wire.PairingCmdResult, err error) {
	if err != nil {
		res.Ok = false
		res.Error = err.Error()
		log.Printf("pairing: req_id=%d failed: %v", res.ReqId, err)
	}
	c.Enqueue(&wire.Envelope{Body: &wire.Envelope_PairingResult{PairingResult: res}})
}

func (c *Client) replyPairingErr(reqID uint64, msg string) {
	log.Printf("pairing: req_id=%d rejected: %s", reqID, msg)
	c.Enqueue(&wire.Envelope{Body: &wire.Envelope_PairingResult{
		PairingResult: &wire.PairingCmdResult{ReqId: reqID, Ok: false, Error: msg},
	}})
}

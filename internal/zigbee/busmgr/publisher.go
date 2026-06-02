package busmgr

import (
	"encoding/hex"
	"time"

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/internal/zigbee/inventory"
	"github.com/bolkedebruin/openaps/wire"
)

// Publisher wraps a Client and converts ecu-zb-side events into wire
// envelopes for the inv-driver UDS.
type Publisher struct {
	C *Client
	// Inventory, when non-nil, records the (peer_uid, short-addr)
	// pair carried in every observed L1 reply. The downstream Send
	// path resolves peer_uid -> short-address through the same map
	// when wrapping outbound L2 in an L1 envelope.
	Inventory *inventory.Map
}

// RawFrame enqueues a reassembled L1 reply for the given short address.
// The input slice is copied: BBPollerHook reuses its reassembly buffer.
// As a side effect, the peer UID embedded in the L1 envelope is added
// to the inventory cross-index so subsequent Send envelopes can address
// this inverter by UID.
func (p *Publisher) RawFrame(shortAddr uint16, l1 []byte) {
	if p == nil || p.C == nil {
		return
	}
	cp := append([]byte(nil), l1...)
	if p.Inventory != nil {
		if uid, ok := peerUIDFromL1Reply(cp); ok {
			p.Inventory.Record(uid, shortAddr)
		}
	}
	p.C.Enqueue(&wire.Envelope{Body: &wire.Envelope_RawFrame{RawFrame: &wire.RawFrame{
		TsMs:      time.Now().UnixMilli(),
		ShortAddr: uint32(shortAddr),
		L1Frame:   cp,
	}}})
}

// peerUIDFromL1Reply extracts the 12-hex peer UID from a reassembled
// L1 reply of the form FC FC <SA hi> <SA lo> <RSSI> <LQI> <UID 6> ...
// Returns (uid, true) on success, ("", false) if the buffer is too
// short or the SOF bytes don't match.
func peerUIDFromL1Reply(l1 []byte) (string, bool) {
	if len(l1) < 12 {
		return "", false
	}
	if l1[0] != codec.L1ReplySOF || l1[1] != codec.L1ReplySOF {
		return "", false
	}
	return hex.EncodeToString(l1[6:12]), true
}

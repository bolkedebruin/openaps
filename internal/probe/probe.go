// Package probe runs cold-start identity queries against inverters
// the driver has seen but doesn't yet have an identity payload for
// (model_code IS NULL). Replies flow back upstream as RawFrame
// envelopes; the existing ingest path matches them against
// codec.MatchOutboundInfoQuery and routes the decoded InfoReply.
package probe

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
)

// Sender abstracts the publisher dispatch path so the probe can be
// unit-tested without spinning up a UDS server.
type Sender interface {
	SendToBackend(backend string, env *wire.Envelope) bool
}

// Probe is a one-shot iterator that queries every inverter row with
// no known model_code. Re-runs are safe: rows that already have
// model_code are skipped, so a probe that races a real reply will
// quietly do nothing on the next attach.
type Probe struct {
	Store   *store.Store
	Server  Sender
	Backend string
	// SendInterval rate-limits per-inverter dispatch. Defaults to
	// 200ms when zero.
	SendInterval time.Duration
	// MaxRows caps a single probe pass. Defaults to 100 when zero.
	MaxRows int
}

// Run iterates inverters with model_code IS NULL and dispatches one
// 0xDC query Send envelope per row. Rows are ordered by uid so a
// reattach probes in a deterministic order. Returns the count of
// envelopes successfully enqueued (i.e. Sender.SendToBackend == true).
func (p *Probe) Run(ctx context.Context) (int, error) {
	if p.Store == nil || p.Server == nil || p.Backend == "" {
		return 0, fmt.Errorf("probe: Store, Server, and Backend must be set")
	}
	interval := p.SendInterval
	if interval == 0 {
		interval = 200 * time.Millisecond
	}
	limit := p.MaxRows
	if limit == 0 {
		limit = 100
	}

	rows, err := p.Store.DB().QueryContext(ctx, `
SELECT uid, short_addr
FROM   inverters
WHERE  model_code IS NULL AND short_addr IS NOT NULL
ORDER  BY uid ASC
LIMIT  ?`, limit)
	if err != nil {
		return 0, fmt.Errorf("probe: query: %w", err)
	}
	defer rows.Close()

	type target struct {
		uid string
		sa  uint16
	}
	var targets []target
	for rows.Next() {
		var uid string
		var sa sql.NullInt64
		if err := rows.Scan(&uid, &sa); err != nil {
			return 0, fmt.Errorf("probe: scan: %w", err)
		}
		if !sa.Valid {
			continue
		}
		targets = append(targets, target{uid: uid, sa: uint16(sa.Int64)})
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("probe: rows: %w", err)
	}
	rows.Close()

	if len(targets) == 0 {
		return 0, nil
	}
	log.Printf("probe: dispatching %d info-query frame(s) to backend=%q", len(targets), p.Backend)

	sent := 0
	for i, tgt := range targets {
		select {
		case <-ctx.Done():
			return sent, ctx.Err()
		default:
		}

		env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
			PeerUid: tgt.uid,
			Frame:   buildInfoQueryFrame(tgt.sa),
		}}}
		if ok := p.Server.SendToBackend(p.Backend, env); ok {
			sent++
		} else {
			// Publisher disappeared or queue full. Stop early; the
			// next attach will re-run the probe for whatever still
			// lacks model_code.
			log.Printf("probe: backend=%q send refused at uid=%s; aborting pass", p.Backend, tgt.uid)
			return sent, nil
		}

		if i < len(targets)-1 {
			select {
			case <-ctx.Done():
				return sent, ctx.Err()
			case <-time.After(interval):
			}
		}
	}
	return sent, nil
}

// buildInfoQueryFrame returns the 28-byte L0+L2 info-query frame for
// the given inverter short-address. Same outer L0 shape as the BB
// telemetry query (AA AA AA AA 55 SA-hi SA-lo 00 00 00 00 00 crc-hi
// crc-lo 0D); the inner L2 is the canonical 0xDC body
// (FB FB 06 DC 00 00 00 00 00 00 E2 FE FE).
func buildInfoQueryFrame(sa uint16) []byte {
	saHi := byte(sa >> 8)
	saLo := byte(sa & 0xFF)
	chk := uint16(0x55) + uint16(saHi) + uint16(saLo)
	chkHi := byte(chk >> 8)
	chkLo := byte(chk & 0xFF)
	return []byte{
		0xAA, 0xAA, 0xAA, 0xAA,
		0x55,
		saHi, saLo,
		0x00, 0x00, 0x00, 0x00, 0x00,
		chkHi, chkLo,
		0x0D,
		// L2 0xDC query (codec.outboundInfoQueryL2).
		0xFB, 0xFB,
		0x06, 0xDC,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xE2,
		0xFE, 0xFE,
	}
}

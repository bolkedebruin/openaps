// Package ingest translates wire.Envelope events into state-store
// writes. It is the only path data takes from the UDS layer into
// SQLite, and the only place v0 contains business logic — everything
// else is plumbing.
package ingest

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/internal/events"
	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
)

// Ingestor owns the store handle and turns wire events into rows.
// One Ingestor is shared by all connection goroutines; the underlying
// store handles its own write serialisation.
//
// Pub, when non-nil, receives a fan-out copy of every successfully
// decoded telemetry envelope (RawFrame after decode, or a legacy
// Telemetry frame as-received). Per-event publish happens after the
// state-store writes so subscribers never see a row the daemon failed
// to persist.
type Ingestor struct {
	S   *store.Store
	Pub *events.Publisher
}

// Handle dispatches an Envelope by oneof body. Returns an error only
// on hard malformed input; transient DB errors are returned so the
// connection layer can decide whether to drop the peer.
func (in *Ingestor) Handle(ctx context.Context, backend string, env *wire.Envelope) error {
	switch b := env.GetBody().(type) {
	case *wire.Envelope_Hello:
		if b.Hello == nil {
			return fmt.Errorf("hello envelope without body")
		}
		log.Printf("ingest hello: backend=%q version=%q hostname=%q role=%s",
			b.Hello.GetBackend(), b.Hello.GetVersion(), b.Hello.GetHostname(), b.Hello.GetRole().String())
		return nil
	case *wire.Envelope_Telemetry:
		if b.Telemetry == nil {
			return fmt.Errorf("telemetry envelope without body")
		}
		if err := in.handleTelemetry(ctx, b.Telemetry); err != nil {
			return err
		}
		in.publish(env)
		return nil
	case *wire.Envelope_RawFrame:
		if b.RawFrame == nil {
			return fmt.Errorf("raw_frame envelope without body")
		}
		return in.handleRawFrame(ctx, b.RawFrame)
	case *wire.Envelope_Info:
		if b.Info == nil {
			return fmt.Errorf("inverter_info envelope without body")
		}
		if err := in.handleInverterInfo(ctx, b.Info); err != nil {
			return err
		}
		in.publish(env)
		return nil
	case *wire.Envelope_DecodeFailed:
		if b.DecodeFailed == nil {
			return fmt.Errorf("decode_failed envelope without body")
		}
		df := b.DecodeFailed
		return in.S.AppendDecodeFailed(ctx, df.GetTsMs(), df.GetShortAddr(), df.GetError(), df.GetRawHex())
	case nil:
		return fmt.Errorf("envelope without body")
	default:
		return fmt.Errorf("unhandled envelope body type %T", b)
	}
}

func (in *Ingestor) handleTelemetry(ctx context.Context, t *wire.Telemetry) error {
	if t.GetPeerUid() == "" {
		return fmt.Errorf("telemetry: empty peer_uid")
	}
	fam := familyFromModel(t.GetModel())
	// short_addr on the wire is uint32 (proto has no uint16); narrow
	// at the store boundary which retains the uint16 column type.
	shortAddr := uint16(t.GetShortAddr())
	uid := t.GetPeerUid()
	tsMs := t.GetTsMs()
	if err := in.S.UpsertInverterFromTelemetry(ctx, uid, shortAddr, fam, t.GetModel(), tsMs); err != nil {
		return err
	}
	if err := in.S.WriteTelemetryLive(ctx, uid, tsMs, t.GetCmd(),
		t.GetActivePowerW(), t.GetGridV(), t.GetFreqHz(), t.GetBusV(), t.GetReportSec()); err != nil {
		return err
	}
	if pp := t.GetPanels(); len(pp) > 0 {
		rows := make([]store.PanelRow, 0, len(pp))
		for _, p := range pp {
			rows = append(rows, store.PanelRow{
				ChannelIdx: int(p.GetIndex()),
				DCV:        p.GetDcV(),
				DCI:        p.GetDcI(),
				W:          p.GetW(),
			})
		}
		if err := in.S.WritePanels(ctx, uid, tsMs, rows); err != nil {
			return err
		}
	}
	if raw := t.GetLifetimeRaw(); len(raw) > 0 {
		scale := t.GetLifetimeScale()
		rows := make([]store.EnergyRow, 0, len(raw))
		for i, r := range raw {
			rows = append(rows, store.EnergyRow{
				ChannelIdx: i,
				Raw:        r,
				Scale:      scale,
			})
		}
		if err := in.S.WriteEnergyLifetime(ctx, uid, tsMs, rows); err != nil {
			return err
		}
	}
	// Light-touch audit row.
	return in.S.AppendEvent(ctx, tsMs, uid, "telemetry", "info")
}

// handleInverterInfo stores the identity / pair-state columns and
// appends an inverter_info audit row. Optional proto3 fields map to
// pointer-typed store columns so an unset field leaves the prior
// column value untouched.
func (in *Ingestor) handleInverterInfo(ctx context.Context, info *wire.InverterInfo) error {
	uid := info.GetPeerUid()
	if uid == "" {
		return fmt.Errorf("inverter_info: empty peer_uid")
	}
	tsMs := info.GetTsMs()
	upd := store.InverterInfoUpdate{
		UID:         uid,
		TsMs:        tsMs,
		ShortAddr:   uint16(info.GetShortAddr()),
		Model:       info.ModelCode,
		SoftwareVer: info.SoftwareVersion,
		Phase:       info.Phase,
		Bound:       info.ZigbeeBound,
		RptOff:      info.TurnedOffRpt,
	}
	if err := in.S.UpsertInverterInfo(ctx, upd); err != nil {
		return err
	}
	return in.S.AppendEvent(ctx, tsMs, uid, "inverter_info", "info")
}

// handleRawFrame decodes the L1 reply into a wire.Telemetry, runs the
// same store writes as the Telemetry path, and publishes a Telemetry
// envelope so subscribers see the decoded view regardless of which
// side decoded the frame. A decode failure is captured as a
// decode_failed event and is NOT published.
func (in *Ingestor) handleRawFrame(ctx context.Context, rf *wire.RawFrame) error {
	raw := rf.GetL1Frame()
	tsMs := rf.GetTsMs()
	if tsMs == 0 {
		tsMs = time.Now().UnixMilli()
	}
	rep, err := codec.DecodeReply(raw)
	if err != nil {
		return in.S.AppendDecodeFailed(ctx, tsMs, rf.GetShortAddr(), err.Error(), truncHex(raw))
	}
	t := telemetryFromReply(rep, tsMs)
	if err := in.handleTelemetry(ctx, t); err != nil {
		return err
	}
	in.publish(&wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: t}})
	return nil
}

// telemetryFromReply maps every codec.Reply field onto wire.Telemetry.
func telemetryFromReply(r codec.Reply, tsMs int64) *wire.Telemetry {
	t := &wire.Telemetry{
		TsMs:          tsMs,
		ShortAddr:     uint32(r.ShortAddr),
		PeerUid:       r.PeerUID,
		Cmd:           uint32(r.Cmd),
		Model:         r.Model,
		GridV:         r.GridV,
		BusV:          r.BusV,
		FreqHz:        r.FreqHz,
		ReportSec:     r.ReportSec,
		ActivePowerW:  r.ActivePowerW,
		ReactiveVar:   r.ReactivePower,
		LifetimeRaw:   r.LifetimeRaw,
		LifetimeScale: r.LifetimeScale,
	}
	if len(r.Panels) > 0 {
		t.Panels = make([]*wire.Panel, len(r.Panels))
		for i, pn := range r.Panels {
			t.Panels[i] = &wire.Panel{
				Index: int32(pn.Index),
				DcV:   pn.DCV,
				DcI:   pn.DCI,
				W:     pn.W,
			}
		}
	}
	return t
}

func (in *Ingestor) publish(env *wire.Envelope) {
	if in.Pub == nil || env == nil {
		return
	}
	in.Pub.Publish(env)
}

// truncHex caps the raw_hex column at 256 bytes (512 hex chars) so a
// malformed frame doesn't bloat the events table.
func truncHex(b []byte) string {
	const cap = 256
	if len(b) > cap {
		b = b[:cap]
	}
	return hex.EncodeToString(b)
}

// familyFromModel maps the human Model string to the lowercase family
// key the eventual capability table will use. "unknown(0xNN)" → "".
func familyFromModel(model string) string {
	switch {
	case strings.HasPrefix(model, "QS1A"):
		return "qs1a"
	case strings.HasPrefix(model, "QS1"):
		return "qs1"
	case strings.HasPrefix(model, "DS3"):
		return "ds3"
	case strings.HasPrefix(model, "DSP"):
		return "dsp"
	case strings.HasPrefix(model, "YC600"):
		return "yc600"
	case strings.HasPrefix(model, "YC1000"):
		return "yc1000"
	default:
		return ""
	}
}

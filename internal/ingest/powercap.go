package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/wire"
)

const (
	// capDriftToleranceW absorbs the commanded-vs-readback quantisation of
	// the DA output cap (well under 1 W for both DS3 and QS1).
	capDriftToleranceW = 2.0
	// capReassertMinInterval bounds how often a drifted cap is re-sent to
	// one inverter, so an inverter that refuses the cap isn't hammered.
	capReassertMinInterval = 10 * time.Minute
	// capReassertBy attributes re-assert events and adopted intents.
	capReassertBy = "inv-driver"
)

// recordCapIntent stores the per-panel cap carried by a routed set-power
// Send (for its peer) or Broadcast (for every inverter of the addressed
// family) as the cap those inverters are meant to hold.
func (in *Ingestor) recordCapIntent(env *wire.Envelope, sender string) {
	if in.S == nil {
		return
	}
	var frame []byte
	var uids []string
	if s := env.GetSend(); s != nil {
		frame, uids = s.GetFrame(), []string{s.GetPeerUid()}
	} else if b := env.GetBroadcast(); b != nil {
		frame = b.GetFrame()
	} else {
		return
	}
	fam, panelW, broadcast, ok := codec.DecodeSetPower(frame)
	if !ok {
		return
	}
	ctx := context.Background()
	if broadcast {
		var err error
		if uids, err = in.S.InverterUIDsInFamily(ctx, fam); err != nil {
			slog.Warn("power cap intent: list family", "family", fam.String(), "err", err)
			return
		}
	}
	w := int(math.Round(panelW))
	now := time.Now().UnixMilli()
	for _, uid := range uids {
		if uid == "" {
			continue
		}
		if err := in.S.SetPowerCapIntent(ctx, uid, w, now, sender); err != nil {
			slog.Warn("power cap intent: record", "uid", uid, "err", err)
		}
	}
}

// checkCapDrift compares a protection page's DA read-back against the cap
// uid is meant to hold and re-sends that cap when they differ. With no cap
// recorded yet, the first in-range read-back becomes the intent, so a cap
// already in place is kept rather than overwritten.
func (in *Ingestor) checkCapDrift(ctx context.Context, uid string, model uint8, page map[string]float64) {
	da, ok := page["DA"]
	if !ok || in.S == nil {
		return
	}
	intent, found, err := in.S.PowerCapIntent(ctx, uid)
	if err != nil {
		slog.Warn("power cap intent: read", "uid", uid, "err", err)
		return
	}
	if !found {
		w := int(math.Round(da))
		if w < int(codec.MinPanelLimitW) || w > int(codec.MaxPanelLimitW) {
			return
		}
		if err := in.S.SetPowerCapIntent(ctx, uid, w, time.Now().UnixMilli(), capReassertBy); err != nil {
			slog.Warn("power cap intent: adopt", "uid", uid, "err", err)
		}
		return
	}
	if math.Abs(da-float64(intent)) <= capDriftToleranceW {
		return
	}
	if in.Router == nil || in.RouteBackend == "" {
		return
	}
	frame, err := codec.EncodeSetPower(model, uint16(intent), false)
	if err != nil {
		slog.Warn("power cap re-assert: encode", "uid", uid, "intent_w", intent, "err", err)
		return
	}
	now := time.Now()
	in.protMu.Lock()
	if last, ok := in.capReassertAt[uid]; ok && now.Sub(last) < capReassertMinInterval {
		in.protMu.Unlock()
		return
	}
	if in.capReassertAt == nil {
		in.capReassertAt = make(map[string]time.Time)
	}
	in.capReassertAt[uid] = now
	in.protMu.Unlock()

	env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{PeerUid: uid, Frame: frame}}}
	if !in.Router.SendToBackend(in.RouteBackend, env) {
		slog.Warn("power cap re-assert: route failed", "uid", uid, "backend", in.RouteBackend)
		return
	}
	slog.Warn("power cap drifted; re-asserting", "uid", uid, "readback_w", da, "intent_w", intent)
	_ = in.S.AppendEvent(ctx, now.UnixMilli(), uid, "power_cap_reasserted", "warn", capReassertBy,
		fmt.Sprintf("readback=%.4g W/panel intent=%d W/panel", da, intent))
	in.maybeReadAfterWrite(env)
}

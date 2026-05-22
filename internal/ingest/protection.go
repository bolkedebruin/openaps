package ingest

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/wire"
)

// handleProtectionPage buffers one paged protection reply per inverter,
// re-decodes the per-UID page set, and caches + publishes the merged
// grid-protection state. Pages arrive as separate replies (0xDD/0xDE/
// 0xD9), so the buffer accumulates the latest of each before decoding.
func (in *Ingestor) handleProtectionPage(ctx context.Context, tsMs int64, env codec.L1Envelope, model uint8) {
	uid := env.PeerUIDString()
	if uid == "" || len(env.L2Frame) < 5 {
		return
	}
	// Page id: DS3 tags every page 0xDD with the selector at byte[4];
	// QS1 uses the page byte as the cmd at byte[3].
	pageKey := env.L2Frame[3]
	if model == codec.ModelDS3 {
		pageKey = env.L2Frame[4]
	}
	frame := append([]byte(nil), env.L2Frame...)

	in.protMu.Lock()
	if in.protPages == nil {
		in.protPages = make(map[string]map[byte][]byte)
	}
	if in.protPages[uid] == nil {
		in.protPages[uid] = make(map[byte][]byte, 3)
	}
	in.protPages[uid][pageKey] = frame
	frames := make([][]byte, 0, len(in.protPages[uid]))
	for _, f := range in.protPages[uid] {
		frames = append(frames, f)
	}
	in.protMu.Unlock()

	reading, err := codec.DecodeProtectionReply(model, frames)
	if err != nil {
		return
	}
	p := readingToProto(uid, tsMs, reading)
	in.cacheProtection(p)
	in.publish(&wire.Envelope{Body: &wire.Envelope_Protection{Protection: p}})

	// Log only when the decoded set changes, so a steady poll is quiet.
	sig := protSignature(reading)
	in.protMu.Lock()
	if in.protSig == nil {
		in.protSig = make(map[string]string)
	}
	changed := in.protSig[uid] != sig
	in.protSig[uid] = sig
	in.protMu.Unlock()
	if changed {
		log.Printf("protection uid=%s model=%s %d fields: %s", uid, reading.Model, len(reading.Values), sig)
	}
	_ = ctx
}

// protSignature is a stable, human-readable digest of a reading's values
// (sorted code=value), used to log protection state only on change.
func protSignature(r *codec.ProtectionReading) string {
	codes := make([]string, 0, len(r.Values))
	for c := range r.Values {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	var b strings.Builder
	for _, c := range codes {
		fmt.Fprintf(&b, "%s=%.4g ", c, r.Values[c])
	}
	return strings.TrimSpace(b.String())
}

// readingToProto maps a decoded ProtectionReading to the wire envelope.
// Each present code sets its optional proto field (absence = not reported).
func readingToProto(uid string, tsMs int64, r *codec.ProtectionReading) *wire.Protection {
	p := &wire.Protection{PeerUid: uid, TsMs: tsMs, Model: r.Model}
	for code, val := range r.Values {
		v := val
		switch code {
		case "AC":
			p.UvStg2 = &v
		case "AD":
			p.OvStg2 = &v
		case "AQ":
			p.UvFast = &v
		case "AY":
			p.OvStg3 = &v
		case "AB":
			p.AvgOv = &v
		case "AH":
			p.VwinLow = &v
		case "AI":
			p.VwinHi = &v
		case "AE":
			p.UfSlow = &v
		case "AF":
			p.OfSlow = &v
		case "AJ":
			p.UfFast = &v
		case "AK":
			p.OfFast = &v
		case "AG":
			p.ReconnectS = &v
		case "AS":
			p.StartS = &v
		case "BN":
			p.ReconnVLow = &v
		case "BO":
			p.ReconnVHi = &v
		case "BP":
			p.ReconnFLow = &v
		case "BQ":
			p.ReconnFHi = &v
		case "BB":
			p.Uv2ClrS = &v
		case "BC":
			p.Ov2ClrS = &v
		case "BD":
			p.Uv3ClrS = &v
		case "BE":
			p.Ov3ClrS = &v
		case "BH":
			p.Uf1ClrS = &v
		case "BI":
			p.Of1ClrS = &v
		case "BJ":
			p.Uf2ClrS = &v
		case "BK":
			p.Of2ClrS = &v
		case "CH":
			p.PfMode = &v
		case "DC":
			p.OfDroopStart = &v
		case "CC":
			p.OfDroopEnd = &v
		case "DD":
			p.OfDroopSlope = &v
		case "CV":
			p.OfDroopMode = &v
		case "DH":
			p.OfCurveUfLow = &v
		case "DI":
			p.OfCurveUfHi = &v
		case "CB":
			p.OfCurveOfLow = &v
		}
	}
	return p
}

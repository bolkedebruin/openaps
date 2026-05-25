package ingest

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/wire"
)

const (
	// protReadAfterWriteDelay lets a protection write land on the inverter
	// before the read-back query is sent.
	protReadAfterWriteDelay = 4 * time.Second
	// protReadFrameSpacing spaces the three page queries; protReadUIDSpacing
	// spaces successive inverters. Without spacing, concurrent reads make
	// ecu-zb's single-frame reassembly cross-talk between inverters.
	protReadFrameSpacing = 300 * time.Millisecond
	protReadUIDSpacing   = 500 * time.Millisecond
	protReadQueueCap     = 16
	// protReadAttempts re-sends the page set per read; pages accumulate in
	// the buffer, so a second pass fills any page whose reply was lost on
	// the radio (on-demand has no continuous-poll redundancy). main.exe
	// retries each page up to 3×.
	protReadAttempts = 2
)

// triggerProtectionRead enqueues a one-shot, three-page read for one
// inverter. A single worker drains the queue so reads are serialised and
// spaced on the wire (reads are on-demand — first-seen + after a
// protection write — so there is no continuous poll). No-op without a
// router/backend.
func (in *Ingestor) triggerProtectionRead(uid string) {
	if in.Router == nil || in.RouteBackend == "" || uid == "" {
		return
	}
	in.protReadOnce.Do(func() {
		in.protReadQueue = make(chan string, protReadQueueCap)
		go in.protReadWorker()
	})
	select {
	case in.protReadQueue <- uid:
	default: // a read is already queued for this/another uid; drop (idempotent)
	}
}

// protReadWorker drains the read queue one inverter at a time, sending
// the three page queries spaced so their replies don't interleave.
func (in *Ingestor) protReadWorker() {
	for uid := range in.protReadQueue {
		for attempt := 0; attempt < protReadAttempts; attempt++ {
			for _, frame := range codec.ProtectionQueryFrames() {
				env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{PeerUid: uid, Frame: frame}}}
				in.Router.SendToBackend(in.RouteBackend, env)
				time.Sleep(protReadFrameSpacing)
			}
		}
		time.Sleep(protReadUIDSpacing)
	}
}

// cacheProtectionValues stores a copy of the native-unit value map from
// a decoded protection reading. Used by ReadbackNative to serve the
// reconciler without re-decoding the proto.  Each update increments the
// per-UID sequence so callers can detect a stale cache.
func (in *Ingestor) cacheProtectionValues(uid string, values map[string]float64) {
	if uid == "" || len(values) == 0 {
		return
	}
	cp := make(map[string]float64, len(values))
	for k, v := range values {
		cp[k] = v
	}
	in.protMu.Lock()
	if in.protValues == nil {
		in.protValues = make(map[string]map[string]float64)
	}
	in.protValues[uid] = cp
	if in.protValueSeq == nil {
		in.protValueSeq = make(map[string]uint64)
	}
	in.protValueSeq[uid]++
	in.protMu.Unlock()
}

// ReadbackSeq returns the current cache sequence for uid.  The sequence
// is incremented each time a protection read-back is stored.  A caller
// can compare seq before a send versus seq after the settle window; if
// they are equal the read-back has not refreshed and a confirmation would
// be based on stale data.  Returns 0 when no read has been cached yet.
func (in *Ingestor) ReadbackSeq(uid string) uint64 {
	in.protMu.RLock()
	defer in.protMu.RUnlock()
	if in.protValueSeq == nil {
		return 0
	}
	return in.protValueSeq[uid]
}

// ReadbackNative returns the latest decoded protection values for uid,
// keyed by APsystems 2-letter code in native units (V, Hz, s).
// ok=false if no protection read has completed for this UID yet.
func (in *Ingestor) ReadbackNative(uid string) (map[string]float64, bool) {
	in.protMu.RLock()
	defer in.protMu.RUnlock()
	if in.protValues == nil {
		return nil, false
	}
	v, ok := in.protValues[uid]
	if !ok || len(v) == 0 {
		return nil, false
	}
	cp := make(map[string]float64, len(v))
	for k, val := range v {
		cp[k] = val
	}
	return cp, true
}

// TriggerRead asks the read pipeline to (re)read uid's protection.
// This is a thin public alias for triggerProtectionRead so external
// callers (e.g. the gridprofile reconciler adapter) can request reads
// without reaching into unexported ingest internals.
func (in *Ingestor) TriggerRead(uid string) {
	in.triggerProtectionRead(uid)
}

// noteProtFamily records an inverter's family (from telemetry) so a
// protection reply whose inferred family contradicts it can be rejected.
func (in *Ingestor) noteProtFamily(uid, model string) {
	if uid == "" {
		return
	}
	in.protMu.Lock()
	if in.protFamily == nil {
		in.protFamily = make(map[string]bool)
	}
	in.protFamily[uid] = model == "DS3"
	in.protMu.Unlock()
}

// protReadOnFirstSeen issues a one-shot startup protection read the first
// time an inverter's telemetry is seen, so the cache populates without a
// continuous poll.  When OnFirstSeen is set it is also called in a new
// goroutine after enqueueing the read, so the gridprofile reconciler can
// run VerifyStartup once the inverter is known.
// protReseenGap is how long telemetry must be absent before a UID's return is
// treated as a reconnect (re-arming the startup read + reconcile). Normal
// telemetry is ~1s; an inverter offline overnight is hours.
const protReseenGap = 5 * time.Minute

func (in *Ingestor) protReadOnFirstSeen(uid string) {
	now := time.Now()
	in.protMu.Lock()
	if in.protSeen == nil {
		in.protSeen = make(map[string]bool)
	}
	if in.protLastSeen == nil {
		in.protLastSeen = make(map[string]time.Time)
	}
	last := in.protLastSeen[uid]
	reseen := !last.IsZero() && now.Sub(last) > protReseenGap
	first := !in.protSeen[uid] || reseen
	in.protSeen[uid] = true
	in.protLastSeen[uid] = now
	in.protMu.Unlock()
	if first {
		go in.triggerProtectionRead(uid)
		if hook := in.OnFirstSeen; hook != nil {
			go hook(uid)
		}
	}
}

// maybeReadAfterWrite schedules a protection read-back after a routed
// protection write lands, so the published thresholds reflect the change
// without polling.
func (in *Ingestor) maybeReadAfterWrite(env *wire.Envelope) {
	s := env.GetSend()
	if s == nil || !codec.IsProtectionWrite(s.GetFrame()) {
		return
	}
	uid := s.GetPeerUid()
	go func() {
		time.Sleep(protReadAfterWriteDelay)
		in.triggerProtectionRead(uid)
	}()
}

// handleProtectionPage buffers one paged protection reply per inverter,
// re-decodes the per-UID page set, and caches + publishes the merged
// grid-protection state. Pages arrive as separate replies (0xDD/0xDE/
// 0xD9), so the buffer accumulates the latest of each before decoding.
func (in *Ingestor) handleProtectionPage(ctx context.Context, tsMs int64, env codec.L1Envelope, model uint8) {
	uid := env.PeerUIDString()
	if uid == "" || len(env.L2Frame) < 5 {
		return
	}
	// Reject a reply whose inferred family contradicts the inverter's
	// known model (from telemetry) — a sign of reply cross-talk on the
	// modem; decoding it with the wrong family yields garbage.
	in.protMu.RLock()
	knownDS3, known := in.protFamily[uid]
	in.protMu.RUnlock()
	if known && knownDS3 != (model == codec.ModelDS3) {
		log.Printf("protection: dropping reply for uid=%s — inferred family != known model (cross-talk?)", uid)
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
	in.cacheProtectionValues(uid, reading.Values)
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

// protCodesUnpublished lists protection codes the decoder produces but
// readingToProto deliberately does NOT map onto the wire.Protection message:
// they have no SunSpec model (707-711) register to carry them. They are
// vendor extras or duplicates of codes already published — see each reason.
// They remain fully observable via the "protection uid=… N fields" log line
// and via ReadbackNative (which the gridprofile reconciler/verify consume),
// so this is a SunSpec-publish scoping choice, not data loss.
//
// TestReadingToProto_NoSilentDrop enforces that every code the codec can
// decode is EITHER published below OR listed here — so a newly-decoded code
// can never be dropped silently; it must be mapped or explicitly classified.
var protCodesUnpublished = map[string]string{
	"AT": "page-A protection stage/enable bitfield; no SunSpec register",
	"AU": "page-A protection stage/enable bitfield; no SunSpec register",
	"AV": "page-A protection stage/enable bitfield; no SunSpec register",
	"AW": "page-A protection stage/enable bitfield; no SunSpec register",
	"AX": "page-A identity scalar; semantic unconfirmed, no SunSpec register",
	"AZ": "QS1A second-stage under-volt (duplicates the AC band); telemetry-only readback, no SunSpec register",
	"BA": "QS1A second-stage over-volt (duplicates the AY band); telemetry-only readback, no SunSpec register",
	"BF": "QS1A extra clear-time register (non-EN code); no SunSpec register",
	"BG": "QS1A extra clear-time register (non-EN code); no SunSpec register",
	"BL": "QS1A extra time register; no SunSpec register",
	"BM": "QS1A extra time register; no SunSpec register",
}

// readingToProto maps a decoded ProtectionReading to the wire envelope.
// Each present code sets its optional proto field (absence = not reported).
// Codes intentionally not carried by the proto are registered in
// protCodesUnpublished (enforced by TestReadingToProto_NoSilentDrop) so the
// switch's silent fall-through can't hide a code that ought to be published.
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

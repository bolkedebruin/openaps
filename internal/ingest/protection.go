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
	// the radio (on-demand has no continuous-poll redundancy).
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

// cacheProtectionValues MERGES the native-unit value map from a decoded
// reading into the per-UID cumulative set and returns a copy of the merged
// result. Merging (not replacing) means a later partial read — a QS1A read
// whose page B reply was dropped or truncated — can't erase a previously-known
// code (e.g. the DA output cap); a real change still overwrites its code on the
// next read that carries it. Used by ReadbackNative (reconciler) and to build
// the published/replayed wire.Protection. Each update increments the per-UID
// sequence so callers can detect a stale cache.
func (in *Ingestor) cacheProtectionValues(uid string, values map[string]float64) map[string]float64 {
	if uid == "" {
		return nil
	}
	in.protMu.Lock()
	defer in.protMu.Unlock()
	if in.protValues == nil {
		in.protValues = make(map[string]map[string]float64)
	}
	acc := in.protValues[uid]
	if acc == nil {
		acc = make(map[string]float64, len(values))
		in.protValues[uid] = acc
	}
	for k, v := range values {
		acc[k] = v
	}
	if in.protValueSeq == nil {
		in.protValueSeq = make(map[string]uint64)
	}
	in.protValueSeq[uid]++
	cp := make(map[string]float64, len(acc))
	for k, v := range acc {
		cp[k] = v
	}
	return cp
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
// protection OR set-power write lands, so the published thresholds and the
// output-cap read-back (code "DA") reflect the change without polling. Set-power
// goes through a different opcode path than protection params, so it must be
// matched too — otherwise the cap is changed on the wire but the read-back (and
// the dashboard's per-inverter and total cap) stays stale.
func (in *Ingestor) maybeReadAfterWrite(env *wire.Envelope) {
	s := env.GetSend()
	if s == nil || (!codec.IsProtectionWrite(s.GetFrame()) && !codec.IsSetPower(s.GetFrame())) {
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
	// Merge into the cumulative per-UID set and publish that, so a dropped or
	// truncated page in this read doesn't erase previously-known codes.
	reading.Values = in.cacheProtectionValues(uid, reading.Values)
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

// readingToProto wraps a decoded ProtectionReading as the wire envelope: the
// aps_code→value map verbatim. The codec is the single source of truth, so a
// newly-decoded code is carried with no proto change and no hand-maintained
// per-code mapping (consumers pick the codes they need by aps_code).
func readingToProto(uid string, tsMs int64, r *codec.ProtectionReading) *wire.Protection {
	return &wire.Protection{PeerUid: uid, TsMs: tsMs, Model: r.Model, Values: r.Values}
}

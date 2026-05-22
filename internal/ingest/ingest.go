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
	"sync"
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
//
// Probe, when non-nil, receives a non-blocking signal on every
// successful telemetry ingest. The probe's SQL gate
// (model_code/software_version IS NULL) makes redundant signals cheap,
// so this side doesn't deduplicate.
type Ingestor struct {
	S     *store.Store
	Pub   *events.Publisher
	Probe chan<- struct{}

	// Router, when non-nil, dispatches Envelope_Send / Envelope_Broadcast
	// frames received from peer connections (typically the inv-driver
	// CLI) to a named backend (typically the active ecu-zb publisher).
	// Without a Router these envelopes are dropped with a log line.
	Router Router

	// RouteBackend names the backend that receives downstream Send /
	// Broadcast envelopes routed through Router. Empty disables routing.
	RouteBackend string

	// ControllerBackends names the set of peer backends permitted to
	// inject Envelope_Send / Envelope_Broadcast frames. A peer whose
	// Hello backend is not on this list is refused. The publisher
	// backend named in RouteBackend is always refused (loopback). Empty
	// list disables Send/Broadcast routing entirely.
	ControllerBackends []string

	// ControllerUIDs is the OS-level UID allow-list for peers permitted
	// to inject Envelope_Send / Envelope_Broadcast. The peer UID is
	// resolved at connect time via SO_PEERCRED (Linux). A nil/empty list
	// disables the UID gate (legacy / non-Linux); a non-empty list
	// rejects any UID not listed. The Hello.Backend string remains
	// informational and ControllerBackends still gates by backend name.
	ControllerUIDs []int

	// protMu guards latestProt, the most-recent Protection envelope per
	// inverter UID. Kept in memory (not the store) and replayed to
	// subscribers on attach so a late-joining ecu-sunspec sees current
	// grid-protection state without waiting for the next poll cycle.
	protMu     sync.RWMutex
	latestProt map[string]*wire.Protection
	// protPages buffers the latest paged protection reply frame per
	// inverter UID, keyed by page id (0xDD/0xDE/0xD9), so the 3 pages
	// (which arrive as separate replies) can be decoded together.
	protPages map[string]map[byte][]byte
	// protSig is the last-logged decoded-values signature per UID, so a
	// steady poll only logs grid-protection state when it changes.
	protSig map[string]string
	// protSeen marks UIDs already given their one-shot startup protection
	// read (on first telemetry), so reads are on-demand (first-seen +
	// after-write) rather than a continuous poll.
	protSeen map[string]bool
}

// cacheProtection stores the latest Protection per UID for replay.
func (in *Ingestor) cacheProtection(p *wire.Protection) {
	if p.GetPeerUid() == "" {
		return
	}
	in.protMu.Lock()
	if in.latestProt == nil {
		in.latestProt = make(map[string]*wire.Protection)
	}
	in.latestProt[p.GetPeerUid()] = p
	in.protMu.Unlock()
}

// SnapshotProtection returns a copy of the latest Protection per UID,
// for replay to a newly-attached subscriber.
func (in *Ingestor) SnapshotProtection() []*wire.Protection {
	in.protMu.RLock()
	defer in.protMu.RUnlock()
	out := make([]*wire.Protection, 0, len(in.latestProt))
	for _, p := range in.latestProt {
		out = append(out, p)
	}
	return out
}

// peerUIDUnknown is the sentinel meaning "no peer UID was resolved".
// On non-Linux hosts and from in-process test callers the UID gate is
// implicitly bypassed when ControllerUIDs is empty.
const peerUIDUnknown = -1

// Router is the subset of ipc.Server the Ingestor needs to forward
// downstream envelopes. Lives here to avoid an ingest -> ipc import.
type Router interface {
	SendToBackend(backend string, env *wire.Envelope) bool
}

// Handle dispatches an Envelope by oneof body. Returns an error only
// on hard malformed input; transient DB errors are returned so the
// connection layer can decide whether to drop the peer.
//
// Handle does not resolve the peer's OS UID; the ControllerUIDs gate is
// implicitly skipped. Callers that have a verified peer UID should call
// HandleWithPeer instead so the gate fires for downstream injections.
func (in *Ingestor) Handle(ctx context.Context, backend string, env *wire.Envelope) error {
	return in.HandleWithPeer(ctx, peerUIDUnknown, backend, env)
}

// HandleWithPeer is Handle with the verified peer UID supplied. Used by
// the IPC server to feed SO_PEERCRED-resolved credentials into the
// downstream-injection gate. peerUID == peerUIDUnknown disables the UID
// check (matches Handle behaviour).
func (in *Ingestor) HandleWithPeer(ctx context.Context, peerUID int, backend string, env *wire.Envelope) error {
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
	case *wire.Envelope_Protection:
		if b.Protection == nil {
			return fmt.Errorf("protection envelope without body")
		}
		in.cacheProtection(b.Protection)
		in.publish(env)
		return nil
	case *wire.Envelope_DecodeFailed:
		if b.DecodeFailed == nil {
			return fmt.Errorf("decode_failed envelope without body")
		}
		df := b.DecodeFailed
		return in.S.AppendDecodeFailed(ctx, df.GetTsMs(), df.GetShortAddr(), df.GetError(), df.GetRawHex())
	case *wire.Envelope_Send, *wire.Envelope_Broadcast:
		return in.routeDownstream(peerUID, backend, env)
	case nil:
		return fmt.Errorf("envelope without body")
	default:
		return fmt.Errorf("unhandled envelope body type %T", b)
	}
}

// routeDownstream forwards an Envelope_Send / Envelope_Broadcast to the
// configured backend via Router. Three gates apply:
//
//  1. sender backend must not equal RouteBackend (loopback).
//  2. sender backend must be on ControllerBackends.
//  3. when ControllerUIDs is non-empty and peerUID != peerUIDUnknown,
//     peerUID must be on ControllerUIDs.
func (in *Ingestor) routeDownstream(peerUID int, sender string, env *wire.Envelope) error {
	if in.Router == nil || in.RouteBackend == "" {
		log.Printf("ingest: downstream envelope dropped (no router/backend configured)")
		return nil
	}
	if sender == in.RouteBackend {
		return fmt.Errorf("downstream route refused: sender backend %q matches RouteBackend (loopback)", sender)
	}
	if !isControllerBackend(in.ControllerBackends, sender) {
		return fmt.Errorf("downstream route refused: sender backend %q is not an allowed controller", sender)
	}
	if len(in.ControllerUIDs) > 0 && peerUID != peerUIDUnknown && !isControllerUID(in.ControllerUIDs, peerUID) {
		return fmt.Errorf("downstream route refused: peer uid=%d not in controller-uids allow-list", peerUID)
	}
	if !in.Router.SendToBackend(in.RouteBackend, env) {
		return fmt.Errorf("downstream route to backend %q failed (publisher absent or queue full)", in.RouteBackend)
	}
	in.maybeReadAfterWrite(env)
	return nil
}

// isControllerUID reports whether peerUID is on the allow-list.
func isControllerUID(allow []int, peerUID int) bool {
	for _, u := range allow {
		if u == peerUID {
			return true
		}
	}
	return false
}

// isControllerBackend reports whether sender is on the allow-list.
func isControllerBackend(allow []string, sender string) bool {
	for _, b := range allow {
		if b == sender {
			return true
		}
	}
	return false
}

// Telemetry-row caps. Real frames carry at most 4 panels (QS1A) and at
// most 4 lifetime rows; the caps add headroom for future models without
// permitting a hostile peer to grow store rows unbounded.
const (
	maxTelemetryPanels       = 16
	maxTelemetryLifetimeRows = 16
)

func (in *Ingestor) handleTelemetry(ctx context.Context, t *wire.Telemetry) error {
	uid := t.GetPeerUid()
	if !isValidPeerUID(uid) {
		return fmt.Errorf("telemetry: invalid peer_uid (expected 12 hex chars)")
	}
	in.protReadOnFirstSeen(uid)
	if n := len(t.GetPanels()); n > maxTelemetryPanels {
		return fmt.Errorf("telemetry: panel count %d exceeds cap %d", n, maxTelemetryPanels)
	}
	if n := len(t.GetLifetimeRaw()); n > maxTelemetryLifetimeRows {
		return fmt.Errorf("telemetry: lifetime row count %d exceeds cap %d", n, maxTelemetryLifetimeRows)
	}
	fam := familyFromModel(t.GetModel())
	// short_addr on the wire is uint32 (proto has no uint16); narrow
	// at the store boundary which retains the uint16 column type.
	shortAddr := uint16(t.GetShortAddr())
	tsMs := t.GetTsMs()
	if err := in.S.UpsertInverterFromTelemetry(ctx, uid, shortAddr, fam, t.GetModel(), tsMs); err != nil {
		return err
	}
	in.signalProbe()
	// Capture previous frame's ts before WriteTelemetryLive overwrites
	// it, so we can stamp t.PrevFrameMs and feed RecordInterval. A
	// zero result (no prior row) is fine — subscribers see 0 and skip
	// interval calc on first frame.
	prevTsMs, err := in.S.PrevTelemetryTsMs(ctx, uid)
	if err != nil {
		return err
	}
	t.PrevFrameMs = prevTsMs
	if err := in.S.WriteTelemetryLive(ctx, uid, tsMs, t.GetCmd(),
		t.GetActivePowerW(), t.GetGridV(), t.GetFreqHz(), t.GetBusV(), t.GetReportSec()); err != nil {
		return err
	}
	if _, err := in.S.RecordInterval(ctx, uid, prevTsMs, tsMs); err != nil {
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
		maxChannelW := maxChannelWForCmd(t.GetCmd(), len(raw))
		if err := in.S.WriteEnergyLifetime(ctx, uid, tsMs, rows, maxChannelW); err != nil {
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
	if !isValidPeerUID(uid) {
		return fmt.Errorf("inverter_info: invalid peer_uid (expected 12 hex chars)")
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

// handleRawFrame parses the L1 envelope once, then dispatches the L2
// body through the registered decoders in priority order. The first
// success wins; no match falls to a decode_failed event.
//
// Decoder ordering: telemetry first (dominant frame kind), then info
// reply, then pair frames. Add new decoders by appending to the slice
// returned by rawFrameDecoders.
//
// Note on trust: any peer with UDS write access can submit RawFrame
// envelopes that exercise these decoders. Successful pair-frame
// dispatch mutates inverter state (zigbee_bound / short_addr). Under
// the same-host-root trust model this is accepted; a future capability
// gate could require an outbound-pair-in-flight before accepting an
// inbound pair frame for the same short_addr.
func (in *Ingestor) handleRawFrame(ctx context.Context, rf *wire.RawFrame) error {
	raw := rf.GetL1Frame()
	tsMs := rf.GetTsMs()
	if tsMs == 0 {
		tsMs = time.Now().UnixMilli()
	}

	env, err := codec.ParseL1(raw)
	if err != nil {
		return in.S.AppendDecodeFailed(ctx, tsMs, rf.GetShortAddr(), err.Error(), truncHex(raw))
	}
	if env.Encrypted {
		return in.S.AppendDecodeFailed(ctx, tsMs, rf.GetShortAddr(), "L1 encrypted", truncHex(raw))
	}

	for _, try := range in.rawFrameDecoders() {
		handled, err := try(ctx, tsMs, env)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
	}
	return in.S.AppendDecodeFailed(ctx, tsMs, rf.GetShortAddr(), "no decoder matched", truncHex(raw))
}

type rawFrameTryFunc func(ctx context.Context, tsMs int64, env codec.L1Envelope) (handled bool, err error)

// rawFrameDecoders returns the decode-then-apply pipeline tried in
// order against each inbound L1 frame.
func (in *Ingestor) rawFrameDecoders() []rawFrameTryFunc {
	return []rawFrameTryFunc{
		in.tryTelemetry,
		in.tryProtectionReply,
		in.tryInfoReply,
		in.tryPairFrame,
	}
}

// tryProtectionReply handles the paged grid-protection read replies
// (0xDD page A / 0xDE page B / 0xD9 page C). 0xDD is shared with the
// info-extended query, so a 0xDD frame is treated as protection only
// when it's longer than any info reply (≥ protReplyMinLen); shorter
// 0xDD frames fall through to tryInfoReply. 0xDE/0xD9 are unambiguous.
//
// The family is inferred from the reply cmd: DS3 tags every page 0xDD
// (page selector at byte[4]); QS1 uses the page byte as the cmd. The
// page is buffered and the per-UID set re-decoded + published.
func (in *Ingestor) tryProtectionReply(ctx context.Context, tsMs int64, env codec.L1Envelope) (bool, error) {
	l2, err := codec.ParseL2(env.L2Frame)
	if err != nil {
		return false, nil
	}
	var model uint8
	switch l2.Cmd {
	case codec.CmdProtReadPageA: // 0xDD → DS3 (all DS3 pages), or short info
		if len(env.L2Frame) < protReplyMinLen {
			return false, nil // short 0xDD = info-extended; let tryInfoReply handle
		}
		model = codec.ModelDS3
	case codec.CmdProtReadPageB, codec.CmdProtReadPageC: // 0xDE/0xD9 → QS1
		model = codec.ModelQS1A
	default:
		return false, nil
	}
	in.handleProtectionPage(ctx, tsMs, env, model)
	return true, nil
}

// protReplyMinLen is the shortest a 0xDD protection page-A reply can be;
// info-extended (0xDD) replies are ≤ ~19 bytes, full protection pages
// are far longer (DS3 page A inner-len ≥ 0x63).
const protReplyMinLen = 32

func (in *Ingestor) tryTelemetry(ctx context.Context, tsMs int64, env codec.L1Envelope) (bool, error) {
	rep, err := codec.DecodeReplyFromEnvelope(env)
	if err != nil {
		return false, nil
	}
	t := telemetryFromReply(rep, tsMs)
	if err := in.handleTelemetry(ctx, t); err != nil {
		return true, err
	}
	in.publish(&wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: t}})
	// Refresh fleet aggregate so today/month/year/lifetime tracks each
	// new sample without subscribers having to poll.
	in.publishFleetSummary(ctx)
	return true, nil
}

func (in *Ingestor) tryInfoReply(ctx context.Context, tsMs int64, env codec.L1Envelope) (bool, error) {
	info, err := codec.DecodeInfoReply(env.L2Frame)
	if err != nil {
		return false, nil
	}
	return true, in.applyInfoReply(ctx, tsMs, env, info)
}

func (in *Ingestor) tryPairFrame(ctx context.Context, tsMs int64, env codec.L1Envelope) (bool, error) {
	pf, ok := codec.ParsePairFrame(codec.DirInbound, env.L2Frame)
	if !ok {
		return false, nil
	}
	return true, in.applyPairFrame(ctx, tsMs, env, pf)
}

// applyInfoReply synthesises an InverterInfo from a decoded 0xDC reply.
// PhaseFromModel returns the family classifier (1 vs 3); we write phase
// only when it's 1 because single-phase implies leg=1 unambiguously.
// Three-phase per-leg phase is operator-configured (separate work).
func (in *Ingestor) applyInfoReply(ctx context.Context, tsMs int64, env codec.L1Envelope, info codec.InfoReply) error {
	modelWire := uint32(info.Model)
	sw := info.SoftwareVersion
	wireInfo := &wire.InverterInfo{
		TsMs:            tsMs,
		PeerUid:         env.PeerUIDString(),
		ShortAddr:       uint32(env.ShortAddr),
		ModelCode:       &modelWire,
		SoftwareVersion: &sw,
	}
	if codec.PhaseFromModel(info.Model) == 1 {
		one := uint32(1)
		wireInfo.Phase = &one
	}
	return in.storeAndPublishInfo(ctx, wireInfo)
}

// applyPairFrame synthesises an InverterInfo carrying the pair-state
// columns set by a matched inbound pair frame. A PairShortAddrReply
// with short_addr == 0 is ignored (sentinel for "no assignment"); a
// PairBindAck always emits because the ack itself is the state change.
func (in *Ingestor) applyPairFrame(ctx context.Context, tsMs int64, env codec.L1Envelope, pf codec.PairFrame) error {
	wireInfo := &wire.InverterInfo{
		TsMs:      tsMs,
		PeerUid:   env.PeerUIDString(),
		ShortAddr: uint32(env.ShortAddr),
	}
	switch pf.Kind {
	case codec.PairBindAck:
		t := true
		wireInfo.ZigbeeBound = &t
		if pf.RptOff {
			r := true
			wireInfo.TurnedOffRpt = &r
		}
	case codec.PairShortAddrReply:
		if pf.ShortAddr == 0 {
			return nil
		}
		wireInfo.ShortAddr = uint32(pf.ShortAddr)
	default:
		return nil
	}
	return in.storeAndPublishInfo(ctx, wireInfo)
}

// storeAndPublishInfo upserts an InverterInfo into the store and
// publishes the envelope to subscribers. Common tail for the info /
// pair decoders. Also republishes a FleetSummary so subscribers can
// keep their fleet-nameplate aggregate fresh without re-attaching.
func (in *Ingestor) storeAndPublishInfo(ctx context.Context, wireInfo *wire.InverterInfo) error {
	if err := in.handleInverterInfo(ctx, wireInfo); err != nil {
		return err
	}
	in.publish(&wire.Envelope{Body: &wire.Envelope_Info{Info: wireInfo}})
	in.publishFleetSummary(ctx)
	return nil
}

// publishFleetSummary computes and broadcasts the current fleet
// aggregate. Non-fatal on error — the next call will retry.
func (in *Ingestor) publishFleetSummary(ctx context.Context) {
	if in.Pub == nil {
		return
	}
	fleet := in.BuildFleetSummary(ctx)
	if fleet == nil {
		return
	}
	in.publish(&wire.Envelope{Body: &wire.Envelope_Fleet{Fleet: fleet}})
}

// BuildFleetSummary returns a fully-populated FleetSummary proto for
// the current state of the store: nameplate sum, inverter count, and
// fleet-level lifetime / today / month / year watt-hours. Returns
// nil when the store isn't ready. Side-effect: seeds period anchor
// rows on first observation in a new day/month/year.
func (in *Ingestor) BuildFleetSummary(ctx context.Context) *wire.FleetSummary {
	if in.S == nil {
		return nil
	}
	totalW, count, err := in.S.FleetSummary(ctx)
	if err != nil {
		return nil
	}
	now := time.Now()
	lifetimeWh, err := in.S.FleetLifetimeWh(ctx)
	if err != nil {
		lifetimeWh = 0
	}
	todayWh, _ := in.S.PeriodEnergyWh(ctx, "day", now, lifetimeWh)
	monthWh, _ := in.S.PeriodEnergyWh(ctx, "month", now, lifetimeWh)
	yearWh, _ := in.S.PeriodEnergyWh(ctx, "year", now, lifetimeWh)
	return &wire.FleetSummary{
		TsMs:            now.UnixMilli(),
		NameplateTotalW: totalW,
		InverterCount:   count,
		LifetimeWh:      uint64(lifetimeWh + 0.5),
		TodayWh:         uint64(todayWh + 0.5),
		MonthWh:         uint64(monthWh + 0.5),
		YearWh:          uint64(yearWh + 0.5),
	}
}

// maxChannelWForCmd returns the rated AC watts per channel for the
// inverter family identified by the L2 cmd byte. Used by the energy
// anomaly check in store.WriteEnergyLifetime as the rate ceiling.
// Returns 0 (anomaly check disabled) for unknown cmds.
func maxChannelWForCmd(cmd uint32, channels int) uint32 {
	if channels <= 0 {
		return 0
	}
	var nameplateW uint32
	switch byte(cmd) {
	case codec.CmdReplyDS3:
		nameplateW = codec.NameplateWattsForModel(codec.ModelDS3)
	case codec.CmdReplyQS1A:
		nameplateW = codec.NameplateWattsForModel(codec.ModelQS1A)
	default:
		return 0
	}
	if nameplateW == 0 {
		return 0
	}
	return nameplateW / uint32(channels)
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
		Rssi:          uint32(r.RSSI),
		Lqi:           uint32(r.LQI),
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
	if f := faultsFromReply(r); f != nil {
		t.Faults = f
	}
	return t
}

// faultsFromReply projects the codec's per-family Faults() decode
// onto the wire.InverterFaults oneof. Returns nil when the reply
// can't be family-classified (e.g. unknown cmd).
func faultsFromReply(r codec.Reply) *wire.InverterFaults {
	switch r.Cmd {
	case codec.CmdReplyDS3:
		f := r.DS3Status.Faults()
		return &wire.InverterFaults{Family: &wire.InverterFaults_Ds3{Ds3: &wire.DS3Faults{
			GridRelayFault:    f.GridRelayFault,
			DcContactorFault:  f.DCContactorFault,
			DcBusFault:        f.DCBusFault,
			DcGroundFault:     f.DCGroundFault,
			IsoFaultA:         f.IsoFaultA,
			IsoFaultB:         f.IsoFaultB,
			AcOverVoltStage1:  f.ACOverVoltStage1,
			AcOverVoltStage2:  f.ACOverVoltStage2,
			AcUnderVoltStage1: f.ACUnderVoltStage1,
			AcUnderVoltStage2: f.ACUnderVoltStage2,
			OverFreqStage1:    f.OverFreqStage1,
			OverFreqStage2:    f.OverFreqStage2,
			OverFreqAux:       f.OverFreqAux,
			OverFreqExtra:     f.OverFreqExtra,
			UnderFreqStage1:   f.UnderFreqStage1,
			UnderFreqStage2:   f.UnderFreqStage2,
			UnderFreqAux:      f.UnderFreqAux,
			UnderFreqExtra:    f.UnderFreqExtra,
		}}}
	case codec.CmdReplyQS1A:
		f := r.QS1AStatus.Faults()
		return &wire.InverterFaults{Family: &wire.InverterFaults_Qs1A{Qs1A: &wire.QS1AFaults{
			GridRelayFault:   f.GridRelayFault,
			DcContactorFault: f.DCContactorFault,
			DcGroundFault:    f.DCGroundFault,
			DcBusFault:       f.DCBusFault,
			CommFault:        f.CommFault,
			OverTemperature:  f.OverTemperature,
			IsoFaultA:        f.IsoFaultA,
			IsoFaultB:        f.IsoFaultB,
			IsoFaultC:        f.IsoFaultC,
			IsoFaultD:        f.IsoFaultD,
			AcOverVoltFast:   f.ACOverVoltFast,
			AcOverVoltSlow:   f.ACOverVoltSlow,
			AcUnderVoltFast:  f.ACUnderVoltFast,
			AcUnderVoltSlow:  f.ACUnderVoltSlow,
			OverFreqFast:     f.OverFreqFast,
			OverFreqSlow:     f.OverFreqSlow,
			OverFreqExtra:    f.OverFreqExtra,
			OverFreqRms:      f.OverFreqRMS,
			UnderFreqFast:    f.UnderFreqFast,
			UnderFreqSlow:    f.UnderFreqSlow,
			UnderFreqExtra:   f.UnderFreqExtra,
			UnderFreqRms:     f.UnderFreqRMS,
			ZbLinkA:          f.ZBLinkA,
			ZbLinkB:          f.ZBLinkB,
		}}}
	}
	return nil
}

func (in *Ingestor) publish(env *wire.Envelope) {
	if in.Pub == nil || env == nil {
		return
	}
	in.Pub.Publish(env)
}

// signalProbe nudges the probe goroutine. Non-blocking and idempotent
// via the channel's cap-1 buffer: a burst of signals coalesces into
// one Run pass.
func (in *Ingestor) signalProbe() {
	if in.Probe == nil {
		return
	}
	select {
	case in.Probe <- struct{}{}:
	default:
	}
}

// isValidPeerUID accepts the 12-char lowercase hex form codec.ParseL1
// emits. Rejects anything else so a misbehaving peer can't grow store
// rows / memory with arbitrary strings.
func isValidPeerUID(s string) bool {
	if len(s) != 12 {
		return false
	}
	for i := 0; i < 12; i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
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

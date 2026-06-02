// Package busmgr publishes decoded telemetry to a sibling inv-driver
// daemon over a Unix domain socket. The client is opt-in and additive:
// when ecu-zb is started without --invdriver-sock it stays disabled,
// and ecu-zb behaves exactly as before.
//
// The client is single-writer and best-effort: a bounded outbound
// queue drops frames when inv-driver is slow or absent, with a rate-
// limited log so the operator can tell. Dial runs in the background
// with exponential backoff so ecu-zb startup is never blocked.
package busmgr

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bolke/ecu-zb/internal/inventory"
	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/wire"
)

// L2 length bounds for downstream Send/Broadcast payloads. The shortest
// valid L2 frame is `FB FB <len=1> <cmd> <sum hi> <sum lo> FE FE` (8
// bytes). The smallest cmd we accept on the wire is the info/BB query
// shape `FB FB 06 <cmd> 00 00 00 00 00 00 <sum hi> <sum lo> FE FE`
// (13 bytes); anything shorter is rejected.
const (
	minL2Len = 13
	maxL2Len = 64
)

// allowedL2Cmds enumerates the L2 cmd bytes a controller backend may
// inject through this client. Anything outside the set is rejected at
// validateFrame so pair / firmware-update / factory-reset paths can't
// reach the modem from a peer.
var allowedL2Cmds = map[byte]struct{}{
	codec.CmdSetPowerQS1Unicast:   {},
	codec.CmdSetPowerQS1Broadcast: {},
	codec.CmdSetPowerDS3Unicast:   {},
	codec.CmdSetPowerDS3Broadcast: {},
	codec.CmdSetPowerC3Unicast:    {},
	codec.CmdSetPowerC3Broadcast:  {},
	// Inverter on/off: unicast only. Broadcast on/off (A1/A2) is
	// deliberately omitted — a broadcast OFF is a fleet-wide shutdown,
	// and ecu-sunspec only ever sends per-inverter (unicast) on/off.
	codec.CmdTurnOnUnicast:  {},
	codec.CmdTurnOffUnicast: {},
	// QS1 grid_recovery_time uses the YC600 builder opcode (unicast only).
	codec.CmdGridRecoveryQS1: {},
	// QS1 slow voltage trips (AQ/AD/AC/AY) and grid frequency trips
	// (AJ/AK/AE/AF) route through set_protection_yc600_one @ 0x66728, which
	// puts the param's sub-byte directly at the L2 cmd slot (frame[3]) — so
	// each trip is a distinct cmd byte rather than the 0x1C opcode. Unicast
	// only; broadcast for these is rejected at the encoder (no opcode swap).
	0x11: {}, // AQ under_voltage_stage_2
	0x12: {}, // AD over_voltage_slow
	0x13: {}, // AC under_voltage_stage_2_90
	0x14: {}, // AY over_voltage_slow_90
	0x57: {}, // AK over_frequency_fast
	0x58: {}, // AJ under_frequency_fast
	0x59: {}, // AF over_frequency_slow
	0x5a: {}, // AE under_frequency_slow
	// Grid-protection read queries (benign reads), injected on-demand by
	// inv-driver (first-seen + after a protection write).
	codec.CmdProtReadPageA:    {},
	codec.CmdProtReadPageB:    {},
	codec.CmdProtReadPageC:    {},
	codec.CmdInfoQuery:        {},
	codec.CmdTelemetryBBQuery: {},
}

// broadcastProtoValidated is the bus-level safety gate for broadcast
// protection frames. The broadcast protection opcodes (0xAB and 0x2C)
// share the same L2 cmd byte as set-power broadcast; the sub-byte at
// L2[4] distinguishes them.
//
// Both opcodes are verified on-wire (see codec test vectors), so the
// gate is open:
//   - 0xAB protection broadcast: predicted vs captured L2 bytes
//     verified against a real broadcast capture.
//   - 0x2C protection broadcast: verified by apply-then-read against
//     the 0xDB page-A read-back.
// Broadcast protection apply stays opt-in behind inv-driver's
// -gridprofile-broadcast flag (default off); the unicast-only yc600 cmds are
// rejected on the broadcast path in dispatchBroadcast regardless of this gate.
const broadcastProtoValidated = true

// setpowerBroadcastSubs lists the only sub-bytes that are permitted on
// the shared broadcast opcodes (0xAB / 0x2C) while broadcastProtoValidated
// is false. These are the on-wire-verified set-power sub-bytes.
var setpowerBroadcastSubs = map[byte]struct{}{
	codec.SubMaxPowerDS3: {}, // 0x27
	codec.SubMaxPowerQS1: {}, // 0x8C
}

// errInvalidFrame signals a frame that failed shape validation.
var errInvalidFrame = errors.New("invalid frame")

const (
	outboundCap     = 256
	dialMinBackoff  = 1 * time.Second
	dialMaxBackoff  = 30 * time.Second
	dropLogInterval = 5 * time.Second
)

// Client maintains an asynchronous UDS connection to inv-driver and
// publishes wire.Envelope frames sent through Enqueue.
type Client struct {
	socketPath  string
	backendName string
	version     string

	// OnSend, if non-nil, is invoked for each downstream
	// Envelope_Send / Envelope_Broadcast command received from
	// inv-driver. The callback receives the fully wrapped L1 frame
	// (outer envelope + L2 body) and is expected to inject it onto
	// the wire (typically via the BB poller's tracked-injection
	// path). The L1 outer envelope is built here from the peer_uid
	// lookup (Send) or a SA=0 broadcast (Broadcast); inv-driver
	// ships L2 only. Set OnSend before Start.
	OnSend func(frame []byte) error

	// Inventory resolves peer_uid -> short-address for downstream
	// Envelope_Send dispatch. Populated externally (e.g. from the
	// inbound reply stream that carries both fields). When nil, all
	// Envelope_Send dispatches with a non-empty peer_uid are
	// rejected at dispatchFrame.
	Inventory *inventory.Map

	// Pairing executes OTA pairing primitives (PairingCmd). It owns the
	// modem mutex and the splice pause/resume around primitives that park
	// the radio off the operating PAN. When nil, PairingCmd envelopes are
	// rejected with an error result so a controller learns pairing is
	// unavailable rather than silently no-op'ing.
	Pairing PairingExecutor

	out chan *wire.Envelope

	startedAtMs int64

	connected     atomic.Bool
	droppedTotal  atomic.Uint64
	sentTotal     atomic.Uint64
	sendOkTotal   atomic.Uint64
	sendFailTotal atomic.Uint64

	dropMu      sync.Mutex
	lastDropLog time.Time
	dropsSince  uint64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Stats is a snapshot of client counters for observability.
type Stats struct {
	Connected     bool
	DroppedTotal  uint64
	SentTotal     uint64
	SendOkTotal   uint64
	SendFailTotal uint64
}

// New constructs a disconnected Client. Call Start to begin dialing.
func New(socketPath, backendName, version string) *Client {
	return &Client{
		socketPath:  socketPath,
		backendName: backendName,
		version:     version,
		out:         make(chan *wire.Envelope, outboundCap),
	}
}

// Start spawns the dialer/writer goroutine. Returns immediately; the
// goroutine retries with backoff until ctx is cancelled.
func (c *Client) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.startedAtMs = time.Now().UnixMilli()
	c.wg.Add(1)
	go c.run()
	return nil
}

// Stop cancels the goroutine and waits for it to exit.
func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

// Enqueue queues an envelope for transmission. Never blocks: on a full
// queue the frame is dropped and droppedTotal is incremented.
func (c *Client) Enqueue(env *wire.Envelope) {
	if env == nil {
		return
	}
	select {
	case c.out <- env:
	default:
		c.droppedTotal.Add(1)
		c.maybeLogDrop()
	}
}

// Stats returns a current snapshot.
func (c *Client) Stats() Stats {
	return Stats{
		Connected:     c.connected.Load(),
		DroppedTotal:  c.droppedTotal.Load(),
		SentTotal:     c.sentTotal.Load(),
		SendOkTotal:   c.sendOkTotal.Load(),
		SendFailTotal: c.sendFailTotal.Load(),
	}
}

func (c *Client) maybeLogDrop() {
	c.dropMu.Lock()
	c.dropsSince++
	now := time.Now()
	if now.Sub(c.lastDropLog) < dropLogInterval {
		c.dropMu.Unlock()
		return
	}
	n := c.dropsSince
	c.dropsSince = 0
	c.lastDropLog = now
	c.dropMu.Unlock()
	log.Printf("busmgr: dropped %d frames (queue full)", n)
}

// run drives the dial/session loop via wire.DialLoop.
func (c *Client) run() {
	defer c.wg.Done()
	_ = wire.DialLoop(c.ctx, wire.DialLoopConfig{
		SocketPath: c.socketPath,
		MinBackoff: dialMinBackoff,
		MaxBackoff: dialMaxBackoff,
		OnConnect: func() {
			log.Printf("busmgr: connected to %s", c.socketPath)
			c.connected.Store(true)
		},
		OnDialError: func(err error, retryIn time.Duration) {
			log.Printf("busmgr: dial %s: %v (retry in %s)", c.socketPath, err, retryIn)
		},
		Session: func(ctx context.Context, conn net.Conn) error {
			defer c.connected.Store(false)
			return c.session(ctx, conn)
		},
	})
}

// session sends Hello, then runs a writer and a reader goroutine
// concurrently. The writer drains c.out; the reader dispatches
// downstream commands (Envelope_Send) to OnSend. Returns when ctx is
// cancelled or either side errors. The first read/write error closes
// conn which unblocks the other goroutine.
func (c *Client) session(ctx context.Context, conn net.Conn) error {
	hostname, _ := os.Hostname()
	hello := &wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{
		Backend:     c.backendName,
		Version:     c.version,
		Hostname:    hostname,
		StartedAtMs: c.startedAtMs,
		BusCaps:     []string{"apsystems-stock-zb"},
	}}}

	var writeMu sync.Mutex
	write := func(env *wire.Envelope) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return wire.WriteFrame(conn, env)
	}

	if err := write(hello); err != nil {
		return err
	}
	c.sentTotal.Add(1)

	sessCtx, sessCancel := context.WithCancel(ctx)
	defer sessCancel()

	// Closing conn on shutdown unblocks the reader's ReadFrame.
	go func() {
		<-sessCtx.Done()
		_ = conn.Close()
	}()

	errCh := make(chan error, 2)

	// Writer.
	go func() {
		for {
			select {
			case <-sessCtx.Done():
				errCh <- sessCtx.Err()
				return
			case env := <-c.out:
				if err := write(env); err != nil {
					errCh <- err
					return
				}
				c.sentTotal.Add(1)
			}
		}
	}()

	// Reader.
	go func() {
		for {
			var env wire.Envelope
			if err := wire.ReadFrame(conn, &env); err != nil {
				errCh <- err
				return
			}
			c.handleInbound(&env)
		}
	}()

	// First side to error wins; signal the other to wind down.
	err := <-errCh
	sessCancel()
	// Drain the second result so the goroutines don't leak past
	// session return.
	<-errCh
	return err
}

// handleInbound dispatches one downstream envelope. Send carries an L2
// body for a specific peer; Broadcast carries an L2 body addressed to
// every inverter (SA=0 in the outer envelope this client builds).
func (c *Client) handleInbound(env *wire.Envelope) {
	switch b := env.GetBody().(type) {
	case *wire.Envelope_Send:
		rawSA := b.Send.GetShortAddr()
		if rawSA > 0xFFFF {
			// Proto field is uint32; guard against a controller
			// sending an out-of-range value that would silently
			// truncate to a different inverter's SA.
			c.sendFailTotal.Add(1)
			log.Printf("busmgr: Send rejected: short_addr 0x%X exceeds uint16 max (peer=%q)",
				rawSA, b.Send.GetPeerUid())
			return
		}
		c.dispatchSend(b.Send.GetFrame(), b.Send.GetPeerUid(), uint16(rawSA), b.Send.GetDeadlineMs())
	case *wire.Envelope_Broadcast:
		c.dispatchBroadcast(b.Broadcast.GetFrame(), b.Broadcast.GetDeadlineMs())
	case *wire.Envelope_PairingCmd:
		c.handlePairingCmd(b.PairingCmd)
	default:
		// Hello echoes, future variants — ignore.
	}
}

// dispatchSend resolves the target short-address and injects the L1 frame.
// When shortAddr is non-zero it is used directly for this frame only,
// bypassing the inventory lookup (inv-driver's telemetry path already holds
// the SA and sends it inline, so ecu-zb doesn't need to have seen the
// inverter first). The inventory is NOT updated from a controller Send:
// the radio-learned UID↔SA map is authoritative and must only be written
// by the inbound telemetry path. When shortAddr is zero the existing
// inventory lookup by peerUID applies.
func (c *Client) dispatchSend(l2 []byte, peerUID string, shortAddr uint16, deadlineMs int64) {
	if err := validateFrame(l2); err != nil {
		c.sendFailTotal.Add(1)
		log.Printf("busmgr: Send rejected: %v (peer=%q frame=%d bytes)", err, peerUID, len(l2))
		return
	}

	var sa uint16
	if shortAddr != 0 {
		sa = shortAddr
		// Use the supplied SA for this frame only. Do NOT call
		// c.Inventory.Record here: the inventory must only be updated
		// from inbound radio frames where the SA is authoritatively
		// bound to the UID by the inverter itself.
	} else {
		if peerUID == "" {
			c.sendFailTotal.Add(1)
			log.Printf("busmgr: Send missing both peer_uid and short_addr (frame=%d bytes)", len(l2))
			return
		}
		var ok bool
		sa, ok = c.Inventory.LookupSA(peerUID)
		if !ok {
			c.sendFailTotal.Add(1)
			log.Printf("busmgr: Send unknown peer_uid %q (deadline_ms=%d frame=%d bytes)",
				peerUID, deadlineMs, len(l2))
			return
		}
	}
	c.injectL1(BuildL1Frame(sa, l2), "Send", peerUID, deadlineMs)
}

func (c *Client) dispatchBroadcast(l2 []byte, deadlineMs int64) {
	if err := validateFrame(l2); err != nil {
		c.sendFailTotal.Add(1)
		log.Printf("busmgr: Broadcast rejected: %v (frame=%d bytes)", err, len(l2))
		return
	}
	// The yc600-builder protection cmds (the sub IS the cmd: AQ/AD/AC/AY,
	// AJ/AK/AE/AF, grid_recovery) are unicast-only — there is no broadcast
	// opcode to swap to, so a frame carrying one on SA=0 would fleet-program
	// a grid-protection trip. The encoder never emits these on broadcast;
	// reject them here so a raw Envelope_Broadcast can't either.
	if isUnicastOnlyCmd(l2) {
		c.sendFailTotal.Add(1)
		log.Printf("busmgr: Broadcast rejected: L2 cmd 0x%02X is unicast-only (yc600 protection)", l2[3])
		return
	}
	c.injectL1(BuildL1Frame(0, l2), "Broadcast", "", deadlineMs)
}

// isUnicastOnlyCmd reports whether an L2 frame carries a cmd that must never
// be broadcast — the yc600-builder protection cmds (codec.QS1YC600ProtCmds),
// where the sub is the cmd byte and no broadcast opcode exists to swap to.
func isUnicastOnlyCmd(l2 []byte) bool {
	return len(l2) > 3 && codec.QS1YC600ProtCmds[l2[3]]
}

func (c *Client) injectL1(frame []byte, kind, peerUID string, deadlineMs int64) {
	cb := c.OnSend
	if cb == nil {
		c.sendFailTotal.Add(1)
		log.Printf("busmgr: %s received but OnSend is not configured (peer=%q deadline_ms=%d frame=%d bytes)",
			kind, peerUID, deadlineMs, len(frame))
		return
	}
	if err := cb(frame); err != nil {
		c.sendFailTotal.Add(1)
		log.Printf("busmgr: %s dispatch failed: %v (peer=%q frame=%d bytes)",
			kind, err, peerUID, len(frame))
		return
	}
	c.sendOkTotal.Add(1)
}

// validateFrame rejects L2 frames whose shape obviously does not belong
// on the bus. Anything passing this gate still has to satisfy the modem;
// this is a first-line filter against junk from a misbehaving or
// malicious controller.
//
// L2 layout:
//
//	[0..1]  FB FB
//	[2]     inner-length byte: 1 (cmd) + body_len; total frame = 7 + l2[2]
//	[3]     cmd byte (allow-listed)
//	[4..]   body bytes (l2[2]-1 of them)
//	[..-3]  body[last]
//	[..-2]  checksum hi (= sum of l2[2 .. 2+l2[2]) over BE)
//	[..-1]  is hi/lo split — see code
//	[-2..]  FE FE
//
// The checksum is the byte-wise sum of the inner-length byte plus the
// cmd + body bytes (l2[2..2+l2[2]]). Hi/lo are big-endian.
func validateFrame(l2 []byte) error {
	if len(l2) < minL2Len {
		return fmt.Errorf("%w: too short (%d < %d)", errInvalidFrame, len(l2), minL2Len)
	}
	if len(l2) > maxL2Len {
		return fmt.Errorf("%w: too long (%d > %d)", errInvalidFrame, len(l2), maxL2Len)
	}
	if l2[0] != codec.L2SOF1 || l2[1] != codec.L2SOF2 {
		return fmt.Errorf("%w: bad L2 SOF", errInvalidFrame)
	}
	innerLen := int(l2[2])
	if innerLen+7 != len(l2) {
		return fmt.Errorf("%w: inner length %d disagrees with total %d (want %d)",
			errInvalidFrame, innerLen, len(l2), innerLen+7)
	}
	if l2[len(l2)-2] != codec.L2EOF1 || l2[len(l2)-1] != codec.L2EOF2 {
		return fmt.Errorf("%w: bad L2 EOF", errInvalidFrame)
	}
	sum := l2SumOver(l2[2 : 3+innerLen])
	gotSum := uint16(l2[3+innerLen])<<8 | uint16(l2[4+innerLen])
	if gotSum != sum {
		return fmt.Errorf("%w: L2 sum mismatch (got 0x%04X want 0x%04X)",
			errInvalidFrame, gotSum, sum)
	}
	cmd := l2[3]
	if _, ok := allowedL2Cmds[cmd]; !ok {
		return fmt.Errorf("%w: L2 cmd 0x%02X not on allow-list", errInvalidFrame, cmd)
	}
	// On the shared broadcast opcodes (DS3 0xAB / QS1A 0x2C) the sub-byte
	// at L2[4] distinguishes set-power frames from protection frames. While
	// broadcastProtoValidated is false, reject any sub-byte that is not a
	// known set-power sub — protection subs on these opcodes are not yet
	// confirmed on-wire and must be fail-closed.
	if !broadcastProtoValidated {
		if cmd == codec.CmdSetPowerDS3Broadcast || cmd == codec.CmdSetPowerQS1Broadcast {
			sub := l2[4]
			if _, ok := setpowerBroadcastSubs[sub]; !ok {
				return fmt.Errorf("%w: broadcast protection sub-byte 0x%02X not permitted until validated (cmd=0x%02X)",
					errInvalidFrame, sub, cmd)
			}
		}
	}
	return nil
}

func l2SumOver(b []byte) uint16 {
	var s uint16
	for _, x := range b {
		s += uint16(x)
	}
	return s
}

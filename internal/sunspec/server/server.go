// Package server hosts a Modbus TCP server backed by a periodically refreshed
// SunSpec Bank.
package server

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bolkedebruin/openaps/internal/sunspec/config"
	"github.com/bolkedebruin/openaps/internal/sunspec/source"
	"github.com/bolkedebruin/openaps/internal/sunspec/sunspec"
	"github.com/simonvetter/modbus"
)

// Provider is anything that can produce a fresh Snapshot. Implementations must
// be safe to call from a single goroutine.
type Provider interface {
	Build(ctx context.Context) (source.Snapshot, error)
}

// Config tunes server behavior. Zero values fall back to sensible defaults.
type Config struct {
	URL             string        // tcp://0.0.0.0:502
	RefreshInterval time.Duration // default 5s
	Timeout         time.Duration // session idle timeout
	MaxClients      uint
	Encoder         sunspec.Options
	Logger          *log.Logger

	// Writes is the file-loaded config (writes.enabled + allow_list).
	// Default (omitted) is enabled; explicit "enabled": false disables all
	// FC06/FC10. Source-address gating still applies via AllowList /
	// AllowLocalNetwork.
	Writes config.Config

	// InvDriver owns every inverter write — Model 123 set-power / on-off
	// and the grid-protection params (Models 703/711/134) — routing them
	// through inv-driver's downstream Send to ecu-zb. nil disables writes.
	// There is no SQLite write fallback.
	// Satisfied by *invdriver.Client; an interface keeps the path testable.
	InvDriver frameSender

	// LimitCache records ecu-sunspec's own set-power caps so the snapshot
	// reports an accurate WMaxLimPct read-back without external storage.
	// Shared with the snapshot Builder. nil disables recording.
	LimitCache *source.PowerLimitCache
}

// Server owns the Modbus listener and the snapshot refresh goroutine.
//
// One bank per Modbus unit ID is held in `banks`:
//
//	uid 1 → aggregate (system-wide bank with Multi-MPPT spanning all panels)
//	uid 2..N+1 → one per microinverter, in declaration order
//
// Other unit IDs fall back to the aggregate so casual scanners don't break.
type Server struct {
	cfg      Config
	provider Provider

	banks atomic.Pointer[map[uint8]*sunspec.Bank]

	// snap holds the most-recently-built source.Snapshot so write handlers
	// can map unit-ID + register-offset to the right inverter UID.
	snap atomic.Pointer[source.Snapshot]

	mu  sync.Mutex
	srv *modbus.ModbusServer

	// tracker holds per-(uid, modelID) state for SunSpec models with an
	// AdptCtlRslt / AdptCrvRslt async-result field (Model 711 today).
	// Reconciled after each snapshot refresh.
	tracker *WriteTracker

	// reverter auto-restores full power when a Model 123 WMaxLimPct write
	// included a non-zero RvrtTms and no refresh arrived in time. nil when
	// no Writer is configured.
	reverter *Reverter

	logger *log.Logger
}

// New constructs a Server. Call Start to begin listening, Stop to shut down.
func New(p Provider, cfg Config) *Server {
	if cfg.URL == "" {
		cfg.URL = "tcp://0.0.0.0:502"
	}
	if cfg.RefreshInterval == 0 {
		cfg.RefreshInterval = 5 * time.Second
	}
	if cfg.Timeout == 0 {
		// 5 minutes — Home Assistant's SunSpec config-flow has UI-step gaps
		// approaching a minute, and pysunspec2 doesn't auto-reconnect after
		// a server-side close. 30 s (the previous default) was getting
		// EPIPE during integration setup.
		cfg.Timeout = 5 * time.Minute
	}
	if cfg.MaxClients == 0 {
		cfg.MaxClients = 32
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	return &Server{
		cfg:      cfg,
		provider: p,
		logger:   cfg.Logger,
		tracker:  NewWriteTracker(),
		reverter: NewReverter(cfg.InvDriver, cfg.LimitCache, cfg.Logger),
	}
}

// Start launches the refresh loop, primes the bank with one synchronous
// refresh, then starts the Modbus listener. Returns when the listener is
// ready (or the priming refresh fails fatally).
func (s *Server) Start(ctx context.Context) error {
	if err := s.refresh(ctx); err != nil {
		// Don't abort startup — clients will receive zero-value registers
		// until a successful refresh lands.
		s.logger.Printf("initial refresh failed: %v", err)
	}

	go s.refreshLoop(ctx)

	srv, err := modbus.NewServer(&modbus.ServerConfiguration{
		URL:        s.cfg.URL,
		Timeout:    s.cfg.Timeout,
		MaxClients: s.cfg.MaxClients,
		Logger:     s.cfg.Logger,
	}, &handler{owner: s})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.srv = srv
	s.mu.Unlock()
	if err := srv.Start(); err != nil {
		return err
	}
	s.logger.Printf("modbus tcp listening on %s", s.cfg.URL)
	return nil
}

// Stop drains the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	s.reverter.Stop()
	if srv == nil {
		return nil
	}
	return srv.Stop()
}

// SetSnapshot is exposed for tests so they can drive the server with a fixed
// snapshot without wiring a Provider.
func (s *Server) SetSnapshot(snap source.Snapshot) {
	s.tracker.Reconcile(snap)
	s.banks.Store(buildBanks(snap, s.cfg.Encoder, s.tracker))
	s.snap.Store(&snap)
}

func (s *Server) refresh(ctx context.Context) error {
	if s.provider == nil {
		return errors.New("no snapshot provider configured")
	}
	snap, err := s.provider.Build(ctx)
	if err != nil {
		return err
	}
	// Reconcile pending write requests against the new snapshot before
	// publishing — so the next bank build emits the correct AdptCtlRslt.
	s.tracker.Reconcile(snap)
	s.tracker.PruneSettled(s.cfg.RefreshInterval)
	s.banks.Store(buildBanks(snap, s.cfg.Encoder, s.tracker))
	s.snap.Store(&snap)
	return nil
}

// buildBanks encodes the aggregate bank at uid 1 and a per-microinverter bank
// at uid 2..N+1. The tracker (may be nil for tests) supplies AdptCtlRslt /
// AdptCrvRslt values per (uid, modelID) for SunSpec models that publish
// async write outcomes. With a nil tracker, all rslts default to COMPLETED.
func buildBanks(snap source.Snapshot, opt sunspec.Options, tracker *WriteTracker) *map[uint8]*sunspec.Bank {
	banks := make(map[uint8]*sunspec.Bank, 1+len(snap.Inverters))
	if tracker != nil {
		opt.WriteRslt = func(uid uint8, modelID uint16) (uint16, uint16) {
			return tracker.Get(uid, modelID)
		}
	}
	agg := sunspec.Encode(snap, opt)
	banks[1] = &agg
	for i, inv := range snap.Inverters {
		uid := uint8(2 + i)
		prot := snap.Protection[inv.UID]
		if prot.Has == nil {
			prot.Has = map[string]bool{}
		}
		b := sunspec.EncodePerInverterWithProtection(inv, snap.ECUID, uint16(uid), opt, prot)
		banks[uid] = &b
	}
	return &banks
}

// bankFor picks the right bank for a unit ID. Unknown unit IDs fall back to
// the aggregate so a casual scanner doesn't see Modbus exception 0x0B.
func (s *Server) bankFor(uid uint8) *sunspec.Bank {
	m := s.banks.Load()
	if m == nil {
		return nil
	}
	if b, ok := (*m)[uid]; ok {
		return b
	}
	return (*m)[1] // aggregate fallback
}

func (s *Server) refreshLoop(ctx context.Context) {
	t := time.NewTicker(s.cfg.RefreshInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.refresh(ctx); err != nil {
				s.logger.Printf("refresh failed: %v", err)
			}
		}
	}
}

// --- modbus handler glue ---

type handler struct {
	owner *Server
}

func (h *handler) HandleHoldingRegisters(req *modbus.HoldingRegistersRequest) ([]uint16, error) {
	if req.IsWrite {
		return h.handleWrite(req)
	}
	bank := h.owner.bankFor(req.UnitId)
	if bank == nil || !bank.Contains(req.Addr, req.Quantity) {
		h.owner.logger.Printf("FC03 read from %s uid=%d addr=%d qty=%d → IllegalDataAddress",
			req.ClientAddr, req.UnitId, req.Addr, req.Quantity)
		return nil, modbus.ErrIllegalDataAddress
	}
	h.owner.logger.Printf("FC03 read from %s uid=%d addr=%d qty=%d",
		req.ClientAddr, req.UnitId, req.Addr, req.Quantity)
	return bank.Slice(req.Addr, req.Quantity), nil
}

// handleWrite gates writes via config (writes.enabled + allow_list), then
// dispatches Model 123 writes through ControlsWriter. Anything outside
// Model 123 returns IllegalDataAddress — we don't allow random writes
// scribbling other parts of the bank.
func (h *handler) handleWrite(req *modbus.HoldingRegistersRequest) ([]uint16, error) {
	o := h.owner

	if !o.cfg.Writes.AllowsWrite(req.ClientAddr) {
		reason := "writes disabled"
		if o.cfg.Writes.Writes.IsEnabled() {
			reason = "client not in allow_list"
		}
		o.logger.Printf("FC06/16 write from %s uid=%d addr=%d → rejected (%s)",
			req.ClientAddr, req.UnitId, req.Addr, reason)
		return nil, modbus.ErrIllegalFunction
	}
	if o.cfg.InvDriver == nil {
		o.logger.Printf("FC06/16 write from %s uid=%d addr=%d → inv-driver not configured",
			req.ClientAddr, req.UnitId, req.Addr)
		return nil, modbus.ErrIllegalFunction
	}

	bank := o.bankFor(req.UnitId)
	if bank == nil {
		return nil, modbus.ErrIllegalDataAddress
	}

	snapPtr := o.snap.Load()
	if snapPtr == nil {
		return nil, modbus.ErrServerDeviceFailure
	}

	// Try each writable model in turn. A write must fall fully inside the
	// body of exactly one of these models.
	type modelWriter struct {
		id      uint16
		bodyLen uint16
		apply   func(ctx context.Context, addrOffset uint16, regs []uint16) error
	}
	candidates := []modelWriter{
		{
			id:      sunspec.ControlsModelID,
			bodyLen: sunspec.ControlsBodyLen,
			apply: (&ControlsWriter{
				uid:      req.UnitId,
				snap:     *snapPtr,
				sender:   o.cfg.InvDriver,
				reverter: o.reverter,
				limits:   o.cfg.LimitCache,
				logger:   o.logger,
			}).Apply,
		},
		{
			id:      sunspec.EnterServiceModelID,
			bodyLen: sunspec.EnterServiceBodyLen,
			apply: (&EnterServiceWriter{
				uid:    req.UnitId,
				snap:   *snapPtr,
				sender: o.cfg.InvDriver,
			}).Apply,
		},
		{
			id:      sunspec.FreqDroopModelID,
			bodyLen: sunspec.FreqDroopBodyLen,
			apply: (&FreqDroopWriter{
				uid:     req.UnitId,
				snap:    *snapPtr,
				sender:  o.cfg.InvDriver,
				tracker: o.tracker,
			}).Apply,
		},
		{
			id:      sunspec.FreqWattCurveModelID,
			bodyLen: sunspec.FreqWattCurveBodyLen,
			apply: (&FreqWattCurveWriter{
				uid:     req.UnitId,
				snap:    *snapPtr,
				sender:  o.cfg.InvDriver,
				tracker: o.tracker,
			}).Apply,
		},
	}

	for _, m := range candidates {
		base, ok := findModelInBank(bank, m.id)
		if !ok {
			continue
		}
		bodyStart := base + 2
		bodyEnd := bodyStart + m.bodyLen
		if req.Addr < bodyStart || req.Addr+req.Quantity > bodyEnd {
			continue
		}
		addrOffset := req.Addr - bodyStart
		if err := m.apply(context.Background(), addrOffset, req.Args); err != nil {
			o.logger.Printf("FC06/16 write from %s uid=%d model=%d addr=%d apply: %v",
				req.ClientAddr, req.UnitId, m.id, req.Addr, err)
			return nil, modbus.ErrIllegalDataValue
		}
		o.logger.Printf("FC06/16 write from %s uid=%d model=%d addr=%d qty=%d → applied",
			req.ClientAddr, req.UnitId, m.id, req.Addr, req.Quantity)
		return nil, nil
	}

	o.logger.Printf("FC06/16 write from %s uid=%d addr=%d qty=%d → no writable model covers this range",
		req.ClientAddr, req.UnitId, req.Addr, req.Quantity)
	return nil, modbus.ErrIllegalDataAddress
}

// findModelInBank walks the bank looking for the given model ID. Returns the
// absolute address of the model's ID register (i.e., body starts at +2).
func findModelInBank(bank *sunspec.Bank, id uint16) (uint16, bool) {
	cur := bank.Base + 2 // skip "SunS"
	end := bank.Base + uint16(len(bank.Regs))
	for cur+1 < end {
		modID := bank.At(cur)
		if modID == sunspec.EndModelID {
			return 0, false
		}
		if modID == id {
			return cur, true
		}
		modL := bank.At(cur + 1)
		next := cur + 2 + modL
		if next <= cur || next > end {
			return 0, false
		}
		cur = next
	}
	return 0, false
}

func (h *handler) HandleInputRegisters(req *modbus.InputRegistersRequest) ([]uint16, error) {
	bank := h.owner.bankFor(req.UnitId)
	if bank == nil || !bank.Contains(req.Addr, req.Quantity) {
		h.owner.logger.Printf("FC04 read from %s uid=%d addr=%d qty=%d → IllegalDataAddress",
			req.ClientAddr, req.UnitId, req.Addr, req.Quantity)
		return nil, modbus.ErrIllegalDataAddress
	}
	h.owner.logger.Printf("FC04 read from %s uid=%d addr=%d qty=%d",
		req.ClientAddr, req.UnitId, req.Addr, req.Quantity)
	return bank.Slice(req.Addr, req.Quantity), nil
}

func (h *handler) HandleCoils(req *modbus.CoilsRequest) ([]bool, error) {
	h.owner.logger.Printf("FC01/05/0F coils from %s uid=%d addr=%d qty=%d (rejecting)",
		req.ClientAddr, req.UnitId, req.Addr, req.Quantity)
	return nil, modbus.ErrIllegalFunction
}

func (h *handler) HandleDiscreteInputs(req *modbus.DiscreteInputsRequest) ([]bool, error) {
	h.owner.logger.Printf("FC02 discrete from %s uid=%d addr=%d qty=%d (rejecting)",
		req.ClientAddr, req.UnitId, req.Addr, req.Quantity)
	return nil, modbus.ErrIllegalFunction
}

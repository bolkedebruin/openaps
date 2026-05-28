package settings

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bolke/inv-driver/wire"
)

// macPattern matches a 6-octet colon-separated hex MAC (e.g.
// 80:97:1b:03:0d:ce); case-insensitive.
var macPattern = regexp.MustCompile(`^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$`)

// panPattern matches 1-4 hex digits (a 16-bit PAN id, e.g. "0DCE").
var panPattern = regexp.MustCompile(`^[0-9a-fA-F]{1,4}$`)

// maxEcuIDLen bounds the operator's free-form ECU identifier;
// maxNameLen bounds each inverter label.
const (
	maxEcuIDLen = 128
	maxNameLen  = 64
)

// Handler answers SettingsRequest: GET returns the current settings, SET
// validates and persists a replacement. inv-driver is the sole writer.
type Handler struct {
	Store *Store

	// OnChange, when set, runs after a successful Save so consumers can
	// react to a settings change. Nil is a no-op.
	OnChange func()

	// LiveMAC supplies the live network MAC used to resolve the effective
	// values shown alongside the operator inputs. Nil defaults to
	// ReadEth0MAC; tests inject a stub.
	LiveMAC func() string

	// Apply pushes a new MAC onto the network interface. Nil defaults to
	// ApplyMAC; tests inject a stub.
	Apply func(ctx context.Context, mac string) error

	// AppendEvent, when set, records an audit row for each MAC-apply
	// outcome (success or failure). Nil is a no-op — handler code never
	// fails on a missing event sink, the apply itself is the source of
	// truth. The "by" arg is filled in by the handler's caller via
	// AppendEventBy.
	AppendEvent func(ctx context.Context, tsMs int64, inverterUID, kind, severity, by, detail string) error

	// AppendEventBy attributes apply events to this backend name (e.g.
	// "ecu-web"). Empty means the events are still logged but with no
	// "by" attribution.
	AppendEventBy string
}

// NewHandler returns a Handler backed by store. The effective-values reader
// defaults to ReadEth0MAC; override Handler.LiveMAC for tests. The MAC
// applier defaults to ApplyMAC; override Handler.Apply for tests.
func NewHandler(store *Store) *Handler {
	return &Handler{Store: store, LiveMAC: ReadEth0MAC, Apply: ApplyMAC}
}

// Handle serves a SettingsRequest. A nil Set op (get or empty) reads the
// current settings; a non-nil Set op validates and persists.
//
// When the operator changes the MAC to a new non-empty value, the
// interface is reconfigured BEFORE the change is persisted — if the
// apply fails the disk state is unchanged so "what's persisted" always
// matches "what eth0 is using." Clearing the MAC (new empty, old set)
// leaves eth0 alone; macapp / next boot will reassert the live MAC.
func (h *Handler) Handle(ctx context.Context, req *wire.SettingsRequest) *wire.SettingsResponse {
	if set := req.GetSet(); set != nil {
		s := fromWire(set)
		if err := validate(s); err != nil {
			return &wire.SettingsResponse{Ok: false, Error: err.Error()}
		}
		oldMAC := h.Store.Get().MAC
		newMAC := s.MAC
		switch {
		case newMAC != "" && newMAC != oldMAC:
			live := h.liveMACReader()()
			if err := h.applier()(ctx, newMAC); err != nil {
				h.emitApplyEvent(ctx, false, live, newMAC, err)
				return &wire.SettingsResponse{Ok: false, Error: err.Error()}
			}
			h.emitApplyEvent(ctx, true, live, newMAC, nil)
		case newMAC == "" && oldMAC != "":
			log.Printf("settings: mac cleared; eth0 keeps current value until next boot")
		}
		if err := h.Store.Save(s); err != nil {
			return &wire.SettingsResponse{Ok: false, Error: err.Error()}
		}
		if h.OnChange != nil {
			h.OnChange()
		}
		return h.respond(true, "")
	}
	return h.respond(true, "")
}

// emitApplyEvent writes one event row (best-effort) for a MAC-apply
// outcome. Failure to write the event is logged but does not affect the
// handler's response: the apply itself is the source of truth.
func (h *Handler) emitApplyEvent(ctx context.Context, ok bool, live, wanted string, applyErr error) {
	if h.AppendEvent == nil {
		return
	}
	kind, sev, detail := ApplyEventFields(ok, live, wanted, applyErr)
	if err := h.AppendEvent(ctx, time.Now().UnixMilli(), "", kind, sev, h.AppendEventBy, detail); err != nil {
		log.Printf("settings: append apply event: %v", err)
	}
}

// ApplyEventFields returns the (kind, severity, detail) triple to record
// for a MAC-apply outcome. Shared between the handler (set-rpc path) and
// boot-time main so the audit-log shape stays consistent. trigger lets
// callers tag the event source (e.g. "boot" / "set"); pass "" to omit.
func ApplyEventFields(ok bool, live, wanted string, applyErr error) (kind, severity, detail string) {
	if ok {
		return "mac_apply_ok", "info", fmt.Sprintf("live=%s applied=%s", live, wanted)
	}
	errMsg := ""
	if applyErr != nil {
		errMsg = applyErr.Error()
	}
	const maxErr = 200
	if len(errMsg) > maxErr {
		errMsg = errMsg[:maxErr] + "...(truncated)"
	}
	return "mac_apply_failed", "error", fmt.Sprintf("live=%s wanted=%s err=%s", live, wanted, errMsg)
}

// applier returns the configured MAC applier or the package default.
func (h *Handler) applier() func(ctx context.Context, mac string) error {
	if h.Apply != nil {
		return h.Apply
	}
	return ApplyMAC
}

// respond builds a SettingsResponse carrying the current settings plus the
// effective values resolved against the live system.
func (h *Handler) respond(ok bool, errMsg string) *wire.SettingsResponse {
	cur := h.Store.Get()
	return &wire.SettingsResponse{
		Ok:        ok,
		Error:     errMsg,
		Settings:  toWire(cur),
		Effective: effectiveToWire(Resolve(cur, h.liveMACReader())),
	}
}

// liveMACReader returns the configured reader or the default.
func (h *Handler) liveMACReader() func() string {
	if h.LiveMAC != nil {
		return h.LiveMAC
	}
	return ReadEth0MAC
}

// effectiveToWire maps an Effective into its wire form.
func effectiveToWire(e Effective) *wire.EffectiveSettings {
	return &wire.EffectiveSettings{
		Mac:        e.MAC,
		Pan:        e.PAN,
		ZigbeeType: e.ZigbeeType,
		Channel:    e.Channel,
	}
}

// macClassReject reports a non-empty reason when mac (already matching
// macPattern) falls into a class that can never name a real Ethernet
// host: all-zero, broadcast, or any multicast (least-significant bit of
// the first octet set). Returns "" for any acceptable MAC.
func macClassReject(mac string) string {
	lower := strings.ToLower(mac)
	if lower == "00:00:00:00:00:00" {
		return "all-zero MAC"
	}
	if lower == "ff:ff:ff:ff:ff:ff" {
		return "broadcast MAC"
	}
	// macPattern already guaranteed two hex chars before ':'.
	first, err := strconv.ParseUint(lower[:2], 16, 8)
	if err != nil {
		// Should be unreachable given macPattern; treat as invalid.
		return "unparseable first octet"
	}
	if first&0x01 == 1 {
		return "multicast MAC (first-octet LSB set)"
	}
	return ""
}

// validate checks the operator-supplied settings against the field rules.
func validate(s Settings) error {
	switch s.ZigbeeType {
	case "", "apsystems", "general":
	default:
		return fmt.Errorf("zigbee_type %q invalid: want \"\", \"apsystems\" or \"general\"", s.ZigbeeType)
	}
	if s.PANOverride != "" && !panPattern.MatchString(s.PANOverride) {
		return fmt.Errorf("pan_override %q invalid: want 1-4 hex digits", s.PANOverride)
	}
	if s.Channel != 0 && (s.Channel < 11 || s.Channel > 26) {
		return fmt.Errorf("channel %d invalid: want 0 (unset) or 11-26", s.Channel)
	}
	if s.MAC != "" {
		if !macPattern.MatchString(s.MAC) {
			return fmt.Errorf("mac %q invalid: want 6 colon-separated hex octets (e.g. 80:97:1b:03:0d:ce)", s.MAC)
		}
		if reason := macClassReject(s.MAC); reason != "" {
			return fmt.Errorf("mac %q invalid: %s", s.MAC, reason)
		}
	}
	if len(s.EcuID) > maxEcuIDLen {
		return fmt.Errorf("ecu_id too long: %d chars (max %d)", len(s.EcuID), maxEcuIDLen)
	}
	for uid, name := range s.InverterNames {
		if len(name) > maxNameLen {
			return fmt.Errorf("inverter name for %s too long: %d chars (max %d)", uid, len(name), maxNameLen)
		}
	}
	return nil
}

// toWire maps a settings.Settings to the wire schema.
func toWire(s Settings) *wire.Settings {
	return &wire.Settings{
		EcuId:         s.EcuID,
		Mac:           s.MAC,
		PanOverride:   s.PANOverride,
		ZigbeeType:    s.ZigbeeType,
		InverterNames: s.InverterNames,
		Channel:       s.Channel,
	}
}

// fromWire maps a wire.Settings to settings.Settings.
func fromWire(w *wire.Settings) Settings {
	return Settings{
		EcuID:         w.GetEcuId(),
		MAC:           w.GetMac(),
		PANOverride:   w.GetPanOverride(),
		ZigbeeType:    w.GetZigbeeType(),
		InverterNames: w.GetInverterNames(),
		Channel:       w.GetChannel(),
	}
}

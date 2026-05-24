package settings

import (
	"context"
	"fmt"
	"regexp"

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
}

// NewHandler returns a Handler backed by store.
func NewHandler(store *Store) *Handler {
	return &Handler{Store: store}
}

// Handle serves a SettingsRequest. A nil Set op (get or empty) reads the
// current settings; a non-nil Set op validates and persists.
func (h *Handler) Handle(_ context.Context, req *wire.SettingsRequest) *wire.SettingsResponse {
	if set := req.GetSet(); set != nil {
		s := fromWire(set)
		if err := validate(s); err != nil {
			return &wire.SettingsResponse{Ok: false, Error: err.Error()}
		}
		if err := h.Store.Save(s); err != nil {
			return &wire.SettingsResponse{Ok: false, Error: err.Error()}
		}
		if h.OnChange != nil {
			h.OnChange()
		}
		return &wire.SettingsResponse{Ok: true, Settings: toWire(h.Store.Get())}
	}
	return &wire.SettingsResponse{Ok: true, Settings: toWire(h.Store.Get())}
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
	if s.MAC != "" && !macPattern.MatchString(s.MAC) {
		return fmt.Errorf("mac %q invalid: want 6 colon-separated hex octets (e.g. 80:97:1b:03:0d:ce)", s.MAC)
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
	}
}

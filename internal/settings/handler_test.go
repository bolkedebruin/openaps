package settings

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bolke/inv-driver/wire"
)

func newHandler(t *testing.T) (*Handler, *Store) {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return NewHandler(st), st
}

func getReq() *wire.SettingsRequest {
	return &wire.SettingsRequest{Op: &wire.SettingsRequest_Get{Get: &wire.Empty{}}}
}

func setReq(s *wire.Settings) *wire.SettingsRequest {
	return &wire.SettingsRequest{Op: &wire.SettingsRequest_Set{Set: s}}
}

func TestHandle_GetReturnsStoreValues(t *testing.T) {
	h, st := newHandler(t)
	if err := st.Save(Settings{EcuID: "roof", MAC: "80:97:1b:03:0d:ce", PANOverride: "0DCE", ZigbeeType: "apsystems"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	resp := h.Handle(context.Background(), getReq())
	if !resp.GetOk() {
		t.Fatalf("get not ok: %s", resp.GetError())
	}
	got := resp.GetSettings()
	if got.GetEcuId() != "roof" || got.GetMac() != "80:97:1b:03:0d:ce" ||
		got.GetPanOverride() != "0DCE" || got.GetZigbeeType() != "apsystems" {
		t.Fatalf("get returned %+v", got)
	}
}

func TestHandle_GetEmptyOpReadsCurrent(t *testing.T) {
	h, st := newHandler(t)
	if err := st.Save(Settings{EcuID: "x"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// A request with no op set is treated as a get.
	resp := h.Handle(context.Background(), &wire.SettingsRequest{})
	if !resp.GetOk() || resp.GetSettings().GetEcuId() != "x" {
		t.Fatalf("empty-op get failed: ok=%v ecu=%q", resp.GetOk(), resp.GetSettings().GetEcuId())
	}
}

func TestHandle_SetValidPersistsAndEchoes(t *testing.T) {
	h, st := newHandler(t)
	in := &wire.Settings{EcuId: "site-a", Mac: "80:97:1b:03:0d:ce", PanOverride: "1f", ZigbeeType: "general"}
	resp := h.Handle(context.Background(), setReq(in))
	if !resp.GetOk() {
		t.Fatalf("set not ok: %s", resp.GetError())
	}
	if resp.GetSettings().GetEcuId() != "site-a" || resp.GetSettings().GetZigbeeType() != "general" {
		t.Fatalf("echo mismatch: %+v", resp.GetSettings())
	}
	persisted := st.Get()
	if persisted.EcuID != "site-a" || persisted.MAC != "80:97:1b:03:0d:ce" ||
		persisted.PANOverride != "1f" || persisted.ZigbeeType != "general" {
		t.Fatalf("not persisted: %+v", persisted)
	}
}

func TestHandle_SetInvalidDoesNotPersist(t *testing.T) {
	cases := []struct {
		name string
		in   *wire.Settings
		want string
	}{
		{"bad zigbee_type", &wire.Settings{ZigbeeType: "zwave"}, "zigbee_type"},
		{"bad pan_override too long", &wire.Settings{PanOverride: "12345"}, "pan_override"},
		{"bad pan_override non-hex", &wire.Settings{PanOverride: "xyz"}, "pan_override"},
		{"bad mac", &wire.Settings{Mac: "80-97-1b-03-0d-ce"}, "mac"},
		{"over-long ecu_id", &wire.Settings{EcuId: strings.Repeat("a", maxEcuIDLen+1)}, "ecu_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, st := newHandler(t)
			resp := h.Handle(context.Background(), setReq(tc.in))
			if resp.GetOk() {
				t.Fatalf("expected refusal, got ok")
			}
			if !strings.Contains(resp.GetError(), tc.want) {
				t.Fatalf("error %q does not mention %q", resp.GetError(), tc.want)
			}
			if got := st.Get(); !reflect.DeepEqual(got, Settings{}) {
				t.Fatalf("invalid set persisted: %+v", got)
			}
		})
	}
}

func TestHandle_OnChangeFiresOnSuccessfulSetOnly(t *testing.T) {
	h, _ := newHandler(t)
	var fired int
	h.OnChange = func() { fired++ }

	// Get does not fire OnChange.
	h.Handle(context.Background(), getReq())
	if fired != 0 {
		t.Fatalf("OnChange fired on get: %d", fired)
	}

	// Invalid set does not fire OnChange.
	h.Handle(context.Background(), setReq(&wire.Settings{ZigbeeType: "bogus"}))
	if fired != 0 {
		t.Fatalf("OnChange fired on invalid set: %d", fired)
	}

	// Valid set fires once.
	h.Handle(context.Background(), setReq(&wire.Settings{EcuId: "ok"}))
	if fired != 1 {
		t.Fatalf("OnChange fired %d times on valid set, want 1", fired)
	}
}

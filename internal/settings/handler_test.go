package settings

import (
	"context"
	"errors"
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
	h := NewHandler(st)
	// Default the MAC applier to a no-op for tests so a SET that
	// changes the MAC doesn't try to reconfigure the test host's eth0.
	h.Apply = func(_ context.Context, _ string) error { return nil }
	return h, st
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
	// The ZigBee radio fields (zigbee_type, pan_override, channel) are no
	// longer validated here — ecu-zb owns that. Only non-radio checks (MAC
	// gating an eth0 apply, ecu_id length) remain.
	cases := []struct {
		name string
		in   *wire.Settings
		want string
	}{
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

func TestValidate_RejectsBadMACClasses(t *testing.T) {
	cases := []struct {
		name    string
		mac     string
		wantSub string
	}{
		{"all-zero", "00:00:00:00:00:00", "all-zero"},
		{"broadcast-lower", "ff:ff:ff:ff:ff:ff", "broadcast"},
		{"broadcast-upper", "FF:FF:FF:FF:FF:FF", "broadcast"},
		{"multicast-01", "01:00:5e:00:00:01", "multicast"},
		{"multicast-33", "33:33:00:00:00:01", "multicast"},
		{"multicast-odd-first-byte", "0f:11:22:33:44:55", "multicast"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(Settings{MAC: tc.mac})
			if err == nil {
				t.Fatalf("validate(%q) returned nil, want non-nil error", tc.mac)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("validate(%q) error %q does not contain %q", tc.mac, err.Error(), tc.wantSub)
			}
		})
	}
}

// inv-driver no longer computes/returns effective radio values — the UI
// derives them client-side and ecu-zb owns validation. The response carries
// the stored Settings only; the radio Effective fields are not populated.
func TestHandle_NoEffectiveRadioOnGet(t *testing.T) {
	h, _ := newHandler(t)
	h.LiveMAC = func() string { return "80:97:1b:03:0d:ce" }
	resp := h.Handle(context.Background(), getReq())
	if eff := resp.GetEffective(); eff != nil &&
		(eff.GetPan() != "" || eff.GetChannel() != 0 || eff.GetZigbeeType() != "") {
		t.Fatalf("radio Effective fields should not be populated: %+v", eff)
	}
}

// Radio fields are stored verbatim — no validation, no derivation. A
// non-canonical pan_override and an out-of-range channel both persist as
// given (ecu-zb rejects bad values at the wire, not here).
func TestHandle_RadioFieldsStoredVerbatim(t *testing.T) {
	h, st := newHandler(t)
	in := &wire.Settings{PanOverride: "12345", ZigbeeType: "zwave", Channel: 99}
	resp := h.Handle(context.Background(), setReq(in))
	if !resp.GetOk() {
		t.Fatalf("set should succeed (no radio validation): %s", resp.GetError())
	}
	got := st.Get()
	if got.PANOverride != "12345" || got.ZigbeeType != "zwave" || got.Channel != 99 {
		t.Fatalf("radio fields not stored verbatim: %+v", got)
	}
}

func TestSet_AppliesMACBeforeSave(t *testing.T) {
	h, st := newHandler(t)
	var gotMAC string
	var applyCalls int
	h.Apply = func(_ context.Context, mac string) error {
		applyCalls++
		gotMAC = mac
		return nil
	}
	in := &wire.Settings{Mac: "aa:bb:cc:dd:ee:ff"}
	resp := h.Handle(context.Background(), setReq(in))
	if !resp.GetOk() {
		t.Fatalf("set not ok: %s", resp.GetError())
	}
	if applyCalls != 1 {
		t.Fatalf("Apply called %d times, want 1", applyCalls)
	}
	if gotMAC != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("Apply got mac %q, want aa:bb:cc:dd:ee:ff", gotMAC)
	}
	if persisted := st.Get(); persisted.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("save did not persist new mac: %+v", persisted)
	}
	if resp.GetSettings().GetMac() != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("response settings mac = %q, want aa:bb:cc:dd:ee:ff", resp.GetSettings().GetMac())
	}
}

func TestSet_AbortsSaveOnApplyFailure(t *testing.T) {
	h, st := newHandler(t)
	if err := st.Save(Settings{MAC: "80:97:1b:03:0d:ce", EcuID: "old"}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	boom := errors.New("ip link down failed")
	h.Apply = func(_ context.Context, _ string) error { return boom }

	in := &wire.Settings{Mac: "aa:bb:cc:dd:ee:ff", EcuId: "new"}
	resp := h.Handle(context.Background(), setReq(in))
	if resp.GetOk() {
		t.Fatalf("set unexpectedly ok with failing apply")
	}
	if !strings.Contains(resp.GetError(), "ip link down failed") {
		t.Errorf("response error %q should mention apply failure", resp.GetError())
	}
	if persisted := st.Get(); persisted.MAC != "80:97:1b:03:0d:ce" || persisted.EcuID != "old" {
		t.Fatalf("save ran despite apply failure: %+v", persisted)
	}
}

func TestSet_NoApplyWhenMACUnchanged(t *testing.T) {
	h, st := newHandler(t)
	if err := st.Save(Settings{MAC: "80:97:1b:03:0d:ce", EcuID: "old"}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	var applyCalls int
	h.Apply = func(_ context.Context, _ string) error {
		applyCalls++
		return nil
	}
	// Same MAC, but EcuID changes — Save must still run.
	in := &wire.Settings{Mac: "80:97:1b:03:0d:ce", EcuId: "renamed"}
	resp := h.Handle(context.Background(), setReq(in))
	if !resp.GetOk() {
		t.Fatalf("set not ok: %s", resp.GetError())
	}
	if applyCalls != 0 {
		t.Fatalf("Apply called %d times for unchanged mac, want 0", applyCalls)
	}
	if persisted := st.Get(); persisted.EcuID != "renamed" {
		t.Fatalf("Save did not run for unchanged mac: %+v", persisted)
	}
}

func TestSet_EmptyMACClears(t *testing.T) {
	h, st := newHandler(t)
	if err := st.Save(Settings{MAC: "80:97:1b:03:0d:ce", EcuID: "old"}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	var applyCalls int
	h.Apply = func(_ context.Context, _ string) error {
		applyCalls++
		return nil
	}
	in := &wire.Settings{Mac: "", EcuId: "old"}
	resp := h.Handle(context.Background(), setReq(in))
	if !resp.GetOk() {
		t.Fatalf("set not ok: %s", resp.GetError())
	}
	if applyCalls != 0 {
		t.Fatalf("Apply called %d times for cleared mac, want 0", applyCalls)
	}
	if persisted := st.Get(); persisted.MAC != "" {
		t.Fatalf("Save did not clear mac: %+v", persisted)
	}
}

// capturedEvent records a single AppendEvent invocation for the apply-event tests.
type capturedEvent struct {
	UID, Kind, Severity, By, Detail string
}

func TestSet_EmitsApplyEventOnSuccess(t *testing.T) {
	h, _ := newHandler(t)
	h.LiveMAC = func() string { return "64:33:db:fa:47:0b" }
	var events []capturedEvent
	h.AppendEvent = func(_ context.Context, _ int64, uid, kind, sev, by, detail string) error {
		events = append(events, capturedEvent{uid, kind, sev, by, detail})
		return nil
	}
	h.AppendEventBy = "ecu-web"

	in := &wire.Settings{Mac: "80:97:1b:03:0d:ce"}
	resp := h.Handle(context.Background(), setReq(in))
	if !resp.GetOk() {
		t.Fatalf("set not ok: %s", resp.GetError())
	}
	if len(events) != 1 {
		t.Fatalf("AppendEvent called %d times, want 1: %+v", len(events), events)
	}
	got := events[0]
	if got.Kind != "mac_apply_ok" {
		t.Errorf("kind = %q, want mac_apply_ok", got.Kind)
	}
	if got.Severity != "info" {
		t.Errorf("severity = %q, want info", got.Severity)
	}
	if got.By != "ecu-web" {
		t.Errorf("by = %q, want ecu-web", got.By)
	}
	if got.UID != "" {
		t.Errorf("inverter_uid = %q, want empty (ECU-level event)", got.UID)
	}
	if !strings.Contains(got.Detail, "live=64:33:db:fa:47:0b") || !strings.Contains(got.Detail, "applied=80:97:1b:03:0d:ce") {
		t.Errorf("detail %q missing live/applied fields", got.Detail)
	}
}

func TestSet_EmitsApplyEventOnFailure(t *testing.T) {
	h, _ := newHandler(t)
	h.LiveMAC = func() string { return "64:33:db:fa:47:0b" }
	boom := errors.New("ip link down failed")
	h.Apply = func(_ context.Context, _ string) error { return boom }
	var events []capturedEvent
	h.AppendEvent = func(_ context.Context, _ int64, uid, kind, sev, by, detail string) error {
		events = append(events, capturedEvent{uid, kind, sev, by, detail})
		return nil
	}
	h.AppendEventBy = "ecu-web"

	in := &wire.Settings{Mac: "80:97:1b:03:0d:ce"}
	resp := h.Handle(context.Background(), setReq(in))
	if resp.GetOk() {
		t.Fatalf("set unexpectedly ok with failing apply")
	}
	if len(events) != 1 {
		t.Fatalf("AppendEvent called %d times, want 1: %+v", len(events), events)
	}
	got := events[0]
	if got.Kind != "mac_apply_failed" {
		t.Errorf("kind = %q, want mac_apply_failed", got.Kind)
	}
	if got.Severity != "error" {
		t.Errorf("severity = %q, want error", got.Severity)
	}
	if got.By != "ecu-web" {
		t.Errorf("by = %q, want ecu-web", got.By)
	}
	if !strings.Contains(got.Detail, "ip link down failed") {
		t.Errorf("detail %q missing apply error", got.Detail)
	}
	if !strings.Contains(got.Detail, "wanted=80:97:1b:03:0d:ce") {
		t.Errorf("detail %q missing wanted", got.Detail)
	}
}

func TestSet_NoApplyEventWhenMACUnchanged(t *testing.T) {
	h, st := newHandler(t)
	if err := st.Save(Settings{MAC: "80:97:1b:03:0d:ce", EcuID: "old"}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	var calls int
	h.AppendEvent = func(_ context.Context, _ int64, uid, kind, sev, by, detail string) error {
		calls++
		return nil
	}
	// Same MAC, but EcuID changes — apply is skipped, so no event row.
	in := &wire.Settings{Mac: "80:97:1b:03:0d:ce", EcuId: "renamed"}
	resp := h.Handle(context.Background(), setReq(in))
	if !resp.GetOk() {
		t.Fatalf("set not ok: %s", resp.GetError())
	}
	if calls != 0 {
		t.Errorf("AppendEvent called %d times for unchanged mac, want 0", calls)
	}
}

func TestApplyEventFields_TruncatesLongError(t *testing.T) {
	long := strings.Repeat("X", 500)
	_, _, detail := ApplyEventFields(false, "a", "b", errors.New(long))
	if !strings.Contains(detail, "(truncated)") {
		t.Errorf("detail %q should mark truncation for very long err", detail)
	}
	// Sanity: short errors are not truncated.
	_, _, detail = ApplyEventFields(false, "a", "b", errors.New("short"))
	if strings.Contains(detail, "truncated") {
		t.Errorf("short err should not be marked truncated: %q", detail)
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

	// Invalid set does not fire OnChange (over-long ecu_id is a non-radio
	// check that still rejects).
	h.Handle(context.Background(), setReq(&wire.Settings{EcuId: strings.Repeat("a", maxEcuIDLen+1)}))
	if fired != 0 {
		t.Fatalf("OnChange fired on invalid set: %d", fired)
	}

	// Valid set fires once.
	h.Handle(context.Background(), setReq(&wire.Settings{EcuId: "ok"}))
	if fired != 1 {
		t.Fatalf("OnChange fired %d times on valid set, want 1", fired)
	}
}

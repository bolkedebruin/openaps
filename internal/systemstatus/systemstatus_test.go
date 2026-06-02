package systemstatus

import (
	"context"
	"testing"

	"github.com/bolkedebruin/openaps/wire"
)

func TestRegistryRecordRemove(t *testing.T) {
	r := NewRegistry()
	r.Record(1, "ecu-zb", "1.0", "ecu", "PUBLISHER", 0, 100)
	r.Record(2, "ecu-web", "0.1", "ecu", "SUBSCRIBER", 0, 200)
	if got := len(r.snapshot()); got != 2 {
		t.Fatalf("snapshot len = %d, want 2", got)
	}
	r.Remove(1)
	snap := r.snapshot()
	if len(snap) != 1 || snap[0].backend != "ecu-web" {
		t.Fatalf("after remove: %+v", snap)
	}
}

func TestManagerHandleIdentityAndPeers(t *testing.T) {
	m := NewManager("ecu-host", []string{"ecu-web", "ecu-sunspec"})
	m.Identity = func() *wire.EcuIdentity {
		return &wire.EcuIdentity{EcuId: "my-roof-ecu"}
	}
	m.Record(1, "ecu-zb", "1.0", "ecu-host", "PUBLISHER", 0, 111)
	m.Record(2, "ecu-web", "0.1", "ecu-host", "SUBSCRIBER", 0, 222)

	resp := m.Handle(context.Background(), nil)
	if !resp.GetOk() {
		t.Fatalf("not ok: %s", resp.GetError())
	}
	ecu := resp.GetEcu()
	if ecu.GetEcuId() != "my-roof-ecu" || ecu.GetHostname() != "ecu-host" {
		t.Errorf("identity wrong: %+v", ecu)
	}
	if len(resp.GetPeers()) != 2 {
		t.Fatalf("peers = %d, want 2", len(resp.GetPeers()))
	}
	// controller flag: ecu-web is on the list, ecu-zb is not.
	byBackend := map[string]bool{}
	for _, p := range resp.GetPeers() {
		byBackend[p.GetBackend()] = p.GetController()
	}
	if !byBackend["ecu-web"] {
		t.Errorf("ecu-web should be flagged controller")
	}
	if byBackend["ecu-zb"] {
		t.Errorf("ecu-zb should NOT be flagged controller")
	}
}

func TestManagerNoIdentityProvider(t *testing.T) {
	m := NewManager("h", nil)
	resp := m.Handle(context.Background(), nil)
	if resp.GetEcu().GetEcuId() != "" {
		t.Errorf("expected blank ecu id with no identity provider")
	}
	if resp.GetEcu().GetHostname() != "h" {
		t.Errorf("hostname should still be set, got %q", resp.GetEcu().GetHostname())
	}
}

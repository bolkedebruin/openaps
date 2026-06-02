package snapshot

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/codec"
	"github.com/bolkedebruin/openaps/wire"
)

func TestApplyTelemetry_DerivesCapLoadOnline(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	s := New(nil)
	s.ApplyInfo(&wire.InverterInfo{
		PeerUid:   "aabbccddeeff",
		ModelCode: u32(uint32(codec.ModelQS1A)), // cap 1600
	})
	s.ApplyTelemetry(&wire.Telemetry{
		PeerUid:      "aabbccddeeff",
		TsMs:         now.UnixMilli(),
		Model:        "QS1A",
		GridV:        230.5,
		FreqHz:       50.01,
		ActivePowerW: 800,
		Panels: []*wire.Panel{
			{Index: 0, DcV: 35.2, DcI: 5.1, W: 179.5},
			{Index: 1, DcV: 34.9, DcI: 5.0, W: 174.5},
		},
	})

	f := s.Fleet(now)
	if f.InverterCount != 1 || f.OnlineCount != 1 {
		t.Fatalf("count/online = %d/%d, want 1/1", f.InverterCount, f.OnlineCount)
	}
	inv := f.Inverters[0]
	if inv.NameplateW != 1600 {
		t.Errorf("NameplateW = %d, want 1600", inv.NameplateW)
	}
	if got := inv.LoadPct; got < 49.9 || got > 50.1 {
		t.Errorf("LoadPct = %.2f, want ~50", got)
	}
	if !inv.Online {
		t.Errorf("Online = false, want true (age 0)")
	}
	if len(inv.Panels) != 2 || inv.Panels[1].DCA != 5.0 {
		t.Errorf("panels not mapped: %+v", inv.Panels)
	}
	if f.ActivePowerW != 800 {
		t.Errorf("fleet ActivePowerW = %.0f, want 800", f.ActivePowerW)
	}
	// no FleetSummary applied => nameplate summed locally
	if f.NameplateTotalW != 1600 {
		t.Errorf("NameplateTotalW = %d, want 1600 (local sum)", f.NameplateTotalW)
	}
}

func TestOnlineGoesStale(t *testing.T) {
	base := time.UnixMilli(5_000_000)
	s := New(nil)
	s.ApplyTelemetry(&wire.Telemetry{PeerUid: "u", TsMs: base.UnixMilli(), ActivePowerW: 100})

	fresh := s.Fleet(base.Add(10 * time.Second))
	if !fresh.Inverters[0].Online {
		t.Fatalf("expected online within window")
	}
	stale := s.Fleet(base.Add(2 * time.Minute))
	if stale.Inverters[0].Online {
		t.Fatalf("expected offline after window")
	}
	if stale.OnlineCount != 0 || stale.ActivePowerW != 0 {
		t.Errorf("offline inverter still counted: online=%d power=%.0f",
			stale.OnlineCount, stale.ActivePowerW)
	}
}

func TestFaults_OnlyActiveBitsEmitted(t *testing.T) {
	now := time.UnixMilli(1)
	s := New(nil)
	s.ApplyTelemetry(&wire.Telemetry{
		PeerUid: "u",
		TsMs:    now.UnixMilli(),
		Faults: &wire.InverterFaults{
			Family: &wire.InverterFaults_Ds3{Ds3: &wire.DS3Faults{
				AcOverVoltStage1: true,
			}},
		},
	})
	inv := s.Fleet(now).Inverters[0]
	if inv.Faults == nil {
		t.Fatal("expected faults JSON")
	}
	var m map[string]bool
	if err := json.Unmarshal(inv.Faults, &m); err != nil {
		t.Fatalf("faults json: %v", err)
	}
	if !m["ac_over_volt_stage1"] {
		t.Errorf("expected ac_over_volt_stage1 active, got %v", m)
	}
	if _, present := m["grid_relay_fault"]; present {
		t.Errorf("inactive bit should be omitted, got %v", m)
	}
}

func TestNoFaults_EmitsNothing(t *testing.T) {
	now := time.UnixMilli(1)
	s := New(nil)
	s.ApplyTelemetry(&wire.Telemetry{
		PeerUid: "u", TsMs: now.UnixMilli(),
		Faults: &wire.InverterFaults{Family: &wire.InverterFaults_Ds3{Ds3: &wire.DS3Faults{}}},
	})
	if inv := s.Fleet(now).Inverters[0]; inv.Faults != nil {
		t.Errorf("expected nil faults for all-clear, got %s", inv.Faults)
	}
}

func TestApplyFleet_UsedOverLocalSum(t *testing.T) {
	now := time.UnixMilli(1)
	s := New(nil)
	s.ApplyTelemetry(&wire.Telemetry{PeerUid: "u", TsMs: now.UnixMilli()})
	s.ApplyFleet(&wire.FleetSummary{NameplateTotalW: 4321, TodayWh: 12345})
	f := s.Fleet(now)
	if f.NameplateTotalW != 4321 {
		t.Errorf("NameplateTotalW = %d, want 4321 (from summary)", f.NameplateTotalW)
	}
	if f.TodayWh != 12345 {
		t.Errorf("TodayWh = %d, want 12345", f.TodayWh)
	}
}

func TestOnChangeFires(t *testing.T) {
	n := 0
	s := New(func() { n++ })
	s.ApplyTelemetry(&wire.Telemetry{PeerUid: "u", TsMs: 1})
	s.ApplyInfo(&wire.InverterInfo{PeerUid: "u"})
	s.ApplyFleet(&wire.FleetSummary{})
	if n != 3 {
		t.Errorf("onChange fired %d times, want 3", n)
	}
}

func TestInvertersStableOrderByUID(t *testing.T) {
	now := time.UnixMilli(1)
	s := New(nil)
	for _, uid := range []string{"cc", "aa", "bb"} {
		s.ApplyTelemetry(&wire.Telemetry{PeerUid: uid, TsMs: now.UnixMilli()})
	}
	for i := 0; i < 5; i++ {
		got := s.Fleet(now).Inverters
		if len(got) != 3 || got[0].UID != "aa" || got[1].UID != "bb" || got[2].UID != "cc" {
			t.Fatalf("unstable/ wrong order: %v", []string{got[0].UID, got[1].UID, got[2].UID})
		}
	}
}

func u32(v uint32) *uint32 { return &v }

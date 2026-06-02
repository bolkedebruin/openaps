// Throwaway fake inv-driver for ecu-web smoke testing and screenshots.
// Listens on the given UDS path and:
//   - For SUBSCRIBER connections: publishes a synthetic living fleet
//     (FleetSummary + InverterInfo + Protection once; Telemetry every 2 s).
//   - For controller/PUBLISHER (ROLE_UNSPECIFIED) connections: answers
//     one-shot request/response ops — SystemStatus, Events, Settings,
//     GridProfile.
//
// All UIDs / MAC / PAN are synthetic ("999900000001" etc., MAC "00:11:22:33:44:55",
// PAN "ABCD"). Not part of the build (the underscore dir is ignored by
// `go build ./...`).
package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"time"

	"github.com/bolkedebruin/openaps/wire"
	"google.golang.org/protobuf/proto"
)

// Synthetic fleet — three inverters mixing DS3 (model code 0x18) and
// QS1A (model code 0x20). Two are AES-encrypted, one is plaintext (so the
// UI shows both encryption-badge states). One has an active overlay so
// the Profiles screen has something to display.
type fakeInv struct {
	UID        string
	ShortAddr  uint32
	Model      string
	ModelCode  uint32
	Phase      uint32
	SwVersion  uint32
	NameplateW float64
	GridV      float64
	FreqHz     float64
	Rssi       uint32
	Lqi        uint32
	Encrypted  bool
	Panels     []*wire.Panel
	Protection map[string]float64 // 60code -> value
}

var fakeFleet = []fakeInv{
	{
		UID: "999900000001", ShortAddr: 0x0101, Model: "DS3", ModelCode: 0x18, Phase: 1,
		SwVersion: 0x010207, NameplateW: 900, GridV: 230.4, FreqHz: 50.01,
		Rssi: 68, Lqi: 220, Encrypted: true,
		Panels: []*wire.Panel{
			{Index: 0, DcV: 41.2, DcI: 7.4, W: 305},
			{Index: 1, DcV: 41.0, DcI: 7.3, W: 299},
		},
		// DA=500 = uncapped (full). DS3 protection readback shape.
		Protection: map[string]float64{"DA": 500, "AB": 253, "CV": 50.2, "DD": 16.7, "DC": 51.5},
	},
	{
		UID: "999900000002", ShortAddr: 0x0102, Model: "QS1A", ModelCode: 0x20, Phase: 1,
		SwVersion: 0x020105, NameplateW: 1200, GridV: 230.7, FreqHz: 49.99,
		Rssi: 72, Lqi: 195, Encrypted: true,
		Panels: []*wire.Panel{
			{Index: 0, DcV: 35.1, DcI: 5.0, W: 175},
			{Index: 1, DcV: 34.8, DcI: 4.9, W: 171},
			{Index: 2, DcV: 35.0, DcI: 5.0, W: 175},
			{Index: 3, DcV: 34.7, DcI: 4.8, W: 167},
		},
		// DA=375 = curtailed to 75% (this is the inverter with an overlay).
		Protection: map[string]float64{"DA": 375, "AB": 253, "CV": 50.2, "CB": 51.5, "DD": 16.7},
	},
	{
		UID: "999900000003", ShortAddr: 0x0103, Model: "DS3", ModelCode: 0x18, Phase: 1,
		SwVersion: 0x010207, NameplateW: 900, GridV: 230.3, FreqHz: 50.00,
		Rssi: 75, Lqi: 165, Encrypted: false, // plaintext — UI flags this
		Panels: []*wire.Panel{
			{Index: 0, DcV: 40.7, DcI: 6.8, W: 277},
			{Index: 1, DcV: 40.9, DcI: 6.7, W: 274},
		},
		Protection: map[string]float64{"DA": 500, "AB": 253, "CV": 50.2, "DD": 16.7, "DC": 51.5},
	},
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: fakedriver <socket-path>")
	}
	sock := os.Args[1]
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("fakedriver listening", sock)
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go handle(c)
	}
}

func handle(c net.Conn) {
	defer c.Close()
	var hello wire.Envelope
	if err := wire.ReadFrame(c, &hello); err != nil {
		return
	}
	h := hello.GetHello()
	if h == nil {
		return
	}

	if h.GetRole() == wire.Role_SUBSCRIBER {
		runPublisher(c)
		return
	}
	// Controller — handle one or more request/response round-trips.
	runController(c)
}

// runPublisher: feed the subscriber a living fleet.
func runPublisher(c net.Conn) {
	now := time.Now().UnixMilli()
	// Identity envelopes (sent once on connect).
	for _, inv := range fakeFleet {
		mc := inv.ModelCode
		sw := inv.SwVersion
		ph := inv.Phase
		bound := true
		_ = send(c, &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{
			TsMs: now, PeerUid: inv.UID, ShortAddr: inv.ShortAddr,
			ModelCode: &mc, SoftwareVersion: &sw, Phase: &ph,
			ZigbeeBound: &bound,
		}}})
		_ = send(c, &wire.Envelope{Body: &wire.Envelope_Protection{Protection: &wire.Protection{
			TsMs: now, PeerUid: inv.UID, Model: inv.Model,
			Values: inv.Protection,
		}}})
	}
	// Fleet-level totals (so the dashboard shows lifetime/today/etc.).
	var nameplate float64
	for _, inv := range fakeFleet {
		nameplate += inv.NameplateW
	}
	_ = send(c, &wire.Envelope{Body: &wire.Envelope_Fleet{Fleet: &wire.FleetSummary{
		TsMs:            now,
		NameplateTotalW: uint32(nameplate),
		TodayWh:         8420,
		MonthWh:         264500,
		YearWh:          1840300,
		LifetimeWh:      4825000,
	}}})

	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	tick := 0
	for {
		now := time.Now().UnixMilli()
		for _, inv := range fakeFleet {
			// Vary AC power lightly so the dashboard sparkline / total moves.
			factor := 0.85 + 0.10*float64((tick%5))/4.0
			var panelW float64
			panels := make([]*wire.Panel, len(inv.Panels))
			for i, p := range inv.Panels {
				w := p.W * factor
				panels[i] = &wire.Panel{Index: p.Index, DcV: p.DcV, DcI: p.DcI, W: w}
				panelW += w
			}
			enc := inv.Encrypted
			tm := &wire.Telemetry{
				TsMs:         now,
				PeerUid:      inv.UID,
				Model:        inv.Model,
				ActivePowerW: panelW * 0.97,
				GridV:        inv.GridV,
				FreqHz:       inv.FreqHz,
				Rssi:         inv.Rssi,
				Lqi:          inv.Lqi,
				Panels:       panels,
				Encrypted:    &enc,
			}
			if err := send(c, &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: tm}}); err != nil {
				return
			}
		}
		tick++
		<-t.C
	}
}

// runController answers control-plane request envelopes one at a time.
func runController(c net.Conn) {
	for {
		var env wire.Envelope
		if err := wire.ReadFrame(c, &env); err != nil {
			return
		}
		resp := handleControl(&env)
		if resp == nil {
			return
		}
		if err := wire.WriteFrame(c, resp); err != nil {
			return
		}
	}
}

func handleControl(env *wire.Envelope) *wire.Envelope {
	now := time.Now().UnixMilli()
	switch {
	case env.GetSystemStatusReq() != nil:
		return &wire.Envelope{Body: &wire.Envelope_SystemStatusResp{SystemStatusResp: &wire.SystemStatusResponse{
			Ok: true, TsMs: now,
			Ecu: &wire.EcuIdentity{EcuId: "DEMO-ECU-9999", Hostname: "openaps-demo"},
			Peers: []*wire.PeerStatus{
				{Backend: "ecu-web", Version: "demo", Hostname: "openaps-demo", Role: "SUBSCRIBER", ConnectedAtMs: now - 30000, Controller: true},
				{Backend: "ecu-zb", Version: "demo", Hostname: "openaps-demo", Role: "PUBLISHER", ConnectedAtMs: now - 60000, Controller: true},
				{Backend: "ecu-sunspec", Version: "demo", Hostname: "openaps-demo", Role: "PUBLISHER", ConnectedAtMs: now - 60000, Controller: true},
			},
		}}}
	case env.GetEventsReq() != nil:
		return &wire.Envelope{Body: &wire.Envelope_EventsResp{EventsResp: &wire.EventsResponse{
			Ok: true, Events: synthEvents(now),
		}}}
	case env.GetSettingsReq() != nil:
		return &wire.Envelope{Body: &wire.Envelope_SettingsResp{SettingsResp: &wire.SettingsResponse{
			Ok: true,
			Settings: &wire.Settings{
				EcuId:       "DEMO-ECU-9999",
				Mac:         "00:11:22:33:44:55",
				PanOverride: "ABCD",
				ZigbeeType:  "apsystems",
				Channel:     16,
				InverterNames: map[string]string{
					"999900000001": "Roof East 1",
					"999900000002": "Roof West",
					"999900000003": "Roof East 2",
				},
			},
		}}}
	case env.GetGridProfileReq() != nil:
		return handleGridProfile(env.GetGridProfileReq())
	}
	return nil
}

func synthEvents(now int64) []*wire.Event {
	type e struct {
		ago int64
		k   string
		sev string
		uid string
		det string
		by  string
	}
	rows := []e{
		{30, "fleet_sunrise", "info", "", "fleet sunrise — first inverter online at 05:42", "inv-driver"},
		{60, "online", "info", "999900000001", "inverter rejoined ZigBee (rssi=-68 lqi=220)", "inv-driver"},
		{75, "online", "info", "999900000002", "inverter rejoined ZigBee (rssi=-72 lqi=195)", "inv-driver"},
		{90, "online", "info", "999900000003", "inverter rejoined ZigBee (rssi=-75 lqi=165 plaintext)", "inv-driver"},
		{300, "power_cap_set", "info", "999900000002", "DA=375 (75% of nameplate)", "ecu-web"},
		{420, "profile_select", "info", "", "active base profile = EN 50549-1 (NL)", "ecu-web"},
		{500, "overlay_set", "info", "999900000002", "Local Site overlay applied (curtailment)", "ecu-web"},
		{900, "settings_set", "info", "", "operator updated inverter labels", "ecu-web"},
		{1500, "protection_set", "info", "999900000001", "CV=50.2 DD=16.7 (over-freq droop)", "ecu-sunspec"},
		{3600, "fleet_sundown", "info", "", "fleet sundown — last inverter offline at 21:14", "inv-driver"},
	}
	out := make([]*wire.Event, 0, len(rows))
	for i, r := range rows {
		out = append(out, &wire.Event{
			Id: int64(1000 + i), TsMs: now - r.ago*1000,
			Kind: r.k, Severity: r.sev, InverterUid: r.uid, Detail: r.det, By: r.by,
		})
	}
	return out
}

func handleGridProfile(req *wire.GridProfileRequest) *wire.Envelope {
	type source struct {
		Ref string `json:"ref"`
	}
	type prof struct {
		ID         string `json:"id"`
		VNomV      int    `json:"vnom_v"`
		Source     source `json:"source"`
		PointCount int    `json:"point_count"`
	}
	type status struct {
		ActiveBase      string `json:"active_base"`
		ReconcilerReady bool   `json:"reconciler_ready"`
	}
	type defaultDTO struct {
		Value float64  `json:"value"`
		Unit  string   `json:"unit"`
		Min   *float64 `json:"min,omitempty"`
		Max   *float64 `json:"max,omitempty"`
	}
	type pointEntry struct {
		Apply struct {
			ApsCode string `json:"aps_code"`
		} `json:"apply"`
		Native struct {
			Value float64 `json:"value"`
			Unit  string  `json:"unit"`
		} `json:"native"`
	}
	type overlay struct {
		Schema string       `json:"schema"`
		ID     string       `json:"id"`
		VNomV  float64      `json:"vnom_v,omitempty"`
		UIDs   []string     `json:"uids"`
		Points []pointEntry `json:"points"`
	}

	mkPoint := func(code string, val float64, unit string) pointEntry {
		var p pointEntry
		p.Apply.ApsCode = code
		p.Native.Value = val
		p.Native.Unit = unit
		return p
	}

	respJSON := func(v any) *wire.Envelope {
		b, _ := json.Marshal(v)
		return &wire.Envelope{Body: &wire.Envelope_GridProfileResp{GridProfileResp: &wire.GridProfileResponse{
			Ok: true, Json: b,
		}}}
	}

	switch req.Op.(type) {
	case *wire.GridProfileRequest_GetStatus:
		return respJSON(status{ActiveBase: "en50549-1-nl", ReconcilerReady: true})
	case *wire.GridProfileRequest_ListProfiles:
		return respJSON([]prof{
			{ID: "en50549-1-nl", VNomV: 230, Source: source{Ref: "EN 50549-1 (NL Liander)"}, PointCount: 12},
			{ID: "vde-ar-n-4105", VNomV: 230, Source: source{Ref: "VDE-AR-N 4105 (DE)"}, PointCount: 12},
			{ID: "c10-11", VNomV: 230, Source: source{Ref: "Synergrid C10/11 (BE)"}, PointCount: 12},
			{ID: "en50549-1-default", VNomV: 230, Source: source{Ref: "EN 50549-1 default"}, PointCount: 12},
		})
	case *wire.GridProfileRequest_GetBase:
		f := func(v float64) *float64 { return &v }
		return respJSON(map[string]defaultDTO{
			"AB": {Value: 253, Unit: "V", Min: f(200), Max: f(280)},
			"CV": {Value: 50.2, Unit: "Hz", Min: f(50), Max: f(52)},
			"CB": {Value: 51.5, Unit: "Hz", Min: f(50), Max: f(53)},
			"DD": {Value: 16.7, Unit: "%/Hz", Min: f(0), Max: f(50)},
			"DC": {Value: 51.5, Unit: "Hz"},
			"CA": {Value: 50.2, Unit: "Hz"},
		})
	case *wire.GridProfileRequest_ListOverlays:
		return respJSON([]overlay{{
			Schema: "invdriver.gridprofile/v1",
			ID:     "local-curtailment-75pct",
			VNomV:  230,
			UIDs:   []string{"999900000002"},
			Points: []pointEntry{
				mkPoint("CV", 50.5, "Hz"),
				mkPoint("DD", 12.0, "%/Hz"),
			},
		}})
	}
	// Unknown op: empty ok response so the client doesn't error out.
	return &wire.Envelope{Body: &wire.Envelope_GridProfileResp{GridProfileResp: &wire.GridProfileResponse{Ok: true, Json: []byte("null")}}}
}

func send(c net.Conn, e *wire.Envelope) error {
	if e == nil {
		return nil
	}
	return wire.WriteFrame(c, e)
}

// silence unused-import lint if proto isn't referenced elsewhere
var _ = proto.Marshal

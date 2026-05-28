// Throwaway fake inv-driver for ecu-web smoke testing. Emits a small,
// living fleet over the UDS so the web UI has data. Not part of the
// build (underscore dir is ignored by `go ./...`).
package main

import (
	"log"
	"net"
	"os"
	"time"

	"github.com/bolke/inv-driver/wire"
)

func main() {
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
	send(c, &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{PeerUid: "aabbccddeeff", ModelCode: u32(0x18)}}})
	send(c, &wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{PeerUid: "112233445566", ModelCode: u32(0x20)}}})
	// Output-cap read-backs (code "DA", per-panel where 500 = full): QS1A
	// curtailed to 75% (DA=375), DS3 uncapped (DA=500).
	send(c, &wire.Envelope{Body: &wire.Envelope_Protection{Protection: &wire.Protection{PeerUid: "aabbccddeeff", Model: "QS1A", Values: map[string]float64{"DA": 375}}}})
	send(c, &wire.Envelope{Body: &wire.Envelope_Protection{Protection: &wire.Protection{PeerUid: "112233445566", Model: "DS3", Values: map[string]float64{"DA": 500}}}})
	send(c, &wire.Envelope{Body: &wire.Envelope_Fleet{Fleet: &wire.FleetSummary{NameplateTotalW: 2350, TodayWh: 4200, MonthWh: 120000, YearWh: 980000, LifetimeWh: 1234567}}})
	for {
		now := time.Now().UnixMilli()
		send(c, &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
			PeerUid: "aabbccddeeff", TsMs: now, Model: "QS1A", ActivePowerW: 800, GridV: 230.5, FreqHz: 50.01, Rssi: 70, Lqi: 200,
			Panels: []*wire.Panel{{Index: 0, DcV: 35.2, DcI: 5.1, W: 179}, {Index: 1, DcV: 34.9, DcI: 5.0, W: 174}},
		}}})
		send(c, &wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
			PeerUid: "112233445566", TsMs: now, Model: "DS3", ActivePowerW: 600, GridV: 231.0, FreqHz: 50.0, Rssi: 60, Lqi: 180,
			Panels: []*wire.Panel{{Index: 0, DcV: 40, DcI: 7.5, W: 300}, {Index: 1, DcV: 39, DcI: 7.6, W: 300}},
		}}})
		if err := send(c, nil); err != nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
}

func send(c net.Conn, e *wire.Envelope) error {
	if e == nil {
		return nil
	}
	return wire.WriteFrame(c, e)
}

func u32(v uint32) *uint32 { return &v }

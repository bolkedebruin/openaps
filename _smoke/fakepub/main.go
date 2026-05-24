// Fake publisher CLIENT for local UI verification: dials inv-driver's
// UDS (like ecu-zb), says Hello as a publisher, then streams Info +
// Telemetry for a few inverters so the real inv-driver ingests them and
// the web UI shows live data. Not part of the build (underscore dir).
package main

import (
	"log"
	"math"
	"net"
	"os"
	"time"

	"github.com/bolke/inv-driver/wire"
)

func main() {
	sock := os.Args[1]
	conn, err := net.Dial("unix", sock)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	send := func(e *wire.Envelope) {
		if err := wire.WriteFrame(conn, e); err != nil {
			log.Fatal(err)
		}
	}
	send(&wire.Envelope{Body: &wire.Envelope_Hello{Hello: &wire.Hello{Backend: "fake-zb", Version: "0", StartedAtMs: time.Now().UnixMilli()}}})

	type inv struct {
		uid   string
		model string
		cmd   uint32
		mc    uint32
		base  float64
		chans int
	}
	invs := []inv{
		{"704000006835", "DS3", 0xBB, 0x20, 480, 2},
		{"806000041211", "QS1A", 0xB1, 0x18, 1120, 4},
		{"806000042582", "QS1A", 0xB1, 0x18, 1010, 4},
	}
	for _, v := range invs {
		send(&wire.Envelope{Body: &wire.Envelope_Info{Info: &wire.InverterInfo{PeerUid: v.uid, ModelCode: &v.mc}}})
	}
	i := 0
	for {
		now := time.Now().UnixMilli()
		for _, v := range invs {
			wobble := math.Sin(float64(i)/8) * 120
			acw := v.base + wobble
			panels := make([]*wire.Panel, v.chans)
			for c := 0; c < v.chans; c++ {
				panels[c] = &wire.Panel{Index: int32(c), DcV: 35 + float64(c)*0.3, DcI: 8.1, W: acw / float64(v.chans)}
			}
			send(&wire.Envelope{Body: &wire.Envelope_Telemetry{Telemetry: &wire.Telemetry{
				PeerUid: v.uid, TsMs: now, Cmd: v.cmd, Model: v.model,
				GridV: 240.2, BusV: 80, FreqHz: 50.02, ActivePowerW: acw, Rssi: 70, Lqi: 200,
				Panels: panels,
			}}})
		}
		i++
		time.Sleep(1 * time.Second)
	}
}

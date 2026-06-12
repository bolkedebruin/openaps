package main

import "testing"

func TestWatchdogEnabled(t *testing.T) {
	cases := []struct {
		name    string
		haveBus bool
		opPAN   uint16
		want    bool
	}{
		{"bus and pan", true, 0x0DCE, true},
		{"bus but no pan", true, 0, false},
		{"pan but no bus", false, 0x0DCE, false},
		{"neither", false, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := watchdogEnabled(c.haveBus, c.opPAN); got != c.want {
				t.Fatalf("watchdogEnabled(%v, 0x%04X) = %v, want %v", c.haveBus, c.opPAN, got, c.want)
			}
		})
	}
}

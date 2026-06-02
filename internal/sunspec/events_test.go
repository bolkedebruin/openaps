package sunspec

import "testing"

// makeBits returns a [4]uint32 with the given APsystems bit positions set.
func makeBits(positions ...int) [4]uint32 {
	var b [4]uint32
	for _, p := range positions {
		if p >= 0 && p < 128 {
			b[p/32] |= 1 << uint(p%32)
		}
	}
	return b
}

func TestMapAPsystemsToSunSpecEvt1(t *testing.T) {
	cases := []struct {
		name string
		set  []int  // APsystems bit positions
		want uint32 // expected SunSpec Evt1
	}{
		{"all clear", nil, 0},

		{"GFDI Locked → GroundFault",
			[]int{17}, Evt1GroundFault},

		{"DC over-volt on channel A",
			[]int{8}, Evt1DCOverVolt},
		{"DC over-volt on channel C (bit 39)",
			[]int{39}, Evt1DCOverVolt},
		{"DC over-volt on multiple channels collapses to one bit",
			[]int{8, 10, 39, 41}, Evt1DCOverVolt},

		{"AC Disconnect", []int{19}, Evt1ACDisconnect},
		{"Remote Shut → ManualShutdown", []int{18}, Evt1ManualShutdown},
		{"Over Critical Temperature", []int{16}, Evt1OverTemp},

		{"Over-frequency stage 1", []int{29}, Evt1OverFrequency},
		{"Over-frequency stage 2", []int{31}, Evt1OverFrequency},
		{"Under-frequency stage 1", []int{30}, Evt1UnderFrequency},

		{"AC over-volt channel A", []int{2}, Evt1ACOverVolt},
		{"AC over-volt stage 1", []int{44}, Evt1ACOverVolt},
		{"AC under-volt channel C", []int{7}, Evt1ACUnderVolt},

		{"Active Anti-island → GridDisconnect",
			[]int{21}, Evt1GridDisconnect},
		{"Relay Failed → HWTestFailure",
			[]int{28}, Evt1HWTestFailure},

		{"Combined: GFDI + Over Temp + AC over-volt",
			[]int{17, 16, 23},
			Evt1GroundFault | Evt1OverTemp | Evt1ACOverVolt},

		{"Channel B DC low (bit 11) maps to NOTHING",
			// "DC voltage too low" has no SunSpec analog — should not produce
			// any standard Evt1 bit.
			[]int{11}, 0},

		{"Live data sample: bits 9 + 11 set",
			// What we observed on this ECU's DS3 — both DC-low flags. Should
			// stay invisible at the standard SunSpec level (raw bits remain
			// in EvtVnd1).
			[]int{9, 11}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MapAPsystemsToSunSpecEvt1(makeBits(c.set...))
			if got != c.want {
				t.Errorf("set=%v: got Evt1=%#08x want %#08x", c.set, got, c.want)
			}
		})
	}
}

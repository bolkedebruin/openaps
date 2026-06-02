package source

import "testing"

// liveSystemPowerW must reflect the *current* sum of online inverters'
// AC power, even when that sum is zero. The pre-fix builder kept the
// stale `each_system_power` DB row when the sum was zero — at dusk
// (online=N but every inverter producing 0 W) that row's pre-sundown
// value would persist as the reported system power, which the
// encoder then translated to StMPPT + phantom watts on the wire.
func TestLiveSystemPowerW(t *testing.T) {
	cases := []struct {
		name string
		invs []Inverter
		want int32
	}{
		{
			name: "all offline (full night)",
			invs: []Inverter{
				{UID: "a", Online: false, ACPowerW: 0},
				{UID: "b", Online: false, ACPowerW: 0},
			},
			want: 0,
		},
		{
			name: "all offline, params still carry stale ACPowerW (firmware quirk)",
			// Some firmware versions leave ACPowerW non-zero in the
			// last row before flipping Online=0. Don't sum offline
			// readings.
			invs: []Inverter{
				{UID: "a", Online: false, ACPowerW: 250},
				{UID: "b", Online: false, ACPowerW: 240},
			},
			want: 0,
		},
		{
			name: "online but zero power (dusk window — the regression)",
			invs: []Inverter{
				{UID: "a", Online: true, ACPowerW: 0},
				{UID: "b", Online: true, ACPowerW: 0},
				{UID: "c", Online: true, ACPowerW: 0},
				{UID: "d", Online: true, ACPowerW: 0},
			},
			want: 0,
		},
		{
			name: "mixed online + idle, only producers count",
			invs: []Inverter{
				{UID: "a", Online: true, ACPowerW: 250},
				{UID: "b", Online: true, ACPowerW: 0},
				{UID: "c", Online: false, ACPowerW: 0},
				{UID: "d", Online: true, ACPowerW: 230},
			},
			want: 480,
		},
		{
			name: "all active (sun-up)",
			invs: []Inverter{
				{UID: "a", Online: true, ACPowerW: 250},
				{UID: "b", Online: true, ACPowerW: 245},
				{UID: "c", Online: true, ACPowerW: 260},
				{UID: "d", Online: true, ACPowerW: 235},
			},
			want: 990,
		},
		{
			name: "no inverters at all",
			invs: nil,
			want: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := liveSystemPowerW(c.invs)
			if got != c.want {
				t.Errorf("got=%d want=%d", got, c.want)
			}
		})
	}
}

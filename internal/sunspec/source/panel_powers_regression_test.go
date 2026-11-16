package source

import (
	"reflect"
	"testing"
)

func TestPanelPowersMissingMeasurements(t *testing.T) {
	cases := []struct {
		name string
		inv  Inverter
		want []int
	}{
		{
			name: "two measured DS3 channels",
			inv: Inverter{
				Online:     true,
				TypeCode:   "01",
				PanelWatts: []int{80, 90},
			},
			want: []int{80, 90},
		},
		{
			name: "missing second DS3 measurement",
			inv: Inverter{
				Online:     true,
				TypeCode:   "01",
				PanelWatts: []int{80},
			},
			want: []int{80, -1},
		},
		{
			name: "genuine zero is preserved",
			inv: Inverter{
				Online:     true,
				TypeCode:   "01",
				PanelWatts: []int{0, 90},
			},
			want: []int{0, 90},
		},
		{
			name: "missing QS2 measurements",
			inv: Inverter{
				Online:     true,
				TypeCode:   "03",
				PanelWatts: []int{40, 41},
			},
			want: []int{40, 41, -1, -1},
		},
		{
			name: "offline DS3",
			inv: Inverter{
				Online:     false,
				TypeCode:   "01",
				PanelWatts: []int{80, 90},
			},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.inv.PanelPowers()
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("PanelPowers() = %v, want %v", got, tc.want)
			}
		})
	}
}

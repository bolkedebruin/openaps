package sunspec

import "testing"

func TestBank_Contains(t *testing.T) {
	b := Encode(sampleSnapshot(), Options{})
	end := b.Base + uint16(len(b.Regs))

	cases := []struct {
		addr, count uint16
		want        bool
		label       string
	}{
		{b.Base, 1, true, "first reg"},
		{b.Base, uint16(len(b.Regs)), true, "exact bank"},
		{b.Base + uint16(len(b.Regs)) - 1, 1, true, "last reg"},
		{b.Base - 1, 2, false, "straddles low boundary"},
		{end - 1, 2, false, "straddles high boundary"},
		{end, 1, false, "first reg past bank"},
		{0, 1, false, "address far below"},
		{50000, 1, false, "address far above"},
		{b.Base, 0, false, "zero count"},
	}
	for _, c := range cases {
		if got := b.Contains(c.addr, c.count); got != c.want {
			t.Errorf("%s: addr=%d count=%d -> %v, want %v",
				c.label, c.addr, c.count, got, c.want)
		}
	}
}

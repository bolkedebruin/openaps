package wire

import "testing"

func TestTypeCodeForModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		model string
		want  string
	}{
		{"QS1A", "03"},
		{"QS1", "03"},
		{"QS1A-foo", "03"},
		{"DS3", "01"},
		{"DS3D", "01"},
		{"DSP", ""},
		{"YC600", ""},
		{"YC1000", ""},
		{"", ""},
		{"unknown(0xAA)", ""},
	}
	for _, c := range cases {
		c := c
		t.Run(c.model, func(t *testing.T) {
			t.Parallel()
			if got := TypeCodeForModel(c.model); got != c.want {
				t.Errorf("TypeCodeForModel(%q) = %q, want %q", c.model, got, c.want)
			}
		})
	}
}

package codec

import "testing"

// qs2TestBody builds a synthetic QS2 reply body (offset 0 = byte after
// the 0xBB cmd) with distinct values in every decoded field.
func qs2TestBody() []byte {
	b := make([]byte, 0x44)
	put16 := func(off int, v uint16) { b[off] = byte(v >> 8); b[off+1] = byte(v) }
	put32 := func(off int, v uint32) {
		b[off] = byte(v >> 24)
		b[off+1] = byte(v >> 16)
		b[off+2] = byte(v >> 8)
		b[off+3] = byte(v)
	}
	put16(0x10, 1000) // A V
	put16(0x12, 1100) // B V
	put16(0x14, 200)  // A I
	put16(0x16, 210)  // B I
	put16(0x18, 880)  // grid V
	put16(0x1a, 5001) // freq
	put16(0x1c, 1234) // report counter
	put16(0x1e, 750)  // active W
	put16(0x20, 50)   // reactive VAR
	put16(0x34, 1200) // C V
	put16(0x36, 1300) // D V
	put16(0x38, 220)  // C I
	put16(0x3a, 230)  // D I
	put32(0x28, 111)  // A lifetime
	put32(0x2c, 222)  // B lifetime
	put32(0x3c, 333)  // C lifetime
	put32(0x40, 444)  // D lifetime
	return b
}

func TestDecodeQS2_FourChannels(t *testing.T) {
	var r Reply
	decodeQS2(qs2TestBody(), &r)

	if len(r.Panels) != 4 {
		t.Fatalf("panels = %d, want 4", len(r.Panels))
	}
	want := []struct{ v, i float64 }{
		{fround(1000 * qs2PanelVScale), fround(200 * qs2PanelIScale)},
		{fround(1100 * qs2PanelVScale), fround(210 * qs2PanelIScale)},
		{fround(1200 * qs2PanelVScale), fround(220 * qs2PanelIScale)},
		{fround(1300 * qs2PanelVScale), fround(230 * qs2PanelIScale)},
	}
	for idx, w := range want {
		p := r.Panels[idx]
		if p.Index != idx || p.DCV != w.v || p.DCI != w.i {
			t.Errorf("panel %d = %+v, want Index=%d V=%g I=%g", idx, p, idx, w.v, w.i)
		}
	}
	if r.GridV != fround(880*qs2GridVScale) {
		t.Errorf("GridV = %v, want %v", r.GridV, fround(880*qs2GridVScale))
	}
	if r.FreqHz != fround(5001*qs2FreqScale) {
		t.Errorf("FreqHz = %v, want %v", r.FreqHz, fround(5001*qs2FreqScale))
	}
	if r.ReportSec != 1234 {
		t.Errorf("ReportSec = %d, want 1234", r.ReportSec)
	}
	if r.ActivePowerW != 750 {
		t.Errorf("ActivePowerW = %v, want 750", r.ActivePowerW)
	}
	if r.ReactivePower != 50 {
		t.Errorf("ReactivePower = %v, want 50", r.ReactivePower)
	}
	if g := r.LifetimeRaw; len(g) != 4 || g[0] != 111 || g[1] != 222 || g[2] != 333 || g[3] != 444 {
		t.Errorf("LifetimeRaw = %v, want [111 222 333 444]", g)
	}
	if r.LifetimeScale != qs2Lifetime {
		t.Errorf("LifetimeScale = %v, want %v", r.LifetimeScale, qs2Lifetime)
	}
}

// TestDecodeQS2_NegativeReactive confirms reactive power is signed.
func TestDecodeQS2_NegativeReactive(t *testing.T) {
	b := qs2TestBody()
	b[0x20], b[0x21] = 0xFF, 0xCE // -50
	var r Reply
	decodeQS2(b, &r)
	if r.ReactivePower != -50 {
		t.Errorf("ReactivePower = %v, want -50", r.ReactivePower)
	}
}

// TestDS3ClassDispatch routes the shared 0xBB reply by model code: QS2
// decodes four channels, DS3 (and an unknown/zero model) decode two.
func TestDS3ClassDispatch(t *testing.T) {
	body := qs2TestBody()
	cases := []struct {
		name      string
		model     uint8
		wantPanel int
	}{
		{"QS2", ModelQS2, 4},
		{"DS3", ModelDS3, 2},
		{"unknown falls back to DS3", 0, 2},
	}
	for _, c := range cases {
		var r Reply
		decodeDS3Class(c.model, body, &r)
		if len(r.Panels) != c.wantPanel {
			t.Errorf("%s: panels = %d, want %d", c.name, len(r.Panels), c.wantPanel)
		}
	}
}

func TestModelLabel_PrefersModelCode(t *testing.T) {
	// QS2 shares cmd 0xBB with DS3; the model code names it.
	if got := (Reply{Cmd: CmdReplyDS3, ModelCode: ModelQS2}).ModelLabel(); got != "QS2" {
		t.Errorf("QS2 label = %q, want QS2", got)
	}
	// Without a model code, the cmd byte still resolves DS3.
	if got := (Reply{Cmd: CmdReplyDS3}).ModelLabel(); got != "DS3" {
		t.Errorf("no-model label = %q, want DS3", got)
	}
}

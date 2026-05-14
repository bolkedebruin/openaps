package paramsfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bolke/inv-driver/wire"
)

func qs1aTel(uid string, freq, gridV, busV float64, p0, p1, p2, p3 float64) *wire.Telemetry {
	return &wire.Telemetry{
		PeerUid: uid,
		Cmd:     0xB1,
		Model:   "QS1A",
		FreqHz:  freq,
		GridV:   gridV,
		BusV:    busV,
		Panels: []*wire.Panel{
			{Index: 0, W: p0},
			{Index: 1, W: p1},
			{Index: 2, W: p2},
			{Index: 3, W: p3},
		},
	}
}

func ds3Tel(uid string, freq, gridV float64, p0, p1 float64) *wire.Telemetry {
	return &wire.Telemetry{
		PeerUid: uid,
		Cmd:     0xBB,
		Model:   "DS3",
		FreqHz:  freq,
		GridV:   gridV,
		Panels: []*wire.Panel{
			{Index: 0, W: p0},
			{Index: 1, W: p1},
		},
	}
}

func TestRender_HeaderHasCountAndTimestamp(t *testing.T) {
	u := New("/tmp/unused")
	u.Now = func() time.Time {
		return time.Date(2026, 5, 4, 17, 12, 0, 0, time.Local)
	}
	if err := u.Update(qs1aTel("806000042582", 50.0, 223.0, 24, 55, 54, 55, 33)); err == nil {
		// Update tries to write the file too; for this in-memory test
		// just ignore Path errors.
	}
	out := u.Render()
	first, _, ok := strings.Cut(out, "\n")
	if !ok {
		t.Fatalf("expected at least 2 lines, got %q", out)
	}
	if first != "01,1,20260504171200" {
		t.Fatalf("header: got %q want %q", first, "01,1,20260504171200")
	}
}

func TestRender_QS1ALineMatchesMainExeShape(t *testing.T) {
	// Reference shape from a live capture:
	//   806000042582,1,03,50.0,124,55,223,54,55,33
	u := New("/tmp/unused")
	u.Now = func() time.Time {
		return time.Date(2026, 5, 4, 17, 12, 0, 0, time.Local)
	}
	_ = u.Update(qs1aTel("806000042582", 50.0, 223.0, 24, 55, 54, 55, 33))

	out := u.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), out)
	}
	if lines[1] != "806000042582,1,03,50.0,124,55,223,54,55,33" {
		t.Fatalf("inverter line:\n  got  %q\n  want %q",
			lines[1], "806000042582,1,03,50.0,124,55,223,54,55,33")
	}
}

func TestRender_DS3LineMatchesMainExeShape(t *testing.T) {
	// Reference shape from live capture:
	//   704000006835,1,01,50.0,129,51,232,30,232
	// DS3 temp uses NTC formula not ported yet; we emit 125 (=25°C
	// after the -100 offset). That matches the format and order;
	// only the absolute temp value differs from the live sample.
	u := New("/tmp/unused")
	u.Now = func() time.Time {
		return time.Date(2026, 5, 4, 17, 12, 0, 0, time.Local)
	}
	_ = u.Update(ds3Tel("704000006835", 50.0, 232.0, 51, 30))

	out := u.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[1] != "704000006835,1,01,50.0,125,51,232,30,232" {
		t.Fatalf("inverter line:\n  got  %q\n  want %q",
			lines[1], "704000006835,1,01,50.0,125,51,232,30,232")
	}
}

func TestRender_OfflineAfterTimeout(t *testing.T) {
	u := New("/tmp/unused")
	now := time.Date(2026, 5, 4, 17, 12, 0, 0, time.Local)
	u.Now = func() time.Time { return now }
	u.OfflineAfter = 30 * time.Second

	_ = u.Update(qs1aTel("806000042582", 50.0, 223.0, 24, 55, 54, 55, 33))

	// Advance the clock past OfflineAfter without a new reply.
	now = now.Add(45 * time.Second)
	out := u.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[1] != "806000042582,0,03" {
		t.Fatalf("offline line: got %q want %q",
			lines[1], "806000042582,0,03")
	}
}

func TestRender_StableUIDOrdering(t *testing.T) {
	u := New("/tmp/unused")
	u.Now = func() time.Time { return time.Date(2026, 5, 4, 17, 12, 0, 0, time.Local) }
	// Updates in non-sorted order — Render() must sort by UID for
	// deterministic output.
	_ = u.Update(qs1aTel("806000041211", 50.0, 223.0, 25, 50, 50, 50, 50))
	_ = u.Update(ds3Tel("704000006835", 50.0, 232.0, 50, 50))
	_ = u.Update(qs1aTel("806000042582", 50.0, 223.0, 24, 55, 54, 55, 33))

	out := u.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	wantOrder := []string{"704000006835", "806000041211", "806000042582"}
	for i, want := range wantOrder {
		if !strings.HasPrefix(lines[i+1], want) {
			t.Fatalf("line %d: prefix %q want %q", i+1, lines[i+1], want)
		}
	}
}

func TestUpdate_AtomicFileWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parameters_app.conf")
	u := New(path)
	u.Now = func() time.Time { return time.Date(2026, 5, 4, 17, 12, 0, 0, time.Local) }
	if err := u.Update(qs1aTel("806000042582", 50.0, 223.0, 24, 55, 54, 55, 33)); err != nil {
		t.Fatalf("Update: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "806000042582,1,03,50.0,124,55,223,54,55,33") {
		t.Fatalf("file content missing inverter line:\n%s", body)
	}
	// Confirm no leftover temp files.
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".params-") {
			t.Fatalf("temp file leaked: %s", e.Name())
		}
	}
}

func TestUpdate_RejectsUnknownModel(t *testing.T) {
	u := New("/tmp/unused")
	err := u.Update(&wire.Telemetry{PeerUid: "abc", Model: "Mystery"})
	if err == nil {
		t.Fatal("expected error for unknown model, got nil")
	}
}

func TestUpdate_RejectsEmptyUID(t *testing.T) {
	u := New("/tmp/unused")
	err := u.Update(&wire.Telemetry{Model: "QS1A"})
	if err == nil {
		t.Fatal("expected error for empty UID, got nil")
	}
}

func TestUpdate_RejectsNil(t *testing.T) {
	u := New("/tmp/unused")
	if err := u.Update(nil); err == nil {
		t.Fatal("expected error for nil telemetry, got nil")
	}
}

func TestTruncFreq_DecimalTruncation(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{50.06, "50.0"},
		{50.10, "50.1"},
		{49.97, "49.9"},
		{0, "0.0"},
	}
	for _, c := range cases {
		if got := truncFreq(c.in); got != c.want {
			t.Errorf("truncFreq(%v): got %q want %q", c.in, got, c.want)
		}
	}
}

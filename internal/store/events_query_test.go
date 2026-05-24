package store

import (
	"context"
	"testing"
)

func TestQueryEvents_FiltersAndOrder(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.AppendEvent(ctx, 100, "uidA", "paired", "info"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, 200, "uidB", "fault_detected", "error"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendDecodeFailed(ctx, 300, 7, "bad crc", "FCFC00"); err != nil {
		t.Fatal(err)
	}

	// Newest-first, all kinds.
	all, err := s.QueryEvents(ctx, EventFilter{Limit: 10, Desc: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d events, want 3", len(all))
	}
	if all[0].TsMs != 300 || all[2].TsMs != 100 {
		t.Errorf("not newest-first: %d..%d", all[0].TsMs, all[2].TsMs)
	}
	// decode_failed carries short_addr + detail + raw_hex.
	if all[0].Kind != "decode_failed" || all[0].ShortAddr != 7 || all[0].Detail != "bad crc" || all[0].RawHex != "FCFC00" {
		t.Errorf("decode_failed row wrong: %+v", all[0])
	}

	// since_ms filter.
	recent, _ := s.QueryEvents(ctx, EventFilter{SinceMs: 250, Limit: 10})
	if len(recent) != 1 || recent[0].TsMs != 300 {
		t.Errorf("since_ms filter wrong: %+v", recent)
	}

	// kind filter.
	byKind, _ := s.QueryEvents(ctx, EventFilter{Kind: "paired", Limit: 10})
	if len(byKind) != 1 || byKind[0].InverterUID != "uidA" {
		t.Errorf("kind filter wrong: %+v", byKind)
	}

	// severity filter.
	bySev, _ := s.QueryEvents(ctx, EventFilter{Severity: "error", Limit: 10})
	if len(bySev) != 1 || bySev[0].Kind != "fault_detected" {
		t.Errorf("severity filter wrong: %+v", bySev)
	}

	// limit.
	one, _ := s.QueryEvents(ctx, EventFilter{Limit: 1, Desc: true})
	if len(one) != 1 || one[0].TsMs != 300 {
		t.Errorf("limit wrong: %+v", one)
	}
}

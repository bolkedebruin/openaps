package store

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// TestConcurrentWritesNoBusy is the regression for the SQLITE_BUSY an
// operator's SetInverterPhase upsert hit while the telemetry path was writing:
// with _txlock=immediate + busy_timeout, contending writers wait rather than
// fail on a lock-upgrade conflict.
func TestConcurrentWritesNoBusy(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/state.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	mc := uint32(0x18)
	for i := 0; i < 5; i++ {
		uid := fmt.Sprintf("80000000000%d", i)
		if _, err := st.UpsertInverterInfo(ctx, InverterInfoUpdate{UID: uid, TsMs: 1, ShortAddr: uint16(100 + i), ModelCode: &mc}); err != nil {
			t.Fatalf("seed %s: %v", uid, err)
		}
	}

	var wg sync.WaitGroup
	errc := make(chan error, 128)

	// Operator-style phase upserts (read-probe then write) contend with...
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				leg := uint32(i%3 + 1)
				if _, err := st.UpsertInverterInfo(ctx, InverterInfoUpdate{
					UID: fmt.Sprintf("80000000000%d", i%5), TsMs: int64(i), Phase: &leg, PreserveLastSeen: true,
				}); err != nil {
					errc <- err
					return
				}
			}
		}()
	}
	// ...telemetry-audit-style event appends (autocommit writes).
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if err := st.AppendEvent(ctx, int64(i), "", "test", "info", "x", ""); err != nil {
					errc <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errc)
	for err := range errc {
		t.Fatalf("concurrent write failed (expected none with immediate txlock + busy_timeout): %v", err)
	}
}

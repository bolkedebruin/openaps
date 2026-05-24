// Seeds a few rows into inv-driver's events table for smoke testing the
// Events API. Not part of the build (underscore dir).
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/bolke/inv-driver/internal/store"
)

func main() {
	st, err := store.Open(context.Background(), os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	now := time.Now().UnixMilli()
	_ = st.AppendEvent(ctx, now-3000, "aabbccddeeff", "paired", "info")
	_ = st.AppendEvent(ctx, now-2000, "112233445566", "fault_detected", "error")
	_ = st.AppendDecodeFailed(ctx, now-1000, 7, "short frame", "FC FC 00 11")
	log.Println("seeded 3 events")
}

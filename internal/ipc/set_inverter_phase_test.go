package ipc

import (
	"context"
	"database/sql"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/bolkedebruin/openaps/internal/events"
	"github.com/bolkedebruin/openaps/internal/ingest"
	"github.com/bolkedebruin/openaps/internal/store"
	"github.com/bolkedebruin/openaps/wire"
)

func seedInverter(t *testing.T, st *store.Store, uid string, modelCode uint32) {
	t.Helper()
	mc := modelCode
	if _, err := st.UpsertInverterInfo(context.Background(), store.InverterInfoUpdate{
		UID: uid, TsMs: 1000, ShortAddr: 100, ModelCode: &mc,
	}); err != nil {
		t.Fatalf("seed %s: %v", uid, err)
	}
}

// callSetPhase drives handleSetInverterPhaseReq over an in-memory pipe and
// returns the decoded response, exercising the real request/response framing.
func callSetPhase(t *testing.T, srv *Server, peerUID int, backend string, req *wire.SetInverterPhaseRequest) *wire.SetInverterPhaseResponse {
	t.Helper()
	cli, srvConn := net.Pipe()
	defer cli.Close()
	done := make(chan error, 1)
	go func() {
		done <- srv.handleSetInverterPhaseReq(context.Background(), peerUID, backend, "test", srvConn, req)
		_ = srvConn.Close()
	}()
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	var env wire.Envelope
	if err := wire.ReadFrame(cli, &env); err != nil {
		t.Fatalf("read resp: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	resp := env.GetSetInverterPhaseResp()
	if resp == nil {
		t.Fatalf("expected SetInverterPhaseResp, got %T", env.GetBody())
	}
	return resp
}

func TestHandleSetInverterPhase(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir()+"/state.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	seedInverter(t, st, "800000000001", 0x18) // QS1A, single-phase
	seedInverter(t, st, "800000000032", 0x32) // QT2, three-phase

	srv := &Server{
		Ingestor: &ingest.Ingestor{S: st, ControllerBackends: []string{"ecu-web"}},
		Store:    st,
	}

	// Non-controller backend is refused before any write.
	if r := callSetPhase(t, srv, 0, "evil", &wire.SetInverterPhaseRequest{Uid: "800000000001", Leg: 2}); r.GetOk() || !strings.Contains(r.GetError(), "controller") {
		t.Fatalf("non-controller: ok=%v err=%q", r.GetOk(), r.GetError())
	}
	assertPhaseNull(t, st, "800000000001")

	// Leg out of range.
	for _, leg := range []uint32{0, 4} {
		if r := callSetPhase(t, srv, 0, "ecu-web", &wire.SetInverterPhaseRequest{Uid: "800000000001", Leg: leg}); r.GetOk() || !strings.Contains(r.GetError(), "leg must be") {
			t.Fatalf("leg=%d: ok=%v err=%q", leg, r.GetOk(), r.GetError())
		}
	}

	// Unknown UID.
	if r := callSetPhase(t, srv, 0, "ecu-web", &wire.SetInverterPhaseRequest{Uid: "999999999999", Leg: 1}); r.GetOk() || !strings.Contains(r.GetError(), "unknown") {
		t.Fatalf("unknown uid: ok=%v err=%q", r.GetOk(), r.GetError())
	}

	// Three-phase inverter is rejected.
	if r := callSetPhase(t, srv, 0, "ecu-web", &wire.SetInverterPhaseRequest{Uid: "800000000032", Leg: 1}); r.GetOk() || !strings.Contains(r.GetError(), "single-phase") {
		t.Fatalf("three-phase: ok=%v err=%q", r.GetOk(), r.GetError())
	}
	assertPhaseNull(t, st, "800000000032")

	// Happy path: single-phase leg 3 persists + audits.
	if r := callSetPhase(t, srv, 0, "ecu-web", &wire.SetInverterPhaseRequest{Uid: "800000000001", Leg: 3}); !r.GetOk() {
		t.Fatalf("happy path: ok=%v err=%q", r.GetOk(), r.GetError())
	}
	var ph sql.NullInt64
	if err := st.DB().QueryRowContext(ctx, `SELECT phase FROM inverters WHERE uid=?`, "800000000001").Scan(&ph); err != nil {
		t.Fatalf("read phase: %v", err)
	}
	if !ph.Valid || ph.Int64 != 3 {
		t.Fatalf("persisted phase = %v; want 3", ph)
	}
	var cnt int
	var by, detail sql.NullString
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*), MAX(by), MAX(error) FROM events WHERE kind='phase_set' AND inverter_uid=?`,
		"800000000001").Scan(&cnt, &by, &detail); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if cnt != 1 || by.String != "ecu-web" || detail.String != "L3" {
		t.Fatalf("audit: cnt=%d by=%q detail=%q; want 1,ecu-web,L3", cnt, by.String, detail.String)
	}
}

// TestSetInverterPhase_BroadcastsInfo is the regression for the "reverts to L1
// while online" bug: a successful set must broadcast an InverterInfo carrying
// the new leg so live subscribers update instead of keeping the stale value.
func TestSetInverterPhase_BroadcastsInfo(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir()+"/state.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	seedInverter(t, st, "800000000001", 0x18)

	pub := events.New()
	srv := &Server{
		Ingestor:  &ingest.Ingestor{S: st, ControllerBackends: []string{"ecu-web"}},
		Store:     st,
		Publisher: pub,
	}
	ch, unsub := pub.Subscribe()
	defer unsub()

	if r := callSetPhase(t, srv, 0, "ecu-web", &wire.SetInverterPhaseRequest{Uid: "800000000001", Leg: 2}); !r.GetOk() {
		t.Fatalf("set failed: %q", r.GetError())
	}
	select {
	case env := <-ch:
		info := env.GetInfo()
		if info == nil || info.GetPeerUid() != "800000000001" || info.GetPhase() != 2 {
			t.Fatalf("broadcast Info = %+v; want uid=800000000001 phase=2", info)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no InverterInfo broadcast after SetInverterPhase")
	}
}

func assertPhaseNull(t *testing.T, st *store.Store, uid string) {
	t.Helper()
	var ph sql.NullInt64
	if err := st.DB().QueryRowContext(context.Background(), `SELECT phase FROM inverters WHERE uid=?`, uid).Scan(&ph); err != nil {
		t.Fatalf("read phase %s: %v", uid, err)
	}
	if ph.Valid {
		t.Fatalf("phase for %s = %d; want NULL (no write)", uid, ph.Int64)
	}
}

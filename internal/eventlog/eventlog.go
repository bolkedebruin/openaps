// Package eventlog answers EventsRequest by querying inv-driver's
// append-only events store and mapping rows to the wire schema. It is a
// read-only view; events are written elsewhere (ingest, pairing, …).
package eventlog

import (
	"context"

	"github.com/bolkedebruin/openaps/internal/store"
	"github.com/bolkedebruin/openaps/wire"
)

// firehoseKinds are high-volume per-frame events that drown the
// operator-facing log; excluded unless the caller asks for one by kind.
var firehoseKinds = []string{"telemetry", "inverter_info"}

// Handler serves EventsRequest from the store.
type Handler struct {
	Store        *store.Store
	DefaultLimit int // applied when the request limit is 0
	MaxLimit     int // hard cap on returned rows
}

// NewHandler returns a Handler with sane limits when zero.
func NewHandler(st *store.Store) *Handler {
	return &Handler{Store: st, DefaultLimit: 200, MaxLimit: 2000}
}

// Handle runs the query and maps results to an EventsResponse.
func (h *Handler) Handle(ctx context.Context, req *wire.EventsRequest) *wire.EventsResponse {
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = h.DefaultLimit
	}
	if h.MaxLimit > 0 && limit > h.MaxLimit {
		limit = h.MaxLimit
	}
	// Exclude the high-volume per-frame kinds unless the caller asks for a
	// specific kind (so an operator can still inspect telemetry directly).
	var exclude []string
	if req.GetKind() == "" {
		exclude = firehoseKinds
	}
	evs, err := h.Store.QueryEvents(ctx, store.EventFilter{
		SinceMs:      req.GetSinceMs(),
		Kind:         req.GetKind(),
		Severity:     req.GetSeverity(),
		InverterUID:  req.GetInverterUid(),
		ExcludeKinds: exclude,
		Limit:        limit,
		Desc:         true, // newest-first for the UI
	})
	if err != nil {
		return &wire.EventsResponse{Ok: false, Error: err.Error()}
	}
	out := &wire.EventsResponse{Ok: true, Events: make([]*wire.Event, 0, len(evs))}
	for _, e := range evs {
		out.Events = append(out.Events, &wire.Event{
			Id:          e.ID,
			TsMs:        e.TsMs,
			InverterUid: e.InverterUID,
			Kind:        e.Kind,
			Severity:    e.Severity,
			ShortAddr:   e.ShortAddr,
			Detail:      e.Detail,
			RawHex:      e.RawHex,
			By:          e.By,
		})
	}
	return out
}

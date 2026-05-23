package gridprofile

import (
	"context"
	"fmt"

	"github.com/bolke/inv-driver/wire"
)

// IPCBroadcaster is a Broadcaster implementation that routes broadcast L2
// frames through an IPC backend (typically the active ecu-zb publisher).
// It wraps ipc.Server's SendToBackend with an Envelope_Broadcast so ecu-zb's
// bus-mgr builds the L1 outer envelope with SA=0.
type IPCBroadcaster struct {
	// Router is the subset of ipc.Server required to forward frames.
	Router interface {
		SendToBackend(backend string, env *wire.Envelope) bool
	}
	// Backend is the publisher backend name to route through.
	Backend string
}

// Broadcast encodes frame into an Envelope_Broadcast and forwards it to the
// configured backend. Returns an error if the backend is absent or its queue
// is full.
func (b *IPCBroadcaster) Broadcast(_ context.Context, frame []byte) error {
	if b.Router == nil || b.Backend == "" {
		return fmt.Errorf("IPCBroadcaster: no router/backend configured")
	}
	env := &wire.Envelope{Body: &wire.Envelope_Broadcast{Broadcast: &wire.Broadcast{
		Frame: frame,
	}}}
	if !b.Router.SendToBackend(b.Backend, env) {
		return fmt.Errorf("IPCBroadcaster: send to backend %q failed (absent or queue full)", b.Backend)
	}
	return nil
}

// IPCSender is a Sender implementation that routes protection-write frames
// through an IPC backend (typically the active ecu-zb publisher). It wraps
// the ipc.Server's SendToBackend so the reconciler can send frames without
// importing the ipc package.
//
// The Backend field names the publisher backend to route through (e.g.
// "apsystems-stock-zb"). Send originates inside inv-driver and therefore
// bypasses the downstream controller-backends gate by construction — the
// frame is enqueued directly into the publisher's outbound channel via
// SendToBackend.
type IPCSender struct {
	// Router is the subset of ipc.Server required to forward frames.
	// Use *ipc.Server directly when constructing, or any value implementing
	// the same SendToBackend method signature.
	Router interface {
		SendToBackend(backend string, env *wire.Envelope) bool
	}
	// Backend is the publisher backend name to route through.
	Backend string
}

// Send encodes uid + frame into an Envelope_Send and forwards it to the
// configured backend. Returns an error if the backend is absent or its
// queue is full.
func (s *IPCSender) Send(_ context.Context, uid string, frame []byte) error {
	if s.Router == nil || s.Backend == "" {
		return fmt.Errorf("IPCSender: no router/backend configured")
	}
	env := &wire.Envelope{Body: &wire.Envelope_Send{Send: &wire.Send{
		PeerUid: uid,
		Frame:   frame,
	}}}
	if !s.Router.SendToBackend(s.Backend, env) {
		return fmt.Errorf("IPCSender: send to backend %q failed (absent or queue full)", s.Backend)
	}
	return nil
}

// IngestReadback is a ReadbackSource implementation backed by the ingest
// protection cache. It satisfies the interface contract pinned in the build
// plan: ReadbackNative returns the latest decoded protection values keyed by
// APsystems 2-letter code in native units; TriggerRead enqueues an on-demand
// protection read.
//
// The Ingestor field must implement the two methods accessed here.
type IngestReadback struct {
	// Ingestor must expose ReadbackNative and TriggerRead; use
	// *ingest.Ingestor directly.
	Ingestor interface {
		ReadbackNative(uid string) (map[string]float64, bool)
		TriggerRead(uid string)
	}
}

// ReadbackNative returns the latest decoded protection values for uid,
// keyed by 2-letter APsystems code in native units. ok=false if no read
// has completed for this UID.
func (rb *IngestReadback) ReadbackNative(uid string) (map[string]float64, bool) {
	if rb.Ingestor == nil {
		return nil, false
	}
	return rb.Ingestor.ReadbackNative(uid)
}

// TriggerRead asks the ingest read pipeline to (re)read uid's protection.
func (rb *IngestReadback) TriggerRead(uid string) {
	if rb.Ingestor == nil {
		return
	}
	rb.Ingestor.TriggerRead(uid)
}

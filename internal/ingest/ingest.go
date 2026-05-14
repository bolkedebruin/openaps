// Package ingest translates wire.Envelope events into state-store
// writes. It is the only path data takes from the UDS layer into
// SQLite, and the only place v0 contains business logic — everything
// else is plumbing.
package ingest

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/bolke/inv-driver/internal/store"
	"github.com/bolke/inv-driver/wire"
)

// Ingestor owns the store handle and turns wire events into rows.
// One Ingestor is shared by all connection goroutines; the underlying
// store handles its own write serialisation.
type Ingestor struct {
	S *store.Store
}

// Handle dispatches an Envelope by oneof body. Returns an error only
// on hard malformed input; transient DB errors are returned so the
// connection layer can decide whether to drop the peer.
//
// TODO: familyFromModel is a string-prefix hack — replace with a
// proper capability table when one exists (architecture review).
func (in *Ingestor) Handle(ctx context.Context, backend string, env *wire.Envelope) error {
	switch b := env.GetBody().(type) {
	case *wire.Envelope_Hello:
		if b.Hello == nil {
			return fmt.Errorf("hello envelope without body")
		}
		return in.S.AppendEvent(ctx, b.Hello.GetStartedAtMs(), "", "backend_hello", "info", protoPayload(b.Hello))
	case *wire.Envelope_Telemetry:
		if b.Telemetry == nil {
			return fmt.Errorf("telemetry envelope without body")
		}
		return in.handleTelemetry(ctx, b.Telemetry)
	case *wire.Envelope_DecodeFailed:
		if b.DecodeFailed == nil {
			return fmt.Errorf("decode_failed envelope without body")
		}
		return in.S.AppendEvent(ctx, b.DecodeFailed.GetTsMs(), "", "decode_failed", "warn", protoPayload(b.DecodeFailed))
	case nil:
		return fmt.Errorf("envelope without body")
	default:
		return fmt.Errorf("unhandled envelope body type %T", b)
	}
}

func (in *Ingestor) handleTelemetry(ctx context.Context, t *wire.Telemetry) error {
	if t.GetPeerUid() == "" {
		return fmt.Errorf("telemetry: empty peer_uid")
	}
	fam := familyFromModel(t.GetModel())
	// short_addr on the wire is uint32 (proto has no uint16); narrow
	// at the store boundary which retains the uint16 column type.
	shortAddr := uint16(t.GetShortAddr())
	if err := in.S.UpsertInverterFromTelemetry(ctx, t.GetPeerUid(), shortAddr, fam, t.GetModel(), t.GetTsMs()); err != nil {
		return err
	}
	if err := in.S.WriteTelemetryLive(ctx, t.GetPeerUid(), t.GetTsMs(),
		t.GetActivePowerW(), t.GetGridV(), t.GetFreqHz(), t.GetBusV(), t.GetReportSec(), protoPayload(t)); err != nil {
		return err
	}
	// Light-touch audit row. Detailed telemetry_history comes later.
	return in.S.AppendEvent(ctx, t.GetTsMs(), t.GetPeerUid(), "telemetry", "info", nil)
}

// protoPayload marshals m to protojson and returns it as a
// json.RawMessage-like type so the store layer writes it verbatim
// (no second JSON round-trip). nil m -> nil payload.
func protoPayload(m proto.Message) any {
	if m == nil {
		return nil
	}
	b, err := protojson.Marshal(m)
	if err != nil {
		// Fall back to a stub object rather than dropping the row.
		return map[string]string{"error": "protojson marshal: " + err.Error()}
	}
	return rawJSON(b)
}

// rawJSON is a []byte alias that the store's json.Marshal will emit
// verbatim (json.RawMessage). Keeping the type local avoids importing
// encoding/json into this file.
type rawJSON []byte

// MarshalJSON implements json.Marshaler.
func (r rawJSON) MarshalJSON() ([]byte, error) {
	if len(r) == 0 {
		return []byte("null"), nil
	}
	return []byte(r), nil
}

// familyFromModel maps the human Model string to the lowercase family
// key the eventual capability table will use. "unknown(0xNN)" → "".
func familyFromModel(model string) string {
	switch {
	case strings.HasPrefix(model, "QS1A"):
		return "qs1a"
	case strings.HasPrefix(model, "QS1"):
		return "qs1"
	case strings.HasPrefix(model, "DS3"):
		return "ds3"
	case strings.HasPrefix(model, "DSP"):
		return "dsp"
	case strings.HasPrefix(model, "YC600"):
		return "yc600"
	case strings.HasPrefix(model, "YC1000"):
		return "yc1000"
	default:
		return ""
	}
}

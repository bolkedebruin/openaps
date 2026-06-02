package server

import (
	"context"
	"fmt"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
	"github.com/bolkedebruin/openaps/codec"
)

// dispatchProtection routes one grid-protection parameter write for an
// inverter through inv-driver, shared by the Model 703/711/134 writers.
//
// RouteDirect params encode to a unicast L2 frame (codec.EncodeSetProtection)
// and go through inv-driver. There is NO legacy fallback: a param inv-driver
// can't yet encode (env-gated or multi-frame — the mode enum, slope, delay,
// reconnect) returns a clear error rather than silently routing elsewhere.
// RouteUnsupported likewise surfaces as an error.
func dispatchProtection(ctx context.Context, sender frameSender, inv source.Inverter, paramName string, value float64) error {
	switch routeFor(familyForModel(inv.Model), paramName) {
	case RouteDirect:
		if sender == nil {
			return fmt.Errorf("%s: inv-driver not configured", paramName)
		}
		frames, err := codec.EncodeSetProtection(uint8(inv.Model), paramName, value)
		if err != nil {
			return fmt.Errorf("%s on family %d (%s): %w", paramName, inv.Model, inv.UID, err)
		}
		// Some params are multi-frame on the wire (the QS1 mode enum);
		// send each in order.
		for _, f := range frames {
			if err := sender.Send(ctx, inv.UID, f); err != nil {
				return err
			}
		}
		return nil
	case RouteUnsupported:
		return fmt.Errorf("%s on family %d (%s) is not supported by inverter firmware on any host-side path",
			paramName, inv.Model, inv.UID)
	}
	return fmt.Errorf("%s: unknown write route", paramName)
}

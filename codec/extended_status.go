package codec

// ExtendedStatus is the family-specific raw-status block surfaced on
// each Reply. Per-family implementations (DS3Status, QS1AStatus, and
// future YC600 / YC1000 / QT2 status types) expose the same two
// projections the generic SunSpec / fault paths consume; the
// family-specific raw accessors stay reachable via a type assertion
// (`if ds, ok := r.ExtendedStatus.(codec.DS3Status); ok { ... }`).
type ExtendedStatus interface {
	// InverterStatus reduces the family's status bits to the
	// family-agnostic InverterStatus that codec.Reply.Status carries.
	InverterStatus() InverterStatus
	// ModbusStatus packs the family's bits into the SunSpec Model 103
	// (St, Evt1) word pair.
	ModbusStatus() (st, evt1 uint16)
}

// InverterStatus returns the family-agnostic aggregator booleans for a
// DS3 reply (DCBus / fault A/B / warning A/B).
func (s DS3Status) InverterStatus() InverterStatus {
	return s.Faults().InverterStatus()
}

// InverterStatus returns the family-agnostic aggregator booleans for a
// QS1A reply (DCBus / fault A/B / warning A/B).
func (s QS1AStatus) InverterStatus() InverterStatus {
	return s.Faults().InverterStatus()
}

// Compile-time checks that both per-family status blocks satisfy the
// ExtendedStatus interface.
var (
	_ ExtendedStatus = DS3Status{}
	_ ExtendedStatus = QS1AStatus{}
)

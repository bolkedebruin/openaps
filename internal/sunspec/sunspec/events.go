package sunspec

// SunSpec Inverter Model 101/102/103 standard Evt1 bit positions.
//
// Reference: SunSpec Inverter Model spec — Evt1 is a 32-bit bitfield (called
// "EVTI" in some specs) carrying inverter-wide event flags. Each constant is
// the OR mask for that flag.
const (
	Evt1GroundFault     uint32 = 1 << 0
	Evt1DCOverVolt      uint32 = 1 << 1
	Evt1ACDisconnect    uint32 = 1 << 2
	Evt1DCDisconnect    uint32 = 1 << 3
	Evt1GridDisconnect  uint32 = 1 << 4
	Evt1CabinetOpen     uint32 = 1 << 5
	Evt1ManualShutdown  uint32 = 1 << 6
	Evt1OverTemp        uint32 = 1 << 7
	Evt1OverFrequency   uint32 = 1 << 8
	Evt1UnderFrequency  uint32 = 1 << 9
	Evt1ACOverVolt      uint32 = 1 << 10
	Evt1ACUnderVolt     uint32 = 1 << 11
	Evt1BlownStringFuse uint32 = 1 << 12
	Evt1UnderTemp       uint32 = 1 << 13
	Evt1MemoryLoss      uint32 = 1 << 14
	Evt1HWTestFailure   uint32 = 1 << 15
)

// MapAPsystemsToSunSpecEvt1 translates the 86-bit APsystems event bitstring
// (packed as [4]uint32 LSB-first) into a standard SunSpec Evt1 bitfield.
//
// Source: /home/local_web/pages/application/language/english/page_lang.php
// `display_status_zigbee_<N>` — the same table the ECU's PHP UI uses to
// render alarm names. Multiple APsystems bits collapse into a single SunSpec
// bit (e.g., AC over-voltage stages 1-4 across channels A-C all OR into
// Evt1ACOverVolt). DC under-voltage flags have no SunSpec analog and are
// only surfaced in EvtVnd* — see the encoder.
func MapAPsystemsToSunSpecEvt1(bits [4]uint32) uint32 {
	// aps(n) tests whether APsystems bit `n` (0..127) is set.
	aps := func(n int) bool {
		if n < 0 || n >= 128 {
			return false
		}
		return bits[n/32]&(1<<uint(n%32)) != 0
	}

	var evt uint32

	if aps(17) { // GFDI Locked
		evt |= Evt1GroundFault
	}
	// DC over-voltage on any channel
	if aps(8) || aps(10) || aps(39) || aps(41) {
		evt |= Evt1DCOverVolt
	}
	if aps(19) { // AC Disconnect
		evt |= Evt1ACDisconnect
	}
	if aps(18) { // Remote Shut
		evt |= Evt1ManualShutdown
	}
	if aps(16) { // Over Critical Temperature
		evt |= Evt1OverTemp
	}
	// Over-frequency: legacy "exceeding range" + per-stage flags
	if aps(0) || aps(29) || aps(31) {
		evt |= Evt1OverFrequency
	}
	// Under-frequency: legacy + per-stage
	if aps(1) || aps(30) || aps(32) {
		evt |= Evt1UnderFrequency
	}
	// AC over-voltage: per-channel A/B/C + per-stage 1-4 + legacy "AC range"
	if aps(2) || aps(4) || aps(6) ||
		aps(23) || aps(33) || aps(35) || aps(37) || aps(44) {
		evt |= Evt1ACOverVolt
	}
	// AC under-voltage: same families
	if aps(3) || aps(5) || aps(7) ||
		aps(24) || aps(34) || aps(36) || aps(38) || aps(45) {
		evt |= Evt1ACUnderVolt
	}
	// Bit 21 (Active Anti-island) — closest standard mapping is GridDisconnect
	if aps(21) {
		evt |= Evt1GridDisconnect
	}
	// Bit 28 (Relay Failed) → HWTestFailure
	if aps(28) {
		evt |= Evt1HWTestFailure
	}

	return evt
}

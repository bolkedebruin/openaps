package codec

// L2 envelope sentinels.
const (
	L2SOF1 byte = 0xFB
	L2SOF2 byte = 0xFB
	L2EOF1 byte = 0xFE
	L2EOF2 byte = 0xFE
)

// L1 outer envelope sentinels for the apsystems-stock-zb backend.
const (
	L1Preamble byte = 0xAA // repeated 4 times
	L1Sentinel byte = 0x55
	L1ReplySOF byte = 0xFC // repeated 2 times on inbound
)

// CleartextGateMin is the lowest L1 gate byte that marks a frame as
// cleartext. Any gate byte below this is AES-wrapped.
const CleartextGateMin byte = 0xF0

// L2 command opcodes.
const (
	CmdSetPowerQS1Unicast   byte = 0x1C
	CmdSetPowerQS1Broadcast byte = 0x2C
	CmdSetPowerDS3Unicast   byte = 0xAA
	CmdSetPowerDS3Broadcast byte = 0xAB
	CmdSetPowerQT2Broadcast byte = 0xAD
	CmdSetPowerC3Unicast    byte = 0xC3
	CmdSetPowerC3Broadcast  byte = 0xA3

	// Inverter on/off. Family-independent: main.exe's zb_boot_single /
	// zb_shutdown_single (and the generic set_protection_yc600 encoder
	// reused by the MQTT / DA-conf dispatchers) emit the same frame for
	// DS3, QS1A, and YC600/C3. The opcode encodes on/off and unicast/
	// broadcast; the body is all zero.
	CmdTurnOnUnicast    byte = 0xC1
	CmdTurnOffUnicast   byte = 0xC2
	CmdTurnOnBroadcast  byte = 0xA1
	CmdTurnOffBroadcast byte = 0xA2

	// CmdGridRecoveryQS1 is the YC600-builder opcode QS1 uses for
	// grid_recovery_time (set_protection_yc600_one); value left-aligned
	// at [4..5], no sub-byte.
	CmdGridRecoveryQS1 byte = 0x5D

	// Protection-param READ queries. main.exe's get_parameters_from_inverter
	// sends three paged read requests per inverter; the inverter replies
	// page A/B/C with the protection thresholds. All-zero body.
	CmdProtReadPageA byte = 0xDD
	CmdProtReadPageB byte = 0xDE
	CmdProtReadPageC byte = 0xD9

	CmdInfoQuery        byte = 0xDC
	CmdInfoExtended     byte = 0xDD // wraps CmdInfoQuery on newer reply forms
	CmdTelemetryBBQuery byte = 0xBB
	CmdReplyQS1A        byte = 0xB1
	CmdReplyDS3         byte = 0xBB
)

// Inner-length byte (offset 2 of an L2 frame) for the three observed
// CmdInfoQuery reply shapes. The value equals the count of cmd + body
// bytes that follow it. V1/V2/V3 are firmware-generation tags: older
// firmware emits the short form, newer firmware wraps CmdInfoQuery
// inside CmdInfoExtended and adds version-byte width.
const (
	InfoReplyV1 byte = 0x09 // 16-byte short form
	InfoReplyV2 byte = 0x0A // 17-byte extended form, 2-segment version
	InfoReplyV3 byte = 0x0C // 19-byte extended form, 3-segment version
)

// Sub-codes within set-power opcodes.
const (
	SubMaxPowerQS1 byte = 0x8C
	SubMaxPowerDS3 byte = 0x27
)

// ACK status bytes returned by inverters.
const (
	AckSuccess byte = 0xDE
	AckFailed  byte = 0xDF
)

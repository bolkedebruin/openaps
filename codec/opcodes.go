package codec

// L2 envelope sentinels.
const (
	L2SOF1 byte = 0xFB
	L2SOF2 byte = 0xFB
	L2EOF1 byte = 0xFE
	L2EOF2 byte = 0xFE
)

// L2TypeInverterCmd is the type byte (offset 2) for inverter-command
// frames. All set-power / info / telemetry queries use it.
const L2TypeInverterCmd byte = 0x06

// L1 outer envelope sentinels for the apsystems-stock-zb backend.
const (
	L1Preamble byte = 0xAA // repeated 4 times
	L1Sentinel byte = 0x55
	L1ReplySOF byte = 0xFC // repeated 2 times on inbound
)

// L2 command opcodes.
const (
	CmdSetPowerQS1Unicast   byte = 0x1C
	CmdSetPowerQS1Broadcast byte = 0x2C
	CmdSetPowerDS3Unicast   byte = 0xAA
	CmdSetPowerDS3Broadcast byte = 0xAB
	CmdSetPowerQT2Broadcast byte = 0xAD
	CmdSetPowerC3Unicast    byte = 0xC3
	CmdSetPowerC3Broadcast  byte = 0xA3

	CmdInfoQuery        byte = 0xDC
	CmdTelemetryBBQuery byte = 0xBB
	CmdReplyQS1A        byte = 0xB1
	CmdReplyDS3         byte = 0xBB
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

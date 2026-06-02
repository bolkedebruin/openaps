package busmgr

import (
	"testing"

	"github.com/bolke/inv-driver/codec"
)

// TestBroadcast_RejectsUnicastOnlyCmds asserts that every yc600-builder
// protection cmd (where the sub IS the cmd and no broadcast opcode exists)
// is treated as unicast-only on the broadcast path, while a legitimate
// broadcast opcode (set-power 0x2C) is allowed.
func TestBroadcast_RejectsUnicastOnlyCmds(t *testing.T) {
	for cmd := range codec.QS1YC600ProtCmds {
		l2 := []byte{codec.L2SOF1, codec.L2SOF2, 0x06, cmd, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, codec.L2EOF1, codec.L2EOF2}
		if !isUnicastOnlyCmd(l2) {
			t.Errorf("cmd 0x%02X must be rejected on broadcast (unicast-only yc600 protection)", cmd)
		}
	}
	// QS1A set-power broadcast (0x2C) is a real broadcast opcode — must pass.
	ok := []byte{codec.L2SOF1, codec.L2SOF2, 0x06, codec.CmdSetPowerQS1Broadcast, codec.SubMaxPowerQS1, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, codec.L2EOF1, codec.L2EOF2}
	if isUnicastOnlyCmd(ok) {
		t.Errorf("cmd 0x%02X (set-power broadcast) must NOT be rejected", codec.CmdSetPowerQS1Broadcast)
	}
}

// TestAllowList_CoversYC600ProtCmds is the drift guard: the bus-mgr allow-list
// must admit every yc600-builder protection cmd the codec can emit on the
// unicast path. Without this, a new trip added to codec.QS1YC600ProtCmds but
// not the allow-list would be silently dropped at the modem.
func TestAllowList_CoversYC600ProtCmds(t *testing.T) {
	for cmd := range codec.QS1YC600ProtCmds {
		if _, ok := allowedL2Cmds[cmd]; !ok {
			t.Errorf("allowedL2Cmds is missing yc600 protection cmd 0x%02X (codec/ecu-zb drift)", cmd)
		}
	}
}

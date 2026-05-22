package codec

// ProtectionQueryFrames returns the three paged protection-param read
// queries to send to one inverter, in order (page A=0xDD, B=0xDE, C=0xD9).
// Each is the all-zero-body L2 frame; the inverter replies per page with
// the loaded grid-protection thresholds. The L1 envelope (target short
// address) is added downstream by ecu-zb.
//
// Mirrors main.exe's get_parameters_from_inverter @ 0x6462c: send the
// pages, collect the replies, then DecodeProtectionReply over them.
func ProtectionQueryFrames() [][]byte {
	return [][]byte{
		BuildL2Frame(CmdProtReadPageA, make([]byte, protQueryBodyLen)),
		BuildL2Frame(CmdProtReadPageB, make([]byte, protQueryBodyLen)),
		BuildL2Frame(CmdProtReadPageC, make([]byte, protQueryBodyLen)),
	}
}

// protQueryBodyLen is the 5-byte zero body of a read query (inner_len
// 0x06 = 1 cmd + 5 body), matching the firmware's query immediates.
const protQueryBodyLen = 5

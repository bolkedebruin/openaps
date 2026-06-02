// Package proxy splices bytes between the host process and the CC2530
// modem while invoking a Hook on each raw tty chunk. v1 wires NoOpHook
// only; the interface exists so v2 can plug in synth/MITM rules without
// touching the splice loop.
package proxy

// FrameDirection identifies the direction of a chunk on the wire.
// Values match the pcap payload byte 0 in tap.DirToModem / DirToHost.
type FrameDirection byte

const (
	DirToModem FrameDirection = 0 // host → CC2530
	DirToHost  FrameDirection = 1 // CC2530 → host
)

func (d FrameDirection) String() string {
	switch d {
	case DirToModem:
		return "to-modem"
	case DirToHost:
		return "to-host"
	}
	return "unknown"
}

// ChunkAction is the result of a Hook examining a wire chunk.
//
// Altered == nil  → forward the original bytes unchanged
// Altered != nil  → forward Altered instead of the original
// Drop == true    → do not forward to the other side
// Mine == true    → this chunk belongs to the hook's injection cycle
//
//	(its own query or the matching response). The
//	tap publishes it on IfaceInject so it shows up on
//	the "zb-inj0" interface in Wireshark instead of
//	the wire "zb0" interface — easy filter and clear
//	provenance even when the bytes are identical to
//	host-originated polling.
type ChunkAction struct {
	Altered []byte
	Drop    bool
	Mine    bool
}

// Hook is the v2 extension point. v1 uses NoOpHook when no inv-driver
// socket is configured, or BusTrackerHook when one is.
type Hook interface {
	OnChunk(dir FrameDirection, raw []byte) ChunkAction
}

type NoOpHook struct{}

func (NoOpHook) OnChunk(_ FrameDirection, _ []byte) ChunkAction {
	return ChunkAction{}
}

// Injector is what a Hook can use to inject traffic toward the modem
// without going through the host pty. The byte stream is written to
// the real UART and mirrored to the pcapng tap in DirToModem, so live
// captures see it as a host-originated frame.
type Injector interface {
	InjectToModem(raw []byte) error
}

// LifecycleHook is implemented by hooks that need a goroutine — e.g.
// a periodic poller. v1 starts/stops these around Splice.Run.
type LifecycleHook interface {
	Hook
	Start(inj Injector) error
	Stop()
}

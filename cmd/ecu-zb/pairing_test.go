package main

import (
	"io"
	"testing"

	"github.com/bolkedebruin/openaps/internal/zigbee/proxy"
)

// fdModem is a modem port double that reports a descriptor. A test uses it to
// prove that the adapter takes the fd from the splice, not from the one it
// captured at construction.
type fdModem struct{ fd uintptr }

func (fdModem) Read([]byte) (int, error)    { return 0, io.EOF }
func (fdModem) Write(b []byte) (int, error) { return len(b), nil }
func (m fdModem) Fd() uintptr               { return m.fd }

// noFdModem is a port that exposes no descriptor at all.
type noFdModem struct{}

func (noFdModem) Read([]byte) (int, error)    { return 0, io.EOF }
func (noFdModem) Write(b []byte) (int, error) { return len(b), nil }

// The splice owns the modem port and replaces it on a hangup. The pairing
// runner must therefore take its write fd from the splice every time. A cached
// number would point at a closed descriptor, or at whatever unrelated file the
// kernel gave that number to next.
func TestPairingAdapter_TakesModemFdFromSplice(t *testing.T) {
	sp := &proxy.Splice{Modem: fdModem{fd: 11}}
	// Construct with a descriptor that is wrong on purpose. Only a refresh
	// from the splice can produce 11.
	a := newPairingAdapter(sp, -1, 0x0DCE, 0x10)

	var got int
	if err := a.withModem(func() error {
		got = a.runner.Fd
		return nil
	}); err != nil {
		t.Fatalf("withModem: %v", err)
	}
	if got != 11 {
		t.Fatalf("runner.Fd = %d during the primitive, want 11 (the splice's current port)", got)
	}
}

// A port with no Fd() must leave the constructed descriptor alone. The runner
// must not write to a fabricated one.
func TestPairingAdapter_KeepsFdWhenPortExposesNone(t *testing.T) {
	sp := &proxy.Splice{Modem: noFdModem{}}
	a := newPairingAdapter(sp, 7, 0x0DCE, 0x10)

	var got int
	if err := a.withModem(func() error {
		got = a.runner.Fd
		return nil
	}); err != nil {
		t.Fatalf("withModem: %v", err)
	}
	if got != 7 {
		t.Fatalf("runner.Fd = %d, want the constructed 7", got)
	}
}

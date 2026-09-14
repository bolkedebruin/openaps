package main

import (
	"io"
	"testing"

	"github.com/bolkedebruin/openaps/internal/zigbee/proxy"
)

// fdModem is a modem port double that reports a descriptor, so a test can
// prove the adapter sources the fd from the splice rather than from the one
// captured when it was constructed.
type fdModem struct{ fd uintptr }

func (fdModem) Read([]byte) (int, error)    { return 0, io.EOF }
func (fdModem) Write(b []byte) (int, error) { return len(b), nil }
func (m fdModem) Fd() uintptr               { return m.fd }

// noFdModem is a port that exposes no descriptor at all.
type noFdModem struct{}

func (noFdModem) Read([]byte) (int, error)    { return 0, io.EOF }
func (noFdModem) Write(b []byte) (int, error) { return len(b), nil }

// The splice owns the modem port and replaces it on a hangup, so the pairing
// runner must take its write fd from the splice every time. A cached number
// would point at a closed descriptor — or at whatever unrelated file the
// kernel handed that number out for next.
func TestPairingAdapter_TakesModemFdFromSplice(t *testing.T) {
	sp := &proxy.Splice{Modem: fdModem{fd: 11}}
	// Construct with a descriptor that is deliberately wrong: only a refresh
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

// A port with no Fd() must leave the constructed descriptor alone rather than
// have the runner write to a fabricated one.
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

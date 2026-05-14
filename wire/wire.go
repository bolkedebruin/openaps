// Package wire defines the framed protobuf protocol between bus-mgr
// backends (e.g. ecu-zb) and inv-driver.
//
// v0 wire format
//
//	┌─────────────────────────────────────┐
//	│ 4 bytes  uint32 BE — body len       │
//	│ N bytes  protobuf-encoded Envelope  │
//	└─────────────────────────────────────┘
//
// The schema lives in proto/busmgr.proto; the generated types are in
// busmgr.pb.go in this same package (see FIRMWARE_REDESIGN_PROPOSAL.md
// §8 and INV_DRIVER_DESIGN.md §2). The 4-byte length-prefix framing
// is fixed; switching the body encoding from JSON to protobuf
// happened transparently above the framing layer.
//
// Max body size is bounded (MaxFrameBytes) so a malformed peer can't
// allocate gigabytes by claiming a giant length.
package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
)

// MaxFrameBytes caps one decoded body. Telemetry frames are <2 KiB;
// 64 KiB leaves ample headroom and bounds the worst case.
const MaxFrameBytes = 64 * 1024

// ReadFrame reads one length-prefixed frame from r and unmarshals it
// into env. Returns io.EOF cleanly when the peer closes before any
// header byte. A partial header read returns io.ErrUnexpectedEOF.
func ReadFrame(r io.Reader, env *Envelope) error {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 {
		return errors.New("wire: zero-length frame")
	}
	if n > MaxFrameBytes {
		return fmt.Errorf("wire: frame too large: %d > %d", n, MaxFrameBytes)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return fmt.Errorf("wire: short read on %d-byte body: %w", n, err)
	}
	// Reset env before unmarshal so stale oneof state doesn't leak
	// from prior frames when the same struct is reused.
	proto.Reset(env)
	if err := proto.Unmarshal(body, env); err != nil {
		return fmt.Errorf("wire: proto: %w", err)
	}
	return nil
}

// WriteFrame marshals env and writes it to w as a length-prefixed
// frame. Not safe for concurrent calls on the same w — callers
// serialise via a single goroutine or a mutex.
func WriteFrame(w io.Writer, env *Envelope) error {
	body, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("wire: marshal: %w", err)
	}
	if len(body) > MaxFrameBytes {
		return fmt.Errorf("wire: encoded frame too large: %d > %d", len(body), MaxFrameBytes)
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	return nil
}

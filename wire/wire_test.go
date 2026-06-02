package wire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestReadWriteFrame_RoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  *Envelope
	}{
		{
			name: "hello full",
			env: &Envelope{Body: &Envelope_Hello{Hello: &Hello{
				Backend:     "apsystems-stock-zb",
				Version:     "0.1.0",
				Hostname:    "ecu-r-pro",
				StartedAtMs: 1747200000000,
				BusCaps:     []string{"zigbee", "tap"},
			}}},
		},
		{
			name: "hello minimal",
			env: &Envelope{Body: &Envelope_Hello{Hello: &Hello{
				Backend:     "apsystems-stock-zb",
				Version:     "0.1.0",
				StartedAtMs: 1747200000001,
			}}},
		},
		{
			name: "telemetry full",
			env: &Envelope{Body: &Envelope_Telemetry{Telemetry: &Telemetry{
				TsMs:         1747200000002,
				ShortAddr:    0x5011,
				PeerUid:      "999900000003",
				Cmd:          0xB1,
				Model:        "QS1A",
				GridV:        232.5,
				BusV:         400.1,
				FreqHz:       50.02,
				ReportSec:    300,
				ActivePowerW: 1234.5,
				ReactiveVar:  -10.0,
				Panels: []*Panel{
					{Index: 0, DcV: 38.1, DcI: 8.2, W: 312.4},
					{Index: 1, DcV: 38.3, DcI: 8.1, W: 311.0},
				},
				LifetimeRaw:   []uint64{12345, 12346},
				LifetimeScale: 0.001,
			}}},
		},
		{
			name: "telemetry ac only",
			env: &Envelope{Body: &Envelope_Telemetry{Telemetry: &Telemetry{
				TsMs:         1747200000003,
				ShortAddr:    0x61F0,
				PeerUid:      "999900000001",
				Cmd:          0xBB,
				Model:        "DS3",
				ActivePowerW: 555.0,
			}}},
		},
		{
			name: "decode failed",
			env: &Envelope{Body: &Envelope_DecodeFailed{DecodeFailed: &DecodeFailed{
				TsMs:      1747200000004,
				ShortAddr: 0x1234,
				Error:     "L2 CRC mismatch",
				RawHex:    "fcfc...truncated",
			}}},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := WriteFrame(&buf, tc.env); err != nil {
				t.Fatalf("WriteFrame: %v", err)
			}
			var got Envelope
			if err := ReadFrame(&buf, &got); err != nil {
				t.Fatalf("ReadFrame: %v", err)
			}
			if !proto.Equal(&got, tc.env) {
				t.Fatalf("round-trip mismatch:\n got=%+v\nwant=%+v", &got, tc.env)
			}
		})
	}
}

func TestReadFrame_ZeroLengthHeader(t *testing.T) {
	t.Parallel()
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 0)
	r := bytes.NewReader(hdr[:])
	var env Envelope
	err := ReadFrame(r, &env)
	if err == nil {
		t.Fatal("expected error on zero-length frame")
	}
	if errors.Is(err, io.EOF) {
		t.Fatalf("zero-length error should not be io.EOF, got %v", err)
	}
	if !strings.Contains(err.Error(), "zero-length") {
		t.Fatalf("error message: got %q, want contains \"zero-length\"", err.Error())
	}
}

func TestReadFrame_OversizedHeader(t *testing.T) {
	t.Parallel()
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], MaxFrameBytes+1)
	r := bytes.NewReader(hdr[:])
	var env Envelope
	err := ReadFrame(r, &env)
	if err == nil {
		t.Fatal("expected error on oversized frame")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("error message: got %q, want contains \"too large\"", err.Error())
	}
}

func TestReadFrame_TruncatedBody(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 10)
	buf.Write(hdr[:])
	buf.Write([]byte("123456789"))

	var env Envelope
	err := ReadFrame(&buf, &env)
	if err == nil {
		t.Fatal("expected error on truncated body")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected wrapped io.ErrUnexpectedEOF, got %v", err)
	}
}

func TestReadFrame_MalformedProto(t *testing.T) {
	t.Parallel()
	// Random bytes that are not valid proto wire-format for Envelope.
	// 0xff as the leading byte is an invalid tag (wire type 7).
	body := []byte{0xff, 0xff, 0xff, 0xff}
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(body)))
	buf.Write(hdr[:])
	buf.Write(body)

	var env Envelope
	err := ReadFrame(&buf, &env)
	if err == nil {
		t.Fatal("expected error on malformed proto body")
	}
	if !strings.Contains(err.Error(), "proto") {
		t.Fatalf("error message: got %q, want contains \"proto\"", err.Error())
	}
}

func TestReadFrame_CleanEOFOnHeader(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	var env Envelope
	err := ReadFrame(&buf, &env)
	if err != io.EOF {
		t.Fatalf("expected exact io.EOF, got %v", err)
	}
}

// failingWriter is an io.Writer that fails if it is written to at
// all. Used to verify WriteFrame rejects oversized bodies before any
// write.
type failingWriter struct {
	t       *testing.T
	written bool
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.written = true
	w.t.Fatalf("WriteFrame wrote %d bytes despite oversized body", len(p))
	return len(p), nil
}

func TestWriteFrame_RejectsOversizedBody(t *testing.T) {
	t.Parallel()
	// Build a payload whose proto encoding exceeds MaxFrameBytes.
	big := strings.Repeat("A", MaxFrameBytes+1)
	env := &Envelope{Body: &Envelope_DecodeFailed{DecodeFailed: &DecodeFailed{
		TsMs:   1,
		Error:  "x",
		RawHex: big,
	}}}
	w := &failingWriter{t: t}
	err := WriteFrame(w, env)
	if err == nil {
		t.Fatal("expected error on oversized body")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("error message: got %q, want contains \"too large\"", err.Error())
	}
	if w.written {
		t.Fatal("WriteFrame wrote bytes despite oversized body")
	}
}

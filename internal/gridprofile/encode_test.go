package gridprofile

import (
	"errors"
	"testing"

	"github.com/bolke/inv-driver/codec"
)

// DS3 model codes for testing.
const (
	testDS3  = codec.ModelDS3
	testQS1A = codec.ModelQS1A
)

func TestEncodeEffectivePoint_DS3_FreqThreshold(t *testing.T) {
	// CB = Over_frequency_Watt_Low_set on DS3 — should produce one L2 frame.
	ep := EffectivePoint{
		ApsCode:     "CB",
		NativeValue: 50.2,
		Unit:        "Hz",
		Range:       &Range{Min: 50.0, Max: 52.0},
	}
	frames, err := EncodeEffectivePoint(testDS3, ep)
	if err != nil {
		t.Fatalf("DS3 CB encode: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("DS3 CB: expected 1 frame, got %d", len(frames))
	}
	// Frame must start with L2 SOF (FB FB)
	f := frames[0]
	if f[0] != 0xFB || f[1] != 0xFB {
		t.Errorf("DS3 CB: bad SOF: %02X %02X", f[0], f[1])
	}
	// Opcode must be DS3 unicast (0xAA)
	if f[3] != codec.CmdSetPowerDS3Unicast {
		t.Errorf("DS3 CB: bad opcode: got %02X want %02X", f[3], codec.CmdSetPowerDS3Unicast)
	}
}

func TestEncodeEffectivePoint_DS3_DD(t *testing.T) {
	// DD = Over_Frequency_Watt_Slope_set; EN 50549-1 = 16.7 %Pref/Hz
	ep := EffectivePoint{
		ApsCode:     "DD",
		NativeValue: 16.7,
		Unit:        "%Pref/Hz",
		Range:       &Range{Min: 0, Max: 50},
	}
	frames, err := EncodeEffectivePoint(testDS3, ep)
	if err != nil {
		t.Fatalf("DS3 DD: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("DS3 DD: expected 1 frame, got %d", len(frames))
	}
}

func TestEncodeEffectivePoint_DS3_AG(t *testing.T) {
	// AG = grid_recovery_time; 20 s
	ep := EffectivePoint{
		ApsCode:     "AG",
		NativeValue: 20.0,
		Unit:        "s",
	}
	frames, err := EncodeEffectivePoint(testDS3, ep)
	if err != nil {
		t.Fatalf("DS3 AG: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("DS3 AG: expected 1 frame, got %d", len(frames))
	}
}

func TestEncodeEffectivePoint_QS1A_CF_Unsupported(t *testing.T) {
	// CF (Over_Frequency_Watt_Slope_set equivalent) is not in QS1A encoder.
	// The code "CF" is not in the forward map; it's a DS3-only code that
	// maps to the same long-name as DD but QS1A has no such encoder.
	// Use DC instead (Over_frequency_Watt_Start_set) which is also rejected
	// by QS1A firmware.
	ep := EffectivePoint{
		ApsCode:     "DC",
		NativeValue: 50.2,
		Unit:        "Hz",
	}
	_, err := EncodeEffectivePoint(testQS1A, ep)
	if err == nil {
		t.Fatal("QS1A DC: expected ErrUnsupported, got nil")
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("QS1A DC: expected ErrUnsupported, got %v", err)
	}
}

func TestEncodeEffectivePoint_QS1A_CG_Unsupported(t *testing.T) {
	ep := EffectivePoint{
		ApsCode:     "CG",
		NativeValue: 5.0,
		Unit:        "s",
	}
	_, err := EncodeEffectivePoint(testQS1A, ep)
	if err == nil {
		t.Fatal("QS1A CG: expected ErrUnsupported")
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("QS1A CG: expected ErrUnsupported, got %v", err)
	}
}

func TestEncodeEffectivePoint_QS1A_FreqThreshold(t *testing.T) {
	// DH = Under_Frequency_Watt_Low_set — QS1A supports this.
	ep := EffectivePoint{
		ApsCode:     "DH",
		NativeValue: 47.5,
		Unit:        "Hz",
		Range:       &Range{Min: 45.0, Max: 50.0},
	}
	frames, err := EncodeEffectivePoint(testQS1A, ep)
	if err != nil {
		t.Fatalf("QS1A DH: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("QS1A DH: expected 1 frame, got %d", len(frames))
	}
}

func TestEncodeEffectivePoint_Clamp(t *testing.T) {
	// Value above range.max should be clamped to max.
	ep := EffectivePoint{
		ApsCode:     "CB",
		NativeValue: 55.0, // above max 52.0
		Unit:        "Hz",
		Range:       &Range{Min: 50.0, Max: 52.0},
	}
	// Should succeed with clamped value (52.0)
	frames, err := EncodeEffectivePoint(testDS3, ep)
	if err != nil {
		t.Fatalf("clamp test: %v", err)
	}
	if len(frames) == 0 {
		t.Fatal("clamp test: no frames")
	}
}

func TestEncodeEffectivePoint_UnknownCode(t *testing.T) {
	ep := EffectivePoint{
		ApsCode:     "ZZ",
		NativeValue: 50.0,
	}
	_, err := EncodeEffectivePoint(testDS3, ep)
	if err == nil {
		t.Fatal("unknown code: expected error")
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("unknown code: expected ErrUnsupported, got %v", err)
	}
}

func TestEncodeEffectivePoint_NoEncoderCode(t *testing.T) {
	// "DC" is in the forward map but has no LongName (decode-only alias,
	// not encodable via EncodeSetProtection).
	ep := EffectivePoint{
		ApsCode:     "DC",
		NativeValue: 50.2,
		Unit:        "Hz",
	}
	_, err := EncodeEffectivePoint(testDS3, ep)
	if err == nil {
		t.Fatal("DC: expected ErrUnsupported (no encoder)")
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("DC: expected ErrUnsupported, got %v", err)
	}
}

func TestEncodeEffectivePoint_DS3_CA_Unsupported(t *testing.T) {
	// CA has no DS3 encoder.
	ep := EffectivePoint{
		ApsCode:     "CA",
		NativeValue: 50.2,
		Unit:        "Hz",
	}
	_, err := EncodeEffectivePoint(testDS3, ep)
	if err == nil {
		t.Fatal("DS3 CA: expected ErrUnsupported")
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("DS3 CA: expected ErrUnsupported, got %v", err)
	}
}

func TestEncodeEffectivePoint_InvalidRange(t *testing.T) {
	ep := EffectivePoint{
		ApsCode:     "CB",
		NativeValue: 50.2,
		Unit:        "Hz",
		Range:       &Range{Min: 52.0, Max: 50.0}, // inverted!
	}
	_, err := EncodeEffectivePoint(testDS3, ep)
	if err == nil {
		t.Fatal("inverted range: expected error")
	}
	if !errors.Is(err, ErrOutOfRange) {
		t.Errorf("inverted range: expected ErrOutOfRange, got %v", err)
	}
}

func TestWriteable_DS3(t *testing.T) {
	// DS3 supports the freq-watt/droop set AND the EN volt/freq trips +
	// clear times. The volt codes regressed once when the capability probe's
	// fixed 50.0 value tripped the new physical envelope (50 V < 100) — guard
	// the whole EN set here so that can't recur silently.
	writeable := []string{
		"CB", "CC", "DH", "DI", "DD", "AG", "CG", "CV",
		"AC", "AD", "AQ", "AY", "AB", // volt trips
		"AE", "AF", "AJ", "AK", // freq trips
		"BB", "BC", "BD", "BE", "BH", "BI", "BJ", "BK", // clear times
	}
	for _, code := range writeable {
		if !Writeable(testDS3, code) {
			t.Errorf("DS3 %q should be writeable; reason: %s", code, UnsupportedReason(testDS3, code))
		}
	}
}

func TestWriteable_QS1A_HardRejects(t *testing.T) {
	// CA/DC (over-freq curve start), CG (response delay), CF — all rejected at firmware level.
	for _, code := range []string{"CA", "DC", "CG", "CF"} {
		if Writeable(testQS1A, code) {
			t.Errorf("QS1A %q should NOT be writeable (hard reject)", code)
		}
	}
}

func TestWriteable_QS1A_Writable(t *testing.T) {
	// QS1A supports the freq-watt set (CB/CC/DH/DI/CV/AG) AND — validated
	// on-wire 2026-05-23 — the EN volt/freq trips + clear times (AC 183→196,
	// the full BB-BK set pushed and read back on both QS1A units). CA is a
	// hard-reject on QS1A. Volt codes are included so the 50.0-probe/envelope
	// interaction can't silently mark them unsupported again.
	writable := []string{
		"CB", "CC", "DH", "DI", "CV", "AG",
		"AC", "AD", "AQ", "AY", "AB", // volt trips
		"AE", "AF", "AJ", "AK", // freq trips
		"BB", "BC", "BD", "BE", "BH", "BI", "BJ", "BK", // clear times
	}
	for _, code := range writable {
		if !Writeable(testQS1A, code) {
			t.Errorf("QS1A %q should be writeable; reason: %s", code, UnsupportedReason(testQS1A, code))
		}
	}
}

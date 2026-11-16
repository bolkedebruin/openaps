package sunspec

import (
	"math"
	"testing"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
)

func TestQS2SunSpecModelLabel(t *testing.T) {
	inv := source.Inverter{
		TypeCode: "03",
		Model:    0x36,
	}

	if got := inverterModelLabel(inv); got != "QS2" {
		t.Errorf("QS2 SunSpec model = %q, want QS2", got)
	}
}

// Verify that the QS2 exposes four distinct panel power readings
// through the actual SunSpec Model 160 register bank.
func TestQS2SunSpecModel160FourPanelRegisters(t *testing.T) {
	inv := source.Inverter{
		UID:        "821000011029",
		Online:     true,
		TypeCode:   "03",
		Model:      0x36,
		ACVoltageV: 230,
		PanelWatts: []int{25, 30, 10, 15},
	}

	bank := encodePerInverter(inv, "ECU", 2, Options{})

	mppt, ok := findModel(bank, MultiMPPTModelID)
	if !ok {
		t.Fatal("SunSpec Model 160 missing")
	}

	wantLen := MultiMPPTFixedBlockLen + MultiMPPTPerModuleLen*4
	if got := bank.At(mppt + 1); got != wantLen {
		t.Errorf("Model 160 length = %d, want %d", got, wantLen)
	}

	body := mppt + 2
	if got := bank.At(body + 6); got != 4 {
		t.Errorf("Model 160 module count = %d, want 4", got)
	}

	for i, want := range inv.PanelWatts {
		module := body + MultiMPPTFixedBlockLen +
			uint16(i)*MultiMPPTPerModuleLen

		// Each module begins with its ID, followed by its
		// label, DCA, DCV and DCW registers.
		if got := bank.At(module + 11); got != uint16(want) {
			t.Errorf(
				"panel %d DCW = %d, want %d",
				i+1, got, want,
			)
		}
	}
}

// Model 160 must expose real panel DC voltage/current rather than
// substituting AC grid voltage and a calculated current.
func TestQS2SunSpecRealDCMeasurements(t *testing.T) {
	inv := source.Inverter{
		UID:          "821000011029",
		Online:       true,
		TypeCode:     "03",
		Model:        0x36,
		ACVoltageV:   231,
		PanelWatts:   []int{42, 49, 43, 38},
		PanelDCVolts: []float64{41.4, 41.6, 41.3, 41.9},
		PanelDCAmps:  []float64{1.02, 1.18, 1.03, 0.90},
	}

	bank := encodePerInverter(inv, "ECU", 2, Options{})

	mppt, ok := findModel(bank, MultiMPPTModelID)
	if !ok {
		t.Fatal("SunSpec Model 160 missing")
	}

	body := mppt + 2

	// DCA_SF=-2, DCV_SF=-1, DCW_SF=0.
	if got := int16(bank.At(body)); got != -2 {
		t.Errorf("DCA scale factor = %d, want -2", got)
	}
	if got := int16(bank.At(body + 1)); got != -1 {
		t.Errorf("DCV scale factor = %d, want -1", got)
	}

	wantVolts := []uint16{414, 416, 413, 419}
	wantAmps := []uint16{102, 118, 103, 90}

	for i := range inv.PanelWatts {
		module := body + MultiMPPTFixedBlockLen +
			uint16(i)*MultiMPPTPerModuleLen

		if got := bank.At(module + 9); got != wantAmps[i] {
			t.Errorf("panel %d DCA = %d, want %d",
				i+1, got, wantAmps[i])
		}
		if got := bank.At(module + 10); got != wantVolts[i] {
			t.Errorf("panel %d DCV = %d, want %d",
				i+1, got, wantVolts[i])
		}
		if got := bank.At(module + 11); got != uint16(inv.PanelWatts[i]) {
			t.Errorf("panel %d DCW = %d, want %d",
				i+1, got, inv.PanelWatts[i])
		}

		// No measured panel temperature is available.
		if got := bank.At(module + 16); got != notImplS16 {
			t.Errorf("panel %d temperature = 0x%04X, want unavailable",
				i+1, got)
		}
	}
}

// The aggregate ECU bank must expose the same real DC measurements
// as the per-inverter bank.
func TestQS2AggregateModel160RealDCMeasurements(t *testing.T) {
	inv := source.Inverter{
		UID:          "821000011029",
		Online:       true,
		TypeCode:     "03",
		Model:        0x36,
		ACVoltageV:   231,
		PanelWatts:   []int{42, 49, 43, 38},
		PanelDCVolts: []float64{41.4, 41.6, 41.3, 41.9},
		PanelDCAmps:  []float64{1.02, 1.18, 1.03, 0.90},
	}

	bank := Encode(source.Snapshot{
		Inverters:           []source.Inverter{inv},
		InverterCount:       1,
		InverterOnlineCount: 1,
	}, Options{})

	mppt, ok := findModel(bank, MultiMPPTModelID)
	if !ok {
		t.Fatal("Aggregate SunSpec Model 160 missing")
	}

	body := mppt + 2

	if got := int16(bank.At(body)); got != -2 {
		t.Errorf("DCA_SF = %d, want -2", got)
	}
	if got := int16(bank.At(body + 1)); got != -1 {
		t.Errorf("DCV_SF = %d, want -1", got)
	}

	wantAmps := []uint16{102, 118, 103, 90}
	wantVolts := []uint16{414, 416, 413, 419}

	for i := range inv.PanelWatts {
		module := body + MultiMPPTFixedBlockLen +
			uint16(i)*MultiMPPTPerModuleLen

		if got := bank.At(module + 9); got != wantAmps[i] {
			t.Errorf("panel %d DCA = %d, want %d",
				i+1, got, wantAmps[i])
		}
		if got := bank.At(module + 10); got != wantVolts[i] {
			t.Errorf("panel %d DCV = %d, want %d",
				i+1, got, wantVolts[i])
		}
		if got := bank.At(module + 11); got != uint16(inv.PanelWatts[i]) {
			t.Errorf("panel %d DCW = %d, want %d",
				i+1, got, inv.PanelWatts[i])
		}
		if got := bank.At(module + 16); got != notImplS16 {
			t.Errorf("panel %d temperature = 0x%04X, want unavailable",
				i+1, got)
		}
	}
}

// Missing and invalid panel measurements must not become fabricated
// measurements or collide with the SunSpec unavailable sentinel.
func TestQS2InvalidDCMeasurements(t *testing.T) {
	inv := source.Inverter{
		PanelDCAmps:  []float64{0, math.NaN(), math.Inf(1), 655.349},
		PanelDCVolts: []float64{41.4, 0, math.Inf(-1), 6553.49},
	}

	cases := []struct {
		amps, volts uint16
	}{
		{0, 414},
		{notImplU16, notImplU16},
		{notImplU16, notImplU16},
		{notImplU16, notImplU16},
	}

	for i, want := range cases {
		amps, volts := panelDCRegisters(inv, i)
		if amps != want.amps || volts != want.volts {
			t.Errorf(
				"panel %d: amps=%d volts=%d; want amps=%d volts=%d",
				i+1, amps, volts, want.amps, want.volts,
			)
		}
	}

	amps, volts := panelDCRegisters(inv, 4)
	if amps != notImplU16 || volts != notImplU16 {
		t.Errorf("missing panel: amps=%d volts=%d, want unavailable", amps, volts)
	}
}

// A known QS2 must keep its SunSpec MPPT model even when it is
// offline at night. Stale measurements must not be exposed.
func TestQS2OfflineKeepsModel160(t *testing.T) {
	inv := source.Inverter{
		UID:          "821000011029",
		Online:       false,
		TypeCode:     "03",
		Model:        0x36,
		PanelWatts:   []int{42, 49, 43, 38},
		PanelDCVolts: []float64{41.4, 41.6, 41.3, 41.9},
		PanelDCAmps:  []float64{1.02, 1.18, 1.03, 0.90},
	}

	banks := map[string]Bank{
		"inverter": encodePerInverter(inv, "ECU", 2, Options{}),
		"aggregate": Encode(source.Snapshot{
			Inverters:     []source.Inverter{inv},
			InverterCount: 1,
		}, Options{}),
	}

	for name, bank := range banks {
		t.Run(name, func(t *testing.T) {
			mppt, ok := findModel(bank, MultiMPPTModelID)
			if !ok {
				t.Fatal("Model 160 disappeared for offline QS2")
			}

			body := mppt + 2
			if got := bank.At(body + 6); got != 4 {
				t.Errorf("MPPT module count = %d, want 4", got)
			}

			for i := 0; i < 4; i++ {
				module := body + MultiMPPTFixedBlockLen +
					uint16(i)*MultiMPPTPerModuleLen

				for _, offset := range []uint16{9, 10} {
					if got := bank.At(module + offset); got != notImplU16 {
						t.Errorf("panel %d offset %d = %d; want unavailable",
							i+1, offset, got)
					}
				}
				if got := bank.At(module + 11); got != notImplS16 {
					t.Errorf("panel %d power = %d; want unavailable", i+1, got)
				}
			}
		})
	}
}

// All known SunSpec inverter type codes must preserve their module
// layout when the inverter is offline, including at night.
func TestModel160OfflinePanelCountByType(t *testing.T) {
	cases := []struct {
		name     string
		typeCode string
		panels   uint16
	}{
		{"DS3_or_YC600", "01", 2},
		{"YC1000", "02", 4},
		{"QS1_or_QS2", "03", 4},
		{"DS3_H", "04", 2},
		{"QT2", "05", 4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := source.Inverter{
				UID:      "123456789012",
				TypeCode: tc.typeCode,
				Online:   false,
			}

			banks := map[string]Bank{
				"inverter": encodePerInverter(
					inv, "ECU", 2, Options{},
				),
				"aggregate": Encode(source.Snapshot{
					Inverters: []source.Inverter{inv},
				}, Options{}),
			}

			for bankName, bank := range banks {
				mppt, ok := findModel(bank, MultiMPPTModelID)
				if !ok {
					t.Errorf("%s: Model 160 missing", bankName)
					continue
				}

				body := mppt + 2

				if got := bank.At(body + 6); got != tc.panels {
					t.Errorf(
						"%s: N=%d, want %d",
						bankName, got, tc.panels,
					)
				}

				wantLength := MultiMPPTFixedBlockLen +
					MultiMPPTPerModuleLen*tc.panels

				if got := bank.At(mppt + 1); got != wantLength {
					t.Errorf(
						"%s: model length=%d, want %d",
						bankName, got, wantLength,
					)
				}

				// Offline panels must not expose fabricated power.
				for i := uint16(0); i < tc.panels; i++ {
					module := body + MultiMPPTFixedBlockLen +
						i*MultiMPPTPerModuleLen

					if got := bank.At(module + 11); got != notImplS16 {
						t.Errorf(
							"%s: panel %d power=%d, want unavailable",
							bankName, i+1, got,
						)
					}
				}
			}
		})
	}
}

// The aggregate Model 160 must retain stable channel identifiers and
// register positions when individual inverters go offline.
func TestModel160MixedFleetStableLayout(t *testing.T) {
	cases := []struct {
		name     string
		qsOnline bool
		dsOnline bool
	}{
		{"both_online", true, true},
		{"QS2_only", true, false},
		{"DS3_only", false, true},
		{"both_offline", false, false},
	}

	var reference []string

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			qs := source.Inverter{
				UID:        "821000011029",
				TypeCode:   "03",
				Model:      0x36,
				Online:     tc.qsOnline,
				PanelWatts: []int{40, 41, 42, 43},
			}

			ds := source.Inverter{
				UID:        "821000022222",
				TypeCode:   "01",
				Online:     tc.dsOnline,
				PanelWatts: []int{80, 81},
			}

			bank := Encode(source.Snapshot{
				Inverters: []source.Inverter{qs, ds},
			}, Options{})

			mppt, ok := findModel(bank, MultiMPPTModelID)
			if !ok {
				t.Fatal("Model 160 missing")
			}

			body := mppt + 2
			if n := bank.At(body + 6); n != 6 {
				t.Fatalf("MPPT channel count = %d, want 6", n)
			}

			labels := make([]string, 6)

			for i := 0; i < 6; i++ {
				module := body + MultiMPPTFixedBlockLen +
					uint16(i)*MultiMPPTPerModuleLen

				labels[i] = readString(bank, module+1, 8)

				// First four channels belong to QS2,
				// last two to DS3.
				online := tc.qsOnline
				if i >= 4 {
					online = tc.dsOnline
				}

				got := bank.At(module + 11)

				if !online && got != notImplS16 {
					t.Errorf(
						"module %d: offline power=%d, want unavailable",
						i+1, got,
					)
				}

				if online && got == notImplU16 {
					t.Errorf(
						"module %d: online power unavailable",
						i+1,
					)
				}
			}

			if reference == nil {
				reference = labels
			} else {
				for i := range labels {
					if labels[i] != reference[i] {
						t.Errorf(
							"module %d moved: got %q, want %q",
							i+1, labels[i], reference[i],
						)
					}
				}
			}
		})
	}
}

// Valid DC voltage/current must survive when panel power is missing.
func TestModel160PartialTelemetryPreservesDC(t *testing.T) {
	inv := source.Inverter{
		UID:          "821000011029",
		Online:       true,
		TypeCode:     "03",
		Model:        0x36,
		PanelWatts:   []int{45},
		PanelDCVolts: []float64{40, 41, 42, 43},
		PanelDCAmps:  []float64{1.1, 1.2, 1.3, 1.4},
	}

	banks := map[string]Bank{
		"inverter": encodePerInverter(inv, "ECU", 2, Options{}),
		"aggregate": Encode(source.Snapshot{
			Inverters: []source.Inverter{inv},
		}, Options{}),
	}

	for name, bank := range banks {
		t.Run(name, func(t *testing.T) {
			mppt, ok := findModel(bank, MultiMPPTModelID)
			if !ok {
				t.Fatal("Model 160 missing")
			}

			// Second panel: voltage/current present, power missing.
			module := mppt + 2 + MultiMPPTFixedBlockLen +
				MultiMPPTPerModuleLen

			dca := bank.At(module + 9)
			dcv := bank.At(module + 10)
			dcw := bank.At(module + 11)

			if dca == notImplU16 {
				t.Error("valid DC current was discarded")
			}
			if dcv == notImplU16 {
				t.Error("valid DC voltage was discarded")
			}
			if dcw != notImplS16 {
				t.Errorf("missing DC power = %d, want unavailable", dcw)
			}
		})
	}
}

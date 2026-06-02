package sunspec

import (
	"testing"

	"github.com/bolkedebruin/openaps/internal/sunspec/source"
)

func sampleInverter() source.Inverter {
	return source.Inverter{
		UID:           "999900000001",
		Online:        true,
		TypeCode:      "01",
		Phase:         1,
		ACPowerW:      468,
		ACVoltageV:    240,
		FrequencyHz:   50.02,
		TemperatureC:  31,
		Model:         32,
		SoftwareVer:   3067,
		LimitedPowerW: 258,
		PanelWatts:    []int{247, 221},
	}
}

func TestEncodePerInverter_BasicShape(t *testing.T) {
	bank := EncodePerInverter(sampleInverter(), "999999999999", 2, Options{})

	// Must contain Common, Inverter101, Multi-MPPT, end (in that order).
	if _, ok := findModel(bank, CommonModelID); !ok {
		t.Error("Common Model missing")
	}
	if _, ok := findModel(bank, InverterModelSinglePhase); !ok {
		t.Error("Inverter Model 101 missing")
	}
	if _, ok := findModel(bank, MultiMPPTModelID); !ok {
		t.Error("Multi-MPPT missing")
	}
	// Per-inverter banks now expose Model 120 (each inverter's own
	// nameplate watts via the model_int → watts table). The fleet-aggregate
	// Basic Settings (121) and Vendor (64202) models still belong only to
	// the aggregate bank.
	if _, ok := findModel(bank, NameplateModelID); !ok {
		t.Error("Nameplate (120) should be present in per-inverter bank")
	}
	if _, ok := findModel(bank, BasicSettingsModelID); ok {
		t.Error("Basic Settings (121) should not be in per-inverter bank")
	}
	if _, ok := findModel(bank, VendorModelID); ok {
		t.Error("Vendor model should not be in per-inverter bank")
	}
}

func TestEncodePerInverter_CommonModelFields(t *testing.T) {
	bank := EncodePerInverter(sampleInverter(), "999999999999", 5, Options{})

	mn := readString(bank, BaseRegister+4, 16)
	if mn != "APsystems" {
		t.Errorf("Mn=%q want APsystems", mn)
	}
	md := readString(bank, BaseRegister+20, 16)
	if md != "DS3" {
		t.Errorf("Md=%q want %q", md, "DS3")
	}
	vr := readString(bank, BaseRegister+44, 8)
	if vr != "3067" {
		t.Errorf("Vr=%q want 3067", vr)
	}
	sn := readString(bank, BaseRegister+52, 16)
	if sn != "999900000001" {
		t.Errorf("SN=%q want 999900000001 (the inverter UID, not the ECU)", sn)
	}
	// DA = unit ID
	da := bank.At(BaseRegister + 68)
	if da != 5 {
		t.Errorf("DA=%d want 5 (unit ID)", da)
	}
}

func TestEncodePerInverter_TypeCodeMapsToModelLabel(t *testing.T) {
	cases := map[string]string{
		"01": "DS3",
		"03": "QS1",
		"04": "DS3-H",
		"":   "microinverter",
		"99": "type-99",
	}
	for ty, want := range cases {
		t.Run(ty, func(t *testing.T) {
			inv := sampleInverter()
			inv.TypeCode = ty
			inv.PanelWatts = []int{100, 80}
			bank := EncodePerInverter(inv, "ECU", 2, Options{})
			got := readString(bank, BaseRegister+20, 16)
			if got != want {
				t.Errorf("TypeCode=%q: Md=%q want %q", ty, got, want)
			}
		})
	}
}

func TestEncodePerInverter_InverterModelW(t *testing.T) {
	bank := EncodePerInverter(sampleInverter(), "ECU", 2, Options{})
	invBase, ok := findModel(bank, InverterModelSinglePhase)
	if !ok {
		t.Fatal("inverter model missing")
	}
	body := invBase + 2
	if got := int16(bank.At(body + 12)); got != 468 {
		t.Errorf("W=%d want 468", got)
	}
	if got := bank.At(body + 14); got != 5002 {
		t.Errorf("Hz=%d want 5002 (50.02 × 100)", got)
	}
	if got := bank.At(body + 8); got != 240 {
		t.Errorf("PhVphA=%d want 240", got)
	}
}

func TestEncodePerInverter_MPPTHasOnlyThisInverterPanels(t *testing.T) {
	// type 01 → 2 panels
	bank01 := EncodePerInverter(sampleInverter(), "ECU", 2, Options{})
	mppt, _ := findModel(bank01, MultiMPPTModelID)
	if got := bank01.At(mppt + 1); got != MultiMPPTFixedBlockLen+MultiMPPTPerModuleLen*2 {
		t.Errorf("type 01 MPPT L=%d want %d (2 panels)",
			got, MultiMPPTFixedBlockLen+MultiMPPTPerModuleLen*2)
	}

	// type 03 → 4 panels
	inv := sampleInverter()
	inv.TypeCode = "03"
	inv.PanelWatts = []int{100, 80, 70, 50}
	bank03 := EncodePerInverter(inv, "ECU", 2, Options{})
	mppt, _ = findModel(bank03, MultiMPPTModelID)
	if got := bank03.At(mppt + 1); got != MultiMPPTFixedBlockLen+MultiMPPTPerModuleLen*4 {
		t.Errorf("type 03 MPPT L=%d want %d (4 panels)",
			got, MultiMPPTFixedBlockLen+MultiMPPTPerModuleLen*4)
	}
}

func TestEncodePerInverter_OfflineHasStSleep(t *testing.T) {
	inv := sampleInverter()
	inv.Online = false
	bank := EncodePerInverter(inv, "ECU", 2, Options{})
	invBase, _ := findModel(bank, InverterModelSinglePhase)
	body := invBase + 2
	if got := bank.At(body + 36); got != StSleep {
		t.Errorf("offline St=%d want StSleep", got)
	}
}

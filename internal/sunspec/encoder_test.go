package sunspec

import (
	"testing"

	"github.com/bolke/ecu-sunspec/internal/source"
)

func sampleSnapshot() source.Snapshot {
	return source.Snapshot{
		ECUID:               "999999999999",
		Firmware:            "2.1.29D",
		Model:               "ECU_R_PRO",
		PollingInterval:     60,
		SystemPowerW:        1234,
		LifetimeEnergyWh:    15_172_417,
		TodayEnergyWh:       26_233,
		MonthEnergyWh:       560_000,
		YearEnergyWh:        2_934_000,
		GridFrequencyHz:     50.02,
		GridVoltageV:        237,
		MaxTemperatureC:     31,
		InverterCount:       3,
		InverterOnlineCount: 3,
		Inverters: []source.Inverter{
			// type 01 (DS3, 2 panels): tail = [tmp, P0, V_ac, P1, V_ac]
			{UID: "999900000001", Online: true, TypeCode: "01", Phase: 1,
				ACPowerW: 468, ACVoltageV: 247, TemperatureC: 31,
				SignalStrength: 200, LimitedPowerW: 258, Model: 32, SoftwareVer: 3067,
				PanelWatts: []int{247, 221},
			},
			// type 03 (QS1, 4 panels): tail = [tmp, P0, V_ac, P1, P2, P3]
			{UID: "999900000002", Online: true, TypeCode: "03", Phase: 1,
				ACPowerW: 691, ACVoltageV: 232, TemperatureC: 38,
				SignalStrength: 195, LimitedPowerW: 258, Model: 24, SoftwareVer: 5203,
				PanelWatts: []int{200, 180, 160, 151},
			},
			// type 03, second
			{UID: "999900000003", Online: true, TypeCode: "03", Phase: 1,
				ACPowerW: 75, ACVoltageV: 230, TemperatureC: 51,
				SignalStrength: 209, LimitedPowerW: 258, Model: 24, SoftwareVer: 5203,
				PanelWatts: []int{25, 20, 15, 15},
			},
		},
	}
}

// findModel walks the SunSpec bank from the magic-skip offset to the end
// marker, returning the absolute address of the requested model's ID
// register. Tests use this so they don't need to recompute offsets when the
// model layout shifts.
func findModel(bank Bank, id uint16) (uint16, bool) {
	cur := bank.Base + 2 // skip "SunS"
	end := bank.Base + uint16(len(bank.Regs))
	for cur+1 < end {
		modID := bank.At(cur)
		if modID == 0xFFFF {
			return 0, false
		}
		if modID == id {
			return cur, true
		}
		modL := bank.At(cur + 1)
		next := cur + 2 + modL
		if next <= cur || next > end { // overflow / out-of-bank → bail
			return 0, false
		}
		cur = next
	}
	return 0, false
}

func TestEncode_LayoutInvariants(t *testing.T) {
	bank := Encode(sampleSnapshot(), Options{})

	// Magic at base.
	if got := bank.At(BaseRegister); got != 0x5375 {
		t.Errorf("magic[0]=%#04x want 0x5375", got)
	}
	if got := bank.At(BaseRegister + 1); got != 0x6E53 {
		t.Errorf("magic[1]=%#04x want 0x6E53", got)
	}

	// Common Model header.
	if got := bank.At(BaseRegister + 2); got != 1 {
		t.Errorf("common ID=%d want 1", got)
	}
	if got := bank.At(BaseRegister + 3); got != 66 {
		t.Errorf("common L=%d want 66", got)
	}

	// Walk the bank with findModel — no hardcoded offsets so the test survives
	// future model additions or removals.

	if invBase, ok := findModel(bank, InverterModelSinglePhase); !ok {
		t.Error("Inverter Model 101 missing")
	} else if bank.At(invBase+1) != 50 {
		t.Errorf("inverter L=%d want 50", bank.At(invBase+1))
	}

	// Sample has 3 inverters: 1 type-01 (2 panels) + 2 type-03 (4 panels each)
	// = 10 panels total. Body length = 8 + 20·10 = 208.
	if mpptBase, ok := findModel(bank, MultiMPPTModelID); !ok {
		t.Error("Multi-MPPT Model 160 missing")
	} else {
		const wantL uint16 = 8 + 20*10
		if got := bank.At(mpptBase + 1); got != wantL {
			t.Errorf("multi-MPPT L=%d want %d", got, wantL)
		}
	}

	if _, ok := findModel(bank, NameplateModelID); !ok {
		t.Error("Nameplate Model 120 missing")
	}
	if _, ok := findModel(bank, BasicSettingsModelID); !ok {
		t.Error("Basic Settings Model 121 missing")
	}
	if _, ok := findModel(bank, VendorModelID); !ok {
		t.Error("Vendor model missing")
	}

	// End marker — no fixed offset, but the last two regs of bank.Regs must
	// be 0xFFFF / 0.
	last := uint16(len(bank.Regs)) - 2
	if got := bank.At(bank.Base + last); got != 0xFFFF {
		t.Errorf("last-but-one register=%#04x want 0xFFFF", got)
	}
	if got := bank.At(bank.Base + last + 1); got != 0 {
		t.Errorf("last register=%d want 0", got)
	}
}

func TestEncode_CommonModelStrings(t *testing.T) {
	bank := Encode(sampleSnapshot(), Options{})

	// Common model body starts at base+4. Mn occupies 16 registers.
	mn := readString(bank, BaseRegister+4, 16)
	if mn != "APsystems" {
		t.Errorf("Mn=%q want %q", mn, "APsystems")
	}

	// Md at base+4+16=20 (16 regs). sampleSnapshot sets Model="ECU_R_PRO"
	// (mirroring what /etc/yuneng/model.conf contains on a stock unit), so
	// that's what should land in Md — not the DefaultModelName fallback.
	md := readString(bank, BaseRegister+20, 16)
	if md != "ECU_R_PRO" {
		t.Errorf("Md=%q want %q", md, "ECU_R_PRO")
	}

	// Vr at base+4+16+16+8=44 (8 regs).
	vr := readString(bank, BaseRegister+44, 8)
	if vr != "2.1.29D" {
		t.Errorf("Vr=%q want 2.1.29D", vr)
	}

	// SN at base+4+16+16+8+8=52 (16 regs).
	sn := readString(bank, BaseRegister+52, 16)
	if sn != "999999999999" {
		t.Errorf("SN=%q want 999999999999", sn)
	}
}

func TestEncode_InverterModelKeyFields(t *testing.T) {
	bank := Encode(sampleSnapshot(), Options{})

	invBase, ok := findModel(bank, InverterModelSinglePhase)
	if !ok {
		t.Fatal("Inverter Model 101 missing")
	}
	body := invBase + 2

	if got := bank.At(body + 12); int16(got) != 1234 {
		t.Errorf("W=%d want 1234", int16(got))
	}
	if got := bank.At(body + 14); got != 5002 {
		t.Errorf("Hz=%d want 5002 (50.02 Hz × 100)", got)
	}

	hi := bank.At(body + 22)
	lo := bank.At(body + 23)
	wh := uint64(hi)<<16 | uint64(lo)
	if wh != 15_172_417 {
		t.Errorf("WH=%d want 15172417", wh)
	}

	if got := bank.At(body + 36); got != StMPPT {
		t.Errorf("St=%d want StMPPT(%d)", got, StMPPT)
	}
}

func TestEncode_SleepStateWhenNoInvertersOnline(t *testing.T) {
	// At night the params file lists every inverter as online=0 and
	// each_system_power has gone stale. Reporting StMPPT off the stale
	// power sample would surface as "Running (MPPT)" in Victron; the
	// online count must gate the state instead.
	s := sampleSnapshot()
	s.InverterOnlineCount = 0
	// Leave SystemPowerW at the sample's nonzero value to model the
	// stale-row case.
	bank := Encode(s, Options{})
	invBase, ok := findModel(bank, InverterModelSinglePhase)
	if !ok {
		t.Fatal("Inverter Model 101 missing")
	}
	body := invBase + 2
	if got := bank.At(body + 36); got != StSleep {
		t.Errorf("St=%d want StSleep(%d)", got, StSleep)
	}
}

func TestEncode_StandbyAtNight(t *testing.T) {
	s := sampleSnapshot()
	s.SystemPowerW = 0
	// Inverters online but not producing — represents grid present, sun down.
	bank := Encode(s, Options{})
	invBase, ok := findModel(bank, InverterModelSinglePhase)
	if !ok {
		t.Fatal("Inverter Model 101 missing")
	}
	body := invBase + 2
	if got := bank.At(body + 36); got != StStandby {
		t.Errorf("St=%d want StStandby(%d)", got, StStandby)
	}
}

func TestAggregateOperatingState(t *testing.T) {
	cases := []struct {
		name        string
		online      int
		systemPower int32
		want        uint16
	}{
		{"no inverters, stale positive power", 0, 1234, StSleep},
		{"no inverters, zero power", 0, 0, StSleep},
		{"inverters online, producing", 3, 1234, StMPPT},
		{"inverters online, idle", 3, 0, StStandby},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := aggregateOperatingState(c.online, c.systemPower); got != c.want {
				t.Errorf("got=%d want=%d", got, c.want)
			}
		})
	}
}

func TestInverterOperatingState(t *testing.T) {
	cases := []struct {
		name    string
		online  bool
		acPower int
		want    uint16
	}{
		{"offline", false, 0, StSleep},
		{"offline with stale power", false, 250, StSleep},
		{"online producing", true, 250, StMPPT},
		{"online idle", true, 0, StStandby},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inverterOperatingState(c.online, c.acPower); got != c.want {
				t.Errorf("got=%d want=%d", got, c.want)
			}
		})
	}
}

func readString(bank Bank, addr uint16, registers int) string {
	buf := make([]byte, 0, registers*2)
	for i := 0; i < registers; i++ {
		r := bank.At(addr + uint16(i))
		buf = append(buf, byte(r>>8), byte(r))
	}
	// Trim trailing zeros — strings are zero-padded to register count.
	for i := len(buf) - 1; i >= 0; i-- {
		if buf[i] != 0 {
			return string(buf[:i+1])
		}
	}
	return ""
}

func TestEncode_DerivedPhaseCurrents(t *testing.T) {
	s := sampleSnapshot()
	bank := Encode(s, Options{})

	// All inverters phase=1 → all power on phase A.
	invBase, ok := findModel(bank, InverterModelSinglePhase)
	if !ok {
		t.Fatal("Inverter Model 101 missing")
	}
	body := invBase + 2
	const offsetAphA uint16 = 1
	const offsetASF uint16 = 4

	totalA := bank.At(body)
	aphA := bank.At(body + offsetAphA)
	asf := int16(bank.At(body + offsetASF))
	if asf != -1 {
		t.Errorf("ASF=%d want -1", asf)
	}
	// Σ power 1234 W / 237 V = 5.21 A → ASF=-1 → register 52.
	if totalA < 50 || totalA > 54 {
		t.Errorf("totalA=%d want ≈52", totalA)
	}
	if aphA < 50 || aphA > 54 {
		t.Errorf("aphA=%d want ≈52", aphA)
	}
}

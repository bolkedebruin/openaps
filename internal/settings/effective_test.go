package settings

import "testing"

func TestResolve_MACFallsBackToLive(t *testing.T) {
	live := func() string { return "80:97:1b:03:0d:ce" }
	got := Resolve(Settings{}, live)
	if got.MAC != "80:97:1b:03:0d:ce" {
		t.Errorf("MAC = %q, want live MAC", got.MAC)
	}
	// Operator-set MAC wins over the live reader.
	got = Resolve(Settings{MAC: "aa:bb:cc:dd:ee:ff"}, live)
	if got.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("operator MAC dropped: %q", got.MAC)
	}
}

func TestResolve_PANDerivedFromMAC(t *testing.T) {
	got := Resolve(Settings{}, func() string { return "80:97:1b:03:0d:ce" })
	if got.PAN != "0DCE" {
		t.Errorf("derived PAN = %q, want 0DCE", got.PAN)
	}
}

func TestResolve_PANOverrideWinsOverMAC(t *testing.T) {
	// Override is honoured even when a MAC would derive a different PAN.
	got := Resolve(Settings{MAC: "80:97:1b:03:0d:ce", PANOverride: "1a2b"}, nil)
	if got.PAN != "1A2B" {
		t.Errorf("PAN = %q, want 1A2B (override + upper)", got.PAN)
	}
	// Short overrides left-pad to 4 chars.
	got = Resolve(Settings{PANOverride: "f"}, nil)
	if got.PAN != "000F" {
		t.Errorf("PAN = %q, want 000F (padded)", got.PAN)
	}
}

func TestResolve_ZigbeeTypeDefaults(t *testing.T) {
	if got := Resolve(Settings{}, nil); got.ZigbeeType != "apsystems" {
		t.Errorf("default zigbee_type = %q, want apsystems", got.ZigbeeType)
	}
	if got := Resolve(Settings{ZigbeeType: "general"}, nil); got.ZigbeeType != "general" {
		t.Errorf("set zigbee_type = %q, want general", got.ZigbeeType)
	}
}

func TestResolve_AllEmpty(t *testing.T) {
	got := Resolve(Settings{}, func() string { return "" })
	if got.MAC != "" {
		t.Errorf("MAC = %q, want empty", got.MAC)
	}
	if got.PAN != "" {
		t.Errorf("PAN = %q, want empty (no MAC to derive from)", got.PAN)
	}
	if got.ZigbeeType != "apsystems" {
		t.Errorf("ZigbeeType = %q, want apsystems (default)", got.ZigbeeType)
	}
}

func TestResolve_MalformedMACDropsPAN(t *testing.T) {
	got := Resolve(Settings{MAC: "not-a-mac"}, nil)
	if got.PAN != "" {
		t.Errorf("PAN = %q, want empty on malformed MAC", got.PAN)
	}
}

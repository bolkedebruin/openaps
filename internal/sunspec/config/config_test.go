package config

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFile(t *testing.T) {
	c, err := Load("/nonexistent/path/sunspec.json")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if !c.Writes.IsEnabled() {
		t.Error("missing file should default to writes ENABLED")
	}
}

func TestLoad_OmittedEnabled_DefaultsTrue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sunspec.json")
	os.WriteFile(path, []byte(`{"writes": {"allow_list": ["10.0.0.1"]}}`), 0644)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Writes.IsEnabled() {
		t.Error("omitted enabled should default to true")
	}
}

func TestLoad_ExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sunspec.json")
	os.WriteFile(path, []byte(`{"writes": {"enabled": false}}`), 0644)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Writes.IsEnabled() {
		t.Error("explicit enabled=false should disable writes")
	}
}

func TestLoad_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sunspec.json")
	if err := os.WriteFile(path, []byte(`{
	  "writes": { "enabled": true, "allow_list": ["10.0.0.1", "192.168.1.0/24"] }
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Writes.IsEnabled() {
		t.Error("writes.enabled should be true")
	}
	if len(c.Writes.AllowList) != 2 {
		t.Errorf("AllowList len=%d want 2", len(c.Writes.AllowList))
	}
}

func TestLoad_InvalidIP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sunspec.json")
	os.WriteFile(path, []byte(`{"writes":{"allow_list":["not-an-ip"]}}`), 0644)
	if _, err := Load(path); err == nil {
		t.Error("expected error for invalid IP, got nil")
	}
}

func TestAllowsWrite_LocalNetwork(t *testing.T) {
	// Stub the local-network detector to claim 10.0.0.0/24 is local.
	prev := ipInLocalNetwork
	defer func() { ipInLocalNetwork = prev }()
	_, localNet, _ := net.ParseCIDR("10.0.0.0/24")
	ipInLocalNetwork = func(ip net.IP) bool { return localNet.Contains(ip) }

	cases := []struct {
		name    string
		allowLN *bool
		remote  string
		want    bool
	}{
		{"default (nil) allows local", nil, "10.0.0.42:5555", true},
		{"default (nil) blocks remote", nil, "8.8.8.8:5555", false},
		{"explicit true allows local", BoolPtr(true), "10.0.0.42:5555", true},
		{"explicit false blocks local", BoolPtr(false), "10.0.0.42:5555", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := Config{Writes: WritesConfig{Enabled: BoolPtr(true), AllowLocalNetwork: c.allowLN}}

			if got := cfg.AllowsWrite(c.remote); got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestAllowsWrite(t *testing.T) {
	cases := []struct {
		name   string
		cfg    Config
		remote string
		want   bool
	}{
		{"writes off",
			Config{Writes: WritesConfig{Enabled: BoolPtr(false), AllowList: []string{"10.0.0.1"}}},
			"10.0.0.1:5555", false},

		{"loopback always allowed when enabled",
			Config{Writes: WritesConfig{Enabled: BoolPtr(true)}},
			"127.0.0.1:5555", true},
		{"loopback not allowed when disabled",
			Config{Writes: WritesConfig{Enabled: BoolPtr(false)}},
			"127.0.0.1:5555", false},

		{"exact IP match",
			Config{Writes: WritesConfig{Enabled: BoolPtr(true), AllowList: []string{"10.0.0.1"}}},
			"10.0.0.1:5555", true},
		{"exact IP no match",
			Config{Writes: WritesConfig{Enabled: BoolPtr(true), AllowList: []string{"10.0.0.1"}}},
			"10.0.0.2:5555", false},

		{"CIDR match",
			Config{Writes: WritesConfig{Enabled: BoolPtr(true), AllowList: []string{"10.0.0.0/24"}}},
			"10.0.0.42:5555", true},
		{"CIDR no match",
			Config{Writes: WritesConfig{Enabled: BoolPtr(true), AllowList: []string{"10.0.0.0/24"}}},
			"10.0.1.42:5555", false},

		{"empty allowlist denies non-loopback",
			Config{Writes: WritesConfig{Enabled: BoolPtr(true), AllowList: nil}},
			"10.0.0.1:5555", false},

		{"address without port still parses",
			Config{Writes: WritesConfig{Enabled: BoolPtr(true), AllowList: []string{"10.0.0.1"}}},
			"10.0.0.1", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.cfg.AllowsWrite(c.remote)
			if got != c.want {
				t.Errorf("AllowsWrite(%q) = %v, want %v", c.remote, got, c.want)
			}
		})
	}
}

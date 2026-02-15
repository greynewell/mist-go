package config

import (
	"testing"
)

type testConfig struct {
	Name  string `toml:"name"`
	Port  int    `toml:"port"`
	Debug bool   `toml:"debug"`
	Rate  float64
}

func TestDecode(t *testing.T) {
	data := map[string]any{
		"name":  "matchspec",
		"port":  int64(8080),
		"debug": true,
		"rate":  1.5,
	}

	var cfg testConfig
	if err := Decode(data, &cfg); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if cfg.Name != "matchspec" {
		t.Errorf("Name = %q, want matchspec", cfg.Name)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.Debug != true {
		t.Errorf("Debug = %v, want true", cfg.Debug)
	}
	if cfg.Rate != 1.5 {
		t.Errorf("Rate = %v, want 1.5", cfg.Rate)
	}
}

func TestDecodeUsesLowercaseFieldName(t *testing.T) {
	data := map[string]any{
		"rate": 2.5,
	}

	var cfg testConfig
	if err := Decode(data, &cfg); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if cfg.Rate != 2.5 {
		t.Errorf("Rate = %v, want 2.5 (matched by lowercased field name)", cfg.Rate)
	}
}

func TestDecodeNestedStruct(t *testing.T) {
	type inner struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	type outer struct {
		Server inner `toml:"server"`
	}

	data := map[string]any{
		"server": map[string]any{
			"host": "localhost",
			"port": int64(9090),
		},
	}

	var cfg outer
	if err := Decode(data, &cfg); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("Server.Host = %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d", cfg.Server.Port)
	}
}

func TestDecodeSlice(t *testing.T) {
	type cfg struct {
		Tags []string `toml:"tags"`
	}

	data := map[string]any{
		"tags": []any{"a", "b", "c"},
	}

	var c cfg
	if err := Decode(data, &c); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(c.Tags) != 3 || c.Tags[0] != "a" {
		t.Errorf("Tags = %v, want [a b c]", c.Tags)
	}
}

func TestDecodeRequiresPointerToStruct(t *testing.T) {
	err := Decode(map[string]any{}, "not a struct")
	if err == nil {
		t.Error("expected error for non-pointer")
	}
}

func TestDecodeFloat64FromInt(t *testing.T) {
	type cfg struct {
		Rate float64 `toml:"rate"`
	}

	data := map[string]any{
		"rate": int64(5),
	}

	var c cfg
	if err := Decode(data, &c); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if c.Rate != 5.0 {
		t.Errorf("Rate = %v, want 5.0", c.Rate)
	}
}

func TestDecodeIntFromFloat(t *testing.T) {
	type cfg struct {
		Port int `toml:"port"`
	}

	data := map[string]any{
		"port": float64(8080),
	}

	var c cfg
	if err := Decode(data, &c); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if c.Port != 8080 {
		t.Errorf("Port = %d, want 8080", c.Port)
	}
}

func TestApplyEnv(t *testing.T) {
	type cfg struct {
		Name string
		Port int
	}

	c := cfg{Name: "default", Port: 80}
	t.Setenv("TEST_NAME", "override")
	t.Setenv("TEST_PORT", "9090")

	applyEnv("TEST", &c)

	if c.Name != "override" {
		t.Errorf("Name = %q, want override", c.Name)
	}
	if c.Port != 9090 {
		t.Errorf("Port = %d, want 9090", c.Port)
	}
}

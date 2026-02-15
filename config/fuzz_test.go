package config

import (
	"strings"
	"testing"
)

// FuzzParseTOML tests that the TOML parser never panics on arbitrary input.
func FuzzParseTOML(f *testing.F) {
	// Valid TOML.
	f.Add(`name = "test"
port = 8080
debug = true
rate = 3.14
tags = ["a", "b", "c"]

[server]
host = "localhost"
`)

	// Edge cases.
	f.Add("")
	f.Add("# comment only")
	f.Add("[table]")
	f.Add("[deeply.nested.table]")
	f.Add(`key = "value with \"escapes\""`)
	f.Add(`key = 'literal string'`)
	f.Add(`key = [1, 2, [3, 4]]`)
	f.Add(`"quoted.key" = "value"`)
	f.Add("key = 999999999999999999999999999")
	f.Add("key = -1")
	f.Add("key = 0.0")
	f.Add("[")
	f.Add("]")
	f.Add("=")
	f.Add("= value")
	f.Add("key =")

	f.Fuzz(func(t *testing.T, input string) {
		// Must never panic.
		result, err := ParseTOML(strings.NewReader(input))
		if err != nil {
			return
		}

		// If parsing succeeded, result should be a valid map.
		if result == nil {
			t.Error("successful parse returned nil map")
		}
	})
}

// FuzzDecode tests that Decode never panics with arbitrary maps.
func FuzzDecode(f *testing.F) {
	f.Add(`name = "test"
port = 8080
debug = true`)

	f.Fuzz(func(t *testing.T, input string) {
		data, err := ParseTOML(strings.NewReader(input))
		if err != nil {
			return
		}

		type Config struct {
			Name    string   `toml:"name"`
			Port    int64    `toml:"port"`
			Debug   bool     `toml:"debug"`
			Rate    float64  `toml:"rate"`
			Tags    []string `toml:"tags"`
		}

		var cfg Config
		Decode(data, &cfg)
		// No panic = pass.
	})
}

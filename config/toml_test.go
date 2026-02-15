package config

import (
	"strings"
	"testing"
)

func TestParseTOMLBasic(t *testing.T) {
	input := `
# comment
name = "matchspec"
port = 8080
debug = true
rate = 1.5
`
	data, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTOML: %v", err)
	}

	if data["name"] != "matchspec" {
		t.Errorf("name = %v, want matchspec", data["name"])
	}
	if data["port"] != int64(8080) {
		t.Errorf("port = %v (%T), want 8080", data["port"], data["port"])
	}
	if data["debug"] != true {
		t.Errorf("debug = %v, want true", data["debug"])
	}
	if data["rate"] != 1.5 {
		t.Errorf("rate = %v, want 1.5", data["rate"])
	}
}

func TestParseTOMLTable(t *testing.T) {
	input := `
[server]
host = "localhost"
port = 9090

[server.tls]
enabled = false
`
	data, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTOML: %v", err)
	}

	srv, ok := data["server"].(map[string]any)
	if !ok {
		t.Fatal("server should be a table")
	}
	if srv["host"] != "localhost" {
		t.Errorf("server.host = %v", srv["host"])
	}
	if srv["port"] != int64(9090) {
		t.Errorf("server.port = %v", srv["port"])
	}

	tls, ok := srv["tls"].(map[string]any)
	if !ok {
		t.Fatal("server.tls should be a table")
	}
	if tls["enabled"] != false {
		t.Errorf("server.tls.enabled = %v", tls["enabled"])
	}
}

func TestParseTOMLArray(t *testing.T) {
	input := `tags = ["eval", "trace", "mist"]`
	data, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTOML: %v", err)
	}

	arr, ok := data["tags"].([]any)
	if !ok {
		t.Fatal("tags should be an array")
	}
	if len(arr) != 3 {
		t.Fatalf("len(tags) = %d, want 3", len(arr))
	}
	want := []string{"eval", "trace", "mist"}
	for i, v := range arr {
		if v != want[i] {
			t.Errorf("tags[%d] = %v, want %v", i, v, want[i])
		}
	}
}

func TestParseTOMLQuotedKey(t *testing.T) {
	input := `"content-type" = "application/json"`
	data, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTOML: %v", err)
	}
	if data["content-type"] != "application/json" {
		t.Errorf("content-type = %v", data["content-type"])
	}
}

func TestParseTOMLLiteralString(t *testing.T) {
	input := `path = 'C:\Users\grey'`
	data, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTOML: %v", err)
	}
	if data["path"] != `C:\Users\grey` {
		t.Errorf("path = %v", data["path"])
	}
}

func TestParseTOMLInlineComment(t *testing.T) {
	input := `port = 8080 # the port`
	data, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTOML: %v", err)
	}
	if data["port"] != int64(8080) {
		t.Errorf("port = %v (%T)", data["port"], data["port"])
	}
}

func TestParseTOMLEmptyArray(t *testing.T) {
	input := `items = []`
	data, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTOML: %v", err)
	}
	arr, ok := data["items"].([]any)
	if !ok {
		t.Fatal("items should be an array")
	}
	if len(arr) != 0 {
		t.Errorf("len(items) = %d, want 0", len(arr))
	}
}

func TestParseTOMLUnclosedTable(t *testing.T) {
	input := `[broken`
	_, err := ParseTOML(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for unclosed table")
	}
}

func TestParseTOMLEmptyKey(t *testing.T) {
	input := ` = "value"`
	_, err := ParseTOML(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestParseTOMLEscapedStrings(t *testing.T) {
	input := `msg = "hello\nworld"`
	data, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTOML: %v", err)
	}
	if data["msg"] != "hello\nworld" {
		t.Errorf("msg = %q", data["msg"])
	}
}

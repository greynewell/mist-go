package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: "json", W: &buf}

	data := map[string]any{"name": "matchspec", "port": 8080}
	if err := w.JSON(data); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"name":"matchspec"`) {
		t.Errorf("output missing name: %s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("JSON output should end with newline")
	}
}

func TestJSONNoHTMLEscape(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: "json", W: &buf}

	data := map[string]string{"url": "http://example.com?a=1&b=2"}
	if err := w.JSON(data); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	if strings.Contains(buf.String(), `\u0026`) {
		t.Error("HTML escaping should be disabled")
	}
}

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: "table", W: &buf}

	headers := []string{"Name", "Port"}
	rows := [][]string{
		{"matchspec", "8080"},
		{"infermux", "8081"},
	}

	w.Table(headers, rows)

	out := buf.String()
	if !strings.Contains(out, "matchspec") {
		t.Error("table should contain matchspec")
	}
	if !strings.Contains(out, "infermux") {
		t.Error("table should contain infermux")
	}
	if !strings.Contains(out, "Name") {
		t.Error("table should contain header")
	}
}

func TestWrite(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: "json", W: &buf}

	if err := w.Write(map[string]string{"key": "val"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.Contains(buf.String(), "key") {
		t.Error("Write should output JSON")
	}
}

func TestNew(t *testing.T) {
	w := New("json")
	if w.Format != "json" {
		t.Errorf("Format = %q, want json", w.Format)
	}
}

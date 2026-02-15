package transport

import (
	"testing"
)

func TestDialHTTP(t *testing.T) {
	tr, err := Dial("http://localhost:8080")
	if err != nil {
		t.Fatalf("Dial http: %v", err)
	}
	if _, ok := tr.(*HTTP); !ok {
		t.Errorf("expected *HTTP, got %T", tr)
	}
}

func TestDialHTTPS(t *testing.T) {
	tr, err := Dial("https://example.com")
	if err != nil {
		t.Fatalf("Dial https: %v", err)
	}
	if _, ok := tr.(*HTTP); !ok {
		t.Errorf("expected *HTTP, got %T", tr)
	}
}

func TestDialFile(t *testing.T) {
	tr, err := Dial("file:///tmp/test.jsonl")
	if err != nil {
		t.Fatalf("Dial file: %v", err)
	}
	if _, ok := tr.(*File); !ok {
		t.Errorf("expected *File, got %T", tr)
	}
}

func TestDialStdio(t *testing.T) {
	tr, err := Dial("stdio://")
	if err != nil {
		t.Fatalf("Dial stdio: %v", err)
	}
	if _, ok := tr.(*Stdio); !ok {
		t.Errorf("expected *Stdio, got %T", tr)
	}
}

func TestDialChannel(t *testing.T) {
	tr, err := Dial("chan://")
	if err != nil {
		t.Fatalf("Dial chan: %v", err)
	}
	if _, ok := tr.(*Channel); !ok {
		t.Errorf("expected *Channel, got %T", tr)
	}
}

func TestDialUnknownScheme(t *testing.T) {
	_, err := Dial("ftp://example.com")
	if err == nil {
		t.Error("expected error for unknown scheme")
	}
}

func TestSplitScheme(t *testing.T) {
	tests := []struct {
		url    string
		scheme string
		rest   string
	}{
		{"http://localhost:8080", "http", "localhost:8080"},
		{"file:///tmp/data.jsonl", "file", "/tmp/data.jsonl"},
		{"stdio://", "stdio", ""},
		{"chan://", "chan", ""},
		{"noscheme", "", "noscheme"},
	}

	for _, tt := range tests {
		scheme, rest := splitScheme(tt.url)
		if scheme != tt.scheme || rest != tt.rest {
			t.Errorf("splitScheme(%q) = (%q, %q), want (%q, %q)",
				tt.url, scheme, rest, tt.scheme, tt.rest)
		}
	}
}

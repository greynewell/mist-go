package config

import (
	"strings"
	"testing"
)

type validationConfig struct {
	Name   string  `validate:"required"`
	Port   int64   `validate:"required,min=1,max=65535"`
	Rate   float64 `validate:"min=0.0,max=1.0"`
	Format string  `validate:"oneof=json text csv"`
	Debug  bool
}

func TestValidateRequired(t *testing.T) {
	cfg := validationConfig{Port: 8080, Rate: 0.5, Format: "json"}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected error for missing Name")
	}
	if !strings.Contains(err.Error(), "Name") {
		t.Errorf("error = %q, want mention of Name", err.Error())
	}
}

func TestValidateMinMax(t *testing.T) {
	cfg := validationConfig{Name: "test", Port: 0, Rate: 0.5, Format: "json"}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected error for Port=0")
	}

	cfg.Port = 99999
	err = Validate(&cfg)
	if err == nil {
		t.Fatal("expected error for Port>65535")
	}
}

func TestValidateFloatRange(t *testing.T) {
	cfg := validationConfig{Name: "test", Port: 80, Rate: 1.5, Format: "json"}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected error for Rate>1.0")
	}

	cfg.Rate = -0.1
	err = Validate(&cfg)
	if err == nil {
		t.Fatal("expected error for Rate<0")
	}
}

func TestValidateOneOf(t *testing.T) {
	cfg := validationConfig{Name: "test", Port: 80, Rate: 0.5, Format: "xml"}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected error for Format=xml")
	}
	if !strings.Contains(err.Error(), "one of") {
		t.Errorf("error = %q, want 'one of'", err.Error())
	}
}

func TestValidateAllValid(t *testing.T) {
	cfg := validationConfig{Name: "test", Port: 8080, Rate: 0.5, Format: "json"}
	err := Validate(&cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateStringMinMax(t *testing.T) {
	type cfg struct {
		Name string `validate:"min=3,max=10"`
	}

	err := Validate(&cfg{Name: "ab"})
	if err == nil {
		t.Error("expected error for too short")
	}

	err = Validate(&cfg{Name: "this is way too long"})
	if err == nil {
		t.Error("expected error for too long")
	}

	err = Validate(&cfg{Name: "hello"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateNoTags(t *testing.T) {
	type cfg struct {
		Name string
		Port int
	}
	err := Validate(&cfg{})
	if err != nil {
		t.Errorf("no tags should pass: %v", err)
	}
}

func TestValidateNonStruct(t *testing.T) {
	s := "not a struct"
	err := Validate(&s)
	if err == nil {
		t.Error("expected error for non-struct")
	}
}

func TestValidateZeroFloatWithMin(t *testing.T) {
	type cfg struct {
		Rate float64 `validate:"min=0"`
	}
	err := Validate(&cfg{Rate: 0})
	if err != nil {
		t.Errorf("zero should satisfy min=0: %v", err)
	}
}

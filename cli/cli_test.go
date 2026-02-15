package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewAppHasVersionCommand(t *testing.T) {
	app := NewApp("test", "1.0.0")
	if _, ok := app.commands["version"]; !ok {
		t.Error("expected built-in version command")
	}
}

func TestAddCommand(t *testing.T) {
	app := NewApp("test", "1.0.0")
	app.AddCommand(&Command{
		Name:  "greet",
		Usage: "Say hello",
		Run:   func(_ *Command, _ []string) error { return nil },
	})

	if _, ok := app.commands["greet"]; !ok {
		t.Error("expected greet command")
	}
}

func TestExecuteUnknownCommand(t *testing.T) {
	app := NewApp("test", "1.0.0")
	app.out = &bytes.Buffer{}

	err := app.Execute([]string{"nope"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected 'unknown command' in error, got %q", err)
	}
}

func TestExecuteNoArgs(t *testing.T) {
	app := NewApp("test", "1.0.0")
	app.out = &bytes.Buffer{}

	err := app.Execute(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteHelp(t *testing.T) {
	app := NewApp("test", "1.0.0")
	var buf bytes.Buffer
	app.out = &buf

	for _, arg := range []string{"help", "-h", "--help"} {
		buf.Reset()
		err := app.Execute([]string{arg})
		if err != nil {
			t.Fatalf("Execute(%q): %v", arg, err)
		}
		if !strings.Contains(buf.String(), "Commands:") {
			t.Errorf("help for %q should contain 'Commands:'", arg)
		}
	}
}

func TestExecuteRunsCommand(t *testing.T) {
	app := NewApp("test", "1.0.0")
	var ran bool

	app.AddCommand(&Command{
		Name:  "run",
		Usage: "Run something",
		Run: func(_ *Command, _ []string) error {
			ran = true
			return nil
		},
	})

	err := app.Execute([]string{"run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ran {
		t.Error("expected command to run")
	}
}

func TestExecutePassesArgs(t *testing.T) {
	app := NewApp("test", "1.0.0")
	var got []string

	app.AddCommand(&Command{
		Name:  "echo",
		Usage: "Echo args",
		Run: func(_ *Command, args []string) error {
			got = args
			return nil
		},
	})

	err := app.Execute([]string{"echo", "hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Errorf("got args %v, want [hello world]", got)
	}
}

func TestAddCommandCreatesDefaultFlagSet(t *testing.T) {
	app := NewApp("test", "1.0.0")
	cmd := &Command{
		Name:  "noflag",
		Usage: "No flags",
		Run:   func(_ *Command, _ []string) error { return nil },
	}
	app.AddCommand(cmd)

	if cmd.Flags == nil {
		t.Error("expected default FlagSet to be created")
	}
}

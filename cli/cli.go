// Package cli provides a zero-dependency subcommand framework shared
// across all MIST stack tools.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"
)

// App is the top-level CLI application.
type App struct {
	Name     string
	Version  string
	commands map[string]*Command
	out      io.Writer
}

// Command is a single CLI subcommand with its own flag set.
type Command struct {
	Name  string
	Usage string
	Flags *flag.FlagSet
	Run   func(cmd *Command, args []string) error
}

// NewApp creates an application with the built-in version command.
func NewApp(name, version string) *App {
	a := &App{
		Name:     name,
		Version:  version,
		commands: make(map[string]*Command),
		out:      os.Stderr,
	}
	a.AddCommand(&Command{
		Name:  "version",
		Usage: "Print the version and exit",
		Run: func(_ *Command, _ []string) error {
			fmt.Fprintf(os.Stdout, "%s %s\n", a.Name, a.Version)
			return nil
		},
	})
	return a
}

// AddCommand registers a subcommand.
func (a *App) AddCommand(c *Command) {
	if c.Flags == nil {
		c.Flags = flag.NewFlagSet(c.Name, flag.ExitOnError)
	}
	a.commands[c.Name] = c
}

// Execute parses the argument list and runs the matching subcommand.
func (a *App) Execute(args []string) error {
	if len(args) == 0 {
		a.printUsage()
		return nil
	}

	name := args[0]
	if name == "-h" || name == "--help" || name == "help" {
		a.printUsage()
		return nil
	}

	cmd, ok := a.commands[name]
	if !ok {
		fmt.Fprintf(a.out, "unknown command: %s\n\n", name)
		a.printUsage()
		return fmt.Errorf("unknown command: %s", name)
	}

	if err := cmd.Flags.Parse(args[1:]); err != nil {
		return err
	}

	return cmd.Run(cmd, cmd.Flags.Args())
}

func (a *App) printUsage() {
	w := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Usage: %s <command> [flags]\n\n", a.Name)
	fmt.Fprintln(w, "Commands:")

	names := make([]string, 0, len(a.commands))
	for name := range a.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintf(w, "  %s\t%s\n", name, a.commands[name].Usage)
	}
	w.Flush()
}

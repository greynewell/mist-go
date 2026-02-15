// mist is a lightweight utility for the MIST stack. It provides
// health checking, message validation, and pipe relay between transports.
//
// Usage:
//
//	mist version          Print version
//	mist ping <url>       Send health.ping to a MIST service
//	mist validate         Read JSON messages from stdin, validate envelope
//	mist relay <src> <dst> Relay messages between two transport URLs
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/greynewell/mist-go/cli"
	"github.com/greynewell/mist-go/protocol"
	"github.com/greynewell/mist-go/transport"
)

var version = "dev"

func main() {
	app := cli.NewApp("mist", version)

	app.AddCommand(&cli.Command{
		Name:  "ping",
		Usage: "Send health.ping to a MIST service URL",
		Run:   cmdPing,
	})

	app.AddCommand(&cli.Command{
		Name:  "validate",
		Usage: "Read JSON messages from stdin, validate envelope format",
		Run:   cmdValidate,
	})

	app.AddCommand(&cli.Command{
		Name:  "relay",
		Usage: "Relay messages between two transport URLs (src dst)",
		Run:   cmdRelay,
	})

	if err := app.Execute(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

func cmdPing(_ *cli.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: mist ping <url>")
	}

	t, err := transport.Dial(args[0])
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer t.Close()

	msg, err := protocol.New("mist-cli", protocol.TypeHealthPing, protocol.HealthPing{
		From: "mist-cli",
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	if err := t.Send(ctx, msg); err != nil {
		return fmt.Errorf("send: %w", err)
	}

	fmt.Fprintf(os.Stderr, "ping sent to %s (%v)\n", args[0], time.Since(start))
	return nil
}

func cmdValidate(_ *cli.Command, _ []string) error {
	decoder := json.NewDecoder(os.Stdin)
	var valid, invalid int

	for decoder.More() {
		var msg protocol.Message
		if err := decoder.Decode(&msg); err != nil {
			fmt.Fprintf(os.Stderr, "invalid: %v\n", err)
			invalid++
			continue
		}

		if msg.Version == "" || msg.Type == "" || msg.Source == "" {
			fmt.Fprintf(os.Stderr, "invalid: missing required fields (id=%s)\n", msg.ID)
			invalid++
			continue
		}
		valid++
	}

	fmt.Fprintf(os.Stdout, `{"valid":%d,"invalid":%d}`+"\n", valid, invalid)
	if invalid > 0 {
		return fmt.Errorf("%d invalid messages", invalid)
	}
	return nil
}

func cmdRelay(_ *cli.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: mist relay <src-url> <dst-url>")
	}

	src, err := transport.Dial(args[0])
	if err != nil {
		return fmt.Errorf("dial src: %w", err)
	}
	defer src.Close()

	dst, err := transport.Dial(args[1])
	if err != nil {
		return fmt.Errorf("dial dst: %w", err)
	}
	defer dst.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var count int64
	fmt.Fprintf(os.Stderr, "relaying %s → %s\n", args[0], args[1])

	for {
		msg, err := src.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			return fmt.Errorf("receive: %w", err)
		}

		if err := dst.Send(ctx, msg); err != nil {
			return fmt.Errorf("send: %w", err)
		}
		count++
	}

	fmt.Fprintf(os.Stderr, "relayed %d messages\n", count)
	return nil
}

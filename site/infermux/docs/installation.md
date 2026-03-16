---
title: "Installation"
description: "Install the infermux CLI, Go library, or Docker image."
---

# Installation

infermux is available as a CLI binary, a Go library, and a Docker image.

## CLI

Install the `infermux` command-line tool with `go install`:

```bash
go install github.com/greynewell/infermux/cmd/infermux@latest
```

This downloads, compiles, and places the binary in `$GOPATH/bin` (typically `~/go/bin`). Make sure that directory is on your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Verify:

```bash
infermux --version
# infermux v0.4.0 (go1.22, darwin/arm64)
```

To install a specific version:

```bash
go install github.com/greynewell/infermux/cmd/infermux@v0.4.0
```

## Go library

To embed infermux in a Go program or use it as a library:

```bash
go get github.com/greynewell/infermux
```

The package root exports the `Router` type and all provider interfaces. See the [Go API reference](/infermux/docs/go-api/) for usage.

## Docker image

```bash
docker pull ghcr.io/greynewell/infermux:latest
```

Run with a config file mounted:

```bash
docker run --rm \
  -p 8080:8080 \
  -v $(pwd)/infermux.yml:/etc/infermux/infermux.yml \
  -e OPENAI_API_KEY \
  -e ANTHROPIC_API_KEY \
  ghcr.io/greynewell/infermux:latest \
  serve --config /etc/infermux/infermux.yml
```

Tagged versions follow the same scheme as the Go module:

```bash
docker pull ghcr.io/greynewell/infermux:v0.4.0
```

The Docker image is a single static binary built from `scratch`. There is no shell, no package manager, and no runtime beyond the binary itself.

## Verifying installation

After installing, run the validator against an empty config to confirm the binary works:

```bash
infermux validate --config /dev/null
# error: config file is empty or missing required fields
# this is expected — it means the binary is working
```

Run the help command to see all available subcommands:

```bash
infermux --help
```

```
infermux — inference router for AI systems

Usage:
  infermux <command> [flags]

Commands:
  serve      Start the infermux server
  validate   Validate a config file without starting the server
  status     Show provider health and circuit breaker state
  providers  List configured providers and their status

Flags:
  --version   Print version and exit
  --help      Show help

Use "infermux <command> --help" for command-specific flags.
```

## System requirements

infermux is a static binary with no runtime dependencies. It requires:

- Linux, macOS, or Windows (arm64 or amd64)
- Network access to your configured LLM providers
- Go 1.21 or later (only required to build from source or use as a library)

## Building from source

```bash
git clone https://github.com/greynewell/infermux
cd infermux
go build -o infermux ./cmd/infermux
./infermux --version
```

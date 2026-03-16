---
title: "Installation"
description: "Install the schemaflux binary using go install, verify the installation, and keep it up to date."
---

# Installation

schemaflux ships as a single static binary with no runtime dependencies. The recommended installation method is `go install`.

## Requirements

- Go 1.21 or later
- A supported OS: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)

## Install with go install

```bash
go install github.com/greynewell/schemaflux/cmd/schemaflux@latest
```

This downloads, compiles, and installs the `schemaflux` binary to your `$GOPATH/bin` directory (typically `~/go/bin`). Make sure that directory is on your `$PATH`:

```bash
# Add to ~/.zshrc or ~/.bashrc if not already present
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Verify the installation

```bash
schemaflux version
```

Expected output:

```
schemaflux v0.9.0 (go1.22.3/darwin/arm64)
```

Run a quick help check:

```bash
schemaflux --help
```

## Install a specific version

To pin to a specific release:

```bash
go install github.com/greynewell/schemaflux/cmd/schemaflux@v0.9.0
```

Available versions are listed on the [GitHub releases page](https://github.com/greynewell/schemaflux/releases).

## Update to the latest version

Re-run the install command:

```bash
go install github.com/greynewell/schemaflux/cmd/schemaflux@latest
```

## Install from source

```bash
git clone https://github.com/greynewell/schemaflux.git
cd schemaflux
go build -o schemaflux ./cmd/schemaflux
mv schemaflux /usr/local/bin/
```

## Use as a Go library

schemaflux can be imported as a Go package for programmatic use or to write custom passes and backends:

```bash
go get github.com/greynewell/schemaflux
```

See the [Go API reference in backends](/schemaflux/docs/backends/#custom-backend) for the Backend interface, and [The 12 Passes](/schemaflux/docs/passes/) for the pass interface.

## Bundled tools

Installing `schemaflux` also installs two bundled tools:

```bash
go install github.com/greynewell/schemaflux/cmd/pssg@latest
go install github.com/greynewell/schemaflux/cmd/graph2md@latest
```

- **pssg** — personal static site generator; the tool that builds evaldriven.org. Pre-configured pipeline with opinionated defaults for personal sites and blogs.
- **graph2md** — reads a compiled IR (or a `schemaflux build --json` output) and writes enriched markdown files with computed frontmatter fields restored.

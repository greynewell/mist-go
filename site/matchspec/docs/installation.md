---
title: Installation
description: Install the matchspec Go package and CLI binary, and verify your setup.
---

# Installation

matchspec has two components: the Go package (`github.com/greynewell/matchspec`) for programmatic use and embedding in Go code, and the CLI binary (`matchspec`) for running evals from the command line and CI pipelines. You can install one or both depending on your use case.

## System requirements

- Go 1.21 or later
- A working Go toolchain (`go`, `GOPATH`, and `GOBIN` configured)

matchspec has no runtime system dependencies. Graders that call external services (such as `semantic_similarity` and `llm_judge`) require network access to those services, but the binary itself depends only on the Go standard library.

## Install the Go package

```bash
go get github.com/greynewell/matchspec
```

This adds matchspec as a module dependency in your `go.mod` and downloads the source. Import it in your Go code:

```go
import "github.com/greynewell/matchspec"
```

To pin a specific version:

```bash
go get github.com/greynewell/matchspec@v0.3.1
```

## Install the CLI

The CLI lives in `github.com/greynewell/matchspec/cmd/matchspec`. Install it with `go install`:

```bash
go install github.com/greynewell/matchspec/cmd/matchspec@latest
```

This builds the binary and places it in `$GOBIN` (or `$GOPATH/bin` if `GOBIN` is not set). Make sure that directory is on your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

To install a specific version of the CLI:

```bash
go install github.com/greynewell/matchspec/cmd/matchspec@v0.3.1
```

## Verify the installation

Check that the CLI is available and shows the correct version:

```bash
matchspec --version
# matchspec v0.3.1 (go1.22.0/darwin/arm64)
```

Run the built-in self-check, which verifies that the binary can find a config file and that all referenced harness files are parseable:

```bash
matchspec validate
# ✓ matchspec.yml found
# ✓ 2 harnesses valid
# ✓ 1 suite configured
```

If no `matchspec.yml` is present, `validate` will report the missing file. Run `matchspec init` to create a starter config.

## Install in CI

For GitHub Actions and other CI systems, install the CLI as part of your workflow setup step. The `go install` approach works but requires a Go environment. See [CI/CD Integration](/matchspec/docs/ci-cd/) for complete workflow examples including caching.

A typical Actions step:

```yaml
- name: Install matchspec
  run: go install github.com/greynewell/matchspec/cmd/matchspec@latest
```

## Upgrading

To upgrade to the latest version of both the package and CLI:

```bash
go get github.com/greynewell/matchspec@latest
go install github.com/greynewell/matchspec/cmd/matchspec@latest
```

Check for breaking changes in the [GitHub releases](https://github.com/greynewell/matchspec/releases) before upgrading across major versions.

## Building from source

Clone the repository and build the CLI manually:

```bash
git clone https://github.com/greynewell/matchspec.git
cd matchspec
go build -o matchspec ./cmd/matchspec
./matchspec --version
```

To run the test suite:

```bash
go test ./...
```

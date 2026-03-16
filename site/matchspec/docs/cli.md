---
title: CLI Reference
description: Complete reference for all matchspec CLI commands, flags, exit codes, and environment variables.
---

# CLI Reference

The `matchspec` CLI provides commands for running eval suites, generating reports, initializing projects, and validating configuration.

## Installation

```bash
go install github.com/greynewell/matchspec/cmd/matchspec@latest
```

See [Installation](/matchspec/docs/installation/) for full setup instructions.

## matchspec run

Run one or more eval suites.

```
matchspec run [path...] [flags]
```

If `path` is omitted, `matchspec run` reads `matchspec.yml` from the current directory and runs all configured suites. You can also pass one or more explicit paths:

```bash
# Run all suites in matchspec.yml
matchspec run

# Run a single harness file directly
matchspec run ./evals/summarization/harness.yml

# Run all harnesses in a directory
matchspec run ./evals/summarization/

# Run multiple harnesses
matchspec run ./evals/summarization/ ./evals/qa/
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--suite`, `-s` | string | — | Run only the named suite (as defined in `matchspec.yml`). |
| `--tags` | string | — | Comma-separated list of tags. Only run examples with at least one matching tag. |
| `--concurrency`, `-c` | integer | from config | Override concurrency for all harnesses. |
| `--timeout` | integer | from config | Override per-example timeout (seconds) for all harnesses. |
| `--output`, `-o` | string | `json` | Output format for results file: `json`, `junit`, `markdown`. |
| `--output-dir` | string | `.matchspec/results` | Directory to write results files. |
| `--no-write` | bool | false | Print results to stdout only; do not write results files. |
| `--show-all-failures` | bool | false | Print all failing examples, not just the first 5. |
| `--fail-fast` | bool | false | Stop after the first harness failure. |
| `--dry-run` | bool | false | Load and validate everything but don't call the model. |
| `--config` | string | `./matchspec.yml` | Path to the config file. |
| `--verbose`, `-v` | bool | false | Print per-example results during the run. |
| `--quiet`, `-q` | bool | false | Suppress all output except the final verdict. |

### Examples

```bash
# Run with verbose per-example output
matchspec run --verbose

# Run only science-tagged examples
matchspec run --tags science

# Run the "smoke" suite only
matchspec run --suite smoke

# Write results in JUnit format for CI test reporting
matchspec run --output junit --output-dir ./test-results

# Dry run to verify config is parseable
matchspec run --dry-run
```

### Exit codes

| Code | Meaning |
|---|---|
| `0` | All suites passed all thresholds. |
| `1` | One or more suites failed. |
| `2` | Configuration error (malformed config, missing file, etc.). |
| `3` | Runtime error (network failure, all model calls failed, etc.). |
| `130` | Interrupted (SIGINT). |

Exit code 0 is the only success code. Any non-zero exit code should fail a CI build.

---

## matchspec report

Generate a report from existing results without re-running evals.

```
matchspec report [results-file] [flags]
```

```bash
# Report on the most recent results
matchspec report

# Report on a specific results file
matchspec report .matchspec/results/summarization-20260315-143022.json

# Generate a markdown summary
matchspec report --format markdown > report.md

# Compare two runs
matchspec report --compare .matchspec/results/run-001.json .matchspec/results/run-002.json
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--format`, `-f` | string | `text` | Output format: `text`, `markdown`, `json`. |
| `--compare` | string | — | Path to a second results file to diff against. |
| `--show-passing` | bool | false | Include passing examples in the report (default: only show failures). |

---

## matchspec init

Initialize a matchspec project in the current directory.

```
matchspec init [flags]
```

Creates a starter `matchspec.yml` and (optionally) example harness and dataset files:

```bash
# Create just matchspec.yml
matchspec init

# Create matchspec.yml plus example harness and dataset
matchspec init --with-examples
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--with-examples` | bool | false | Generate example harness and dataset files. |
| `--force` | bool | false | Overwrite existing `matchspec.yml`. |

---

## matchspec validate

Validate all configuration files without running evals.

```
matchspec validate [flags]
```

Checks:
- `matchspec.yml` is present and parseable
- All referenced harness files exist and are valid
- All referenced dataset files exist and are valid
- All grader types are known
- All threshold values are in range

```bash
matchspec validate
# ✓ matchspec.yml found
# ✓ harness: summarization-v2 (./evals/summarization/harness.yml)
# ✓ harness: qa-v1 (./evals/qa/harness.yml)
# ✓ dataset: summarization-basic (120 examples)
# ✓ dataset: qa-basic (80 examples)
# ✓ 2 harnesses, 2 datasets, 4 graders — all valid
```

Validation errors:

```bash
matchspec validate
# ✓ matchspec.yml found
# ✗ harness: qa-v1: dataset file not found: ./evals/qa/dataset.yml
# ✗ harness: summarization-v2: unknown grader type "fuzzy_match"
# 2 errors found. Fix before running.
# exit code: 2
```

---

## matchspec serve

Start the HTTP API server.

```
matchspec serve [flags]
```

See [HTTP API](/matchspec/docs/http-api/) for complete documentation.

```bash
matchspec serve --port 8090
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--port`, `-p` | integer | `8090` | Port to listen on. |
| `--host` | string | `127.0.0.1` | Host to bind to. Use `0.0.0.0` to listen on all interfaces. |
| `--config` | string | `./matchspec.yml` | Path to the config file. |

---

## Environment variables

| Variable | Description |
|---|---|
| `MATCHSPEC_CONFIG` | Path to `matchspec.yml`. Overrides `--config`. |
| `MATCHSPEC_OUTPUT_DIR` | Directory for results files. Overrides `--output-dir`. |
| `MATCHSPEC_CONCURRENCY` | Default concurrency for all harnesses. Overrides harness config. |
| `MATCHSPEC_LOG_LEVEL` | Log verbosity: `debug`, `info`, `warn`, `error`. Default: `info`. |
| `MATCHSPEC_API_KEY` | API key for the HTTP server's authentication middleware. |

Model API keys (e.g., `OPENAI_API_KEY`) are referenced in harness configs via `api_key_env` and read directly from the environment at runtime. They are not matchspec-specific environment variables — they just need to be set in the shell or CI environment where `matchspec run` executes.

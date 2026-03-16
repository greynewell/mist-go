---
title: "CLI Reference"
description: "All infermux CLI commands, flags, environment variables, and exit codes."
---

# CLI Reference

The `infermux` binary exposes four commands: `serve`, `validate`, `status`, and `providers`.

## infermux serve

Start the inference router server.

```bash
infermux serve [flags]
```

**Flags:**

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--config` | `INFERMUX_CONFIG` | `infermux.yml` | Path to the config file |
| `--listen` | `INFERMUX_LISTEN` | `:8080` | Address and port to listen on |
| `--log-level` | `INFERMUX_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `--log-format` | `INFERMUX_LOG_FORMAT` | `text` | Log format: `text`, `json` |
| `--no-health-checks` | — | `false` | Disable startup health checks |
| `--graceful-shutdown` | — | `15s` | Timeout for graceful shutdown |

**Examples:**

```bash
# Start with the default config in the current directory
infermux serve

# Specify config and listen address
infermux serve --config /etc/infermux/infermux.yml --listen :9090

# Enable JSON structured logging (useful in production / when piping to a log collector)
infermux serve --log-format json

# Debug mode with verbose logging
infermux serve --log-level debug
```

**Startup behavior:**

1. Load and validate config file.
2. Resolve all environment variable references (`${VAR}` syntax).
3. Run health checks against all providers (unless `--no-health-checks` is set).
4. Start the HTTP listener.
5. Print a startup summary to stdout.

If config validation fails, `infermux serve` prints the error and exits with code 1 before any network activity.

**Graceful shutdown:**

On `SIGTERM` or `SIGINT`, infermux stops accepting new connections and waits up to `--graceful-shutdown` for in-flight requests to complete before exiting.

## infermux validate

Validate a config file without starting the server. No network connections are made. Useful in CI to catch config errors before deployment.

```bash
infermux validate [flags]
```

**Flags:**

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--config` | `INFERMUX_CONFIG` | `infermux.yml` | Path to the config file |
| `--strict` | — | `false` | Fail on unknown fields (default allows unknown fields for forward-compat) |

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Config is valid |
| 1 | Config file not found |
| 2 | Config file is malformed YAML |
| 3 | Config fails semantic validation (e.g., duplicate provider names, unknown strategy) |

**Examples:**

```bash
infermux validate --config infermux.yml
# ok: config is valid (3 providers, 2 route groups)

infermux validate --config infermux.yml --strict
# error: unknown field "retries" at providers[0] (did you mean "rate_limits"?)
```

## infermux status

Print the current health and circuit breaker state of all providers. Connects to a running infermux server via its management API.

```bash
infermux status [flags]
```

**Flags:**

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--server` | `INFERMUX_SERVER` | `http://localhost:8080` | Address of the infermux server |
| `--format` | — | `table` | Output format: `table`, `json` |

**Example output:**

```
PROVIDER     TYPE        STATE    ERROR RATE  P95 LATENCY  RPM
openai       openai      closed   1.2%        412ms        847
anthropic    anthropic   open*    68.4%       8200ms       103
ollama       ollama      closed   0.0%        112ms        24

* circuit open; recovery in 18s
```

With `--format json`:

```bash
infermux status --format json | jq '.[] | select(.circuit != "closed")'
```

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | All providers have closed circuits |
| 1 | One or more providers have open circuits |
| 2 | Could not connect to the infermux server |

The non-zero exit code when circuits are open makes `infermux status` useful in monitoring scripts.

## infermux providers

List configured providers and their current status. Similar to `status` but includes more metadata about provider configuration.

```bash
infermux providers [flags]
```

**Flags:**

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--server` | `INFERMUX_SERVER` | `http://localhost:8080` | Address of the infermux server |
| `--format` | — | `table` | Output format: `table`, `json`, `yaml` |

**Example output:**

```
PROVIDER     TYPE        MODELS                           HEALTHY  CIRCUIT  LAST CHECK
openai       openai      gpt-4o, gpt-4o-mini, ...        yes      closed   3s ago
anthropic    anthropic   gpt-4o→claude-opus (alias), ...  no       open     22s ago
ollama       ollama      llama3.2, mistral               yes      closed   8s ago
```

## Global flags

These flags apply to all commands:

| Flag | Description |
|------|-------------|
| `--version` | Print version information and exit |
| `--help` | Show help for the command |

## Environment variables

All flags that have an associated environment variable (shown in the tables above) can be set via environment. The flag value takes precedence over the environment variable if both are set.

Additional environment variables:

| Variable | Description |
|----------|-------------|
| `INFERMUX_LOG_TIMESTAMPS` | Set to `false` to omit timestamps from log lines (useful when the log collector adds them) |
| `INFERMUX_MANAGEMENT_TOKEN` | Bearer token required to access management API endpoints (`/_infermux/*`). If unset, management endpoints are unauthenticated. |

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (config not found, invalid args) |
| 2 | Config validation failed |
| 3 | Server failed to start (port in use, bind error) |
| 130 | Interrupted by signal (SIGINT) |

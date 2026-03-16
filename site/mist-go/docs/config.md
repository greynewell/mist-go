---
title: config
description: The config package — TOML loading, struct decoding, environment variable overlay with prefix, validation, and example configuration.
---

# config

Import path: `github.com/greynewell/mist-go/config`

The `config` package loads TOML configuration files into Go structs, then applies environment variable overrides. The TOML parser is implemented from scratch — no external dependencies. The env overlay uses a configurable prefix so multiple MIST tools can coexist in the same environment without variable name collisions.

## Loading configuration

```go
type Config struct {
    Addr       string `toml:"addr"`
    Workers    int    `toml:"workers"`
    InferURL   string `toml:"infer_url"`
    LogLevel   string `toml:"log_level"`
    Checkpoint string `toml:"checkpoint_dir"`
}

var cfg Config
if err := config.Load("matchspec.toml", "MATCHSPEC", &cfg); err != nil {
    log.Fatalf("config: %v", err)
}
```

`Load` takes three arguments:
1. Path to a TOML file
2. Environment variable prefix (e.g., `"MATCHSPEC"`)
3. Pointer to the struct to populate

If the file does not exist, `Load` returns an error. If you want a config file to be optional, handle `os.IsNotExist`:

```go
var cfg Config
err := config.Load("matchspec.toml", "MATCHSPEC", &cfg)
if err != nil && !os.IsNotExist(err) {
    log.Fatalf("config: %v", err)
}
```

## TOML file format

The TOML parser supports:
- `key = value` assignments with string, integer, float, bool, and array values
- `[section]` and `[section.sub]` table headers
- Single-line `#` comments
- Bare keys and double-quoted keys
- Single-quoted literal strings (no escape processing)
- Inline comments after values

```toml
# matchspec.toml
addr = ":8080"
workers = 8
infer_url = "http://localhost:8081"
log_level = "info"
checkpoint_dir = "/var/lib/matchspec/checkpoints"

[limits]
max_tasks = 1000
timeout_s = 300
```

Struct fields are matched by their `toml` tag. If no tag is present, the field name is lowercased for matching:

```go
type Config struct {
    Addr    string `toml:"addr"`
    Workers int    `toml:"workers"`

    Limits struct {
        MaxTasks int `toml:"max_tasks"`
        TimeoutS int `toml:"timeout_s"`
    } `toml:"limits"`
}
```

## Environment variable overlay

After reading the TOML file, `Load` applies environment variables that match `PREFIX_FIELDNAME`. The prefix and field name are both uppercased:

```
MATCHSPEC_ADDR         → cfg.Addr
MATCHSPEC_WORKERS      → cfg.Workers
MATCHSPEC_INFER_URL    → cfg.InferURL (field name match: INFER_URL → InferURL)
MATCHSPEC_LOG_LEVEL    → cfg.LogLevel
```

Environment variables override TOML values. This is the standard 12-factor app pattern: config file provides defaults, environment provides deployment-specific overrides.

Supported field types for env overlay: `string`, `int`, `int64`, `float64`, `bool`. Boolean env vars: `"true"` or `"1"` means true; anything else means false. Nested struct fields are not currently supported via env — use top-level fields for settings that need env override.

## Parsing TOML directly

If you need TOML parsing without the full `Load` pipeline:

```go
// Parse from an io.Reader.
data, err := config.ParseTOML(file)
// data is map[string]any with nested maps for tables

// Decode a map into a struct.
var cfg Config
if err := config.Decode(data, &cfg); err != nil {
    return err
}
```

`Decode` matches map keys to struct fields using `toml` tags (falling back to lowercased field names), and recursively decodes nested tables into struct fields.

## Example: full application config

A complete config struct for a MIST-based tool:

```go
type ServerConfig struct {
    Addr            string `toml:"addr"`
    ReadTimeoutS    int    `toml:"read_timeout_s"`
    WriteTimeoutS   int    `toml:"write_timeout_s"`
    ShutdownTimeoutS int   `toml:"shutdown_timeout_s"`
}

type InferConfig struct {
    URL             string  `toml:"url"`
    TimeoutS        int     `toml:"timeout_s"`
    MaxRetries      int     `toml:"max_retries"`
    CircuitThreshold int    `toml:"circuit_threshold"`
    CircuitTimeoutS  int    `toml:"circuit_timeout_s"`
}

type AppConfig struct {
    Server    ServerConfig `toml:"server"`
    Infer     InferConfig  `toml:"infer"`
    Workers   int          `toml:"workers"`
    LogLevel  string       `toml:"log_level"`
    LogFormat string       `toml:"log_format"`
    CheckpointDir string   `toml:"checkpoint_dir"`
}
```

Corresponding TOML:

```toml
workers = 8
log_level = "info"
log_format = "json"
checkpoint_dir = "/var/lib/matchspec/checkpoints"

[server]
addr = ":8080"
read_timeout_s = 10
write_timeout_s = 10
shutdown_timeout_s = 30

[infer]
url = "http://infermux:8081"
timeout_s = 60
max_retries = 3
circuit_threshold = 5
circuit_timeout_s = 30
```

Load with:

```go
var cfg AppConfig
if err := config.Load("matchspec.toml", "MATCHSPEC", &cfg); err != nil {
    log.Fatalf("config: %v", err)
}

// Override via environment:
// MATCHSPEC_WORKERS=16 overrides cfg.Workers
// MATCHSPEC_LOG_LEVEL=debug overrides cfg.LogLevel
// Note: nested structs (cfg.Server, cfg.Infer) are not overridable via env.
// Use top-level fields for env-configurable settings.
```

## Validation

`config` does not include built-in validation. Validate your struct after loading:

```go
func (c *AppConfig) Validate() error {
    if c.Server.Addr == "" {
        return fmt.Errorf("server.addr is required")
    }
    if c.Workers < 1 {
        return fmt.Errorf("workers must be >= 1, got %d", c.Workers)
    }
    if c.Infer.URL == "" {
        return fmt.Errorf("infer.url is required")
    }
    if _, err := url.Parse(c.Infer.URL); err != nil {
        return fmt.Errorf("infer.url is not a valid URL: %w", err)
    }
    return nil
}

var cfg AppConfig
if err := config.Load("matchspec.toml", "MATCHSPEC", &cfg); err != nil {
    log.Fatal(err)
}
if err := cfg.Validate(); err != nil {
    log.Fatalf("invalid config: %v", err)
}
```

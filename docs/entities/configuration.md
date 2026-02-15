---
title: Configuration
slug: configuration
order: 6
---

# Configuration

MIST tools use TOML configuration files with environment variable
overrides.

## Loading Config

```go
type Config struct {
    Port    int    `toml:"port"`
    Host    string `toml:"host"`
    Debug   bool   `toml:"debug"`
}

var cfg Config
err := config.Load("matchspec.toml", "MATCHSPEC", &cfg)
```

## Environment Overrides

Environment variables with the tool prefix override file values:

```bash
MATCHSPEC_PORT=9090 MATCHSPEC_DEBUG=true matchspec serve
```

For a prefix `MATCHSPEC` and field `Port`, `MATCHSPEC_PORT` takes
precedence over the value in the TOML file.

## Example TOML

```toml
# matchspec.toml
port = 8080
host = "0.0.0.0"
debug = false

[services]
infermux = "http://localhost:8081"
tokentrace = "http://localhost:8083"

[eval]
timeout = 30
parallel = 4
suites = ["math", "coding", "reasoning"]
```

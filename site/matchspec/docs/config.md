---
title: Configuration
description: Complete matchspec.yml schema — every field with type, default, and description, plus environment variable overrides.
---

# Configuration

matchspec is configured via a `matchspec.yml` file in your project root. This file defines suites, references harness files, sets thresholds, and configures output behavior.

## Config file discovery

`matchspec run` looks for `matchspec.yml` in the following order:

1. The path specified by `--config` flag
2. The path specified by the `MATCHSPEC_CONFIG` environment variable
3. `./matchspec.yml` (current working directory)
4. `$HOME/.matchspec/config.yml` (global user config)

The first file found is used. If no file is found, `matchspec run` exits with an error unless you pass a harness file directly as an argument.

## Full schema

```yaml
# matchspec.yml — complete example with all fields

version: 1  # required; must be 1

# Global defaults applied to all suites and harnesses unless overridden.
defaults:
  concurrency: 4          # parallel model calls per harness
  timeout_seconds: 30     # per-example model call timeout
  retries: 0              # number of retries on model call failure
  retry_delay_ms: 250     # delay between retries

# Output configuration.
output:
  dir: ".matchspec/results"   # where results files are written
  format: json                # "json", "junit", or "markdown"
  retain_days: 30             # delete results older than N days (0 = keep forever)

# Statistics configuration.
statistics:
  confidence_level: 0.95      # Wilson score CI confidence level
  use_lower_bound: false      # compare threshold to CI lower bound
  min_sample_size: 0          # minimum examples required before enforcing thresholds
  min_sample_action: warn     # "warn" or "fail"

# Suite definitions.
suites:
  - name: smoke                 # required; suite identifier
    description: "Fast smoke check on a small sample."
    harnesses:
      - ./evals/smoke/harness.yml
    tags: [smoke]               # only run examples with these tags
    thresholds:
      overall: 0.90             # default threshold for all graders in this suite
      exact_match: 0.85         # per-grader-name threshold (overrides overall)
      semantic_similarity: 0.80
    statistics:                 # per-suite statistics config (overrides global)
      min_sample_size: 10
      min_sample_action: warn

  - name: production-gate
    description: "Full eval suite required to pass before deployment."
    harnesses:
      - ./evals/summarization/harness.yml
      - ./evals/qa/harness.yml
      - ./evals/classification/harness.yml
    thresholds:
      overall: 0.80
    statistics:
      confidence_level: 0.95
      use_lower_bound: true
      min_sample_size: 50
      min_sample_action: fail
```

## Field reference

### Top-level

| Field | Type | Default | Description |
|---|---|---|---|
| `version` | integer | — | **Required.** Must be `1`. |
| `defaults` | object | see below | Global defaults for all harnesses. |
| `output` | object | see below | Output file configuration. |
| `statistics` | object | see below | Statistical settings. |
| `suites` | array | — | **Required.** List of suite definitions. |

### defaults

| Field | Type | Default | Description |
|---|---|---|---|
| `concurrency` | integer | `4` | Max parallel model calls per harness. |
| `timeout_seconds` | integer | `30` | Per-example timeout in seconds. |
| `retries` | integer | `0` | Retry count on model call failure. |
| `retry_delay_ms` | integer | `250` | Milliseconds between retries. |

### output

| Field | Type | Default | Description |
|---|---|---|---|
| `dir` | string | `.matchspec/results` | Directory for results files. Created if absent. |
| `format` | string | `json` | Results file format: `json`, `junit`, `markdown`. |
| `retain_days` | integer | `0` | Delete results files older than N days. 0 keeps all. |

### statistics

| Field | Type | Default | Description |
|---|---|---|---|
| `confidence_level` | float | `0.95` | Confidence level for Wilson score interval. |
| `use_lower_bound` | bool | `false` | Apply threshold to CI lower bound. |
| `min_sample_size` | integer | `0` | Minimum examples. 0 disables the check. |
| `min_sample_action` | string | `warn` | `"warn"` prints a warning. `"fail"` exits non-zero. |

### suites[]

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Suite identifier. Used in `--suite` flag and reports. |
| `description` | string | no | Human-readable description. |
| `harnesses` | array of strings | yes | Paths to harness YAML files. |
| `tags` | array of strings | no | Run only examples with these tags. |
| `thresholds` | object | no | Per-suite threshold overrides. |
| `statistics` | object | no | Per-suite statistics config. |

### suites[].thresholds

| Field | Type | Default | Description |
|---|---|---|---|
| `overall` | float | `1.0` | Default threshold for all graders not explicitly listed. |
| `<grader_name>` | float | — | Per-grader-name threshold override. |

## Environment variable overrides

Any config value can be overridden with an environment variable. The pattern is `MATCHSPEC_` followed by the dot-notation path, uppercased, with dots replaced by underscores:

| Variable | Overrides |
|---|---|
| `MATCHSPEC_CONFIG` | Config file path |
| `MATCHSPEC_OUTPUT_DIR` | `output.dir` |
| `MATCHSPEC_OUTPUT_FORMAT` | `output.format` |
| `MATCHSPEC_DEFAULTS_CONCURRENCY` | `defaults.concurrency` |
| `MATCHSPEC_DEFAULTS_TIMEOUT_SECONDS` | `defaults.timeout_seconds` |
| `MATCHSPEC_DEFAULTS_RETRIES` | `defaults.retries` |
| `MATCHSPEC_STATISTICS_CONFIDENCE_LEVEL` | `statistics.confidence_level` |
| `MATCHSPEC_STATISTICS_USE_LOWER_BOUND` | `statistics.use_lower_bound` |
| `MATCHSPEC_API_KEY` | HTTP API authentication key |
| `MATCHSPEC_LOG_LEVEL` | Log verbosity: `debug`, `info`, `warn`, `error` |

Environment variables take precedence over values in `matchspec.yml`. Command-line flags take precedence over environment variables.

## Minimal config

The smallest valid `matchspec.yml`:

```yaml
version: 1
suites:
  - name: default
    harnesses:
      - ./evals/harness.yml
    thresholds:
      overall: 0.80
```

## Example: multiple environments

Use environment variables to vary thresholds between environments without multiple config files:

```yaml
version: 1
suites:
  - name: production-gate
    harnesses:
      - ./evals/summarization/harness.yml
    thresholds:
      overall: 0.85
```

In CI before merging to `main`:

```bash
MATCHSPEC_STATISTICS_MIN_SAMPLE_SIZE=100 \
MATCHSPEC_STATISTICS_USE_LOWER_BOUND=true \
matchspec run --suite production-gate
```

In a fast smoke check on a PR:

```bash
matchspec run --suite smoke --tags smoke
```

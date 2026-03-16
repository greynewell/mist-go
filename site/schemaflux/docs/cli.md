---
title: "CLI Reference"
description: "All schemaflux commands: build, watch, validate, and graph. Flags, environment variables, and exit codes."
---

# CLI Reference

schemaflux provides four commands: `build`, `watch`, `validate`, and `graph`.

```
Usage: schemaflux <command> [flags]

Commands:
  build     Compile content and emit output
  watch     Compile and watch for changes
  validate  Validate content without emitting output
  graph     Print the compiled IR as JSON
  version   Print version information
```

---

## schemaflux build

Compiles all source files through the 12-pass pipeline and writes output to the configured backends.

```bash
schemaflux build [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|---|---|---|---|
| `--config` | string | `./schemaflux.yml` | Path to config file |
| `--output` | string | from config | Override output directory |
| `--drafts` | bool | `false` | Include entities with `draft: true` |
| `--clean` | bool | `false` | Delete output directory before building |
| `--verbose` | bool | `false` | Print per-pass timing and entity counts |
| `--quiet` | bool | `false` | Suppress all output except errors |
| `--no-validate` | bool | `false` | Skip the validate pass (use with caution) |
| `--only` | string | `""` | Run only the specified backend (e.g. `--only html`) |

**Examples:**

```bash
# Standard build
schemaflux build

# Use a custom config file
schemaflux build --config ./config/production.yml

# Build including drafts
schemaflux build --drafts

# Clean build with verbose output
schemaflux build --clean --verbose

# Build only the HTML backend, skip JSON and sitemap
schemaflux build --only html

# Override output directory
schemaflux build --output ./_dist
```

**Exit codes:**

| Code | Meaning |
|---|---|
| 0 | Build succeeded |
| 1 | Build failed (see stderr for diagnostics) |
| 2 | Configuration error |
| 3 | Validation error (pass 9 failures) |

**Verbose output example:**

```
parsing    342 files            8ms
pass 1/12  slugify              2ms   342 entities
pass 2/12  sort                 1ms   342 entities
pass 3/12  enrich               45ms  342 entities
pass 4/12  taxonomy             3ms   342 entities  →  28 taxonomy entities
pass 5/12  relationships        62ms  370 entities
pass 6/12  graph                8ms   370 entities
pass 7/12  url-resolve          4ms   370 entities
pass 8/12  schema-gen           6ms   370 entities  →  4 schemas
pass 9/12  validate             3ms   370 entities  →  0 violations
pass 10/12 emit-html            180ms 412 files
pass 11/12 emit-json            12ms  1 file
pass 12/12 emit-sitemap         2ms   2 files

total      412 files in ./_site/
built in   336ms
```

---

## schemaflux watch

Starts a build, then watches the content and templates directories for changes. Re-runs the full pipeline on any change.

```bash
schemaflux watch [flags]
```

**Flags:**

All flags from `schemaflux build`, plus:

| Flag | Type | Default | Description |
|---|---|---|---|
| `--serve` | bool | `false` | Serve output directory over HTTP |
| `--port` | int | `3000` | Port for the local HTTP server |
| `--host` | string | `127.0.0.1` | Host for the local HTTP server |
| `--open` | bool | `false` | Open browser on start |

**Examples:**

```bash
# Watch and rebuild on changes
schemaflux watch

# Watch, serve, and open browser
schemaflux watch --serve --open

# Watch with a custom port
schemaflux watch --serve --port 8080
```

When `--serve` is enabled, the local HTTP server serves the output directory with basic MIME type handling. It does not provide live reload; rebuild output is logged to the terminal.

---

## schemaflux validate

Runs passes 0 through 9 (parse through validate) without emitting any output. Exits 0 if validation passes, non-zero if any violations are found.

```bash
schemaflux validate [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|---|---|---|---|
| `--config` | string | `./schemaflux.yml` | Path to config file |
| `--drafts` | bool | `false` | Include draft entities in validation |
| `--strict` | bool | `false` | Treat warnings as errors |
| `--format` | string | `text` | Output format: `text` or `json` |

**Examples:**

```bash
# Validate content
schemaflux validate

# Validate including drafts
schemaflux validate --drafts

# Output violations as JSON (useful in CI)
schemaflux validate --format json

# Strict mode: warnings become errors
schemaflux validate --strict
```

**JSON output format:**

```json
{
  "valid": false,
  "violations": [
    {
      "entity": "blog/hello-world",
      "field": "description",
      "rule": "require-description-in-blog",
      "message": "field 'description' is required in section 'blog'"
    }
  ],
  "warnings": []
}
```

---

## schemaflux graph

Runs passes 0 through 7 (parse through url-resolve) and prints the compiled IR as JSON to stdout. Useful for debugging the pipeline, inspecting computed fields, and extracting data without building the full output.

```bash
schemaflux graph [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|---|---|---|---|
| `--config` | string | `./schemaflux.yml` | Path to config file |
| `--drafts` | bool | `false` | Include draft entities |
| `--filter` | string | `""` | Filter entities by section (e.g. `--filter blog`) |
| `--entity` | string | `""` | Print a single entity by ID |
| `--fields` | string | `""` | Comma-separated list of fields to include |
| `--pretty` | bool | `true` | Pretty-print JSON output |

**Examples:**

```bash
# Print full graph
schemaflux graph

# Print only blog entities
schemaflux graph --filter blog

# Print a single entity
schemaflux graph --entity blog/hello-world

# Print only id, title, url, related for all entities
schemaflux graph --fields id,title,url,related

# Pipe to jq for processing
schemaflux graph --no-pretty | jq '.entities[] | select(.section == "blog") | .url'
```

---

## schemaflux version

Prints version, Go runtime, and build information.

```bash
schemaflux version
# schemaflux v0.9.0 (go1.22.3/darwin/arm64)
# built 2026-03-15T00:00:00Z
# commit abc1234
```

---

## Environment variables

All config file values can be overridden with environment variables. The naming convention is `SCHEMAFLUX_` prefix followed by the uppercase dotted path with dots replaced by underscores:

| Variable | Overrides |
|---|---|
| `SCHEMAFLUX_SITE_URL` | `site.url` |
| `SCHEMAFLUX_SITE_TITLE` | `site.title` |
| `SCHEMAFLUX_INPUT_DIR` | `input.dir` |
| `SCHEMAFLUX_INPUT_DRAFTS` | `input.drafts` |
| `SCHEMAFLUX_OUTPUT_DIR` | `output.dir` |
| `SCHEMAFLUX_TEMPLATES_DIR` | `templates.dir` |

**Example:**

```bash
SCHEMAFLUX_SITE_URL=https://staging.example.com \
SCHEMAFLUX_OUTPUT_DIR=./_staging \
  schemaflux build
```

---

## Using schemaflux in CI

```yaml
# .github/workflows/build.yml
- name: Install schemaflux
  run: go install github.com/greynewell/schemaflux/cmd/schemaflux@latest

- name: Validate content
  run: schemaflux validate --format json

- name: Build site
  run: schemaflux build --clean
  env:
    SCHEMAFLUX_SITE_URL: https://example.com
```

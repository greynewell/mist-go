---
title: "Configuration"
description: "Complete schemaflux.yml schema. Every field, its type, default value, and what it controls."
---

# Configuration

schemaflux is configured by a YAML file, conventionally named `schemaflux.yml`, in the root of your project. Pass a different path with `--config path/to/config.yml`.

A complete annotated configuration file:

```yaml
# schemaflux.yml

# ─── Site metadata ────────────────────────────────────────────────────────────
site:
  title: "My Site"              # string; used in templates as .Site.Title
  url: https://example.com      # string; base URL for canonical links and sitemap
  description: "A site."        # string; available as .Site.Description in templates
  author: "Your Name"           # string; available as .Site.Author

# ─── Input ────────────────────────────────────────────────────────────────────
input:
  dir: ./content                # string; required; root directory to walk for source files
  include:                      # list of glob patterns; default: ["**/*.md"]
    - "**/*.md"
  exclude:                      # list of glob patterns to skip
    - "_drafts/**"
    - "README.md"
  drafts: false                 # bool; include entities with draft: true

# ─── Output ───────────────────────────────────────────────────────────────────
output:
  dir: ./_site                  # string; directory for all emitted files

# ─── Templates ────────────────────────────────────────────────────────────────
templates:
  dir: ./templates              # string; directory containing .html template files

# ─── Pass configuration ───────────────────────────────────────────────────────
passes:
  slugify:
    strategy: default           # "default" | "preserve" | "ascii"
    separator: "-"              # word separator character

  sort:
    primary: weight             # "weight" | "date" | "title"
    secondary: date             # "date" | "title" | "weight"
    direction: asc              # "asc" | "desc"

  enrich:
    markdown:
      extensions:
        - tables
        - strikethrough
        - autolink
        - taskList
      syntaxHighlight: true     # bool; wrap fenced code blocks with highlight spans
      safeHTML: false           # bool; strip raw HTML from markdown bodies

  taxonomy:
    tags:
      enabled: true
      urlPrefix: /tags/         # URL prefix for tag pages
      layout: tag-list          # template name for tag pages
      minCount: 1               # minimum occurrences to generate a page
    categories:
      enabled: false
      urlPrefix: /categories/
      layout: category-list
      minCount: 1
    series:
      enabled: false
      urlPrefix: /series/
      layout: series-list
      minCount: 1

  relationships:
    topN: 5                     # int; number of related entities to store per entity
    minScore: 0.1               # float; minimum similarity score to include
    weights:
      tagOverlap: 0.5           # float; weight for Jaccard tag similarity
      sectionMatch: 0.2         # float; weight for same-section bonus
      linkOverlap: 0.2          # float; weight for shared outbound links
      titleTokenOverlap: 0.1    # float; weight for title word Jaccard similarity

  graph:
    backlinks: true             # bool; build backlink index
    parentChild: true           # bool; derive parent/child from URL prefixes
    prevNext: true              # bool; set prev/next within scope
    prevNextScope: section      # "section" | "global" | "taxonomy"

  urlResolve:
    strategy: clean             # "clean" | "pretty" | "flat"
    trailingSlash: true         # bool; append trailing slash to clean URLs

  schemaGen:
    enabled: true               # bool; infer JSON Schema per section
    emit: false                 # bool; write schema files to emitDir
    emitDir: ./_schemas/        # string; output directory for schema files
    requiredThreshold: 1.0      # float; fraction (0.0–1.0) of entities that must have
                                #   a field for it to be marked required

  validate:
    schema: true                # bool; validate entities against inferred schema
    uniqueSlugs: true           # bool; fail on duplicate slugs
    uniqueURLs: true            # bool; fail on duplicate resolved URLs
    internalLinks: true         # bool; fail on broken internal links
    rules: []                   # list of custom validation rules (see below)

# ─── Backend configuration ────────────────────────────────────────────────────
backends:
  html:
    enabled: true
    outputDir: ./_site          # string; can differ from global output.dir
    templates:
      dir: ./templates          # string; template directory (overrides global)
      default: base             # string; layout name for entities without a layout
      extension: .html          # string; template file extension
    pagination:
      pageSize: 10              # int; entities per page (0 = no pagination)
      sections: []              # list of section names to paginate (empty = all)
      pageLayout: list          # string; template for paginated list pages

  json:
    enabled: false
    perEntity: false            # bool; write one JSON file per entity
    combined: true              # bool; write a single combined JSON file
    combinedFile: entities.json # string; filename for combined output
    fields: []                  # list of field names to include (empty = all)
    exclude:                    # list of field names to exclude
      - rawContent
      - schema

  rss:
    enabled: false
    section: blog               # string; section to include in feed
    file: feed.xml              # string; output filename
    title: ""                   # string; feed title (defaults to site.title)
    description: ""             # string; feed description
    link: ""                    # string; feed link (defaults to site.url)
    limit: 20                   # int; maximum items in feed

  sitemap:
    enabled: true
    file: sitemap.xml           # string; output filename
    changefreq: weekly          # "always"|"hourly"|"daily"|"weekly"|"monthly"|"yearly"|"never"
    priority: 0.5               # float; default priority for all URLs
    exclude: []                 # list of URL glob patterns to exclude from sitemap

  search:
    enabled: false
    file: search.json           # string; output filename
    fields:                     # list of entity fields to include in index
      - title
      - description
      - content
      - tags
      - url
```

## Field reference

### site

| Field | Type | Default | Description |
|---|---|---|---|
| `title` | string | `""` | Site title; available as `.Site.Title` in templates |
| `url` | string | `""` | Canonical base URL; used for sitemap and RSS |
| `description` | string | `""` | Site description; available as `.Site.Description` |
| `author` | string | `""` | Author name; available as `.Site.Author` |

### input

| Field | Type | Default | Description |
|---|---|---|---|
| `dir` | string | required | Root directory to walk for source files |
| `include` | list | `["**/*.md"]` | Glob patterns for files to include |
| `exclude` | list | `[]` | Glob patterns for files to exclude |
| `drafts` | bool | `false` | Process entities with `draft: true` |

### output

| Field | Type | Default | Description |
|---|---|---|---|
| `dir` | string | `./_site` | Output directory for emitted files |

### templates

| Field | Type | Default | Description |
|---|---|---|---|
| `dir` | string | `./templates` | Directory containing `.html` template files |

## Custom validation rules

```yaml
passes:
  validate:
    rules:
      - name: "require-description-in-blog"
        # Apply rule only to entities in the blog section
        section: blog
        # All listed fields must be present and non-empty
        require:
          - description
          - date

      - name: "require-input-in-evals"
        section: evals
        require:
          - input
          - expected_output
          - difficulty
```

## Environment variable overrides

Any config value can be overridden with an environment variable using the `SCHEMAFLUX_` prefix and uppercase dot-separated path:

```bash
SCHEMAFLUX_SITE_URL=https://staging.example.com schemaflux build
SCHEMAFLUX_INPUT_DRAFTS=true schemaflux build
SCHEMAFLUX_OUTPUT_DIR=./_staging schemaflux build
```

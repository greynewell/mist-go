---
title: "Output Backends"
description: "HTML, JSON, RSS, sitemap, search index, and custom backends. Template variables, output schemas, feed configuration, and the Go Backend interface."
---

# Output Backends

A backend is a pass that reads the compiled IR and writes output files. schemaflux ships five built-in backends. Custom backends can be written in Go by implementing the `Backend` interface.

Backends are configured under `backends:` in `schemaflux.yml`. Multiple backends run in sequence after the validation pass completes.

---

## HTML backend

The HTML backend renders every entity to an HTML file using Go templates. It is the primary backend for static site generation.

### Template selection

Each entity's `Layout` field determines which template file to use. Template files are looked up in the templates directory configured at `templates.dir`. A layout value of `post` resolves to `{templates.dir}/post.html`. If an entity has no `Layout`, the `backends.html.templates.default` value is used.

### Template variables

Every template receives the current entity as `.` (dot). The full set of available fields:

| Variable | Type | Description |
|---|---|---|
| `.ID` | `string` | Stable entity identifier |
| `.Slug` | `string` | URL-safe slug |
| `.URL` | `string` | Canonical URL |
| `.Title` | `string` | Entity title |
| `.Description` | `string` | Short description |
| `.Content` | `template.HTML` | Rendered HTML body |
| `.Date` | `time.Time` | Publication date |
| `.UpdatedAt` | `time.Time` | Last updated date |
| `.Weight` | `int` | Sort weight |
| `.Draft` | `bool` | Draft status |
| `.Tags` | `[]Tag` | Tag list; each Tag has `.Name` and `.Slug` |
| `.Categories` | `[]Category` | Category list |
| `.Series` | `string` | Series name |
| `.Related` | `[]*Entity` | Top-N related entities |
| `.Backlinks` | `[]*Entity` | Entities linking to this one |
| `.Parent` | `*Entity` | URL parent entity (may be nil) |
| `.Children` | `[]*Entity` | URL child entities |
| `.Prev` | `*Entity` | Previous entity in section order |
| `.Next` | `*Entity` | Next entity in section order |
| `.Frontmatter` | `map[string]any` | All frontmatter including custom fields |
| `.Section` | `string` | First path component |
| `.Depth` | `int` | URL depth |
| `.Graph` | `*Graph` | The full entity graph |
| `.Site` | `*SiteConfig` | Site-level config (title, URL) |

### Graph variables

`.Graph` exposes the full compiled IR:

| Variable | Type | Description |
|---|---|---|
| `.Graph.Entities` | `[]*Entity` | All non-draft entities |
| `.Graph.BySection` | `map[string][]*Entity` | Entities grouped by section |
| `.Graph.ByTag` | `map[string][]*Entity` | Entities grouped by tag slug |
| `.Graph.Nav` | `[]*Entity` | Top-level navigation entities (weight-sorted, section roots) |
| `.Graph.Tags` | `[]*Entity` | All taxonomy tag entities |
| `.Graph.Sections` | `[]string` | All section names |

### Template example

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>{{ .Title }} — {{ .Site.Title }}</title>
  {{ if .Description }}<meta name="description" content="{{ .Description }}">{{ end }}
  <link rel="canonical" href="{{ .Site.URL }}{{ .URL }}">
</head>
<body>
  <nav>
    {{ range .Graph.Nav }}
    <a href="{{ .URL }}"{{ if eq .URL $.URL }} aria-current="page"{{ end }}>{{ .Title }}</a>
    {{ end }}
  </nav>

  <main>
    <article>
      <h1>{{ .Title }}</h1>
      {{ if not .Date.IsZero }}
      <time datetime="{{ .Date.Format "2006-01-02" }}">{{ .Date.Format "January 2, 2006" }}</time>
      {{ end }}

      {{ if .Tags }}
      <ul>
        {{ range .Tags }}<li><a href="/tags/{{ .Slug }}/">{{ .Name }}</a></li>{{ end }}
      </ul>
      {{ end }}

      {{ .Content }}
    </article>

    {{ if .Related }}
    <aside>
      <h2>Related</h2>
      {{ range .Related }}
      <a href="{{ .URL }}">{{ .Title }}</a>
      {{ end }}
    </aside>
    {{ end }}

    <nav aria-label="Next and previous">
      {{ if .Prev }}<a href="{{ .Prev.URL }}">← {{ .Prev.Title }}</a>{{ end }}
      {{ if .Next }}<a href="{{ .Next.URL }}">{{ .Next.Title }} →</a>{{ end }}
    </nav>
  </main>
</body>
</html>
```

### Taxonomy templates

Taxonomy entities (tag pages, category pages) have the same variables as regular entities, with `Children` populated with all entities bearing that taxonomy term:

```html
<!-- templates/tag-list.html -->
<h1>Posts tagged "{{ .Title }}"</h1>
{{ range .Children }}
<article>
  <a href="{{ .URL }}">{{ .Title }}</a>
  <time>{{ .Date.Format "2006-01-02" }}</time>
</article>
{{ end }}
```

### Pagination

When `pagination.pageSize` is set for a section, schemaflux generates multiple list pages with paginated entity sets:

```yaml
backends:
  html:
    pagination:
      pageSize: 10
      sections:
        - blog
      pageLayout: list
```

Paginated templates receive additional variables:

| Variable | Type | Description |
|---|---|---|
| `.Pagination.Page` | `int` | Current page number (1-based) |
| `.Pagination.TotalPages` | `int` | Total number of pages |
| `.Pagination.Prev` | `string` | URL of previous page (empty on first page) |
| `.Pagination.Next` | `string` | URL of next page (empty on last page) |
| `.Pagination.Items` | `[]*Entity` | Entities on this page |

### Template inheritance

Go templates support partial inclusion via `{{ template "name" . }}`. Define partials in separate files and include them:

```html
<!-- templates/partials/head.html -->
{{ define "head" }}
<head>
  <meta charset="UTF-8">
  <title>{{ .Title }} — {{ .Site.Title }}</title>
</head>
{{ end }}

<!-- templates/base.html -->
{{ template "head" . }}
<body>{{ .Content }}</body>
```

---

## JSON backend

The JSON backend serializes entities to JSON. Use it to produce machine-readable datasets for downstream tools, including matchspec.

### Output schema

Each serialized entity:

```json
{
  "id": "blog/hello-world",
  "slug": "hello-world",
  "url": "/blog/hello-world/",
  "title": "Hello, World",
  "description": "The first post on this site.",
  "content": "<p>This is the first post...</p>",
  "date": "2026-03-15T00:00:00Z",
  "updated_at": null,
  "tags": [
    { "name": "announcements", "slug": "announcements" }
  ],
  "section": "blog",
  "related": [
    { "id": "blog/second-post", "url": "/blog/second-post/", "title": "Second Post" }
  ],
  "backlinks": [],
  "frontmatter": {
    "title": "Hello, World",
    "date": "2026-03-15",
    "tags": ["announcements"],
    "custom_field": "custom value"
  }
}
```

### Combined output

With `combined: true`, schemaflux writes a single `entities.json`:

```json
{
  "generated_at": "2026-03-15T10:00:00Z",
  "count": 42,
  "entities": [ ... ]
}
```

### Field filtering

Use `fields:` to include only the fields you need:

```yaml
backends:
  json:
    enabled: true
    combined: true
    combinedFile: dataset.json
    fields:
      - id
      - title
      - frontmatter
      - content
```

---

## RSS backend

Produces a valid RSS 2.0 feed for a specified section.

```yaml
backends:
  rss:
    enabled: true
    section: blog
    file: feed.xml
    title: "My Site"
    description: "Recent posts from my site"
    link: https://example.com
    limit: 20
```

The feed includes: title, link, description, pubDate, and CDATA-wrapped content for each item.

---

## Sitemap backend

Writes an XML sitemap conforming to the sitemap protocol (sitemaps.org).

```yaml
backends:
  sitemap:
    enabled: true
    file: sitemap.xml
    changefreq: weekly
    priority: 0.5
    exclude:
      - /tags/**
      - /search/
```

Entities with `draft: true` are always excluded from the sitemap.

---

## Search index backend

Writes a JSON search index suitable for use with client-side search libraries (e.g. Fuse.js, Pagefind).

```yaml
backends:
  search:
    enabled: false
    file: search.json
    fields:
      - title
      - description
      - content
      - tags
      - url
```

Output format:

```json
[
  {
    "title": "Hello, World",
    "description": "The first post.",
    "content": "This is the first post...",
    "tags": ["announcements"],
    "url": "/blog/hello-world/"
  }
]
```

---

## Custom backend

Implement the `Backend` interface to emit any output format:

```go
// github.com/greynewell/schemaflux/backend
type Backend interface {
    Name() string
    Emit(ctx context.Context, graph *ir.Graph, cfg BackendConfig) error
}
```

A minimal custom backend:

```go
package mybackend

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"

    "github.com/greynewell/schemaflux/backend"
    "github.com/greynewell/schemaflux/ir"
)

type JSONLBackend struct{}

func (b *JSONLBackend) Name() string { return "jsonl" }

func (b *JSONLBackend) Emit(ctx context.Context, graph *ir.Graph, cfg backend.BackendConfig) error {
    outDir := cfg.StringOr("outputDir", "./_site")

    // Group entities by tag
    for tag, entities := range graph.ByTag {
        path := filepath.Join(outDir, "tags", tag+".jsonl")
        if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
            return err
        }
        f, err := os.Create(path)
        if err != nil {
            return err
        }
        enc := json.NewEncoder(f)
        for _, e := range entities {
            if err := enc.Encode(e); err != nil {
                f.Close()
                return err
            }
        }
        f.Close()
    }
    return nil
}
```

Register and use the backend in a custom `main.go`:

```go
package main

import (
    "github.com/greynewell/schemaflux/compiler"
    "github.com/greynewell/schemaflux/backend"
    "myproject/mybackend"
)

func main() {
    c := compiler.New()
    c.RegisterBackend(&mybackend.JSONLBackend{})
    if err := c.Build("schemaflux.yml"); err != nil {
        log.Fatal(err)
    }
}
```

See the [Writing a Custom Backend tutorial](/schemaflux/tutorials/custom-backend/) for a complete walkthrough.

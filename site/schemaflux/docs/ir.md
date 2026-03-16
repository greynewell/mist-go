---
title: "Intermediate Representation"
description: "The schemaflux intermediate representation: entity fields, how passes modify them, and why the IR enables cross-document features that direct template rendering cannot."
---

# Intermediate Representation

The intermediate representation (IR) is the in-memory graph of typed entities that schemaflux builds from your source files and progressively enriches during the 12-pass pipeline. Every pass reads from the IR and writes back to it. By the time the emit passes run, the IR is complete — every entity has its final slug, URL, tags, related entities, backlinks, and validated fields.

## Entity structure

Every source file becomes one entity in the IR. The entity type in Go is:

```go
type Entity struct {
    // Identity
    ID       string            // stable identifier: relative path without extension
    Slug     string            // URL-safe slug (computed in pass 1)
    URL      string            // absolute path URL (computed in pass 7)

    // Content
    Title       string
    Description string
    Content     string         // rendered HTML of the markdown body
    RawContent  string         // original markdown source

    // Metadata
    Date        time.Time
    UpdatedAt   time.Time
    Weight      int            // sort order; lower = earlier
    Draft       bool
    Layout      string         // template name (without extension)
    Permalink   string         // overrides computed URL when set

    // Taxonomy
    Tags        []Tag
    Categories  []Category
    Series      string

    // Relationships (computed)
    Related     []*Entity      // top-N related entities by score
    Backlinks   []*Entity      // entities that link to this one
    Children    []*Entity      // entities whose URL is a child of this one
    Parent      *Entity        // nearest URL ancestor

    // Schema
    Schema      map[string]any // inferred JSON Schema for this entity type
    Frontmatter map[string]any // all parsed frontmatter, including custom fields

    // Graph position
    Section     string         // first path component after input root
    Depth       int            // URL depth
    Prev        *Entity        // previous entity in sort order within section
    Next        *Entity        // next entity in sort order within section
}
```

## Fields populated at parse time

After pass 0 (parse), every entity has:

| Field | Source |
|---|---|
| `ID` | relative file path without extension |
| `RawContent` | raw markdown body |
| `Title` | `title` frontmatter, or first H1 in body if absent |
| `Description` | `description` frontmatter |
| `Date` | `date` frontmatter |
| `UpdatedAt` | `updated` frontmatter |
| `Weight` | `weight` frontmatter (default: 0) |
| `Draft` | `draft` frontmatter (default: false) |
| `Layout` | `layout` frontmatter |
| `Permalink` | `permalink` frontmatter |
| `Tags` | `tags` frontmatter list |
| `Categories` | `categories` frontmatter list |
| `Series` | `series` frontmatter |
| `Frontmatter` | all frontmatter key-value pairs |

## Fields populated by passes

Each subsequent pass writes additional fields:

| Pass | Fields written |
|---|---|
| 1 — slugify | `Slug` |
| 2 — sort | `Weight` (normalized), ordering in the entity list |
| 3 — enrich | `Content` (rendered HTML), `Section`, `Depth` |
| 4 — taxonomy | `Tags[].Slug`, `Categories[].Slug`, taxonomy entity stubs |
| 5 — relationships | `Related`, relationship scores |
| 6 — graph | `Backlinks`, `Children`, `Parent`, `Prev`, `Next` |
| 7 — url-resolve | `URL`, all backlink and related entity URLs |
| 8 — schema-gen | `Schema` (inferred per entity type) |
| 9 — validate | no new fields; fails build on violations |
| 10–12 — emit | no IR writes; reads IR to produce output files |

## How to think about the IR

Think of the IR as a database that starts empty and gets populated by each pass. Pass 0 does the initial load from disk. Each subsequent pass is a transformation: it reads whatever fields it needs, computes new data, and writes it back. Passes run in a fixed order because later passes depend on fields written by earlier ones — the relationship-scoring pass (5) needs slugs from pass 1 to identify link targets, and the graph-enrichment pass (6) needs relationship scores from pass 5 to build the backlink index.

The key insight is that when pass 10 (emit-html) runs, every entity already has its full URL, its related entities, its backlinks, its parent and children, and its prev/next navigation — computed from the complete corpus. No template needs to fetch this data at render time. The template just reads the entity struct.

## Why the IR enables cross-document features

Consider the `Backlinks` field. To know which entities link to a given entity, you must have parsed all entities first. In a file-by-file SSG, when you are rendering entity A, entity B may not have been processed yet, so you cannot know that B links to A. In schemaflux, by the time any template runs, all entities are in the IR, all links have been resolved, and the backlink index is complete.

The same logic applies to taxonomy pages. A tag page for "announcements" needs to know every entity tagged "announcements." In schemaflux, that list is built in pass 4 and is available to the HTML backend as a synthetic entity whose `Children` field contains all matching entities. No source file needs to exist for that page; schemaflux creates the entity in the IR and emits it like any other.

Related entity scoring works similarly. The relationship-scoring pass reads the full entity corpus and computes a similarity score for every entity pair based on shared tags, shared section, link overlap, and title token overlap. The top-N results per entity are stored in `Related`. The scoring algorithm and the value of N are configurable in `schemaflux.yml`.

## Accessing the IR from templates

In Go templates, the current entity is available as `.` (dot). The full graph is available as `.Graph`:

```html
<!-- Current entity fields -->
<h1>{{ .Title }}</h1>
<p>{{ .Date.Format "2006-01-02" }}</p>

<!-- Backlinks (entities that link to this one) -->
{{ if .Backlinks }}
<aside>
  <h2>Referenced by</h2>
  {{ range .Backlinks }}
  <a href="{{ .URL }}">{{ .Title }}</a>
  {{ end }}
</aside>
{{ end }}

<!-- Related entities (computed by pass 5) -->
{{ range .Related }}
<a href="{{ .URL }}">{{ .Title }}</a>
{{ end }}

<!-- Navigation: prev/next within section -->
{{ if .Prev }}<a href="{{ .Prev.URL }}">← {{ .Prev.Title }}</a>{{ end }}
{{ if .Next }}<a href="{{ .Next.URL }}">{{ .Next.Title }} →</a>{{ end }}
```

See [Templates](/schemaflux/docs/templates/) for the complete variable reference.

## Custom fields

Any frontmatter key not recognized by schemaflux is passed through to `Frontmatter` without modification. In templates, access custom fields via:

```html
{{ index .Frontmatter "author" }}
{{ index .Frontmatter "difficulty" }}
{{ index .Frontmatter "expected_output" }}
```

The schema-gen pass (8) infers JSON Schema for custom fields based on the values it observes across all entities in a section. If a custom field is present on every entity in a section, it is marked required in the inferred schema. If it is present on some but not all, it is optional. The validate pass uses this schema to flag entities that are missing fields that other entities in their section provide.

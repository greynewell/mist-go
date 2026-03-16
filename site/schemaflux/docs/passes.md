---
title: "The 12 Passes"
description: "Every schemaflux compiler pass explained: what it reads from the IR, what it writes, and how to configure it."
---

# The 12 Passes

schemaflux processes all entities through 12 sequential passes before emitting any output. Each pass reads from the shared IR, writes to it, and returns control to the scheduler. Passes run in a fixed order because later passes depend on fields written by earlier ones.

The full pipeline:

```
0  parse          read source files → entity stubs
1  slugify        entity IDs → URL-safe slugs
2  sort           apply weight and date ordering
3  enrich         render markdown, derive section/depth
4  taxonomy       group entities by tags/categories
5  relationships  score entity similarity
6  graph          build backlinks, parent/child, prev/next
7  url-resolve    compute and resolve all URLs
8  schema-gen     infer JSON Schema per entity type
9  validate       enforce schema, check invariants
10 emit-html      render HTML pages
11 emit-json      serialize IR to JSON
12 emit-sitemap   write sitemap.xml and RSS feed
```

---

## Pass 0 — parse

**Reads:** source files on disk
**Writes:** `ID`, `RawContent`, `Title`, `Description`, `Date`, `UpdatedAt`, `Weight`, `Draft`, `Layout`, `Permalink`, `Tags`, `Categories`, `Series`, `Frontmatter`

The parse pass walks the input directory recursively, reads every `.md` file, splits the YAML frontmatter from the markdown body, and creates one `Entity` in the IR per file. Files with `draft: true` in their frontmatter are skipped unless the `--drafts` flag is passed to `schemaflux build`.

**Configuration:**

```yaml
input:
  dir: ./content        # required; root directory to walk
  include:              # glob patterns to include (default: **/*.md)
    - "**/*.md"
  exclude:              # glob patterns to exclude
    - "_drafts/**"
    - "README.md"
  drafts: false         # include draft entities
```

---

## Pass 1 — slugify

**Reads:** `ID`, `Permalink`, frontmatter `slug`
**Writes:** `Slug`

Converts the entity's ID (relative file path without extension) into a URL-safe slug. The default strategy lowercases the path, replaces spaces and underscores with hyphens, and strips non-alphanumeric characters except hyphens and slashes. If a `slug` key is present in frontmatter, it overrides the computed value. If `permalink` is set, the slug is derived from the permalink's final path segment.

**Configuration:**

```yaml
passes:
  slugify:
    strategy: default   # "default" | "preserve" | "ascii"
    separator: "-"      # character to use for word separation
```

**Strategies:**

| Strategy | Behavior |
|---|---|
| `default` | lowercase, spaces → hyphens, strip special chars |
| `preserve` | lowercase only; preserves underscores and dots |
| `ascii` | transliterate non-ASCII characters before slugifying |

---

## Pass 2 — sort

**Reads:** `Weight`, `Date`, entity list order
**Writes:** normalized `Weight`, entity list order

Sorts the entity list by the combination of `weight` (ascending) and `date` (descending within the same weight group). Entities with `weight: 0` are sorted by date. Entities with explicit weight values are placed before zero-weight entities. The sort is stable: entities with equal weight and date retain their original file system order.

**Configuration:**

```yaml
passes:
  sort:
    primary: weight       # "weight" | "date" | "title"
    secondary: date       # "date" | "title" | "weight"
    direction: asc        # "asc" | "desc"
```

---

## Pass 3 — enrich

**Reads:** `RawContent`, `ID`
**Writes:** `Content` (rendered HTML), `Section`, `Depth`, inferred `Title` (if missing)

Renders the markdown body to HTML using the built-in CommonMark renderer. If an entity's `Title` field is empty after parsing, this pass extracts the first H1 heading from the markdown body and uses it. `Section` is set to the first path component of the entity's ID relative to the input root. `Depth` is the number of path separators in the ID.

**Configuration:**

```yaml
passes:
  enrich:
    markdown:
      extensions:
        - tables
        - strikethrough
        - autolink
        - taskList
      syntaxHighlight: true   # wrap code blocks with highlight spans
      safeHTML: false         # strip raw HTML in markdown bodies
```

---

## Pass 4 — taxonomy

**Reads:** `Tags`, `Categories`, `Series` for all entities
**Writes:** `Tags[].Slug`, `Categories[].Slug`; creates synthetic taxonomy entities

For every unique tag value across all entities, this pass creates a synthetic entity representing the taxonomy term page. The taxonomy entity has its own URL (`/tags/{slug}/`), its `Children` field pre-populated with all entities bearing that tag, and `Layout` set to the configured taxonomy template. The same process runs for categories and series.

Taxonomy entities are inserted into the IR and processed by all subsequent passes like any other entity.

**Configuration:**

```yaml
passes:
  taxonomy:
    tags:
      enabled: true
      urlPrefix: /tags/
      layout: tag-list       # template to use for tag pages
      minCount: 1            # only create page if tag appears >= N times
    categories:
      enabled: false
      urlPrefix: /categories/
      layout: category-list
    series:
      enabled: false
      urlPrefix: /series/
      layout: series-list
```

---

## Pass 5 — relationships

**Reads:** `Tags`, `Section`, `Content` (for link extraction), `Title` for all entities
**Writes:** `Related` (top-N related entities per entity with scores)

Scores every entity pair for similarity using four signals:

1. **Tag overlap** — Jaccard similarity of tag sets (default weight: 0.5)
2. **Section match** — 1.0 if entities share a section, 0.0 otherwise (default weight: 0.2)
3. **Link overlap** — fraction of outbound links pointing to the same targets (default weight: 0.2)
4. **Title token overlap** — Jaccard similarity of lowercased title word sets (default weight: 0.1)

The top-N entities by combined score are stored in `Related` for each entity. Entities with a score below `minScore` are excluded.

**Configuration:**

```yaml
passes:
  relationships:
    topN: 5                    # number of related entities to store
    minScore: 0.1              # minimum similarity score to include
    weights:
      tagOverlap: 0.5
      sectionMatch: 0.2
      linkOverlap: 0.2
      titleTokenOverlap: 0.1
```

---

## Pass 6 — graph

**Reads:** entity list order, `URL` (preliminary), `Content` (for link extraction)
**Writes:** `Backlinks`, `Parent`, `Children`, `Prev`, `Next`

Builds the structural graph of the entity corpus. For every outbound link found in an entity's content, if the link target resolves to another entity in the IR, the target's `Backlinks` list gains an entry. Parent/child relationships are derived from URL prefix matching: entity `/docs/overview/` is a child of `/docs/`. `Prev` and `Next` are set based on sort order within the same section.

**Configuration:**

```yaml
passes:
  graph:
    backlinks: true
    parentChild: true
    prevNext: true
    prevNextScope: section   # "section" | "global" | "taxonomy"
```

---

## Pass 7 — url-resolve

**Reads:** `Slug`, `Permalink`, `Section`, `Depth`
**Writes:** `URL` (final), resolves all entity reference URLs in `Backlinks`, `Related`, `Children`, `Parent`

Computes the final canonical URL for every entity. The default URL strategy produces clean URLs: `/blog/hello-world/` for a file at `content/blog/hello-world.md`. If `permalink` is set in frontmatter, that value is used verbatim. After URLs are set, all entity references throughout the IR (Backlinks, Related, etc.) are updated with the resolved URLs.

**Configuration:**

```yaml
passes:
  urlResolve:
    strategy: clean           # "clean" | "pretty" | "flat"
    trailingSlash: true
```

**URL strategies:**

| Strategy | Example input | Example output |
|---|---|---|
| `clean` | `blog/hello-world.md` | `/blog/hello-world/` |
| `pretty` | `blog/hello-world.md` | `/blog/hello-world/index.html` |
| `flat` | `blog/hello-world.md` | `/blog/hello-world.html` |

---

## Pass 8 — schema-gen

**Reads:** `Frontmatter`, `Section` for all entities
**Writes:** `Schema` (per entity); emits schema files if configured

Groups entities by section, then for each section infers a JSON Schema from the observed frontmatter keys and value types. Fields present on every entity in the section are marked `required`. Fields present on some entities are marked optional. Type inference: string values → `string`, numeric values → `number`, boolean → `boolean`, lists → `array`, maps → `object`.

**Configuration:**

```yaml
passes:
  schemaGen:
    enabled: true
    emit: false              # write inferred schemas to output dir
    emitDir: ./_schemas/     # where to write schema files if emit: true
    requiredThreshold: 1.0   # fraction of entities that must have field for it to be required
```

---

## Pass 9 — validate

**Reads:** `Schema`, all entity fields
**Writes:** nothing; fails build on violations

Runs every entity against its section schema. Fails the build with a list of violations if any required field is missing. Also enforces cross-entity invariants: duplicate slugs, duplicate URLs, broken internal links, and any custom rules defined in the config.

**Configuration:**

```yaml
passes:
  validate:
    schema: true             # validate entities against inferred schema
    uniqueSlugs: true        # fail on duplicate slugs
    uniqueURLs: true         # fail on duplicate URLs
    internalLinks: true      # fail on broken internal links
    rules: []                # custom validation rules (see below)
```

**Custom rules:**

```yaml
passes:
  validate:
    rules:
      - name: "require-description"
        section: blog
        require:
          - description
      - name: "date-in-past"
        section: blog
        assert: "entity.Date.Before(now)"
```

---

## Pass 10 — emit-html

**Reads:** full IR
**Writes:** HTML files to output directory

Renders every non-draft entity using its configured layout template. Taxonomy entities use the layout specified in the taxonomy pass configuration. Paginated list pages are generated for sections with `paginate` configured.

See [Output Backends — HTML](/schemaflux/docs/backends/#html) for the full template variable reference.

**Configuration:**

```yaml
backends:
  html:
    enabled: true
    outputDir: ./_site
    templates:
      dir: ./templates
      default: base          # layout to use when entity has no layout
    pagination:
      pageSize: 10
      pageLayout: list
```

---

## Pass 11 — emit-json

**Reads:** full IR
**Writes:** JSON files to output directory

Serializes entities to JSON. By default, one JSON file is written per entity at the same URL path with a `.json` extension. Optionally, a single combined `entities.json` file containing all entities is emitted.

See [Output Backends — JSON](/schemaflux/docs/backends/#json) for the output schema.

**Configuration:**

```yaml
backends:
  json:
    enabled: false
    perEntity: false         # write one JSON file per entity
    combined: true           # write a single entities.json
    combinedFile: entities.json
    fields:                  # fields to include (default: all)
      - id
      - title
      - description
      - url
      - date
      - tags
      - content
```

---

## Pass 12 — emit-sitemap

**Reads:** full IR
**Writes:** `sitemap.xml`, `feed.xml` (RSS)

Writes an XML sitemap containing every non-draft entity with its URL and last-modified date. Optionally emits an RSS feed for a specified section.

**Configuration:**

```yaml
backends:
  sitemap:
    enabled: true
    file: sitemap.xml
    changefreq: weekly       # "always" | "hourly" | "daily" | "weekly" | "monthly" | "yearly" | "never"
    priority: 0.5
    exclude:
      - /tags/**
  rss:
    enabled: false
    section: blog
    file: feed.xml
    title: "My Site Feed"
    description: "Recent posts"
    limit: 20
```

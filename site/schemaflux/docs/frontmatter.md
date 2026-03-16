---
title: "Frontmatter"
description: "All recognized YAML frontmatter keys, their effect on the IR, type coercion rules, and how custom fields are passed through."
---

# Frontmatter

YAML frontmatter is the block between `---` delimiters at the top of a source file. schemaflux parses frontmatter and maps recognized keys to typed fields on the entity IR. Unrecognized keys are passed through verbatim in the `frontmatter` map.

## Recognized keys

### title

```yaml
title: "Hello, World"
```

Type: string. Sets `Entity.Title`. If absent, schemaflux extracts the first H1 heading from the markdown body during the enrich pass. If neither is present, the title falls back to a title-cased version of the slug.

---

### description

```yaml
description: "A short description used in meta tags and search indexes."
```

Type: string. Sets `Entity.Description`. Used in `<meta name="description">` by convention in templates. Available in the search index backend.

---

### date

```yaml
date: 2026-03-15
date: 2026-03-15T10:00:00Z    # RFC3339 also accepted
```

Type: string (parsed to `time.Time`). Sets `Entity.Date`. Used by the sort pass when `sort.primary: date`. Included in RSS feed items and sitemap `<lastmod>`.

Accepted formats: `2006-01-02`, `2006-01-02T15:04:05Z`, `2006-01-02T15:04:05-07:00`, `January 2, 2006`.

---

### updated

```yaml
updated: 2026-03-20
```

Type: string (parsed to `time.Time`). Sets `Entity.UpdatedAt`. Used in sitemap `<lastmod>` when present (takes precedence over `date`).

---

### weight

```yaml
weight: 10
```

Type: int. Sets `Entity.Weight`. Lower values sort earlier. Entities with `weight: 0` (the default) are sorted by date within their weight group.

---

### draft

```yaml
draft: true
```

Type: bool. Default: `false`. When `true`, the entity is excluded from all output unless `input.drafts: true` is set in the config or `--drafts` is passed to `schemaflux build`. Draft entities are never included in the sitemap, RSS feed, or search index regardless of the drafts setting.

---

### layout

```yaml
layout: post
```

Type: string. Sets `Entity.Layout`. The HTML backend looks up `{templates.dir}/{layout}{templates.extension}` to render this entity. If absent, the value of `backends.html.templates.default` is used.

---

### permalink

```yaml
permalink: /custom/path/
```

Type: string. Overrides the computed URL for this entity. The value must begin with `/`. When set, the slug is derived from the last path segment of the permalink value. All other entities' backlinks and related entries use the permalink value for their URL references.

---

### slug

```yaml
slug: custom-slug
```

Type: string. Overrides the computed slug for this entity. The URL is then derived from the slug (unless `permalink` is also set, which takes full precedence). The slug must be URL-safe.

---

### tags

```yaml
tags:
  - announcements
  - schemaflux
  - go
```

Type: list of strings. Sets `Entity.Tags`. Each tag value is slugified and used to create a taxonomy entity page (if `passes.taxonomy.tags.enabled: true`). Tags are also used by the relationship-scoring pass to compute entity similarity.

---

### categories

```yaml
categories:
  - tutorials
  - reference
```

Type: list of strings. Sets `Entity.Categories`. Behaves identically to `tags` but uses the `categories` taxonomy configuration.

---

### series

```yaml
series: "Building with MIST"
```

Type: string. Sets `Entity.Series`. Entities sharing a series value are grouped together by the taxonomy pass when `passes.taxonomy.series.enabled: true`.

---

### section

```yaml
section: blog
```

Type: string. Overrides the inferred section for this entity. By default, section is derived from the first path component of the entity's ID relative to the input root. Setting this explicitly changes which section-scoped features (prev/next, pagination, schema inference) apply to this entity.

---

### noindex

```yaml
noindex: true
```

Type: bool. Default: `false`. When `true`, the entity is excluded from the search index backend output. Does not affect sitemap or HTML emission.

---

### sitemap

```yaml
sitemap:
  changefreq: daily
  priority: 0.8
  exclude: true
```

Type: object. Per-entity sitemap overrides. `exclude: true` removes this entity from the sitemap. `changefreq` and `priority` override the sitemap backend defaults for this entity.

---

### canonical

```yaml
canonical: https://otherdomain.com/original-post/
```

Type: string. Sets an explicit canonical URL pointing to a different domain. When set, the HTML backend renders `<link rel="canonical" href="...">` with this value instead of the entity's own URL.

---

## Type coercion

schemaflux is lenient with frontmatter types. YAML allows `date: 2026-03-15` to be parsed as either a string or a date depending on the parser. schemaflux handles both:

- `date` and `updated`: accepted as YAML date, ISO 8601 string, or RFC3339 string
- `weight`: accepted as integer or float (float is truncated)
- `draft`: accepted as bool (`true`/`false`) or string (`"true"`/`"false"`)
- `tags` and `categories`: accepted as a list or a single string (single string is wrapped in a list)

## Custom fields

Any frontmatter key not listed above is stored in `Entity.Frontmatter` without modification:

```yaml
---
title: "Eval Example 001"
difficulty: hard
input: "Summarize the following text in one sentence."
expected_output: "A concise one-sentence summary."
grader: semantic_similarity
min_score: 0.85
---
```

Access in templates:

```html
<p>Difficulty: {{ index .Frontmatter "difficulty" }}</p>
<p>Grader: {{ index .Frontmatter "grader" }}</p>
```

Access in the JSON backend output:

```json
{
  "frontmatter": {
    "difficulty": "hard",
    "input": "Summarize the following text in one sentence.",
    "expected_output": "A concise one-sentence summary.",
    "grader": "semantic_similarity",
    "min_score": 0.85
  }
}
```

## Required vs optional fields

No frontmatter fields are required by default. The validate pass infers which fields are required based on what is present across entities in the same section — if every entity in the `blog` section has a `description` field, the validate pass will flag blog entities that are missing `description`. This threshold is configurable via `passes.schemaGen.requiredThreshold`.

To enforce specific required fields unconditionally, use custom validation rules:

```yaml
passes:
  validate:
    rules:
      - name: "blog-requires-description"
        section: blog
        require:
          - description
          - date
```

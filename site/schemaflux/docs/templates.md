---
title: "Template Engine"
description: "Go template syntax in schemaflux. Variables available in page and list templates, built-in filters and functions, template inheritance, and traversing the entity graph."
---

# Template Engine

schemaflux uses Go's standard `text/template` package for HTML rendering, extended with a set of built-in functions. Template files live in the directory configured at `templates.dir` (default: `./templates`).

## Template file naming

Each file in the templates directory is a named template. A file at `templates/post.html` is referenced as `layout: post` in frontmatter. A file at `templates/tag-list.html` is referenced as `layout: tag-list`. The extension is configured at `backends.html.templates.extension` (default: `.html`).

## Variables

Every template receives the current entity as the root context (`.`). The entity struct and all its fields are directly accessible.

### Page template variables

| Variable | Type | Description |
|---|---|---|
| `.Title` | `string` | Entity title |
| `.Slug` | `string` | URL-safe slug |
| `.URL` | `string` | Canonical URL path |
| `.Description` | `string` | Short description |
| `.Content` | `template.HTML` | Rendered HTML body (safe to render with `{{ .Content }}`) |
| `.Date` | `time.Time` | Publication date |
| `.UpdatedAt` | `time.Time` | Last updated date |
| `.Weight` | `int` | Sort weight |
| `.Draft` | `bool` | Draft status |
| `.Tags` | `[]Tag` | Tags; each has `.Name` (string) and `.Slug` (string) |
| `.Categories` | `[]Category` | Categories; same structure as Tags |
| `.Series` | `string` | Series name |
| `.Section` | `string` | First path component |
| `.Depth` | `int` | URL depth (number of path segments) |
| `.Related` | `[]*Entity` | Top-N related entities (computed) |
| `.Backlinks` | `[]*Entity` | Entities that link to this one (computed) |
| `.Parent` | `*Entity` | Nearest URL parent (may be `nil`) |
| `.Children` | `[]*Entity` | URL child entities |
| `.Prev` | `*Entity` | Previous entity in section sort order (may be `nil`) |
| `.Next` | `*Entity` | Next entity in section sort order (may be `nil`) |
| `.Frontmatter` | `map[string]any` | All frontmatter key-value pairs |
| `.Graph` | `*Graph` | Full entity graph |
| `.Site` | `*SiteConfig` | Site-level configuration |

### Graph variables

| Variable | Type | Description |
|---|---|---|
| `.Graph.Entities` | `[]*Entity` | All non-draft entities in sort order |
| `.Graph.BySection` | `map[string][]*Entity` | Entities grouped by section name |
| `.Graph.ByTag` | `map[string][]*Entity` | Entities grouped by tag slug |
| `.Graph.Nav` | `[]*Entity` | Top-level navigation: root-depth entities sorted by weight |
| `.Graph.Tags` | `[]*Entity` | All taxonomy tag entities |
| `.Graph.Sections` | `[]string` | All unique section names |
| `.Graph.TotalCount` | `int` | Total entity count |

### Site config variables

| Variable | Type | Description |
|---|---|---|
| `.Site.Title` | `string` | Site title from config |
| `.Site.URL` | `string` | Site base URL from config |
| `.Site.Description` | `string` | Site description from config |
| `.Site.Author` | `string` | Site author from config |

### List template variables (taxonomy and pagination)

Taxonomy pages (tag pages, category pages) and paginated list pages have these additional variables:

| Variable | Type | Description |
|---|---|---|
| `.Children` | `[]*Entity` | All entities in this taxonomy term or page |
| `.Pagination.Page` | `int` | Current page number (1-based) |
| `.Pagination.TotalPages` | `int` | Total pages |
| `.Pagination.Prev` | `string` | URL of previous page |
| `.Pagination.Next` | `string` | URL of next page |
| `.Pagination.Items` | `[]*Entity` | Entities on this page (same as `.Children` when not paginating) |

## Built-in functions

schemaflux extends Go's template functions with the following:

### Date formatting

```html
{{ .Date | dateFormat "January 2, 2006" }}
{{ .Date | dateFormat "2006-01-02" }}
{{ .Date | iso8601 }}         {{/* outputs: 2026-03-15T00:00:00Z */}}
{{ .Date | humanDate }}       {{/* outputs: March 15, 2026 */}}
{{ .Date | relativeDate }}    {{/* outputs: 2 days ago */}}
```

### String manipulation

```html
{{ .Title | lower }}          {{/* lowercase */}}
{{ .Title | upper }}          {{/* uppercase */}}
{{ .Title | title }}          {{/* title case */}}
{{ .Title | truncate 60 }}    {{/* truncate to 60 chars with ellipsis */}}
{{ .Slug | slugify }}         {{/* re-slugify a string */}}
{{ "hello world" | replace " " "-" }}
```

### Content helpers

```html
{{ .Content | stripHTML }}    {{/* strip HTML tags, return plain text */}}
{{ .Content | excerpt 160 }}  {{/* first 160 chars of stripped content */}}
{{ .RawContent | markdownify }}  {{/* render markdown string to HTML */}}
```

### Collection helpers

```html
{{ .Graph.Entities | limit 5 }}          {{/* first 5 entities */}}
{{ .Graph.Entities | offset 10 }}        {{/* skip first 10 */}}
{{ .Graph.Entities | where "Section" "blog" }}   {{/* filter by field value */}}
{{ .Graph.Entities | sortBy "Date" "desc" }}     {{/* sort by field */}}
{{ .Tags | pluck "Name" }}               {{/* extract field from list */}}
{{ .Graph.BySection | keys }}            {{/* keys of a map */}}
```

### URL helpers

```html
{{ .URL | absURL }}           {{/* prepend site.url */}}
{{ "/path/" | absURL }}       {{/* https://example.com/path/ */}}
{{ .URL | relURL }}           {{/* ensure relative URL */}}
```

### Conditional helpers

```html
{{ .Date | isZero }}          {{/* bool: true if zero value */}}
{{ .Description | default "No description available." }}
{{ .Layout | default "base" }}
```

### Math

```html
{{ add 1 2 }}                 {{/* 3 */}}
{{ sub .Pagination.TotalPages 1 }}
{{ mul .Weight 10 }}
{{ div 100 .Pagination.PageSize }}
{{ mod .Pagination.Page 2 }}
```

## Template inheritance and partials

Go templates support `{{ define }}` and `{{ template }}` for composition. The recommended pattern:

**templates/partials/head.html**

```html
{{ define "head" }}
<head>
  <meta charset="UTF-8">
  <title>{{ .Title }} — {{ .Site.Title }}</title>
  {{ if .Description }}<meta name="description" content="{{ .Description }}">{{ end }}
  <link rel="canonical" href="{{ .Site.URL }}{{ .URL }}">
</head>
{{ end }}
```

**templates/partials/nav.html**

```html
{{ define "nav" }}
<nav>
  {{ range .Graph.Nav }}
  <a href="{{ .URL }}"{{ if eq .URL $.URL }} aria-current="page"{{ end }}>{{ .Title }}</a>
  {{ end }}
</nav>
{{ end }}
```

**templates/base.html**

```html
<!DOCTYPE html>
<html lang="en">
  {{ template "head" . }}
  <body>
    {{ template "nav" . }}
    <main>
      <h1>{{ .Title }}</h1>
      {{ .Content }}
    </main>
  </body>
</html>
```

**templates/post.html**

```html
<!DOCTYPE html>
<html lang="en">
  {{ template "head" . }}
  <body>
    {{ template "nav" . }}
    <main>
      <article>
        <h1>{{ .Title }}</h1>
        <time datetime="{{ .Date | iso8601 }}">{{ .Date | humanDate }}</time>
        {{ if .Tags }}
        <ul class="tags">
          {{ range .Tags }}
          <li><a href="/tags/{{ .Slug }}/">{{ .Name }}</a></li>
          {{ end }}
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
    </main>
  </body>
</html>
```

schemaflux loads all template files from the templates directory at build start, making all `{{ define }}` blocks available to all templates without explicit import.

## Accessing the entity graph from templates

Templates can traverse the full entity graph to build navigation, tag clouds, related content sections, and more:

```html
<!-- Site-wide navigation from all root-level entities -->
<nav>
  {{ range .Graph.Nav }}
  <a href="{{ .URL }}">{{ .Title }}</a>
  {{ end }}
</nav>

<!-- All posts in the blog section, sorted by date -->
{{ range .Graph.BySection.blog | sortBy "Date" "desc" }}
<article>
  <a href="{{ .URL }}">{{ .Title }}</a>
  <time>{{ .Date | humanDate }}</time>
</article>
{{ end }}

<!-- Tag cloud with post counts -->
{{ range .Graph.Tags }}
<a href="{{ .URL }}" style="font-size: {{ len .Children | mul 0.1 | add 0.9 }}em">
  {{ .Title }} ({{ len .Children }})
</a>
{{ end }}

<!-- Most recent 5 posts (any section) -->
{{ .Graph.Entities | where "Section" "blog" | sortBy "Date" "desc" | limit 5 }}

<!-- Breadcrumb navigation using Parent chain -->
{{ $e := . }}
{{ range $e | ancestors }}
<a href="{{ .URL }}">{{ .Title }}</a> /
{{ end }}
```

## Taxonomy template variables

Taxonomy pages (tag pages, category pages) have `.Title` set to the term name and `.Children` set to all matching entities:

```html
<!-- templates/tag-list.html -->
<h1>Posts tagged "{{ .Title }}"</h1>
<p>{{ len .Children }} posts</p>
{{ range .Children | sortBy "Date" "desc" }}
<article>
  <a href="{{ .URL }}">{{ .Title }}</a>
  <time>{{ .Date | humanDate }}</time>
  {{ if .Description }}<p>{{ .Description }}</p>{{ end }}
</article>
{{ end }}
```

## Accessing custom frontmatter fields

Custom fields from frontmatter are available via `.Frontmatter`:

```html
{{ $author := index .Frontmatter "author" }}
{{ if $author }}<p>By {{ $author }}</p>{{ end }}

{{ $difficulty := index .Frontmatter "difficulty" }}
{{ if $difficulty }}<span class="badge {{ $difficulty }}">{{ $difficulty }}</span>{{ end }}
```

## Conditional rendering

```html
{{ if .Parent }}<a href="{{ .Parent.URL }}">← Back to {{ .Parent.Title }}</a>{{ end }}
{{ if .Backlinks }}
<aside>
  <h3>Referenced by {{ len .Backlinks }} pages</h3>
  {{ range .Backlinks }}<a href="{{ .URL }}">{{ .Title }}</a>{{ end }}
</aside>
{{ end }}
{{ if not .Date.IsZero }}<time>{{ .Date | humanDate }}</time>{{ end }}
```

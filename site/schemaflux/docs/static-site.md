---
title: "Building a Static Site"
description: "Use schemaflux as a full static site generator. Content structure, layout templates, navigation, taxonomy pages, pagination, and search — end to end."
---

# Building a Static Site

This guide walks through building a complete static site with schemaflux: content structure, layout templates, site-wide navigation, taxonomy pages, pagination, and a search index. It assumes you have completed the [Quick Start](/schemaflux/docs/quick-start/).

## Project structure

A typical schemaflux site looks like this:

```
my-site/
  content/
    index.md              # home page
    about.md              # about page
    blog/
      2026-01-post.md
      2026-02-post.md
      2026-03-post.md
    docs/
      overview.md
      quick-start.md
      reference.md
  templates/
    partials/
      head.html
      nav.html
      footer.html
    base.html
    post.html
    list.html
    tag-list.html
  static/
    style.css
    favicon.ico
  schemaflux.yml
```

## Content structure

### Home page

```markdown
---
title: "My Site"
layout: base
permalink: /
weight: 0
description: "A site about things."
---

Welcome to my site.
```

Setting `weight: 0` and `permalink: /` ensures this entity appears first in navigation.

### Blog posts

```markdown
---
title: "My First Post"
date: 2026-01-15
layout: post
tags:
  - go
  - tooling
description: "A post about building with schemaflux."
---

Post content here.
```

Place all blog posts in `content/blog/`. The `section` field will be automatically inferred as `blog`.

### Documentation pages

```markdown
---
title: "Overview"
layout: docs-page
weight: 1
section: docs
description: "Introduction to the project."
---

Documentation content here.
```

Use `weight` to control page ordering within the docs section.

## Configuration

```yaml
# schemaflux.yml
site:
  title: "My Site"
  url: https://example.com
  description: "A site about things."
  author: "Your Name"

input:
  dir: ./content

output:
  dir: ./_site

templates:
  dir: ./templates

passes:
  taxonomy:
    tags:
      enabled: true
      urlPrefix: /tags/
      layout: tag-list
      minCount: 1
  relationships:
    topN: 5
  graph:
    prevNextScope: section

backends:
  html:
    enabled: true
    templates:
      default: base
    pagination:
      pageSize: 10
      sections:
        - blog
      pageLayout: list
  sitemap:
    enabled: true
    file: sitemap.xml
    exclude:
      - /tags/**
  rss:
    enabled: true
    section: blog
    file: feed.xml
    title: "My Site — Blog"
    limit: 20
  search:
    enabled: true
    file: search.json
    fields:
      - title
      - description
      - content
      - tags
      - url
```

## Layout templates

### Shared partials

**templates/partials/head.html**

```html
{{ define "head" }}
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{ .Title }} — {{ .Site.Title }}</title>
  {{ if .Description }}<meta name="description" content="{{ .Description }}">{{ end }}
  <link rel="canonical" href="{{ .Site.URL }}{{ .URL }}">
  <link rel="stylesheet" href="/static/style.css">
  <link rel="alternate" type="application/rss+xml" title="{{ .Site.Title }}" href="/feed.xml">
</head>
{{ end }}
```

**templates/partials/nav.html**

```html
{{ define "nav" }}
<nav class="site-nav">
  <a href="/" class="site-title">{{ .Site.Title }}</a>
  <ul>
    {{ range .Graph.Nav }}
    <li>
      <a href="{{ .URL }}"{{ if eq .URL $.URL }} aria-current="page"{{ end }}>
        {{ .Title }}
      </a>
    </li>
    {{ end }}
  </ul>
</nav>
{{ end }}
```

**templates/partials/footer.html**

```html
{{ define "footer" }}
<footer>
  <p>© {{ now | dateFormat "2006" }} {{ .Site.Author }}</p>
  <nav>
    <a href="/feed.xml">RSS</a>
    <a href="/sitemap.xml">Sitemap</a>
  </nav>
</footer>
{{ end }}
```

### Base layout

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
    {{ template "footer" . }}
  </body>
</html>
```

### Blog post layout

**templates/post.html**

```html
<!DOCTYPE html>
<html lang="en">
  {{ template "head" . }}
  <body>
    {{ template "nav" . }}
    <main>
      <article>
        <header>
          <h1>{{ .Title }}</h1>
          <div class="post-meta">
            <time datetime="{{ .Date | iso8601 }}">{{ .Date | humanDate }}</time>
            {{ if .Tags }}
            <span class="tags">
              {{ range .Tags }}
              <a href="/tags/{{ .Slug }}/" class="tag">{{ .Name }}</a>
              {{ end }}
            </span>
            {{ end }}
          </div>
          {{ if .Description }}<p class="description">{{ .Description }}</p>{{ end }}
        </header>

        <div class="post-content">
          {{ .Content }}
        </div>

        <footer class="post-footer">
          <nav class="prevnext">
            {{ if .Prev }}
            <a href="{{ .Prev.URL }}" class="prev">← {{ .Prev.Title }}</a>
            {{ end }}
            {{ if .Next }}
            <a href="{{ .Next.URL }}" class="next">{{ .Next.Title }} →</a>
            {{ end }}
          </nav>

          {{ if .Related }}
          <aside class="related">
            <h2>Related posts</h2>
            <ul>
              {{ range .Related }}
              <li><a href="{{ .URL }}">{{ .Title }}</a></li>
              {{ end }}
            </ul>
          </aside>
          {{ end }}
        </footer>
      </article>
    </main>
    {{ template "footer" . }}
  </body>
</html>
```

### Blog list layout (paginated)

**templates/list.html**

```html
<!DOCTYPE html>
<html lang="en">
  {{ template "head" . }}
  <body>
    {{ template "nav" . }}
    <main>
      <h1>Blog</h1>

      <ol class="post-list">
        {{ range .Pagination.Items }}
        <li>
          <a href="{{ .URL }}">{{ .Title }}</a>
          <time>{{ .Date | humanDate }}</time>
          {{ if .Description }}<p>{{ .Description }}</p>{{ end }}
        </li>
        {{ end }}
      </ol>

      {{ if gt .Pagination.TotalPages 1 }}
      <nav class="pagination">
        {{ if .Pagination.Prev }}
        <a href="{{ .Pagination.Prev }}">← Newer</a>
        {{ end }}
        <span>Page {{ .Pagination.Page }} of {{ .Pagination.TotalPages }}</span>
        {{ if .Pagination.Next }}
        <a href="{{ .Pagination.Next }}">Older →</a>
        {{ end }}
      </nav>
      {{ end }}
    </main>
    {{ template "footer" . }}
  </body>
</html>
```

### Tag page layout

**templates/tag-list.html**

```html
<!DOCTYPE html>
<html lang="en">
  {{ template "head" . }}
  <body>
    {{ template "nav" . }}
    <main>
      <h1>Posts tagged "{{ .Title }}"</h1>
      <p>{{ len .Children }} post{{ if ne (len .Children) 1 }}s{{ end }}</p>

      <ol class="post-list">
        {{ range .Children | sortBy "Date" "desc" }}
        <li>
          <a href="{{ .URL }}">{{ .Title }}</a>
          <time>{{ .Date | humanDate }}</time>
          {{ if .Description }}<p>{{ .Description }}</p>{{ end }}
        </li>
        {{ end }}
      </ol>
    </main>
    {{ template "footer" . }}
  </body>
</html>
```

## Navigation generation

The `.Graph.Nav` variable returns all entities at depth 1 (direct children of the root) sorted by weight. To control which pages appear in the navigation, set `weight` on entities you want included and leave out the `layout` field on index pages you don't want to surface:

```markdown
---
title: "Blog"
layout: base
permalink: /blog/
weight: 20
---
```

For multi-level navigation, use the `.Children` field recursively:

```html
{{ define "nav-item" }}
<li>
  <a href="{{ .URL }}">{{ .Title }}</a>
  {{ if .Children }}
  <ul>{{ range .Children }}{{ template "nav-item" . }}{{ end }}</ul>
  {{ end }}
</li>
{{ end }}

<nav>
  <ul>{{ range .Graph.Nav }}{{ template "nav-item" . }}{{ end }}</ul>
</nav>
```

## Static assets

schemaflux copies the `static/` directory to the output directory verbatim. Reference static files with root-relative paths in templates:

```html
<link rel="stylesheet" href="/static/style.css">
<img src="/static/logo.png" alt="Logo">
```

Configure the static directory in `schemaflux.yml`:

```yaml
static:
  dir: ./static
  outputDir: ./static   # relative to output.dir
```

## Adding search

With `backends.search.enabled: true`, schemaflux writes `search.json` to the output directory. Use a client-side library like Fuse.js to enable search:

```html
<!-- In your base template, include Fuse.js and the search index -->
<script src="/static/fuse.min.js"></script>
<script>
  fetch('/search.json')
    .then(r => r.json())
    .then(data => {
      const fuse = new Fuse(data, {
        keys: ['title', 'description', 'content'],
        threshold: 0.3
      });
      // Wire fuse to a search input
      document.getElementById('search').addEventListener('input', e => {
        const results = fuse.search(e.target.value);
        // Render results
      });
    });
</script>
```

## Building and deploying

```bash
# Build
schemaflux build --clean

# Deploy (example: rsync to a server)
rsync -avz _site/ user@server:/var/www/html/

# Deploy to Netlify (netlify.toml)
# [build]
# command = "go install github.com/greynewell/schemaflux/cmd/schemaflux@latest && schemaflux build"
# publish = "_site"
```

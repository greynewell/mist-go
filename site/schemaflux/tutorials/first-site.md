---
title: "Building a Static Site"
description: "Create a content directory, write layout templates, configure schemaflux, and compile a complete static site with taxonomy pages and a search index."
difficulty: beginner
time: 15 min
tags:
  - HTML backend
  - templates
  - taxonomy
---

# Building a Static Site

<div class="tutorial-meta">
  <span class="meta-tag">beginner</span>
  <span class="tutorial-time">15 min</span>
</div>

In this tutorial you will build a small but complete static site from scratch. By the end you will have a compiled site with a home page, an about page, several blog posts, auto-generated tag pages, and a search index — all from markdown files and a single config.

**What you will build:**

- 3 content pages and 3 blog posts
- Layout templates for pages and posts
- Taxonomy pages for each tag
- A search index JSON file
- An XML sitemap

**Prerequisites:** schemaflux installed (`schemaflux version` should print a version). See [Installation](/schemaflux/docs/installation/).

---

<div class="step">
<div class="step-number">Step 1</div>

## Create the project

```bash
mkdir my-first-site
cd my-first-site
```

Create the directory structure:

```bash
mkdir -p content/blog templates/partials static
```

</div>

<div class="step">
<div class="step-number">Step 2</div>

## Write the content files

Create the home page at `content/index.md`:

```markdown
---
title: "Home"
layout: base
permalink: /
weight: 0
description: "A site about Go and tooling."
---

Welcome to my site. I write about Go, compilers, and developer tooling.
```

Create the about page at `content/about.md`:

```markdown
---
title: "About"
layout: base
permalink: /about/
weight: 10
description: "About this site and its author."
---

This site is compiled with [schemaflux](https://miststack.dev/schemaflux/),
a structured data compiler built on mist-go.
```

Create three blog posts:

**content/blog/hello-schemaflux.md**

```markdown
---
title: "Hello, schemaflux"
date: 2026-01-10
layout: post
tags:
  - schemaflux
  - tooling
description: "First impressions of schemaflux as a static site generator."
---

schemaflux takes a different approach to static sites. Instead of rendering
files one by one, it builds a complete graph of all your content before
emitting a single file.
```

**content/blog/go-templates.md**

```markdown
---
title: "Go Templates for Static Sites"
date: 2026-02-05
layout: post
tags:
  - go
  - templates
description: "Using Go's text/template package for static site layouts."
---

Go's template package is more capable than it first appears. Combined with
schemaflux's template functions, you can build complex layouts cleanly.
```

**content/blog/compiler-architecture.md**

```markdown
---
title: "The schemaflux Compiler Architecture"
date: 2026-03-01
layout: post
tags:
  - schemaflux
  - compilers
  - go
description: "A look at the 12-pass IR pipeline that powers schemaflux."
---

schemaflux processes content through 12 sequential passes before emitting
any output. Here's why that matters and what each pass does.
```

</div>

<div class="step">
<div class="step-number">Step 3</div>

## Write the config

Create `schemaflux.yml` at the project root:

```yaml
site:
  title: "My First Site"
  url: https://example.com
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
  relationships:
    topN: 3

backends:
  html:
    enabled: true
    templates:
      default: base
  sitemap:
    enabled: true
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

</div>

<div class="step">
<div class="step-number">Step 4</div>

## Write the templates

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
</head>
{{ end }}
```

**templates/partials/nav.html**

```html
{{ define "nav" }}
<nav>
  <a href="/">{{ .Site.Title }}</a>
  <a href="/about/">About</a>
  <a href="/blog/">Blog</a>
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
        <time>{{ .Date | humanDate }}</time>
        {{ if .Tags }}
        <ul class="tags">
          {{ range .Tags }}
          <li><a href="/tags/{{ .Slug }}/">{{ .Name }}</a></li>
          {{ end }}
        </ul>
        {{ end }}
        {{ .Content }}
        {{ if .Related }}
        <aside>
          <h2>Related</h2>
          {{ range .Related }}
          <a href="{{ .URL }}">{{ .Title }}</a>
          {{ end }}
        </aside>
        {{ end }}
        <nav>
          {{ if .Prev }}<a href="{{ .Prev.URL }}">← {{ .Prev.Title }}</a>{{ end }}
          {{ if .Next }}<a href="{{ .Next.URL }}">{{ .Next.Title }} →</a>{{ end }}
        </nav>
      </article>
    </main>
  </body>
</html>
```

**templates/tag-list.html**

```html
<!DOCTYPE html>
<html lang="en">
  {{ template "head" . }}
  <body>
    {{ template "nav" . }}
    <main>
      <h1>Posts tagged "{{ .Title }}"</h1>
      <ul>
        {{ range .Children | sortBy "Date" "desc" }}
        <li>
          <a href="{{ .URL }}">{{ .Title }}</a>
          <time>{{ .Date | humanDate }}</time>
        </li>
        {{ end }}
      </ul>
    </main>
  </body>
</html>
```

</div>

<div class="step">
<div class="step-number">Step 5</div>

## Run the build

```bash
schemaflux build
```

You should see output like:

```
parsing   5 files
pass 1/12 slugify
pass 2/12 sort
pass 3/12 enrich
pass 4/12 taxonomy
pass 5/12 relationships
pass 6/12 graph
pass 7/12 url-resolve
pass 8/12 schema-gen
pass 9/12 validate        5 entities  →  0 violations
pass 10/12 emit-html      13 files
pass 12/12 emit-sitemap   2 files

output    15 files in ./_site/
built in  24ms
```

</div>

<div class="step">
<div class="step-number">Step 6</div>

## Inspect the output

```bash
find _site -type f | sort
```

```
_site/index.html
_site/about/index.html
_site/blog/hello-schemaflux/index.html
_site/blog/go-templates/index.html
_site/blog/compiler-architecture/index.html
_site/tags/schemaflux/index.html
_site/tags/tooling/index.html
_site/tags/go/index.html
_site/tags/templates/index.html
_site/tags/compilers/index.html
_site/sitemap.xml
_site/search.json
```

schemaflux created tag pages for every unique tag across your posts — without any source files for those pages. Open `_site/tags/schemaflux/index.html` and you'll see it lists the two posts tagged `schemaflux`.

Open `_site/search.json` to see the structured search index — an array of objects with `title`, `description`, `content`, `tags`, and `url` for each entity. You can wire this to any client-side search library.

</div>

<div class="step">
<div class="step-number">Step 7 (optional)</div>

## Preview locally

```bash
schemaflux watch --serve --open
# Serving ./_site on http://127.0.0.1:3000
```

Edit any content file or template and schemaflux will rebuild automatically.

</div>

---

## What you built

- A complete static site compiled from 5 markdown files
- Auto-generated tag pages derived from frontmatter (zero source files)
- Related posts computed by the relationship-scoring pass
- A JSON search index
- An XML sitemap

## Next steps

- [Configuration reference](/schemaflux/docs/config/) — all schemaflux.yml options
- [Templates reference](/schemaflux/docs/templates/) — all template variables and functions
- [The 12 Passes](/schemaflux/docs/passes/) — understand what each pass does
- [Compiling Eval Datasets tutorial](/schemaflux/tutorials/eval-dataset/) — use schemaflux for eval data

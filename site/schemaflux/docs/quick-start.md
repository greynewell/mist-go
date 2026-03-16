---
title: "Quick Start"
description: "Install schemaflux, create a content directory, write a minimal config, and build your first site in under five minutes."
---

# Quick Start

This guide gets you from zero to a compiled static site in about five minutes. You will install the binary, create three markdown files, write a minimal config, and run your first build.

## 1. Install

```bash
go install github.com/greynewell/schemaflux/cmd/schemaflux@latest
```

Verify the installation:

```bash
schemaflux version
# schemaflux v0.9.0 (go1.22.3/darwin/arm64)
```

See [Installation](/schemaflux/docs/installation/) for alternative methods and verification steps.

## 2. Create a content directory

Create the following file structure:

```
my-site/
  content/
    index.md
    about.md
    blog/
      hello-world.md
  templates/
    base.html
    post.html
  schemaflux.yml
```

Create the content files:

**content/index.md**

```markdown
---
title: "Home"
layout: base
permalink: /
weight: 0
---

Welcome to my site. Built with schemaflux.
```

**content/about.md**

```markdown
---
title: "About"
layout: base
permalink: /about/
weight: 10
---

This site is compiled by schemaflux, a structured data compiler built on mist-go.
```

**content/blog/hello-world.md**

```markdown
---
title: "Hello, World"
date: 2026-03-15
layout: post
tags:
  - announcements
  - schemaflux
description: "The first post on this site."
---

This is the first post. schemaflux compiled it from a markdown file with YAML frontmatter.
```

## 3. Write a minimal schemaflux.yml

```yaml
input:
  dir: ./content

output:
  dir: ./_site

templates:
  dir: ./templates

backends:
  - html
  - sitemap

site:
  url: https://example.com
  title: "My Site"
```

## 4. Write layout templates

schemaflux uses Go template syntax. Create two minimal templates.

**templates/base.html**

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>{{ .Title }} — {{ .Site.Title }}</title>
  <meta name="description" content="{{ .Description }}">
</head>
<body>
  <nav>
    {{ range .Graph.Nav }}
    <a href="{{ .URL }}">{{ .Title }}</a>
    {{ end }}
  </nav>
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
<head>
  <meta charset="UTF-8">
  <title>{{ .Title }} — {{ .Site.Title }}</title>
  <meta name="description" content="{{ .Description }}">
</head>
<body>
  <main>
    <h1>{{ .Title }}</h1>
    <p>{{ .Date.Format "January 2, 2006" }}</p>
    <div>
      {{ range .Tags }}
      <a href="/tags/{{ .Slug }}/">{{ .Name }}</a>
      {{ end }}
    </div>
    {{ .Content }}
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

## 5. Run the build

```bash
cd my-site
schemaflux build
```

Output:

```
parsing   3 files
pass 1/12 slugify
pass 2/12 sort
pass 3/12 enrich
pass 4/12 taxonomy
pass 5/12 relationships
pass 6/12 graph
pass 7/12 url-resolve
pass 8/12 schema-gen
pass 9/12 validate
pass 10/12 emit-html
pass 11/12 emit-json
pass 12/12 emit-sitemap

output    7 files in _site/
built in  12ms
```

The `_site/` directory now contains:

```
_site/
  index.html          # from content/index.md
  about/
    index.html        # from content/about.md
  blog/
    hello-world/
      index.html      # from content/blog/hello-world.md
  tags/
    announcements/
      index.html      # generated taxonomy page
    schemaflux/
      index.html      # generated taxonomy page
  sitemap.xml
```

The taxonomy pages under `tags/` were generated automatically from the `tags` frontmatter field — no source files required.

## What's next

- [IR](/schemaflux/docs/ir/) — understand the entity graph the pipeline builds
- [The 12 Passes](/schemaflux/docs/passes/) — what each pass does and how to configure it
- [Configuration](/schemaflux/docs/config/) — complete schemaflux.yml reference
- [Templates](/schemaflux/docs/templates/) — all template variables and functions

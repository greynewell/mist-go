---
title: "Overview"
description: "What schemaflux compiles, why it uses an intermediate representation, how it differs from a static site generator, and where it fits in the MIST stack."
---

# Overview

schemaflux is a structured data compiler. It reads a directory of markdown files with YAML frontmatter, processes them through a 12-pass IR pipeline, and writes typed output to one or more backends — HTML pages, JSON files, RSS feeds, XML sitemaps, and full-text search indexes.

## What schemaflux compiles

The input is a set of markdown files with YAML frontmatter. Each file represents one **entity** in the output graph. The frontmatter provides structured metadata — title, date, tags, layout, relationships, custom fields — and the markdown body provides the content that gets rendered into the entity's `content` field after the parse pass.

```
content/
  blog/
    introducing-schemaflux.md
    compiler-architecture.md
  docs/
    overview.md
    quick-start.md
  _data/
    authors.yml
```

After a `schemaflux build`, the output directory contains one file per entity (for HTML and JSON backends), plus generated pages for every taxonomy term, a sitemap, an RSS feed, and a search index JSON file. None of those generated pages require source files — schemaflux derives them from the compiled IR.

## Why an IR instead of direct template rendering

Most static site generators work file-by-file. They read a markdown file, apply a template, and write an HTML file. This is efficient for simple sites, but the model has a hard limit: a file can only reference data it can find at render time, and render time happens before any other file has been processed.

schemaflux inverts this. It reads all source files, builds a complete in-memory graph of every entity and their relationships, and then runs the pipeline. By the time any template executes, the full graph is available — including computed fields that didn't exist in any source file:

- **Backlinks**: which entities link to this one (computed by the relationship-scoring pass)
- **Related**: entities with the highest relationship score to this one
- **Taxonomy pages**: a page per unique tag value, listing all entities with that tag
- **Breadcrumbs and navigation**: derived from URL structure or explicitly specified weight fields

Templates can traverse the graph in any direction. A post template can render its related posts. A tag page can render all posts with that tag. A search index can include fields from the entire entity corpus. These would require custom plugins in most static site generators; in schemaflux they follow automatically from the compiled IR.

## The compiler vs SSG distinction

schemaflux is a compiler first and a static site generator second. The distinction matters for how you think about configuration and debugging.

In a compiler model, you define the transformation pipeline and the output backends, and the compiler handles the rest. You don't configure per-page behavior — you configure passes that apply uniformly to all entities. If something goes wrong, the validate pass catches it before any output is written. The build either succeeds completely or fails with a diagnostic. There is no partial output that looks correct but contains broken links or missing data.

The SSG behavior — HTML pages, layouts, pagination — is provided by the HTML backend, which is one of several backends the compiler can target. You can run the HTML backend alongside the JSON and sitemap backends in a single build, or you can run the JSON backend alone to produce a machine-readable dataset without ever touching templates.

## Use cases

**Static sites.** The most common use of schemaflux. Define a content directory, write layout templates in Go template syntax, configure the HTML and sitemap backends, and run `schemaflux build` to produce a complete static site. evaldriven.org is built with schemaflux via the `pssg` tool.

**Eval datasets.** Annotate markdown files with evaluation-specific frontmatter — input, expected output, grader config — and use the JSON backend to emit a structured dataset. The 12-pass pipeline enriches the dataset with taxonomy groupings, difficulty scores, and relationship metadata before serialization. The output is ready to load into matchspec.

**Data pipelines.** schemaflux can compile any structured markdown corpus into typed JSON. Use it as an offline compilation step: content editors write markdown, the compiler normalizes it, validates it against the inferred schema, and writes clean JSON for downstream consumers. The `graph2md` tool inverts this: it reads a compiled IR and writes markdown files with enriched frontmatter.

**Documentation sites.** Documentation has structure that flat SSGs handle poorly — section ordering, prev/next navigation, cross-references between pages. The schemaflux `weight`, `section`, and relationship features handle all of this at the IR level, so documentation layout templates are as simple as blog templates.

## Relationship to mist-go

schemaflux is built on [mist-go](/mist-go/), the shared MIST stack core library. It uses mist-go for its internal graph representation, type system, and the pass scheduler. If you want to add a custom pass or backend, you import `mist-go` packages directly and plug into the same pipeline schemaflux uses internally.

schemaflux is also designed to feed [matchspec](/matchspec/). The eval dataset compilation workflow produces JSON that matchspec can load as a dataset without any transformation. The frontmatter schema for eval examples maps directly to matchspec's `Example` type.

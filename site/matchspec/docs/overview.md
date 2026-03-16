---
title: Overview
description: What matchspec is, the eval-driven development philosophy, and how it fits into the MIST stack.
---

# Overview

matchspec is an eval framework for AI systems. It gives you a structured way to define correctness, measure it against real model outputs, and enforce it as part of your deployment pipeline.

## The eval-driven development philosophy

Shipping features for an AI system without evals is like shipping backend code without tests — you might get lucky once, but you won't know when you break something, and you'll eventually make a change that degrades quality in a way you only discover from user complaints.

Eval-driven development treats evals as first-class engineering artifacts. You write them before or alongside your prompts and models, you version them in source control, you run them in CI, and you gate deployments on passing thresholds. The goal is not to prove your system is perfect — it's to have a clear, reproducible signal that quality has not regressed since the last time you measured.

The key insight is that **correctness must be defined as code**. Natural language descriptions of what a system should do are ambiguous and unenforceable. A dataset of 100 input/output pairs that exercises the cases you care about is specific, runnable, and reviewable.

## Where matchspec fits

matchspec sits between your model or prompt and your production deployment:

1. You already have a model (fine-tuned or off-the-shelf) and a prompt template.
2. You define what "correct" means for your use case — a set of example inputs and expected outputs.
3. You write graders that score model outputs against those expectations.
4. You configure thresholds: the minimum pass rate required before you're allowed to deploy.
5. matchspec runs your evals in CI and enforces those thresholds.

It is not a training framework. It does not manage models, fine-tuning, or inference serving. For inference routing and provider failover, see [infermux](/infermux/). For structured data compilation from model outputs, see [schemaflux](/schemaflux/).

## Architecture

An eval pipeline in matchspec has five layers:

**Dataset** — A collection of examples, each consisting of an input (what you send to the model), an expected output (what you want back), and optional metadata. Datasets can be defined in YAML or as Go structs, and they can be loaded from files, embedded in Go binaries, or seeded from existing data.

**Grader** — A function that takes a model output and an expected output and returns a score between 0.0 and 1.0. matchspec ships with five built-in graders: `exact_match`, `contains`, `regex`, `semantic_similarity`, and `llm_judge`. You can also implement the `Grader` interface to write your own.

**Harness** — The wiring layer. A harness binds a dataset, a model function, and one or more graders. When you run a harness, it calls your model on each example in the dataset, scores the output with each grader, and collects per-example results.

**Suite** — A collection of one or more harnesses with per-grader thresholds. Running a suite produces a report showing the aggregate pass rate for each grader, whether each threshold was met, and an overall PASS or FAIL verdict.

**Results** — Structured output from a suite run. Results are printed to stdout in human-readable form, written to a JSON file for machine consumption, and (optionally) served via the HTTP API. Non-zero exit code on failure means CI integration requires no custom logic.

## Relationship to mist-go

matchspec is built on [mist-go](/mist-go/), the shared core library for the MIST stack. mist-go provides the transport layer, protocol definitions, circuit breaking, and checkpointing that all MIST tools share. You don't need to understand mist-go to use matchspec — it's a build dependency, not a runtime requirement — but if you're building custom integrations or extending matchspec, familiarity with mist-go's interfaces will help.

The zero-dependency guarantee applies at the binary level: `matchspec run` has no runtime dependencies beyond the Go standard library. Graders that call external services (like `semantic_similarity` or `llm_judge`) require network access to those services, but the binary itself carries no vendored dependencies.

## Next steps

- [Quick Start](/matchspec/docs/quick-start/) — Run your first eval suite in five minutes.
- [Datasets](/matchspec/docs/datasets/) — Learn the full dataset format and loading options.
- [Graders](/matchspec/docs/graders/) — All built-in grader types and how to write your own.

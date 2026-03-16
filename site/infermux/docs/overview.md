---
title: "Overview"
description: "What infermux solves, how it works, and where it fits in the MIST stack."
---

# Overview

infermux is an inference router. It sits between your application code and your LLM providers and handles provider selection, failure detection, automatic failover, and cost attribution. Your application sends requests to infermux using the standard OpenAI HTTP API. infermux decides which provider to use, forwards the request, and returns the response — with routing metadata attached as headers.

## What problems infermux solves

**Provider lock-in.** If your application talks directly to the OpenAI API, switching to Anthropic means rewriting HTTP client code, handling different authentication, mapping model names, and dealing with API shape differences. infermux normalizes all of this behind a single OpenAI-compatible interface. Add a new provider to your config file; no application code changes.

**Reliability.** LLM providers have outages, rate limits, and periods of elevated latency. Most teams handle this with retry logic written directly into application code — which is inconsistent, unobservable, and doesn't account for the difference between a retryable error and a hard failure. infermux centralizes this logic with a circuit breaker per provider. When OpenAI starts returning 5xx errors, the circuit opens and requests automatically route to Anthropic (or Ollama, or wherever you've configured as fallback) without any application code involvement.

**Cost visibility.** Without a routing layer, cost attribution requires parsing every API response for token counts, maintaining pricing tables, and writing aggregation logic separately in each service. infermux does this for every request. Cost data is available immediately via the management API, tagged by caller, model, and route group.

## Architecture

infermux has four layers:

**Provider registry.** A map of named providers, each with an endpoint URL, authentication credentials, rate limit configuration, model aliases, and health state. Providers are declared in `infermux.yml` and loaded at startup. The registry tracks per-provider health status, updated continuously by the circuit breaker layer.

**Router.** The router receives each incoming HTTP request and selects a target provider. The selection algorithm depends on the configured strategy: `round_robin` cycles through healthy providers, `least_latency` uses an exponentially-weighted moving average of recent response times, `cost_weighted` selects the provider that minimizes expected cost for the requested model, and `priority` always tries providers in declared order. The router consults the provider registry to filter out providers with open circuits before selection.

**Circuit breaker.** Each provider has an independent circuit breaker that tracks the error rate and p95 latency over a sliding window. When either metric crosses a configured threshold, the circuit opens and the provider is removed from the healthy set. After a configurable recovery window, the circuit enters half-open state and allows a single probe request through. If the probe succeeds, the circuit closes. If it fails, the recovery window resets. This is the standard three-state circuit breaker pattern, implemented using `mist-go/circuitbreaker`.

**Transport layer.** The actual HTTP request to the selected provider is made via `mist-go/transport`, which handles TLS, timeouts, connection pooling, and request signing. This is the same transport layer used by other MIST stack tools and inherits its observability instrumentation: every request emits a `mist-go/metrics` event that includes provider name, model, latency, token counts, and cost.

## Relationship to mist-go packages

infermux is built directly on top of mist-go packages rather than wrapping them:

- `mist-go/circuitbreaker` — provides the `Breaker` type used by the provider registry. infermux configures one `Breaker` per provider using `circuitbreaker.Config`.
- `mist-go/transport` — provides the HTTP client used for all outbound provider requests.
- `mist-go/metrics` — infermux emits `InferenceRequest` and `ProviderCost` events on this bus. Any `Subscriber` you've registered (tokentrace's collector, a custom exporter) receives them.
- `mist-go/protocol` — request and response types use the mist protocol envelope for inter-service communication when infermux is embedded as a library.

## Use cases

**Multi-provider resilience.** Run OpenAI as primary and Anthropic as fallback. When OpenAI is degraded, requests automatically shift to Anthropic without alarming users or requiring on-call intervention.

**Cost arbitrage.** Different providers charge different prices for equivalent quality on simple tasks. Route summarization and classification requests to a cheap model (Gemini Flash, Claude Haiku, GPT-4o-mini) and reserve expensive models for complex reasoning tasks.

**Local development and testing.** Route to a local Ollama instance in development, a staging cluster in CI, and OpenAI in production — using the same application config.

**Spend control for multi-tenant systems.** Assign a caller ID per tenant. Set per-caller monthly budgets. When a tenant approaches their limit, infermux routes their requests to a cheaper model or returns a 429 before the request reaches the provider.

**Eval pipelines.** Run inference during eval at the cheapest possible rate. Pair infermux with matchspec: route eval requests through infermux's cost-optimized strategy so your eval spend doesn't balloon when you increase dataset size.

---
title: "Provider Setup"
description: "Step-by-step setup for OpenAI, Anthropic, Ollama, and Azure OpenAI. API key management, rate limits, and model aliases."
---

# Provider Setup

This guide walks through configuring each supported provider. For configuration schema reference, see the [Providers](/infermux/docs/providers/) documentation.

## OpenAI

**1. Get an API key.**

Sign in to [platform.openai.com](https://platform.openai.com), navigate to API Keys, and create a new secret key. Store it as an environment variable:

```bash
export OPENAI_API_KEY=sk-proj-...
```

**2. Find your rate limits.**

Your rate limits depend on your usage tier. Check them at platform.openai.com → Settings → Limits. Tier 1 (default after first payment) typically has:

- 3,500 RPM for GPT-4o
- 10,000 RPM for GPT-4o-mini
- 800,000 TPM across all models

Configure the most restrictive limit that applies to your deployment:

```yaml
providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models:
      - gpt-4o
      - gpt-4o-mini
      - gpt-3.5-turbo
      - text-embedding-3-small
      - text-embedding-3-large
    rate_limits:
      requests_per_minute: 3500
      tokens_per_minute: 800000
    timeout: 60s
```

**3. Configure model aliases (optional).**

If you want callers to use non-OpenAI model names and have them routed to OpenAI:

```yaml
    model_aliases:
      fast: gpt-4o-mini
      smart: gpt-4o
      embed: text-embedding-3-small
```

## Anthropic

**1. Get an API key.**

Sign in to [console.anthropic.com](https://console.anthropic.com), go to API Keys, and create a key:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

**2. Configure model aliases.**

Anthropic's API uses its own model names. Since callers typically send OpenAI model names, you need aliases to translate. Map every OpenAI model name your callers might use:

```yaml
providers:
  - name: anthropic
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    model_aliases:
      gpt-4o: claude-opus-4-5
      gpt-4o-mini: claude-haiku-3-5
      gpt-3.5-turbo: claude-haiku-3-5
      o1: claude-opus-4-5
    rate_limits:
      requests_per_minute: 1000
      tokens_per_minute: 400000
    timeout: 90s    # Anthropic can be slower for long completions; increase if needed
```

**3. Note on API format differences.**

infermux translates the OpenAI Messages API format to Anthropic's format transparently. There are a few differences to be aware of:

- The `system` role message is extracted and passed as Anthropic's top-level `system` parameter. If you have multiple system messages, they are concatenated.
- `logprobs` and `logit_bias` are silently dropped; Anthropic does not support them.
- Anthropic's `stop_sequences` is populated from OpenAI's `stop` parameter.

## Ollama (local)

**1. Install and start Ollama.**

Follow the instructions at [ollama.com](https://ollama.com) to install Ollama. Pull the models you want to use:

```bash
ollama pull llama3.2
ollama pull mistral
ollama pull qwen2.5-coder:7b
```

Verify Ollama is running:

```bash
curl http://localhost:11434/api/tags
```

**2. Configure infermux.**

Ollama exposes an OpenAI-compatible API at port 11434. Use the `ollama` provider type, which sets the correct base URL and disables API key requirements:

```yaml
providers:
  - name: ollama
    type: ollama
    base_url: "http://localhost:11434"
    models:
      - llama3.2
      - mistral
      - qwen2.5-coder:7b
    health_check:
      interval: 10s    # shorter interval; local service should respond quickly
      timeout: 3s
```

**3. Model aliases for seamless routing.**

To let callers use OpenAI model names and have them route to Ollama:

```yaml
    model_aliases:
      gpt-4o-mini: llama3.2
      gpt-3.5-turbo: mistral
```

**4. Running Ollama on a different machine.**

If Ollama is running on a separate machine (common in CI or when you have a dedicated GPU server), change `base_url` to the remote address:

```yaml
    base_url: "http://gpu-server.internal:11434"
```

No authentication is required unless you've configured Ollama with a token.

## Azure OpenAI

**1. Deploy a model in Azure.**

In the Azure portal, create an Azure OpenAI resource and deploy a model. Note down:

- **Resource name** (e.g., `my-openai-resource`)
- **Deployment name** (e.g., `gpt-4o-deployment`) — this is the name you gave the deployment, not the model name
- **API version** (use the latest: `2024-08-01-preview`)

```bash
export AZURE_OPENAI_API_KEY=...
```

**2. Configure infermux.**

```yaml
providers:
  - name: azure-openai
    type: azure_openai
    api_key: "${AZURE_OPENAI_API_KEY}"
    base_url: "https://my-openai-resource.openai.azure.com"
    azure_deployment: "gpt-4o-deployment"
    azure_api_version: "2024-08-01-preview"
    models:
      - gpt-4o
    rate_limits:
      requests_per_minute: 1000
      tokens_per_minute: 300000
```

**Note:** Azure OpenAI uses a single deployment per model. If you have multiple models deployed, create a separate provider entry for each deployment:

```yaml
providers:
  - name: azure-gpt4o
    type: azure_openai
    api_key: "${AZURE_OPENAI_API_KEY}"
    base_url: "https://my-openai-resource.openai.azure.com"
    azure_deployment: "gpt-4o-deployment"
    azure_api_version: "2024-08-01-preview"
    models: [gpt-4o]

  - name: azure-gpt4o-mini
    type: azure_openai
    api_key: "${AZURE_OPENAI_API_KEY}"
    base_url: "https://my-openai-resource.openai.azure.com"
    azure_deployment: "gpt-4o-mini-deployment"
    azure_api_version: "2024-08-01-preview"
    models: [gpt-4o-mini]
```

## API key management

Hardcoding API keys in `infermux.yml` is a security risk. Always use environment variable substitution:

```yaml
api_key: "${OPENAI_API_KEY}"
```

infermux resolves `${VAR}` and `$VAR` patterns at startup. If a referenced variable is not set, infermux logs a warning and leaves the value empty — which will cause auth failures when requests reach that provider. Set all required variables before starting infermux.

For production deployments, inject secrets via your platform's secret management:

```bash
# Kubernetes: mount as environment variables from a Secret
# AWS ECS: use Secrets Manager with the secrets field in task definition
# Fly.io: fly secrets set OPENAI_API_KEY=sk-...
# Docker: pass via --env-file
```

Never commit `infermux.yml` with live API keys to source control. Add it to `.gitignore` if it contains any credentials, or use a secrets-free config file with all sensitive values in environment variables.

---
title: HTTP API
description: Run evals and retrieve results over HTTP — all endpoints, request/response schemas, authentication, and daemon mode.
---

# HTTP API

matchspec includes an HTTP API for triggering eval runs and retrieving results programmatically. Use it to integrate matchspec with pipelines that can't run `go install`, or to run evals asynchronously from a service.

## Starting the server

```bash
matchspec serve --port 8090
```

The server binds to `127.0.0.1:8090` by default. To listen on all interfaces (e.g., inside a container):

```bash
matchspec serve --host 0.0.0.0 --port 8090
```

## Authentication

Protect the API with a bearer token by setting `MATCHSPEC_API_KEY`:

```bash
export MATCHSPEC_API_KEY="your-secret-token"
matchspec serve
```

All requests must include the token in the `Authorization` header:

```
Authorization: Bearer your-secret-token
```

Requests without a valid token receive `401 Unauthorized`. If `MATCHSPEC_API_KEY` is not set, authentication is disabled (all requests are accepted). Do not expose an unauthenticated server to the public internet.

---

## POST /run

Start an eval run. Returns immediately with a run ID; the run executes asynchronously.

### Request

```
POST /run
Content-Type: application/json
Authorization: Bearer <token>
```

```json
{
  "suite": "summarization",
  "tags": ["science", "ml"],
  "concurrency": 8,
  "config": "./matchspec.yml"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `suite` | string | no | Name of the suite to run. Runs all suites if omitted. |
| `tags` | array of strings | no | Filter examples by tags. |
| `concurrency` | integer | no | Override concurrency for this run. |
| `config` | string | no | Path to matchspec.yml. Uses server default if omitted. |

### Response

```json
{
  "run_id": "run-20260315-143022-a3f7",
  "status": "running",
  "suite": "summarization",
  "started_at": "2026-03-15T14:30:22Z",
  "results_url": "/results/run-20260315-143022-a3f7"
}
```

| Field | Type | Description |
|---|---|---|
| `run_id` | string | Unique identifier for this run. |
| `status` | string | `"running"`, `"passed"`, `"failed"`, `"error"` |
| `suite` | string | Name of the suite being run. |
| `started_at` | string | ISO 8601 timestamp. |
| `results_url` | string | URL to poll for results. |

### Status codes

| Code | Meaning |
|---|---|
| `202 Accepted` | Run started. |
| `400 Bad Request` | Invalid request body. |
| `401 Unauthorized` | Missing or invalid API key. |
| `404 Not Found` | Named suite not found in config. |

---

## GET /results/:id

Retrieve the status and results of a run.

### Request

```
GET /results/run-20260315-143022-a3f7
Authorization: Bearer <token>
```

### Response (running)

```json
{
  "run_id": "run-20260315-143022-a3f7",
  "status": "running",
  "suite": "summarization",
  "started_at": "2026-03-15T14:30:22Z",
  "progress": {
    "total": 120,
    "completed": 47,
    "percent": 39.2
  }
}
```

### Response (completed)

```json
{
  "run_id": "run-20260315-143022-a3f7",
  "status": "passed",
  "suite": "summarization",
  "started_at": "2026-03-15T14:30:22Z",
  "finished_at": "2026-03-15T14:31:05Z",
  "duration_seconds": 43,
  "verdict": "PASS",
  "grader_results": [
    {
      "name": "exact_match",
      "score": 0.74,
      "threshold": 0.70,
      "passed": true,
      "n": 120,
      "ci_lower": 0.651,
      "ci_upper": 0.820
    },
    {
      "name": "semantic_similarity",
      "score": 0.91,
      "threshold": 0.85,
      "passed": true,
      "n": 120,
      "ci_lower": 0.851,
      "ci_upper": 0.950
    }
  ],
  "examples": [
    {
      "id": "ex-001",
      "input": "Summarize in one sentence: ...",
      "expected": "Researchers reduced training compute by 40%.",
      "output": "Researchers cut neural net training compute by 40% using structured pruning.",
      "scores": {
        "exact_match": 0.0,
        "semantic_similarity": 0.94
      },
      "passed": true,
      "tags": ["science", "ml"]
    }
  ],
  "model_errors": 0
}
```

### Response (failed)

Same structure as completed, but with `"status": "failed"` and `"verdict": "FAIL"`. Failing graders have `"passed": false`.

### Status codes

| Code | Meaning |
|---|---|
| `200 OK` | Run found. Check `status` field for current state. |
| `401 Unauthorized` | Missing or invalid API key. |
| `404 Not Found` | Run ID not found. |

---

## GET /suites

List all suites defined in the server's config.

### Request

```
GET /suites
Authorization: Bearer <token>
```

### Response

```json
{
  "suites": [
    {
      "name": "summarization",
      "harnesses": ["summarization-v2"],
      "thresholds": {
        "overall": 0.80
      }
    },
    {
      "name": "qa",
      "harnesses": ["qa-v1"],
      "thresholds": {
        "overall": 0.85
      }
    }
  ]
}
```

---

## GET /results

List recent run results.

### Request

```
GET /results?suite=summarization&limit=10
Authorization: Bearer <token>
```

### Query parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `suite` | string | — | Filter by suite name. |
| `status` | string | — | Filter by status: `passed`, `failed`, `running`, `error`. |
| `limit` | integer | `20` | Maximum number of results to return. |
| `offset` | integer | `0` | Pagination offset. |

### Response

```json
{
  "runs": [
    {
      "run_id": "run-20260315-143022-a3f7",
      "status": "passed",
      "suite": "summarization",
      "started_at": "2026-03-15T14:30:22Z",
      "finished_at": "2026-03-15T14:31:05Z",
      "verdict": "PASS"
    }
  ],
  "total": 42,
  "limit": 10,
  "offset": 0
}
```

---

## GET /health

Liveness check. Returns 200 if the server is running.

### Request

```
GET /health
```

### Response

```json
{
  "status": "ok",
  "version": "0.3.1"
}
```

No authentication required.

---

## Running as a daemon

To run the matchspec server as a long-lived daemon in a Docker container:

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN go install github.com/greynewell/matchspec/cmd/matchspec@latest

FROM alpine:3.19
COPY --from=builder /root/go/bin/matchspec /usr/local/bin/matchspec
COPY matchspec.yml /app/matchspec.yml
COPY evals/ /app/evals/

WORKDIR /app
EXPOSE 8090
ENV MATCHSPEC_API_KEY=""
CMD ["matchspec", "serve", "--host", "0.0.0.0", "--port", "8090"]
```

Build and run:

```bash
docker build -t matchspec-server .
docker run -p 8090:8090 \
  -e MATCHSPEC_API_KEY="your-token" \
  -e OPENAI_API_KEY="sk-..." \
  matchspec-server
```

## Polling for results

Since runs are asynchronous, poll `/results/:id` until `status` is no longer `"running"`:

```bash
# Trigger a run
RUN_ID=$(curl -s -X POST http://localhost:8090/run \
  -H "Authorization: Bearer $MATCHSPEC_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"suite": "summarization"}' | jq -r .run_id)

echo "Started run: $RUN_ID"

# Poll until complete
while true; do
  STATUS=$(curl -s "http://localhost:8090/results/$RUN_ID" \
    -H "Authorization: Bearer $MATCHSPEC_API_KEY" | jq -r .status)
  echo "Status: $STATUS"
  if [ "$STATUS" != "running" ]; then break; fi
  sleep 5
done

# Check verdict
VERDICT=$(curl -s "http://localhost:8090/results/$RUN_ID" \
  -H "Authorization: Bearer $MATCHSPEC_API_KEY" | jq -r .verdict)

echo "Verdict: $VERDICT"
[ "$VERDICT" = "PASS" ] || exit 1
```

---
title: Set Up Cost and Quality Alerts
description: Define cost_spike and quality_drop alert rules, configure webhook delivery, and attach matchspec eval scores to spans so quality regressions page you before users notice.
---

# Set Up Cost and Quality Alerts

<div class="tutorial-meta">
  <span class="meta-tag">intermediate</span>
  <span class="meta-tag">20 min</span>
  <span class="meta-tag">alerts</span>
  <span class="meta-tag">webhooks</span>
</div>

This tutorial builds on a working tokentrace setup to add alert rules for cost and quality. By the end you will have:

- A `cost_spike` rule that fires when hourly spend exceeds a threshold
- A `quality_drop` rule that fires when average eval scores fall below a floor
- Both rules delivering alert payloads to a webhook
- A local webhook receiver so you can test the full delivery path

**Prerequisites:**
- Completed the [Instrument Your First Inference Call](/tokentrace/tutorials/first-trace/) tutorial, or any working tokentrace setup
- `curl` for testing webhook delivery
- Basic understanding of HTTP webhooks

---

<div class="step">
<div class="step-number">Step 1</div>

## Run a local webhook receiver

You need something listening at a URL to receive alert payloads during development. The simplest option is a small Go HTTP handler:

```go
// webhook/main.go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/alerts", func(w http.ResponseWriter, r *http.Request) {
        body, err := io.ReadAll(r.Body)
        if err != nil {
            http.Error(w, "bad request", 400)
            return
        }
        var payload map[string]any
        if err := json.Unmarshal(body, &payload); err != nil {
            fmt.Printf("received (non-JSON): %s\n", body)
        } else {
            pretty, _ := json.MarshalIndent(payload, "", "  ")
            fmt.Printf("ALERT RECEIVED:\n%s\n\n", pretty)
        }
        w.WriteHeader(http.StatusOK)
    })

    log.Println("webhook receiver listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

In a separate terminal:

```
go run webhook/main.go
```

Leave this running. You will see alert payloads appear here when rules fire.
</div>

<div class="step">
<div class="step-number">Step 2</div>

## Add a cost spike alert

Update your tracer configuration to add a `CostSpike` rule. For this tutorial, set a low threshold ($0.01) so it fires quickly during testing:

```go
tracer := tokentrace.New(tokentrace.Config{
    Transport: tokentrace.FileTransport("./traces.jsonl"),
    HTTPServer: &tokentrace.HTTPServerConfig{
        Addr: ":9090",
    },
    Alerts: []tokentrace.AlertRule{
        tokentrace.CostSpike(tokentrace.CostSpikeRule{
            Name:      "hourly-cost-spike",
            Threshold: 0.01,      // $0.01 for testing; use a realistic value in production
            Window:    time.Hour,
            Cooldown:  5 * time.Minute, // re-fire at most once every 5 minutes during testing
            MinSpans:  3,              // require at least 3 spans before evaluating
            Delivery: tokentrace.HTTPDeliveryWith(tokentrace.HTTPDeliveryOptions{
                URL:     "http://localhost:8080/alerts",
                Timeout: 5 * time.Second,
            }),
        }),
    },
})
```

<div class="callout note">
<p>In production, set <code>Threshold</code> to a value that represents genuinely unexpected spending — for example, 3x your average hourly cost. Set <code>Cooldown</code> to at least one <code>Window</code> duration to avoid alert fatigue during sustained high spend.</p>
</div>

</div>

<div class="step">
<div class="step-number">Step 3</div>

## Trigger the alert

Make several inference calls to push the cost over $0.01. Run your program in a loop:

```go
for i := 0; i < 10; i++ {
    if err := summarizeDocument(tracer, longDocument); err != nil {
        log.Printf("error on call %d: %v", i, err)
    }
    time.Sleep(200 * time.Millisecond)
}

// Flush all spans before exit
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
tracer.Flush(ctx)
```

After a few calls, you should see the alert appear in the webhook receiver terminal:

```json
ALERT RECEIVED:
{
  "alert": "hourly-cost-spike",
  "fired_at": "2026-03-15T14:30:00.000Z",
  "metric": "total_cost",
  "op": "gt",
  "value": 0.0142,
  "threshold": 0.01,
  "window": "1h",
  "span_count": 6,
  "filter": {},
  "rule_id": "alert_7f3a1c2b"
}
```

The `value` field shows the current metric value. The `threshold` shows what was configured. `span_count` tells you how many spans contributed to the metric.

Verify the current cost via the HTTP API:

```
$ curl http://localhost:9090/metrics/cost?window=1h
{
  "total_usd": 0.0142,
  "per_call_usd": 0.00237,
  "by_model": {"gpt-4o": 0.0142},
  "span_count": 6
}
```
</div>

<div class="step">
<div class="step-number">Step 4</div>

## Attach eval scores to spans

The `quality_drop` alert reads the `eval.score` attribute on spans. To use it, you need to compute a score for each model output and attach it when recording the span.

For this tutorial, we use a simple keyword-based scorer as a stand-in for a real eval. In production you would run a matchspec grader, an embedding similarity score, or an LLM-as-judge evaluation.

Add a scorer function:

```go
// scoreOutput is a placeholder for your real eval grader.
// Returns a float64 in [0, 1] representing output quality.
func scoreOutput(prompt, output string) float64 {
    // Trivial: score based on output length relative to expected
    // A real grader would use semantic similarity or LLM-as-judge
    expectedMinLen := 50
    if len(output) >= expectedMinLen {
        return 1.0
    }
    return float64(len(output)) / float64(expectedMinLen)
}
```

Update `summarizeDocument` to attach the score:

```go
func summarizeDocument(tracer *tokentrace.Tracer, document string) error {
    ctx := context.Background()
    trace := tracer.Start("summarize-document")

    start := time.Now()
    resp, err := callModel(ctx, "Summarize this document in two sentences: "+document)
    elapsed := time.Since(start)

    if err != nil {
        trace.RecordError(err)
        trace.End()
        return fmt.Errorf("model call failed: %w", err)
    }

    // Compute the eval score for this output.
    score := scoreOutput(document, resp.Content)

    trace.Record(tokentrace.Span{
        Model:        "gpt-4o",
        Provider:     "openai",
        PromptTokens: resp.Usage.PromptTokens,
        CompTokens:   resp.Usage.CompletionTokens,
        LatencyMs:    elapsed.Milliseconds(),
        Cost:         tokentrace.Cost("gpt-4o", resp.Usage),
        Status:       tokentrace.StatusOK,
        Attributes: map[string]any{
            "eval.score": score,
            "eval.suite": "summarization-v1",
        },
    })

    trace.End()
    fmt.Printf("Summary: %s (score: %.2f)\n", resp.Content, score)
    return nil
}
```
</div>

<div class="step">
<div class="step-number">Step 5</div>

## Add a quality drop alert

Add the `QualityDrop` rule to your tracer config. Again, use a high threshold ($0.90) so it fires easily during testing:

```go
Alerts: []tokentrace.AlertRule{
    tokentrace.CostSpike(tokentrace.CostSpikeRule{
        Name:      "hourly-cost-spike",
        Threshold: 0.01,
        Window:    time.Hour,
        Cooldown:  5 * time.Minute,
        MinSpans:  3,
        Delivery:  tokentrace.HTTPDelivery("http://localhost:8080/alerts"),
    }),
    tokentrace.QualityDrop(tokentrace.QualityDropRule{
        Name:      "quality-drop",
        Threshold: 0.90, // fire if average eval.score falls below 0.90
        Window:    time.Hour,
        Cooldown:  10 * time.Minute,
        MinSpans:  5,  // require at least 5 scored spans
        Delivery:  tokentrace.HTTPDelivery("http://localhost:8080/alerts"),
    }),
},
```

Simulate a quality regression by making the stub model return a short, low-quality output for some calls:

```go
// callModel stub — returns degraded output every third call
var callCount int
func callModel(ctx context.Context, prompt string) (*ModelResponse, error) {
    callCount++
    time.Sleep(600 * time.Millisecond)

    if callCount%3 == 0 {
        // Simulate a degraded (very short) response
        return &ModelResponse{
            Content: "Too short.",
            Usage:   ModelUsage{PromptTokens: len(prompt) / 4, CompletionTokens: 3},
        }, nil
    }
    return &ModelResponse{
        Content: "The document describes a fox. It is notable for its speed and color.",
        Usage:   ModelUsage{PromptTokens: len(prompt) / 4, CompletionTokens: 18},
    }, nil
}
```

Run the loop again. After 5+ spans are recorded, the alert engine evaluates the `quality_drop` rule. Because every third call returns `"Too short."` (which scores ~0.18 with our scorer), the average score will fall below 0.90, and you should see:

```json
ALERT RECEIVED:
{
  "alert": "quality-drop",
  "fired_at": "2026-03-15T14:35:00.000Z",
  "metric": "quality_score",
  "op": "lt",
  "value": 0.72,
  "threshold": 0.90,
  "window": "1h",
  "span_count": 9,
  "filter": {},
  "rule_id": "alert_b2c3d4e5"
}
```
</div>

<div class="step">
<div class="step-number">Step 6</div>

## Silence a rule

During an incident (or while testing), you may want to silence a rule temporarily without changing code. Use the HTTP API:

```
$ curl -X POST http://localhost:9090/alerts/quality-drop/silence \
  -H "Content-Type: application/json" \
  -d '{"duration": "15m"}'

{"silenced_until": "2026-03-15T14:50:00.000Z"}
```

The rule will not fire again until the silence expires or you restart the process.

Check the current rule state:

```
$ curl http://localhost:9090/alerts | jq '.[] | {name, firing, silenced, current_value}'
[
  {"name": "hourly-cost-spike", "firing": false, "silenced": false, "current_value": 0.0142},
  {"name": "quality-drop",      "firing": true,  "silenced": true,  "current_value": 0.72}
]
```
</div>

<div class="step">
<div class="step-number">Step 7</div>

## Move config to tokentrace.yml

For production, move the alert config out of Go code and into `tokentrace.yml`. This lets you change thresholds and delivery URLs without recompiling:

```yaml
# tokentrace.yml
transport:
  type: file
  file:
    path: ./traces.jsonl
    rotate: true

http_server:
  enabled: true
  addr: ":9090"

alerts:
  - name: hourly-cost-spike
    metric: total_cost
    op: gt
    threshold: 5.00
    window: 1h
    cooldown: 4h
    min_spans: 10
    delivery:
      type: http
      url: "${ALERT_WEBHOOK_URL}"

  - name: quality-drop
    metric: quality_score
    op: lt
    threshold: 0.75
    window: 1h
    cooldown: 1h
    min_spans: 20
    delivery:
      type: http
      url: "${ALERT_WEBHOOK_URL}"
```

Update `main.go` to load from file:

```go
tracer, err := tokentrace.NewFromFile("./tokentrace.yml")
if err != nil {
    log.Fatal(err)
}
```

Set the environment variable:

```
export ALERT_WEBHOOK_URL=https://hooks.example.com/alerts
```

Now threshold and URL changes are a config edit, not a deploy.
</div>

<div class="step">
<div class="step-number">Next steps</div>

## What to do next

**Replace the stub scorer with a real eval.** In production, use [matchspec](/matchspec/) to define a grader and run it on each model output. Attach the matchspec score as `eval.score` in the span attributes. The `quality_drop` alert will then reflect real quality rather than a heuristic.

**Add a latency regression alert.** Add a `LatencyRegression` rule alongside the cost and quality rules to complete your observability baseline:

```go
tokentrace.LatencyRegression(tokentrace.LatencyRegressionRule{
    Name:       "p95-latency-regression",
    Percentile: 95,
    Threshold:  3000,
    Window:     30 * time.Minute,
    Delivery:   tokentrace.HTTPDelivery("http://localhost:8080/alerts"),
})
```

**Build a Grafana dashboard.** See the [Grafana Integration guide](/tokentrace/docs/grafana/) to visualize cost, latency, and quality together so you have the full picture before an alert fires.

**Instrument an agent loop.** If you're running multi-step agents, see [Instrumenting an Agent Loop](/tokentrace/docs/agent-instrumentation/) to add step-level cost tracking and a budget hard stop.

</div>

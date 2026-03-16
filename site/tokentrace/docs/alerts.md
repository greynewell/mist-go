---
title: Alerts
description: tokentrace alert rules — definition, built-in rule types, delivery channels, silencing, cooldowns, and integrating with matchspec eval scores.
---

# Alerts

Alert rules tell tokentrace to notify you when a metric crosses a threshold. Rules are evaluated on a timer against the metrics aggregator. When a rule fires, a delivery channel sends an alert payload to a webhook, stdout, or a custom handler. Rules support cooldowns so you aren't paged repeatedly for a sustained regression.

## Alert rule definition

```go
type AlertRule struct {
    // Name identifies this rule in alert payloads and logs.
    Name string

    // Metric is the built-in or custom metric to evaluate.
    // Examples: "total_cost", "latency_p95", "quality_score", "error_rate"
    Metric string

    // Op is the comparison operator.
    Op AlertOp

    // Threshold is the value to compare against.
    Threshold float64

    // Window is the time window over which the metric is computed.
    // Example: time.Hour evaluates the metric over the last 60 minutes.
    Window time.Duration

    // EvalInterval is how often the rule is evaluated.
    // Defaults to Window / 10, minimum 30 seconds.
    EvalInterval time.Duration

    // Cooldown is the minimum time between successive firings of this rule.
    // After a rule fires, it will not fire again until Cooldown has elapsed.
    // Defaults to Window.
    Cooldown time.Duration

    // MinSpans is the minimum number of spans in the window required to evaluate.
    // If fewer spans exist, the rule is skipped. Prevents false positives at startup.
    MinSpans int

    // Filter restricts the rule to spans matching an attribute key/value pair.
    // Example: Filter{"model": "gpt-4o"} only evaluates spans from gpt-4o.
    Filter map[string]string

    // Delivery is the channel that receives alert payloads when the rule fires.
    Delivery AlertDelivery

    // Silenced disables the rule without removing it from config.
    Silenced bool
}
```

### Comparison operators

```go
const (
    OpGreaterThan        AlertOp = "gt"
    OpGreaterThanOrEqual AlertOp = "gte"
    OpLessThan           AlertOp = "lt"
    OpLessThanOrEqual    AlertOp = "lte"
)
```

## Built-in rule types

tokentrace provides named constructors for common alert patterns:

### CostSpike

Fires when total cost in the window exceeds a dollar threshold.

```go
tokentrace.CostSpike(tokentrace.CostSpikeRule{
    Name:      "hourly-cost-spike",
    Threshold: 10.00,        // fire if total cost exceeds $10 in the window
    Window:    time.Hour,
    Delivery:  tokentrace.HTTPDelivery("https://hooks.example.com/alerts"),
})
```

### LatencyRegression

Fires when p95 latency exceeds a millisecond threshold.

```go
tokentrace.LatencyRegression(tokentrace.LatencyRegressionRule{
    Name:      "p95-latency-regression",
    Percentile: 95,
    Threshold:  3000,        // fire if p95 latency exceeds 3000 ms
    Window:     30 * time.Minute,
    Delivery:   tokentrace.HTTPDelivery("https://hooks.example.com/alerts"),
})
```

### QualityDrop

Fires when average `eval.score` falls below a threshold.

```go
tokentrace.QualityDrop(tokentrace.QualityDropRule{
    Name:      "quality-drop",
    Threshold: 0.75,         // fire if average eval.score falls below 0.75
    Window:    time.Hour,
    MinSpans:  20,           // require at least 20 scored spans in the window
    Delivery:  tokentrace.HTTPDelivery("https://hooks.example.com/alerts"),
})
```

### ErrorRateSpike

Fires when error rate (non-OK spans / total spans) exceeds a fraction.

```go
tokentrace.ErrorRateSpike(tokentrace.ErrorRateSpikeRule{
    Name:      "error-rate-spike",
    Threshold: 0.05,         // fire if more than 5% of calls are failing
    Window:    15 * time.Minute,
    MinSpans:  10,
    Delivery:  tokentrace.StdoutDelivery(),
})
```

## Alert payload

When a rule fires, the delivery channel receives a JSON payload:

```json
{
  "alert":     "hourly-cost-spike",
  "fired_at":  "2026-03-15T15:00:00.000Z",
  "metric":    "total_cost",
  "op":        "gt",
  "value":     12.47,
  "threshold": 10.00,
  "window":    "1h",
  "span_count": 312,
  "filter":    {},
  "rule_id":   "alert_7f3a1c2b"
}
```

## Delivery channels

### StdoutDelivery

Prints the alert payload as JSON to stdout. Suitable for development and debugging.

```go
tokentrace.StdoutDelivery()
```

### HTTPDelivery

POSTs the alert payload to a webhook URL. Suitable for Slack, PagerDuty, or any custom alert receiver.

```go
tokentrace.HTTPDelivery("https://hooks.example.com/alerts")
```

With options:

```go
tokentrace.HTTPDeliveryWith(tokentrace.HTTPDeliveryOptions{
    URL:     "https://hooks.example.com/alerts",
    Timeout: 5 * time.Second,
    Headers: map[string]string{
        "Authorization": "Bearer " + os.Getenv("ALERT_TOKEN"),
    },
    MaxRetries: 2,
})
```

### Custom delivery

Implement `AlertDelivery` to send alerts anywhere:

```go
type AlertDelivery interface {
    Deliver(ctx context.Context, alert Alert) error
}
```

## Silencing and cooldowns

**Cooldown** prevents a rule from firing repeatedly during a sustained regression. After a rule fires, it will not fire again until `Cooldown` has elapsed. The default cooldown equals the rule's `Window` — a rule with a 1h window will fire at most once per hour.

```go
tokentrace.AlertRule{
    Name:     "hourly-cost-spike",
    Cooldown: 4 * time.Hour, // suppress re-fires for 4 hours after a firing
    // ...
}
```

**Silencing** disables a rule without removing it. Set `Silenced: true` to mute a rule temporarily. You can also silence rules via the HTTP API:

```
POST /alerts/{rule_name}/silence
{"duration": "2h"}
```

This silences the rule for 2 hours without requiring a code change or redeploy.

**Resolving** — tokentrace does not currently emit explicit "resolved" notifications. If your webhook system requires a resolved event, you can poll `GET /alerts/{rule_name}/status` from your own tooling to check whether a rule's current value has returned below threshold.

## Integrating alert rules with matchspec eval scores

The `quality_drop` rule reads the `eval.score` attribute on spans. To use it, attach eval scores from [matchspec](/matchspec/) to each span after running evaluation:

```go
// Run the inference call
start := time.Now()
resp, _ := callModel(prompt)
elapsed := time.Since(start)

// Run the eval (or receive a pre-computed score from your eval pipeline)
score := runEval(prompt, resp.Content) // returns float64 in [0, 1]

// Record the span with the eval score attached
trace.Record(tokentrace.Span{
    Model:        "gpt-4o",
    PromptTokens: resp.Usage.PromptTokens,
    CompTokens:   resp.Usage.CompletionTokens,
    LatencyMs:    elapsed.Milliseconds(),
    Cost:         tokentrace.Cost("gpt-4o", resp.Usage),
    Status:       tokentrace.StatusOK,
    Attributes: map[string]any{
        "eval.score": score,
        "eval.suite": "summarization-v2",
    },
})
```

With eval scores on spans, the `quality_drop` alert rule gives you a real-time quality signal tied directly to inference traffic — not just offline eval runs.

## Next steps

- [Metrics](/tokentrace/docs/metrics/) — The metrics that alert rules evaluate against.
- [HTTP API](/tokentrace/docs/http-api/) — Manage alert rules via API, including POST /alerts and silence endpoints.
- [Configuration](/tokentrace/docs/config/) — Define alert rules in `tokentrace.yml`.

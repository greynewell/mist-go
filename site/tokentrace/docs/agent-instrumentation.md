---
title: Instrumenting an Agent Loop
description: Add step-level tracing to a multi-step agent — one span per step, cost accumulation, detecting context rot through latency trends, and setting a budget with hard stop.
---

# Instrumenting an Agent Loop

Multi-step agent loops are where token observability matters most. A single agent run can make dozens of inference calls, each adding to the prompt context. Costs compound. Latency trends upward as the context window fills. Token budgets can be exceeded silently. This guide shows how to instrument a realistic agent loop with tokentrace so you can see all of this happening in real time.

## What we're building

A simple agent loop that: reads a problem description, calls a model to plan steps, executes each step (each step calls the model), and terminates when the model declares completion or when a budget is hit.

We will add:
- One span per inference call, nested under a root trace
- Cost accumulation with a hard budget stop
- Latency trend detection (rising latency often signals context rot)
- Custom attributes for step number and step name
- A quality score on the final output

## Setting up the tracer

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/greynewell/tokentrace"
)

var tracer = tokentrace.New(tokentrace.Config{
    Transport: tokentrace.FileTransport("./traces.jsonl"),
    HTTPServer: &tokentrace.HTTPServerConfig{
        Addr: ":9090",
    },
    Alerts: []tokentrace.AlertRule{
        tokentrace.QualityDrop(tokentrace.QualityDropRule{
            Name:      "agent-quality-drop",
            Threshold: 0.70,
            Window:    time.Hour,
            MinSpans:  5,
            Delivery:  tokentrace.StdoutDelivery(),
        }),
    },
})
```

## The instrumented agent loop

```go
type AgentResult struct {
    Answer string
    Steps  int
    Cost   float64
}

func runAgent(ctx context.Context, problem string) (*AgentResult, error) {
    // Start a trace for this agent run.
    // All spans recorded inside this function share the same TraceID.
    trace := tracer.Start("agent-run")
    budget := tokentrace.NewBudget(0.25) // $0.25 hard stop

    var steps []string
    var totalCost float64
    var prevLatency int64

    // Step 1: planning call
    planSpan, plan, err := callStep(ctx, trace, "plan", 0, buildPlanPrompt(problem))
    if err != nil {
        trace.End()
        return nil, fmt.Errorf("planning failed: %w", err)
    }
    if err := budget.Add(planSpan); err != nil {
        trace.End()
        return nil, fmt.Errorf("budget exceeded during planning: %w", err)
    }
    totalCost += planSpan.Cost
    prevLatency = planSpan.LatencyMs

    // Execute each planned step
    for i, step := range parsePlan(plan) {
        span, output, err := callStep(ctx, trace, step.Name, i+1, buildStepPrompt(problem, steps, step))
        if err != nil {
            // Record the error span but continue — non-fatal step failures
            // are expected in some agent designs
            trace.RecordError(err)
            continue
        }

        // Detect context rot: latency increasing faster than expected
        // as the context window fills up
        if i > 0 && span.LatencyMs > prevLatency*2 {
            // Log a warning as a span attribute so it's queryable
            span.Attributes["warning"] = "latency_doubled"
            span.Attributes["prev_latency_ms"] = prevLatency
        }
        prevLatency = span.LatencyMs

        if err := budget.Add(span); err != nil {
            // Budget exceeded mid-run — record and stop cleanly
            span.Attributes["budget_stop"] = true
            trace.Record(span)
            trace.End()
            return &AgentResult{
                Answer: summarizePartial(steps),
                Steps:  i + 1,
                Cost:   budget.Spent(),
            }, fmt.Errorf("budget exceeded at step %d: spent $%.4f of $0.25", i+1, budget.Spent())
        }

        trace.Record(span)
        totalCost += span.Cost
        steps = append(steps, output)

        if isComplete(output) {
            break
        }
    }

    // Final synthesis call
    synthSpan, answer, err := callStep(ctx, trace, "synthesize", len(steps)+1,
        buildSynthesisPrompt(problem, steps))
    if err != nil {
        trace.End()
        return nil, fmt.Errorf("synthesis failed: %w", err)
    }

    // Attach an eval score to the synthesis span
    // In production, run your matchspec grader here
    evalScore := scoreAnswer(problem, answer)
    synthSpan.Attributes["eval.score"] = evalScore
    synthSpan.Attributes["eval.suite"] = "agent-quality-v1"
    trace.Record(synthSpan)

    trace.End()

    return &AgentResult{
        Answer: answer,
        Steps:  len(steps) + 2, // plan + steps + synthesis
        Cost:   totalCost + synthSpan.Cost,
    }, nil
}
```

## The callStep helper

```go
func callStep(
    ctx context.Context,
    trace *tokentrace.Trace,
    stepName string,
    stepNum int,
    prompt string,
) (tokentrace.Span, string, error) {
    start := time.Now()
    resp, err := callModel(ctx, prompt)
    elapsed := time.Since(start)

    status := tokentrace.StatusOK
    errStr := ""
    if err != nil {
        status = tokentrace.StatusError
        errStr = err.Error()
    }

    span := tokentrace.Span{
        Model:        "gpt-4o",
        Provider:     "openai",
        PromptTokens: resp.Usage.PromptTokens,
        CompTokens:   resp.Usage.CompletionTokens,
        LatencyMs:    elapsed.Milliseconds(),
        Cost:         tokentrace.Cost("gpt-4o", resp.Usage),
        Status:       status,
        Error:        errStr,
        Attributes: map[string]any{
            "step":      stepNum,
            "step_name": stepName,
            "agent_run": trace.ID(), // redundant with TraceID, but queryable by attr
        },
    }

    // Note: we return the span without recording it so the caller can
    // attach additional attributes (budget_stop, warning, eval.score)
    // before calling trace.Record(span).

    if err != nil {
        return span, "", err
    }
    return span, resp.Content, nil
}
```

## Querying the results

After a few agent runs, query the HTTP API to see how the costs and latencies look:

```
$ curl "http://localhost:9090/metrics/cost?window=24h"
{
  "total_usd": 3.84,
  "per_call_usd": 0.0128,
  "by_model": {"gpt-4o": 3.84},
  "span_count": 300
}

$ curl "http://localhost:9090/metrics/latency?window=24h&groupby=attr.step_name"
{
  "by_step_name": {
    "plan":       {"p50_ms": 710,  "p95_ms": 1240},
    "synthesize": {"p50_ms": 1840, "p95_ms": 4200},
    "execute":    {"p50_ms": 920,  "p95_ms": 2100}
  }
}
```

Synthesis calls are the latency bottleneck — not surprising since they see the entire accumulated context. If you see synthesis latency growing over time, the context window is filling faster than expected.

## Detecting context rot

Context rot is the gradual performance degradation that happens as a language model's context grows too large. Symptoms: rising latency (the model processes more tokens per call), rising completion token counts (the model repeats itself more), and eventually, quality degradation.

With tokentrace, you can detect context rot by querying latency and token trends over time:

```
$ curl "http://localhost:9090/traces?attr.step_name=synthesize&since=2026-03-15T00:00:00Z"
```

Look at the `prompt_tokens` field across synthesis spans over time. If it's growing run-to-run without a corresponding increase in problem complexity, you are accumulating context you do not need.

Alternatively, set an alert:

```go
tokentrace.AlertRule{
    Name:      "synthesis-latency",
    Metric:    "latency_p95",
    Op:        tokentrace.OpGreaterThan,
    Threshold: 5000,
    Window:    30 * time.Minute,
    Filter:    map[string]string{"step_name": "synthesize"},
    Delivery:  tokentrace.StdoutDelivery(),
}
```

## Setting a hard budget stop

The `Budget` helper from the [Go API](/tokentrace/docs/go-api/) enforces a cost ceiling. `budget.Add(span)` returns a non-nil error the moment accumulated cost exceeds `maxUSD`. The error message includes the amount spent and the limit.

The pattern above (`budget.Add(span)` checked before recording) means that a budget-exceeded span is still recorded — it just ends the loop. The span carries `"budget_stop": true` in its attributes, so you can filter for these spans in the HTTP API to understand how often your agents are hitting the ceiling.

If you want to prevent any span that would exceed the budget from executing in the first place, check `budget.Remaining()` before making the model call:

```go
estimatedCost := tokentrace.EstimateCost("gpt-4o", len(prompt)/4, 256)
if estimatedCost > budget.Remaining() {
    return fmt.Errorf("estimated cost $%.4f would exceed remaining budget $%.4f",
        estimatedCost, budget.Remaining())
}
```

`tokentrace.EstimateCost` computes a rough cost estimate given a model, an estimated prompt token count, and an assumed completion length. Token count estimation from character count is approximate — use 4 characters per token as a conservative rule of thumb.

## Next steps

- [Grafana Integration](/tokentrace/docs/grafana/) — Build dashboards showing agent cost and latency over time.
- [Alerts](/tokentrace/docs/alerts/) — Set up quality drop and budget alert rules.
- [Traces & Spans](/tokentrace/docs/traces/) — Full Span field reference including ParentSpanID for trace hierarchies.

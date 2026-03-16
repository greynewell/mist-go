---
title: Grafana Integration
description: Export tokentrace metrics to Prometheus, import the pre-built dashboard JSON, and customize panels for cost over time, latency heatmap, model usage breakdown, and alert history.
---

# Grafana Integration

tokentrace exports all metrics in Prometheus text format. This guide walks through configuring Prometheus to scrape tokentrace, importing the pre-built Grafana dashboard, and building custom panels.

## Prerequisites

- tokentrace HTTP server enabled (see [Installation](/tokentrace/docs/installation/))
- Prometheus 2.x or later
- Grafana 10.x or later

## 1. Enable the Prometheus endpoint

In `tokentrace.yml`:

```yaml
http_server:
  enabled: true
  addr: ":9090"
  prometheus_path: /metrics/prometheus
```

Verify it works:

```
$ curl http://localhost:9090/metrics/prometheus | head -30
# HELP tokentrace_cost_total Total cost in USD.
# TYPE tokentrace_cost_total counter
tokentrace_cost_total{model="gpt-4o",provider="openai",caller=""} 7.91
# HELP tokentrace_spans_total Total spans recorded.
# TYPE tokentrace_spans_total counter
tokentrace_spans_total{status="ok",model="gpt-4o",provider="openai"} 312
...
```

## 2. Configure Prometheus to scrape tokentrace

Add a scrape job to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: tokentrace
    static_configs:
      - targets:
          - localhost:9090
    metrics_path: /metrics/prometheus
    scrape_interval: 15s
```

If tokentrace requires authentication:

```yaml
scrape_configs:
  - job_name: tokentrace
    static_configs:
      - targets:
          - localhost:9090
    metrics_path: /metrics/prometheus
    authorization:
      credentials: "${TOKENTRACE_API_TOKEN}"
```

Reload Prometheus and confirm the target is up:

```
$ curl http://localhost:9091/targets | grep tokentrace
```

## 3. Import the pre-built dashboard

tokentrace ships a dashboard JSON in the repository at `dashboards/grafana.json`. Import it into Grafana:

1. Open Grafana → Dashboards → Import
2. Upload `dashboards/grafana.json` or paste its contents
3. Select your Prometheus data source
4. Click Import

The dashboard includes five pre-configured panels:

**Cost over time** — A time-series panel showing `tokentrace_cost_total` as a rate (cost per minute), with one line per model. Useful for spotting when a deployment change caused a sudden cost increase.

**Latency heatmap** — A heatmap of the `tokentrace_latency_ms` histogram. Each row is a latency bucket (100 ms, 500 ms, 1000 ms, etc.); each column is a time bucket. Dense rows at the top of the chart indicate latency concentration in that range. A heatmap makes p95/p99 movements more visible than a simple percentile gauge.

**Model usage breakdown** — A stacked bar chart showing span count by model over time. Use this to verify that traffic is routing to the correct model, or to see the impact of a model migration.

**Quality score over time** — A time-series of `tokentrace_quality_score` (average eval.score per scrape interval). Only visible if you are attaching `eval.score` attributes to spans. The panel includes a threshold line at 0.75 — adjust this to match your `quality_drop` alert threshold.

**Alert history** — A table panel showing recent `tokentrace_alert_firings_total` increments. Each row shows the rule name, the time it fired, and the metric value at firing. This panel requires the Grafana Loki data source if you want full alert payload details; otherwise it shows only the counter-derived timing.

## 4. Key PromQL queries

Use these queries to build custom panels:

### Hourly cost by model

```promql
increase(tokentrace_cost_total[1h])
```

Break out by model:

```promql
increase(tokentrace_cost_total[1h]) by (model)
```

### Cost rate (USD/min)

```promql
rate(tokentrace_cost_total[5m]) * 60
```

### p95 latency (from histogram)

```promql
histogram_quantile(0.95, rate(tokentrace_latency_ms_bucket[5m]))
```

### Error rate

```promql
rate(tokentrace_spans_total{status="error"}[5m])
/
rate(tokentrace_spans_total[5m])
```

### Quality score (requires eval.score attributes on spans)

```promql
tokentrace_quality_score
```

### Token consumption rate

```promql
rate(tokentrace_tokens_total[5m]) by (model)
```

### Cost per span

```promql
rate(tokentrace_cost_total[5m])
/
rate(tokentrace_spans_total{status="ok"}[5m])
```

## 5. Setting up Grafana alerts

You can define Grafana alert rules against tokentrace metrics as an alternative to (or in addition to) tokentrace's built-in alert rules. Grafana alerts are useful when you want alert conditions visible in the same interface as your dashboards.

Example: alert when p95 latency exceeds 3000 ms over a 10-minute window:

1. Open the Latency panel → Edit → Alert tab
2. Set condition: `histogram_quantile(0.95, rate(tokentrace_latency_ms_bucket[10m])) > 3000`
3. Set evaluation interval: every 1 minute
4. Set pending period: 5 minutes (avoid transient spikes)
5. Configure notification channel (Slack, PagerDuty, etc.)

## 6. Customizing the dashboard

### Add a cost attribution panel

To see cost by caller (useful in multi-service deployments):

1. Add a new panel → Time series
2. Query: `increase(tokentrace_cost_total[1h]) by (caller)`
3. Set legend: `{{caller}}`
4. Title: Cost by caller (1h)

### Add a prompt length trend

Rising prompt token p95 often signals context accumulation in agent loops:

1. Add a new panel → Time series
2. Query: `histogram_quantile(0.95, rate(tokentrace_prompt_tokens_bucket[10m]))`
3. Title: Prompt token p95
4. Add threshold line at your context window limit (e.g., 128000 for GPT-4o)

### Add a quality degradation indicator

To see when quality drops correlate with model updates or prompt changes:

1. Add a new panel → Stat
2. Query: `tokentrace_quality_score`
3. Thresholds: green above 0.85, yellow 0.75–0.85, red below 0.75
4. Display: Last value

## Next steps

- [Metrics Reference](/tokentrace/docs/metrics-reference/) — Complete list of Prometheus metric names and labels.
- [Alerts](/tokentrace/docs/alerts/) — tokentrace's built-in alert rules for cost, latency, and quality.
- [HTTP API](/tokentrace/docs/http-api/) — Query metrics directly without Prometheus.

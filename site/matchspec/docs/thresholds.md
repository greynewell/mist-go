---
title: Thresholds
description: Set pass/fail thresholds, configure confidence intervals, and enforce minimum sample sizes for statistical validity.
---

# Thresholds

Thresholds are the enforcement layer in matchspec. A threshold defines the minimum pass rate required before a suite reports success. When a grader's pass rate falls below its threshold, the suite fails and `matchspec run` exits non-zero.

## Per-grader thresholds

Set a threshold directly on each grader in your harness config:

```yaml
graders:
  - type: exact_match
    name: exact_match
    threshold: 0.90

  - type: semantic_similarity
    name: semantic_similarity
    threshold: 0.82
```

A grader's threshold is the minimum fraction of examples that must score 1.0 (or above, for float-scored graders) for that grader to pass. If either grader fails, the harness fails regardless of the other grader's score.

## Suite-level thresholds

The `thresholds` block in `matchspec.yml` sets defaults that apply when a harness doesn't specify its own:

```yaml
# matchspec.yml
suites:
  - name: production-gate
    harnesses:
      - ./evals/summarization/harness.yml
      - ./evals/qa/harness.yml
    thresholds:
      overall: 0.80         # applies if no per-grader threshold is set
      exact_match: 0.85     # per-grader-name override
      semantic_similarity: 0.78
```

Resolution order:
1. Per-grader threshold set in the harness file (`graders[].threshold`)
2. Per-grader-name threshold set in the suite (`thresholds.<grader_name>`)
3. Suite `overall` threshold
4. Default: 1.0 (all examples must pass — intentionally strict to catch misconfiguration)

## Statistical confidence intervals

A pass rate of 0.74 on 10 examples is not the same as a pass rate of 0.74 on 500 examples. With 10 examples, the true pass rate could easily be anywhere from 0.45 to 0.92. matchspec computes **Wilson score confidence intervals** to give you this context.

Enable confidence intervals in your suite config:

```yaml
# matchspec.yml
suites:
  - name: production-gate
    harnesses:
      - ./evals/summarization/harness.yml
    thresholds:
      overall: 0.80
    statistics:
      confidence_level: 0.95   # 95% confidence interval (default)
      use_lower_bound: true     # apply threshold to lower CI bound, not point estimate
```

With `use_lower_bound: true`, the threshold comparison uses the lower bound of the confidence interval instead of the raw pass rate. This means a suite will only pass when the pass rate is high enough that we can be confident (at the specified level) that the true rate is above the threshold.

Output with confidence intervals enabled:

```
suite: production-gate
─────────────────────────────────────────────────────────────
                    score  ci_lower  ci_upper  threshold  status
exact_match         0.830     0.783     0.870      0.800    ✓
semantic_similarity 0.910     0.871     0.940      0.850    ✓
─────────────────────────────────────────────────────────────
overall             PASS  (n=120, 95% CI)
```

### Wilson score interval

matchspec uses the Wilson score interval (also called the Wilson interval), which is more accurate than the naive normal approximation for proportions, especially at extreme values or with small sample sizes. The formula for the lower bound at confidence level `z` is:

```
lower = (p + z²/2n - z√(p(1-p)/n + z²/4n²)) / (1 + z²/n)
```

where `p` is the observed pass rate, `n` is the sample size, and `z` is the z-score corresponding to the desired confidence level (1.645 for 90%, 1.960 for 95%, 2.576 for 99%).

## Minimum sample sizes

You can require a minimum number of examples before a threshold is enforced. This prevents false passes when a tagged subset has very few examples:

```yaml
statistics:
  min_sample_size: 30     # refuse to evaluate threshold if n < 30
  min_sample_action: warn # "warn" or "fail" (default: "warn")
```

With `min_sample_action: warn`, running a suite with fewer than 30 examples prints a warning but does not fail:

```
WARNING: exact_match scored on 12 examples (min_sample_size: 30).
         Pass rate may not be statistically meaningful.
         exact_match  0.92  ✓  (≥0.80) [low confidence — n=12]
```

With `min_sample_action: fail`, the suite fails immediately if any harness has fewer than the minimum examples:

```
ERROR: semantic_similarity: only 12 examples match filters (min_sample_size: 30).
       Increase dataset size or lower min_sample_size to proceed.
```

## What happens on failure

When a suite fails:

1. **Exit code** — `matchspec run` exits with code 1. Exit code 0 means all thresholds passed. This is the primary integration point for CI/CD systems.

2. **Report** — A human-readable report is printed to stdout showing which graders failed and by how much:

```
suite: production-gate
─────────────────────────────────────────────────
exact_match          0.72  ✗  (≥0.80)  DELTA: -0.08
semantic_similarity  0.91  ✓  (≥0.85)
─────────────────────────────────────────────────
overall              FAIL

Failed graders: exact_match
  Pass rate 0.72 is below threshold 0.80 (delta: -0.08).
  Failing examples:
    ex-003: expected "Paris", got "The answer is Paris, France."
    ex-007: expected "4", got "Four"
    ex-011: expected "blue", got "The color blue"
    ... and 9 more. Run with --show-all-failures to see every failing example.
```

3. **JSON results** — Full results are written to `.matchspec/results/` regardless of pass or fail. The JSON file includes per-example scores for debugging.

## Threshold configuration reference

All fields in the `statistics` block:

| Field | Type | Default | Description |
|---|---|---|---|
| `confidence_level` | float | `0.95` | Confidence level for Wilson score interval. Common values: 0.90, 0.95, 0.99. |
| `use_lower_bound` | bool | `false` | If true, compare threshold against CI lower bound instead of point estimate. |
| `min_sample_size` | integer | `0` | Minimum number of examples required. 0 means no minimum. |
| `min_sample_action` | string | `"warn"` | What to do when sample size is below minimum: `"warn"` or `"fail"`. |

## Choosing thresholds

Setting thresholds requires judgment about what failure rate is acceptable for your use case. Some guidelines:

- **Start permissive, tighten over time**: Start with a threshold 10–15 percentage points below your current pass rate. Once your eval pipeline is running reliably in CI, tighten the threshold toward your actual performance.
- **Use `exact_match` conservatively**: A 0.90 exact_match threshold is very strict. Consider 0.70–0.80 as a starting point for most classification tasks.
- **Use `semantic_similarity` more aggressively**: 0.82–0.88 is a reasonable range for generation tasks where paraphrase is acceptable.
- **Set `min_sample_size` early**: Require at least 30 examples per harness before treating pass rates as meaningful. This prevents CI from passing on a 5-example smoke test.

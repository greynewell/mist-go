---
layout: tutorial-layout.njk
tool: matchspec
title: Gate Deployments with matchspec
description: Set up a GitHub Actions workflow that runs your eval suite on every PR, fails the build on regression, and posts results as a comment.
difficulty: intermediate
time: "20 minutes"
---

# Gate Deployments with matchspec

<div class="tutorial-meta">
  <span class="meta-tag">intermediate</span>
  <span>20 minutes</span>
</div>

Eval results are only useful if they block bad deployments. This tutorial sets up a complete GitHub Actions workflow that:

1. Runs your eval suite on every pull request
2. Fails the build when pass rate drops below threshold
3. Posts a markdown summary of results as a PR comment
4. Updates the comment on subsequent pushes rather than creating duplicates

## Prerequisites

- A GitHub repository with a matchspec project (see [Run Your First Eval Suite](/matchspec/tutorials/first-eval/) if you need one)
- An OpenAI API key (or substitute your own LLM provider)
- Basic familiarity with GitHub Actions

---

<div class="step">
<div class="step-number">Step 1</div>

## Store your API key as a secret

Go to your repository on GitHub: `Settings → Secrets and variables → Actions → New repository secret`.

Add a secret named `OPENAI_API_KEY` with your API key as the value.

matchspec reads this at eval time via the `api_key_env` field in your harness config:

```yaml
model:
  type: http
  endpoint: "https://api.openai.com/v1/chat/completions"
  api_key_env: "OPENAI_API_KEY"
```

Never commit API keys to your repository. The `api_key_env` pattern reads the value from the environment at runtime.

</div>

<div class="step">
<div class="step-number">Step 2</div>

## Update your harness to use a real model

If you completed the first-eval tutorial, your harness uses a stub command model. Update `evals/qa/harness.yml` to call an actual LLM:

```yaml
version: 1
name: qa-basic
description: "Factual QA eval suite."
dataset: ./dataset.yml

model:
  type: http
  endpoint: "https://api.openai.com/v1/chat/completions"
  api_key_env: "OPENAI_API_KEY"
  request_template: |
    {
      "model": "gpt-4o-mini",
      "messages": [
        {
          "role": "system",
          "content": "Answer questions concisely. Reply with only the answer, no explanation."
        },
        {
          "role": "user",
          "content": "{{input}}"
        }
      ],
      "max_tokens": 50,
      "temperature": 0
    }
  response_path: "choices[0].message.content"

graders:
  - type: exact_match
    name: exact_match
    threshold: 0.70
    config:
      trim_whitespace: true
      case_sensitive: false

  - type: semantic_similarity
    name: semantic_similarity
    threshold: 0.85
    config:
      embedding_endpoint: "https://api.openai.com/v1/embeddings"
      model: "text-embedding-3-small"
      api_key_env: "OPENAI_API_KEY"

concurrency: 4
timeout_seconds: 30
retries: 2
```

Two graders: `exact_match` catches cases where the model returns the exact string, and `semantic_similarity` catches cases where the answer is semantically correct but phrased differently. Both must pass.

</div>

<div class="step">
<div class="step-number">Step 3</div>

## Create the workflow file

Create `.github/workflows/eval.yml`:

```yaml
name: Eval Gate

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

permissions:
  pull-requests: write
  contents: read

jobs:
  eval:
    name: Run Eval Suite
    runs-on: ubuntu-latest
    timeout-minutes: 15

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - name: Cache Go modules
        uses: actions/cache@v4
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-

      - name: Install matchspec
        run: go install github.com/greynewell/matchspec/cmd/matchspec@latest

      - name: Validate config
        run: matchspec validate

      - name: Run evals
        id: eval
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: |
          set +e
          matchspec run \
            --output json \
            --output-dir ./eval-results
          echo "exit_code=$?" >> $GITHUB_OUTPUT
          set -e

      - name: Generate markdown report
        id: report
        if: github.event_name == 'pull_request'
        run: |
          RESULTS_FILE=$(ls ./eval-results/*.json | sort -r | head -1)
          if [ -z "$RESULTS_FILE" ]; then
            echo "report=No eval results found." >> $GITHUB_OUTPUT
            exit 0
          fi
          REPORT=$(matchspec report --format markdown "$RESULTS_FILE")
          {
            echo "report<<MATCHSPEC_EOF"
            echo "$REPORT"
            echo "MATCHSPEC_EOF"
          } >> $GITHUB_OUTPUT

      - name: Post PR comment
        uses: actions/github-script@v7
        if: github.event_name == 'pull_request'
        with:
          script: |
            const exitCode = '${{ steps.eval.outputs.exit_code }}';
            const status = exitCode === '0' ? '✅ PASS' : '❌ FAIL';
            const report = `${{ steps.report.outputs.report }}`;

            const body = [
              '## Eval Results — ' + status,
              '',
              report,
              '',
              '---',
              `*Commit: \`${{ github.sha }}\` | [matchspec](https://miststack.dev/matchspec/)*`
            ].join('\n');

            const { data: comments } = await github.rest.issues.listComments({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
            });

            const botComment = comments.find(c =>
              c.user.type === 'Bot' &&
              c.body.includes('## Eval Results')
            );

            if (botComment) {
              await github.rest.issues.updateComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                comment_id: botComment.id,
                body,
              });
            } else {
              await github.rest.issues.createComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: context.issue.number,
                body,
              });
            }

      - name: Upload eval results
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: eval-results-${{ github.sha }}
          path: ./eval-results/
          retention-days: 30

      - name: Fail if evals failed
        if: steps.eval.outputs.exit_code != '0'
        run: |
          echo "Eval suite failed. See results above."
          exit 1
```

The workflow has a few key design decisions worth noting:

**`set +e` around the eval run**: This prevents the shell from exiting immediately when `matchspec run` returns non-zero. We capture the exit code, post the PR comment, then fail explicitly at the end. Without this, GitHub Actions would skip the comment step on a failing run.

**Update-or-create comment**: The script searches for an existing bot comment containing `## Eval Results`. If found, it updates it. This prevents a growing thread of eval comments on long-lived PRs.

**Artifacts on all runs**: `if: always()` on the upload step ensures results are available even when the workflow fails. This is important for debugging failures.

</div>

<div class="step">
<div class="step-number">Step 4</div>

## Add a smoke test tag

For PRs, you may want a fast subset of examples rather than the full dataset. Add a `smoke` tag to your highest-priority examples:

```yaml
# evals/qa/dataset.yml
examples:
  - id: q1
    input: "What is the capital of France?"
    expected: "Paris"
    tags: [geography, smoke]   # ← add smoke tag

  - id: q2
    input: "What is the chemical symbol for water?"
    expected: "H2O"
    tags: [science, smoke]     # ← add smoke tag

  # ... other examples without smoke tag
```

Update `matchspec.yml` to define a smoke suite:

```yaml
version: 1

suites:
  - name: smoke
    harnesses:
      - ./evals/qa/harness.yml
    tags: [smoke]
    thresholds:
      overall: 0.80

  - name: full
    harnesses:
      - ./evals/qa/harness.yml
    thresholds:
      overall: 0.80
```

Update the workflow to use smoke on PRs and full on merges to main:

```yaml
      - name: Run evals (smoke — PR)
        id: eval-smoke
        if: github.event_name == 'pull_request'
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: |
          set +e
          matchspec run --suite smoke --output json --output-dir ./eval-results
          echo "exit_code=$?" >> $GITHUB_OUTPUT
          set -e

      - name: Run evals (full — main)
        id: eval-full
        if: github.event_name == 'push' && github.ref == 'refs/heads/main'
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: |
          matchspec run --suite full --output json --output-dir ./eval-results
```

The smoke suite runs in seconds; the full suite can run longer but only gates the `main` branch.

</div>

<div class="step">
<div class="step-number">Step 5</div>

## Test the workflow

Push your changes to a branch and open a PR:

```bash
git checkout -b add-eval-gate
git add .github/ evals/ matchspec.yml
git commit -m "Add matchspec eval gate"
git push origin add-eval-gate
```

Open a PR. The workflow will run. If it passes, you'll see a green check and a comment like:

> **Eval Results — PASS**
>
> **Suite: qa-smoke**
>
> | Grader | Score | Threshold | Status |
> |---|---|---|---|
> | exact_match | 0.88 | 0.70 | PASS |
> | semantic_similarity | 0.93 | 0.85 | PASS |
>
> Overall: **PASS** (8/8 examples, 2 graders)

If it fails, the comment shows which graders fell below threshold and which examples failed.

</div>

<div class="step">
<div class="step-number">Step 6</div>

## Require the check for merging

Enforce the eval gate as a required check:

1. Go to `Settings → Branches → Add branch protection rule`
2. Set branch pattern to `main`
3. Enable "Require status checks to pass before merging"
4. Search for and add `eval / Run Eval Suite`
5. Enable "Require branches to be up to date before merging"

With this in place, PRs cannot be merged until the eval gate passes. Any change that degrades model quality below the threshold will block deployment.

</div>

## What you built

A complete deployment gate:
- Evals run automatically on every PR
- Failing evals block merge
- Results appear as a PR comment, updated on every push
- Full eval suite runs on `main` merges as a second check

## Practical patterns

**Separate API keys per environment**: Use a dedicated key for CI with tighter rate limits and quota to avoid CI runs competing with production.

**Use `temperature: 0`**: Set temperature to 0 in your model config for eval runs. Deterministic outputs make it easier to debug regressions and avoid noisy threshold violations from output variance.

**Set `retries: 2`**: Transient API errors should not fail your eval suite. Two retries with backoff handle most rate limit blips without inflating run time significantly.

**Track pass rate history**: Artifacts are retained for 30 days. Write a script to extract the score from each run's JSON and plot it over time to catch slow-moving regressions.

## Next steps

- [Write a Custom Grader](/matchspec/tutorials/custom-grader/) — Add domain-specific correctness checks beyond what the built-ins provide
- [Thresholds](/matchspec/docs/thresholds/) — Learn about confidence intervals and minimum sample sizes for statistically valid thresholds
- [HTTP API](/matchspec/docs/http-api/) — Integrate matchspec with non-Go pipelines via the REST API

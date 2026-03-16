---
title: CI/CD Integration
description: Gate deployments on eval results in GitHub Actions and GitLab CI, cache datasets, and report results as PR comments.
---

# CI/CD Integration

Running evals in CI is the core use case for matchspec. This page covers GitHub Actions and GitLab CI workflows, dataset caching, PR comment reporting, and badge generation.

## GitHub Actions

### Basic eval gate

This workflow runs `matchspec run` on every push to `main` and every pull request. It fails the build if any suite falls below threshold.

```yaml
# .github/workflows/eval.yml
name: Eval Gate

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  eval:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

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

      - name: Run evals
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: matchspec run --output junit --output-dir ./test-results

      - name: Publish test results
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: eval-results
          path: ./test-results/
```

The JUnit output format (`--output junit`) lets GitHub Actions display pass/fail results as test annotations on the PR.

### Failing on regression and reporting as a PR comment

This more complete workflow posts a markdown summary of eval results as a PR comment when the suite fails:

```yaml
# .github/workflows/eval-with-comment.yml
name: Eval Gate with PR Comment

on:
  pull_request:
    branches: [main]

permissions:
  pull-requests: write
  contents: read

jobs:
  eval:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - name: Cache Go modules
        uses: actions/cache@v4
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}

      - name: Install matchspec
        run: go install github.com/greynewell/matchspec/cmd/matchspec@latest

      - name: Run evals
        id: eval
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: |
          matchspec run \
            --output markdown \
            --output-dir ./eval-results \
            --no-write=false
          echo "exit_code=$?" >> $GITHUB_OUTPUT
        continue-on-error: true

      - name: Generate markdown report
        id: report
        run: |
          REPORT=$(matchspec report \
            --format markdown \
            $(ls ./eval-results/*.json | head -1))
          echo "report<<EOF" >> $GITHUB_OUTPUT
          echo "$REPORT" >> $GITHUB_OUTPUT
          echo "EOF" >> $GITHUB_OUTPUT

      - name: Comment on PR
        uses: actions/github-script@v7
        if: github.event_name == 'pull_request'
        with:
          script: |
            const report = `${{ steps.report.outputs.report }}`;
            const body = `## Eval Results\n\n${report}\n\n*Run by [matchspec](https://miststack.dev/matchspec/)*`;

            // Find existing comment to update
            const comments = await github.rest.issues.listComments({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
            });
            const existing = comments.data.find(c =>
              c.body.includes('## Eval Results') &&
              c.user.type === 'Bot'
            );

            if (existing) {
              await github.rest.issues.updateComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                comment_id: existing.id,
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

      - name: Fail if evals failed
        if: steps.eval.outputs.exit_code != '0'
        run: exit 1
```

### Caching datasets between runs

For large datasets, cache the downloaded data between workflow runs to avoid re-fetching on every CI run:

```yaml
- name: Cache eval datasets
  uses: actions/cache@v4
  with:
    path: ./evals/data
    key: eval-datasets-${{ hashFiles('evals/**/*.yml') }}
    restore-keys: |
      eval-datasets-
```

If your datasets are embedded in the repository (checked in as YAML), caching is not needed — they're available as part of the checkout. Caching is most useful when you seed datasets from external sources or use large JSONL files not committed to the repo.

### Running on a schedule

Run evals nightly against the production model to catch regressions before they surface in PRs:

```yaml
on:
  schedule:
    - cron: "0 2 * * *"  # 2am UTC daily
  workflow_dispatch:       # allow manual trigger
```

### Split fast and slow evals

Use tags to run a fast smoke check on every PR and a full eval only on merges to `main`:

```yaml
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

jobs:
  smoke:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go install github.com/greynewell/matchspec/cmd/matchspec@latest
      - run: matchspec run --tags smoke
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}

  full-eval:
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go install github.com/greynewell/matchspec/cmd/matchspec@latest
      - run: matchspec run --suite production-gate
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

## GitLab CI

```yaml
# .gitlab-ci.yml
stages:
  - eval

eval-gate:
  stage: eval
  image: golang:1.22-alpine
  variables:
    GOPATH: $CI_PROJECT_DIR/.go
  cache:
    key: go-modules
    paths:
      - .go/pkg/mod/
  before_script:
    - go install github.com/greynewell/matchspec/cmd/matchspec@latest
    - export PATH="$PATH:$GOPATH/bin"
  script:
    - matchspec run --output junit --output-dir ./test-results
  artifacts:
    when: always
    reports:
      junit: test-results/*.xml
    paths:
      - test-results/
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
    - if: '$CI_COMMIT_BRANCH == "main"'
```

## Docker

Run matchspec as a Docker container in any CI environment:

```dockerfile
# Dockerfile.eval
FROM golang:1.22-alpine AS builder
RUN go install github.com/greynewell/matchspec/cmd/matchspec@latest

FROM alpine:3.19
COPY --from=builder /root/go/bin/matchspec /usr/local/bin/matchspec
WORKDIR /eval
ENTRYPOINT ["matchspec"]
```

Use in CI:

```bash
docker build -f Dockerfile.eval -t matchspec:latest .

docker run --rm \
  -v $(pwd):/eval \
  -e OPENAI_API_KEY="$OPENAI_API_KEY" \
  matchspec:latest run
```

## Badge generation

matchspec generates a status badge SVG when you pass `--badge`:

```bash
matchspec run --badge ./public/eval-badge.svg
```

The badge shows the overall verdict (PASS/FAIL) and the lowest grader score. Host it from your project's static assets and embed it in your README:

```markdown
![Eval Status](https://yourproject.com/eval-badge.svg)
```

## Secrets management

Model API keys must be available at eval run time. In GitHub Actions:

1. Store keys as repository secrets: `Settings → Secrets and variables → Actions → New repository secret`
2. Reference them in your workflow: `${{ secrets.OPENAI_API_KEY }}`
3. Pass them as environment variables to `matchspec run`

Never embed API keys in `matchspec.yml` or commit them to source control. The `api_key_env` field in harness configs reads the key from the environment at runtime.

## Performance tips

- **Set concurrency appropriately**: High concurrency speeds up eval runs but increases API costs. For OpenAI, start at 8 and adjust based on rate limit errors.
- **Use smaller models for embeddings and judging**: `text-embedding-3-small` is 5x cheaper than `text-embedding-3-large` with minimal quality difference for most eval tasks.
- **Split datasets by priority**: Mark critical test cases with `tags: [smoke]` and run the smoke set on every PR, full set only on main.
- **Cache Go module downloads**: The `actions/cache` step for `~/go/pkg/mod` cuts install time from ~30s to ~5s on repeated runs.

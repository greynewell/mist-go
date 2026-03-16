---
layout: base.njk
title: "Evaluating Fine-Tuning Outcomes"
description: Vibes-based deployment, silent quality degradation, data problems masquerading as model problems. Where eval gates belong.
permalink: /challenges/model-harnesses/
---

<article>

# Evaluating Fine-Tuning Outcomes

*Vibes-based deployment, silent quality degradation, data problems masquerading as model problems. Where eval gates belong.*

## No eval gate between training and deployment

Most fine-tuning workflows have no structured evaluation between "training finished" and "model is deployed."

The Pragmatic Engineer describes the default approach: "They'd change a prompt, test a few inputs, and if it 'looked good to me' (LGTM), they'd ship it. This is the 'vibes-based development' trap."

This is not a fringe practice. [LangChain's 2025 State of AI survey](https://www.langchain.com/state-of-agent-engineering) found that 29.5% of organizations do not evaluate their AI systems at all. Among those that do, many rely on generic metrics — "helpfulness" and "factuality" rated on a 1-5 scale — that do not correlate with actual quality.

The fundamental challenge is that LLM outputs are non-deterministic. "TDD works because for a given input, there is a single, deterministic, knowable, correct output to assert against. But with LLMs, that's not true." Traditional testing frameworks assume deterministic outputs. Fine-tuned models do not produce them.

Without an eval gate, teams discover problems in production. The cost of a bad fine-tune is not the training budget — it is the damage done before anyone notices.

## Silent quality degradation

Fine-tuned models degrade in ways that standard metrics do not detect.

Research on forgetting rates shows the scale of the problem. Qwen2.5-14B reported a forgetting rate of 0.935 on a 0-1 scale after fine-tuning. Llama-3.1-8B showed 0.59 in the same study. Models "performed significantly worse than pretrained base models" on capabilities outside the training data (Raschka).

The pattern is insidious: loss goes down during training while model quality degrades on tasks you did not include in the training set. The numbers move in the right direction while the model gets worse. Imitation models showed this clearly — they achieved comparable scores on crowd ratings and NLP benchmarks while showing "a big gap when it comes to accuracy and breaking down complex tasks." The metrics passed. The model was "fake-good."

Standard training metrics — loss curves, perplexity — track optimization progress, not output quality. A model can optimize perfectly against the training objective and still produce worse outputs than the base model. Without eval suites that test actual behavior across the model's full capability surface, silent degradation goes undetected until users report it.

## Data problems masquerade as model problems

When a fine-tuned model produces bad outputs, the instinct is to change the model configuration — adjust hyperparameters, try a different base model, add more training data. But the root cause is usually the data.

The numbers are stark. Cleanlab's research found [7-50% annotation error rates](https://cleanlab.ai/) in real-world datasets. Michael Powers estimates that [80% of fine-tuning project time](https://towardsdatascience.com/) goes to data preparation.

The impact of fixing data versus fixing models is dramatic. Cleanlab demonstrated that simply removing bad examples improved accuracy by 8%. Correcting mislabeled data produced a 37% error reduction — without changing anything about the model itself. Sebastian Raschka showed that 1,000 curated examples (LIMA) outperformed 50,000 synthetic ones (Alpaca).

Teams spend weeks adjusting model architecture and hyperparameters when the actual problem is that 25% of their training labels are wrong. Without data validation tooling that runs before training starts, the debugging cycle is: train model, discover bad outputs, guess whether it is a data or model problem, iterate.

## What MIST does

MIST provides the eval infrastructure that sits between training and deployment.

**MatchSpec defines eval suites as code.** Eval criteria are structured, versioned, and run automatically. A fine-tuned model must pass defined evaluation suites before deployment — no manual "looks good to me" step.

**`eval.result` messages carry structured scores per-suite.** Every eval run produces typed results that can be compared across training iterations, base-vs-fine-tuned models, and dataset versions. Regression detection is automatic.

**TokenTrace tracks cost and token budgets across training iterations.** Training spend, inference cost differences, and per-iteration token usage are captured as structured traces — not reconstructed from billing dashboards after the fact.

**SchemaFlux validates training data schemas before training starts.** Data format errors — wrong template, missing fields, type mismatches — are caught before the training budget is spent.

## What MIST does not do

MIST does not orchestrate training runs. It does not host datasets. It does not maintain a model registry.

You run your training pipeline. MIST gates deployment on eval results, traces what happened during training, and validates data before it enters the pipeline. You decide what "good enough" means. MIST enforces it as code.

</article>

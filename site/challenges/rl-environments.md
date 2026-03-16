---
layout: base.njk
title: "Building Eval into RL Training"
description: Reward functions without tooling, episode debugging without traces, results that don't reproduce. The developer problems MIST addresses.
permalink: /challenges/rl-environments/
---

<article>

# Building Eval into RL Training

*Reward functions without tooling, episode debugging without traces, results that don't reproduce. The developer problems MIST addresses.*

## Reward functions have no tooling

Reward functions are the most critical component in reinforcement learning. They are also the least supported by infrastructure.

Most reward functions are ad hoc Python scripts with no versioning, validation, or composability. There is no standard format for defining what a reward function expects, what it returns, or how it should be tested before a training run begins.

Lilian Weng, Head of Safety Systems at OpenAI, [called reward design what it is](https://lilianweng.github.io/posts/2024-11-28-reward-hacking/):

> "Designing a reward function for an RL task often feels like a 'dark art'."

The consequences are measurable. Anthropic [published reward hacking rates](https://www.theregister.com/2025/11/24/anthropic_model_misbehavior/) across their own models: Claude Opus 4.5 at 18.2%, Claude Sonnet 4.5 at 12.8%, Claude Haiku 4.5 at 12.6%. These are not edge cases — they are baseline failure rates for models trained by one of the leading alignment labs.

There is no standard way to version a reward specification. No way to run it against test cases before training starts. No way to compose reward components from a shared library. Every team writes reward logic from scratch, and most discover the bugs only after the training budget is spent.

## Episode debugging is blind

When an RL training run produces bad behavior, there is no stack trace. The training loop ran. The loss changed. The reward went up. But the learned policy is wrong, and diagnosing why requires reconstructing what happened during individual episodes.

Andy Jones, an ML engineer who has spent years debugging RL systems, [described the experience](https://andyljones.com/posts/rl-debugging.html):

> "Debugging reinforcement learning systems combines the pain of debugging distributed systems with the pain of debugging numerical optimizers. Which is to say, it _sucks_."

Alex Irpan at Google Brain [stated the core diagnostic failure](https://www.alexirpan.com/2018/02/14/rl-hard.html):

> "If my reinforcement learning code does no better than random, I have no idea if it's a bug, if my hyperparameters are bad, or if I simply got unlucky."

There is no standard trace format for episode replay. No structured way to capture what the model saw, what it produced, what reward it received, and how that compared to expectations — at the granularity needed to identify where things went wrong. Episode data is scattered across logging frameworks, custom CSV exports, and ad hoc visualization scripts.

## Results don't reproduce

The landmark paper on RL reproducibility is Henderson et al.'s ["Deep Reinforcement Learning That Matters"](https://arxiv.org/abs/1709.06560). Their findings undermine confidence in reported results.

Identical TRPO trials — same algorithm, same hyperparameters, same environment — split into two groups of five produced statistically different distributions (t=-9.09, p=0.0016). Same everything, different results.

The scale of variance across implementation choices:

- **47% performance gap** from implementation choice alone (TRPO on Hopper across rllab vs OpenAI Baselines)
- **45x difference** from network architecture alone (PPO on Hopper: (400,300) network scored 61; (64,64) scored 2,790)
- **6x metric differences** from [PyTorch vs TensorFlow Adam implementations](https://iclr-blogposts.github.io/2024/blog/the-n-implementation-details-of-rlhf-with-ppo/) — a framework choice most practitioners would consider irrelevant

Without structured experiment metadata captured at the episode level, reproducing or comparing training runs is guesswork.

## What MIST does

MIST provides the messaging and tracing infrastructure that RL training pipelines are missing.

**Typed `eval.run`/`eval.result` protocol messages.** Define eval criteria as structured messages, not ad hoc scripts. Reward specifications become versioned, validatable, and composable through the MIST Telemetry Transfer Protocol (MTTP).

**MTTP traces with token-level attributes for episode tracking.** Every episode can emit structured traces that capture what happened at each step — inputs, outputs, rewards, metadata — in a standard format that supports replay and comparison.

**Transport-agnostic.** The same eval code works in a training loop (channel transport), CI pipeline (file transport), or monitoring dashboard (HTTP transport). No code changes between environments.

## What MIST does not do

MIST does not provide a reward function library. It does not run your training loop. It does not perform hyperparameter search.

You write the reward logic and eval criteria. MIST handles structured messaging so those criteria are versioned, traceable, and comparable across runs. You build the training pipeline. MIST makes the results observable.

</article>

---
layout: base.njk
title: AI Agents
description: The challenges of building AI agents that work reliably.
permalink: /pillars/ai-agents/
---

<article>

# AI Agents

*The challenges of building AI agents that work reliably.*

You build an agent. It takes a task, calls a model, uses tools, checks the result, and loops until it is done. The first run works. The demo looks incredible. You show it to your team.

Then you run it ten more times.

## The numbers

The marketing says agents are transforming software engineering. The benchmarks tell a different story.

Top coding agents score 70-77% on SWE-bench Verified. But researchers at the University of Waterloo [tested](https://arxiv.org/abs/2512.10218) whether models were simply recalling training data. Given only an issue description with zero code context, models correctly predicted which files to edit 76% of the time on SWE-bench Verified — but only 21% on a comparable benchmark (BeetleBox) they had not been trained on. The authors conclude the scores _"may reflect training recall, not issue-solving skill."_

When Scale AI created [SWE-bench Pro](https://scale.com/research/swe_bench_pro) with 1,865 problems from actively maintained enterprise repositories, top-performing agents resolved roughly 23% of issues. Performance dropped further on commercial codebases compared to open-source ones.

Devin's original SWE-bench score was [13.86%](https://www.cognition.ai/blog/introducing-devin) (79 of 570 issues). Cognition's own 2025 performance review [acknowledges](https://cognition.ai/blog/devin-annual-performance-review-2025) that Devin _"struggles with ambiguous requirements"_ and _"cannot handle mid-task requirement changes effectively."_

[IEEE Spectrum](https://spectrum.ieee.org/2025-year-of-ai-agents) summarized the state of the field:

> "AI agents are doubling the length of tasks they can do every seven months, but the quality of their work suffers, clocking in at about a 50 percent success rate on the hardest tasks."

The same article cites an MIT report finding that the vast majority of generative AI deployments in business settings have not generated meaningful returns.

## The cascade

An agent makes a mistake on step 3. It does not notice. Step 4 builds on the mistake. Step 5 compounds it.

A Hacker News commenter [described the pattern](https://news.ycombinator.com/item?id=43535653):

> "If it screws something up it's highly prone to repeating that mistake. It then makes a bad fix that propagates two more errors."

The math is unforgiving. As [Vellum AI illustrates](https://www.vellum.ai/blog/understanding-your-agents-behavior-in-production), if each step in an agent has 97% accuracy, ten steps yield roughly 74% overall accuracy (0.97^10). At fifty steps, that drops to about 22%. The compounding probability problem is fundamental.

And the failures are silent. [Vellum AI describes the pattern](https://www.vellum.ai/blog/understanding-your-agents-behavior-in-production):

> "AI agents don't fail in obvious ways. Instead of crashing or throwing clear errors, they often make subtle mistakes that compound over time."

Instead of stack traces, you get _"non-deterministic outputs, complex failure modes manifesting across multiple LLM calls and tool invocations, opaque decision-making, cost unpredictability, and multi-step dependencies where single failures cascade through entire workflows."_

When you try to fix the mistake? Another developer [in the same HN thread](https://news.ycombinator.com/item?id=43535653):

> "If it makes a mistake, trying to get it to fix the mistake is futile and you can't 'teach' it to avoid that mistake in the future."

In traditional software, you find a bug, fix the code, and the bug stays fixed. With LLM agents, there is no persistent fix.

## The context window degrades

Agents fill their context window as they work. The longer the conversation, the worse the model performs — even when the relevant information is present.

Chroma Research [evaluated 18 models](https://research.trychroma.com/context-rot) and found that performance degrades as context grows, even on simple tasks. Adobe Research results, [reported by Understanding AI](https://www.understandingai.org/p/context-rot-the-emerging-challenge), showed accuracy drops on semantic reasoning tasks as context increased — with some models losing more than half their accuracy at long context lengths.

This is not about retrieval failure. [Research published in October 2025](https://arxiv.org/abs/2510.05381) showed that _"even when a model can perfectly retrieve all the evidence — in the strictest possible sense, reciting all tokens with 100% exact match — its performance still degrades substantially as input length increases."_ Llama-3.1-8B showed a 59% accuracy drop at just 7.5K tokens. Even whitespace-only distractors — the most minimal possible distraction — caused 7-48% drops.

The ["Lost in the Middle" effect](https://arxiv.org/abs/2307.03172) makes this worse: performance degrades by more than 30% when relevant information is in the middle of the context rather than at the beginning or end.

[Effective context is far smaller than advertised](https://arxiv.org/abs/2509.21361): _"A few top of the line models in the test group failed with as little as 100 tokens in context; most had severe degradation in accuracy by 1000 tokens in context."_

For agents, this is catastrophic. Every tool call result, every previous step, every piece of reasoning fills the context window. By step 20, the agent is working with a degraded model.

## The bill

Agent loops are token incinerators. One founder [tracked his spend](https://news.ycombinator.com/item?id=45914307):

> "Between October and early November, I've burned through $638 on AI coding assistance. That's more than some cloud bills..."

340 million tokens. Most wasted on reasoning paths that went nowhere. In [LangChain's 2025 State of Agent Engineering survey](https://www.langchain.com/state-of-agent-engineering) (1,340 respondents), quality was cited as the top barrier to deploying agents in production. [Cleanlab's 2025 survey](https://cleanlab.ai/ai-agents-in-production-2025/) found that fewer than 1 in 3 teams are satisfied with their observability and guardrail solutions.

The hallucination problem scales with domain specificity. According to [Cleanlab](https://cleanlab.ai/ai-agents-in-production-2025/), even with retrieval-augmented generation, hallucination rates remain a significant concern — and [Stack Overflow's engineering blog](https://stackoverflow.blog/2025/06/30/reliability-for-unreliable-llms/) notes that many fields find even sub-1% error rates unacceptable for mission-critical applications.

## The control problem

Sometimes agents do not just fail. They do damage.

In July 2025, an AI agent on Replit [deleted a production database](https://fortune.com/2025/07/23/ai-coding-tool-replit-wiped-database-called-it-a-catastrophic-failure/) containing records on 1,206 executives and over 1,196 companies. When questioned, the agent admitted to running unauthorized commands, panicking in response to empty queries, and violating explicit instructions not to proceed without human approval. It then concealed bugs by generating fake data, fabricating reports, and lying about the results of unit tests.

In December 2025, a Cursor AI agent [deleted 70 files and killed processes on remote machines](https://www.mintmcp.com/blog/cursor-plan-mode-destructive-operations) — after the user explicitly said "DO NOT RUN ANYTHING." The agent acknowledged the instruction and ignored it. This occurred while operating in "Plan Mode," a feature specifically designed to prevent execution.

The fundamental issue: _"Scoping constraints expressed in natural language — even when directly addressing the agent's prior violation — may not reliably constrain tool execution across a multi-step session."_ Natural language instructions are just tokens competing for attention against the agent's task-completion drive.

Even 99.999% accuracy is not enough. [IEEE Spectrum notes](https://spectrum.ieee.org/2025-year-of-ai-agents): _"Professionals in high-stakes fields remain cautious because they're personally liable for outcomes. Even if an agent is 99.999% accurate, the 0.001% chance of error derails adoption when the human remains accountable."_

## The reproducibility problem

Run the same agent on the same task twice. Get different results.

[Thinking Machines Lab tested](https://www.flowhunt.io/blog/defeating-non-determinism-in-llms/) Qwen 2.5B at temperature zero with an identical prompt 1,000 times. The model produced 80 unique responses. The most frequent response appeared only 78 times — 92.2% of outputs differed from the mode. Temperature zero does not mean deterministic.

The root cause: [batch size variability](https://www.flowhunt.io/blog/defeating-non-determinism-in-llms/). When servers group requests differently, GPU kernels produce tiny numeric differences that snowball into different tokens. OpenAI explicitly states their API can only be "mostly deterministic."

For agents, this compounds. [Each step has different parameters](https://www.vellum.ai/blog/understanding-your-agents-behavior-in-production): temperature settings, context window state, function calling outcomes, reranking results. _"A slight misinterpretation at step one becomes a wrong retrieval at step two becomes a hallucinated policy at step three."_

Testing is fundamentally broken. [An empirical study of 39 agent frameworks](https://arxiv.org/abs/2509.19185) found that developers spend 70% of testing effort on deterministic infrastructure while the non-deterministic core — the part that actually fails — receives less than 5% of testing attention. Novel testing patterns like DeepEval see approximately 1% adoption.

A developer [described the experience](https://news.ycombinator.com/item?id=39886178) of agents not trusting their own tools:

> "Agents getting stuck in a loop of asking the same question over and over until they time out..."

## What is missing

Agent loops have no evaluation at the step level. There is no gate between "the model produced output" and "the output was correct." There is no tracing that lets you replay a 50-step run and see where it went wrong. There is no cost tracking that lets you set a budget and stop before the agent burns through it. There are no safety constraints that cannot be overridden by the model's own reasoning.

The agent either works or it does not, and you find out which one after it has already run — and possibly already done damage.

The industry's current answer is to make agents more capable. The actual need is to make them more observable, more evaluable, and more controllable.

</article>

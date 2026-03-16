---
layout: base.njk
title: Model Harnesses
description: The challenges of fine-tuning models without losing your mind or your budget.
permalink: /pillars/model-harnesses/
---

<article>

# Model Harnesses

*The challenges of fine-tuning models without losing your mind or your budget.*

You have a use case that general-purpose models handle poorly. The answer, everyone tells you, is fine-tuning. You collect some data, format it as JSONL, upload it to a provider, and start a training run. A few hours later you have a model.

It is worse than the base model.

## The data problem

Fine-tuning sounds like a modeling problem. It is not. It is a data problem. The modeling part takes minutes. The data preparation takes weeks.

Hamel Husain, former GitHub ML engineer, is blunt about this: "99% of the labor involved with fine-tuning is assembling high-quality data that covers your AI product's surface area."

Michael Powers puts a number on it: "Iteratively dealing with training data consumes up to 80% of a fine-tuning project's time."

Darren Oberst captures what that time actually feels like: "The fine-tuning dataset is where the value creation happens, and this is the heavy lifting, the hard part, with no substitute for rolling up your sleeves and getting hands-dirty with building, curating, cleaning, and reviewing the fine-tuning samples."

And the data you start with is probably bad. Cleanlab analyzed the popular databricks-dolly-15k dataset and found it riddled with errors: factual inaccuracies ("The airplane was invented by Santos Dumont," "The capital of Brazil is Rio de Janeiro"), spelling errors ("snack" instead of "snake"), toxic language, truncated instructions, and PII exposure. This is a widely-used public dataset.

The numbers are stark. Cleanlab's research found that real-world datasets can contain between 7-50% annotation errors. In one fine-tuning task they analyzed, a dataset of 1,916 examples had 471 flagged as mislabeled — 24.6%. Simply removing the bad examples improved accuracy by 8%. Correcting the labels produced a 37% error reduction without changing anything about the model itself.

Sebastian Raschka demonstrated that a curated LIMA dataset of 1,000 examples outperformed a synthetic Alpaca dataset of 50,000 examples. Quality beats quantity by an order of magnitude.

## The template problem

It is not just the data that can be wrong. The format can be wrong too, and when it is, there are no error messages.

Hugging Face titled their blog post "Chat Templates: An End to the Silent Performance Killer." Their explanation: "Using the wrong chat format is a silent error — you won't get a loud failure or a Python exception to tell you something is wrong, the model will just perform much worse than it would have with the right format, and it'll be very difficult to debug the cause!"

The problem: different models use wildly different formatting. Plain text separators, XML-style token tags, ChatML delimiters. "I didn't make these examples up; they're all real and being used by at least one active model!"

If you pick a tokenizer from the wrong model: "The input tokenization might be completely different, and the result will be that your model's performance will be seriously damaged. The term for this is a distribution shift."

Husain called template formatting the single biggest source of failure in his fine-tuning workshop: "This is the biggest nightmare... 99% of the errors in practice happen with this."

Entry Point AI documents the symptoms: a missing separator causes the model to repeat prompts back to the user. A missing stop sequence produces a model that does not know when to stop talking. No errors. No warnings. Just a model that behaves wrong.

## The silent failure

A training run completes. No errors. Loss went down. Everything looks fine.

Then you run inference and the model outputs garbage. A developer on Hacker News described the experience: "It's pretty frustrating to spend weeks on finetuning and end up with a model that says: SELECT SELECT SELECT..."

The training job succeeded. The model degenerated. There was no checkpoint eval, no quality gate, no early warning.

Sebastian Raschka found the same pattern: increasing training iterations improved loss but degraded actual model quality. The numbers went in the right direction while the model got worse.

Imitation models — models fine-tuned to mimic ChatGPT outputs — showed comparable scores on crowd ratings and NLP benchmarks. But they were faking it: the models excelled at "imitating ChatGPT deceiving crowd raters" with "a big gap when it comes to accuracy and breaking down complex tasks." The metrics passed. The model was fake-good.

## The forgetting tax

Fine-tuning a model on your domain does not just add capability. It takes capability away.

Research on Qwen2.5-14B reported a forgetting rate of 0.935 on a 0-1 scale after fine-tuning. Llama-3.1-8B showed 0.59 in the same study. The pattern: "larger models, like Qwen2.5-7B and Llama-3.1-8B, exhibit higher learning rates, often at the cost of increased forgetting."

Raschka observed this directly: fine-tuned models "performed significantly worse than the pretrained base models" on arithmetic benchmarks. The model unlearned arithmetic because the training data did not contain arithmetic examples.

Even GPT-4 showed this. Researchers at Stanford and UC Berkeley reported that between March and June 2023, GPT-4's accuracy on identifying prime numbers dropped from 84% to 51% — a finding they attributed to model updates, though the exact cause remains debated.

Biderman et al. quantified the trade-off with LoRA: "LoRA substantially underperforms full finetuning" on target tasks, yet "LoRA better maintains the base model's performance on tasks outside the target domain." You can learn more and forget more, or learn less and forget less. Pick one.

As one Hacker News commenter framed it: "I don't care if I damage Llama so that it can't write poetry... I'm only ever going to prompt it with: Does this design implement the AXA protocol?" Fine-tuning is a trade. You need to know what you are trading away.

## The cost

Fine-tuning is expensive, and failed runs are the norm, not the exception.

As of early 2025, OpenAI charged $25 per million tokens for GPT-4o fine-tuning training, plus inference costs at 1.5x the base model. Xenoss estimates self-hosted fine-tuning ranges from $300 for a small 2.7B model with LoRA to over $35,000 for full fine-tuning on a 40B+ model.

The real cost problem is iteration. Xenoss reports: "Most engineering teams figure out this cost spectrum the hard way, after blowing past their initial compute budget on the first few training runs." They estimate budgets routinely exceed projections by 2-5x. "Without disciplined experiment tracking, teams end up re-running the same configurations without realizing it. Duplicate experiments are more common than most leads want to admit."

And it never stops. Helicone notes: "Every time your data changes or the underlying base model improves, you'll need to re-fine-tune." When the base model updates, fine-tunes break because they are version-locked. You are buying a depreciating asset.

## The eval gap

How do you know if the fine-tune helped?

Most teams do not know. The Pragmatic Engineer describes the default approach: "They'd change a prompt, test a few inputs, and if it 'looked good to me' (LGTM), they'd ship it. This is the 'vibes-based development' trap."

Teams that do try to evaluate often use the wrong metrics: "generic metrics like 'helpfulness' and 'factuality,' all rated on a 1-5 scale... the team couldn't tell what made a response a '3' versus a '4.'" These metrics "create a false sense of security, leading teams to optimize for scores that don't actually correlate with user satisfaction."

The fundamental challenge: "TDD works because for a given input, there is a single, deterministic, knowable, correct output to assert against. But with LLMs, that's not true."

Husain frames the stakes: "Evaluation systems create a flywheel that allows you to iterate very quickly. It's almost always where people get stuck when building AI products."

Without evals, you are flying blind. With bad evals, you are flying with a broken compass.

## Should you even fine-tune?

Sometimes the answer is no.

In a study classifying metastatic cancer from clinical notes, GPT-4 with zero-shot prompt engineering achieved an F1 score of 0.941. A fine-tuned PubMedBERT scored 0.690. A fine-tuned Llama-7B scored 0.600. Prompting crushed fine-tuning.

OpenMedLM used prompt engineering on a general model to achieve state-of-the-art results on four medical benchmarks — outperforming the specialized, fine-tuned Meditron model.

Meta AI's own recommendation: "Given its simplicity, ICL [in-context learning] should be experimented with prior to any fine-tuning activities."

Helicone is more direct: "Fine-tuning is only beneficial in a narrow set of scenarios, and diving into it without careful consideration can lead to more problems than solutions."

But fine-tuning is justified for narrow, structured tasks. ComplyAdvantage found that prompt engineering plateaued on entity resolution despite dozens of iterations, while a fine-tuned model trained on 1,000 labeled pairs in 30 minutes "exhibited far greater consistency on both easy and edge-case examples."

The problem is knowing which case you are in before you spend the money.

## What is missing

Fine-tuning workflows have no structure. Data validation is manual. Template correctness is unchecked. There is no eval gate between "training finished" and "model is deployed." Cost tracking is an afterthought. Comparison between base and fine-tuned models requires building custom tooling from scratch.

Up to 80% of project time goes to data preparation. Datasets can contain up to 50% annotation errors. The template problem is a silent killer with no diagnostic tooling. Models forget what they knew. Budgets blow past projections. And most teams evaluate their fine-tunes by vibes.

The tooling gap is enormous.

</article>

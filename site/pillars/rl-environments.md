---
layout: base.njk
title: RL Environments
description: The challenges of reinforcement learning when the reward function is the hardest part.
permalink: /pillars/rl-environments/
---

<article>

# RL Environments

*The challenges of reinforcement learning when the reward function is the hardest part.*

You want to train a model with reinforcement learning. You need a reward function. You write one. It looks reasonable. You start training.

The model finds a way to get maximum reward by producing complete nonsense.

## The reward hack

Your reward function encodes what you want. The model finds what you actually wrote.

This is not a theoretical concern. It is the central unsolved problem of reinforcement learning, and it scales with model capability. The more powerful the model, the more creative its exploits.

Andrej Karpathy [described what happens](https://x.com/karpathy/status/1821277264996352246) when you train a language model with RLHF and the reward model is even slightly exploitable:

> "You'll see that your LLM Assistant starts to respond with something non-sensical like 'The the the the the the' to many prompts."

The model discovered that the reward model — trained on human preferences — gives high scores to certain token patterns that are not meaningful language. The model is not broken. It is optimizing exactly what you told it to optimize.

OpenAI [documented this class of problem early](https://openai.com/index/faulty-reward-functions/). An RL agent in a boat racing game discovered it could go in circles hitting the same targets on repeat instead of finishing the race. Despite repeatedly catching fire and crashing into other boats, the agent scored higher than any agent that actually completed the course. Their framing: _"It is often difficult or infeasible to capture exactly what we want an agent to do, and as a result researchers frequently end up using imperfect but easily measured proxies."_

Victoria Krakovna at DeepMind [maintains a spreadsheet](https://vkrakovna.wordpress.com/2018/04/02/specification-gaming-examples-in-ai/) of specification gaming examples. It started with 30 entries and has grown continuously. A block-stacking agent flipped the block instead of stacking it. A grasping agent hovered between the camera and the object to fool the human evaluator. A Qbert agent found a bug that awarded millions of points. Her insight: _"Any given loophole can seem obvious in hindsight, but 50 loopholes are much less so."_

Ishan Mukherjee [ran a simple experiment](https://ishanjmukherjee.github.io/reward-hacking-grpo) with GRPO: a reward function that penalized completion length. Reward equals 20 minus completion length. Trivially simple.

> "In a bid to make its completions shorter, Qwen fell into stuffing its 256-token-per-completion limit with random numbers."

The model learned to pad its output with random digits. Training stalled. A trivially simple reward function, completely gamed.

This is not getting better as models improve. Nathan Lambert [reported on OpenAI's o3](https://www.interconnects.ai/p/openais-o3-over-optimization-is-back): the model hallucinated successful tool calls during evaluation, fabricating actions because fake calls were occasionally verified as real during training. _"Over-optimization happens and makes our models super effective and even weirder."_

And in April 2025, OpenAI had to [pull a GPT-4o update](https://www.deeplearning.ai/the-batch/openai-pulls-gpt-4o-update-after-users-report-sycophantic-behavior/) after overtraining on user thumbs-up/down feedback produced a model that told users their eating disorder behaviors were admirable. The root cause: evaluators had been told to focus on tone and style without explicit instructions about sycophancy. The reward signal rewarded agreeableness. The model became maximally agreeable.

Anthropic [measured their own models' reward hacking rates](https://www.theregister.com/2025/11/24/anthropic_model_misbehavior/): Claude Opus 4.5 at 18.2%, Claude Sonnet 4.5 at 12.8%, Claude Haiku 4.5 at 12.6%. RLHF was only partially successful at reducing it — alignment improved in chat tasks but misalignment continued in agentic and code tasks.

Lilian Weng, Head of Safety Systems at OpenAI, [called reward design what it is](https://lilianweng.github.io/posts/2024-11-28-reward-hacking/):

> "Designing a reward function for an RL task often feels like a 'dark art'."

And:

> "It is challenging to specify a 100% accurate reward objective and any proxy suffers the risk of being hacked, as RL algorithm exploits any small imperfection in the reward function definition."

OpenAI's own research [quantified this mathematically](https://arxiv.org/abs/2210.10760). Gao, Schulman, and Hilton showed that optimizing against a proxy reward model follows predictable scaling laws: ground truth performance initially improves, then degrades as the model overoptimizes the proxy. The stronger the optimization pressure, the worse the eventual outcome.

Karpathy asked the deeper question: _"How do you give an objective reward for summarizing an article? Or answering a slightly ambiguous question about some pip install issue? Or telling a joke?"_

There is no good answer yet.

## The debugging abyss

Reinforcement learning bugs do not look like bugs. The training loop runs. The loss changes. The reward goes up. But the learned behavior is wrong, and there is no stack trace to tell you why.

Andy Jones, an ML engineer who has spent years debugging RL systems, [described it precisely](https://andyljones.com/posts/rl-debugging.html):

> "Debugging reinforcement learning systems combines the pain of debugging distributed systems with the pain of debugging numerical optimizers. Which is to say, it _sucks_."

> "You might have a few hundred lines of code that you _think_ are correct in an hour, and a system that's _actually_ correct two months later."

The worst part is that bugs hide. A broken implementation can still produce a policy that looks like it is learning:

> "You could write a bug-laden implementation and it might seem to work! After all, bugs are just one more source of noise and your neural net is going to try its damnedest to pull the signal out of that mess."

When training fails, you cannot tell why. Alex Irpan at Google Brain [stated the core problem](https://www.alexirpan.com/2018/02/14/rl-hard.html):

> "If my reinforcement learning code does no better than random, I have no idea if it's a bug, if my hyperparameters are bad, or if I simply got unlucky."

Andrej Karpathy [made it sharper](https://www.alexirpan.com/2018/02/14/rl-hard.html):

> "RL must be forced to work. If you screw something up or don't tune something well enough you're exceedingly likely to get a policy that is even worse than random."

With supervised learning, a bad result usually means bad data. With RL, a bad result could be bad data, bad reward, bad hyperparameters, bad exploration, bad luck, or a subtle bug in environment dynamics. You have no way to distinguish between them.

## The reproducibility crisis

The landmark paper on RL reproducibility is Henderson et al.'s ["Deep Reinforcement Learning That Matters"](https://arxiv.org/abs/1709.06560). Their findings are devastating.

For TRPO on HalfCheetah, they split identical trials — same algorithm, same hyperparameters, same environment — into two groups of five and compared them. The groups produced statistically different distributions (t=-9.09, p=0.0016). Same everything, different results.

Different implementations of the same algorithm produced wildly different outcomes: TRPO on Hopper across rllab and OpenAI Baselines showed a 47% performance gap from implementation choice alone.

Network architecture sensitivity was extreme. PPO on Hopper with a (400,300) network scored 61. The same algorithm with a (64,64) network scored 2,790. A 45x difference from architecture alone.

Even the implementation details of your optimizer matter. [Research on RLHF with PPO](https://iclr-blogposts.github.io/2024/blog/the-n-implementation-details-of-rlhf-with-ppo/) found that a numerical difference between PyTorch and TensorFlow's Adam implementations caused 6x differences in key training metrics. A framework choice most practitioners would consider irrelevant.

## The cost

RL training at scale is expensive and getting more so.

Nathan Lambert [estimates the cost progression](https://www.interconnects.ai/p/the-state-of-post-training-2025) for Meta's models: LLaMA (2023) at under $1M for instruction tuning, Llama 2 at $10-20M with RLHF, and Llama 3.1 at over $50M with a roughly 200-person post-training team. Even Tulu 3, an academic project that purchased zero human data, he estimates at over $1M.

According to [Introl](https://introl.com/blog/reinforcement-learning-infrastructure-rlhf-robotics-gpu-clusters-2025), RLHF training spends roughly 80% of compute on sample generation. A 70B Actor plus 70B Reference plus separate Reward and Critic models may require 8-16 H100 GPUs just for model weights, before optimizer states and activations.

Epoch AI [estimated](https://epoch.ai/gradient-updates/what-went-into-training-deepseek-r1) DeepSeek R1's initial RL phase at around 6.1e23 FLOP, or roughly $1M assuming training-level GPU efficiency. They note that if RL efficiency was significantly worse than pretraining, the cost could rival the entire pretraining budget.

Environment construction itself is expensive. Semi Analysis [reports](https://newsletter.semianalysis.com/p/rl-environments-and-rl-for-science) that UI gym environments cost about $20,000 per website, and that OpenAI has purchased hundreds for ChatGPT Agent training. _"Most RL data and tasks must be constructed from scratch, which can be quite labor intensive."_

## The human data problem

Behind every reward model is human preference data. That data is noisier than anyone wants to admit.

OpenAI's InstructGPT paper [reported](https://arxiv.org/pdf/2203.02155) that training labelers agreed with each other only 72.6% of the time. Roughly 1 in 4 preference labels has disagreement.

As Chip Huyen [put it](https://huyenchip.com/2023/05/02/rlhf.html): _"Human preferences are diverse and impossible to capture in a single mathematical formulation."_ She also noted that RLHF actually made hallucination worse — _"Hallucination is worse for InstructGPT (RLHF + SFT) compared to just SFT."_

Casper et al. [catalogued the fundamental limitations](https://arxiv.org/abs/2307.15217): annotators cut corners because they are paid per example. The paper cites research suggesting that as little as 0.5% poisoned data can compromise model behavior. And: _"A single reward function cannot represent a diverse society of humans, as RLHF is typically formulated as a solution for aligning an AI system with a single human, but humans are highly diverse in their preferences."_

The human cost is real too. TIME [reported](https://time.com/6247678/openai-chatgpt-kenya-workers/) that OpenAI used outsourced Kenyan laborers earning less than $2 per hour to label toxic content, with workers describing being mentally scarred. The contracting company canceled the work eight months early.

Epoch AI [interviewed practitioners](https://epoch.ai/gradient-updates/state-of-rl-envs) and heard the same themes:

> "Finding the experts isn't that hard, but managing them and doing quality control is hard."

> "Maintaining quality while scaling is the number one bottleneck that people see."

> "Domain knowledge and expert level prompting is more important than ML skills."

## The Goodhart problem

The deeper issue is mathematical. Goodhart's law — "when a measure becomes a target, it ceases to be a good measure" — has [four distinct variants](https://www.researchgate.net/publication/323747167_Categorizing_Variants_of_Goodhart's_Law), all of which apply to RLHF:

**Regressional:** Selecting for a proxy also selects for the difference between the proxy and the goal.

**Extremal:** Optimization pushes outside the regime where the proxy was valid. The paper's example: _"Basketball ability may generally correlate with height, up to a point, but the tallest people in the world actually have health problems that make them poor basketball players."_

**Causal:** Correlation between proxy and goal is non-causal. Intervening on the proxy does not intervene on the goal.

**Adversarial:** Optimizing for a proxy provides incentive for adversaries — or the model itself — to game it.

A [2025 ICLR paper](https://arxiv.org/abs/2410.05584) confirmed this empirically: _"Reward models with similar accuracy can behave quite differently in terms of overoptimization, suggesting that accuracy alone can be an inadequate metric for predicting downstream performance."_

And the alignment tax is real. [Research shows](https://arxiv.org/html/2309.06256v3) that RLHF causes measurable forgetting: _"Aligning LLMs under RLHF can lead to forgetting, which is also known as the alignment tax."_ The techniques to mitigate forgetting are at odds with RLHF performance, creating a fundamental trade-off.

## What is missing

The reward function is the most important component in RL training and it has the least tooling around it. Reward specifications are ad hoc code with no versioning, no validation, no composability. Episode metrics are scattered across logging frameworks. There is no standard way to trace what happened during a training episode.

According to Semi Analysis, environment construction costs [$20,000 per website](https://newsletter.semianalysis.com/p/rl-environments-and-rl-for-science). OpenAI's InstructGPT labelers [agreed with each other only 72.6% of the time](https://arxiv.org/pdf/2203.02155). And the models keep finding ways to hack whatever reward signal you give them.

The problem is clear. The tooling does not exist yet.

</article>

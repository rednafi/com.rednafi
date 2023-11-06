---
title: Oh my poor business logic
date: 2023-11-05
tags:
    - Meta
---

Adopting existing tools that work, applying them to the business problems at hand, and
quickly iterating in the business domain rather than endlessly swirling in the vortex of
technobabble is woefully underrated. I've worked at two kinds of companies before:

One that only cares about measurable business outcomes, accruing technical debt and blaming
engineers when no one wants to work with their codebase, ultimately hurting the product.
Another that has staff engineers spending all day on linter configurations and writing
seldom-read RFCs while juniors want to ditch Celery for Kafka because the latter is hip.

While both are equally bad, technical people love to lambaste the former while remaining
blissfully ignorant about the second type. Maybe because there's no incentive for doing that
and resume-driven development[^1] genuinely pays better. As long as companies keep making
people solve obscure puzzles that has nothing to do with the job or hiring managers keep
employing automated systems to look for keywords in resumes, a group of smart people will
always engage in techno-maximalism to prepare for the next big opportunity; setting the
underlying product up for failure.

The bigger and more established the company is, the more the properties of the second type
start manifesting. Add middle managers with zero ideas of what the worker bees are cooking
underneath, and you have the perfect recipe for disaster. Raking in absurd sums to tweak
linters or buttons may not be the worst thing in the world, if it also didn't lead these
bored people to dream of becoming architecture astronauts by introducing absurdly complex
tools to solve imaginary problems.

The situation exacerbates when companies start introducing useless metrics like LOCs, PR
counts, or the number of feature tickets as to quantify developer productivity. This often
leads to the creation of needless tickets and kickstarts the vicious PR cycle where
developers endlessly debate the best practices, micro-optimizations, gratuitous niceties,
and everything else other than the core business logic. If working on the business logic
isn't rewarded, why should anyone focus on making that better? Obviously it's more
profitable to introduce a dead letter queue to the callpath of an RPC instead of just
writing a retry decorator and monitoring if that works or not.

Now that microservices are no longer in vogue, and numerous companies have been burnt by
adopting the Netflix way of working, despite not having that level of revenue or manpower,
there's no shortage of articles[^2] on how bad it is to adopt SoA when a PostgreSQL-backed
Django monolith would probably do the job. Also, how terrible GraphQL is when a simple
denormalized secondary index would suffice, or how the high churn rate of JavaScript
frontend frameworks has wasted time, effort, and money. However, few of them mention how
organizational structures and policies force people to take that route.

There must be a middle ground where developers can focus on the core business logic that
yields the most value without incurring technical debt and making the development process a
nightmare. I don't have an answer for that, nor have I worked at a company that found the
perfect balance. Plus, I'm not a technical lead, manager, or business owner. So if you are
one of them, I'd love to hear how you or your organization plan to tackle this!

[^1]: [Resume-driven development](https://arxiv.org/abs/2101.12703)
[^2]: [The costs of microservices](https://robertovitillo.com/costs-of-microservices/)

---
title: Inversion of instrumentation
date: 2025-07-26
tags:
    - Go
    - Observability
    - Distributed Systems
---

_tl;dr_ — Instrumentation of your code can't be an afterthought. The workflow needs to be
inverted and included in your development cycle from the get-go, even if everything is a
no-op at the beginning. While we, as an industry, talk about the philosophy of this quite
often, there's few examples on where to even start. Observability vendors often make the
whole discussion space too noisy, and extracting the signal becomes impossibly difficult. So
much so that many just give up and hope to figure it out post hoc. This aims to spark some
conversation with a few overly simplified examples in Go on how someone might start to
navigate this messy space and include some instrumentations right from their first commit.

_Instrumentation isn't something you add, it's something you grow._

## _Genesis_

In broad strokes, [industrial programming] has two parts: development and operations. DevOps
tried to bring them closer and even succeeded to some extent. Then we gave it all up, and
DevOps practitioners moved away from development and closer to operations again, taking on
titles like SRE or Platform Engineer.

We've collectively discovered widely known practices like TDD, DDD, SOLID, Hexagonal
Architecture, and GoF patterns on the development side, and things like automation,
containerization, and orchestration on the operations side. But there's a lot less buzz
around the practices that need to stem from development to make operations smoother. Ops
folks can't do those in a silo without devs taking ownership of the overlapping concerns and
integrating those practices early in the dev workflow. Instrumentation is one of those concerns.

## _Post hoc instrumentation_

How often do you see fresh grads getting excited about benchmarking, instrumentation, or
black-box/white-box debugging? Way less than they are about reading _Clean Code_, applying
GoF patterns, or correcting someone's SOLID mistakes (pun intended). In practice,
oftentimes, a lot of this work ends up dumped into a vague bucket called _production
readiness_.

The problem with production readiness is that it almost always comes in as an afterthought.
And that's fine for things that don't depend on the development process, like writing
runbooks, hardening security, or setting up dashboards and alert thresholds. But for
dashboards to show the right data or alerts to trigger on the right signals, instrumentation
needs to be baked in from the start. Treating it as an afterthought either doesn't work or
ends up being prohibitively costly.

> _Do you bake observability right into your code as you’re writing it? The best engineers
> do a form of “observability-driven-development” — they understand their software as they
> write it, include instrumentation when they ship it, then check it regularly to make sure
> it looks as expected. You can't just tack this on after the fact, “when it’s done”._
>
> — _Charity Majors, Observability: A Manifesto_

## _The hurdles_

People gravitate towards post hoc instrumentation for a few reasons, but mostly because the whole space feels overwhelming when you just want to prototype something and get your idea out. Observability is probably the last thing on your mind at that point, much like tests. And like tests, retrofitting observability is a terrible experience all around in a brownfield application. But there are a few other reasons worth highlighting.


### _Ignoring vendor noise is hard_

Before the advent of OpenTelemetry, o11y vendors had the last say on how you'd instrument your code, and more so in enterprise scenarios. This resulted in a hundred different ways of collecting the basic signals like traces, metrics, and logs. The tooling scenario exploded into a wild ecosystem where no two applications were doing instrumentation in the same way. Vendor-specific SDKs would run rampant in your codebase, and often there was no migration path to a different vendor when things didn't work out. Also the shenanigans that vendors pull to stop you from switching once you're in.

On top of that, vendors publish opinionated and sometimes informative blog posts about what you should be doing. It’s incredibly hard to extract the useful part from the sales pitch, and many fall into the trap of pulling in their SDK, smearing their whole codebase with SDK calls, and thinking they’ve done observability. That works until the product starts failing, the price goes up, or the support goes quiet. Then you’re stuck with vendor glue deep in your logic and no easy way out.

Don’t start there. Don’t wire a vendor directly into your app logic. Don’t copy their example repo without reading the fine print. Don’t treat a marketing blog post like engineering guidance.

Use OpenTelemetry or something vendor-neutral. Emit signals in a standard way. Keep vendors at the edge. Pick where to send the data later. If you want to switch vendors, you should be flipping a config value, not rewriting your whole codebase.

You can still read vendor content. Just don’t let it dictate how you structure your instrumentation. Pull what you need. Leave the rest. Keep control.


### _Observability is a cross-cutting concern_

No matter how hard you try, instrumentation code will end up polluting your core logic. In a
typical setup, you pull in the OTEL or vendor SDK and start adding instrumentation. The
problem is, this instrumentation logic is incredibly hard to keep separate from your
business logic. Sometimes it seeps in so deeply that it completely eclipses what your
application is supposed to do.

> _A codebase, such as a web framework or a database client, takes a dependency on various
> signals in order to describe itself. OpenTelemetry instrumentation code can then be mixed
> into the other code within that codebase. This makes OpenTelemetry a cross-cutting
> concern, a piece of software which is mixed into many other pieces of software in order to
> provide value. Cross-cutting concerns, by their very nature, violate a core design
> principle: separation of concerns. As a result, OpenTelemetry client design requires extra
> care and attention to avoid creating issues for the codebases which depend upon these
> cross-cutting APIs._
>
> — OTEL docs

Even OTEL admits it will bleed into your code, and tries to keep the blast radius small.
Better OTEL than some vendor SDK running rampage through your codebase.

What we need are more discussions, blog posts, and examples that show how to reduce this
kind of pollution. Just like Hexagonal Architecture came about to push transport concerns
like HTTP and gRPC out of core business logic, we need similar patterns for observability.
Otherwise, o11y code ends up drowning out what your app is actually meant to do. We have to
think about these while we start writing our code, not afterward.

### _OTEL has simultaneously improved & complicated the situation_

OpenTelemetry’s biggest win is the promise of vendor neutrality. The project defines one
cross‑language API, one data model, and one wire protocol called OTLP, so the code that
creates telemetry is no longer tied to any particular backend.

> _One of the biggest advantages of the OpenTelemetry project is its vendor neutrality … we
> strive to provide a level playing field for all observability providers, avoid lock‑in to
> any vendor, and interoperate with other OSS projects in the telemetry and observability
> ecosystem._
>
> — Grafana Labs

That contract liberates teams from proprietary agents. You instrument once, send OTLP, and
decide later where the data lands. The same spans and metrics can flow to a SaaS platform
during evaluation, to an on‑prem cluster next quarter, or to two destinations at the same
time while you compare costs. The instrumentation code never changes, only the _collector_
configuration does. The promise is not having to yell at Datadog when their plan gets too expensive for you.

The flip side is the size of the toolbox. OpenTelemetry is not a single library but a
constellation of language SDKs, a _collector_ that can run as an agent or a gateway,
auto‑instrumentation bundles, semantic conventions that revise every quarter, resource
detectors for every cloud and container platform, and an operator for Kubernetes. Binding
all of those pieces to the same version stamp is a ton of work, and upgrade cadence can feel
relentless. Sometimes even wrapping your head around all of it as an application developers
feel too daunting and make you want to push it all to the ops folks.

Many people, including myself have complained and thrown the towel multiple times due to OTELs
complexity. The complaint is fair. The project tries to cover Go functions, JVM monoliths,
kernel eBPF probes, serverless handlers, and everything in between. Solving telemetry for
every language, runtime, and deployment model means the spec must keep widening, which
inevitably adds moving parts and surface area.

## _Navigating the chaos_

### _OTEL is good, embrace it_

Some tools are complex for the sake of complexity and some are so because the underlying problem they are trying to solve is sprawling. I fee like OTEL falls victim to the latter. But it's almost to the point where saying that "nobody got fired for suggesting OTEL" is going to be the norm soon.

Complexity aside, adopting OpenTelemetry delivers leverage that closed agents never will. It
is the second most active project in the Cloud Native Computing Foundation after Kubernetes,
with more than twelve hundred contributors pushing code every month. ([The New Stack][3])
That community momentum guarantees quick bug fixes, frequent signal additions, and
first‑class support from every serious observability vendor.

Because the contract is open, shifting providers becomes a configuration tweak, not a
rewrite. Point the _collector_ at two backends, run parallel dashboards for a week, drop the
loser, and keep the same code in production. Of course it won't as rosy in a real production environment but at least the economics change from sunk‑cost commitment
to an option that stays open.

The story does not end with commercial platforms. OTLP is equally at home in open source
stacks. The official documentation notes that exporters can send data straight to Jaeger for
traces, Prometheus or Mimir for metrics, and Loki for logs, using the very same protocol
that commercial systems accept. ([OpenTelemetry][4]) That path lets a team start with a
hosted service, migrate gradually to self‑hosted software when scale or budget demands, and
never touch the application code again.

In practice the rule is simple. Accept the overhead of the Collector and version juggling,
because the payoff is permanent freedom to choose where your telemetry lives and how much
you pay for it.

### _Even today, start with the three signals, née pillers_

### _Fight the cross-cutting concerns from polluting your core_

- make a package, call it `otel` or whatever and make sure your never import anything inside
  this other than otel sdk.
- other pkgs will depend on this but this won't depend on others.
- keep the mess contained behind simple methods, as much as possible.
- start with broad stroke instrumentation and dive inward from the edge to your core as
  necessary
- tune it according to your specific need, there's no one right way to do this

### _Keeping o11y stuff at the handler or the start is a good start_

```

```

### _Leverage semantic conventions_

### _Think about your sink, but not too much at the beginning_

<!-- references -->

<!-- prettier-ignore-start -->
[industrial programming]:
    https://peter.bourgon.org/go-for-industrial-programming/

<!-- prettier-ignore-end -->

[1]:
https://grafana.com/blog/2024/09/12/opentelemetry-and-vendor-neutrality-how-to-build-an-observability-strategy-with-maximum-flexibility/
"OpenTelemetry and vendor neutrality: how to build an observability strategy with maximum
flexibility | Grafana Labs " [2]:
https://www.datanami.com/2024/04/01/opentelemetry-is-too-complicated-victoriametrics-says/
"OpenTelemetry Is Too Complicated, VictoriaMetrics Says" [3]:
https://thenewstack.io/opentelemetry-whats-new-with-the-second-biggest-cncf-project/?utm_source=chatgpt.com
"OpenTelemetry: What’s New With the Second-Biggest CNCF Project?" [4]:
https://opentelemetry.io/docs/languages/js/exporters/ "Exporters | OpenTelemetry"

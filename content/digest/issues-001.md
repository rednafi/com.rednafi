---
title: "Issue #000 — Lessons from 3,000 incidents"
date: 2026-05-02
slug: issues-001
type_label: digest
description: >-
  Marc Brooker on what 3,000 postmortems taught him, plus a handful of Go and
  infra reads.
---

Hiya! Starting this at a coworker's suggestion. Not sure if it'll be worth your time, but
here we go.

A short digest of what I've read, written, or found interesting. If you've been around this
site, you already know the theme: databases, distributed systems, platform shenanigans, and
software patterns.

## Wrote

- [Hoisting wire plumbing out of your Go handlers](/go/hoist-wire-plumb/). Four of the five
  steps in every unary RPC handler are wire plumbing. Pin the service function signature and
  they fit into a single generic adapter per transport.

- [Go quirks: function closures capturing mutable references](/go/closure-mutable-refs/). A
  Go closure keeps a live reference to whatever it captures, not a snapshot. I tried to list
  a few common footguns that appear in concurrent execution due to this.

## Read

- [NilAway: Practical Nil Panic Detection for Go](https://www.uber.com/blog/nilaway-practical-nil-panic-detection-for-go/).
  Uber's static analyzer for catching nil panics in Go. I found the process of tracing the
  fault lines quite interesting. In practice though, I ran into a ton of false positives. It
  also has a few limitations, like not being able to analyze complex code paths with deeply
  nested function calls.

- [Data Race Patterns in Go](https://www.uber.com/blog/data-race-patterns-in-go/). Uber ran
  the race detector across their Go monorepo for six months. They wrote up the patterns
  behind the roughly 1,100 races they fixed. I've read it like 3 times. I love Go, but the
  concurrency footguns at scale demand higher-level abstractions than the primitives
  available. My biggest pet peeve is Go not having common constructs for basic
  [structured concurrency](/go/structured-concurrency/). This results in extremely verbose
  code where it's fairly easy to make mistakes.

- [What is a service mesh?](https://linkerd.io/what-is-a-service-mesh/) I've been playing
  around with service meshes at home and wanted to explore a bit more. This was a fun intro.

## Watched / Listened

- [AWS Distinguished Eng: Learning From 3000 Incidents](https://www.youtube.com/watch?v=u3GjIXP9N0s).
  Ryan Peterman sits down with Marc Brooker, an AWS distinguished engineer. They get into
  what 3,000 postmortems taught him, why caches are often a foot-gun, and what AI is doing
  to engineering work. This was so good that I listened to it in a single go while doing
  house chores.

- [Cup o' Go #156: OpenAPI 3.1 in kin-openapi, and a critical look at agentic coding](https://cupogo.dev/episodes/openapi-3-1-0-support-in-kin-openapi-and-a-critical-look-at-agentic-coding).
  kin-openapi finally lands OpenAPI 3.1 support. Git 2.54 ships. And the hosts are pretty
  skeptical of agentic coding, which I appreciated.

## Aside

I've been doing a ton of k8s munging these days, both on my homelab and at work. So revising
the third edition of
_[Kubernetes: Up and Running](https://www.amazon.com/Kubernetes-Running-Dive-Future-Infrastructure/dp/109811020X)_
has been quite useful.

---
title: What belongs in Go's context values?
date: 2026-03-17
slug: what-belongs-in-go-context-values
tags:
    - Go
    - API
    - Distributed Systems
description: >-
  A simple litmus test for when to use context values in Go.
---

Another common [question] popped up in r/golang:

> I've been reading mixed opinions lately about using context to pass values like
> request IDs, auth info, or tenant IDs through middleware layers. Some people argue
> it's fine and exactly what context was extended for after 1.7. Others say it's a code
> smell that leads to hidden dependencies and untestable code. I see both sides. On one
> hand it keeps function signatures clean. On the other hand you lose compile-time safety
> and it's not obvious what a function needs from ctx.
>
> Curious how the community here approaches this. Do you use typed getters and setters
> with context or avoid it entirely in favor of explicit parameters?

---

I think this one is easier to answer. I took a stab at it in a [comment] there. But before
expanding on it, the canonical definition of `context` from the [stdlib doc] helps:

> Package context defines the Context type, which carries deadlines, cancellation signals,
> and other request-scoped values across API boundaries and between processes.

So context exists for three things: deadlines, cancellation signals, and request-scoped values.
Anything that doesn't fall into one of those three shouldn't be in a context. The first two are
clear enough. "Request-scoped values" is where people get confused.

There's a simpler litmus test for it. If your code cannot proceed without some value, that
value should not go in a context. All context values must be optional, but not all optional
values belong in context.

Your application can't do much without a user ID or a database connection. Those are hard
dependencies. Your function needs them to do its job, so they belong in the function signature.
On the other hand, your app runs just fine without a trace ID or a request ID. Nothing breaks
if they're missing. That's context territory.

The [Google Go Style Guide] says the same thing:

> Values of the `context.Context` type carry security credentials, tracing information,
> deadlines, and cancellation signals across API and process boundaries.

And separately:

> If you have application data to pass around, put it in a parameter, in the receiver,
> in globals, or in a `Context` value if it truly belongs there.

Notice what the style guide lists as context-appropriate: security credentials, tracing
information, deadlines, cancellation signals. All cross-cutting infrastructure concerns. They
flow through the call chain without affecting what your function actually computes.

What belongs in context values:

- Trace IDs, request IDs, correlation IDs
- Authentication tokens for middleware propagation
- Logging attributes like request-scoped logger fields
- Idempotency keys

What doesn't:

- User data that your function needs to operate
- Database connections or service clients
- Configuration values
- Business logic inputs

Here are a couple of examples from well-known Go projects.

Prometheus stores query origin metadata in context for [logging]. The engine can execute
queries without it. When present, the metadata gets attached to log entries:

```go
// prometheus/promql/engine.go

type QueryOrigin struct{}

func NewOriginContext(
    ctx context.Context, data map[string]any) context.Context {
    return context.WithValue(ctx, QueryOrigin{}, data)
}

// During query logging:
if origin := ctx.Value(QueryOrigin{}); origin != nil {
    for k, v := range origin.(map[string]any) {
        f = append(f, slog.Any(k, v))
    }
}
```

etcd stores operation [traces] in context. If a trace is present, timing and step data get
recorded. If not, a no-op trace is returned and the operation proceeds normally:

```go
// etcd/pkg/traceutil/trace.go

type TraceKey struct{}

func Get(ctx context.Context) *Trace {
    if trace, ok := ctx.Value(TraceKey{}).(*Trace); ok && trace != nil {
        return trace
    }
    return TODO()
}

func EnsureTrace(
    ctx context.Context, lg *zap.Logger,
    operation string, fields ...Field) (context.Context, *Trace) {
    trace := Get(ctx)
    if trace.IsEmpty() {
        trace = newTrace(operation, lg, fields...)
        ctx = context.WithValue(ctx, TraceKey{}, trace)
    }
    return ctx, trace
}
```

Both follow the same pattern. The context values are infrastructure concerns, not business
inputs. Prometheus uses it for query observability. etcd uses it for operation tracing. Neither
changes the outcome of the core operation. The code works correctly with or without the value
being present.

So no, using context for request-scoped values isn't an anti-pattern. It's what context was
designed for. The confusion comes from a loose reading of "request-scoped." Stick to the litmus
test: if the function can't work without it, it's a parameter, not a context value.

---

_[Paweł Grzybek] asked whether passing a user ID from auth middleware to a handler through
context violates the litmus test above. The short answer is no. I answered in a
[follow-up shard]._

<!-- references -->
<!-- prettier-ignore-start -->

[Paweł Grzybek]:
    https://bsky.app/profile/pawelgrzybek.com

[follow-up shard]:
    /shards/2026/03/user-id-through-context/

[question]:
    https://www.reddit.com/r/golang/comments/1rvjpyw/

[comment]:
    https://www.reddit.com/r/golang/comments/1rvjpyw/comment/oazfcay/

[stdlib doc]:
    https://pkg.go.dev/context

[Google Go Style Guide]:
    https://google.github.io/styleguide/go/decisions#contexts

[logging]:
    https://github.com/prometheus/prometheus/blob/main/promql/engine.go

[traces]:
    https://github.com/etcd-io/etcd/blob/main/pkg/traceutil/trace.go#L39

<!-- prettier-ignore-end -->

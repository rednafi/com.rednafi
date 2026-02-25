---
title: Debugging context cancellation in Go
date: 2026-02-24
slug: context-cancellation-cause
aliases:
    - /go/context_cancellation_cause/
tags:
    - Go
description: >-
    How Go 1.20's WithCancelCause and Go 1.21's WithTimeoutCause let you attach a reason
    to context cancellation, plus a gotcha with manual cancel and the stdlib pattern that
    covers every path.
---

I've spent more time than I'd like debugging `context canceled` and
`context deadline exceeded` errors. The error tells you a context was canceled but not why.
It could be any of:

- The client disconnected
- A parent deadline expired
- The server started shutting down
- Some code somewhere called `cancel()` explicitly

By default, there's no reason attached. Go 1.20 and 1.21 added cause-tracking functions to
the `context` package that fix this, but there's a subtlety with `WithTimeoutCause` that
most examples skip.

## What "context canceled" actually tells you

To see why this is frustrating, take something like this. A function that processes an
order by calling three services, one after another, under a shared 5-second timeout:

```go
func processOrder(ctx context.Context, orderID string) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)  // (1)
    defer cancel()  // (2)

    if err := checkInventory(ctx, orderID); err != nil {
        return err  // (3)
    }
    if err := chargePayment(ctx, orderID); err != nil {
        return err
    }
    return shipOrder(ctx, orderID)
}
```

- (1) creates a derived context that automatically cancels after 5 seconds
- (2) cleans up the timer when the function returns, standard practice per the
  [context package documentation]
- (3) if anything goes wrong, including a context cancellation, the error is returned as-is

The `return err` on line (3) is the common pattern, and it's where the debugging problem
starts. When a context gets canceled, the underlying reason is either `context.Canceled` or
`context.DeadlineExceeded`. Libraries wrap these in their own types (`*url.Error` for
`net/http`, gRPC status codes for `grpc`), but `errors.Is` still matches the sentinel.

So if `checkInventory` makes an HTTP call and the client disconnects while it's in flight,
the error that bubbles all the way up is:

```
context canceled
```

If the 5-second timeout fires while `chargePayment` is waiting on a slow payment gateway:

```
context deadline exceeded
```

Two sentinel errors. No reason, no origin, nothing. The caller of `processOrder` has no
idea what actually happened.

You'd think wrapping the error helps:

```go
if err := checkInventory(ctx, orderID); err != nil {
    return fmt.Errorf("checking inventory for %s: %w", orderID, err)
}
```

Now the log says:

```
checking inventory for ord-123: context canceled
```

Better. You know it happened during the inventory check. But you still don't know *why*
the context was canceled. Was it the 5-second timeout? A parent context's deadline? The
client hanging up? A graceful shutdown signal? The error doesn't say.

Without the cause, you can't tell whether to retry, alert, or ignore, and your logs don't
give on-call enough to triage.

When this happens in production, you end up scanning logs for other errors around the same
timestamp, hoping something nearby gives you a clue. If the logs don't help, you trace the
context from where it was created, through every function that receives it, looking for
cancel calls and timeouts. In a small service this takes a few minutes. In a larger
codebase with middleware, interceptors, and nested timeouts, it can take a lot longer.

This has been a known pain point in the Go community for years. Bryan C. Mills noted this in [issue #26356] back in 2018:

> _I've seen this sort of issue crop up several times now. I wonder if context.Context should
> record a bit of caller information... Then we could add a debugging hook to interrogate
> *why* a particular context.Context was cancelled._
>
> _-- [bcmills on #26356]_

On [proposal #51365], which eventually led to the cause APIs, bullgare described the
production experience:

> _I had a case when on production I got random "context canceled" log messages. And in the
> case like that you don't even know where to dig and how to investigate it further. Or how
> to reproduce it on a local machine._
>
> _-- [bullgare on #51365]_

That proposal resulted in the cause APIs that shipped in [go 1.20 release notes].

## Attaching a cause with WithCancelCause

`context.WithCancelCause` gives you a `CancelCauseFunc` that takes an error instead of a
plain `CancelFunc`. Here's the same `processOrder` rewritten to use it:

```go
func processOrder(ctx context.Context, orderID string) error {
    ctx, cancel := context.WithCancelCause(ctx)
    defer cancel(nil)  // (1)

    if err := checkInventory(ctx, orderID); err != nil {
        cancel(fmt.Errorf(
            "order %s: inventory check failed: %w", orderID, err,
        ))  // (2)
        return err
    }
    if err := chargePayment(ctx, orderID); err != nil {
        cancel(fmt.Errorf(
            "order %s: payment failed: %w", orderID, err,
        ))
        return err
    }
    return shipOrder(ctx, orderID)
}
```

- (1) `cancel(nil)` as the default, sets the cause to `context.Canceled`
- (2) before returning the error, records a specific reason that includes the original
  error via `%w`

Now you can read the cause with `context.Cause(ctx)`. If `checkInventory` fails because of
a connection error, the cause comes back as:

```
order ord-123: inventory check failed: connection refused
```

Instead of just `context canceled`. You know it was the inventory check, you know it was a
connection error, and because the original error is wrapped with `%w`, the full error chain
is preserved for programmatic inspection.

The first call to `cancel` wins. Once a cause is recorded, subsequent calls are no-ops. So
`defer cancel(nil)` only takes effect if nothing else canceled the context first. This means
the most specific cancel, the one closest to the actual failure, is what gets recorded. If
`checkInventory` sets a cause and then `defer cancel(nil)` runs on the way out, the
inventory cause is preserved.

`context.Cause` is a standalone function rather than a method on `Context` because Go's
compatibility promise means the `Context` interface can't add new methods. `Err()` will
always return `nil`, `Canceled`, or `DeadlineExceeded`. If you call `context.Cause` on a
context that wasn't created with one of the cause-aware functions, it returns whatever
`ctx.Err()` returns. On an uncanceled context, it returns `nil`.

This handles explicit cancellation, but the function still has no timeout. The original
version used `WithTimeout` for the 5-second deadline. To label that timeout with a cause,
Go 1.21 added `WithTimeoutCause`:

```go
ctx, cancel := context.WithTimeoutCause(
    ctx,
    5*time.Second,
    fmt.Errorf("order %s: 5s processing timeout exceeded", orderID),
)
defer cancel()
```

When the timer fires, `context.Cause(ctx)` returns the custom error instead of a bare
`context.DeadlineExceeded`. There's also `WithDeadlineCause`, which is the same thing but
takes an absolute `time.Time`. If all you need is a label on the timeout path,
`WithTimeoutCause` works. But there's a subtlety in how it interacts with `defer cancel()`
that can silently discard your cause.

## Why defer cancel() discards the cause

`WithTimeoutCause` returns `(Context, CancelFunc)`, not `(Context, CancelCauseFunc)`. The
cancel function you get back doesn't accept an error argument. [Proposal #56661] defined it
this way explicitly:

```go
func WithTimeoutCause(
    parent Context, timeout time.Duration, cause error,
) (Context, CancelFunc)
```

Think about what happens when `processOrder` finishes normally in 100ms, well before the
5-second timeout:

```go
ctx, cancel := context.WithTimeoutCause(
    ctx,
    5*time.Second,
    fmt.Errorf("order %s: 5s timeout exceeded", orderID),
)
defer cancel()  // (1)
// ... returns in 100ms ...
```

- (1) `cancel()` fires on return, before the timer

If the timer fires first (the function ran too long), the context is canceled with
`DeadlineExceeded` and `context.Cause(ctx)` returns your custom message. That path works
correctly.

But if the function returns first, which is the common case, `defer cancel()` fires. Since
it's a plain `CancelFunc`, it can't take a cause argument. The Go source shows what it does
internally:

```go
return c, func() { c.cancel(true, Canceled, nil) }
```

It passes `Canceled` with a nil cause. Your custom cause only gets recorded when the
internal timer fires. On the normal return path, the cause is just `context.Canceled`.

This isn't a bug. `WithTimeoutCause` is a new function, so it could have returned
`CancelCauseFunc`. The Go team chose not to. rsc explained the reasoning when closing
[proposal #51365]:

> _WithDeadlineCause and WithTimeoutCause require you to say ahead of time what the cause
> will be when the timer goes off, and then that cause is used in place of the generic
> DeadlineExceeded. The cancel functions they return are plain CancelFuncs (with no
> user-specified cause), not CancelCauseFuncs, the reasoning being that the cancel on one of
> these is typically just for cleanup and/or to signal teardown that doesn't look at the
> cause anyway._
>
> _-- [rsc on #51365]_

He also acknowledged that this creates a subtle distinction between the two APIs:

> _That distinction makes sense, but it makes WithDeadlineCause and WithTimeoutCause
> different in an important, subtle way from WithCancelCause. We missed that in the
> discussion..._
>
> _-- [rsc on #51365]_

So `WithTimeoutCause` only carries the custom cause when the timeout actually fires. On the
normal return path and on any explicit cancellation path, `defer cancel()` discards it. If you
have a middleware that logs `context.Cause(ctx)` for every request, it'll see
`context.Canceled` instead of something useful on the most common path.

## Covering every path with a manual timer

The way around this is to skip `WithTimeoutCause` and wire the timer yourself using
`WithCancelCause`. Since there's only one `CancelCauseFunc`, every path goes through the
same door, and first-cancel-wins handles the rest. Here's `processOrder` one more time:

```go
func processOrder(ctx context.Context, orderID string) error {
    ctx, cancel := context.WithCancelCause(ctx)  // (1)
    defer cancel(errors.New("processOrder completed"))  // (2)

    timer := time.AfterFunc(5*time.Second, func() {
        cancel(fmt.Errorf("order %s: 5s timeout exceeded", orderID))  // (3)
    })
    defer timer.Stop()  // (4)

    if err := checkInventory(ctx, orderID); err != nil {
        cancel(fmt.Errorf("order %s: inventory check failed: %w", orderID, err))
        return err
    }
    if err := chargePayment(ctx, orderID); err != nil {
        cancel(fmt.Errorf("order %s: payment failed: %w", orderID, err))
        return err
    }
    return shipOrder(ctx, orderID)
}
```

- (1) one `CancelCauseFunc` for everything
- (2) the default cause if nothing else cancels first
- (3) the timer fires with a timeout-specific cause
- (4) stop the timer on normal return

Three possible paths, one cancel function. If the timer fires, `context.Cause(ctx)` returns:

```
order ord-123: 5s timeout exceeded
```

If `checkInventory` fails with a connection error:

```
order ord-123: inventory check failed: connection refused
```

On normal completion:

```
processOrder completed
```

This is actually what the stdlib does internally; `WithDeadline` uses `time.AfterFunc`
under the hood.

The trade-off is that `ctx.Err()` always returns `context.Canceled`, never
`context.DeadlineExceeded`, because you're using `WithCancelCause` instead of `WithTimeout`.
`ctx.Deadline()` also returns the zero value, which matters if downstream code or frameworks
use it to propagate deadlines (gRPC, for example, sends the deadline across service
boundaries via `ctx.Deadline()`). If downstream code branches on
`errors.Is(err, context.DeadlineExceeded)`, that check won't match either.

## When you also need DeadlineExceeded

If downstream code relies on `errors.Is(err, context.DeadlineExceeded)` to distinguish
timeouts from explicit cancellations, stack a `WithCancelCause` on top of a
`WithTimeoutCause`:

```go
func processOrder(ctx context.Context, orderID string) error {
    ctx, cancelCause := context.WithCancelCause(ctx)       // (1)
    ctx, cancelTimeout := context.WithTimeoutCause(         // (2)
        ctx,
        5*time.Second,
        fmt.Errorf("order %s: 5s timeout exceeded", orderID),
    )
    defer cancelTimeout()                                   // (3)
    defer cancelCause(errors.New("processOrder completed")) // (4)

    if err := checkInventory(ctx, orderID); err != nil {
        cancelCause(fmt.Errorf(
            "order %s: inventory check failed: %w", orderID, err,
        ))
        return err
    }
    if err := chargePayment(ctx, orderID); err != nil {
        cancelCause(fmt.Errorf(
            "order %s: payment failed: %w", orderID, err,
        ))
        return err
    }
    return shipOrder(ctx, orderID)
}
```

- (1) outer context for error-path and normal-completion causes
- (2) inner context with a timeout cause for the deadline path
- (3) deferred first, runs last (LIFO), cleans up the inner timeout context
- (4) deferred second, runs first (LIFO), cancels the outer context with a cause

When the timeout fires, the inner context gets canceled with `DeadlineExceeded` and the
custom cause. `errors.Is(ctx.Err(), context.DeadlineExceeded)` works as expected. On the
error path, `cancelCause(specificErr)` cancels the outer context, which propagates to the
inner. On normal completion, `cancelCause("processOrder completed")` runs first because of
LIFO defer ordering, canceling the outer and propagating to the inner. Then
`cancelTimeout()` finds the inner already canceled and does nothing.

> Notice the defer ordering. `cancelCause` must be deferred *after* `cancelTimeout` so it
> runs *before* it (LIFO). If you reverse them, `cancelTimeout()` cancels the inner context
> with `context.Canceled` before `cancelCause` gets a chance to set a meaningful cause.

One subtlety: after line (2), `ctx` points to the inner context. If you call
`context.Cause(ctx)` on it after a `cancelCause(specificErr)` call, you'll see
`context.Canceled` (propagated from the outer), not the specific error. The specific cause
lives on the outer context. In practice this doesn't matter because the caller inspects the
returned `error`, not `context.Cause`, but it's worth knowing if you add logging inside
`processOrder` itself.

The manual timer pattern is simpler and covers most cases. This stacked approach is for
when downstream code specifically relies on `errors.Is(err, context.DeadlineExceeded)`.

## Reading and logging the cause

`context.Cause` returns an `error`, so the full `errors.Is` and `errors.As` machinery works
on it. Since the cause in `processOrder` wraps the original error with `%w`, you can unwrap
through it to reach the underlying error.

If `checkInventory` failed because the inventory service refused the connection, the cause
is `"order ord-123: inventory check failed: connection refused"`, and the wrapped error is
a `*net.OpError`. You can pull it out:

```go
cause := context.Cause(ctx)

var netErr *net.OpError
if errors.As(cause, &netErr) {
    // The inventory service is unreachable.
    slog.Error("network failure",
        "op", netErr.Op,
        "addr", netErr.Addr,
    )
}
```

`errors.Is` works the same way. If the timer cause had wrapped `context.DeadlineExceeded`
(e.g., with `fmt.Errorf("order timeout: %w", context.DeadlineExceeded)`), you could check
for it:

```go
if errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
    // A timeout fired; maybe adjust the deadline or retry.
}
```

For logging, `ctx.Err()` and `context.Cause(ctx)` serve different purposes. `ctx.Err()`
gives you the category (cancellation or timeout), and `context.Cause(ctx)` gives you the
specific reason. Keeping them as separate structured log fields makes them easy to query:

```go
if ctx.Err() != nil {
    slog.Error("request failed",
        "err", ctx.Err(),
        "cause", context.Cause(ctx),
    )
}
```

That produces:

```
level=ERROR msg="request failed" err="context deadline exceeded"
    cause="order ord-123: 5s timeout exceeded"
```

A useful pattern is wrapping the request context with `WithCancelCause` at the middleware
level so every handler downstream gets automatic cause tracking:

```go
func withCause(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithCancelCause(r.Context())  // (1)
        defer cancel(errors.New("request completed"))         // (2)

        next.ServeHTTP(w, r.WithContext(ctx))

        if ctx.Err() != nil {  // (3)
            slog.Error("request context canceled",
                "method", r.Method,
                "path", r.URL.Path,
                "err", ctx.Err(),
                "cause", context.Cause(ctx),
            )
        }
    })
}
```

- (1) wrap the request context with `WithCancelCause`
- (2) default cause for normal completion
- (3) only fires if the context was canceled *during* request handling (client disconnect,
  handler cancel), not on normal completion — `defer cancel(...)` hasn't run yet at this
  point

Any handler deeper in the stack that calls `cancel(specificErr)` sets the cause. First
cancel wins, so the most specific reason is what shows up in the logs.

One thing to know: the stdlib's HTTP server and most third-party libraries cancel contexts
without setting a cause, since they predate Go 1.20. If a client disconnects,
`context.Cause(ctx)` will return `context.Canceled`, not a custom error. The cause APIs are
most useful for reasons set by your own code.

## Closing words

I default to the manual timer pattern now. The extra three lines over `WithTimeoutCause`
buy you causes on every path, and you stop wondering why things were canceled.

The cause APIs have seen steady adoption since Go 1.20. `golang.org/x/sync/errgroup` uses
`WithCancelCause` internally since v0.3.0, so `context.Cause(ctx)` on an errgroup-canceled
context returns the actual goroutine error. [docker cli] uses it to distinguish OS signals from normal cancellation.
[kubernetes cluster-api] migrated its codebase to the `*Cause` variants. gRPC-Go had a
[proposal] to use it for telling apart client disconnects, gRPC timeouts, and connection
closures (closed without implementation, but the motivation shows the pattern's appeal).

Runnable examples:

- [playground: the debugging problem]
- [playground: attaching a cause]
- [playground: the timeout gotcha]
- [playground: manual timer pattern]
- [playground: stacked contexts]
- [playground: reading and logging]

<!-- references -->
<!-- prettier-ignore-start -->

[context package documentation]:
    https://pkg.go.dev/context

[go 1.20 release notes]:
    https://go.dev/blog/go1.20

[proposal #51365]:
    https://github.com/golang/go/issues/51365

[proposal #56661]:
    https://github.com/golang/go/issues/56661

[issue #26356]:
    https://github.com/golang/go/issues/26356

[bcmills on #26356]:
    https://github.com/golang/go/issues/26356#issuecomment-404870718

[bullgare on #51365]:
    https://github.com/golang/go/issues/51365#issuecomment-1064461434

[rsc on #51365]:
    https://github.com/golang/go/issues/51365#issuecomment-1307812595

[docker cli]:
    https://github.com/docker/cli/blob/419e5d136cc8785f9aae7b36f068decedb9115e0/cmd/docker/docker.go#L56

[kubernetes cluster-api]:
    https://github.com/kubernetes-sigs/cluster-api/issues/11280

[proposal]:
    https://github.com/grpc/grpc-go/issues/7541

[playground: the debugging problem]:
    https://go.dev/play/p/sxY1R_yD15S

[playground: attaching a cause]:
    https://go.dev/play/p/zGkd2EzoYRS

[playground: the timeout gotcha]:
    https://go.dev/play/p/GfEv42EdKRc

[playground: manual timer pattern]:
    https://go.dev/play/p/WmX6WywiL7o

[playground: stacked contexts]:
    https://go.dev/play/p/ASYa0IngONt

[playground: reading and logging]:
    https://go.dev/play/p/l7XlYaAg0Qw

<!-- prettier-ignore-end -->

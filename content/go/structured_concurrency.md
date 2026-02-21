---
title: Structured concurrency & Go
date: 2026-02-21
slug: structured-concurrency
aliases:
    - /go/structured_concurrency/
tags:
    - Go
    - Python
    - Kotlin
description: >-
    How Python and Kotlin provide structured concurrency out of the box while Go achieves
    the same patterns explicitly using errgroup, WaitGroup, and context.
---

At my workplace, a lot of folks are coming to Go from Python and Kotlin. Both languages have
structured concurrency built into their async runtimes, and people are often surprised that
Go doesn't. The `go` statement just launches a goroutine and walks away. There's no scope
that waits for it, no automatic cancellation if the parent dies, no built-in way to collect
its errors.

This post looks at where the idea of structured concurrency comes from, what it looks like
in Python and Kotlin, and how you get the same behavior in Go using `errgroup`, `WaitGroup`,
and `context`.

## From goto to structured programming

In 1968, Dijkstra wrote a letter to the editor of Communications of the ACM titled [Go To
Statement Considered Harmful]. His core argument was that unrestricted use of `goto` made
programs nearly impossible to reason about:

> _The unbridled use of the go to statement has as an immediate consequence that it becomes
> terribly hard to find a meaningful set of coordinates in which to describe the process
> progress._

Structured programming replaced `goto` with scoped constructs like `if`, `while`, and
functions. The key insight was that control flow should be lexically scoped: you can look at
a block of code and know where it starts, where it ends, and that everything in between
finishes before execution moves on.

The same problem showed up later in concurrent programming.

## The same problem, but with concurrency

Spawning a thread or goroutine that outlives its parent is the concurrency equivalent of
`goto`. The spawned work escapes the scope that created it, and now you have to reason about
lifetimes that cross boundaries.

Martin Sustrik, creator of ZeroMQ, coined the term "structured concurrency" in his
[Structured Concurrency] blog post. He framed the idea as an extension of how block
lifetimes work in structured programming:

> _Structured concurrency prevents lifetime of green thread B launched by green thread A to
> exceed lifetime of A._

Eric Niebler later expanded on Sustrik's idea, tying it directly to how function calls work
in sequential code:

> _"Structured concurrency" refers to a way to structure async computations so that child
> operations are guaranteed to complete before their parents, just the way a function is
> guaranteed to complete before its caller._
>
> _-- Eric Niebler, [Structured Concurrency (Niebler)]_

Nathaniel J. Smith (NJS) took this further in his essay [Notes on structured concurrency]:

> _That's right: go statements are a form of goto statement._

NJS's broader point was that spawning a background task breaks function abstraction the same
way `goto` does. Once a function can spawn work that outlives it, the caller can no longer
reason about when the function's effects are complete:

> _Every time our control splits into multiple concurrent paths, we want to make sure that
> they join up again._

Structured concurrency boils down to a few rules:

- Concurrent tasks are spawned within a scope and can't outlive it
- If the parent scope is cancelled or a task fails, the remaining tasks are cancelled too
- The scope doesn't exit until all its tasks have finished
- Errors propagate from children back to the parent

This essay prompted [Go proposal #29011], filed by smurfix, which proposed adding structured
concurrency to Go. NJS participated in the discussion and made a point that stuck with me:

> _Right now you can structure things this way in Go, but it's way more cumbersome than just
> typing `go myfunc()`, so Go ends up encouraging the "unstructured" style._
>
> _-- Nathaniel J. Smith, [Go proposal #29011]_

The proposal was eventually closed. Before getting into Go's approach, it helps to see what
structured concurrency actually looks like in practice across the three languages.

## Python's TaskGroup

Python 3.11 introduced [asyncio.TaskGroup] as the structured concurrency primitive. Here's
an example that runs three tasks concurrently, where one of them fails:

```python
import asyncio


async def fetch(url: str, should_fail: bool = False) -> str:
    await asyncio.sleep(0.1)  # (1)
    if should_fail:
        raise ValueError(f"failed to fetch {url}")
    return f"fetched {url}"


async def main() -> None:
    try:
        async with asyncio.TaskGroup() as tg:  # (2)
            tg.create_task(fetch("/api/users"))  # (3)
            tg.create_task(fetch("/api/orders", should_fail=True))
            tg.create_task(fetch("/api/products"))
    except* ValueError as eg:  # (4)
        for exc in eg.exceptions:
            print(f"caught: {exc}")
    finally:
        print("cleanup runs no matter what")  # (5)


asyncio.run(main())
```

Here:

- (1) `await` is a cancellation point; the runtime can interrupt the coroutine here
- (2) `async with` creates a scope that waits for all tasks to finish
- (3) tasks are spawned inside the group and tied to its lifetime
- (4) if any task raises, the group cancels the remaining tasks and collects the errors
- (5) `finally` runs regardless of success or failure

The thing that makes this work is that `await` expressions are cancellation points. When the
group decides to cancel, the runtime delivers that cancellation at the next `await` in each
running coroutine.

## Kotlin's coroutineScope

Kotlin has had structured concurrency since kotlinx.coroutines 0.26. The equivalent
construct is [coroutineScope]. Here's the same scenario with three tasks and one failure:

```kotlin
import kotlinx.coroutines.*

suspend fun fetch(url: String, shouldFail: Boolean = false): String {
    delay(100)  // (1)
    if (shouldFail) throw IllegalStateException("failed to fetch $url")
    return "fetched $url"
}

suspend fun main() {
    try {
        coroutineScope {  // (2)
            launch { fetch("/api/users") }  // (3)
            launch { fetch("/api/orders", shouldFail = true) }
            launch { fetch("/api/products") }
        }
    } catch (e: IllegalStateException) {  // (4)
        println("caught: ${e.message}")
    } finally {
        println("cleanup runs no matter what")  // (5)
    }
}
```

Here:

- (1) `delay` is a suspension point where cancellation can be delivered
- (2) `coroutineScope` waits for all children and cancels siblings if one fails
- (3) `launch` starts a coroutine tied to this scope
- (4) the exception propagates after all children are cancelled
- (5) `finally` runs as expected

Like Python's `await`, Kotlin's suspension functions (`delay`, channel operations, etc.) are
cancellation points. When the scope cancels, the runtime delivers a `CancellationException`
at the next suspension point in each running coroutine.

Kotlin also has [supervisorScope], which is the variant where siblings keep running when one
fails. We'll see the Go equivalent of that shortly.

## Go doesn't do this by default

Go's `go` statement is unstructured. When you write `go func() { ... }()`, the runtime
spawns a background goroutine and immediately moves on. The calling function doesn't wait
for it, doesn't get notified when it finishes, and has no way to cancel it. Unless you
explicitly synchronize with something like a `WaitGroup` or a channel, that goroutine can
outlive the function that spawned it. There's no built-in scope that ties their lifetimes
together.

But you can compose the same patterns using channels, `sync.WaitGroup`, `context`, and
`errgroup` from `x/sync`.

### errgroup for cancel-on-error

This is Go's equivalent of `TaskGroup` and `coroutineScope`. Same scenario: three tasks, one
fails, siblings get cancelled:

```go
func run() error {
    g, ctx := errgroup.WithContext(context.Background())  // (1)

    g.Go(func() error {  // (2)
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(100 * time.Millisecond):
            fmt.Println("fetched /api/users")
            return nil
        }
    })

    g.Go(func() error {  // (3)
        return fmt.Errorf("failed to fetch /api/orders")
    })

    g.Go(func() error {  // (4)
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(100 * time.Millisecond):
            fmt.Println("fetched /api/products")
            return nil
        }
    })

    err := g.Wait()  // (5)
    fmt.Println("cleanup runs no matter what")
    return err
}
```

Here:

- (1) creates a group with a derived context; if any goroutine fails, the context cancels
- (2) fetches users; observes cancellation via `ctx.Done()`
- (3) fails immediately, triggering cancellation of the shared context
- (4) fetches products; also observes cancellation
- (5) `Wait` blocks until all goroutines finish and returns the first non-nil error

> Notice how the Go version requires each goroutine to explicitly check `ctx.Done()`. In
> Python and Kotlin, the runtime handles that at `await`/suspension points. In Go, you wire
> it in yourself.

### WaitGroup for supervisor-like behavior

This is Go's equivalent of Kotlin's `supervisorScope`. Siblings keep running regardless of
individual failures:

```go
func run() []error {
    var (
        wg   sync.WaitGroup
        mu   sync.Mutex
        errs []error
    )

    urls := []string{"/api/users", "/api/orders", "/api/products"}

    for _, url := range urls {
        wg.Add(1)  // (1)
        go func() {
            defer wg.Done()
            time.Sleep(100 * time.Millisecond)
            if url == "/api/orders" {
                mu.Lock()
                err := fmt.Errorf("failed to fetch %s", url)
                errs = append(errs, err) // (2)
                mu.Unlock()
                return
            }
            fmt.Printf("fetched %s\n", url)
        }()
    }

    wg.Wait()  // (3)
    fmt.Println("cleanup runs no matter what")
    return errs
}
```

Here:

- (1) each goroutine increments the counter before launch
- (2) errors are collected but don't affect other goroutines
- (3) `Wait` blocks until all goroutines call `Done`

Those two examples cover Go's equivalents of the structured patterns in Python and Kotlin.
But the code looks noticeably different, and the reason comes down to how these runtimes
handle concurrent execution.

## Goroutines aren't coroutines

The fundamental difference between the Python/Kotlin approach and Go's approach comes down
to how cancellation gets delivered.

In Python, `async def` functions are coroutines. They run on a single-threaded event loop
and yield control at every `await`. In Kotlin, `suspend` functions are coroutines. They run
on cooperative dispatchers (which can be backed by thread pools) and yield at every
suspension point. Both languages have [colored functions] (`async`/`suspend`) - the "color"
means async functions can only be called from other async functions, which lets the runtime
track every point where a coroutine can yield. These yield points are also cancellation
points, so when a scope cancels, the runtime delivers the cancellation at the next such
point.

Go's goroutines aren't coroutines. They're functions running on a preemptive scheduler
backed by OS threads. The runtime multiplexes goroutines onto OS threads and can preempt
them, but it has no knowledge of application-level cancellation. There's no concept of a
"suspension point" where the runtime can inject a cancellation signal. A goroutine doing
CPU-bound work will keep running even if its context was cancelled. The goroutine has to
check `ctx.Done()` explicitly via a `select` statement.

Here's the cooperative pattern in Go:

```go
func worker(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():  // (1)
            return ctx.Err()
        default:
            doUnitOfWork()  // (2)
        }
    }
}
```

- (1) checks for cancellation on each iteration
- (2) does a chunk of work, then loops back to the cancellation check

And here's a goroutine that ignores cancellation:

```go
func busyWorker(ctx context.Context) {
    for {
        // CPU-bound work, never checks ctx.Done()
        heavyComputation()
    }
}
```

This goroutine will keep running until the process exits, regardless of whether its context
was cancelled.

Python and Kotlin workers also need to cooperate for cancellation to actually work. If a
coroutine does CPU-bound work without hitting an `await` or a suspension point, the runtime
can't interrupt it either.

In Python, a non-cooperative worker looks like this:

```python
async def stubborn_worker() -> None:
    while True:
        heavy_computation()  # (1)
```

- (1) no `await` anywhere, so the runtime never gets a chance to deliver cancellation

To make it cooperative, you insert an explicit cancellation check:

```python
async def cooperative_worker() -> None:
    while True:
        await asyncio.sleep(0)  # (1)
        heavy_computation()
```

- (1) `await asyncio.sleep(0)` yields control back to the event loop, giving it a chance to
  cancel this coroutine

In Kotlin, the same situation looks like this:

```kotlin
suspend fun stubbornWorker() {
    while (true) {
        heavyComputation()  // (1)
    }
}
```

- (1) no suspension point, so cancellation can't be delivered

To fix this, use `coroutineContext.ensureActive()` to check whether the coroutine's scope
has been cancelled:

```kotlin
suspend fun cooperativeWorker() {
    while (true) {
        coroutineContext.ensureActive()  // (1)
        heavyComputation()
    }
}
```

- (1) throws `CancellationException` if the scope has been cancelled

This isn't too different from what Go does with `ctx.Done()`. In all three languages, a
tight loop doing CPU-bound work won't cancel unless the worker explicitly checks. The
difference is that in Python and Kotlin, most standard library functions (`asyncio.sleep`,
`delay`, channel operations) are cancellation points by default, so you hit them naturally
in typical code.

## Explicit by design

Go's concurrency model is built on CSP (Communicating Sequential Processes). Goroutines
communicate via channels, not via structured scopes. The `go` statement is deliberately
low-level. It gives you a concurrent execution unit and gets out of your way.

Python and Kotlin start from the structured side and require you to opt out. Python's
`asyncio.create_task` outside a group, or Kotlin's `supervisorScope`, are the escape
hatches. Go starts from the unstructured side and requires you to opt in. `errgroup` and
`WaitGroup` are how you add structure. Different design priorities lead to different
defaults.

[Go proposal #29011] was closed after Ian Lance Taylor pointed out the practical problem:

> _I think these ideas are definitely interesting. But your specific suggestion would break
> essentially all existing Go code, so that is a non-starter._

In a later comment, he acknowledged that there are good ideas in the space but argued for
improving existing primitives rather than changing the language:

> _There are likely good ideas in the area of structured concurrency that we can do better
> at, in the language or the standard library or both._

NJS also noted that structured concurrency helps with error propagation, because when a
goroutine exits with an error, there is somewhere to propagate that error to. That's a real
shortcoming of the current model. The response from the Go team was that `errgroup`,
`context`, and `WaitGroup` already provide the building blocks, and language-level changes
weren't justified given the cost.

There's also a [Trio forum discussion] on Go's situation. NJS was cautious about overstating
the benefits, noting that structured concurrency wouldn't have prevented about a quarter of
the concurrency bugs in a [study on real-world Go bugs] they examined (classic race
conditions). But he pointed out that some of the hardest-to-understand bugs involved
standard library modules that spawned surprising background goroutines. That couldn't happen
in a language with truly scoped concurrency. He also observed that all mistakes in using
Go's `WaitGroup` API seemed like they'd be trivially prevented by structured concurrency.

## Making Go's concurrency more structured

If you're writing Go and want structured concurrency, there are a few practices that help.
The core idea is:

> _Never start a goroutine without knowing when it will stop._
>
> _-- Dave Cheney, [Practical Go]_

Here are some concrete ways to follow that:

- **Know the lifetime of every goroutine you spawn.** Before writing `go func()`, you should
  be able to answer: what signals this goroutine to stop, and what waits for it to finish?
  If you can't answer both, the goroutine's lifetime is unknown and it can leak.

- **Use `go func()` sparingly.** A bare `go func() { ... }()` sends a goroutine into the
  background with no handle to wait on it or cancel it. Prefer launching goroutines through
  `errgroup` or behind a `WaitGroup` so something always owns their lifetime.

- **Let the caller decide concurrency.** If you're writing a library function, return a
  result instead of spawning a goroutine internally. Let the caller choose how to run it
  concurrently. This keeps goroutine lifetimes visible at the call site.

- **Pass context down, check it inside.** Accept `context.Context` as the first parameter
  and check `ctx.Done()` in long-running loops or blocking operations. This is how the
  caller communicates "I don't need this anymore."

Here's what a well-structured goroutine launch pattern looks like:

```go
func processItems(ctx context.Context, items []string) error {
    g, ctx := errgroup.WithContext(ctx)  // (1)

    for _, item := range items {
        g.Go(func() error {  // (2)
            select {
            case <-ctx.Done():
                return ctx.Err()
            default:
                return handle(ctx, item)  // (3)
            }
        })
    }

    return g.Wait()  // (4)
}
```

- (1) the group owns the goroutines and the context ties their lifetimes to the caller
- (2) each goroutine is launched through the group, so `Wait` knows about it
- (3) the actual work, which also receives the context for deeper cancellation
- (4) all goroutines finish before this function returns

Every goroutine has a clear owner and exit condition. If any task fails, the context cancels
and the others observe it on their next check.

## Catching mistakes

Since Go doesn't enforce structured concurrency at the language level, it's possible to leak
goroutines or miss cancellation signals. I wrote about one common case in [early return and
goroutine leak].

There are a few tools that help catch these issues:

- [goleak] is a library from Uber that you add to `TestMain`. It checks that no goroutines
  are still running when your tests finish. It's useful for catching the "forgot to cancel"
  class of bugs, which is the most common way unstructured goroutines cause trouble.
- The race detector (`go test -race`) catches data races between goroutines. It won't catch
  leaks, but unstructured goroutines with unclear lifetimes are more likely to race because
  their access to shared state is harder to reason about.
- [testing/synctest] (Go 1.24+) lets you test concurrent code with fake time. You can verify
  that goroutines exit when their context cancels or their parent scope ends, without
  relying on real `time.Sleep` calls that make tests slow and flaky.
- Go 1.26 adds an experimental [goroutine leak profile] via `runtime/pprof`. It uses the
  garbage collector's reachability analysis to find goroutines permanently blocked on
  synchronization primitives that no runnable goroutine can reach. Unlike `goleak`, which
  only works in tests, this profile can be collected from a running program via
  `/debug/pprof/goroutineleak`, making it useful for finding leaks in production.

## Closing words

If you're coming from languages like Python or Kotlin, Go's concurrency can feel overly
verbose, and it is. Wiring up `errgroup`, checking `ctx.Done()` in every goroutine, guarding
shared state with a mutex around a `WaitGroup`; that's a lot of ceremony for something the
other languages hand you for free.

But as covered earlier, the concurrency paradigms are fundamentally different. Python and
Kotlin's cooperative runtimes can own the cancellation because they own the scheduling. Go's
preemptive scheduler doesn't know what your goroutine is doing or when it should stop.
That's why cancellation is your job.

The same structured patterns are all achievable in Go. You just build them yourself out of
`errgroup`, `WaitGroup`, `context`, and channels. That gives you more control over goroutine
lifetimes, but it also means more surface area for bugs. Forget a `ctx.Done()` check and a
goroutine leaks. Misuse a `WaitGroup` and you deadlock. The [study on real-world Go bugs]
found 171 concurrency bugs across projects like Docker and Kubernetes, with more than half
caused by Go-specific issues around message passing and goroutine management.

<!-- references -->
<!-- prettier-ignore-start -->

[asyncio.TaskGroup]:
    https://docs.python.org/3/library/asyncio-task.html#asyncio.TaskGroup

[coroutineScope]:
    https://kotlinlang.org/api/kotlinx.coroutines/kotlinx-coroutines-core/kotlinx.coroutines/coroutine-scope.html

[supervisorScope]:
    https://kotlinlang.org/api/kotlinx.coroutines/kotlinx-coroutines-core/kotlinx.coroutines/supervisor-scope.html

[colored functions]:
    https://journal.stuffwithstuff.com/2015/02/01/what-color-is-your-function/

[Go To Statement Considered Harmful]:
    https://homepages.cwi.nl/~storm/teaching/reader/Dijkstra68.pdf

[Structured Concurrency (Niebler)]:
    https://ericniebler.com/2020/11/08/structured-concurrency/

[Structured Concurrency]:
    https://www.250bpm.com/p/structured-concurrency

[Notes on structured concurrency]:
    https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/

[Go proposal #29011]:
    https://github.com/golang/go/issues/29011

[Trio forum discussion]:
    https://trio.discourse.group/t/structured-concurrency-in-golang/174

[study on real-world Go bugs]:
    https://songlh.github.io/paper/go-study.pdf

[Practical Go]:
    https://dave.cheney.net/practical-go/presentations/qcon-china.html

[early return and goroutine leak]:
    /go/early-return-and-goroutine-leak/

[goleak]:
    https://github.com/uber-go/goleak

[testing/synctest]:
    https://pkg.go.dev/testing/synctest

[goroutine leak profile]:
    https://go.dev/doc/go1.26#goroutineleakprofile

<!-- prettier-ignore-end -->

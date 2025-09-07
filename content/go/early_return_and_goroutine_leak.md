---
title: Early return and goroutine leak
date: 2025-09-07
tags:
    - Go
    - Concurrency
---


At work, one of the most common mistakes I notice when reviewing candidates' home
assignments is how they wire goroutines to channels and then return early.

The pattern usually looks like this:

- start a few goroutines
- each goroutine sends a `result` to its own unbuffered channel
- in the main goroutine, read from those channels one by one
- if any read contains an error, return early

The trap is the early return. With an unbuffered channel, a send blocks until a receiver is
ready. If you return before reading from the remaining channels, the goroutines writing to
them block forever. That's a goroutine leak.

Here's how the bug appears in a tiny example: one worker intentionally fails, causing the
main goroutine to bail early. That early return skips the receive from `ch2`, leaving the
sender on `ch2` stuck.

```go
type result struct{ err error }

func Example() error {
    ch1 := make(chan result) // unbuffered
    ch2 := make(chan result) // unbuffered

    // Simulate a failing worker by sending an error into ch1.
    // This is intentional to trigger the early return below.
    go func() { ch1 <- result{err: fmt.Errorf("oops")} }()

    // Simulate a successful worker that will try to send into ch2.
    go func() { ch2 <- result{err: nil} }()

    // Receive the first result.
    res1 := <-ch1
    if res1.err != nil {
        // We return right away because of the error.
        // Because we never read from ch2, the goroutine sending to ch2
        // is now blocked forever on its send. That goroutine leaks.
        return res1.err
    }

    // This receive is skipped on the error path above.
    res2 := <-ch2
    if res2.err != nil {
        return res2.err
    }
    return nil
}
```

One simple fix is to make sure you always read from both channels before you decide what to
do. This guarantees that every send has a matching receive and no goroutine gets stuck:

```go
func ExampleDrain() error {
    ch1 := make(chan result)
    ch2 := make(chan result)

    go func() { ch1 <- result{err: fmt.Errorf("oops")} }() // same failure
    go func() { ch2 <- result{err: nil} }()                // same success

    // Always receive both. Both sends now complete.
    res1 := <-ch1
    res2 := <-ch2

    if res1.err != nil {
        return res1.err
    }
    if res2.err != nil {
        return res2.err
    }
    return nil
}
```

This is safe but it means you always wait for both workers even when the first one already
failed and the second result is irrelevant. If you want to return early without leaking,
another option is to use buffered channels so the producers do not block on send. A buffer
of size one is enough for this pattern.

```go
func ExampleBuffered() error {
    ch1 := make(chan result, 1) // buffered so sends do not block
    ch2 := make(chan result, 1)

    go func() { ch1 <- result{err: fmt.Errorf("oops")} }() // failure
    go func() { ch2 <- result{err: nil} }()                // success

    // Receive the first result and decide.
    res1 := <-ch1
    if res1.err != nil {
        // Safe to return early. The send to ch2 already completed
        // into its buffer even though we have not read it yet.
        return res1.err
    }

    // If we do continue, still read from ch2 to consume the buffered value.
    res2 := <-ch2
    if res2.err != nil {
        return res2.err
    }
    return nil
}
```

Buffered channels remove the blocked send, but they also make it easier to forget that a
second result exists at all. If that second value carries data you must process, you should
still receive it. If it is truly fire and forget, buffering is fine.

Often the cleanest approach is to drop the channel plumbing when you only need to run tasks
and aggregate errors. The `errgroup` package lets each goroutine return an error while the
group does the waiting. There is nothing to forget to receive, so there is nothing to leak.

```go
import (
    "fmt"
    "golang.org/x/sync/errgroup"
)

func ExampleErrgroup() error {
    var g errgroup.Group

    // Task 1 fails and returns an error.
    g.Go(func() error {
        return fmt.Errorf("oops")
    })

    // Task 2 succeeds.
    g.Go(func() error {
        return nil
    })

    // Wait waits for both tasks and returns the first error, if any.
    return g.Wait()
}
```

Sometimes you also want peers to stop once one task fails. `errgroup.WithContext` gives you
a context that gets canceled as soon as any task returns an error. You pass that context
into your workers and have them check `ctx.Done()` so they can exit quickly.

```go
import (
    "context"
    "fmt"
    "time"

    "golang.org/x/sync/errgroup"
)

func ExampleErrgroupWithContext() error {
    // When any task returns an error, ctx is canceled.
    g, ctx := errgroup.WithContext(context.Background())

    // Task 1 fails quickly to simulate an early error.
    g.Go(func() error {
        return fmt.Errorf("oops")
    })

    // Task 2 is long running but cooperates with cancellation.
    g.Go(func() error {
        for {
            select {
            case <-ctx.Done():
                // Exits because Task 1 failed and canceled the context.
                return ctx.Err()
            default:
                time.Sleep(10 * time.Millisecond)
            }
        }
    })

    return g.Wait()
}
```

At this point it is natural to ask if tools can catch the original bug for you. `go vet`
cannot. Vet is static analysis that runs at build time. Whether a send blocks depends on
runtime control flow and timing. Vet cannot prove that the function returns before a
particular receive in a general way, so it does not flag this pattern.

`go test -race` cannot either. The race detector detects unsynchronized concurrent memory
access. A goroutine stuck on a channel send is not a data race. You may see a test hang
until timeout, but the tool will not point to a leaking goroutine.

You can turn this into a failing test with [goleak] from Uber. `goleak` fails if
goroutines are still alive when a test ends. It snapshots all goroutines via the runtime,
filters out the standard background ones, and reports the rest. Wire it into a test that
triggers the early return and you will see the blocked sender's stack in the output.

Here is a test that leaks and fails:

```go
package example_test

import (
    "fmt"
    "testing"

    "go.uber.org/goleak"
)

type result struct{ err error }

func buggyEarlyReturn() error {
    ch1 := make(chan result)
    ch2 := make(chan result)

    // Force the early-return path by sending an error on ch1.
    go func() { ch1 <- result{err: fmt.Errorf("oops")} }()

    // This send will block forever on the failing path because nobody receives ch2.
    go func() { ch2 <- result{err: nil} }()

    r1 := <-ch1
    if r1.err != nil {
        return r1.err // leak: ch2 sender is stuck
    }

    _ = <-ch2
    return nil
}

func TestBuggyLeaks(t *testing.T) {
    defer goleak.VerifyNone(t) // fails if any goroutines are stuck at test end
    _ = buggyEarlyReturn()
}
```

This test fails and prints the goroutine stack stuck in the send to `ch2`.

```
=== RUN   TestBuggyLeaks
    main_test.go:34: found unexpected goroutines:
        [Goroutine 24 in state chan send, with thing.buggyEarlyReturn.func2 on top of the stack:
        thing.buggyEarlyReturn.func2()
                /Users/rednafi/canvas/rednafi.com/thing/main_test.go:20 +0x28
        created by thing.buggyEarlyReturn in goroutine 22
                /Users/rednafi/canvas/rednafi.com/thing/main_test.go:20 +0xc0
        ]
--- FAIL: TestBuggyLeaks (0.44s)
FAIL
exit status 1
```


If you switch the implementation to a fixed version, the test passes. For example, the draining fix:

```go
func fixedDrain() error {
    ch1 := make(chan result)
    ch2 := make(chan result)

    go func() { ch1 <- result{err: fmt.Errorf("oops")} }()
    go func() { ch2 <- result{err: nil} }()

    r1 := <-ch1
    r2 := <-ch2

    if r1.err != nil {
        return r1.err
    }
    if r2.err != nil {
        return r2.err
    }
    return nil
}

func TestFixedNoLeaks(t *testing.T) {
    defer goleak.VerifyNone(t)
    _ = fixedDrain()
}
```

If you prefer suite wide enforcement, add goleak to your `TestMain`. This way your entire
test run fails if any test leaks goroutines.

```go
package main

import (
    "os"
    "testing"

    "go.uber.org/goleak"
)

func TestMain(m *testing.M) {
    // VerifyTestMain wraps the whole test run
    // and fails if any goroutines are left behind.
    goleak.VerifyTestMain(m)
}
```

If you start goroutines that send on channels, think carefully about early returns. An
unbuffered send waits for a receive, and if you return before that receive happens, you've
leaked a goroutine.

You can avoid this by:

- always draining all channels
- buffering intentionally so sends don't block
- or using `errgroup`, with or without context, so tasks return errors and cooperate on
  cancellation

Add goleak to your tests so leaks surface early during development.

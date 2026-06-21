---
title: Channel iteration and goroutine leak
date: 2026-06-21
slug: channel-iteration-goroutine-leak
atprotoPath: /go/channel-iteration-goroutine-leak/
tags:
    - Go
    - Concurrency
description: >-
  A for-range over an unclosed channel leaks the receiver. Why three explicit receives
  are safe, why a range isn't, and how to catch it with Go 1.27's leak profile.
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mosowt5kgz2n"
---

I ran into the classic "_range over a channel_" leak while working on a custom cron
scheduler. I've debugged it on prod many times before, but writing one myself in a small
piece of code reminded me how easy it is to write bugs like this even when you know about
it.

Here:

- on each tick, the scheduler dispatches the jobs that are due
- each job reports its outcome on a channel
- one collector ranges over that channel to record the run

```go {hl_lines=["8","14","21"]}
// cron/scheduler.go
func tick(due []Job) []outcome {
    results := make(chan outcome)

    var wg sync.WaitGroup
    for _, j := range due {
        wg.Add(1)
        go func() {
            results <- outcome{job: j.Name, err: j.Run()} // (1)
        }()
    }

    var log []outcome
    go func() {
        for r := range results { // (2)
            log = append(log, r)
            wg.Done()
        }
    }()

    wg.Wait()
    // (3) no close(results)
    return log
}
```

- (1) each due job sends its outcome on the unbuffered channel
- (2) the collector ranges over `results`, recording each outcome and marking it done
- (3) once every job has reported, `wg.Wait` unblocks and `tick` returns

The producers are fine. Every send is matched by the collector's receive, so each job
goroutine sends once and exits. The collector is the problem. After the last outcome it
loops back to the range and waits for the next value, but nothing ever closes the channel.
So it blocks on that receive for the life of the process. Every tick leaks another one.

Drain that same channel by hand and it never leaks. Send three values and take exactly
three:

```go
ch := make(chan int)
go func() {
    <-ch
    <-ch
    <-ch
}()
ch <- 1
ch <- 2
ch <- 3
```

Three receives, then the goroutine returns. Swap those receives for a range and it leaks:

```go
ch := make(chan int)
go func() {
    for range ch {
    }
}()
ch <- 1
ch <- 2
ch <- 3
```

The two forms stop on different conditions. Three explicit receives stop on their own after
the third value. A `range` keeps reading until the channel closes. Back in the scheduler,
nothing closes `results`, so the ranging collector blocks on a receive that never completes.

The fix is the one line the buggy version is missing: close `results` once every job has
reported. The range ends and the collector returns:

```go {hl_lines=["2"]}
// cron/scheduler.go
// ...

    wg.Wait()
    close(results) // ends the range, the collector returns
    return log
```

> [!WARNING]
>
> Reaching for a buffered channel instead won't fix this. A `range` ends only when the
> channel is closed. No matter how big the buffer is, the receiver keeps waiting for a close
> that never comes.

This is a fairly well-documented leak. Uber called it [channel iteration misuse].

Typically you'd catch a leak like this with [goleak]:

- wire up goleak
- exercise the path that leaks in a test
- the test fails with the stuck goroutine's stack

I wrote about the goleak workflow in the [early return leak]. But goleak only catches the
leak if a test actually exercises the path that leaks. My scheduler tests never ran that
path, so goleak never saw it.

What caught it was [Go 1.27's new leak profile]. I was running it over my own code while
[writing about it], and it doesn't need a test at all. It leans on the garbage collector to
find goroutines blocked on something nothing can ever reach, and reports only those. Run it
at `debug=2` and the stuck collector shows up tagged `(leaked)`:

```
goroutine 25 [chan receive (leaked)]:
main.tick.func2()
    scheduler/main.go:43 +0x60
created by main.tick in goroutine 1
    scheduler/main.go:42 +0x15c
```

`main.tick.func2` is the collector, parked on the range at line 43. The profile finds leaks
like this deterministically, with no false positives and without a test ever exercising the
path.

Find the leak and the fix in the [example repo].

<!-- references -->
<!-- prettier-ignore-start -->

[channel iteration misuse]:
    https://www.uber.com/blog/leakprof-featherlight-in-production-goroutine-leak-detection/

[early return leak]:
    /go/early-return-and-goroutine-leak/

[goleak]:
    https://github.com/uber-go/goleak

[Go 1.27's new leak profile]:
    https://go.dev/doc/go1.27#goroutine-leak-profile

[writing about it]:
    /shards/2026/06/go-goroutine-leak-profile/

[example repo]:
    https://github.com/rednafi/examples/tree/main/channel-iteration-leak

<!-- prettier-ignore-end -->

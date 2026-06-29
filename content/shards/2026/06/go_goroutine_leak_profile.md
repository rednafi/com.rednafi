---
title: "Accepted proposal: a goroutine leak profile in the Go standard library"
slug: go-goroutine-leak-profile
date: 2026-06-18
description: >-
    Notes on Go's accepted goroutine leak profile and how it reuses the GC to find them.
tags:
    - Go
    - Concurrency
    - Profiling
images:
    - "https://blob.rednafi.com/shards/2026/06/go-goroutine-leak-profile/cover-a9bb2d7f6e5f.png"
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /shards/2026/06/go-goroutine-leak-profile/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mom6loqgyn2e"
---

Go 1.27 is getting a goroutine leak detector in `runtime/pprof`. The [proposal] was accepted
in April.

## A few common goroutine leaks

A goroutine leaks when it blocks on a channel or lock that nothing will ever release, so it
lingers for the life of the process. I've been using [uber-go/goleak] to catch them in
tests.

One is an early return that strands a sender, which I covered in [Early return and goroutine
leak]. It looks like this:

```go
func run(tasks []func() error) error {
    errs := make(chan error) // unbuffered
    var wg sync.WaitGroup
    for _, task := range tasks {
        wg.Go(func() { errs <- task() }) // (1)
    }
    for range tasks {
        if err := <-errs; err != nil {
            return err // (2)
        }
    }
    wg.Wait()
    return nil
}
```

Here:

- (1) each task sends its result on the unbuffered channel through `wg.Go`
- (2) the first error returns early, so the tasks still queued to send block forever

Giving `errs` a buffer big enough for every task, or draining all the results before
returning, keeps the sends from blocking.

A related leak shows up when you send a request to several replicas and keep only the first
answer:

```go
func replicate(replicas []func() string) string {
    results := make(chan string) // unbuffered
    for _, r := range replicas {
        go func() { results <- r() }() // (1)
    }
    return <-results // (2)
}
```

Here:

- (1) every replica races to send its answer on the unbuffered channel
- (2) the first answer returns, and the slower replicas block forever on their sends

Same as before, a buffer sized for every replica lets the slower ones send and exit.

Another is a forgotten `close`:

```go
func stream(work []int) {
    out := make(chan int)
    go func() {
        for v := range out { // (1)
            handle(v)
        }
    }()
    for _, v := range work {
        out <- v
    }
    // (2) no close(out)
}
```

Here:

- (1) the range keeps pulling from `out` until it's closed
- (2) `stream` returns without `close(out)`, so the range never ends and the goroutine leaks

The fix is to `close(out)` after the last send, which ends the range and lets the goroutine
return.

They're obvious once you spot them, but easy to let slip past under an early return or once
the surrounding code grows. goleak catches them in tests. In production you've got the
regular `/debug/pprof/goroutine` profile. It shows what each goroutine is blocked on, not
whether it will ever unblock, so you're guessing which are stuck for good and which are just
idle.

This list is nowhere near exhaustive, and not every leak is in your own code. A dependency,
or one of its transitive deps, can leak one too. Uber [catalogued the patterns across its Go
monorepo].

## The stdlib leak profile can now find them

It came out of Uber, the same place as goleak, and was designed by Vlad Saioc and Milind
Chabbi. The [detection rides on the garbage collector]. A goroutine is leaked when it's
blocked on a channel or lock that no runnable goroutine can reach, directly or through
another goroutine a runnable one could unblock. Nothing can ever wake it. The GC flags it.

> [!NOTE]
>
> Read that as a reachability test. If a goroutine is blocked on primitive `P`, and `P` is
> unreachable from any runnable goroutine or from any goroutine those runnable ones could
> unblock, then `P` cannot be unblocked. The goroutine can never wake up.

goleak and the profile answer different questions:

|                 | goleak                                 | `goroutineleak` profile              |
| --------------- | -------------------------------------- | ------------------------------------ |
| Asks            | what's still running you didn't expect | what can never run again             |
| How it decides  | a snapshot, no proof                   | a reachability proof, via the GC     |
| Works in        | tests, at teardown                     | a live process                       |
| False positives | yes, on a live server                  | none, only provably stuck goroutines |

The split is about where each one runs. At a test's teardown nothing should be left running.
Handing back whatever's there is exactly what you want from goleak. A live server is the
opposite. Most of its goroutines are blocked on purpose, waiting for the next request, and
goleak can't tell those from a real leak.

The profile proves it instead. It starts from the goroutines that can still run, follows
what they can reach, and rescues any blocked goroutine whose channel or lock is still in
play. Whatever's left has nothing that could ever touch it. It's stuck for good. Uber had
already tried [in-production leak detection] with a sampling tool, but sampling flags by
heuristic and turns up false alarms. The GC pass reports only goroutines it can prove are
stuck. That's the [no false positives] guarantee.

The profile ships without goleak's `VerifyNone(t)` or `VerifyTestMain(m)`. The [test
section] shows how to roll your own.

The API is tiny. There's no new type or function, just a profile named `goroutineleak`. It
ships registered, and the standard `pprof` tooling reads it like any other profile.

## You can pull the profile in the usual four ways

> [!NOTE]
>
> For now the profile is behind a build flag. Run the examples below with
> `GOEXPERIMENT=goroutineleakprofile`, or `pprof.Lookup("goroutineleak")` returns nil. Go
> 1.27 will make it [generally available] and drop the flag.

### From your own code

You pull the profile and write it out yourself. Start with `debug=0`, which dumps a gzipped
protobuf to a file:

```go
func main() {
    f, _ := os.Create("leak.pb.gz")
    pprof.Lookup("goroutineleak").WriteTo(f, 0)
}
```

`pprof.Lookup` returns the profile, and `WriteTo` runs the leak-detecting GC cycle before
writing it out. Open the file with `go tool pprof`, the same as a CPU or heap profile.

`WriteTo`'s second argument is the `debug` level. `1` and `2` give text instead, which you
can send straight to `os.Stdout`. At `debug=1`, a signal handler lets `kill -USR1 <pid>`
dump leaks on demand:

```go
// kill -USR1 <pid> to dump leaks
sig := make(chan os.Signal, 1)
signal.Notify(sig, syscall.SIGUSR1)
go func() {
    for range sig {
        pprof.Lookup("goroutineleak").WriteTo(os.Stdout, 1)
    }
}()
```

The text points straight at the goroutines that leaked:

```
goroutineleak profile: total 2
1 @ ...
#    0x...    main.leakSend.func1+0x27    formats/main.go:15
1 @ ...
#    0x...    main.leakRange.func1+0x33    formats/main.go:21
```

`debug=2` is a full goroutine dump, with the leaked goroutines tagged `(leaked)`:

```
goroutine 7 [chan send (leaked)]:
main.leakSend.func1()
    formats/main.go:15 +0x28
created by main.leakSend in goroutine 1
    formats/main.go:15 +0x6c

goroutine 8 [chan receive (leaked)]:
main.leakRange.func1()
    formats/main.go:21 +0x34
created by main.leakRange in goroutine 1
    formats/main.go:20 +0x6c
```

A normal dump reads `[chan send]` and `[chan receive]`. The `(leaked)` suffix is what the
profile adds.

### In a test

One helper runs the detection and returns whatever's stuck:

```go
func leaked() (string, bool) {
    p := pprof.Lookup("goroutineleak")
    if p == nil {
        return "", false // experiment off, nothing to detect
    }
    var b bytes.Buffer
    p.WriteTo(&b, 1)
    return b.String(), p.Count() > 0
}
```

For one test, wrap it in a `verifyNone` you `defer`:

```go
// verifyNone mirrors goleak.VerifyNone.
func verifyNone(t *testing.T) {
    t.Helper()
    if report, ok := leaked(); ok {
        t.Fatalf("leaked goroutines:\n%s", report)
    }
}

func TestRun(t *testing.T) {
    defer verifyNone(t)
    // ... exercise the code under test ...
}
```

For the whole suite, write a `verifyTestMain` and call it from `TestMain`:

```go
// verifyTestMain mirrors goleak.VerifyTestMain.
func verifyTestMain(m *testing.M) {
    code := m.Run()
    if code == 0 {
        if report, ok := leaked(); ok {
            fmt.Fprintf(os.Stderr, "leaked goroutines:\n%s", report)
            code = 1
        }
    }
    os.Exit(code)
}

func TestMain(m *testing.M) {
    verifyTestMain(m)
}
```

### Over HTTP

Importing `net/http/pprof` registers it on the default mux. Serve that mux, the `nil`
handler below, and the endpoint is live with no extra code:

```go
// registers /debug/pprof/goroutineleak on http.DefaultServeMux
import _ "net/http/pprof"

http.ListenAndServe("localhost:6060", nil)
```

Then read the profile off the endpoint:

```console
$ curl 'localhost:6060/debug/pprof/goroutineleak?debug=1'
goroutineleak profile: total 1
1 @ ...
#    0x...    main.main.func1+0x27    server/main.go:14
```

### With go tool pprof

`go tool pprof` reads it like any other profile, pointed at that endpoint or a saved
`debug=0` dump:

```console
$ go tool pprof -top 'http://localhost:6060/debug/pprof/goroutineleak'
Type: goroutineleak
      flat  flat%   sum%        cum   cum%
         1   100%   100%          1   100%  runtime.gopark
         0     0%   100%          1   100%  main.main.func1
         0     0%   100%          1   100%  runtime.chansend
         0     0%   100%          1   100%  runtime.chansend1
```

## What it can't catch

It won't catch every leak. The Go 1.27 notes admit it [can't catch every case] and only
promise a large class of them.

The reason is the no-false-positives rule. To avoid ever flagging a goroutine that could
still wake up, the GC leaves alone any whose channel or lock is still reachable. A global,
or the locals of a runnable goroutine, can keep that channel or lock reachable long after
anything will actually touch it, so the goroutine blocked on it goes unflagged. Everything
the profile reports is a real leak. Some real leaks just don't show up.

Every snippet here is a runnable program in the [example repo]. I ran them on the 1.26
toolchain and the profile flagged each leak at the exact line.

<!-- references -->
<!-- prettier-ignore-start -->

[proposal]:
    https://github.com/golang/go/issues/74609

[uber-go/goleak]:
    https://github.com/uber-go/goleak

[Early return and goroutine leak]:
    /go/early-return-and-goroutine-leak/

[detection rides on the garbage collector]:
    https://go.dev/design/74609-goroutine-leak-detection-gc

[generally available]:
    https://go.dev/doc/go1.27#goroutineleak-profiles

[can't catch every case]:
    https://go.dev/doc/go1.27#:~:text=impossible%20to%20detect%20permanently%20blocked

[no false positives]:
    https://go.googlesource.com/proposal/+/master/design/74609-goroutine-leak-detection-gc.md#:~:text=theoretically%20sound

[in-production leak detection]:
    https://www.uber.com/blog/leakprof-featherlight-in-production-goroutine-leak-detection

[test section]:
    #in-a-test

[catalogued the patterns across its Go monorepo]:
    https://arxiv.org/abs/2312.12002

[example repo]:
    https://github.com/rednafi/examples/tree/main/goroutine-leak-profile

<!-- prettier-ignore-end -->

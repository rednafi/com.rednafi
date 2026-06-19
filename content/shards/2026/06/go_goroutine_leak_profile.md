---
title: "Accepted proposal: a goroutine leak profile in the Go standard library"
date: 2026-06-18
slug: go-goroutine-leak-profile
atprotoPath: /shards/2026/06/go-goroutine-leak-profile/
tags:
    - Go
    - Concurrency
    - Profiling
description: >-
  Notes on Go's accepted goroutine leak profile and how it reuses the GC to find them.
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mom6loqgyn2e"
---

Go 1.27 is getting a goroutine leak detector in `runtime/pprof`. The [proposal] was accepted
in April.

## A few common goroutine leaks

A goroutine leaks when it blocks forever on a channel or a lock that nothing will ever
release, so it lingers for the life of the process. I've been using [uber-go/goleak] to
catch them in tests.

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

They're obvious once you spot them, but easy to let slip past, especially under an early
return or once the surrounding code grows. goleak catches them in tests, but not in a
running production process. There you're left reading `/debug/pprof/goroutine` and guessing
which of the blocked goroutines are stuck for good and which are just idle.

This list is nowhere near exhaustive. There are a ton of other ways to leak a goroutine by
accident, and not all of them are in your own code - a dependency or one of its transitive
deps can leak one too. Uber [catalogued the patterns across its Go monorepo].

## The standard library can now find them at runtime

It came out of Uber, the same place as goleak, and was designed by Vlad Saioc and Milind
Chabbi. The [detection rides on the garbage collector]. A goroutine is leaked when it's
blocked on a channel or lock that no runnable goroutine can reach, directly or through
another goroutine a runnable one could unblock. Nothing can ever wake it, so the GC flags
it.

It improves on goleak in two ways. It runs against a live process and catches the production
leaks goleak never sees, the kind of [in-production detection Uber built]. And because the
GC proves a leak instead of sampling for one, it has [no false positives]. Anything it flags
is blocked for good, not just slow or idle.

The profile ships without goleak's `VerifyNone(t)` or `VerifyTestMain(m)`. The [test
section] shows how to roll your own.

The API is tiny. There's no new type or function, just a profile named `goroutineleak`. It
ships registered and works with everything that already speaks `pprof`.

## You can pull the profile in the usual four ways

> [!NOTE]
>
> Until Go 1.27 the profile is behind a build flag. Run the examples below with
> `GOEXPERIMENT=goroutineleakprofile`, or `pprof.Lookup("goroutineleak")` returns nil. From
> 1.27 on, it's [generally available] and the flag is gone.

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
#	0x...	main.leakSend.func1+0x27	formats.go:15
1 @ ...
#	0x...	main.leakRange.func1+0x33	formats.go:21
```

`debug=2` is a full goroutine dump, with the leaked goroutines tagged `(leaked)`:

```
goroutine 7 [chan send (leaked)]:
main.leakSend.func1()
	formats.go:15 +0x28
created by main.leakSend in goroutine 1
	formats.go:15 +0x6c

goroutine 8 [chan receive (leaked)]:
main.leakRange.func1()
	formats.go:21 +0x34
created by main.leakRange in goroutine 1
	formats.go:20 +0x6c
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

Importing `net/http/pprof` registers it on a live server with no extra code:

```go
import _ "net/http/pprof" // registers /debug/pprof/goroutineleak

http.ListenAndServe("localhost:6060", nil)
```

Then read the profile off the endpoint:

```console
$ curl 'localhost:6060/debug/pprof/goroutineleak?debug=1'
goroutineleak profile: total 1
1 @ ...
#	0x...	main.main.func1+0x27	server.go:13
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
```

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

[no false positives]:
    https://go.googlesource.com/proposal/+/master/design/74609-goroutine-leak-detection-gc.md#:~:text=theoretically%20sound

[in-production detection Uber built]:
    https://www.uber.com/blog/leakprof-featherlight-in-production-goroutine-leak-detection

[test section]:
    #in-a-test

[catalogued the patterns across its Go monorepo]:
    https://arxiv.org/abs/2312.12002

[example repo]:
    https://github.com/rednafi/examples/tree/main/goroutine-leak-profile

<!-- prettier-ignore-end -->

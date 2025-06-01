---
title: Preventing accidental struct copies in Go
date: 2025-04-21
tags:
    - Go
    - TIL
---

By default, Go copies values when you pass them around. But sometimes, that can be
undesirable. For example, if you accidentally copy a mutex and multiple goroutines work on
separate instances of the lock, they won't be properly synchronized. In those cases, passing
a pointer to the lock avoids the copy and works as expected.

Take this example: passing a `sync.WaitGroup` by value will break things in subtle ways:

```go
func f(wg sync.WaitGroup) {
    // ... do something with the waitgroup
}

func main() {
    var wg sync.WaitGroup
    f(wg) // oops! wg is getting copied here!
}
```

`sync.WaitGroup` lets you wait for multiple goroutines to finish some work. Under the hood,
it's a struct with methods like `Add`, `Done`, and `Wait` to sync concurrently running
goroutines.

That snippet compiles fine but leads to buggy behavior because we're copying the lock
instead of referencing it in the `f` function.

Luckily, `go vet` catches it. If you run vet on that code, you'll get a warning like this:

```txt
f passes lock by value: sync.WaitGroup contains sync.noCopy
call of f copies lock value: sync.WaitGroup contains sync.noCopy
```

This means we're passing `wg` by value when we should be passing a reference. Here's the
fix:

```go
func f(wg *sync.WaitGroup) { // pass by reference
    // ... do something with the waitgroup
}

func main() {
    var wg sync.WaitGroup
    f(&wg) // pass a pointer to wg
}
```

Since this kind of incorrect copy doesn't throw a compile-time error, if you skip `go vet`,
you might never catch it. Another reason to always vet your code.

I was curious how the Go toolchain enforces this. The clue is in the vet warning:

```txt
call of f copies lock value: sync.WaitGroup contains sync.noCopy
```

So the `sync.noCopy` struct inside `sync.WaitGroup` is doing something to alert `go vet`
when you pass it by value.

Looking at the implementation of [sync.WaitGroup], you'll see:

```go
type WaitGroup struct {
    noCopy noCopy

    state atomic.Uint64
    sema  uint32
}
```

Then I traced the definition of `noCopy` in [sync/cond.go]:

```go
// noCopy may be added to structs which must not be copied
// after the first use.

// Note that it must not be embedded, due to the Lock and Unlock methods.
type noCopy struct{}

// Lock is a no-op used by -copylocks checker from `go vet`.
func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
```

Just having those no-op `Lock` and `Unlock` methods on `noCopy` is enough. This implements
the [Locker] interface. Then if you put that struct inside another one, `go vet` will flag
cases where you try to copy the outer struct.

Also, note the comment: don't _embed_ `noCopy`. Include it explicitly. Embedding would
expose `Lock` and `Unlock` on the outer struct, which you probably don't want.

The Go toolchain enforces this with the [copylock checker]. It's part of `go vet`. You can
exclusively invoke it with `go vet -copylocks ./...`. It looks for value copies of any
struct that nests a struct with `Lock` and `Unlock` methods. It doesn't matter what those
methods do, just having them is enough.

When vet runs, it walks the AST and applies the checker on assignments, function calls,
return values, struct literals, range loops, channel sends, basically anywhere values can
get copied. If it sees you copying a struct with `noCopy`, it yells. You can see the
implementation of the check here[^4].

Interestingly, if you define `noCopy` as anything other than a struct and implement the
`Locker` interface, vet ignores that. I tested this on Go 1.24:

```go
type noCopy int     // this is valid but vet doesn't get triggered
func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
```

This doesn't trigger vet. It only works when `noCopy` is a struct. The reason is that vet
takes a [shortcut] while checking when to trigger the warning. Currently, it explicitly
looks for a struct that satisfies the `Locker` interface and ignores any other type even if
it implements the interface.

You'll see this in other parts of the sync package too. `sync.Mutex` uses the same trick:

```go
type Mutex struct {
    _ noCopy

    mu isync.Mutex
}
```

Same with `sync.Once`:

```go
type Once struct {
    done   uint32
    m      Mutex
    noCopy noCopy
}
```

Here's a complete example of abusing `-copylocks` to prevent copying our own struct:

```go
type Svc struct{ _ noCopy }

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// Use this
func main() {
    var svc Svc
    _ = svc // go vet will complain about this copy op
}
```

Running `go vet` on this gives:

```txt
assignment copies lock value to s: play.Svc contains play.noCopy
call of fmt.Println copies lock value: play.Svc contains play.noCopy
```

---

Someone on Reddit asked me what actually triggers the `copylock` checker in `go vet`—is it
the struct's literal name `noCopy` or the fact that it implements the `Locker` interface?

The name `noCopy` isn't special. You can call it whatever you want. As long as it implements
the `Locker` interface, `go vet` will complain if the surrounding struct gets copied. See
this Go Playground [snippet].

<!-- Resources -->
<!-- prettier-ignore-start -->

[sync.WaitGroup]:
    https://cs.opensource.google/go/go/+/refs/tags/go1.24.2:src/sync/waitgroup.go;l=25-30

<!-- noCopy in sync/cond.go -->
[sync/cond.go]:
    https://cs.opensource.google/go/go/+/refs/tags/go1.24.2:src/sync/cond.go;l=111-122

[Locker]:
    https://github.com/golang/go/blob/336626bac4c62b617127d41dccae17eed0350b0f/src/sync/mutex.go#L37

[copylock checker]:
    https://cs.opensource.google/go/x/tools/+/master:go/analysis/passes/copylock/copylock.go;l=39;drc=bacd4ba3666bbac3f6d08bede00fdcb2f5cbaacf

<!-- copylock only checks for structs -->
[shortcut]:
    https://cs.opensource.google/go/x/tools/+/refs/tags/v0.32.0:go/analysis/passes/copylock/copylock.go;l=338

<!--The name noCopy isn't special-->
[snippet]:
    https://go.dev/play/p/M-vR6nOn00j

<!-- prettier-ignore-end -->

---
title: Safe mutation with mutex closures
date: 2026-03-05
slug: mutex-closure
aliases:
    - /go/mutex_closure/
tags:
    - Go
description: >-
    Why your mutex wrapper should accept a closure for mutation instead of a plain value,
    with examples from the standard library and Tailscale.
---

When you have shared mutable state in Go, a common approach is to bundle the value and its
mutex into a small generic wrapper. Callers get methods like `Get` and `Set` instead of
touching the fields directly. Something like this:

```go
type Locked[T any] struct {
    mu sync.Mutex
    v  T
}

func NewLocked[T any](initial T) *Locked[T] {
    return &Locked[T]{v: initial}
}

func (l *Locked[T]) Get() T {
    l.mu.Lock()
    defer l.mu.Unlock()
    return l.v
}

func (l *Locked[T]) Set(v T) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.v = v
}
```

You keep `mu` and `v` unexported, pass around `*Locked[T]`, and callers use `Get` to read
and `Set` to write:

```go
counter := NewLocked(0)

counter.Set(42)
fmt.Println(counter.Get()) // 42
```

This works fine when you're replacing the value wholesale - just call `counter.Set(42)` and
move on. But when your mutation depends on the current value, `Get` and `Set` can race
against each other.

## The problem with Set

Say you want to increment the counter instead of replacing it. You'd have to do:

```go
v := counter.Get()
v++
counter.Set(v)
```

Each individual call is safe - `Get` holds the lock while reading, `Set` holds it while
writing. But the three calls together aren't atomic. Between `Get` and `Set`, another
goroutine can modify the value, and your increment overwrites theirs. That's the classic
lost-update bug.

It gets worse with compound state. Say the wrapper holds a struct:

```go
type State struct {
    Count int
    Name  string
}

state := NewLocked(State{})
```

And you want to conditionally update both fields:

```go
s := state.Get()
if s.Count < 10 {
    s.Count++
    s.Name = fmt.Sprintf("item-%d", s.Count)
}
state.Set(s)
```

Same problem. `Get` returns a copy, you mutate the copy, then `Set` writes it back. If
another goroutine modified `state` between those two calls, your write clobbers it.

> [!IMPORTANT]
>
> The race detector (`go test -race`) won't catch this. It detects data races - two
> goroutines accessing the same memory without synchronization. Here, every `Get` and `Set`
> properly acquires the mutex, so each individual access is synchronized. The bug is a
> logical race (lost update), not a data race. The race detector sees nothing wrong.
>
> You can prove this with a simple test. Ten goroutines each increment the counter 1000
> times, so the final value should be 10000:
>
> ```go
> func TestSetValue(t *testing.T) {
>     counter := NewLocked(0)
>
>     var wg sync.WaitGroup
>     for range 10 {
>         wg.Go(func() {
>             for range 1000 {
>                 v := counter.Get()
>                 v++
>                 counter.SetValue(v)
>             }
>         })
>     }
>
>     wg.Wait()
>
>     got := counter.Get()
>     if got != 10000 {
>         t.Errorf("got %d, want 10000 (lost %d updates)", got, 10000-got)
>     }
> }
> ```
>
> Running `go test -race` produces no race warnings, but the test fails:
>
> ```txt
> === RUN   TestSetValue
>     locked_test.go:30: got 1855, want 10000 (lost 8145 updates)
> --- FAIL: TestSetValue (0.02s)
> ```
>
> The race detector is silent. The updates are just gone.

## Take a function instead

Instead of taking a value, have `Set` take a function:

```go
func (l *Locked[T]) Set(f func(*T)) {
    l.mu.Lock()
    defer l.mu.Unlock()
    f(&l.v)
}
```

Now the counter increment becomes:

```go
counter.Set(func(v *int) {
    *v++
})
```

And the compound mutation:

```go
state.Set(func(s *State) {
    if s.Count < 10 {
        s.Count++
        s.Name = fmt.Sprintf("item-%d", s.Count)
    }
})
```

The lock is held for the entire closure. There's no gap between reading and writing, so no
other goroutine can interfere. Both fields update together or not at all.

The function takes a pointer to `T` rather than a value of `T` for two reasons. First, it
lets you mutate the state in place instead of working on a copy. Second, if `T` is a large
struct, passing a pointer avoids copying the whole thing into the closure on every call.

## The stdlib already does this

Go's `database/sql` package has an internal [withLock] helper that follows the same pattern:

```go
// withLock runs while holding lk.
func withLock(lk sync.Locker, fn func()) {
    lk.Lock()
    defer lk.Unlock() // in case fn panics
    fn()
}
```

It's used throughout `database/sql` to serialize access to the underlying driver connection.
For example, when pinging a connection:

```go
if pinger, ok := dc.ci.(driver.Pinger); ok {
    withLock(dc, func() {
        err = pinger.Ping(ctx)
    })
}
```

Or when preparing a statement:

```go
withLock(dc, func() {
    si, err = ctxDriverPrepare(ctx, dc.ci, query)
})
```

Or committing a transaction:

```go
withLock(tx.dc, func() {
    err = tx.txi.Commit()
})
```

There are about 18 call sites in `sql.go` alone. In those snippets, `dc` is a
`*driverConn` - the struct that wraps a database driver connection. It embeds `sync.Mutex`
directly, so it satisfies `sync.Locker` and can be passed straight to `withLock`.

> [!NOTE]
>
> `withLock` accepts `sync.Locker` instead of `*sync.Mutex`, so it also works with the read
> side of an `RWMutex`:
>
> ```go
> withLock(rs.closemu.RLocker(), func() {
>     doClose, ok = rs.nextLocked()
> })
> ```
>
> Here `rs.closemu` is a `sync.RWMutex`, and `.RLocker()` returns a `sync.Locker` that
> acquires the read lock. The same `withLock` function handles both cases.

## The proposal to add this to sync

In 2021, twmb filed [proposal #49563] to add a `Mutex.Locked(func())` method to the standard
library:

```go
func (m *Mutex) Locked(fn func()) {
    m.Lock()
    defer m.Unlock()
    fn()
}
```

The idea was that if `sync.Mutex` had this method natively, you wouldn't need to write a
wrapper at all for simple cases - you'd just call `mu.Locked(fn)` directly. It also
eliminates forgotten unlocks and guards against panics leaving the mutex locked. esote
pointed out that `database/sql` already had an internal version of this - the same
`withLock` helper we saw earlier.

zephyrtronium raised the `sync.Locker` point:

> I think there are advantages to making this a function that takes a Locker rather than a
> method on Mutex. This would allow using it with either end of an RWMutex, or another
> custom Locker.
>
> -- [zephyrtronium on #49563]

rsc declined it on philosophical grounds:

> In general we try not to have two different ways to do something, and for better or worse
> we have the current idioms.
>
> -- [rsc on #49563]

The more interesting pushback came from bcmills, who argued the proposal didn't go far
enough. With generics arriving, he wanted something that also prevents unguarded access to
the protected data, not just forgotten unlocks:

> Now that we have generics on the way, I would rather see us move in a direction that
> _also_ eliminates unlocked-access bugs, not just incrementally update `Mutex` for
> forgotten-`defer` bugs.
>
> -- [bcmills on #49563]

He sketched out what that could look like:

```go
type Synchronized[T any] struct {
    mu  Mutex
    val T
}

func (s *Synchronized[T]) Do(fn func(*T)) {
    s.mu.Lock()
    defer s.mu.Unlock()
    fn(&s.val)
}
```

This is essentially the `Locked[T]` wrapper from the beginning of this post. The proposal
was declined, but bcmills' suggestion is the direction the community ended up going
anyway-just outside the standard library.

## Tailscale's MutexValue

Tailscale's [syncs] package has a `MutexValue[T]` type that follows this direction:

```go
type MutexValue[T any] struct {
    mu sync.Mutex
    v  T
}

func (m *MutexValue[T]) WithLock(f func(p *T)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    f(&m.v)
}

func (m *MutexValue[T]) Load() T {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.v
}

func (m *MutexValue[T]) Store(v T) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.v = v
}
```

They provide both `Store` for simple replacements and `WithLock` for compound mutations.
When you need to read-modify-write, you go through `WithLock` so the lock covers the whole
operation.

## When a plain Set is fine

If `T` is small and you only ever replace the whole value without reading it first, a plain
`Set` works. A boolean flag that gets toggled from one place, a config value that gets
swapped wholesale - those are fine.

But most state doesn't stay that simple. You start with a single integer, it becomes a
struct with three fields, and now you need to update two of them based on the third. At that
point, `Set(func(*T))` is the only safe option.

> [!IMPORTANT]
>
> The proposal benchmarks showed about 35% overhead for the closure-based approach (14.65
> ns/op vs 10.82 ns/op for direct lock/unlock) due to closures and `defer` not being
> inlineable. In practice this rarely matters. If your critical section does any real work,
> the lock overhead dominates.

<!-- references -->
<!-- prettier-ignore-start -->

[withLock]:
    https://cs.opensource.google/go/go/+/refs/tags/go1.24.0:src/database/sql/sql.go;l=3576

[proposal #49563]:
    https://github.com/golang/go/issues/49563

[zephyrtronium on #49563]:
    https://github.com/golang/go/issues/49563#issuecomment-968093753

[rsc on #49563]:
    https://github.com/golang/go/issues/49563#issuecomment-983955169

[bcmills on #49563]:
    https://github.com/golang/go/issues/49563#issuecomment-984092316

[syncs]:
    https://github.com/tailscale/tailscale/blob/v1.94.2/syncs/syncs.go#L110

<!-- prettier-ignore-end -->

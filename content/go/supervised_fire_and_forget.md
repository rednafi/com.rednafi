---
title: "Supervised fire-and-forget in Go"
slug: supervised-fire-and-forget
date: 2026-07-25
description: >-
    A bounded worker pool for small background tasks, with task-owned contexts, panic
    recovery, and graceful shutdown.
tags:
    - Go
    - Concurrency
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /go/supervised-fire-and-forget/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mrirbf5kti26"
---

These days, unmanaged `go func()` calls don't appear as often as they used to in the early
days of Go.

At this point, everyone knows Dave Cheney's maxim to ["never start a goroutine without
knowing how it will stop"]. Even so, I still occasionally run into unsynchronized
`go func()` calls doing fire-and-forget work.

I'm not talking about jobs handed to a dedicated task queue such as [Asynq]. In that case,
the task system owns the lifecycle of the tasks it starts. I mean smaller, best-effort jobs
where it's super tempting to start a task with `go func()` and just forget about it. Sending
a notification or writing an expensive diagnostic log often falls into this category.

It typically looks like this:

```go {hl_lines=["6","9"]}
func (h *handler) createOrder(w http.ResponseWriter, r *http.Request) {
    user := r.URL.Query().Get("user")
    ctx := r.Context()

    go func() {
        h.tasks.SendNotification(ctx, user) // (1)
    }()
    go func() {
        h.tasks.WriteDiagnosticLog(ctx, user) // (2)
    }()

    w.WriteHeader(http.StatusAccepted)
}
```

In the handler above:

- (1) sends a notification in a background goroutine
- (2) writes a diagnostic log in another background goroutine

The response doesn't wait for either one. On a long-running server this looks reasonable, as
`main` will most likely outlive both calls. But this suffers from a few issues:

- every request can start two more goroutines, with no limit on active work
- both tasks get `r.Context()`, which `net/http` cancels when the handler returns. If the
  jobs are cancellation aware (ideally they should be), they can bail before they're done
- a panic in either goroutine crashes the whole process, because nothing recovers it
- `main` has no way to wait for either task during shutdown. A restart kills whatever is
  still running

## Every task goes through one worker pool

One fix I've been using is a small worker pool backed by a buffered channel:

- each background task is sent to the channel as a `func()` closure
- workers started at the [composition root], usually `main`, drain the channel and run the
  closures
- the number of workers is fixed, so concurrency stays bounded
- nothing else in the application starts a background goroutine. All background work goes
  through this pool
- at shutdown, `main` closes the channel

In its simplest form, it looks like this:

```go {hl_lines=["8","15"]}
// Make a buffered channel in main.
tasks := make(chan func(), 64)

// Still in main, start a fixed set of workers to drain
// the channel and run the tasks.
for range 4 {
    go func() {
        for task := range tasks {
            task()
        }
    }()
}

// From a handler, or anywhere else, send a task as a closure.
tasks <- func() {
    doWork()
}

// At shutdown, close the channel.
close(tasks)
```

Each receive loop runs one task at a time, so four workers mean at most four tasks run at
once. The buffer holds 64 pending closures. Once it fills, the send blocks until a worker
receives a task. Closing the channel ends the receive loops. The workers drain whatever is
left in the buffer and exit.

## A real pool needs more than a channel

While the simple form above tackles the bounded concurrency issue, it doesn't handle a few
things that we need in a real worker pool:

- error handling: no caller waits for a result, so failures are handled inside the task. A
  panic must not crash the server
- context propagation: a task must not inherit the request's cancellation, but it may still
  need the request's values
- timeouts: once a task is detached from the request context, nothing cancels it. It needs
  its own deadline
- graceful shutdown: after a shutdown signal, every accepted task should finish before
  `main` exits

The following API aims to cover all of these requirements.

## The entire API is three functions

The [complete implementation] is a small `bg` package that wraps the channel in a struct
called `Background`:

```go
// internal/bg/bg.go
type Background struct {
    tasks   chan func()
    workers sync.WaitGroup
    onPanic func(any)

    mu      sync.RWMutex
    stopped bool
}

func New(capacity, workers int, onPanic func(any)) *Background
func (b *Background) Submit(task func()) bool
func (b *Background) Stop()
```

`New` builds the pool, `Submit` queues a task, and `Stop` shuts the pool down. The
constructor makes the channel and starts the workers (argument validation elided):

```go
// internal/bg/bg.go
func New(capacity, workers int, onPanic func(any)) *Background {
    background := &Background{
        tasks:   make(chan func(), capacity),
        onPanic: onPanic,
    }
    for range workers {
        background.workers.Go(background.work)
    }
    return background
}
```

`workers.Go` is [`WaitGroup.Go`], added in Go 1.25. It starts each worker goroutine and
tracks it in the group in one call. `Stop` waits on that group during shutdown.

Each worker drains the channel:

```go
// internal/bg/bg.go
func (b *Background) work() {
    for task := range b.tasks {
        b.run(task)
    }
}
```

This is the receive loop from the sketch. It keeps pulling tasks until the channel is closed
and drained. The `capacity` and `workers` arguments bound the concurrency: at most `workers`
tasks run at once, and at most `capacity` more wait in the queue.

`Submit` sends work to that channel:

```go
// internal/bg/bg.go
func (b *Background) Submit(task func()) bool {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.stopped {
        return false
    }
    b.tasks <- task
    return true
}
```

The send blocks when the queue is full, so a burst of requests waits for the workers instead
of spawning more goroutines. The boolean just says whether the task was accepted.

The `sync.RWMutex` exists because a send on a closed channel panics. `Stop` closes the
channel, so the pool has to make sure no `Submit` is halfway through a send when that
happens.

`Submit` sends while holding the read lock, which any number of goroutines can share. `Stop`
closes the channel while holding the write lock, which it can only take once every reader
has released it. That way the close can never land in the middle of a send, and once `Stop`
has run, a later `Submit` sees `stopped` and returns false instead of sending.

A plain `sync.Mutex` would prevent the panic just as well, but it would force submissions
through one at a time for no benefit. Sending on a channel is already safe from multiple
goroutines, so the submitters don't need protecting from each other. Only the close needs to
be kept apart from the sends, and the two halves of the `RWMutex` do exactly that.

The worker calls accepted tasks through `run`:

```go
// internal/bg/bg.go
func (b *Background) run(task func()) {
    defer func() {
        if value := recover(); value != nil {
            b.handlePanic(value)
        }
    }()
    task()
}

func (b *Background) handlePanic(value any) {
    if b.onPanic == nil {
        return
    }
    defer func() {
        _ = recover()
    }()
    b.onPanic(value)
}
```

`run` wraps every task in a deferred `recover`. When a task panics, the worker catches the
value instead of letting it crash the process. It hands the value to `handlePanic` and moves
on to the next task.

`handlePanic` forwards the value to the `onPanic` callback passed to `New`. Its own deferred
`recover` protects the worker in case the callback panics as well. The callback runs on
whichever worker caught the panic, so several workers may call it at once. It has to be safe
for concurrent use.

This covers the panic half of error handling. Ordinary errors stay inside the task, and the
next section comes back to that.

`Stop` closes admission and waits for the workers:

```go
// internal/bg/bg.go
func (b *Background) Stop() {
    b.mu.Lock()
    if !b.stopped {
        b.stopped = true
        close(b.tasks)
    }
    b.mu.Unlock()

    b.workers.Wait()
}
```

`Stop` takes the write lock, flips `stopped` so `Submit` refuses new tasks, and closes the
channel. Closing a channel doesn't discard its buffered values. The workers drain the
accepted tasks, then exit their receive loops. `workers.Wait` returns once every worker is
done.

`Stop` is safe to call more than once or from several goroutines. That's the pool's half of
graceful shutdown. The other half is the order `main` calls things in, which comes at the
end.

## Tasks build their own contexts

The task signature is just `func()`. It doesn't take a context or return an error.

For background work, you don't want to take the upstream context verbatim. The handler
context is canceled when the handler returns, so a background task that inherited it would
get canceled mid-run. Instead, each closure can create the context it needs:

```go {hl_lines=["3","7","16"]}
// order/handler.go
user := r.URL.Query().Get("user")
detached := context.WithoutCancel(r.Context()) // (1)

h.background.Submit(func() {
    ctx, cancel := context.WithTimeout(
        context.Background(), // (2)
        h.taskTimeout,
    )
    defer cancel()
    h.tasks.SendNotification(ctx, user)
})

h.background.Submit(func() {
    ctx, cancel := context.WithTimeout(
        detached, // (3)
        h.taskTimeout,
    )
    defer cancel()
    h.tasks.WriteDiagnosticLog(ctx, user)
})
```

In the two submissions:

- (1) [context.WithoutCancel] keeps the request's values, such as a request ID, but drops
  its cancellation and deadline
- (2) the notification only needs the extracted `user`, so its context starts from scratch
- (3) the diagnostic log may also need values from the request context, so it builds on the
  detached one

Both timeouts start when a worker calls the closure, not while it waits in the queue. The
task methods must pass the context to blocking calls. A timeout can't stop code that ignores
it.

Errors stay inside the closure too. Nobody waits for a result from fire-and-forget work. The
[example handler] logs errors there. A task could just as well update a metric or a trace.

## Stop the server before the pool

The [main program] creates the pool before starting the HTTP server:

```go
// cmd/main.go
background := bg.New(64, 4, logPanic)
defer background.Stop()
```

Here the queue holds 64 pending tasks, four workers drain it, and `logPanic` records a panic
value and stack. The defer shuts the pool down on every return path.

When `main` receives a shutdown signal, it stops the HTTP server before the workers:

```go
// cmd/main.go
shutdownCtx, cancel := context.WithTimeout(
    context.WithoutCancel(ctx),
    2*time.Second,
)
defer cancel()

if err := server.Shutdown(shutdownCtx); err != nil {
    return err
}

// The deferred background.Stop() runs when main returns.
```

The signal has already canceled `ctx`, so `WithoutCancel` removes that cancellation before
`WithTimeout` adds a two-second deadline.

`Server.Shutdown` stops new requests and waits for active handlers. Once it returns
successfully, those handlers have finished their `Submit` calls. The deferred
`Background.Stop` then closes admission, drains every accepted task, and waits for the
workers.

This is still an in-memory queue. A process crash loses accepted work. A task that ignores
its context can delay `Stop`, since Go can't force a goroutine to exit. If crash recovery or
retries matter, use a durable task queue such as Asynq.

The complete implementation is in the [example repo].

<!-- references -->
<!-- prettier-ignore-start -->

["never start a goroutine without knowing how it will stop"]:
    https://dave.cheney.net/2016/12/22/never-start-a-goroutine-without-knowing-how-it-will-stop

[Asynq]:
    https://github.com/hibiken/asynq

[composition root]:
    https://blog.ploeh.dk/2011/07/28/CompositionRoot/

[complete implementation]:
    https://github.com/rednafi/examples/blob/main/supervised-fire-and-forget/internal/bg/bg.go

[`WaitGroup.Go`]:
    https://pkg.go.dev/sync#WaitGroup.Go

[example handler]:
    https://github.com/rednafi/examples/blob/main/supervised-fire-and-forget/order/handler.go

[main program]:
    https://github.com/rednafi/examples/blob/main/supervised-fire-and-forget/cmd/main.go

[example repo]:
    https://github.com/rednafi/examples/tree/main/supervised-fire-and-forget

[context.WithoutCancel]:
    https://pkg.go.dev/context#WithoutCancel

<!-- prettier-ignore-end -->

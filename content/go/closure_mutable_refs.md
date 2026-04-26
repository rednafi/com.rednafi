---
title: "Go quirks: function closures capturing mutable references"
date: 2026-04-25
slug: closure-mutable-refs
tags:
    - Go
    - Concurrency
description: >-
  A Go closure holds a live reference to whatever it captures, not a snapshot.
  Real examples of where this trips people up, and how to keep it boring.
---

I was browsing the [hegel-go] codebase and ran into this rule in its [go-concurrency] agent
skill:

> **Function closures capturing mutable references**
>
> `conn.crashMessageFn = s.serverCrashMessage` captures `s` and reads `s.logFile` — any
> field the method touches is shared state. Prefer capturing immutable values (strings,
> ints) rather than pointers to mutable structs.

It's the most concise representation I've seen of the behavior that has bitten me in the
past.

Calling it a footgun would be a bit disingenuous. Every language has to pick how closures
see captured variables. Java lambdas can only read effectively-final locals, so the value is
frozen at the moment of capture. C++ makes you say up front whether each variable is
captured by value or by reference. Go made every closure [capture-by-reference], which can
lead to some surprising behavior.

## A closure with a pointer sees future writes

Take a `Client` whose `addr` function closes over a `*Config`, then mutate `cfg`:

```go {hl_lines=["14","20-21"]}
type Config struct {
    Host string
    Port int
}

type Client struct {
    addr func() string
}

cfg := &Config{Host: "localhost", Port: 8080}

c := &Client{
    addr: func() string {
        return fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
    },
}

fmt.Println(c.addr()) // localhost:8080

cfg.Host = "example.com"
cfg.Port = 9090

fmt.Println(c.addr()) // example.com:9090
```

Run it on [Go playground].

The closure didn't bake in `"localhost:8080"`. It captured `cfg`, which is a pointer, and
went back to the same struct every time it ran. Mutating the struct between calls changed
what the closure printed.

To freeze a multi-field snapshot, copy what the closure needs into a local before creating
it:

```go {hl_lines=6}
type addr struct {
    host string
    port int
}

snap := addr{host: cfg.Host, port: cfg.Port}

c.addr = func() string {
    return fmt.Sprintf("%s:%d", snap.host, snap.port)
}
```

## Capture-by-reference is what makes counters work

The [spec] puts it like this:

> Function literals are closures: they may refer to variables defined in a surrounding
> function. Those variables are then shared between the surrounding function and the
> function literal, and they survive as long as they are accessible.

Take a counter:

```go
func counter() func() int {
    n := 0
    return func() int {
        n++
        return n
    }
}
```

If `n` were copied into the closure at creation time, calling the returned function twice
would print `1, 1`. Instead it prints `1, 2`, because every call reaches the same `n` on the
heap. The Go FAQ entry on [closures running as goroutines] spells out the same mechanic for
loop variables, and Russ Cox's [Off to the Races] notes that locals whose addresses escape
end up on the heap automatically. The compiler effectively lifts the captured variable to
the heap and gives the closure a pointer to it, so the same address is shared by anyone
holding the closure.

Every time you write `func() { ... cfg.Host ... }`, the closure keeps `cfg` alive and
reaches through it on every call.

## A few more examples

### A connection's crash message races with log rotation

The original Hegel example has a server with a log file and a `Conn` that knows how to
format a crash message. If we expand, it might look like this:

```go {hl_lines=12}
type Server struct {
    logFile *os.File
}

type Conn struct {
    crashMsg func() string
}

func (s *Server) newConn() *Conn {
    return &Conn{
        crashMsg: func() string {
            return "log: " + s.logFile.Name()
        },
    }
}
```

Now imagine the server rotates its log file, or sets `s.logFile = nil` during shutdown. Both
are reasonable things to do. The closure keeps reading `s.logFile` whenever something asks
the connection for its crash message. If that read happens during cleanup, you have a race
on `s.logFile`. If it happens after rotation, the message points at the new file, not the
file the connection was actually using.

The fix is to copy what the closure needs at construction time:

```go {hl_lines=2}
func (s *Server) newConn() *Conn {
    name := s.logFile.Name() // copy now, while we know it's valid

    return &Conn{
        crashMsg: func() string {
            return "log: " + name
        },
    }
}
```

`Conn` no longer holds a pointer to `Server`. Rotation, shutdown, and mutation of
`s.logFile` no longer concern it.

### Concurrent requests share one captured bool

Philippe Gaultier [reproduced this exact bug] in a rate-limiting middleware:

```go {hl_lines=4}
func NewMiddleware(next http.Handler, rateLimitEnabled bool) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.HasPrefix(r.URL.Path, "/admin") {
            rateLimitEnabled = false
        }
        if rateLimitEnabled {
            // ... rate limit ...
        }
        next.ServeHTTP(w, r)
    })
}
```

The `rateLimitEnabled` bool is a parameter to `NewMiddleware`, but the closure captures it
by reference. Every concurrent HTTP request runs the same closure and every one of them
mutates the same captured `bool`. One admin request flips the switch off for everyone else.
The race detector didn't even catch this on the original middleware in Gaultier's tests; he
had to write a separate reproducer to make it fire.

The fix is a one-line shadow at the top of the closure body, so each request gets its own
copy:

```go {hl_lines=3}
func NewMiddleware(next http.Handler, rateLimitEnabled bool) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rateLimitEnabled := rateLimitEnabled // per-request copy
        if strings.HasPrefix(r.URL.Path, "/admin") {
            rateLimitEnabled = false
        }
        if rateLimitEnabled {
            // ... rate limit ...
        }
        next.ServeHTTP(w, r)
    })
}
```

`:=` declares a new local. With `=`, the closure would still write to the captured
parameter.

I find this one especially nasty because there's no goroutine in the source. The goroutines
are added by `net/http` when it dispatches handlers.

From [Uber's data race study]:

> Developers are quite often unaware that a variable used inside a closure is a free
> variable and captured by reference, especially when the closure is large. More often than
> not, Go developers use closures as goroutines. As a result of capture-by-reference and
> goroutine concurrency, Go programs end up potentially having unordered accesses to free
> variables unless explicit synchronization is performed.

### Loop variables shared one slot before Go 1.22

The famous version of this is loop-variable capture:

```go
for _, v := range xs {
    go func() {
        use(v)
    }()
}
```

Before Go 1.22 this printed the last value `len(xs)` times because every iteration shared
one `v`. It got [patched in go1.22], which gives each iteration its own copy. Eli Bendersky
has a [great explainer] of what was happening under the hood pre-1.22 if you're curious.

Go 1.22 only changed loop-variable lifetime. Pointer captures and method values on
long-lived receivers still behave the way they always did, because the language can't tell
that you didn't mean exactly that.

### Method values capture their receiver

`s.serverCrashMessage` is a method value. Under the hood it's a closure that captures `s`
the same way any other closure captures a free variable. From the hegel-go skill again:

> `conn.crashMessageFn = s.serverCrashMessage` captures `s` and reads `s.logFile` — any
> field the method touches is shared state.

If `serverCrashMessage` reads `s.logFile`, the resulting function value carries a live
pointer to `s` and re-reads `s.logFile` every time it's called. [Bendersky's article] walks
through the same gotcha with a `Show()` method on a pointer receiver: `go m.Show()` shares
the receiver across goroutines, and nothing at the call site warns you.

## When you can't just snapshot

If the closure genuinely needs to see live state, leave it as a pointer and guard the reads
with the same mutex (or atomic, or channel) that the writers use. That's a different choice
with a different cost (more synchronization, fewer surprises) and you should make it on
purpose.

A few things that help spot the bug when snapshots aren't an option:

- When a callback or method value lands on a long-lived struct, ask: which fields does this
  read? Write the answer next to the field declaration.
- If a closure only ever needs primitives, prefer passing them in as values rather than
  reaching through a pointer.
- `go vet`'s `loopclosure` checker still catches the loop-variable case in pre-1.22 modules.
  It cannot catch the broader struct-capture case.
- The race detector (`go test -race`) catches the concurrent ones. It can't catch
  single-threaded "wrong value at the wrong time" bugs like the log-rotation example.

## Drop the rule into your agent's prompt

Pasting the two sentences into your `AGENTS.md`, `CLAUDE.md`, or whatever your agent reads
is often enough:

```md
### Function closures capturing mutable references

`conn.crashMessageFn = s.serverCrashMessage` captures `s` and reads
`s.logFile` — any field the method touches is shared state. Prefer capturing
immutable values (strings, ints) rather than pointers to mutable structs.
```

<!-- references -->
<!-- prettier-ignore-start -->

[Go playground]:
    https://go.dev/play/p/kdjbeNhdZ1i

[capture-by-reference]:
    https://go.dev/ref/spec#Function_literals:~:text=Function%20literals%20are%20closures%3A%20they%20may%20refer%20to%20variables%20defined%20in%20a%20surrounding%20function.%20Those%20variables%20are%20then%20shared%20between%20the%20surrounding%20function%20and%20the%20function%20literal%2C%20and%20they%20survive%20as%20long%20as%20they%20are%20accessible.

[spec]:
    https://go.dev/ref/spec#Function_literals:~:text=Function%20literals%20are%20closures%3A%20they%20may%20refer%20to%20variables%20defined%20in%20a%20surrounding%20function.%20Those%20variables%20are%20then%20shared%20between%20the%20surrounding%20function%20and%20the%20function%20literal%2C%20and%20they%20survive%20as%20long%20as%20they%20are%20accessible.

[hegel-go]:
    https://github.com/hegeldev/hegel-go

[go-concurrency]:
    https://github.com/hegeldev/hegel-go/blob/main/.claude/skills/go-concurrency/SKILL.md#function-closures-capturing-mutable-references

[closures running as goroutines]:
    https://go.dev/doc/faq#closures_and_goroutines

[Off to the Races]:
    https://research.swtch.com/gorace

[Uber's data race study]:
    https://www.uber.com/en-US/blog/data-race-patterns-in-go/

[reproduced this exact bug]:
    https://gaultier.github.io/blog/a_subtle_data_race_in_go.html

[great explainer]:
    https://eli.thegreenplace.net/2019/go-internals-capturing-loop-variables-in-closures/

[Bendersky's article]:
    https://eli.thegreenplace.net/2019/go-internals-capturing-loop-variables-in-closures/

[patched in go1.22]:
    https://go.dev/blog/loopvar-preview

<!-- prettier-ignore-end -->

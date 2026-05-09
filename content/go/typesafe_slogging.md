---
title: Type-safe slogging
date: 2026-05-09
slug: typesafe-slogging
tags:
    - Go
    - Logging
    - API
description: >-
    The default slog API is loose enough that a careless line ships broken JSON to
    production. Pin it down with Attr constructors, LogAttrs, a context-borne logger,
    and sloglint.
---

Typically on a brownfield project I don't care much about logging libraries and just go
with whatever's already set up. Before slog, I was an avid zap/zerolog user for years. But
since Go 1.21, I've dropped third-party logging libraries in favor of slog. I even
[recently ranted a bit on r/golang] about people pulling in third-party libs when slog is
right there. The common complaints against slog are:

- The typical usage pattern isn't type-safe.
- It allocates a bit more than zap or zerolog.

Working on a fairly large-scale deployment (1000+ k8s pods serving 250rps), I haven't run
into a case where slog's extra allocations were the reason behind memory pressure or tail
latency. So going with the stdlib now is a no-brainer to me. The API isn't bad either,
once a few patterns settle. The rest of the post is the small workflow I default to.
It's not a slog API tour. The [stdlib docs] do that better than I could, and my
[earlier post on slog] has the basics.

## The nicest API isn't type-safe

Most slog code I see reaches for the level helpers like `slog.Info` and `slog.Warn`,
which take `(msg string, args ...any)`. The args are meant to alternate between keys
and values, but the compiler sees `any` and won't enforce that. Forget a value and the
trailing key becomes a `!BADKEY`:

```go
slog.Info("placed", "order_id", id, "amount")
// {"msg":"placed","order_id":"ord_001","!BADKEY":"amount"}
```

Swap a key and a value and the dashboard indexes records under the value:

```go
slog.Info("placed", id, "order_id")
// {"msg":"placed","ord_001":"order_id"}
```

Pass the wrong type and the field ships as the wrong JSON type:

```go
slog.Info("placed", "amount", "1290")
// {"msg":"placed","amount":"1290"}    // string, not int
```

Two of the three ship valid JSON that's wrong. Queries filtering on `amount` skip
records where the field arrived as a string. Dashboards keyed on `order_id` end up
indexed by user values. Nothing alerts you.

## Pass the logger as a dependency

The package-level `slog.Info`, `slog.Warn`, and `slog.Default()` route through a
mutable global. Anyone can swap it via `slog.SetDefault`, which makes parallel tests
racy and leaves library code that calls `slog.Info` dependent on `main` having set the
default. Forget the setup in some new entry point and you fall back to the plain-text
default with no compile-time hint.

I pass `*slog.Logger` in as a constructor argument:

```go
type Service struct {
    logger *slog.Logger
}

func NewService(logger *slog.Logger) *Service {
    return &Service{logger: logger}
}
```

`main` builds one logger and threads it through. Tests build their own writing into a
buffer.

## LogAttrs over Info

On that logger, use `LogAttrs` instead of `Info`:

```go
logger.LogAttrs(ctx, slog.LevelInfo, "placed",
    slog.String("order_id", id),
    slog.Int64("amount", amount),
)
```

The constructors pin the value type at the call site, so none of the three failures
from the kv form compile.

`LogAttrs` is also cheaper. `Info` boxes every arg into an `any` (heap-allocating ints
and small values), walks the slice at runtime to pair keys with values, and
type-switches each value to build the `Attr`. `LogAttrs` skips all that. The attrs
arrive pre-typed, so the `int64` sits in a typed field inside `Value` and the slice
goes straight to the handler. Both paths allocate the record, but `Info` adds N
interface boxes plus a runtime parse loop on top.

The trade-off is typing. `LogAttrs` is the most verbose method, and slog gives you
plenty to pick from. `Info`, `Warn`, `Error`, and `Debug` take kv pairs and no context.
They fall back to `context.Background()` internally. `InfoContext` and friends add an
explicit context. `Log` takes a context and an explicit level. `LogAttrs` takes the
same context and level but swaps the kv pairs for typed attrs. Every call site asks
you to pick the one that fits.

Defaulting to `LogAttrs` everywhere trades typing for fewer decisions. No "`Info` or
`InfoContext`?" question, no "kv or typed?" question. Every call is the same shape:
`logger.LogAttrs(ctx, level, msg, attrs...)`.

When there's no surrounding context, I pass `context.TODO()` instead of
`context.Background()`. `Background()` is reserved for `main` and the composition root,
so `TODO()` further down signals "no context plumbed through yet". If I'm reaching for
`TODO()` a lot, that's a prompt to ask whether the layer needs a context plumbed in or
shouldn't be logging at all.

## Push attrs into helpers

`LogAttrs` fixes types at the call site, but `slog.String("order_id", id)` written
inline still puts the same key string everywhere an order ID gets logged. Decide
tomorrow that you want it spelled `orderID` and you're grepping. Decide that emails
shouldn't ship in logs and you're grepping again, hoping you didn't miss a typo.

I keep every attribute helper in one file inside an `internal/log` package:

```go {hl_lines=["6-9"]}
// internal/log/attrs.go
package log

import "log/slog"

func OrderID(s string) slog.Attr    { return slog.String("order_id", s) }
func UserID(s string) slog.Attr     { return slog.String("user_id", s) }
func AmountCents(c int64) slog.Attr { return slog.Int64("amount_cents", c) }
func Err(e error) slog.Attr         { return slog.String("err", e.Error()) }
```

Imported as `applog` to dodge the stdlib `log` collision. Every call site reads the
same way:

```go {hl_lines=["2-4"]}
logger.LogAttrs(ctx, slog.LevelInfo, "placed order",
    applog.OrderID(o.ID),
    applog.UserID(o.UserID),
    applog.AmountCents(o.AmountCents),
)
```

For fields that always log together, push them into a single helper that returns
`[]slog.Attr`:

```go
type Order struct {
    ID          string
    UserID      string
    AmountCents int64
}

// internal/log/attrs.go
func Order(o Order) []slog.Attr {
    return []slog.Attr{
        OrderID(o.ID),
        UserID(o.UserID),
        AmountCents(o.AmountCents),
    }
}
```

And spread with `...` at the call site:

```go
logger.LogAttrs(ctx, slog.LevelInfo, "placed order", applog.Order(o)...)
```

Add a field like `currency` to `Order()` and every order log picks it up.

Renames are one-line edits in `attrs.go`. Types live in one place, so
`AmountCents(int64)` won't take an `int` from any caller.

Spelling mistakes can't drift across files. Inline `slog.String("request_id", id)` in
one place and `slog.String("reqwest_id", id)` somewhere else, and you'll be wondering
why half the logs don't show up under `request_id`. With one helper per attribute, the
typo lives in one function and either every call site has it or none does.

Need to redact `Email`? Change `Email()` to return `slog.String("email", "[redacted]")`
and every call site updates. LLMs pick up the pattern fast too. Tell an agent to log a
new field and it adds a helper in `attrs.go` and calls it from the right place.

## But what about nested structures?

Same trick. The helper returns a group instead of a single value:

```go
type User struct {
    ID    string
    Name  string
    Email string
    Tier  string
}

// internal/log/attrs.go
func User(u User) slog.Attr {
    return slog.Group("user",
        slog.String("id", u.ID),
        slog.String("tier", u.Tier),
    )
}
```

`Email` doesn't appear inside the group, so no caller can leak it via `applog.User(u)`.
Used the same way as the rest:

```go
logger.LogAttrs(ctx, slog.LevelInfo, "user signed in",
    applog.User(u),
)
```

```json
{"msg":"user signed in","user":{"id":"u_42","tier":"gold"}}
```

## In the wild

The shape shows up across plenty of production Go codebases. [syncthing] keeps its slog
helpers in `internal/slogutil/slogvalues.go`. [BloodHound] has a package literally named
`attr` for them. [Teleport] does the same in
`lib/join/internal/diagnostic/diagnostic.go`, with zero-value suppression baked in.
[FerretDB] exports `Error(err) slog.Attr`.

A minimal end-to-end example is on this [Go Playground share]. It has the helpers in
`internal/log`, a `Service` that uses them, and a `main` that calls into it.

## Enforce it with sloglint

[sloglint] enforces the workflow on every PR. The rules I default to:

```yaml
linters-settings:
  sloglint:
    attr-only: true
    no-global: "all"
    context: "all"
    static-msg: true
    key-naming-case: snake
```

`attr-only` rejects the kv form. `no-global: "all"` blocks `slog.Info` and
`slog.Default()`. `context: "all"` rejects any call without a context. `static-msg`
keeps the message a string literal. `key-naming-case: snake` flags any key that
isn't snake_case.

> [!Gist]
>
> - Take the logger as a constructor argument. Never reach for `slog.Default()` or any
>   package-level slog function.
> - Always use `logger.LogAttrs(ctx, level, msg, attrs...)`. Not `logger.Info`,
>   `logger.Warn`, or any of the kv-flavored helpers.
> - Every attribute comes from a helper in `internal/log/attrs.go`. Write
>   `applog.OrderID(o.ID)`, never `slog.String("order_id", o.ID)` inline.
> - [sloglint] enforces all three on every commit so the workflow doesn't erode.

<!-- references -->
<!-- prettier-ignore-start -->

[recently ranted a bit on r/golang]:
    https://old.reddit.com/r/golang/comments/1t2jilv/just_use_slog_itll_be_fine/

[syncthing]:
    https://github.com/syncthing/syncthing/blob/main/internal/slogutil/slogvalues.go

[BloodHound]:
    https://github.com/SpecterOps/BloodHound/blob/main/packages/go/bhlog/attr/attr.go

[Teleport]:
    https://github.com/gravitational/teleport/blob/master/lib/join/internal/diagnostic/diagnostic.go

[FerretDB]:
    https://github.com/FerretDB/FerretDB/blob/main/internal/util/logging/logging.go

[sloglint]:
    https://github.com/go-simpler/sloglint

[stdlib docs]:
    https://pkg.go.dev/log/slog

[earlier post on slog]:
    /go/structured-logging-with-slog

[Go Playground share]:
    https://go.dev/play/p/25NKrP9xxoJ

<!-- prettier-ignore-end -->

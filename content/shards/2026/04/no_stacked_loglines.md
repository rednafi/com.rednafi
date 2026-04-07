---
title: Stacked log lines considered harmful
date: 2026-04-07
slug: no-stacked-loglines
tags:
    - Go
    - Distributed Systems
    - Observability
description: >-
  Why logging at every layer of a service produces noise, and how to log only
  at the handler level while propagating context from below.
---

Consider a typical layered Go service. There's a repository layer that talks to the
database, a service layer that contains the business logic, and a handler that deals with
protocols like HTTP or gRPC.

One common thing I often see is emitting log lines from all three layers. When an
error occurs, each layer logs it as it bubbles up, producing a stack of duplicate
lines for the same failure that makes incidents harder to debug.

## One error, three log lines

The repository hits a database timeout and logs it:

```go
func (r *UserRepo) GetByID(ctx context.Context, id string) (User, error) {
    user, err := r.db.QueryRow(ctx, "SELECT ...")
    if err != nil {
        log.Error("failed to query user", "id", id, "error", err) // log #1
        return User{}, fmt.Errorf("repository: %w", err)
    }
    return user, nil
}
```

The service catches the wrapped error and logs it again:

```go
func (s *UserService) GetUser(ctx context.Context, id string) (User, error) {
    user, err := s.repo.GetByID(ctx, id)
    if err != nil {
        log.Error("failed to get user", "id", id, "error", err) // log #2
        return User{}, fmt.Errorf("service: %w", err)
    }
    return user, nil
}
```

The handler logs it a third time before returning a 500:

```go
func (h *UserHandler) HandleGetUser(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    user, err := h.svc.GetUser(r.Context(), id)
    if err != nil {
        log.Error("failed to handle request", "error", err) // log #3
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(user)
}
```

One database timeout, three error log lines. You search during an incident, see 3,000
errors, and think three thousand things broke. In reality it was one thousand requests
that each logged the same failure three times.

> [!IMPORTANT]
> There's a related but separate problem in `GetByID` and `GetUser`: they log the
> error _and_ return it. Dave Cheney in [Let's talk about logging] warns us against
> it: "if you choose to handle the error by logging it, by definition it's not an
> error any more - you handled it."
>
> The [Uber Go Style Guide] says the same: "the caller should not, for example, log
> the error and then return it, because its callers may handle the error as well."
> Either log it or return it, not both.

## Move the log line to the top

Lower layers should return errors with context, not log them. Wrap each error on the
way up:

```go
// Repository
return User{}, fmt.Errorf("userRepo.GetByID: query user %s: %w", id, err)

// Service
return User{}, fmt.Errorf("userService.GetUser: %w", err)
```

The handler gets `userService.GetUser: userRepo.GetByID: query user abc123:
connection refused`. The full call chain is in the error string and no layer had to
log independently. The handler logs it once:

```go
func (h *UserHandler) HandleGetUser(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    user, err := h.svc.GetUser(r.Context(), id)
    if err != nil {
        log.Error("request failed",
            "method", r.Method,
            "path", r.URL.Path,
            "error", err,
        )
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(user)
}
```

One error, one log line, with request context attached.

Caddy's deferred [logRequest] and HashiCorp Nomad's [wrap] follow the same pattern -
one structured log line at the boundary, nothing below it.

## Collecting log fields on the way up

Error wrapping gets failure details to the handler. But what about things like how many
database queries a request made, or how long a downstream call took? The handler
doesn't know these things unless the lower layers pass them up.

The approach is to stash a mutable field collector in `context.Context` at the start
of a request. Lower layers append to it. The handler reads it back and includes
everything in one log line.

Start with a thread-safe container for log fields:

```go
type logFields struct {
    mu     sync.Mutex
    fields []slog.Attr
}

type ctxKey struct{}
```

Then a helper that lower layers call to attach fields without logging:

```go
func AddLogField(ctx context.Context, key string, value any) {
    if lf, ok := ctx.Value(ctxKey{}).(*logFields); ok {
        lf.mu.Lock()
        lf.fields = append(lf.fields, slog.Any(key, value))
        lf.mu.Unlock()
    }
}
```

The middleware creates the collector, passes it down through context, and emits one
log line after the handler chain completes:

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        lf := &logFields{}
        ctx := context.WithValue(r.Context(), ctxKey{}, lf) // (1)
        rec := &statusRecorder{ResponseWriter: w}
        start := time.Now()

        next.ServeHTTP(rec, r.WithContext(ctx)) // (2)

        lf.mu.Lock()
        attrs := append([]slog.Attr{
            slog.String("method", r.Method),
            slog.String("path", r.URL.Path),
            slog.Int("status", rec.status),
            slog.Duration("duration", time.Since(start)),
        }, lf.fields...) // (3)
        lf.mu.Unlock()

        level := slog.LevelInfo
        if rec.status >= 500 {
            level = slog.LevelError // (4)
        }
        slog.LogAttrs(ctx, level, "request", attrs...)
    })
}
```

Here:

- (1) stashes an empty field collector in context
- (2) runs the handler chain; lower layers may call `AddLogField` during this
- (3) merges request fields with whatever lower layers attached
- (4) only two log levels: `Info` for normal requests, `Error` for 5xx

Now a repository can attach timing data without emitting a log line:

```go
func (r *UserRepo) GetByID(ctx context.Context, id string) (User, error) {
    start := time.Now()
    user, err := r.db.QueryRow(ctx, "SELECT ...")
    AddLogField(ctx, "db_duration", time.Since(start))
    if err != nil {
        return User{}, fmt.Errorf("userRepo.GetByID: %w", err)
    }
    return user, nil
}
```

The middleware's log line includes `db_duration` alongside method, path, status, and
duration. The repository contributed diagnostic data without logging anything itself.

The Kubernetes API server does exactly this with [AddKeyValue] and [respLogger].
Caddy does the same with `ExtraLogFieldsCtxKey`.

Stripe took this pattern to its logical conclusion with the [canonical log line] -
one rich structured line per request containing everything that happened during that
request. Brandur Leach, who worked on the pattern at Stripe, called it "[the single,
simplest, best method of getting easy insight into production that there is]." A
canonical log line from Stripe:

```txt
canonical-log-line alloc_count=9123 auth_type=api_key
  database_queries=34 duration=0.009 http_method=POST
  http_path=/v1/charges http_status=200 key_id=mk_123
  permissions_used=account_write rate_allowed=true
  rate_quota=100 rate_remaining=99 request_id=req_123
  team=acquiring user_id=usr_123
```

The `database_queries=34` and `alloc_count=9123` fields are exactly the kind of
non-error context that lower layers attached via the middleware pattern described
above. [go-chi/httplog] implements this in Go on top of `log/slog`.

<!-- references -->
<!-- prettier-ignore-start -->

[respLogger]:
    https://github.com/kubernetes/apiserver/blob/master/pkg/server/httplog/httplog.go#L259-L296

[AddKeyValue]:
    https://github.com/kubernetes/apiserver/blob/master/pkg/server/httplog/httplog.go#L234-L238

[logRequest]:
    https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/server.go#L380

[wrap]:
    https://github.com/hashicorp/nomad/blob/main/command/agent/http.go#L738-L815

[Let's talk about logging]:
    https://dave.cheney.net/2015/11/05/lets-talk-about-logging

[Uber Go Style Guide]:
    https://github.com/uber-go/guide/blob/master/style.md#handle-errors-once

[canonical log line]:
    https://stripe.com/blog/canonical-log-lines

[the single, simplest, best method of getting easy insight into production that there is]:
    https://brandur.org/nanoglyphs/025-logs

[go-chi/httplog]:
    https://github.com/go-chi/httplog

<!-- prettier-ignore-end -->

---
title: "Go errors: to wrap or not to wrap?"
date: 2026-03-07
slug: to-wrap-or-not-to-wrap
tags:
    - Go
    - Error Handling
description: >-
    Exploring the tradeoffs between wrapping errors at every return site versus
    wrapping only at boundaries, with no definitive answer - just honest tradeoffs
    for the kind of software I write.
---

A lot of the time, the software I write boils down to three phases: parse some input, run it
through a state machine, and persist the result. In this kind of code, you spend a lot of
time knitting your error path, hoping that it'd be easier to find the root cause during an
incident. This raises the following questions:

- When to `fmt.Errorf("doing X: %w", err)`
- When to use `%v` instead of `%w`
- When to just `return err`

There's no consensus, and the answer changes depending on the kind of application you're
writing. The [Go 1.13 blog] already covers the mechanics and offers some guidance, but I
wanted to collect more evidence of what people are actually doing in the open and share
what's worked for me.

## The problem with bare errors

Here's a function that places an order by calling into a few different packages:

```go
func placeOrder(ctx context.Context, req OrderReq) error {
    user, err := users.Get(ctx, req.UserID)
    if err != nil {
        return err
    }
    err = inventory.Reserve(ctx, req.ItemID, req.Qty)
    if err != nil {
        return err
    }
    err = payments.Charge(ctx, user.PaymentID, req.Total)
    if err != nil {
        return err
    }
    return saveOrder(ctx, user.ID, req.ItemID)
}
```

All four calls can fail with `connection refused`. When one of them does, your log says:

```
connection refused
```

Which call? No idea. You grep the codebase, add temporary logging, narrow it down. In a
service with dozens of dependencies, debugging this trail of errors can turn into a huge
time sink.

One obvious fix is to wrap the error at every return site:

```go
user, err := users.Get(ctx, req.UserID)
if err != nil {
    return fmt.Errorf("getting user %s: %w", req.UserID, err)
}
err = inventory.Reserve(ctx, req.ItemID, req.Qty)
if err != nil {
    return fmt.Errorf("reserving stock for %s: %w", req.ItemID, err)
}
```

Now the log says:

```
reserving stock for item-123: connection refused
```

That tells you exactly which call failed and which item it was for.

## The case for wrapping at every return site

Dave Cheney advocated for this in his 2016 talk [Don't just check errors]. His `pkg/errors`
library introduced `errors.Wrap`, which adds a message and a stack trace at the point where
the error occurs. The idea is that each function knows what operation it was attempting, and
that context is lost if you don't capture it immediately.

CockroachDB takes this further. They use [cockroachdb/errors], a drop-in replacement for the
stdlib `errors` package that captures a stack trace at every wrap site:

```go
// cockroachdb style: stack trace at every wrap
if err := r.validateCmd(ctx, cmd); err != nil {
    return errors.Wrap(err, "validating command")
}
if err := r.stage(ctx, cmd); err != nil {
    return errors.Wrap(err, "staging command")
}
```

The Terraform AWS provider does the same thing with `fmt.Errorf("...: %w", err)` at every
layer. Their [contributor guidelines] mandate a consistent format for all resource
operations:

```go
// terraform-provider-aws style
output, err := conn.CreateVpc(ctx, input)
if err != nil {
    return fmt.Errorf("creating EC2 VPC: %w", err)
}

d.SetId(aws.ToString(output.Vpc.VpcId))

if _, err := WaitVPCAvailable(ctx, conn, d.Id()); err != nil {
    return fmt.Errorf(
        "waiting for EC2 VPC (%s) available: %w",
        d.Id(), err,
    )
}
```

The [wrapcheck] linter codifies this as a rule. It doesn't flag every bare `return err`,
only errors that originated from a different package:

```go
func placeOrder(ctx context.Context, req OrderReq) error {
    // users.Get is in another package: wrapcheck flags
    user, err := users.Get(ctx, req.UserID)
    if err != nil {
        return err // not wrapped: linter warning
    }

    // validate is in the same package: wrapcheck allows
    err = validate(req)
    if err != nil {
        return err // fine, same package
    }
    // ...
}
```

The reasoning is that when an error crosses a package boundary, the receiving code is the
last place that knows what it was trying to do. Within a package, the caller already has
that context.

For many cases, wrapping everything is the right default:

> The risk of overwrapping, especially in my private code, is much lower than the risk of
> underwrapping when the service crashes and you get `io.EOF`.
>
> -- [Peter Bourgon on Go Time #91]

But wrapping has costs that only show up as the codebase grows.

## The cost of overwrapping

### Messages pile up

When every layer wraps, your error messages become nested chains:

```
placing order: reserving stock for item-123:
    checking warehouse: querying database:
    connection refused
```

Four layers of context for one `connection refused`. The middle layers (`checking warehouse`
and `querying database`) don't add a warehouse ID or a query. They just restate the call
chain.

It also makes the error string fragile. It changes whenever someone renames an intermediate
function or refactors the call chain. If you had an alert matching on
`checking warehouse: querying database: connection refused`, it breaks the moment someone
renames `checkWarehouse` to `checkStock`. The same root cause (`connection refused`) wrapped
through different code paths produces different error strings, making it hard to aggregate
them in your logging dashboard.

[Jay Conrod]'s error handling guidelines address this:

> Each function is responsible for including its own values in the error message, except for
> arguments passed to the function that returned the wrapped error.

In other words, if `os.Open` already puts the file path in its error, your wrapper shouldn't
add the path again:

```go
// redundant: the path appears twice
return fmt.Errorf("opening %s: %w", path, err)
// open /etc/app.yaml: opening /etc/app.yaml: permission denied

// better: add what you were doing, not what Open already said
return fmt.Errorf("reading config: %w", err)
// reading config: open /etc/app.yaml: permission denied
```

The [Google Go Style Guide] says the same:

> When adding information to errors, avoid redundant information that the underlying error
> already provides.

You should still wrap, but only when you're adding information - a user ID, an item ID, the
name of the external service you were calling.

> [!IMPORTANT]
>
> If a function is just passing through a call to another function within the same package,
> the wrapper is noise.

### `%w` creates contracts you didn't mean to

`%w` in `fmt.Errorf` creates an error chain that callers can traverse with `errors.Is` and
`errors.As`. That means the wrapped error becomes part of your function's API surface.

The [Go 1.13 blog] uses `sql.ErrNoRows` to illustrate this. Say your `LookupUser` function
calls `database/sql` internally:

```go
func LookupUser(ctx context.Context, id string) (*User, error) {
    row := db.QueryRowContext(ctx, "SELECT ...", id)
    var u User
    if err := row.Scan(&u.Name, &u.Email); err != nil {
        return nil, fmt.Errorf(
            "looking up user %s: %w", id, err,
        )
    }
    return &u, nil
}
```

Because of `%w`, callers can now do `errors.Is(err, sql.ErrNoRows)` to check whether the
user wasn't found. That works until you switch from `database/sql` to an ORM, or put a cache
in front of the query. The callers matching on `sql.ErrNoRows` silently break.

The [Go 1.13 blog] is explicit about this:

> Wrapping an error makes that error part of your API. If you don't want to commit to
> supporting that error as part of your API in the future, you shouldn't wrap the error.

The [Error Values FAQ] makes the same point:

> Callers can depend on the type and value of the error you're wrapping, so changing that
> error can now break them. [...] At that point, you must always return `sql.ErrTxDone` if
> you don't want to break your clients, even if you switch to a different database package.

Same thing with typed errors. If your repository wraps a `pgconn.PgError` with `%w`, callers
can unwrap through to the Postgres error code:

```go
if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
    log.Println(pgErr.Code) // e.g. "23505" (unique violation)
}
```

When you migrate to MySQL or put a cache in front of the database, those callers silently
break.

The [Google Go Style Guide] notes that `%w` is appropriate when your package's API
guarantees that certain underlying errors can be unwrapped and checked by callers. If you
don't want to make that guarantee, use `%v`.

> [!IMPORTANT]
>
> `%w` makes the wrapped error part of your function's API. Callers can `errors.Is` and
> `errors.As` through it, which means they can start depending on the inner error type. If
> you later change that inner error (swap databases, add a cache layer), those callers
> break. Use `%w` only when you intend to expose the inner error.

## `%v` as the conservative default

`%v` adds the same context text (the human reading the log sees the identical message) but
severs the error chain. No caller can `errors.Is` or `errors.As` through it:

```go
// %w: callers can errors.Is(err, sql.ErrNoRows)
return fmt.Errorf("getting user %s: %w", id, err)

// %v: same message text, but the chain is severed
return fmt.Errorf("getting user %s: %v", id, err)
```

Both produce the same log output. But with `%v`, you're free to swap the database later
without breaking callers who were depending on the inner error type.

At system boundaries, the [Google Go Style Guide] recommends translating rather than
wrapping:

> At points where your system interacts with external systems like RPC, IPC, or storage,
> it's often better to translate domain-specific errors into a standardized error space
> (e.g., gRPC status codes) rather than simply wrapping the raw underlying error with `%w`.

Say your repository layer talks to Postgres via `pgx`. Wrapping with `%w` exposes `pgx`
errors to callers:

```go
func (r *UserRepo) Get(ctx context.Context, id string) (*User, error) {
    row := r.db.QueryRow(ctx, "SELECT ...", id)
    if err := row.Scan(&u.Name, &u.Email); err != nil {
        return nil, fmt.Errorf("getting user %s: %w", id, err)
    }
    return &u, nil
}
```

Now any caller can `errors.Is(err, pgx.ErrNoRows)`, tying them to your database driver.
Translating means mapping the storage error into your own domain before it crosses the
boundary:

```go
var ErrNotFound = errors.New("not found")

func (r *UserRepo) Get(ctx context.Context, id string) (*User, error) {
    row := r.db.QueryRow(ctx, "SELECT ...", id)
    if err := row.Scan(&u.Name, &u.Email); err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf("getting user %s: %v", id, err)
    }
    return &u, nil
}
```

Callers check `errors.Is(err, ErrNotFound)` - which is yours - instead of
`errors.Is(err, pgx.ErrNoRows)`. When you swap from Postgres to MySQL, callers don't break.
And at system boundaries, consider translating entirely instead of wrapping.

## How the stdlib handles errors

The standard library also uses [sentinel errors] and [custom error types] alongside `%w` and
`%v`.

Packages like `io` define sentinel errors - package-level variables that callers check with
`errors.Is`. The `io` package defines `EOF` and returns it from `Read` when there's no more
data:

```go
// definition
var EOF = errors.New("EOF")

// inside a Reader implementation
func (r *myReader) Read(p []byte) (int, error) {
    if r.pos >= len(r.data) {
        return 0, io.EOF
    }
    // ...
}
```

A caller uses the sentinel to distinguish "end of input" from a real failure:

```go
n, err := reader.Read(buf)
if errors.Is(err, io.EOF) {
    // done reading, not an error
    break
}
if err != nil {
    return err
}
```

Sentinels work when the caller only needs to know _which_ failure occurred. When callers
need structured metadata - not just identity - the stdlib uses custom error types. `os.Open`
defines a `*fs.PathError` struct and returns it with the operation name, file path, and
underlying syscall error as struct fields:

```go
// definition in the fs package
type PathError struct {
    Op   string // "open", "read", "write"
    Path string // the file path
    Err  error  // the underlying syscall error
}

func (e *PathError) Unwrap() error { return e.Err }

// inside os.Open
func Open(name string) (*File, error) {
    // ...
    return nil, &PathError{Op: "open", Path: name, Err: err}
}
```

Because `PathError` implements `Unwrap()`, `errors.Is(err, fs.ErrNotExist)` works through
the chain. But unlike `fmt.Errorf` wrapping, the context is in typed struct fields. A caller
can extract those fields to decide what to do:

```go
f, err := os.Open("/etc/app.yaml")
if err != nil {
    if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
        // pathErr.Op is "open", pathErr.Path is "/etc/app.yaml"
        // pathErr.Err is the syscall error (e.g. ENOENT)
        log.Printf(
            "%s failed on %s: %v",
            pathErr.Op, pathErr.Path, pathErr.Err,
        )
    }
    return err
}
```

`net.OpError` follows the same pattern with Op, Net, Source, Addr, and Err fields. The
package controls exactly what's exposed via `Unwrap()`, and callers get structured metadata
they can act on programmatically.

The stdlib also uses `fmt.Errorf` with both `%w` and `%v`, and the `database/sql` package
shows why the choice matters. `Rows.Scan` wraps scanner errors with `%w`:

```go
return fmt.Errorf(
    `sql: Scan error on column index %d, name %q: %w`,
    i, rs.rowsi.Columns()[i], err,
)
```

Before Go 1.16, `Rows.Scan` used `%v` here, which severed the chain. Custom `Scanner`
implementations returning sentinel errors couldn't be inspected with `errors.Is` by callers.
[Issue #38099] fixed this by switching to `%w`. But in the same package, internal type
conversion errors use `%v` because the underlying `strconv` parse error is an implementation
detail callers don't need to inspect:

```go
return fmt.Errorf(
    "converting driver.Value type %T (%q) to a %s: %v",
    src, s, dv.Kind(), err,
)
```

The `database/sql` migration from `%v` to `%w` was safe because it only exposed more to
callers. Going the other direction would break callers who started depending on `errors.Is`.

> [!IMPORTANT]
>
> Going from `%v` to `%w` is a backwards-compatible change (it exposes more to callers).
> Going from `%w` to `%v` is a breaking change (callers who relied on `errors.Is` or
> `errors.As` through the chain will stop working). When in doubt, start with `%v`.

Kubernetes went through a similar migration. They historically used `%v` for most wrapping,
which meant `errors.As` couldn't traverse the chain. [Issue #123234] tracked the codebase-
wide migration from `%v` to `%w`, acknowledging that `%v` may still be preferred in some
places "to abstract the implementation details" but that such cases should be rare.

For most application code, `fmt.Errorf` with `%w` or `%v` is enough. Custom error types like
`PathError` make more sense in libraries and shared packages where callers need structured
metadata. But wrapping isn't the only way to attach context to an error.

## Structured logging as an alternative to wrapping

Dave Cheney is the person who created `pkg/errors` and popularized error wrapping in Go. He
eventually walked away from his own advice. In 2021, when looking for new maintainers for
`pkg/errors`, he wrote:

> I no longer use this package, in fact I no longer wrap errors.
>
> -- [Dave Cheney on pkg/errors #245]

His reasoning was that structured logging can carry the debugging context that wrapping was
meant to provide. Compare the two approaches. With wrapping, you bake the context into the
error string:

```go
err = inventory.Reserve(ctx, req.ItemID, req.Qty)
if err != nil {
    return fmt.Errorf(
        "reserving stock for %s: %w", req.ItemID, err,
    )
}
```

The log line looks like:

```
reserving stock for item-123: connection refused
```

With structured logging, you keep the error value clean and attach the context as separate
key-value fields:

```go
err = inventory.Reserve(ctx, req.ItemID, req.Qty)
if err != nil {
    slog.Error("reserve stock failed",
        "item_id", req.ItemID,
        "err", err,
    )
    return err
}
```

The log line looks like:

```
level=ERROR msg="reserve stock failed"
    item_id=item-123 err="connection refused"
```

The same information is there, but in structured fields that your logging dashboard can
index, filter, and aggregate on. The error value itself stays as `connection refused`
without a chain of prefixes.

The tradeoff is that structured logging requires a logging pipeline that can query on
fields. If all you have is `grep` on a log file, the wrapping version is easier to work
with.

> [!NOTE]
>
> Structured logging and wrapping aren't mutually exclusive. You can wrap at package
> boundaries for the error string and log with `slog` at the handler for request-scoped
> context (user IDs, request IDs, trace IDs). The handler example in the Services section
> below does both.

## How wrapping changes by application type

So how do you actually decide? It depends on what you're building. Marcel van Lohuizen from
the Go team described his own approach:

> I do and don't... If I wanna have context, I wrap it. If I create a new error, I wrap it.
> But sometimes you're not really adding too much information, and then I don't. So it
> depends on the situation.
>
> -- [Marcel van Lohuizen on Go Time #91]

### Libraries

Be conservative. The Google style guide applies most directly here because you're shipping
an API contract. Use `%v` by default so you don't accidentally expose implementation
details. Use `%w` only when you intentionally want callers to inspect the inner error, and
document that you're doing so.

A library that wraps with `%w` ties its callers to its dependencies. If `v2` switches from
`pgx` to `database/sql`, every caller doing `errors.Is(err, pgconn.something)` breaks. Use
`%v` by default, and define your own sentinels when callers need to branch on the error:

```go
var ErrNotFound = errors.New("item not found")

func (c *Client) Fetch(ctx context.Context, id string) (*Item, error) {
    resp, err := c.http.Get(ctx, c.url+"/items/"+id)
    if err != nil {
        if isNotFound(err) {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf("fetching item %s: %v", id, err)
    }
    // ...
}
```

Callers check `errors.Is(err, ErrNotFound)` - which is yours - without being coupled to your
HTTP client. Same pattern as the `UserRepo` translation example earlier.

### CLI tools

Wrap freely with `%w`. The call stack is shallow, the error message is the user-facing
output, and nobody is calling `errors.Is` on your CLI's errors. Maximum context helps the
human reading the terminal:

```go
func run() error {
    cfg, err := loadConfig(cfgPath)
    if err != nil {
        return fmt.Errorf("loading config %s: %w", cfgPath, err)
    }
    conn, err := connect(cfg.DatabaseURL)
    if err != nil {
        return fmt.Errorf("connecting to database: %w", err)
    }
    return migrate(conn)
}
```

The user sees:

```
loading config /etc/app.yaml:
    open /etc/app.yaml: permission denied
```

### Services

In my experience, services are where it's the hardest to give a formulaic answer to this.
You have structured logging and distributed tracing, but you also have deep call stacks and
many dependencies.

The approach I've landed on: wrap at package boundaries with context about what you were
trying to do. Use `%w` within your own codebase where callers should be able to inspect the
inner error. Use `%v` when the error crosses a system boundary (RPCs, database calls,
third-party APIs). Skip wrapping for same-package calls.

Here's the `placeOrder` function from the beginning, rewritten:

```go
func placeOrder(ctx context.Context, req OrderReq) error {
    user, err := users.Get(ctx, req.UserID) // (1)
    if err != nil {
        return fmt.Errorf("getting user %s: %w", req.UserID, err)
    }
    err = inventory.Reserve(ctx, req.ItemID, req.Qty) // (2)
    if err != nil {
        return fmt.Errorf("reserving stock for %s: %w", req.ItemID, err)
    }
    err = payments.Charge(ctx, user.PaymentID, req.Total) // (3)
    if err != nil {
        return fmt.Errorf("charging payment: %w", err)
    }
    return saveOrder(ctx, user.ID, req.ItemID) // (4)
}
```

- (1) `users.Get` is in another package - wrap with the user ID
- (2) `inventory.Reserve` is in another package - wrap with the item ID
- (3) `payments.Charge` is in another package - wrap with the operation name
- (4) internal helper in the same package - bare return is enough

At the handler, use `%v` to translate into the external domain without exposing internals:

```go
func handlePlaceOrder(
    ctx context.Context, req *pb.OrderReq,
) (*pb.OrderResp, error) {
    err := placeOrder(ctx, fromProto(req))
    if err != nil {
        slog.Error("placing order",
            "user_id", req.UserId,
            "item_id", req.ItemId,
            "err", err,
        )
        // %v: context for humans, no chain for callers
        return nil, status.Errorf(codes.Internal, "placing order: %v", err)
    }
    return &pb.OrderResp{}, nil
}
```

The handler logs the full error with request context for debugging, then returns a gRPC
status with `%v` so the caller gets a useful message without being able to `errors.Is`
through to your database driver.

## Where I've landed

There's no consensus on how much to wrap, and I don't think there needs to be. Here's what I
do:

- Within a package, bare `return err`. The caller already has context.
- At package boundaries, `fmt.Errorf("doing X: %w", err)` with identifying info (user IDs,
  item IDs, file paths). The [wrapcheck] linter can enforce this automatically. Only wrap
  when you're adding information the inner error doesn't already carry.
- At system boundaries (RPCs, database calls, third-party APIs), translate rather than wrap.
  Map implementation errors into your own [sentinel errors] or [custom error types] so
  callers depend on your package, not your dependencies. Use `%v` for the fallback path.
- In libraries, `%v` by default. Own sentinels (`ErrNotFound`, `ErrConflict`) for cases
  callers need to inspect. `%w` only when you intentionally want callers to unwrap, and
  document that you're doing so.
- In CLIs, `%w` everywhere. The error message is the user-facing output.
- In services, all of the above plus `slog` at the handler level for request-scoped context,
  so the error value doesn't need to carry all of that.

<!-- references -->
<!-- prettier-ignore-start -->

[Go 1.13 blog]:
    https://go.dev/blog/go1.13-errors#whether-to-wrap

[Google Go Style Guide]:
    https://google.github.io/styleguide/go/best-practices.html#error-extra-info

[Don't just check errors]:
    https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully#annotating-errors

[sentinel errors]:
    https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully#:~:text=Sentinel%20errors

[custom error types]:
    https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully#:~:text=Error%20types

[Peter Bourgon on Go Time #91]:
    https://changelog.com/gotime/91#t=16:22

[Jay Conrod]:
    https://jayconrod.com/posts/116/error-handling-guidelines-for-go#context-and-wrapping

[cockroachdb/errors]:
    https://github.com/cockroachdb/errors

[issue #123234]:
    https://github.com/kubernetes/kubernetes/issues/123234

[wrapcheck]:
    https://github.com/tomarrell/wrapcheck

[Dave Cheney on pkg/errors #245]:
    https://github.com/pkg/errors/issues/245#issue-988166855

[Marcel van Lohuizen on Go Time #91]:
    https://changelog.com/gotime/91#t=12:03

[Error Values FAQ]:
    https://go.dev/wiki/ErrorValueFAQ#i-am-already-using-fmterrorf-with-v-or-s-to-provide-context-for-an-error-when-should-i-switch-to-w

[issue #38099]:
    https://github.com/golang/go/issues/38099

[contributor guidelines]:
    https://hashicorp.github.io/terraform-provider-aws/error-handling/#wrap-errors

<!-- prettier-ignore-end -->

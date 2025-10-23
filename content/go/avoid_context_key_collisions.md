---
title: Avoiding collisions in Go context keys
date: 2025-10-22
slug: avoid-context-key-collisions
tags:
    - Go
---

Along with propagating deadlines and cancellation signals, Go's `context` package can also
carry request-scoped values across API boundaries and processes.

There's only two public API constructs associated with context values:

```go
func WithValue(parent Context, key, val any) Context
func (c Context) Value(key any) any
```

The naive workflow to store and retrieve values in a context looks like this:

```go
ctx := context.Background()

// Store some value against a key
ctx = context.WithValue(ctx, "userID", 42)

// Retrieve the value
v := ctx.Value("userID")

// Value returns any, so you need a type assertion
id, ok := v.(int)
if !ok {
    fmt.Println("unexpected type")
}
fmt.Println(id) // 42
```

`WithValue` returns a derived context that points to the parent context.

`Value` returns `any` (an alias to `interface{}`), so you must assert the expected type.
Without that, Go cannot verify the concrete type, and a direct cast without the `ok` check
would panic if it's wrong.

The issue with this setup is that it risks collision. If another package sets some value
against the same key, one value overwrites the other:

```go
package main

import (
    "context"
    "fmt"
)

func main() {
    ctx := context.WithValue(context.Background(), "key", "from-main")
    ctx = foo(ctx)
    fmt.Println(ctx.Value("key")) // from-foo
}

func foo(ctx context.Context) context.Context {
    // Accidentally reuse the same key in another package
    return context.WithValue(ctx, "key", "from-foo")
}
```

The first value becomes inaccessible because the context variable is reassigned and the same
string key is reused. Since `WithValue` returns a new context that shadows parent values
with the same key, `ctx.Value("key")` now returns `"from-foo"`. The original value still
exists in the parent context but is unreachable through the reassigned `ctx` variable.

The [doc has the following advice] to prevent that:

> _The provided key must be comparable and should not be of type string or any other
> built-in type to avoid collisions between packages using context. Users of WithValue
> should define their own types for keys. To avoid allocating when assigning to an
> interface{}, context keys often have concrete type struct{}. Alternatively, exported
> context key variables' static type should be a pointer or interface._

In short:

- Keys must be comparable (`string`, `int`, `struct`, `pointer`, etc.)
- Define unique key types per package to avoid collisions
- Use `struct{}` keys to avoid allocation when stored as `any`
- Exported key variables should have pointer or interface types

Here's how defining a unique key type prevents collisions:

```go
type userIDKey string

// Store value
ctx := context.WithValue(context.Background(), userIDKey("id"), 42)

// Retrieve value
id := ctx.Value(userIDKey("id"))
fmt.Println(id) // 42
```

Even if another package uses the string `"id"`, the key types differ, so they cannot
collide.

To avoid allocation when assigning to `any`, you can define an empty struct key. Unlike
strings or integers, which allocate when boxed into an interface, a zero-sized struct
occupies no memory and needs no allocation:

```go
type key struct{}

// Store value
ctx := context.WithValue(context.Background(), key{}, "value")

// Retrieve value
v := ctx.Value(key{})
fmt.Println(v) // value
```

Empty structs are ideal for local, unexported keys. They are unique by type and add no
overhead.

Alternatively, exported keys can use pointers, which also avoid allocation and guarantee
uniqueness. When a pointer is boxed into an `interface{}`, no data copy occurs because the
interface just holds the pointer reference. Pointers are also ideal for keys that need to be
shared across packages.

```go
type userIDKey struct {
    name string
}

var UserIDKey = &userIDKey{"user-id"}

// Store value
ctx := context.WithValue(context.Background(), UserIDKey, 42)

// Retrieve value
id := ctx.Value(UserIDKey)
fmt.Println(id) // 42
```

Here, `UserIDKey` points to a unique struct instance, so equality checks work by pointer
identity. The `name` field exists only for debugging. This avoids allocation and ensures
exported keys remain unique even when shared between packages.

When exposing context values across APIs, you can approach it in two ways depending on how
much control and safety you want to give your users.

### 1. Expose keys directly

You can export the key itself and let users interact with it freely:

```go
type APIKey string

var APIKeyContextKey = APIKey("api-key")

// Store value
ctx := context.WithValue(context.Background(), APIKeyContextKey, "secret")

// Retrieve value
v := ctx.Value(APIKeyContextKey).(string) // caller must do this assertion
fmt.Println(v) // secret
```

When you export the key directly the caller gains direct access, but they also must:

- do the type assertion themselves and handle the ok result to avoid panics
- ensure they don't accidentally overwrite values using the wrong key

The [net/http] package uses this approach for some of its exported context keys:

```go
type contextKey struct {
    name string
}

// Notice the exported keys
var (
    ServerContextKey    = &contextKey{"http-server"}
    LocalAddrContextKey = &contextKey{"local-addr"}
)
```

Each variable points to a distinct struct, making them unique by pointer identity. The
`name` field is there to make debugging easier, not for equality checks.

The [serve_test.go] file uses these keys like this:

```go
ctx := context.WithValue(context.Background(), http.ServerContextKey, srv)
srv2, ok := ctx.Value(http.ServerContextKey).(*http.Server) // type assertion
if ok {
    fmt.Println(srv == srv2) // true
}
```

The server value is stored in the context and later retrieved using the same pointer key.
The user must perform a type assertion and handle it safely.

### 2. Expose accessor functions

The other approach is to hide the key and provide accessor functions to set and retrieve
values. This removes the need for users to remember the right key type or perform type
assertions manually.

```go
// define a private key type to avoid collisions
type contextKey struct {
    name string
}

// define the key
var userIDKey = &contextKey{"user-id"}

// public accessor to store a value to ctx
func WithUserID(ctx context.Context, id int) context.Context {
    return context.WithValue(ctx, userIDKey, id)
}

// public accessor to fetch a value from ctx
func UserIDFromContext(ctx context.Context) (int, bool) {
    v, ok := ctx.Value(userIDKey).(int)
    return v, ok
}

// store value
ctx := WithUserID(context.Background(), 42)

// retrieve value
id, ok := UserIDFromContext(ctx)
if ok {
    fmt.Println(id) // 42
} else {
    fmt.Println("no user ID found in context")
}
```

This approach centralizes how values are stored and retrieved from the context. It ensures
the correct key and type are always used, preventing collisions and runtime panics. It also
keeps calling code shorter since users don't need to repeat type assertions everywhere.

This `WithX` / `XFromContext` convention appears throughout the Go standard library:

- **[net/http/httptrace]**

    ```go
    func WithClientTrace(ctx context.Context, trace *ClientTrace) context.Context
    func ContextClientTrace(ctx context.Context) *ClientTrace
    ```

- **[runtime/pprof]**

    ```go
    func WithLabels(ctx context.Context, labels LabelSet) context.Context
    func Labels(ctx context.Context) LabelSet
    ```

Outside the standard library, the [OpenTelemetry Go SDK] follows the same model:

```go
func ContextWithSpan(parent context.Context, span Span) context.Context
func SpanFromContext(ctx context.Context) Span
```

This technique standardizes how values are passed across APIs, eliminates redundant type
assertions, and prevents key misuse across packages.

### Closing words

I usually use a pointer to a struct as a key and [expose accessor functions] when building
user-facing APIs. Otherwise, an empty struct works fine as a key. In services, I often
define empty struct keys and expose them publicly to avoid the ceremony around accessor
functions.

<!-- References -->

<!-- prettier-ignore-start -->

 [doc has the following advice]:
    https://pkg.go.dev/context#example-WithValue:~:text=The%20provided%20key,pointer%20or%20interface.

[net/http]:
    https://cs.opensource.google/go/go/+/refs/tags/go1.25.3:src/net/http/server.go;l=239-251

[serve_test.go]:
    https://cs.opensource.google/go/go/+/refs/tags/go1.25.3:src/net/http/serve_test.go;l=5132-5144

[net/http/httptrace]:
    https://github.com/golang/go/blob/39ed968832ad8923a4bd1fb6bc3d9090ddd98401/src/net/http/httptrace/trace.go#L20-L68

[runtime/pprof]:
https://github.com/golang/go/blob/39ed968832ad8923a4bd1fb6bc3d9090ddd98401/src/runtime/pprof/label.go#L60-L63

[opentelemetry go sdk]:
    https://github.com/open-telemetry/opentelemetry-go/blob/f0c24571557de839332e48790714a5899c4fd2c6/trace/context.go

[expose accessor functions]:
    #2-expose-accessor-functions

<!-- prettier-ignore-end -->

---
title: Avoiding collisions in Go context keys
date: 2025-10-22
slug: avoid-context-key-collisions
tags:
    - Go
description: >-
  Master Go context keys with custom types, avoid collisions using empty structs, and
  learn accessor patterns for safe request-scoped values.
---

Along with propagating deadlines and cancellation signals, Go's `context` package can also
carry request-scoped values across API boundaries and processes.

There are only two public API constructs associated with context values:

```go
func WithValue(parent Context, key, val any) Context
func (c Context) Value(key any) any
```

`WithValue` can take any comparable value as both the key and the value. The key defines how
the stored value is identified, and the value can be any data you want to pass through the
call chain.

`Value`, on the other hand, also returns `any`, which means the compiler cannot infer the
concrete type at compile time. To use the returned data safely, you must perform a type
assertion.

A naive workflow to store and retrieve values in a context looks like this:

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

`WithValue` returns a new context that wraps the parent. `Value` walks up the chain of
contexts and returns the first matching key it finds. Since the return type is `any`, a type
assertion is required to recover the original type. Without the `ok` check, a mismatch would
cause a panic.

The issue with this setup is that it risks collision. If another package sets a value
against the same key, one overwrites the other:

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

The first value becomes inaccessible because `WithValue` returns a new derived context that
shadows parent values with the same key. The original value still exists in the parent
context but is unreachable through the reassigned variable.

To understand why this collision occurs, you need to know how Go compares interface values.
When you assign a value to an `interface{}` (or `any`), Go boxes that value into an internal
representation made up of two [machine words]: one points to the type information, and the
other points to the underlying data.

For example:

```go
var a any = "key"
var b any = "key"
fmt.Println(a == b) // true
```

Each boxed interface here stores two things: a pointer to the type `string` and a pointer to
the data `"key"`. Since both type and data pointers match, the comparison returns true.

`WithValue` stores both the key and the value as `any`. When you later call `Value`, Go
compares the boxed key you pass in with those stored in the context chain. If two different
packages use the same built-in key type and data, like both passing `"key"` as a string,
their boxed representations look identical. Go sees them as equal, and the most recent value
shadows the earlier one.

If you want to learn more about how interfaces are represented and compared, [Russ Cox's
post on Go interface internals] explains it in detail with pretty pictures.

The fix is to make sure the keys have unique types so their boxed representations differ. If
you define a custom type, the type pointer changes even if the data looks the same. For
example:

```go
type userKey string

var a any = userKey("key")
var b any = "key"
fmt.Println(a == b) // false
```

Even though the underlying value is `"key"`, the two interfaces now hold different type
information, so Go considers them unequal. That difference in type identity is what prevents
collisions.

The [context documentation] gives this advice:

> The provided key must be comparable and should not be of type string or any other built-in
> type to avoid collisions between packages using context. Users of WithValue should define
> their own types for keys. To avoid allocating when assigning to an interface{}, context
> keys often have concrete type struct{}. Alternatively, exported context key variables'
> static type should be a pointer or interface.

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

To avoid allocation when `WithValue` assigns the inbound value to interface `any`, you can
define an empty struct key. Unlike strings or integers, which allocate when boxed into an
interface, a zero-sized struct occupies no memory and needs no allocation:

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
uniqueness. When a pointer is boxed into an interface, no data copy occurs because the
interface just holds the pointer reference. Pointers are also ideal for keys that need to be
shared across packages.

```go
type userIDKey struct {
    name string
}

// Struct pointer as key
var UserIDKey = &userIDKey{"user-id"}

// Store value. No allocation here since userIDKey is a pointer
// to a struct
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

## 1. Expose keys directly

You can export the key itself and let users interact with it freely:

```go
type APIKey string

// Allow the other packages to directly use this key
var APIKeyContextKey = APIKey("api-key")

// Store value. An allocation will occur since the key is of type string
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

Each variable points to a distinct struct, making them unique by pointer identity.

The [serve_test.go] file uses these keys like this:

```go
ctx := context.WithValue(
    context.Background(), http.ServerContextKey, srv,
)

// Type assertion to recover the concrete type
srv2, ok := ctx.Value(http.ServerContextKey).(*http.Server)
if ok {
    fmt.Println(srv == srv2) // true
}
```

The server value is stored in the context and later retrieved using the same pointer key.
The user must perform a type assertion and handle it safely.

## 2. Expose accessor functions

The other approach is to hide the key and provide accessor functions to set and retrieve
values. This removes the need for users to remember the right key type or perform type
assertions manually.

```go
// Define a private key type to avoid collisions
type contextKey struct {
    name string
}

// Define the key
var userIDKey = &contextKey{"user-id"}

// Public accessor to store a value to ctx
func WithUserID(ctx context.Context, id int) context.Context {
    // No allocation here since userIDKey is a pointer to a struct
    return context.WithValue(ctx, userIDKey, id)
}

// Public accessor to fetch a value from ctx
func UserIDFromContext(ctx context.Context) (int, bool) {
    v, ok := ctx.Value(userIDKey).(int)
    return v, ok
}

// Store value
ctx := WithUserID(context.Background(), 42)

// Retrieve value
id, ok := UserIDFromContext(ctx)
if ok {
    fmt.Println(id) // 42
} else {
    fmt.Println("no user ID found in context")
}
```

This approach centralizes how values are stored and retrieved from the context. It ensures
the correct key and type are always used, preventing collisions and runtime panics. It also
keeps the calling code shorter since your API users won't need to repeat type assertions
everywhere.

`WithX` / `XFromContext` accessors appear throughout the Go standard library:

- **[net/http/httptrace]**

    ```go
    func WithClientTrace(
        ctx context.Context, trace *ClientTrace,
    ) context.Context
    func ContextClientTrace(ctx context.Context) *ClientTrace
    ```

- **[runtime/pprof]**

    ```go
    func WithLabels(ctx context.Context, labels LabelSet) context.Context
    func Labels(ctx context.Context) LabelSet
    ```

You can find similar examples outside of the stdlib. For instance, the [OpenTelemetry Go
SDK] follows the same model:

```go
func ContextWithSpan(parent context.Context, span Span) context.Context
func SpanFromContext(ctx context.Context) Span
```

This technique standardizes how values are passed across APIs, eliminates redundant type
assertions, and prevents key misuse across packages.

## Closing words

I usually use a pointer to a struct as a key and [expose accessor functions] when building
user-facing APIs. Otherwise, in services, I often define empty struct keys and expose them
publicly to avoid the ceremony around accessor functions.

<!-- references -->
<!-- prettier-ignore-start -->

[machine words]:
    https://unicminds.com/what-is-a-machine-word-and-its-implications/

[russ cox's post on go interface internals]:
    https://research.swtch.com/interfaces

[context documentation]:
    https://pkg.go.dev/context#WithValue

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

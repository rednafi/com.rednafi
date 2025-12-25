---
title: Go structured error handling pitfalls
---

Here’s the short version up front:

- In your first snippet, `errors.As` doesn’t match because the dynamic error value is `E` (a
  value), but you ask for `*E` (a pointer).
- In your second snippet, the dynamic error value is `*E`, so it matches `*E` and works.
- The rule behind this is “assignability” plus method sets; `As` needs a value in the chain
  that’s assignable to the type you ask for. ([Go Packages][1])

# Three-line intro

I hit a tiny error-handling puzzle in Go. Two nearly identical programs; one “catches” the
error, the other shrugs. The fix is one line, the reason lives in method sets and
assignability.

# The code

This one doesn’t print “yes error happened”:

```go
package main

import (
    "errors"
    "fmt"
)

type E struct {
    message string
    code    int
}

func (e E) Error() string { return fmt.Sprintf("error: %s, code: %d", e.message, e.code) }

func foo() error { return E{message: "some error happened", code: 101} }

func main() {
    fmt.Println("Hello, 世界")

    var e *E
    if errors.As(foo(), &e) { // asking for *E
        fmt.Println("yes error happened")
    }
}
```

This one does:

```go
package main

import (
    "errors"
    "fmt"
)

type E struct {
    message string
    code    int
}

func (e *E) Error() string { return fmt.Sprintf("error: %s, code: %d", e.message, e.code) }

func foo() error { return &E{message: "some error happened", code: 101} }

func main() {
    fmt.Println("Hello, 世界")

    var e *E
    if errors.As(foo(), &e) { // asking for *E
        fmt.Println("yes error happened")
    }
}
```

# Why the first one misses and the second one hits

## What `errors.As` really does

`As` “sets target to that error value” if it finds one in the chain that matches, and
“target must be a pointer.” More precisely, an error matches when “the error’s concrete
value is assignable to the value pointed to by target.” ([Go Packages][1])

- In the first program, `foo()` returns an `E` **value**. Your target is `*E`. A value of
  type `E` is **not assignable** to `*E`, so `As` returns false. ([Go Packages][1])
- In the second program, `foo()` returns `*E`. Your target is `*E`. `*E` is assignable to
  `*E`, so `As` succeeds and sets your `var e *E`. ([Go Packages][1])

## “But my `Error()` has a value receiver in the first program—doesn’t _that_ make `*E` implement `error` too?”

Yes, but that’s a different rule. The spec says: “The method set of a pointer to a defined
type `T` … is the set of all methods declared with receiver `*T` or `T`.” So with a **value
receiver** method, **both** `E` and `*E` implement `error`. That affects **interface
satisfaction**, not **assignability** between `E` and `*E`. `As` cares about assignability
to the **concrete target type**, not just “implements `error`.” ([Go][2])

## The quick matrix

| `Error()` receiver              | `foo()` returns | target var | `errors.As` |
| ------------------------------- | --------------- | ---------- | ----------- |
| value (`func (e E) Error()`)    | `E`             | `var e *E` | **false**   |
| value                           | `E`             | `var e E`  | **true**    |
| pointer (`func (e *E) Error()`) | `*E`            | `var e *E` | **true**    |
| pointer                         | `*E`            | `var e E`  | **false**   |

If you keep `foo()` returning `E`, change your target to `var e E` and pass `&e` to `As`. If
you want to keep `var e *E`, return `*E` from `foo()`.

# “Isn’t `var e *E` and then `errors.As(err, &e)` a double pointer?”

It looks like it, but the shape is exactly what `As` expects. You declare a variable **of
the target type** (`e` of type `*E`) and pass **a pointer to it** (`&e`) so `As` can assign
into it. The docs are explicit: `As` “sets target to that error value” and “panics if target
is not a non-nil pointer.” Your `&e` has type `**E`, but the “value pointed to by target” is
`*E`, which is the thing you want filled. ([Go Packages][1])

# A casual map of error handling in Go

- **Plain errors**: `errors.New` and `fmt.Errorf` create error values. “Package errors
  implements functions to manipulate errors,” and `fmt.Errorf("… %w …", err)` wraps an
  underlying error. ([Go Packages][1])
- **Sentinel checks** with `errors.Is`: Ask “does anything in the chain equal this target,
  or say it is equivalent?” The docs say `Is` matches if the error “is equal to that target”
  or implements a compatible `Is` method. This is what you use for values like
  `fs.ErrNotExist`. ([Go Packages][1])
- **Typed/structured checks** with `errors.As`: Ask “does anything in the chain have a
  concrete type assignable to what I want?” It also fills your variable so you can read
  fields like `code`. ([Go Packages][1])

If you like names: call the `Is` style **sentinel errors**, and the `As` style **typed
errors** or **structured errors**. The official materials avoid the taxonomy, but the usage
is clear in the package docs and the Go 1.13 blog post that introduced wrapping and chain
inspection. ([Go Packages][1], [Go][3])

# The bigger picture in the spec

Two spec rules explain everything:

1. **Method sets**: a pointer to `T` has methods with receiver `T` **or** `*T`. This is why
   `*E` also implements `error` when `Error()` has a value receiver. ([Go][2])
2. **Assignability** drives `errors.As`: it succeeds only when the concrete error value is
   assignable to the target type. The package docs say it plainly: match occurs when “the
   error’s concrete value is assignable to the value pointed to by target.” ([Go
   Packages][1])

# Minimal fixes you can apply

- Keep your first program’s shape and make the target a value:

    ```go
    var e E
    if errors.As(foo(), &e) { /* ... */ }
    ```

- Or keep the pointer target and return a pointer:

    ```go
    func foo() error { return &E{message: "...", code: 101} }
    ```

# Title ideas (single sentence, not clickbaity)

- **Why `errors.As` didn’t match: value vs pointer errors**
- **Matching typed errors in Go with `errors.As`**
- **`errors.As` and method sets: when your typed error is missed**

# References

- **Package `errors` docs**: overview, `Is`, `As`, wrapping, and the assignability rule for
  `As`. Short quotes above are from here. ([Go Packages][1])
- **Go spec: method sets** — pointer to `T` includes methods with receiver `T` or `*T`.
  ([Go][2])
- **Go blog: Working with Errors in Go 1.13** — background on wrapping and inspecting error
  chains. ([Go][3])
- **Go blog: Errors are values** — philosophy and idioms for error handling in Go. ([Go][4])

If you want, I can turn this into a ready-to-publish post with your preferred title and code
blocks lined up.

[1]: https://pkg.go.dev/errors "errors package - errors - Go Packages"
[2]:
    https://go.dev/ref/spec
    "The Go Programming Language Specification - The Go Programming Language"
[3]:
    https://go.dev/blog/go1.13-errors?utm_source=chatgpt.com
    "Working with Errors in Go 1.13 - The Go Programming Language"
[4]:
    https://go.dev/blog/errors-are-values?utm_source=chatgpt.com
    "Errors are values - The Go Programming Language"

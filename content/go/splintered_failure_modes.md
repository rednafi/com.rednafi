---
title: Splintered failure modes in Go
date: 2025-11-30
slug: splintered-failure-modes
tags:
    - Go
---

> _A man with a watch knows what time it is. A man with two watches is never sure._
>
> — [Segal's law]

Take this example:

```go
func validate(input string) (bool, error) {
    if input == "" {
        return false, nil
    }
    if isCorrupted(input) {
        return false, fmt.Errorf("corrupted")
    }
    return true, nil
}
```

This function returns two signals: a **boolean** to indicate if the string is valid, and
an **error** to explain any problem.

The issue is that these two signals are independent. Put together they produce four possible
combinations:

1.  `true, nil` means the input is valid and the function encountered no issues. This is the
    only obvious mode.

2.  `false, nil` implies the function didn't hit a system error but the input was invalid.
    However, in many codebases this combination is often used to hide real errors that were
    swallowed. The caller cannot be sure if this is a legitimate failure state or a bug in
    the code.

3.  `true, err` is a contradiction. The function claims success and failure at the same
    time. The caller has to decide which value to believe without any guidance.

4.  `false, err` looks like a clean failure, but it creates a priority trap. The Go
    convention dictates you must always check the error first. If a caller ignores this rule
    and checks the boolean first, they might see `false` and treat a major system crash as a
    simple validation failure.

In this specific case we never return `true, err`, but the caller doesn't know that. They
have to read the code to understand which subset of the possible combinations the function
actually uses.

## Splintered failure modes

For the lack of a better term, I call this **splintered failure modes**. It's one of the
special cases that the adage [make illegal state unrepresentable] aims to prevent.

In our case, `validate` encodes the failure modes in both the result and the error values.
These two signals can disagree and create contradictions. The boolean tries to express
validity and the error tries to express failure, yet both answer the same question: _did
this succeed?_ When combinations like `false, nil` or `true, err` appear, the caller has to
decide which signal to trust.

## Represent failure modes exclusively via error values

We fix the ambiguity by removing the boolean status flag entirely. Instead of returning a
flag about the data, we return the dumb result and the error values.

In this refactored version, the `error` assumes total responsibility for the function's
state (success vs. failure). The first return value becomes purely the payload.

The caller checks one place and one place only to understand the function's state: the
error.

```go
// We return the data (string), not a flag (bool)
func validate(input string) (string, error) {
    if input == "" {
        return "", fmt.Errorf("input cannot be empty")
    }
    if isCorrupted(input) {
        return "", fmt.Errorf("corrupted")
    }
    // If we are here, the data is valid
    return input, nil
}
```

This makes the call site trivial because the state is no longer split. Check the error. If
it is non-nil, the operation failed and the data is invalid. If it is nil, the operation
succeeded and the data is safe to use.

## But sometimes only success/failure isn't enough

Collapsing all the failure modes into the error value is relatively easy when you only need
to know whether some operation succeeded or failed. But sometimes just knowing whether
something failed or not isn't enough. The caller also need to know the type failure to take
specific action depending on the error kind. For this, we can leverage sentinel errors. For
instance:

We can use [sentinel errors] to introduce multiple failure modes:

```go
var (
    ErrEmpty     = errors.New("input cannot be empty")
    ErrCorrupted = errors.New("input is corrupted")

    // More system errors
    // ErrSystem =
)

func validate(input string) (string, error) {
    if input == "" {
        return "", ErrEmpty
    }
    if isCorrupted(input) {
        return "", ErrCorrupted
    }
    return input, nil
}
```

Here we've encoded two different failure modes with different error values like `ErrEmpty`
and `ErrCorrupted`. We could also add more error values here to signal some mechanical
upstream error if needed. This way the same `error` value in the `validate` function can
encode both validation and system errors at the same time.

Then the caller can use `errors.Is` to switch between the failure modes:

```go
val, err := validate(userData)
if err != nil {
    switch {
    case errors.Is(err, ErrEmpty):
        return
    case errors.Is(err, ErrCorrupted):
        log.Fatal(err)
    default:
        log.Fatal(err)
    }
}
```

If you need structured detail in your errors, use [error types] without changing the shape
of the function. The error value continues to own the entire sprectrum of the failure modes;
now with fields the caller can inspect.

```go
type ValidationError struct {
    Field  string
    Reason string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

// Add more types for system errors in similar fashion ...

func validate(input string) (string, error) {
    if input == "" {
        return "", &ValidationError{Field: "input", Reason: "empty"}
    }
    if isCorrupted(input) {
        return "", &ValidationError{Field: "input", Reason: "corrupted"}
    }
    return input, nil
}
```

Caller can use `errors.As` to switch between the failure modes:

```go
val, err := validate(userData)
if err != nil {
    var vErr *ValidationError
    if errors.As(err, &vErr) {
        switch vErr.Reason {
        case "empty":
            fmt.Println("please provide a value")
            return
        case "corrupted":
            log.Fatal("critical: data corrupted")
        default:
            log.Fatal(err)
        }
    }
    log.Fatal(err)
}
```

<!-- References -->

<!-- prettier-ignore-start -->

[segal's law]:
    https://en.wikipedia.org/wiki/Segal%2527s_law

[make illegal state unrepresentable]:
    https://khalilstemmler.com/articles/typescript-domain-driven-design/make-illegal-states-unrepresentable/

[sentinel errors]:
    https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully#:~:text=three%20core%20strategies.-,Sentinel%20errors,-The%20first%20category

[error types]:
    https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully#:~:text=will%20discuss%20next.-,Error%20types,-Error%20types%20are


<!-- prettier-ignore-end -->

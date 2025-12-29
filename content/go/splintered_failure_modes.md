---
title: Splintered failure modes in Go
date: 2025-11-30
slug: splintered-failure-modes
tags:
    - Go
description: >-
  Simplify Go error handling by consolidating validation and system errors. Learn when to
  return boolean vs error for clearer failure modes.
---

> _A man with a watch knows what time it is. A man with two watches is never sure._
>
> — [Segal's Law]

Take this example:

```go
func validate(input string) (bool, error) {
    // Validation check 1
    if input == "" {
        return false, nil
    }
    // Validation check 2
    if isCorrupted(input) {
        return false, nil
    }
    // System check
    if err := checkUpstream(); err != nil {
        return false, err
    }

    return true, nil
}
```

This function returns two signals: a **boolean** to indicate if the string is valid, and an
**error** to explain any problem the function might run into.

The issue is that these two signals are independent. Put together, they produce four
possible combinations:

1.  `true, nil`: The input is valid and the function encountered no issues. This is the only
    obvious mode.
2.  `false, nil`: Implies the function didn't hit a system error but the input was invalid.
    However, in many codebases, this combination is accidentally used to hide real errors
    that were swallowed.
3.  `true, err`: A contradiction. The function claims success and failure at the same time.
4.  `false, err`: Looks like a clean failure, but it creates a priority trap. The Go
    convention dictates you must check the error first. If a caller checks the boolean
    first, they might see `false` and treat a major system crash as a simple validation
    failure.

In this specific case, we never return `true, err`, but the caller doesn't know that. They
have to read the code to understand which subset of the possible combinations the function
actually uses.

## Splintered failure modes

For lack of a better term, I call this **splintered failure modes**. It is one of the cases
that the adage _[make illegal state unrepresentable]_ aims to prevent.

In our case, `validate` encodes the success/failure state in _two_ places. These two signals
can disagree. The boolean tries to express validity, and the error tries to express system
failure, yet both attempt to answer the same question: _did this succeed?_

When combinations like `false, nil` or `true, err` appear, the caller needs to know how to
reconcile the conflicting states.

## Represent failure modes exclusively via the error

We fix the ambiguity by removing the boolean status flag entirely.

In this refactored version, the `error` assumes total responsibility for the function's
state (success vs. failure). The first return value becomes purely the payload.

The caller checks one place and one place only: the error.

```go
// We return the data (string), not a flag (bool)
func validate(input string) (string, error) {
    if input == "" {
        return "", fmt.Errorf("input cannot be empty")
    }
    if isCorrupted(input) {
        return "", fmt.Errorf("input is corrupted")
    }
    if err := checkUpstream(); err != nil {
        return "", err
    }

    // If we are here, the data is valid
    return input, nil
}
```

This makes the call site trivial because the state is no longer split. If the error is
non-nil, the operation failed. If it is nil, the operation succeeded.

## Distinguishing failure types within the error

Sometimes the caller of a function needs to take different actions depending on the type of
an error. In that case, just knowing whether a function succeeded or failed isn't enough.

Removing the boolean removes the ambiguity, but it introduces a new question: _How do we
distinguish between "validation error" and "system failure"?_

Previously, the boolean represented validation outcome (valid/invalid), and the error
represented the system failures (crash/upstream). Now that we have consolidated everything
into `error`, we need a way to differentiate the _kind_ of failure without re-introducing a
second return value.

### Sentinel errors

We can use [sentinel errors] to encode multiple failure modes into one error variable. The
`error` return value remains the single source of truth for "did it fail?", but the
_content_ of that error tells us "how it failed."

```go
var (
    // Domain/Logic failures
    ErrEmpty     = errors.New("input cannot be empty")
    ErrCorrupted = errors.New("input is corrupted")

    // System/Mechanical failures
    ErrSystem    = errors.New("system failure")
)

func validate(input string) (string, error) {
    if input == "" {
        return "", ErrEmpty
    }
    if isCorrupted(input) {
        return "", ErrCorrupted
    }
    if err := checkUpstream(); err != nil {
        // We could return err directly, or wrap it
        return "", ErrSystem
    }
    return input, nil
}
```

We have unified the failure state (it is always just an `error`), but we haven't lost the
granularity. The caller can now use `errors.Is` to switch between the failure modes:

```go
val, err := validate(userData)
if err != nil {
    switch {
    case errors.Is(err, ErrEmpty):
        // Handle logic failure 1 (e.g. prompt user)
        return
    case errors.Is(err, ErrCorrupted):
        // Handle logic failure 2 (e.g. reject payload)
        return
    case errors.Is(err, ErrSystem):
        // Handle system failure (e.g. alert ops team)
        log.Fatal(err)
    default:
        log.Fatal(err)
    }
}
```

### Error types

If sentinels aren't enough (for example, if you need to know _which_ field failed
validation), you can use [error types]. This allows the single `error` value to carry
structured metadata while still adhering to the standard error interface.

Here, we map both "Empty" and "Corrupted" to a `ValidationError` type, while leaving system
errors as standard errors.

```go
type ValidationError struct {
    Field  string
    Reason string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

func validate(input string) (string, error) {
    if input == "" {
        return "", &ValidationError{Field: "input", Reason: "empty"}
    }
    if isCorrupted(input) {
        return "", &ValidationError{Field: "input", Reason: "corrupted"}
    }
    if err := checkUpstream(); err != nil {
        return "", err
    }
    return input, nil
}
```

The caller can then use `errors.As` to inspect the failure mode in detail:

```go
val, err := validate(userData)
if err != nil {
    var vErr *ValidationError

    // Check if the error is a logical ValidationError
    if errors.As(err, &vErr) {
        fmt.Printf("Validation failed on %s: %s", vErr.Field, vErr.Reason)
        return
    }

    // If not, it is a system failure
    log.Fatal(err)
}
```

By sticking to the `error` value as the single indicator of failure, we eliminate the _"two
watches"_ paradox. Whether the failure is a simple validation error or a catastrophic system
crash, all the failure modes are encapsulated inside the single error value itself.

<!-- references -->

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

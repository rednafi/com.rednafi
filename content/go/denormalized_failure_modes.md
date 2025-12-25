---

title: Denormalized failure modes in Go
date: 2025-11-25
slug: denormalized-failure-modes
tags:
    - Go

---

> _A man with a watch knows what time it is. A man with two watches is never sure._
>
> — [Segal's law]

Consider this example:

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

This function takes a string and tries to figure out if it's valid. It kicks back two
signals: a **boolean** to tell you if the string is **valid**, and an **error** that
explains any **problem** it ran into.

The issue is that these two signals are independent. Put together they produce four possible
combinations:

1.  `true, nil` means the input is valid and the function didn't run into trouble. This is
    the only obvious mode. The rest force the caller to guess what the function meant.

2.  `false, nil` implies the function didn't hit any system error but the input was invalid.
    However, in many codebases this combination is often used to hide real errors that were
    swallowed, so the caller can't be sure if this is a failure or a success state.

3.  `true, err` claims success and failure at the same time. The caller has to decide which
    value to believe without any guidance.

4.  `false, err` looks like a clean failure. But this creates a priority trap for the
    caller. The Go convention is that you must always check the error first. If a caller
    ignores this rule and checks the boolean first, they might see false and treat a major
    system crash (like a disk error) as a simple validation failure.

In our case we never return `true, err`, but the caller doesn't know that. They have to read
the code to understand which subset of all the possible combinations the function actually
uses.

## The failure mode is denormalized across result & error values

For the lack of a better term, I call this **denormalized failure modes**. It's one of the
special cases that the adage [make illegal state unrepresentable] aims to prevent.

In database theory, denormalization means storing the same data in multiple places. It can
be desirable for read speed and undesirable for writes and consistency. SQL design typically
favors normalization: each fact appears once. The write inconsistency is prevented with
transactions.

In our case, `validate` encodes the failure mode in both the result value and the error
value. These two signals can disagree and create contradictions. The boolean tries to
express validity and the error tries to express failure, yet both answer the same question:
did this succeed. When combinations like `false, nil` or `true, err` appear, the caller has
to decide which signal to trust.

A more dire version of the same issue shows up when the result value isn't a boolean but an
enum. That variant makes the ambiguity even easier to produce, because the function can mix
domain failure mode

```go
type Status byte

const (
    Unknown Status = iota
    Success
    Failure
)

func process(input string) (Status, error) {
    if somethingWrong(input) {
        return Failure, nil
    }
    if isCorrupted(input) {
        return Unknown, fmt.Errorf("corrupted")
    }
    return Success, nil
}
```

Here the enum tries to express the failure mode while the error reports some other kind of
failure. Nothing in the function signature explains how the two are supposed to relate or
which combinations are valid. Since they operate independently, every enum value can appear
with or without an error. That produces pairs the caller cannot interpret without studying
the implementation.

To understand the behavior of `process` the caller has to inspect each branch and piece
together the unwritten rules:

- Is `Unknown` a real domain state or just a corruption fallback?
- Is `Failure, nil` an actual domain failure or a masked error?
- When does the error override the enum and vice versa?

Without answers to those questions, any `(Result, error)` combination is ambiguous and the
function's contract is something the caller has to reverse engineer.

## Use only `error` to signal failure modes

We fix the ambiguity by collapsing all failure modes into the error. Use `error` to signal
success or failure and use the result value only for data. The entire spectrum of failure
modes live inside the error value instead of being split across a result value and an error
value.

The caller checks one place and one place only. If the error is non-nil the operation
failed. If the error is nil the operation succeeded and the data is valid.

Applied to `validate`, the function returns the validated input and an error. The error owns
the failure mode; the data is just data.

```go
func validate(input string) (string, error) {
    if input == "" {
        return "", fmt.Errorf("input cannot be empty")
    }
    if isCorrupted(input) {
        return "", fmt.Errorf("corrupted")
    }
    return input, nil
}
```

This makes the call site trivial. Check the error. If it is non-nil the string is unusable.
If it is nil the string is valid.

## Sometimes you may need to differentiate between failure modes

Sometimes the caller needs to react to different failure modes. Maybe they retry on
transient errors, show a friendly message for bad input, or crash on corruption.

These distinctions still belong inside the error value. You can keep the failure mode of the
domain and the system all in the same error value. Don't reintroduce a second signal through
booleans or enums for this.

We can use [sentinel errors][sentinel errors] to introduce multiple failure modes:

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

If you need structured detail, use [typed errors][typed errors] without changing the shape
of the function. The error continues to own the entire failure mode; now with fields the
caller can inspect.

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

The `process` function can be fixed in a similar way. Since the function produces
side-effects and doesn't need to return any result value, we can get rid of the `Status`
enum altogether here. Instead we collapse all the failure modes in the sentinel error
values.

```go
var (
    ErrInvalid   = errors.New("validation failed")
    ErrCorrupted = errors.New("data corrupted")
)

func process(input string) error {
    if somethingWrong(input) {
        return ErrInvalid
    }
    if isCorrupted(input) {
        return ErrCorrupted
    }
    return nil
}
```

Similarly, the caller can use `errors.Is` to switch between the failure modes:

```go
err := process(userData)
if err != nil {
    switch {
    case errors.Is(err, ErrCorrupted):
        log.Fatal("critical failure")
    case errors.Is(err, ErrInvalid):
        fmt.Println("please check your input")
    default:
        log.Fatal(err)
    }
}
```

If you later need more context, replace `ErrCorrupted` with [custom typed errors]. The
signature stays the same and you never reintroduce a second success or failure signal.

> Never split the failure mode across multiple values. Collapse all failure modes into the
> error. If you need to distinguish failure modes encode them as sentinel errors or custom
> error types.

[segal's law]:
    https://www.google.com/search?q=%5Bhttps://en.wikipedia.org/wiki/Segal%2527s_law%5D(https://en.wikipedia.org/wiki/Segal%2527s_law)
[make illegal state unrepresentable]:
    https://www.google.com/search?q=%5Bhttps://www.angus-morrison.com/blog/type-driven-design-go%5D(https://www.angus-morrison.com/blog/type-driven-design-go)
[sentinel errors]:
    https://www.google.com/search?q=%5Bhttps://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully%23:~:text%3Dthree%2520core%2520strategies.-,Sentinel%2520errors,-The%2520first%2520category%5D(https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully%23:~:text%3Dthree%2520core%2520strategies.-,Sentinel%2520errors,-The%2520first%2520category)
[typed errors]:
    https://www.google.com/search?q=%5Bhttps://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully%23:~:text%3Dwill%2520discuss%2520next.-,Error%2520types,-Error%2520types%2520are%5D(https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully%23:~:text%3Dwill%2520discuss%2520next.-,Error%2520types,-Error%2520types%2520are)

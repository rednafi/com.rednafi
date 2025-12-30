---
title: Retry function in Go
date: 2024-02-04
slug: retry-function
aliases:
    - /go/retry_function/
tags:
    - Go
    - TIL
description: >-
  Build retry logic in Go without reflection using generics. Implement exponential
  backoff and configurable retry strategies with type safety.
---

I used to reach for reflection whenever I needed a `Retry` function in Go. It's fun to
write, but gets messy quite quickly.

Here's a rudimentary `Retry` function that does the following:

-   It takes in another function that accepts arbitrary arguments.
-   Then tries to execute the wrapped function.
-   If the wrapped function returns an error after execution, `Retry` attempts to run the
    underlying function `n` times with some backoff.

The following implementation leverages the `reflect` module to achieve the above goals.
We're intentionally avoiding complex retry logic for brevity:

```go
func Retry(
    fn any, args []any, maxRetry int,
    startBackoff, maxBackoff time.Duration) ([]reflect.Value, error) {

    fnVal := reflect.ValueOf(fn)
    if fnVal.Kind() != reflect.Func {
        return nil, errors.New("retry: function type required")
    }

    argVals := make([]reflect.Value, len(args))
    for i, arg := range args {
        argVals[i] = reflect.ValueOf(arg)
    }

    for attempt := 0; attempt < maxRetry; attempt++ {
        result := fnVal.Call(argVals)
        errVal := result[len(result)-1]

        if errVal.IsNil() {
            return result, nil
        }
        if attempt == maxRetry-1 {
            return result, errVal.Interface().(error)
        }
        time.Sleep(startBackoff)
        if startBackoff < maxBackoff {
            startBackoff *= 2
        }
        fmt.Printf(
            "Retrying function call, attempt: %d, error: %v\n",
            attempt+1, errVal,
        )
    }
    return nil, fmt.Errorf("retry: max retries reached without success")
}
```

The `Retry` function uses reflection to call a function passed as `any`. It takes the
function's arguments as a `[]any` slice, allowing us to run functions with varied
signatures. Using `reflect.ValueOf(fn).Call(argVals)`, it dynamically invokes the target
function after converting arguments from `any` to `reflect.Value`.

The retry logic runs up to `maxRetry` times with exponential backoff. The delay starts at
`startBackoff`, doubles after each failure, and caps at `maxBackoff`. If the last return
value is an error and retries remain, it waits and tries again. Otherwise, it gives up.

You can wrap a dummy function that always returns an error to see how `Retry` works:

```go
func main() {
    someFunc := func(a, b int) (int, error) {
        fmt.Printf("Function called with a: %d and b: %d\n", a, b)
        return 42, errors.New("some error")
    }
    result, err := Retry(
        someFunc, []any{42, 100}, 3, 1*time.Second, 4*time.Second,
    )
    if err != nil {
        fmt.Println("Function execution failed:", err)
        return
    }
    fmt.Println("Function executed successfully:", result[0])
}
```

Running it will give you the following output:

```txt
Function called with a: 42 and b: 100
Retrying function call, attempt: 1, error: some error
Function called with a: 42 and b: 100
Retrying function call, attempt: 2, error: some error
Function called with a: 42 and b: 100
Function execution failed: some error
```

This isn't too terrible for reflection-heavy code. But now that Go has generics, I wanted to
see if I could avoid the metaprogramming. While reflection is powerful, it's prone to
runtime panics and the compiler can't type-check dynamic code.

Turns out, there's a way to write the same functionality with generics if you don't mind
trading off some flexibility for shorter and more type-safe code. Here's how:

```go
// Define a generic function type that can return an error
type Func[T any] func() (T, error)

func Retry[T any](
    fn Func[T], args []any, maxRetry int,
    startBackoff, maxBackoff time.Duration) (T, error) {

    var zero T // Zero value for the function's return type

    for attempt := 0; attempt < maxRetry; attempt++ {
        result, err := fn(args...)

        if err == nil {
            return result, nil
        }
        if attempt == maxRetry-1 {
            return zero, err // Return with error after max retries
        }
        fmt.Printf(
            "Retrying function call, attempt: %d, error: %v\n",
            attempt+1, err,
        )
        time.Sleep(startBackoff)
        if startBackoff < maxBackoff {
            startBackoff *= 2
        }
    }
    return zero, fmt.Errorf("retry: max retries reached without success")
}
```

Functionally, the generic implementation works the same way as the previous one. However, it
has a few limitations:

-   The generic `Retry` assumes the wrapped function returns `(result, error)`. This fits
    Go's common idiom, but the reflection version could handle varied return patterns.

-   The reflection-based `Retry` wraps any function via the empty interface. The generic
    version requires a matching signature, so you need a thin wrapper to adapt it.

Here's how you'd use the generic `Retry` function:

```go
func main() {
    someFunc := func(a, b int) (int, error) {
        fmt.Printf("Function called with a: %d and b: %d\n", a, b)
        return 42, errors.New("some error")
    }
    wrappedFunc := func(args ...any) (any, error) {
        return someFunc(args[0].(int), args[1].(int))
    }
    result, err := Retry(
        wrappedFunc,
        []any{42, 100},
        3,
        1*time.Second,
        4*time.Second,
    )
    if err != nil {
        fmt.Println("Function execution failed:", err)
    } else {
        fmt.Println("Function executed successfully:", result)
    }
}
```

Running it will give you the same output as before.

Notice how `someFunc` is wrapped in `wrappedFunc` to match the signature `Retry` expects.
This adaptation is necessary for type safety. I don't mind it if it means avoiding
reflection—plus the generic version is slightly faster.

After this entry went live, [Anton Zhiyanov pointed out on Twitter] that there's a
closure-based approach that's even simpler and eliminates the need for generics. The
implementation looks like this:

```go
func Retry(
    fn func() error,
    maxRetry int, startBackoff, maxBackoff time.Duration) {

    for attempt := 0; ; attempt++ {
        if err := fn(); err == nil {
            return
        }

        if attempt == maxRetry-1 {
            return
        }

        fmt.Printf("Retrying after %s\n", startBackoff)
        time.Sleep(startBackoff)
        if startBackoff < maxBackoff {
            startBackoff *= 2
        }
    }
}
```

Now calling `Retry` is easier since the closure signature is static—you don't need to adapt
the call when the wrapped function's signature changes:

```go
func main() {
    someFunc := func(a, b int) (int, error) {
        fmt.Printf("Function called with a: %d and b: %d\n", a, b)
        return 42, errors.New("some error")
    }

    var res int
    var err error

    Retry(
        func() error {
            res, err = someFunc(42, 100)
            return err
        },
        3, 1*time.Second,
        4*time.Second,
    )

    fmt.Println(res, err)
}
```

The runtime behavior of this version is the same as the ones before.

Fin!

<!-- references -->
<!-- prettier-ignore-start -->

[anton zhiyanov pointed out on twitter]:
    https://twitter.com/ohmypy/status/1754105508863393835

<!-- prettier-ignore-end -->

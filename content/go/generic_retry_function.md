---
title: Generic retry function in Go
date: 2024-02-04
tags:
    - Go
    - TIL
---

Writing a retry function in Go usually involves mucking around with reflections and doing
some metaprogramming magic. It's fun to write, but whenever I come back to the code a few
months later, I always cringe at the mess.

Here's a rudimentary `Retry` function that does the following:

-   It takes in another function that accepts arbitrary arguments.
-   Then tries to execute the wrapped function.
-   If the wrapped function returns an error after execution, `Retry` attempts to run the
    underlying function `n` times with some backoff.

The following implementation leverages the `reflections` module to achieve the above goals.
We're intentionally avoiding complex retry logic for brevity:

```go
func Retry(fn interface{}, args []interface{}, maxRetry int,
    backoff, maxBackoff time.Duration) ([]reflect.Value, error) {

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
        if errVal := result[len(result)-1]; !errVal.IsNil() {
            if attempt == maxRetry-1 {
                return result, errVal.Interface().(error)
            }
            time.Sleep(backoff)
            if backoff < maxBackoff {
                backoff *= 2
            }
            fmt.Printf(
                "Retrying function call, attempt: %d, error: %v\n",
                attempt+1, errVal,
            )
        } else {
            return result, nil
        }
    }
    return nil, fmt.Errorf("retry: max retries reached without success")
}
```

The Retry function uses reflection to call a function passed as an `interface{}`. It handles
the function's arguments, which are given in an `interface{}` slice. This approach allows us
to run functions with varied signatures. Reflection, using
`reflect.ValueOf(fn).Call(argVals)`, dynamically invokes the target function. It converts
its arguments from `interface{}` to `reflect.Value` types.

Within the retry logic, it tries up to `maxRetry` times, using exponential backoff to set
the delay between retries. The delay begins at `startBackoff`, doubles after each failure,
and is limited by `maxBackoff` to avoid long waits. The function looks for errors in the
last return value of the called function. If it finds an error and there are retries left,
it waits for the backoff period before trying again. Otherwise, it gives up with an error
message.

You can wrap a dummy function that always returns an error to see how `Retry` works:

```go
func main() {
    someFunc := func(a, b int) (int, error) {
        fmt.Printf("Function called with a: %d and b: %d\n", a, b)
        return 42, errors.New("some error")
    }

    result, err := Retry(
        someFunc, []interface{}{42, 100}, 3, 1*time.Second, 4*time.Second,
    )

    if err != nil {
        fmt.Println("Function execution failed:", err)
    } else {
        fmt.Println("Function executed successfully:", result[0])
    }
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

This isn't too terrible for a reflection infested code snippet. However, now that Go has
generics, I wanted to see if I could leverage that to avoid metaprogramming. While
reflection is powerful, it's quite easy to run buggy code that causes runtime panics. Plus,
the compiler can't do many of the type checks when the underlying code leverages the dynamic
features.

Turns out, there's a way to write the same functionality with generics if you don't mind
trading off some flexibility for shorter and more type safe code. Here's how:

```go
// Define a generic function type that can return an error
type Func[T any] func(args ...any) (T, error)

func Retry[T any](
    fn Func[T], args []any, maxRetry int,
    backoff, maxBackoff time.Duration) (T, error) {

    var zero T // Zero value for the function's return type

    for attempt := 0; attempt < maxRetry; attempt++ {
        result, err := fn(args...)
        if err != nil {
            if attempt == maxRetry-1 {
                return zero, err // Return with error after max retries
            }
            fmt.Printf(
                "Retrying function call, attempt: %d, error: %v\n",
                attempt+1, err,
            )
            time.Sleep(backoff)
            if backoff < maxBackoff {
                backoff *= 2
            }
        } else {
            return result, nil // Successful call without error
        }
    }
    return zero, fmt.Errorf("retry: max retries reached without success")
}
```

Functionally, the generic implementation works the same way as the previous one. However, it
has a few limitations:

-   The generic `Retry` function assumes that the wrapped function will alwasy return the
    result as the first value and error as the second. This works well since it's a common
    Go idiom, but the reflection version could dynamically handle different return value
    patterns.

-   The reflection-based `Retry` can directly wrap any function because it accepts an empty
    interface. The generic `Retry` needs the target function to match the expected
    signature. So you have to create a thin wrapper function to adapt the signatures. This
    wrapper function is necessary to make the process somewhat type-safe.

-   With reflection, `Retry` can call functions with varied argument types since it works
    with empty interfaces. The generic version needs the argument types to match. So there
    may be cases where you have to do more work to adapt the arguments.

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
        wrappedFunc, []interface{}{42, 100}, 3, 1*time.Second, 4*time.Second,
    )

    if err != nil {
        fmt.Println("Function execution failed:", err)
    } else {
        fmt.Println("Function executed successfully:", result)
    }
}
```

Running it will give you the same output as before.

```txt
Function called with a: 42 and b: 100
Retrying function call, attempt: 1, error: some error
Function called with a: 42 and b: 100
Retrying function call, attempt: 2, error: some error
Function called with a: 42 and b: 100
Function execution failed: some error
```

Notice how `someFunc` is wrapped in a `wrappedFunc` where `wrappedFunc` has the signature
that `Retry` expects. Then inside, the `someFunc` function is called with the appropriate
arguments. This type adaptation gymnastic is necessary to make the process acceptably type
safe. Personally, I don't mind it if it means I get to avoid reflections to achieve the same
result. Also, the generic version is a tad bit more performant!

Fin!

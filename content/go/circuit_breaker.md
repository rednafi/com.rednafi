---
title: Writing a circuit breaker in Go
date: 2024-10-06
tags:
    - Networking
    - Go
mermaid: true
---

Besides retries, circuit breakers[^1] are probably one of the most commonly employed
resilience patterns in distributed systems. While writing a retry routine is pretty
simple, implementing a circuit breaker needs a little bit of work.

I realized that I usually just go for off-the-shelf libraries for circuit breaking and
haven't written one from scratch before. So, this is an attempt to create a sloppy one in
Go. I picked Go instead of Python because I didn't want to deal with sync-async
idiosyncrasies or abstract things away under a soup of decorators.

## Circuit breakers

A circuit breaker acts like an automatic switch that prevents your application from
repeatedly trying to execute an operation that's likely to fail. In a distributed system,
you don't want to bombard a remote service when it's already failing, and circuit breakers
prevent that.

It has three states: **Closed**, **Open**, and **Half-Open**. Here's a diagram that shows
the state transitions:

<!-- prettier-ignore-start -->

{{< mermaid >}}
stateDiagram-v2
    [*] --> Closed: Start
    Closed --> Open: Failure threshold reached
    Open --> HalfOpen: Recovery period expired
    HalfOpen --> Closed: Success threshold reached
    HalfOpen --> Open: Request failed

    note right of Closed: All requests are allowed
    note right of Open: Requests are blocked
    note right of HalfOpen: Limited requests allowed to check recovery
{{</ mermaid >}}

<!-- prettier-ignore-end -->

1. **Closed**: This is the healthy operating state where all requests are allowed to pass
   through to the service. If a certain number of consecutive requests fail (reaching a
   failure threshold), the circuit breaker switches to the **Open** state.

2. **Open**: In this state, all requests are immediately blocked, and an error is returned
   to the caller without attempting to contact the failing service. This prevents
   overwhelming the service and gives it time to recover. After a predefined recovery
   period, the circuit breaker transitions to the **Half-Open** state.

3. **Half-Open**: The circuit breaker allows a limited number of test requests to see if the
   service has recovered. If these requests succeed, it transitions back to the **Closed**
   state. If any of them fail, it goes back to the **Open** state.

## Building one in Go

Here's a simple circuit breaker in Go.

### Defining states

First, we'll define the constants for our states and create the `circuitBreaker` struct,
which holds all the configurable knobs.

```go
// The three possible states of a circuit breaker
const (
    Closed   = "closed"
    Open     = "open"
    HalfOpen = "half-open"
)

// circuitBreaker manages the state and behavior of the circuit breaker
type circuitBreaker struct {
    mu                   sync.Mutex // Guards the circuit breaker state
    state                string  // Current state of the circuit breaker
    failureCount         int     // Number of consecutive failures
    lastFailureTime      time.Time // Time of the last failure
    halfOpenSuccessCount int    // Successful requests in half-open state

    failureThreshold     int   // Failures to trigger open state
    recoveryTime         time.Duration // Wait time before half-open
    halfOpenMaxRequests  int   // Requests allowed in half-open state
    timeout              time.Duration // Timeout for requests
}
```

This struct includes:

-   `mu`: A mutex to ensure thread-safe access to the circuit breaker.
-   `state`: The current state of the circuit breaker (`Closed`, `Open`, or `Half-Open`).
-   `failureCount`: The current count of consecutive failures.
-   `lastFailureTime`: The timestamp of the last failure.
-   `halfOpenSuccessCount`: The number of successful requests in the Half-Open state.
-   `failureThreshold`: The number of consecutive failures allowed before opening the
    circuit.
-   `recoveryTime`: The cool-down period before the circuit breaker transitions from Open to
    Half-Open.
-   `halfOpenMaxRequests`: The maximum number of successful requests needed to close the
    circuit.
-   `timeout`: The maximum duration to wait for a request to complete.

### Initializing the circuit breaker

Next, we provide a constructor function to initialize a new `circuitBreaker` instance.

```go
// NewCircuitBreaker initializes a new CircuitBreaker
func NewCircuitBreaker(
    failureThreshold int,
    recoveryTime time.Duration,
    halfOpenMaxRequests int,
    timeout time.Duration,
) *circuitBreaker {
    return &circuitBreaker{
        state:               Closed,
        failureThreshold:    failureThreshold,
        recoveryTime:        recoveryTime,
        halfOpenMaxRequests: halfOpenMaxRequests,
        timeout:             timeout,
    }
}
```

This function sets the initial state to `Closed` and initializes the thresholds and timeout.

### Implementing the Call method

The `Call` method is the primary interface for executing functions through the circuit
breaker. It dispatches the appropriate state handler based on the current state.

```go
// Call attempts to execute the provided function, managing state transitions
func (cb *circuitBreaker) Call(fn func() (any, error)) (any, error) {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    slog.Info("Making a request", "state", cb.state)

    switch cb.state {
    case Closed:
        return cb.handleClosedState(fn)
    case Open:
        return cb.handleOpenState()
    case HalfOpen:
        return cb.handleHalfOpenState(fn)
    default:
        return nil, errors.New("unknown circuit state")
    }
}
```

We use a mutex to protect against concurrent access since the circuit breaker might be used
by multiple goroutines. The `Call` method uses a switch statement to delegate the function
call to the appropriate handler based on the current state.

### Handling closed states

In the **Closed** state, all requests are allowed to pass through. We monitor the requests
for failures to decide when to trip the circuit breaker.

```go
// handleClosedState executes the function and monitors failures
func (cb *circuitBreaker) handleClosedState(fn func() (any, error)) (any, error) {
    result, err := cb.runWithTimeout(fn)
    if err != nil {
        slog.Warn(
            "Request failed in closed state",
            "failureCount", cb.failureCount+1,
        )
        cb.failureCount++
        cb.lastFailureTime = time.Now()

        if cb.failureCount >= cb.failureThreshold {
            cb.state = Open
            slog.Error("Failure threshold reached, transitioning to open")
        }
        return nil, err
    }

    slog.Info("Request succeeded in closed state")
    cb.resetCircuit()
    return result, nil
}
```

In this function:

-   We attempt to execute the provided function `fn` using `runWithTimeout` to handle
    possible timeouts.
-   If the function call fails, we increment the `failureCount` and update
    `lastFailureTime`.
-   If the `failureCount` reaches the `failureThreshold`, we transition the circuit to the
    **Open** state.
-   If the function call succeeds, we reset the circuit breaker to the **Closed** state by
    calling `resetCircuit`.

#### Resetting the circuit breaker

When a request succeeds, we reset the failure count and keep the circuit in the **Closed**
state.

```go
// resetCircuit resets the circuit breaker to closed state
func (cb *circuitBreaker) resetCircuit() {
    cb.failureCount = 0
    cb.state = Closed
    slog.Info("Circuit reset to closed state")
}
```

### Handling open states

In the **Open** state, all requests are blocked to prevent further strain on the failing
service. We check if the recovery period has expired before transitioning to the
**Half-Open** state.

```go
// handleOpenState blocks requests if recovery time hasn't passed
func (cb *circuitBreaker) handleOpenState() (any, error) {
    if time.Since(cb.lastFailureTime) > cb.recoveryTime {
        cb.state = HalfOpen
        cb.halfOpenSuccessCount = 0
        cb.failureCount = 0
        slog.Info("Recovery period over, transitioning to half-open")
        return nil, nil
    }

    slog.Warn("Circuit is still open, blocking request")
    return nil, errors.New("circuit open, request blocked")
}
```

Here:

-   We check if the recovery period (`recoveryTime`) has passed since the last failure.
-   If it has, we transition to the **Half-Open** state and reset the counters.
-   If not, we block the request and return an error immediately.

### Handling half-open states

In the **Half-Open** state, we allow a limited number of requests to test if the service has
recovered.

```go
// handleHalfOpenState executes the function and checks for recovery
func (cb *circuitBreaker) handleHalfOpenState(
    fn func() (any, error)) (any, error) {

    result, err := cb.runWithTimeout(fn)
    if err != nil {
        slog.Error("Failed in half-open state, transitioning to open")
        cb.state = Open
        cb.lastFailureTime = time.Now()
        return nil, err
    }

    cb.halfOpenSuccessCount++
    slog.Info("Succeeded in half-open",
        "successCount", cb.halfOpenSuccessCount)

    if cb.halfOpenSuccessCount >= cb.halfOpenMaxRequests {
        slog.Info("Max success, transitioning to closed")
        cb.resetCircuit()
    }

    return result, nil
}
```

In this function:

-   We attempt to execute the provided function `fn`.
-   If the function call fails, we transition back to the **Open** state.
-   If the function call succeeds, we increment `halfOpenSuccessCount`.
-   Once the success count reaches `halfOpenMaxRequests`, we reset the circuit breaker to
    the **Closed** state.

### Running functions with timeout

To prevent the circuit breaker from hanging on slow or unresponsive functions, we implement
a timeout mechanism. You probably noticed that inside each state handler we called the
wrapped functions with `runWithTimeout`.

```go
// runWithTimeout executes the provided function with a timeout
func (cb *circuitBreaker) runWithTimeout(fn func() (any, error)) (any, error) {
    ctx, cancel := context.WithTimeout(context.Background(), cb.timeout)
    defer cancel()

    resultChan := make(chan struct {
        result any
        err    error
    }, 1)

    go func() {
        result, err := fn()
        resultChan <- struct {
            result any
            err    error
        }{result, err}
    }()

    select {
    case <-ctx.Done():
        return nil, errors.New("request timed out")
    case res := <-resultChan:
        return res.result, res.err
    }
}
```

This function:

-   Creates a context with a timeout using `context.WithTimeout`.
-   Executes the provided function `fn` in a separate goroutine.
-   Waits for either the result or the timeout.
-   Returns an error if the function takes longer than the specified timeout.

### Taking it for a spin

Let's test our circuit breaker with an unreliable service that sometimes fails.

```go
func unreliableService() (any, error) {
    if time.Now().Unix()%2 == 0 {
        return nil, errors.New("service failed")
    }
    return "Success!", nil
}
```

In the `main` function, we'll create a circuit breaker and make several calls to the
unreliable service.

```go
func main() {
    cb := cb.NewCircuitBreaker(
        2,             // Failure threshold
        2*time.Second, // Recovery time
        2,             // Half-open max requests
        2*time.Second, // Half-open max time
    )

    for i := 0; i < 5; i++ {
        result, err := cb.Call(unreliableService)
        if err != nil {
            slog.Error("Service request failed", "error", err)
        } else {
            slog.Info("Service request succeeded", "result", result)
        }

        time.Sleep(1 * time.Second)
        log.Println("-------------------------------------------")
    }
}
```

This loop simulates multiple service calls, using the circuit breaker to handle failures and
transitions between states.

This prints:

```txt
2024/10/06 17:24:27 INFO Making a request state=closed
2024/10/06 17:24:27 INFO Request succeeded in closed state
2024/10/06 17:24:27 INFO Circuit reset to closed state
2024/10/06 17:24:27 INFO Service request succeeded result=42
2024/10/06 17:24:28 -----------------------------------------------
2024/10/06 17:24:28 INFO Making a request state=closed
2024/10/06 17:24:28 WARN Request failed in closed state failureCount=1
2024/10/06 17:24:28 ERROR Service request failed error="service failed"
2024/10/06 17:24:29 -----------------------------------------------
2024/10/06 17:24:29 INFO Making a request state=closed
2024/10/06 17:24:29 INFO Request succeeded in closed state
2024/10/06 17:24:29 INFO Circuit reset to closed state
2024/10/06 17:24:29 INFO Service request succeeded result=42
2024/10/06 17:24:30 -----------------------------------------------
2024/10/06 17:24:30 INFO Making a request state=closed
2024/10/06 17:24:30 WARN Request failed in closed state failureCount=1
2024/10/06 17:24:30 ERROR Service request failed error="service failed"
2024/10/06 17:24:31 -----------------------------------------------
2024/10/06 17:24:31 INFO Making a request state=closed
2024/10/06 17:24:31 INFO Request succeeded in closed state
2024/10/06 17:24:31 INFO Circuit reset to closed state
2024/10/06 17:24:31 INFO Service request succeeded result=42
2024/10/06 17:24:32 -----------------------------------------------
```

The log messages will give you a sense of what's happening when we retry an intermittently
failing function wrapped in a circuit breaker.

## The API could be better

One limitation of Go generics is that you can't use type parameters with methods that have a
receiver. This means you can't define a method like
`func (cb *CircuitBreaker[T]) Call(fn func() (T, error)) (T, error)`.

Due to this constraint, we have to use workarounds such as using `any` (an alias for
`interface{}`) as the return type in our function signatures. While this sacrifices some
type safety, it allows us to create a flexible circuit breaker that can handle functions
returning different types.

## Handling incompatible function signatures

What if the function you want to wrap doesn't match the `func() (any, error)` signature? You
can easily adapt it by wrapping your function to fit the required signature.

Suppose you have a function like this:

```go
func fetchData(id int) (Data, error) {
    // ... implementation ...
}
```

You can wrap it like this:

```go
wrappedFunc := func() (any, error) {
    return fetchData(42) // Replace 42 with your desired argument
}
```

Now, `wrappedFunc` matches the `func() (any, error)` signature and can be used with our
circuit breaker.

Here's the complete implementation[^2] with tests.

[^1]: [Circuit breaker — Martin Fowler](https://martinfowler.com/bliki/CircuitBreaker.html)

[^2]: [Circuit breaker implementation in Go](https://github.com/rednafi/circuit-breaker)

---
title: Function types and single-method interfaces in Go
date: 2024-12-22
tags:
    - Go
---

People love single-method interfaces (SMIs) in Go. They're simple to implement and easy to
reason about. The standard library is packed with SMIs like `io.Reader`, `io.Writer`,
`io.Closer`, `io.Seeker`, and more.

One cool thing about SMIs is that you don't always need to create a full-blown struct with a
method to satisfy the interface. You can define a function type, attach the interface method
to it, and use it right away. This approach works well when there's no state to maintain, so
the extra struct becomes unnecessary. However, I find the syntax for this a bit abstruce.
So, I'm jotting down a few examples here to reference later.

## Using a struct to implement an interface

This is how interfaces are typically implemented. Here, we'll satisfy the `io.Writer`
interface to create a writer that logs some stats before saving data to an in-memory buffer.

The standard library defines `io.Writer` like this:

```go
type Writer interface {
    Write(p []byte) (n int, err error)
}
```

We can implement `io.Writer` by defining a struct type, `LoggingWriter`, and attaching a
`Write` method with the required signature:

```go
// LoggingWriter writes data to an underlying writer and logs stats.
type LoggingWriter struct {
    w io.Writer
}

func (lw *LoggingWriter) Write(data []byte) (int, error) {
    fmt.Printf("LoggingWriter: Writing %d bytes\n", len(data))
    return lw.w.Write(data)
}
```

Here's how to use it:

```go
func main() {
    var buf bytes.Buffer
    logWriter := &LoggingWriter{w: &buf}

    _, err := logWriter.Write([]byte("Hello, world!"))
    if err != nil {
        fmt.Println("Error writing data:", err)
        return
    }

    fmt.Println("Buffer content:", buf.String())
}
```

Running this will log the stats before writing to the buffer:

```txt
LoggingWriter: Writing 13 bytes
Buffer content: Hello, world!
```

## Using a function type instead

Instead of defining the `LoggingWriter` struct, you can use a function type to satisfy
`io.Writer`. This works well for SMIs but doesn't make sense for interfaces with multiple
methods. In those cases, we need to resort back to the methods-on-struct approach.

Here's how it looks:

```go
// WriteFunc is a function type that implements io.Writer.
type WriteFunc func(data []byte) (int, error)

// Write makes WriteFunc satisfy io.Writer.
func (wf WriteFunc) Write(data []byte) (int, error) {
    return wf(data)
}
```

You can use `WriteFunc` like this:

```go
func main() {
    var buf bytes.Buffer

    // Define a WriteFunc to log stats and write data.
    logWriter := WriteFunc(func(data []byte) (int, error) {
        fmt.Printf("WriteFunc: Writing %d bytes\n", len(data))
        return buf.Write(data)
    })

    _, err := logWriter.Write([]byte("Hello, world!"))
    if err != nil {
        fmt.Println("Error writing data:", err)
        return
    }

    fmt.Println("Buffer content:", buf.String())
}
```

`WriteFunc` satisfies `io.Writer` by defining a `Write` method with the expected signature.
You can adapt any function to match the signature `(data []byte) (int, error)` using
`WriteFunc`, so there's no need for a struct when no state is involved.

In `main`, an anonymous function logs the number of bytes and writes the data to a buffer.
Wrapping this function with `WriteFunc` lets it implement the `io.Writer` interface. The
`.Write` method is called on the wrapped function to log stats and write data to the buffer.
Finally, the buffer's content is printed to verify everything worked.

> _For a simple example like this, using a function type to implement an interface might
> feel like overkill. But there are cases where it simplifies things. The next sections
> explore real-world examples where function types make interface implementation a bit more
> ergonomic._

## Mocking interfaces for testing

Function types let you mock interfaces without creating dedicated structs. Here's how it
works with an `Authenticator` interface:

```go
type Authenticator interface {
    Authenticate(username, password string) (bool, error)
}

type AuthFunc func(username, password string) (bool, error)

func (af AuthFunc) Authenticate(username, password string) (bool, error) {
    return af(username, password)
}
```

The `AuthFunc` type implements the `Authenticate` method by calling itself with the provided
arguments. This lets you create mock implementations inline in your tests.

Here's how to use it in a test:

```go
func TestLogin(t *testing.T) {
    mockAuth := AuthFunc(func(u, p string) (bool, error) {
        fmt.Printf("MockAuth called with username=%s, password=%s\n", u, p)
        return true, nil
    })

    success, err := PerformLogin("john_doe", "secret", mockAuth)
    if err != nil || !success {
        t.Fatalf("Authentication failed")
    }
}
```

And in application code:

```go
func main() {
    auth := AuthFunc(func(u, p string) (bool, error) {
        return u == "admin" && p == "password123", nil
    })

    if success, _ := auth.Authenticate("admin", "password123"); success {
        fmt.Println("Authentication successful!")
    }
}
```

## Building HTTP middlewares

The standard library's `http.HandlerFunc` demonstrates function types in action. Here's how
to build a logging middleware that times requests:

```go
type Handler interface {
    ServeHTTP(http.ResponseWriter, *http.Request)
}

func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        fmt.Printf("Started %s %s\n", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
        fmt.Printf("Completed %s in %v\n", r.URL.Path, time.Since(start))
    })
}
```

`http.HandlerFunc` converts functions into HTTP handlers. The logging middleware wraps the
next handler and adds timing and logging.

We use it as follows:

```go
func main() {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello, World!")
    })

    http.Handle("/", LoggingMiddleware(handler))
    http.ListenAndServe(":8080", nil)
}
```

## Adapting function types for database queries

Function types can abstract database query execution for testing or supporting different
database implementations:

```go
type QueryExecutor interface {
    Execute(query string, args ...any) (Result, error)
}

type QueryFunc func(query string, args ...any) (Result, error)

func (qf QueryFunc) Execute(query string, args ...any) (Result, error) {
    return qf(query, args...)
}
```

`QueryFunc` turns regular functions into `QueryExecutor` implementations, making it easy to
swap implementations or create mocks.

This is how to use it:

```go
func main() {
    executor := QueryFunc(func(query string, args ...any) (Result, error) {
        fmt.Printf("Executing query: %s with args: %v\n", query, args)
        return Result{RowsAffected: 1}, nil
    })

    result, _ := executor.Execute("SELECT * FROM users WHERE id = ?", 1)
    fmt.Printf("Rows affected: %d\n", result.RowsAffected)
}
```

## Implementing retry logic

Function types can encapsulate retry behavior without creating configuration structs:

```go
type Retryer interface {
    Retry(fn func() error) error
}

type RetryFunc func(fn func() error) error

func (rf RetryFunc) Retry(fn func() error) error {
    return rf(fn)
}
```

`RetryFunc` converts functions with the matching signature into a `Retryer`, letting you
swap retry strategies or create test versions.

We use it as such:

```go
func main() {
    retry := RetryFunc(func(fn func() error) error {
        for i := 0; i < 3; i++ {
            if err := fn(); err == nil {
                return nil
            }
            time.Sleep(time.Second * time.Duration(i+1))
        }
        return fmt.Errorf("operation failed after 3 retries")
    })

    err := retry.Retry(func() error {
        return nil // Your operation here
    })

    if err != nil {
        fmt.Printf("Failed to execute operation: %v\n", err)
    }
}
```

Go lets us define methods on custom types, including function types. While this can be handy
for adapting a function type to an interface, it can make the code hard to read at times. So
I don't always reach for it. It's perfectly fine to define an empty struct with a single
method if that makes the code more readable. Nonetheless, it's a neat trick to keep in your
repertoire.

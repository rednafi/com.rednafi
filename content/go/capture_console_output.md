---
title: Capturing console output in Go tests
date: 2025-04-12
tags:
    - Go
    - Testing
    - TIL
---

Ideally, every function that writes to the stdout probably should ask for a `io.Writer` and
write to it instead. However, it's common to encounter functions like this:

```go
func frobnicate() {
    fmt.Println("do something")
}
```

This would be easier to test if `frobnicate` would ask for a writer to write to. For
instance:

```go
func frobnicate(w io.Writer) {
    fmt.Fprintln(w, "do something")
}
```

You could pass `os.Stdout` to `frobnicate` explicitly to write to the console:

```go
func main() {
    frobnicate(os.Stdout)
}
```

This behaves exactly the same way as the first version of `frobnicate`.

During test, instead of `os.Stdout`, you'd just pass a `bytes.Buffer` and assert its content
as follows:

```go
func TestFrobnicate(t *testing.T) {
    // Create a buffer to capture the output
    var buf bytes.Buffer

    // Call the function with the buffer
    frobnicate(&buf)

    // Check if the output is as expected
    expected := "do something\n"
    if buf.String() != expected {
        t.Errorf("Expected %q, got %q", expected, buf.String())
    }
}
```

This is all good. But many functions or methods that emit logs just do that directly to
stdout. So we want to test the first version of `frobnicate` without making any changes to
it.

I found this neat pattern to test functions that write to stdout without accepting a writer.

The idea is to write a helper function named `captureStdout` that looks like this:

```go
// captureStdout replaces os.Stdout with a buffer and returns the captured output.
func captureStdout(f func()) string {
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    f() // run the function that writes to stdout

    _ = w.Close()
    var buf bytes.Buffer
    _, _ = io.Copy(&buf, r)
    os.Stdout = old

    return buf.String()
}
```

Here's what's happening under the hood:

We use `os.Pipe()` to create a pipe: a connected pair of file descriptors—a reader (`r`) and
a writer (`w`). Think of it like a temporary tunnel. Whatever we write to `w`, we can read
back from `r`. Since both are just files as far as Go is concerned, we can temporarily
replace `os.Stdout` with the writer end of the pipe:

```go
os.Stdout = w
```

This means anything printed to stdout during the function run actually goes into our pipe.
After the function runs, we close the writer to signal that we're done writing, then read
from the reader into a buffer and restore the original stdout.

Now we can test `frobnicate` without touching its implementation:

```go
func TestFrobnicate(t *testing.T) {
    output := captureStdout(func() {
        frobnicate()
    })

    expected := "do something\n"
    if output != expected {
        t.Errorf("Expected %q, got %q", expected, output)
    }
}
```

No need to refactor `frobnicate`. This works great for quick tests when you don't control
the code or just want to assert some printed output.

## A more robust capture out

The above version of `captureStdout` works fine for simple cases. But in practice, functions
might also write to `stderr`, especially if they’re using Go's `log` package or if a panic
happens. For example, this would not be captured by the simple `captureStdout` helper:

```go
log.Println("something went wrong")
```

Even though it looks like a normal print statement, `log` writes to `stderr` by default. So
if you want to catch that output too, or generally capture everything that's printed to the
console during a function call, we need to upgrade our helper a bit. I found this example
from the immudb[^1] repo.

Here's a more complete version:

```go
// captureOut captures both stdout and stderr.
func captureOut(f func()) string {
    // Create a pipe to capture stdout
    custReader, custWriter, err := os.Pipe()
    if err != nil {
        panic(err)
    }

    // Save the original stdout and stderr to restore later
    origStdout := os.Stdout
    origStderr := os.Stderr

    // Restore stdout and stderr when done
    defer func() {
        os.Stdout = origStdout
        os.Stderr = origStderr
    }()

    // Set the stdout and stderr to the pipe
    os.Stdout, os.Stderr = custWriter, custWriter
    log.SetOutput(custWriter)

    // Create a channel to read the output from the pipe
    out := make(chan string)

    // Use a goroutine to read from the pipe and send the output to the channel
    var wg sync.WaitGroup
    wg.Add(1)
    go func() {
        var buf bytes.Buffer
        wg.Done()
        io.Copy(&buf, custReader)
        out <- buf.String()
    }()
    wg.Wait()

    // Call the function that writes to stdout
    f()

    // Close the writer to signal that we're done
    _ = custWriter.Close()

    // Wait for the goroutine to finish reading from the pipe
    return <-out
}
```

This version does a few more things:

- **Captures everything**: It redirects both `os.Stdout` and `os.Stderr` to ensure all
  standard output streams are captured. It also explicitly redirects the standard `log`
  package's output, which often bypasses `os.Stderr`.

- **Prevents deadlocks**: Output is read concurrently in a separate goroutine. This is
  crucial because if `f` generates more output than the internal pipe buffer can hold,
  writing would block without a concurrent reader, causing a deadlock.

- **Ensure reader readiness**: A `sync.WaitGroup` guarantees the reading goroutine is active
  before `f` starts executing. This prevents a potential race condition where initial output
  could be lost if `f` writes before the reader is ready.

- **Guarantees cleanup**: Using `defer`, the original `os.Stdout` and `os.Stderr` are always
  restored, even if `f` panics. This prevents the function from permanently altering the
  program's standard output streams.

You'd use `captureOut` the same way as the naive `captureStdout`. This version is safer and
more complete, and works well when you're testing CLI commands, log-heavy code, or anything
that might write to the terminal in unexpected ways.

It's not a replacement for writing functions that accept `io.Writer`, but when you're
dealing with existing code or want to quickly assert on terminal output, it gets the job
done.

[^1]:
    [Capture out](https://github.com/codenotary/immudb/blob/cf9a5d8b9b4d3784c6b9fa8c874902bf1318a6e8/cmd/immuclient/immuclienttest/helper.go#L143)

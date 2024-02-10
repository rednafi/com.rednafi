---
title: Anemic stack traces in Go
date: 2024-02-10
tags:
    - Go
---

While I like Go's approach of treating errors as values as much as the next person, it
inevitably leads to a situation where there isn't a one-size-fits-all strategy for error
handling like in Python or JavaScript.

The usual way of dealing with errors entails returning error values from the bottom of the
call chain and then handling them at the top. But it's not universal since there are cases
where you might want to handle errors as early as possible and fail catastrophically. Yet,
it's common enough that we can use it as the base of our conversation.

This simple but verbose error handling works okay and makes us painfully aware of all the
possible error paths. Yet, the model doesn't hold up as your program grows in scope and
complexity, forcing you to devise custom patterns to add context and build thin stack
traces. There's no avoiding that.

But the good thing is that building an emaciated stack trace is fairly straightforward, and
some of the patterns are quite portable. After reading Rob Pike's blog on error handling in
the upspin[^1] project, I had some ideas on creating custom errors to emulate stack traces.
I ended up spending a few hours this morning experimenting with some of the ideas in a more
limited scope.

Let's say we're building a file-copy service that will accept a `src` and `dst` path and
copy the contents from source to destination.

```go
func copyFile(src, dst string) error {
    // Open the source file for reading
    sourceFile, err := os.Open(src)
    if err != nil {
        return err
    }
    defer sourceFile.Close()

    // Create the destination file for writing
    destFile, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer destFile.Close()

    // Copy the contents from source to destination file
    _, err = io.Copy(destFile, sourceFile)
    if err != nil {
        return err
    }

    // Ensure that the destination file's content is successfully written
    err = destFile.Sync()
    if err != nil {
        return err
    }

    return nil
}
```

This typical error handling pattern involves returning error values from lower-level
functions and addressing them in top-level ones. Here, the `main` function manages the
error:

```go
func main() {
    // Define the source and destination file paths
    src := "path/to/source/file"
    dst := "path/to/destination/file"

    // Call copyFile and handle any errors
    err := copyFile(src, dst)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error copying file: %s\n", err)
        os.Exit(1)
    }

    fmt.Println("File copied successfully.")
}
```

Running this function gives us the following output:

```txt
Error copying file: open path/to/source/file: no such file or directory
exit status 1
```

This is usually enough if you're building a CLI or a small program. Also, squinting at the
error message gives us a hint that among the 4 error-return paths, the `copyFile` function
bailed at the first one when it couldn't find the source file.

A proper way to handle this in larger applications is to wrap the errors and provide them
with your own context. Then, in the top-level function, you can unwrap the error message or
just log it verbatim as before. So, `copyFile` can be rewritten as follows:

```go
func copyFile(src, dst string) error {
    sourceFile, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("cannot open source file: %w", err)
    }
    defer sourceFile.Close()

    destFile, err := os.Create(dst)
    if err != nil {
        return fmt.Errorf("cannot create destination file: %w", err)
    }
    defer destFile.Close()

    _, err = io.Copy(destFile, sourceFile)
    if err != nil {
        return fmt.Errorf("cannot copy file contents: %w", err)
    }

    err = destFile.Sync()
    if err != nil {
        return fmt.Errorf("cannot sync destination file: %w", err)
    }

    return nil
}
```

Notice how we're adding extra context to the error values with the `%w` verb in the
`fmt.Errorf` function.

If you keep the previous `main` function unchanged and run it, you'll get the following
output:

```go
Error copying file: cannot open source file: \
open path/to/source/file: no such file or directory
exit status 1
```

This time, since you know where you added the context, you also know which error-path the
`copyFile` function returned from. However, even in this case, the `main` function just
relays whatever comes out of `copyFile` and logs the error message.

How would you make the error message prettier without losing context? Also, how would you
attach file names and line numbers to make debugging easier?

We can define a custom error struct to represent a generic error in the package that houses
`copyFile`.

```go
type Error struct {
    Op       string
    Path     string
    LineNo   int
    FileName string
    Err      error
    Debug    bool
}

func (e *Error) Error() string {
    if e.Debug {
        return fmt.Sprintf(
            "%s: %s: %s\n\t%s:%d", e.Op, e.Path, e.Err, e.FileName, e.LineNo,
        )
    }
    msg := e.Err.Error()
    msgs := strings.Split(msg, ":")
    msg = strings.TrimSpace(msgs[len(msgs)-1])
    return fmt.Sprintf("%s: %s: \n\t%s", e.Op, e.Path, msg)
}
```

Inside the `Error` struct, `Op` represents the name of the function that the error
originates from, `Path` is the file path, `LineNo` and `FileName` denote the precise
location of the error, `Err` is the original error we're wrapping, and finally the `debug`
boolean is be used to control the verbosity of error messages.

Then the `Error()` method on the struct builds a either a rudimentary stack trace or a
prettier error message depending on the value of `debug` flag. The `Error` struct can be
constructed with the following constructor function:

```go
func NewError(op string, path string, err error, debug bool) *Error {
    _, file, line, ok := runtime.Caller(1)

    if !ok {
        file = "???"
        line = 0
    }

    return &Error{
        Op:       op,
        Path:     path,
        LineNo:   line,
        FileName: file,
        Err:      err,
        Debug:    debug,
    }
}
```

This uses the `runtime` package to add the location data of the caller. This will be called
in the `copyFile` function as follows:

```go
func copyFile(src, dst string, debug bool) error {
    // Open the source file for reading.
    sourceFile, err := os.Open(src)
    if err != nil {
        return NewError("os.Open", src, err, debug)
    }
    defer sourceFile.Close()

    // Create the destination file for writing.
    destFile, err := os.Create(dst)
    if err != nil {
        return NewError("os.Create", dst, err, debug)
    }
    defer destFile.Close()

    // Copy the contents from source to destination file.
    _, err = io.Copy(destFile, sourceFile)
    if err != nil {
        return NewError("io.Copy", dst, err, debug)
    }

    // Ensure that the destination file's content is successfully written.
    err = destFile.Sync()
    if err != nil {
        return NewError("destFile.Sync", dst, err, debug)
    }

    return nil
}
```

You can turn on the `debug` flag to print the stack trace in the `main` function:

```txt
os.Open: /path/to/source/file: open /path/to/source/file: no such file or directory
        /Users/rednafi/canvas/rednafi.com/main.go:54
exit status 1
```

Toggling `debug` to `false` and running the snippet will return:

```txt
os.Open: /path/to/source/file:
        no such file or directory
exit status 1
```

You can add even more context to this error in different calling locations like this:

```go
sourceFile, err := os.Open(src)
    if err != nil {
        return fmt.Errorf(
            "more context: %w", NewError("os.Open", src, err, debug),
        )
    }
```

It'll be pretty-printed like this when `debug` is `false`:

```fmt
more ctx: os.Open: /path/to/source/file:
        no such file or directory
exit status 1
```

Now depending on your needs, you can customize the `Error` struct and `NewError` constructor
to enable more elaborate error tracing.

However, this isn't a proper stack in the sense that it only unwinds errors one level deep.
But it can be extended to recursively build the full error trace if needed. The Upspin[^2]
repo demonstrates a few techniques on how to do so. But for this particular case, one level
deep stack is borderline overkill.

Here's the complete working example[^2].

Fin!

[^1]:
    [Error handling in Upspin](https://commandcenter.blogspot.com/2017/12/error-handling-in-upspin.html)

[^2]:
    [Upspin's error package](https://github.com/upspin/upspin/blob/master/errors/errors.go)

[^2]: [Complete example](https://gist.github.com/rednafi/d090a16ba6ddd19c7fe8bdaae746205c)

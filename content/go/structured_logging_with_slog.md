---
title: Go structured logging with slog
date: 2023-08-10
tags:
    - Go
    - TIL
---

Before the release of version 1.21, you couldn't set levels for your log messages in Go
without either using third-party libraries or writing your own boilerplates. Coming from
Python, I've always found this odd, considering that this capability has been in the Python
standard library forever. However, it seems like the new `log/slog` subpackage in Go allows
you to do that and a whole lot more.

Apart from being able to add levels to log messages, `slog` also allows you to emit
JSON-structured log messages and group them by certain attributes. The ability to do all
this in-house is quite neat and I wanted to take it for a spin. The official [documentation]
on this is on the terser side but still comprehensive. So, here, instead of repeating the
same information, I wanted to write something for me that mainly highlights the most common
cases.

## Kickoff

Here's how you'd add levels to your log messages:

```go
package main

import (
    "log/slog"
)

func main() {
    slog.Debug("a debug message")
    slog.Info("an info message")
    slog.Warn("a warning message")
    slog.Error("an error message")
}
```

Running this will print the following output.

```txt
2023/08/10 17:10:11 INFO an info message
2023/08/10 17:10:11 WARN a warning message
2023/08/10 17:10:11 ERROR an error message
```

Notice how the concomitant local time and level are prepended to each log message. Also,
observe that the `DEBUG` message is missing there. That's because the default log handler
will only print messages if the log level is `INFO` or higher. We'll see how we can set
custom log levels shortly. But before that here's a quick overview of how the different
components of `slog` work together.

## Machineries

The `slog` package lets you create `Logger` instances. These instances have methods like
`Info()` and `Error()` that you can call to log stuff. When you call one of these methods,
it creates a `Record` from the data you passed in and sends it to a `Handler`. The `Handler`
figures out what to actually do with the log—like print it somewhere or send it over the
network. You can write your own or use one of the predefined `TextHandler` or `JSONHandler`
to format your log output.

There's a default `Logger` you can use right away with functions like `Info()` and `Error()`
at the top level. Underneath, the `Info()` function calls the `Logger.Info()` method. This
means you don't need to create a `Logger` instance by hand just to start logging. You've
already seen how we can use these top-level functions to send different levels of logs to
the stdout.

Each log entry has an associated severity level which is represented by an integer. The
more severe the log level is, the higher the value of the integer will be. The default
logger only emits `LevelInfo` or higher levels of log messages. Predefined levels have the
following values:

```go
const (
    LevelDebug Level = -4
    LevelInfo  Level = 0
    LevelWarn  Level = 4
    LevelError Level = 8
)
```

## Using custom log handlers

You can use predefined custom handlers to change the format of your log output. The
following snippet creates a new `Logger` instance from a `TextHandler` instance and then
uses that to print log messages to the stdout:

```go
// Define a new TextHandler
h := slog.NewTextHandler(os.Stdout, nil)

// Update the default Logger to use the new handler
slog.SetDefault(slog.New(h))

// Use the logger as usual
slog.Info("an info message")
slog.Warn("a warning message")
```

Running this prints:

```txt
time=2023-08-10T23:57:39.914-04:00 level=INFO msg="an info message"
time=2023-08-10T23:57:39.915-04:00 level=WARN msg="a warning message"
```

The `NewTextHandler` function has two arguments: the first one takes in a type that
implements the `io.Writer` interface and the second one accepts a `HandlerOptions` struct.
The `HandlerOptions` struct can be used to customize the output format. We can pass `nil`
for this value if we don't need to change the handler's default output format.

We're passing `os.Stdout` as the first argument to direct the log messages to stdout and
`nil` as the second argument. The `NewTextHandler` returns a `*slog.TextHandler` struct
pointer which is passed to `slog.New` to get a new `Logger` instance. Then we set this newly
created `Logger` as the default one via the `slog.SetDefault()` function. Finally, the
updated logger is used to print an info and a warning message. Notice how the `TextHandler`
output records are constituted as key-value attribute pairs.

## Printing log messages in JSON format

Similar to `NewTextHandler`, `NewJSONHandler` can be used to create a `JSONHandler`, which
prints the log records as JSON objects:

```go
// Define a new TextHandler
h := slog.NewJSONHandler(os.Stdout, nil)

// Update the default Logger to use the new handler
slog.SetDefault(slog.New(h))

// Use the logger as usual
slog.Info("an info")
slog.Warn("a warning")
```

This prints:

```txt
{"time":"2023-08-11T00:13:44.734365-04:00","level":"INFO","msg":"an info"}
{"time":"2023-08-11T00:13:44.734505-04:00","level":"WARN","msg":"a warning"}
```

## Changing log levels

You've already seen that the default logger only prints log messages of level `Info` and
up. We'll need to define a custom log handler to change the default log level. Here's an
example that enables printing `Debug` messages:

```go
var programLevel = new(slog.LevelVar) // Info by default
h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel})

slog.SetDefault(slog.New(h))
programLevel.Set(slog.LevelDebug) // Update log level to Debug

slog.Debug("a debug message")
slog.Info("an info message")
```

It'll print:

```txt
time=2023-08-10T23:53:16.654-04:00 level=DEBUG msg="a debug message"
time=2023-08-10T23:53:16.654-04:00 level=INFO msg="an info message"
```

First, we create an instance of `slog.LevelVar` with the `new` allocator. Next, we create a
`TextHandler` instance and the `programLevel` to the `slog.HandlerOptions` struct pointer.
Then we create a new `Logger` instance as before and set that as the default logger. In the
last step, the `programLevel` is updated so that it signals the handler to allow emitting
`Debug` messages.

## Defining custom log levels

Apart from `Debug`, `Info`, `Warn`, and `Error`, you can define your own custom log levels.
Here's an example of doing that with the default `Logger` instance:

```go
// Defining a few custom levels
const (
    logMeh = slog.Level(2)
    logFatal = slog.Level(13)
)

// Getting the default logger
logger := slog.Default()

// Use the Log method on the logger and pass the log level
logger.Log(nil, logMeh, "a meh message")
logger.Log(nil, logFatal, "a fatal message")
```

This will return:

```txt
2023/08/11 00:45:35 INFO+2 a meh message
2023/08/11 00:45:35 ERROR+5 a fatal message
```

Observe that you'll have to use `Logger.Log()` to pass your custom log level. Another
example with a custom log handler:

```go
// Defining a custom log level
const logPanic = slog.Level(15)

// Setting up a TextHandler
h := slog.NewTextHandler(os.Stderr, nil)

// Setting up a logger that uses the TextHandler
logger := slog.New(h)

// Use the Log method on the logger and pass the log level
logger.Log(nil, logPanic, "a panic message")
```

This prints:

```txt
time=2023-08-11T00:52:08.903-04:00 level=ERROR+7 msg="a panic message"
```

## Adding or removing log attributes

Log attributes are just key-value pairs. The following example appends a new key and a value
to the log message:

```go
slog.Info("an info message", "new_key", "new_value")
```

```txt
2023/08/11 01:10:18 INFO an info message new_key=new_value
```

To remove attributes from log records, you'll need to configure your custom handler and
create a logger instance from that:

```go
ReplaceAttr := func(group []string, a slog.Attr) slog.Attr {
    if a.Key == "time" {
        return slog.Attr{}
    }
    return slog.Attr{Key: a.Key, Value: a.Value}
}

// Before removing the time attribute
h1 := slog.NewJSONHandler(os.Stdout, nil)
slog.SetDefault(slog.New(h1))

slog.Info("an info message")

// After removing the time attribute
h2 := slog.NewJSONHandler(
    os.Stdout, &slog.HandlerOptions{ReplaceAttr: ReplaceAttr},
)
slog.SetDefault(slog.New(h2))

slog.Info("an info message")
```

Running this will print the following. The `time` key no longer exists on the second
log record:

```txt
{
  "time":"2023-08-11T01:23:58.936984-04:00",
  "level":"INFO",
  "msg":"an info message"
}
{"level":"INFO","msg":"an info message"}
```

The main focus here is the `ReplaceAttr` function which is used to transform or remove
attributes before they are processed by a handler. It accepts two arguments: a slice of
group names and an `Attr` struct. The group name allows attributes to be qualified into
different scopes, which we won't use right now. The `Attr` contains the `Key` and `Value` of
the attribute that's being logged.

In this case, `ReplaceAttr` checks if the attribute key is `time` and if so, returns an
empty `Attr` struct, effectively signaling the handler not to include that attribute. If the
key is not `time`, it returns the original `Attr` unchanged.

## Adding sticky attributes

Sometimes you want to have a few common attributes that should persist across multiple log
calls. This can be done via `Logger.With()` method:

```go
// Make the attributes sticky with Logger.With method
logger := slog.Default().With("sticky_key" , "sticky_value")

// Look how we don't need to repeat sticky_key and sticky_value here
logger.Info("an info message")
logger.Error("an error message")
```

It prints:

```txt
2023/08/11 01:56:44 INFO an info message sticky_key=sticky_value
2023/08/11 01:56:44 ERROR an error message sticky_key=sticky_value
```

The `Logger.With()` method accepts key-value pairs of attributes. This saves you from
passing the same attributes over and over again to make them persist across multiple log
calls.

## Grouping log attributes

You can group the log attributes for better organization. Adding a group makes the attribute
keys of a log record qualified by the group name. What *qualify* means here can vary
depending on whether you're using a `TextHandler` or a `JSONHandler`. Here's an example that
demonstrates both:

```go
// The first string is the group name and the remaining
// strings are key-value attribute pairs
group := slog.Group("group_a", "key_a", "value_a")

// Grouping for default Logger
slog.Info("info message", group)

// For TextHandler
textHandler := slog.NewTextHandler(os.Stdout, nil)
textLogger := slog.New(textHandler)
textLogger.Warning("warning message", group)

// For JSONHandler
jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
jsonLogger := slog.New(jsonHandler)
jsonLogger.Error("error message", group)
```

This prints:

```txt
2023/08/11 16:41:12 INFO info message group_a.key_a=value_a

time=2023-08-11T16:41:12.072-04:00 level=WARN msg="warning message" \
    group_a.key_a=value_a

{
  "time": "2023-08-11T16:41:12.072635-04:00",
  "level": "ERROR",
  "msg": "error message",
  "group_a": {
    "key_a": "value_a"
  }
}
```

Here, in the case of the text logger, the log attribute key is qualified by the group name
as `group_a.key_a`. On the other hand, the JSON logger emits the log record in a way where
the group name `group_a` is used as the key of a nested object containing the
`{"key_a": "value_a"}` log attributes.

## Making log groups sticky

Akin to attributes, you can also make attribute group sticky with the `Logger.WithGroup()`
method:

```go
// Default logger
logger := slog.Default().WithGroup("group_a")
logger.Info("info message", "key_b", "value_b")

// Text logger
textHandler := slog.NewTextHandler(os.Stdout, nil)
textLogger := slog.New(textHandler).WithGroup("group_a")
textLogger.Info("info message", "key_b", "value_b")

// JSON logger
jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
jsonLogger := slog.New(jsonHandler).WithGroup("group_a")
jsonLogger.Info("info message", "key_b", "value_b")
```

This returns:

```txt
2023/08/11 16:11:30 INFO info message group_a.key_b=value_b

time=2023-08-11T16:11:30.913-04:00 level=INFO msg="info message" \
    group_a.key_b=value_b

{
  "time": "2023-08-11T16:11:30.913892-04:00",
  "level": "INFO",
  "msg": "info message",
  "group_a": {
    "key_b": "value_b"
  }
}
```

## Directing logs to different sinks

The predefined `TextHandler` and `JSONHandler` takes in a type that implements the
`io.Writer` interface as the first argument. We can leverage this aspect to change the
destination of a structured logger. The following example shows how you can direct the
structured log stream to both stdout and a file:

```go
type TeeWriter struct {
    stdout *os.File
    file   *os.File
}

func (t *TeeWriter) Write(p []byte) (n int, err error) {
    n, err = t.stdout.Write(p)
    if err != nil {
        return n, err
    }
    n, err = t.file.Write(p)
    return n, err
}

func main() {
    file, _ := os.Create("output.txt")
    writer := &TeeWriter{
        stdout: os.Stdout,
        file:   file,
    }
    h := slog.NewTextHandler(writer, nil)
    logger := slog.New(h)
    logger.Info("Hello, World!")
}
```

The `TeeWriter` struct associates stdout and a file handle. It implements a custom `Write`
method to write to both streams, enabling *teeing* of output. In `main()`, a `TeeWriter`
instance is created with stdout and a file. A pointer to `TeeWriter` is then passed to the
`TextHandler`. Next, the `TextHandler` is used to create a new `Logger`, so when the
`Logger` logs, the messages go through the `TextHandler`'s `TeeWriter` and are written to
both the console and a file via the custom `Write` method.

## Leveraging Attrs and Values for performance

When using a logger, you can pass in key-value pairs called Attrs instead of separate keys
and values. For example:

```go
slog.Info("info message", slog.Int("some_int", 7))
```

This is the same as:

```go
slog.Info("info message", "some_int", 7)
```

There are helper functions like `Int()`, `String()`, and `Bool()` to create Attrs for common
types. You can also use `Any()` to make an Attr for any type.

The real benefit is that Attrs are more efficient than separate keys and values. So for max
speed, we can use the `LogAttrs()` instead of `Log()`.

For example:

```go
logger.LogAttrs(nil, slog.LevelInfo, "info message", slog.Int("some int", 7))
```

This avoids extra allocations while giving the same result as:

```go
slog.Info("info message", "some int", 7)
```

## References

* [slog docs][documentation]

[documentation]: https://pkg.go.dev/log/slog

---
title: "Accepted proposal: Go examples with any signature"
slug: go-example-any-signature
date: 2026-07-28
description: >-
    Go examples with parameters or results can appear in documentation without becoming
    tests.
tags:
    - Go
    - Testing
    - API
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /shards/2026/07/go-example-any-signature/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mrp4m3jwi62t"
---

Go's [proposal to allow examples with any signature] was [accepted] on July 8. Today, an
example must look like `func ExampleXxx()`. [go/doc skips the function] as soon as it sees a
parameter or result. The example doesn't appear in `go doc` or on pkg.go.dev.

The restriction comes from the two jobs an example can do: appear in documentation and run
as a test. The test behavior starts when the function has a trailing `// Output:` comment.

The generated test harness can call `ExampleXxx()` because it needs no arguments and returns
nothing. `go test` captures the function's stdout, then compares that text with the comment.
Without an output comment, the example is compiled but isn't executed.

The zero-argument signature gets in the way when an API expects a value supplied by the
caller. For example, `testing/synctest.Test` takes a `*testing.T` as its first argument. An
example of a normal call needs the same parameter:

```go
func ExampleTest(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        start := time.Now()
        time.Sleep(time.Hour)
        if elapsed := time.Since(start); elapsed != time.Hour {
            t.Fatalf("time advanced by %v", elapsed)
        }
    })
}
```

Current tooling leaves that function out of the generated examples. The [accepted rules]
change that. `go doc` will put it alongside `synctest.Test` and show the complete
declaration, including the signature and any doc comment attached to the function.

The new rule also covers examples that return an error. They can use a regular Go signature
instead of calling `log.Fatal` or wrapping the body in an anonymous function:

```go
func ExampleCreateTemp() error {
    f, err := os.CreateTemp("", "example")
    if err != nil {
        return err
    }
    defer os.Remove(f.Name())

    if _, err := f.WriteString("content"); err != nil {
        f.Close()
        return err
    }
    return f.Close()
}
```

Examples with parameters or results are documentation only. They don't get a playground Run
button, and `go test` doesn't call them. They still have to compile with the rest of the
test package. Adding an output comment exposes the missing execution rules:

```go
func ExampleAdd(a, b int) int {
    return a + b
    // Output: 3
}
```

The harness doesn't know what to pass for `a` and `b`. It also can't compare the returned
`3` with the comment because the function writes nothing to stdout. A `*testing.T` parameter
has the same problem. The example runner has no `*testing.T` argument to pass.

> [!IMPORTANT]
>
> `// Output:` tells `go test` to run an example and compare its captured stdout with the
> comment. If the example has parameters or results, `go test` reports an error. The same
> rule applies to `// Unordered output:`.

The proposal doesn't define argument values or a meaning for returned values. Authors who
want regression coverage can call the example from a regular test. The test supplies the
missing context and decides what counts as success:

```go
func TestExampleCreateTemp(t *testing.T) {
    if err := ExampleCreateTemp(); err != nil {
        t.Fatal(err)
    }
}
```

When stdout is the behavior being shown, the example can keep the existing
`func ExampleXxx()` signature and put concrete inputs inside the body. Those examples keep
their current playground and output-checking behavior.

The [implementation] is under review. The proposal remains in the Go backlog with no target
release attached yet.

<!-- references -->
<!-- prettier-ignore-start -->

[proposal to allow examples with any signature]:
    https://github.com/golang/go/issues/79808

[accepted]:
    https://github.com/golang/go/issues/79808#issuecomment-4917832399

[go/doc skips the function]:
    https://github.com/golang/go/blob/go1.26.5/src/go/doc/example.go#L74-L80

[accepted rules]:
    https://github.com/golang/go/issues/79808#issuecomment-4733943866

[implementation]:
    https://go-review.googlesource.com/c/go/+/800000

<!-- prettier-ignore-end -->

---
title: Organizing Go tests
date: 2025-10-08
aliases:
    - /go/organizing-tests/
tags:
    - Go
    - Testing
---

When it comes to test organization, Go's standard `testing` library only gives you a few
options. I think that's a great thing because there are fewer details to remember and fewer
things to onboard people to. However, during code reviews, I often see people contravene a
few common conventions around test organization, especially those who are new to the
language.

If we distill the most common questions that come up when organizing tests, they are:

- Where to put the unit tests for a package
- How to enable [white-box] and [black-box] testing
- Where the [executable examples], [benchmarks], and [fuzz tests] should live
- Where the integration and end-to-end tests for a service should live

To answer these, let's consider a simple test subject.

## System under test (SUT)

Let's define a small app called `myapp` that contains a single package `mypkg`. It has a
`Greet` function that returns a greeting message as a string. We'll use this throughout the
discussion and evolve the directory structure as needed.

```txt
myapp/
└── mypkg/
    ├── greet.go
    └── greet_test.go
```

Here's how `greet.go` looks:

```go
// greet.go
package mypkg

func Greet(name string) string {
    if name == "" {
        return "Hello, stranger"
    }
    return "Hello, " + name
}
```

## In-package tests

Most Go tests live next to the code they verify. These are called _in-package tests_, and
they share the same package name as the code under test. This setup gives them access to
unexported functions and variables, making them ideal for unit tests that target specific
internal logic.

```go
// greet_test.go
package mypkg // The test file lives under `mypkg`

import "testing"

func TestGreet(t *testing.T) {
    got := Greet("Go") // The test can access mypkg deps without an import
    want := "Hello, Go"
    if got != want {
        t.Fatalf("Greet() = %q, want %q", got, want)
    }
}
```

The structure stays the same:

```txt
myapp/
└── mypkg/
    ├── greet.go         # under package mypkg
    └── greet_test.go    # under package mypkg
```

These are your bread-and-butter unit tests. You can run them with `go test ./...`, and
they'll have full access to unexported details in the package.

The [Go documentation] explains it as:

> _The test file can be in the same package as the one being tested. If the test file is in
> the same package, it may refer to unexported identifiers within the package._

This approach is called _white-box testing_. Your test code has full access to the package
internals, allowing you to test them directly when needed. For example, if there's an
unexported function in `greet.go`, the test in `greet_test.go` can call it directly.
Following the [test pyramid], most tests in your system should be written this way.

## Co-located external tests

Sometimes you want to verify that your package behaves correctly from the outside. At this
point, you're not concerned with its internals and just want to confirm that the public API
works as intended.

Go makes this possible by letting you write tests under a package name that ends with
`_test`. This creates a separate test package that lives alongside the package under test.
For example:

```go
// greet_external_test.go
package mypkg_test // Note the package definition

import (
    "testing"
    "myapp/mypkg" // Explicitly import the SUT package
)

func TestGreetingExternal(t *testing.T) {
    got := mypkg.Greet("External")
    want := "Hello, External"
    if got != want {
        t.Fatalf("unexpected output: got %q, want %q", got, want)
    }
}
```

Your directory now includes both internal and external tests:

```txt
myapp/
└── mypkg/
    ├── greet.go                 # under package mypkg
    ├── greet_test.go            # under package mypkg
    └── greet_external_test.go   # under package mypkg_test
```

In this setup, the `mypkg` directory can only contain the `mypkg` and `mypkg_test` packages.
The compiler recognizes the `_test` suffix and disallows any other package names in the same
directory.

A key detail is that the Go test harness doesn't build the tests of `mypkg_test` together
with those of `mypkg`. It compiles two separate test binaries: one containing the package
code and its in-package tests, and another containing the external tests. Each binary runs
independently, and the external one links against the compiled `mypkg` archive just like any
other importing package. You can find more about this process in the [Go documentation on
how tests are run].

This structure is particularly useful for validating public contracts and ensuring that
refactors don't break exported APIs.

As noted in the official testing package docs:

> _If the file is in a separate `_test` package, the package being tested must be imported
> explicitly, and only its exported identifiers may be used. This is known as “black-box"
> testing._

It's a neat way to test your package from the outside without moving your tests into a
separate directory tree. You can find examples of this style in [net/http], [context], and
[errors].

## Examples, benchmarks, and fuzz tests

Go's testing tool treats examples, benchmarks, and fuzz tests as first-class test functions.
They use the same `go test` command as your regular unit tests and usually live in the same
package. This makes them part of the same discovery and execution process but with different
entry points.

Here's how all three can coexist in the same package:

```go
// greet_test.go
package mypkg // same package as the unit tests

import (
    "fmt"
    "testing"
)

// ... other unit tests

func ExampleGreet() {
    fmt.Println(Greet("Alice"))
    // Output: Hello, Alice
}

func BenchmarkGreet(b *testing.B) {
    for b.Loop() {
        Greet("Go")
    }
}

func FuzzGreet(f *testing.F) {
    f.Add("Bob")
    f.Fuzz(func(t *testing.T, name string) {
        Greet(name)
    })
}
```

This setup doesn't change your layout:

```txt
myapp/
└── mypkg/
    ├── greet.go         # under package mypkg
    └── greet_test.go    # under package mypkg
```

If you prefer to separate these test types, you can move them into their own file while
keeping them in the same package:

```txt
myapp/
└── mypkg/
    ├── greet.go                        # under package mypkg
    ├── greet_test.go                   # under package mypkg
    └── greet_bench_fuzz_example.go     # under package mypkg
```

In this layout, `greet_bench_fuzz_example.go` houses the benchmarks, fuzz tests, and
examples, but all files still declare the same `package mypkg`. These are regular unit tests
with specialized entry points. See how packages like [encoding/json] or [html] organize
their fuzz tests.

It's not a strict rule to keep them in the same package. You can also put them in a `_test`
package. The [sort] package, for example, keeps its examples in `sort_test`.

As mentioned in the testing docs, benchmarks are discovered and executed with the `-bench`
flag, and fuzz tests with the `-fuzz` flag.

## Integration and end-to-end tests

When your project grows into multiple packages, you'll want to verify that everything works
together, not just in isolation. That's where integration and end-to-end tests come in. They
typically live outside the package tree because they often span multiple packages or
processes.

```txt
myapp/
├── mypkg/
│   ├── greet.go                # under package mypkg
│   └── greet_test.go           # under package mypkg
└── integration/
    └── greet_integration_test.go   # under package integration
```

Here's what one might look like:

```go
package integration

import (
    "testing"
    "myapp/mypkg" // Explicitly import the SUT pkg to use its deps
)

func TestGreetingFlow(t *testing.T) {
    got := mypkg.Greet("Integration")
    want := "Hello, Integration"
    if got != want {
        t.Fatalf("unexpected output: got %q, want %q", got, want)
    }
}
```

Integration tests import real packages and test their interactions. They can spin up
servers, connect to databases, or coordinate subsystems. The integration test packages are
just like any other package: to communicate with any other package, it needs to be imported
explicitly.

You'll see this pattern in [kubernetes], which has a `test` directory with subpackages like
`integration` and `e2e`.

## Closing

The general rule of thumb is:

- Unit tests stay in the same package as the code.
- Black-box tests use a `_test` package in the same directory.
- Examples, benchmarks, and fuzz tests live with the unit tests, though you may put them in
  `_test` if needed.
- Integration and end-to-end tests live outside the SUT package tree.

The following tree attempts to capture the full picture:

```txt
myapp/
├── mypkg/
│   ├── greet.go                     # package mypkg — production code
│   ├── greet_test.go                # package mypkg — unit tests, white-box tests
│   ├── greet_external_test.go       # package mypkg_test — black-box tests
│   └── greet_bench_fuzz_example.go  # package mypkg — examples, benchmarks, fuzz tests
└── integration/
    └── greet_integration_test.go    # package integration — integration or e2e tests
```


<!-- References -->

<!-- prettier-ignore-start -->

[white-box]:
    https://en.wikipedia.org/wiki/White-box_testing

[black-box]:
    https://en.wikipedia.org/wiki/Black-box_testing

[executable examples]:
    https://go.dev/blog/examples

[benchmarks]:
    https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go

[fuzz tests]:
    https://go.dev/doc/tutorial/fuzz

[go documentation]:
    https://pkg.go.dev/testing

[test pyramid]:
    https://martinfowler.com/articles/practical-test-pyramid.html

[go documentation on how tests are run]:
    https://pkg.go.dev/cmd/go#hdr-Test_packages

[net/http]:
    https://github.com/golang/go/tree/master/src/net/http

[context]:
    https://github.com/golang/go/tree/master/src/context

[errors]:
    https://github.com/golang/go/tree/master/src/errors

[encoding/json]:
    https://github.com/golang/go/tree/master/src/encoding/json

[html]:
    https://github.com/golang/go/tree/master/src/html

[sort]:
    https://github.com/golang/go/tree/master/src/sort

[kubernetes]:
    https://github.com/kubernetes/kubernetes/tree/master/test

<!-- prettier-ignore-end -->

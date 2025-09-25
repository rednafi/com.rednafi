---
title: Flags for discoverable test config in Go
date: 2025-06-28
slug: test-config-with-flags
aliases:
    - /go/test_config_with_flags/
tags:
    - Go
    - Testing
---

As your test suite grows, you need ways to toggle certain kinds of tests on or off. Maybe
you want to enable [snapshot tests], skip long-running [integration tests], or switch
between real services and mocks. In every case, you're really saying, "Run this test only if
_X_ is true."

So where does _X_ come from?

I like to rely on Go's standard tooling so that integration and snapshot tests can live
right beside ordinary unit tests. Because I usually run these heavier tests in
[testcontainers], I don't always want them running while I'm iterating on a feature or
chasing a bug. So I need to enable them in an optional manner.

To fetch the _X_ and conditionally run some tests, you'll typically see three approaches:

1. **Build tags** – place integration or snapshot tests in files guarded by build tags, so
   they're compiled only when you include the tag.
2. **Environment variables** – have each test look for an environment variable (e.g.,
   `RUN_INTEGRATION=1`) and skip itself if it's absent.
3. **Custom `go test` flags** (my preferred approach) – define your own flags so you can
   run, for example, `go test -run Integration -integration`.

## Build tags are hard to discover

Build tags are special comments you place at the top of a `.go` file to tell Go to include
that file only when certain tags are set during the build. This is how they typically look:

```go
//go:build snapshot

package main

import "testing"

func TestSnapshot(t *testing.T) {
    t.Log("running snapshot")
}
```

This file will only be compiled and included when you run:

```sh
go test -tags=snapshot
```

If you don't pass the tag, the file is skipped entirely during the build. Go won't even see
the test.

The upside is that it gives you a clean separation. You can group slow tests or
environment-dependent tests into their own files. But the downsides add up quickly.

First, there's no way to discover which tags are used without grepping through the codebase.
Go itself won't tell you. `go help test` doesn't mention them. There's no built-in list or
summary. You need to solely depend on documentation.

Second, build tags are applied per file, not per package. That means if even one test in a
file is guarded by a tag, the entire file is excluded unless the tag is passed. This makes
it difficult to mix optional and always-on tests in the same file.

And third, once you have more than a couple of tags, managing them becomes guesswork. You
end up running things like:

```bash
go test -tags=slow,mock,external
```

But you no longer remember what each one does or what combinations are safe. There's no
validation. It gets messy fast.

## Envvars are a bit better

Environment variables let you control test behavior at runtime. You don't need to recompile
anything, and you can pass them inline when running tests.

Here's a typical example:

```go
import "os"

func TestSnapshot(t *testing.T) {
    if os.Getenv("SNAPSHOT") != "1" {
        t.Skip("set SNAPSHOT=1 to run this test")
    }

    t.Log("running snapshot")
}
```

You run it like:

```bash
SNAPSHOT=1 go test -v
```

This is more dynamic than build tags. You don't have to split tests into separate files, and
you don't have to rebuild with special flags. More importantly, the test itself can detect
when the environment variable is missing and tell you what to do. It can skip itself and
print a message like "set SNAPSHOT=1 to run this test." That feedback loop is helpful.

But the discovery problem remains. There's no built-in way to ask, "what environment
variables does this test suite support?" You still have to read the code to find out.

It can get worse if the check is buried deep in a helper. Maybe some setup logic does:

```go
if os.Getenv("SNAPSHOT") == "1" {
    useRealService()
}
```

Now the test runs, but the behavior changes silently based on the environment. Nothing in
the test output tells you that the envvar was involved. You may not even realize that you're
running in a different mode.

And just like with build tags, there's no central registry. No docs or summary. You can only
hope someone left a good comment or wrote it down somewhere.

## Custom flags are almost always better

The cleanest and most discoverable way to control optional test behavior in Go is by
defining your own test flags. They're typed, explicit, and work well with Go’s built-in
tooling. Instead of toggling tests with magic file-level build tags or invisible environment
variables, you can wire up test configuration using the `flag` package, just like any other
Go binary.

There are two common approaches for defining test flags:

- Package-level flags via `TestMain`
- Per-file flags via `init()`.

Both approaches register the flag in the global flag set, so every test in the package can
see the value once parsing has happened. The trade-off is indirection versus locality:
`TestMain` centralizes all flags in one place, while file-level `init()` keeps each flag
next to the code that cares about it.

Here's how it looks with `TestMain`:

```go
package snapshot_test

import (
    "flag"
    "os"
    "testing"
)

var snapshot = flag.Bool("snapshot", false, "run snapshot tests")

func TestMain(m *testing.M) {
    flag.Parse()
    os.Exit(m.Run())
}

func TestSnapshot(t *testing.T) {
    if !*snapshot {
    	t.Skip("pass -snapshot to run this test")
    }
    t.Log("running snapshot")
}
```

And here's the equivalent using `init()` to keep everything in the same file:

```go
package snapshot_test

import (
    "flag"
    "testing"
)

var snapshot bool

func init() {
    flag.BoolVar(&snapshot, "snapshot", false, "run snapshot tests")
}

func TestSnapshot(t *testing.T) {
    if !snapshot {
    	t.Skip("pass -snapshot to run this test")
    }
    t.Log("running snapshot")
}
```

Once you've defined a flag, you run the snapshot tests like this:

```sh
go test -v -snapshot
```

You can also list all the flags using:

```sh
go test -v -args -h
```

This prints all registered flags, including your own:

```txt
  -snapshot
        run snapshot tests
  -test.v
        verbose: print all tests as they are run.
  -test.run
        run only those tests and examples matching the regular expression.
   # ...
```

A detail about names: built-in flags show up in the help output with a `test.` prefix
(`-test.v`, `-test.run`, `-test.timeout`), yet you pass them without that prefix (`-v`,
`-run`, `-timeout`) while running tests. The Go tool strips `test.` for you. Custom flags
don't get this treatment. Whatever string you register is the exact string you must pass. If
you register `snapshot` you run:

```bash
go test -snapshot
```

If you register `test.snapshot` you must run:

```bash
go test -test.snapshot
```

There is no automatic collapsing just because the name starts with `test.`.

The flag `-args` lets you pass additional arguments to the test binary. When the binary sees
`-h` after `-args`, it prints every flag and exits. No tests run, though the binary is
built. That one command exposes the full configuration surface of your tests.

If you namespace your flags like this:

```go
flag.BoolVar(&snapshot, "custom.snapshot", false, "run snapshot tests")
```

Then you can grep for them:

```bash
go test -v -args -h | grep custom
```

Define the global flags in `TestMain` when several files need the same switches or when you
have package-wide setup (containers, databases, global mocks). Define flags in `init()` when
a switch is relevant to one test file and you want the declaration right next to the logic
it controls. I usually prefer per-test- file-level flags that don't need to depend on any
global magic.

Either way, the flag lives in code, is easy to grep, appears in `-h`, and tells everyone
exactly what it controls. The only downside I can think of with this approach is that,
similar to the environment variable technique, you'll have to check for the flag in every
test and make a decision. But in practice, I prefer the flexibility over the all-or-nothing
approach with build tags.

---

I think flags are the best way to configure your apps and tools. Even when environment
variables are involved, I often map them to flags for documentation purposes. The goal is to
give users a single `-h` command they can run to see all available options for tuning
behavior. Tests are no exception. I was quite happy to find out that Peter Bourgon conveyed
the same sentiment in this [seminal 2018 blog post].

<!-- References -->

<!-- prettier-ignore-start -->

[snapshot tests]:
    https://www.reddit.com/r/golang/comments/yytw1f/snapshot_testing_in_golang/

[integration tests]:
    https://www.reddit.com/r/golang/comments/18xmkuz/how_do_you_write_integration_tests_in_go/

[testcontainers]:
    https://testcontainers.com/

<!-- go for industrial programming - peter bourgon -->
[seminal 2018 blog post]:
    https://peter.bourgon.org/go-for-industrial-programming/#program-configuration

<!-- prettier-ignore-end -->

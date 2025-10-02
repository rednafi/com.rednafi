---
title: Subtest grouping in Go
date: 2025-10-01
slug: subtest-grouping
tags:
    - Go
    - Testing
---

Go has [support for subtests] starting from version 1.7. With `t.Run`, you can nest tests,
assign names to cases, and let the runner execute work in parallel by calling `t.Parallel`
from subtests if needed.

For small suites, a flat set of `t.Run` calls is usually enough. That's where I tend to
begin. As the suite grows, your setup and teardown requirements may demand subtest grouping.
There are multiple ways to handle that.

One option is to group subtests using nested `t.Run`. However, since `t.Run` supports
arbitrary nesting, it's easy to create tests that are hard to read and reason about,
especially when each group has its own setup and teardown. When you add calls to
`t.Parallel`, it can also become unclear which groups of tests run sequentially and which
run in parallel.

This is all a bit hand wavy without examples. We'll start with the simplest possible subtest
grouping and work our way up. Coming up with examples that make the point while still
fitting in a blog is tricky, so you'll have to bear with my toy examples and use a bit of
imagination.

## System under test (SUT)

Let's say we're writing tests for a calculator that, for the sake of argument, can only do
addition and multiplication. Instead of going for table-driven tests, we'll split the tests
for addition and multiplication into two groups using subtests. The reason being, let's say
addition and multiplication need different kinds of setup and teardown for some reason.

I know I'm reaching, but bear with me. I'd rather make the point without dragging in mocks,
a real database, or [testcontainers] and getting lost in details. But you can find similar
setup in a real codebase everywhere where you might be talking to a database and your read
and write path have separate [lifecycles].

## Keep it flat until you can’t

If we didn't need different setup and teardown for the two groups, the simplest way to test
a system would be through a set of table-driven tests:

```go
func TestCalc(t *testing.T) {
    // Common setup and teardown

    tests := []struct {
        name string
        got  int
        want int
    }{
        {"1+1=2", 1 + 1, 2},
        {"2+3=5", 2 + 3, 5},
        {"2*2=4", 2 * 2, 4},
        {"3*3=9", 3 * 3, 9},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if tt.got != tt.want {
                t.Fatalf("got %d, want %d", tt.got, tt.want)
            }
        })
    }
}
```

Running the tests returns:

```txt
--- PASS: TestCalc (0.00s)
    --- PASS: TestCalc/1+1=2 (0.00s)
    --- PASS: TestCalc/2+3=5 (0.00s)
    --- PASS: TestCalc/2*2=4 (0.00s)
    --- PASS: TestCalc/3*3=9 (0.00s)
PASS
```

Unrolling the tests would give you this. The following is equivalent to the above test
suite:

```go
func TestCalc(t *testing.T) {
    // Common setup and teardown

    // Addition
    t.Run("1+1=2", func(t *testing.T) {
        if 1+1 != 2 {
            t.Fatal("want 2")
        }
    })

    t.Run("2+3=5", func(t *testing.T) {
        if 2+3 != 5 {
            t.Fatal("want 5")
        }
    })

    // Multiplication
    t.Run("2*2=4", func(t *testing.T) {
        if 2*2 != 4 {
            t.Fatal("want 4")
        }
    })

    t.Run("3*3=9", func(t *testing.T) {
        if 3*3 != 9 {
            t.Fatal("want 9")
        }
    })
}
```

Observe that all the subtests live at the same level. The names of the tests are the
indicator of which function of the calculator they're testing. But this obviously doesn't
allow us to have separate lifecycles for the addition and multiplication groups. There's no
grouping as of now.

## Group subtests with nested `t.Run` when lifecycle diverges

To allow different setup and teardown for addition and multiplication, we can introduce
grouping by nesting the subtests via `t.Run`. Notice:

```go
func TestCalc(t *testing.T) {
    // Common setup and teardown

    t.Run("addition", func(t *testing.T) {
        // addition-specific setup
        defer func() {
            // addition-specific teardown
        }()

        t.Run("1+1=2", func(t *testing.T) {
            if 1+1 != 2 {
                t.Fatal("want 2")
            }
        })

        t.Run("2+3=5", func(t *testing.T) {
            if 2+3 != 5 {
                t.Fatal("want 5")
            }
        })
    })

    t.Run("multiplication", func(t *testing.T) {
        // multiplication-specific setup
        defer func() {
            // multiplication-specific teardown
        }()

        t.Run("2*2=4", func(t *testing.T) {
            if 2*2 != 4 {
                t.Fatal("want 4")
            }
        })

        t.Run("3*3=9", func(t *testing.T) {
            if 3*3 != 9 {
                t.Fatal("want 9")
            }
        })
    })
}
```

In this case, you can run the common setup and teardown in the top-level test function and
the groups can have their own lifecycle operations alongside. Introducing the group also
allows us to name them properly and they show up when we run the tests:

```txt
--- PASS: TestCalc (0.00s)
    --- PASS: TestCalc/addition (0.00s)
        --- PASS: TestCalc/addition/1+1=2 (0.00s)
        --- PASS: TestCalc/addition/2+3=5 (0.00s)
    --- PASS: TestCalc/multiplication (0.00s)
        --- PASS: TestCalc/multiplication/2*2=4 (0.00s)
        --- PASS: TestCalc/multiplication/3*3=9 (0.00s)
PASS
```

From the output it's clear which subtests belong to which group. This setup also allows you
to run the groups in parallel by calling `t.Parallel` in each group.

```go
func TestCalc(t *testing.T) {
    // Common setup and teardown

    t.Run("addition", func(t *testing.T) {
        t.Parallel()
    })

    t.Run("multiplication", func(t *testing.T) {
        t.Parallel()
    })
}
```

Starting with flat subtests and nesting them one extra level with `t.Run` should suffice in
the majority of cases. Readability of your tests usually starts hurting when you need to
introduce any additional nesting.

I almost always frown when I encounter more than two degrees of nesting in a test suite. On
top of that, if your overly nested subtests start calling `t.Parallel` then it's quite
difficult to reason about the test execution flow. Plus, maintaining the lifecycles of the
nested subgroups can get out of hand pretty quickly.

But even when you're grouping subtests with two degrees of nesting, if the individual test
logic starts getting longer, that might start hurting readability. Named functions for the
subtests can help here in most cases.

## Extract subtest groups into functions

We can rewrite the subtest grouping example of the previous section by extracting subtests
into two group-specific functions like this:

```go
func TestCalc(t *testing.T) {
    // Common setup and teardown

    t.Run("addition", addgroup)
    t.Run("multiplication", multgroup)
}

func addgroup(t *testing.T) {
    // addition-specific setup
    defer func() {
        // addition-specific teardown
    }()

    t.Run("1+1=2", func(t *testing.T) {
        if 1+1 != 2 {
            t.Fatal("want 2")
        }
    })

    t.Run("2+3=5", func(t *testing.T) {
        if 2+3 != 5 {
            t.Fatal("want 5")
        }
    })
}

func multgroup(t *testing.T) {
    // multiplication-specific setup
    defer func() {
        // multiplication-specific teardown
    }()

    t.Run("2*2=4", func(t *testing.T) {
        if 2*2 != 4 {
            t.Fatal("want 4")
        }
    })

    t.Run("3*3=9", func(t *testing.T) {
        if 3*3 != 9 {
            t.Fatal("want 9")
        }
    })
}
```

All we did here is extract the groups into their own functions. Other than that this test is
identical to the previous two-degree subtest grouping. You can call `t.Parallel` from the
subgroup functions:

```go
func TestCalc(t *testing.T) {
    // Common setup and teardown
    // ...
}

func addgroup(t *testing.T) {
    // Run the group in parallel
    t.Parallel()
}

func multgroup(t *testing.T) {
    // Run the group in parallel
    t.Parallel()
}
```

Or you can bring the `t.Parallel` at the top-level test function:

```go
func TestCalc(t *testing.T) {
    // Common setup and teardown

    t.Run("addition", func(t *testing.T) {
        t.Parallel()
        addgroup(t)  // addgroup doesn't have t.Parallel
    })

    t.Run("multiplication", func(t *testing.T) {
        t.Parallel()
        multgroup(t) // multgroup doesn't have t.Parallel
    })
}
```

That's all there is to it. But some people don't like the manual wiring that we needed to do
in the top-level `TestCalc` function. Also, in a larger codebase, you'll need some
discipline to make sure the pattern is followed by others extending the code.

So often people want the subtest groups to be automatically discovered without them having
to manually wire them in the main test function. While I'm not a big fan of automagical
group discovery, I got curious about it nonetheless. The gRPC-go has a [group discovery
function] that does this.

## gRPC-go uses reflection to discover groups

If we were writing tests inside the grpc-go repository, we could lean on its small helper
package, `internal/grpctest`, which reflects over a value you pass in, discovers methods
whose names start with `Test`, and runs each of those as a subtest. Crucially, the helper
also runs setup before and teardown after each discovered test method, which gives you a
clear spot for per-group lifecycle work. The public surface is tiny: `RunSubTests(t, x)`
plus a default hook carrier `Tester` that you embed to get `Setup` and `Teardown`.

Here is our same calculator suite in that style, as if we were adding tests inside grpc-go:

```go
// NOTE: This import path only works inside the grpc-go repo family.
// External modules cannot import google.golang.org/grpc/internal/*.
package calc

import (
    "testing"
    "google.golang.org/grpc/internal/grpctest"
)

// CalcSuite: embed grpctest.Tester so we get Setup and Teardown hooks.
// The runner will discover TestAddition and TestMultiplication below.
type CalcSuite struct{ grpctest.Tester }

// TestAddition is discovered because the name starts with "Test".
func (CalcSuite) TestAddition(t *testing.T) {
    // addition-specific setup and teardown for this group
    defer func() {
        // tear down addition fixtures
    }()

    t.Run("1+1=2", func(t *testing.T) {
        if 1+1 != 2 {
            t.Fatal("want 2")
        }
    })
    t.Run("2+3=5", func(t *testing.T) {
        if 2+3 != 5 {
            t.Fatal("want 5")
        }
    })
}

// A second discovered group.
func (CalcSuite) TestMultiplication(t *testing.T) {
    // multiplication-specific setup and teardown for this group
    defer func() {
        // tear down multiplication fixtures
    }()

    t.Run("2*2=4", func(t *testing.T) {
        if 2*2 != 4 {
            t.Fatal("want 4")
        }
    })
    t.Run("3*3=9", func(t *testing.T) {
        // call t.Parallel() here if overlapping with other subtests is safe
        if 3*3 != 9 {
            t.Fatal("want 9")
        }
    })
}

// Top-level entry that "go test" sees.
// RunSubTests reflects over CalcSuite, then runs Setup, the test method, then Teardown.
func TestCalc(t *testing.T) {
    grpctest.RunSubTests(t, CalcSuite{})
}
```

Outside grpc-go you can't import `google.golang.org/grpc/internal/grpctest` because it lives
under an `internal/` path. Go's visibility rule only allows packages within that module tree
to use it. If you want the subtest discoverer, there's nothing stopping you from [blatantly
copying the code]. It's only a few dozen lines and devoid of any dependencies other than the
leak checker. You can drop the file in your tests, remove the leak checker code if you don't
need that, adjust the import paths, and start using `RunSubTests`. To avoid repetition, I'll
leave that as an exercise to the reader.

Another thing to point out is that `grpctest.RunSubTests` doesn't change the standard
scheduler; you still opt into concurrency with `t.Parallel()` where it is safe.

## Subgroup with third party libraries

If you like automatic subgroup discovery but want something you can use outside grpc-go, two
common options are [testify's suite] and [Bloomberg's go-testgroup]. Both let you organize
tests into named groups and keep per-group setup/teardown close to the cases.

### Testify's suite

Testify models a suite as a struct with `Test*` methods and gives you `s.Run` for subtests
and assertion helpers.

```go
package calc

import (
    "testing"
    "github.com/stretchr/testify/suite"
)

type CalcSuite struct{ suite.Suite }

func (s *CalcSuite) TestAddition() {
    s.Run("1+1=2", func() { s.Equal(2, 1+1) })
    s.Run("2+3=5", func() { s.Equal(5, 2+3) })
}

func (s *CalcSuite) TestMultiplication() {
    s.Run("2*2=4", func() { s.Equal(4, 2*2) })
    s.Run("3*3=9", func() { s.Equal(9, 3*3) })
}

func TestCalc(t *testing.T) {
    suite.Run(t, new(CalcSuite))
}
```

One limitation is that the [suite runner doesn't support using `t.Parallel`] to run the
suite methods (`TestAddition`, `TestMultiplication`) in parallel. Bloomberg's test group
allows you to do that.

### Bloomberg's go-testgroup

Bloomberg's library also groups by methods, but passes a `*testgroup.T` and provides two
runners so you can choose serial or parallel execution at the group level.

```go
package calc

import (
    "testing"
    "github.com/bloomberg/go-testgroup"
)

type CalcGroup struct{}

func (g *CalcGroup) Addition(t *testgroup.T) {
    t.Run("1+1=2", func(t *testgroup.T) { t.Equal(2, 1+1) })
    t.Run("2+3=5", func(t *testgroup.T) { t.Equal(5, 2+3) })
}

func (g *CalcGroup) Multiplication(t *testgroup.T) {
    t.Run("2*2=4", func(t *testgroup.T) { t.Equal(4, 2*2) })
    t.Run("3*3=9", func(t *testgroup.T) { t.Equal(9, 3*3) })
}

func TestCalcSerial(t *testing.T)   { testgroup.RunSerially(t, &CalcGroup{}) }

// Or run in parallel.
// Don't call t.Parallel inside methods
func TestCalcParallel(t *testing.T) { testgroup.RunInParallel(t, &CalcGroup{}) }
```

`RunInParallel` handles group-level parallelism for you and documents not to mix in your own
`t.Parallel` inside those methods.

## Closing

While there are multiple ways to organize subtest groups, I try to keep them flat for as
long as possible. When grouping becomes necessary, I gradually add a single extra level of
nesting with `t.Run`.

In larger tests, [extracting groups into their own named functions] improves readability and
maintainability quite a bit. I almost never use reflection-based wiring because that's one
extra bit of code to carry around.

I also tend to eschew pulling in third-party test suites unless I am already working in a
codebase that uses them. Tools like testify or go-testgroup require you to define a struct
and attach tests to it. I prefer to keep tests as standalone functions. In addition,
[testing frameworks often develop into mini-languages of their own], which makes onboarding
harder. Notice how different the APIs of testify suite and go-testgroup are despite doing
pretty much the same thing.

In my experience, even in large codebases, a bit of discipline is usually enough to get by
with manual subtest grouping.

<!-- References -->
<!-- prettier-ignore-start -->
[support for subtests]:
    https://go.dev/blog/subtests

[testcontainers]:
    https://golang.testcontainers.org/

[lifecycles]:
    /go/lifecycle-management-in-tests/

[group discovery function]:
    https://github.com/grpc/grpc-go/blob/d0ebcdffc75dc76f18966ab9cccafe6a949d6fb5/internal/grpctest/grpctest.go#L109

[blatantly copy the code]:
    https://github.com/grpc/grpc-go/blob/d0ebcdffc75dc76f18966ab9cccafe6a949d6fb5/internal/grpctest/grpctest.go

[testify's suite]:
    https://github.com/stretchr/testify

[suite runner doesn't support using `t.Parallel`]:
    https://github.com/stretchr/testify?tab=readme-ov-file#suite-package

[bloomberg's go-testgroup]:
    https://github.com/bloomberg/go-testgroup

[extracting the groups into their own named functions]:
    /go/subtest-grouping/#extract-subtest-groups-into-functions

[third party testing tools come with their own DSLs]:
    https://go.dev/doc/faq#testing_framework

<!-- prettier-ignore-end-->

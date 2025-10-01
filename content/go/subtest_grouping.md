---
title: Subtest grouping in Go
date: 2025-10-01
slug: subtest-grouping
tags:
    - Go
    - Testing
draft: true
---

Go has [support for subtests] since version 1.7. With `t.Run`, you can nest tests, assign
names to cases, and let the runner execute work in parallel by calling `t.Parallel` from
subtests if needed.

For small suites, a flat set of `t.Run` calls is usually enough and that is where I tend to
begin. As the suite grows, though, there are times when your setup and teardown requirements
demand subtest grouping and there are multiple ways of doing that.

One way is by grouping the subtests using nested `t.Run`. But the dilemma is, since `t.Run`
supports arbitrary nesting, it is easy to create tests that are difficult to read and reason
about, especially when each group has its own setup and teardown. When calls to `t.Parallel`
are added, it can also be unclear which groups of tests run sequentially and which run in
parallel.

This is all a bit hand wavy without examples. So we’ll start with the simplest possible
subtest grouping and work our way up. Coming up with examples that make the point and still
fit a blog is tricky, so you’ll have to bear with my toy examples here and use a bit of
imagination.

## System under test (SUT)

Let’s say we’re writing tests for a calculator that, for the sake of argument, can only do
addition and multiplication. Instead of going for table-driven tests, we’ll split the tests
for addition and multiplication into two groups using subtests. The reason being, let’s say
addition and multiplication need different kinds of setup and teardown for some reason. I
know I'm reaching, but bear with me. I’d rather make the point without dragging in a real
database and getting lost in details. But you can find similar setup in a real codebase
everywhere.

## Keep it flat until you can’t

If we didn't need different setup and teardown for the two groups, the simplest way to test
a system supra would be through a set of table driven tests:

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

Unrolling the tests would give you this. The following is equivalent of the above test
suite:

```go
func TestCalc(t *testing.T) {
    // Common setup and teardown

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

Observe that all the subtests live in the same level. The names of the tests are the
indicator which function of of the calculator they're testing. But this obviously doesn't
allow us to have separate lifecycle for the addition and multiplication groups. There's no
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

From the output it's clear which subtest belong to which group. This setup also allows you
run the groups in parallel by calling `t.Parallel` in each group.

```go
func TestCalc(t *testing.T) {
    // Common setup and teardown

    t.Run("addition", func(t *testing.T) {
		t.Parallel()
        // ... same as above
    })

    t.Run("multiplication", func(t *testing.T) {
		t.Parallel()
        // ... same as above
    })
}
```

Starting with flat subtests and nesting them one extra level with `t.Run` should suffice
majority of the cases. Readability of your tests usually starts hurting when you need to
introduce any additional nesting. I almost always frown when I encounter more than two
degrees of nesting in a test suite. If your overly nested subtests start calling
`t.Parallel` then it's quite difficult to reason about the test execution flow. Also
maintaining the lifecycles of the nested subgroups can get out of hand pretty quickly.

But even when you're grouping subtests with 2-degrees of nesting, if the individual test
logic starts getting longer, that might start hurting readability. Named functions for the
subtests can help here in most cases.

## Extract subtests into functions for clarity

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

All we did here is extracting the groups into their their own functions. Other than that this test is identical
to the previous 2-degree subtest grouping. You can call `t.Parallel` from the subgroup functions:

```go
func TestCalc(t *testing.T) {
    // Common setup and teardown
}

func addgroup(t *testing.T) {
    t.Parallel()
    // ... same as before
}

func multgroup(t *testing.T) {
    t.Parallel()
    // ... same as before
}

```

Or you can bring the `t.Parallel` at the top level test function:

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

That's all there is to it. But



As a remedy people often leverage [testify's test suite] to logically group their subtests.
However, their

```go

```

Flat subtests start simple and then fall apart

Here is the obvious version. Every case lives at the same level in one function.

package mathops

import "testing"

func TestMath(t *testing.T) { t.Run("1+1=2", func(t *testing.T) { if 1+1 != 2 {
t.Fatal("want 2") } })

    t.Run("2+3=5", func(t *testing.T) {
    	if 2+3 != 5 {
    		t.Fatal("want 5")
    	}
    })

    t.Run("2*2=4", func(t *testing.T) {
    	if 2*2 != 4 {
    		t.Fatal("want 4")
    	}
    })

    t.Run("3*3=9", func(t *testing.T) {
    	// Imagine this one runs in parallel.
    	t.Parallel()
    	if 3*3 != 9 {
    		t.Fatal("want 9")
    	}
    })

}

The output is readable while the file is small:

=== RUN TestMath === RUN TestMath/1+1=2 === RUN TestMath/2+3=5 === RUN TestMath/2*2=4 ===
RUN TestMath/3*3=9 --- PASS: TestMath (0.00s)

Then reality happens. You add ten more addition cases, a dozen multiplication cases, and a
handful of error paths. The flat list turns into a wall. You cannot see which tests belong
to which operation without scanning every name. You cannot put setup and teardown logic in a
natural place because there is no grouping boundary. Parallel execution becomes hard to
reason about because a few t.Parallel calls in a flat space tell the runner to overlap
unrelated cases.

gRPC-Go uses reflection to discover groups

The gRPC-Go repository carries a small helper called grpctest. It uses reflection to find
methods named like tests on a value that you pass in. The runner calls each discovered
method as a subtest. You still use \*testing.T in those methods. The runner wraps each
method with Setup and Teardown hooks so a suite can perform logging and leak checks
consistently. The public API is one function, RunSubTests(t, x), and one helper type,
Tester, that provides default behavior for the hooks. The docs describe the discovery rule
and the hooks, and link to a simple example in the repository. Go Packages

Here is the same math suite written in that style.

package mathops

import ( "testing"

    "google.golang.org/grpc/internal/grpctest"

)

// MathSuite is a plain value. We do not need fields for this toy example. type MathSuite
struct{}

// TestAddition is discovered by grpctest because the name starts with "Test". func
(MathSuite) TestAddition(t *testing.T) { t.Run("1+1=2", func(t *testing.T) { if 1+1 != 2 {
t.Fatal("want 2") } }) t.Run("2+3=5", func(t \*testing.T) { if 2+3 != 5 { t.Fatal("want 5")
} }) }

// TestMultiplication is a second group. We will show nested parallelism here. func
(MathSuite) TestMultiplication(t *testing.T) { t.Run("2*2=4", func(t *testing.T) { if 2*2 !=
4 { t.Fatal("want 4") } }) t.Run("3*3=9", func(t *testing.T) { t.Parallel() // this case
runs concurrently with other subtests if 3\*3 != 9 { t.Fatal("want 9") } }) }

// TestMath is the only top-level symbol that "go test" sees. // RunSubTests reflects over
MathSuite and runs all Test* methods as subtests. // grpctest.Tester provides default Setup
and Teardown hooks. func TestMath(t *testing.T) { type suite struct { grpctest.Tester //
embed default hooks (logging, leak checks) MathSuite } grpctest.RunSubTests(t, suite{}) }

The output gains a clear hierarchy.

=== RUN TestMath === RUN TestMath/TestAddition === RUN TestMath/TestAddition/1+1=2 === RUN
TestMath/TestAddition/2+3=5 === RUN TestMath/TestMultiplication === RUN
TestMath/TestMultiplication/2*2=4 === RUN TestMath/TestMultiplication/3*3=9 --- PASS:
TestMath (0.00s)

There is one subtle point about serial and parallel here. grpctest does not add its own
suite-level parallel mode. It defers to the normal testing semantics. If you want
concurrency, you call t.Parallel in the test methods that should overlap, or you make those
methods spawn parallel subtests. The runner’s job is discovery and hooks, not scheduling.
The documentation for RunSubTests explains that it runs every method whose name starts with
Test and that it calls Setup and Teardown around each one. Go Packages

Bloomberg’s testgroup wraps testing.T and manages parallelism for you

Bloomberg’s go-testgroup takes a slightly different approach. You write a struct whose
exported methods are the group names, but each method receives a *testgroup.T instead of
*testing.T. That wrapper delegates to the standard library and also provides assertion
helpers. The library gives you two entry points. RunSerially runs each group one after
another. RunInParallel runs groups at the same time and asks you not to call t.Parallel
yourself because it will enable parallel execution for you and arrange the hooks in a
predictable order. The README and package documentation are explicit about this behavior and
show the exact execution sequence of group and test hooks when you run in parallel. GitHub
+1

Here is our math example with both modes.

package mathops

import ( "testing"

    "github.com/bloomberg/go-testgroup"

)

type MathGroup struct{}

// Addition defines two cases. We can still nest subtests, but the test handle // is
*testgroup.T, so we use its assertion helpers if we want to. func (g *MathGroup) Addition(t
*testgroup.T) { t.Run("1+1=2", func(t *testgroup.T) { t.Equal(2, 1+1) }) t.Run("2+3=5",
func(t \*testgroup.T) { t.Equal(5, 2+3) }) }

// Multiplication defines two cases. We will not call t.Parallel here because //
RunInParallel can take care of parallelism at the group level. func (g *MathGroup)
Multiplication(t *testgroup.T) { t.Run("2*2=4", func(t *testgroup.T) { t.Equal(4, 2*2) })
t.Run("3*3=9", func(t *testgroup.T) { t.Equal(9, 3*3) }) }

// Serial: groups run one after another. func TestMathSerial(t \*testing.T) {
testgroup.RunSerially(t, &MathGroup{}) }

// Parallel: groups run at the same time. Do not call t.Parallel inside methods. // The
runner creates a parent test, starts subtests in parallel, and then // runs PostGroup after
all of them finish. See the docs for the exact order. func TestMathParallel(t \*testing.T) {
testgroup.RunInParallel(t, &MathGroup{}) }

In serial mode the output looks like this:

=== RUN TestMathSerial === RUN TestMathSerial/Addition === RUN TestMathSerial/Addition/1+1=2
=== RUN TestMathSerial/Addition/2+3=5 === RUN TestMathSerial/Multiplication === RUN
TestMathSerial/Multiplication/2*2=4 === RUN TestMathSerial/Multiplication/3*3=9 --- PASS:
TestMathSerial (0.00s)

In parallel mode the groups overlap. The library places a parent test above the parallel
subtests so the pre- and post-hooks fire in the right places. The documentation calls out
that arrangement and reminds you not to mix your own t.Parallel with its group-level
parallelism. GitHub +1

I use third-party helpers sparingly because suites tend to become small DSLs

I am happy to import testify for a few assertions. I do not reach for its suite package
unless there is no alternative. The suite layer wires its own setup and teardown interfaces,
keeps state on a struct, and has known limitations around parallel tests. The package
documentation states that the suite layer does not support parallel tests and points to an
open issue. If you ever need to migrate away from it, you discover how many files now depend
on the library’s lifecycle rather than the standard library’s shape. Go Packages +1

There is also a philosophical point. The Go FAQ and several community writeups explain why
heavy test frameworks often grow a mini language of their own. You end up learning helper
macros and a small rulebook for how the framework behaves, when the language already gives
you functions, control flow, and data structures. I prefer to write tests in Go, not in a
framework’s dialect. Matt Proud has a recent essay that collects the argument and quotes the
FAQ directly. Go.dev +1

I usually group subtests by hand because it stays explicit

The smallest thing that could possibly work is a top-level driver that calls helpers. Each
helper owns a named t.Run block, does any setup that group needs, and defines its subtests.
You can also decide where parallelism belongs without any magic.

package mathops

import "testing"

func TestSuite(t \*testing.T) { additionGroup(t) multiplicationGroup(t) }

// additionGroup demonstrates a simple group that does not need setup. func additionGroup(t
*testing.T) { t.Run("Addition", func(t *testing.T) { t.Run("1+1=2", func(t *testing.T) { if
1+1 != 2 { t.Fatal("want 2") } }) t.Run("2+3=5", func(t *testing.T) { if 2+3 != 5 {
t.Fatal("want 5") } }) }) }

// multiplicationGroup shows scoped setup, teardown, and parallel subtests. func
multiplicationGroup(t *testing.T) { t.Run("Multiplication", func(t *testing.T) { // Setup
for the whole group. base := 2

    	// Teardown is local and obvious.
    	defer func() {
    		base = 0
    	}()

    	t.Run("2*2=4", func(t *testing.T) {
    		got := base * 2
    		if got != 4 {
    			t.Fatalf("want 4, got %d", got)
    		}
    	})

    	// This subtest can run in parallel with others in this process.
    	t.Run("3*3=9", func(t *testing.T) {
    		t.Parallel()
    		if 3*3 != 9 {
    			t.Fatal("want 9")
    		}
    	})
    })

}

The output tells a clear story.

=== RUN TestSuite === RUN TestSuite/Addition === RUN TestSuite/Addition/1+1=2 === RUN
TestSuite/Addition/2+3=5 === RUN TestSuite/Multiplication === RUN
TestSuite/Multiplication/2*2=4 === RUN TestSuite/Multiplication/3*3=9 --- PASS: TestSuite
(0.00s)

This shape scales. Each group keeps its own setup and teardown next to the cases that need
them. If a group requires a temporary directory or a server, you create it inside the group,
pass handles down to its subtests, and clean it up with a defer. Parallelism is visible
where it matters. If a set of cases can overlap safely, you add t.Parallel in those specific
subtests. If they cannot, you leave it out. Nothing is hidden in a runner, so there is
nothing to reverse engineer later.

If I really want less wiring I add a tiny reflection runner

Sometimes I want the convenience of gRPC-style discovery but I do not want a dependency. It
only takes a small helper to reflect over a suite value, find methods that look like tests,
and run them as named groups. This is the core idea behind grpctest: enumerate methods,
filter by name, and call them with a \*testing.T. The docs describe the same rule in gRPC’s
helper. Go Packages

Here is a minimal version that I have used in a few projects.

package mathops

import ( "reflect" "strings" "testing" )

func runSuite(t \*testing.T, suite any) { v := reflect.ValueOf(suite) typ := v.Type()

    for i := 0; i < typ.NumMethod(); i++ {
    	m := typ.Method(i)
    	if m.PkgPath != "" { // not exported
    		continue
    	}
    	if !strings.HasPrefix(m.Name, "Test") {
    		continue
    	}
    	t.Run(m.Name, func(t *testing.T) {
    		// method must be func(receiver, *testing.T)
    		m.Func.Call([]reflect.Value{v, reflect.ValueOf(t)})
    	})
    }

}

type MathSuite struct{}

func (MathSuite) TestAddition(t *testing.T) { t.Run("1+1=2", func(t *testing.T) { if 1+1 !=
2 { t.Fatal("want 2") } }) t.Run("2+3=5", func(t \*testing.T) { if 2+3 != 5 { t.Fatal("want
5") } }) }

func (MathSuite) TestMultiplication(t *testing.T) { t.Run("2*2=4", func(t *testing.T) { if
2*2 != 4 { t.Fatal("want 4") } }) t.Run("3*3=9", func(t *testing.T) { t.Parallel() if 3\*3
!= 9 { t.Fatal("want 9") } }) }

func TestMath(t \*testing.T) { runSuite(t, MathSuite{}) }

You can add optional hooks if you want to mirror gRPC’s structure more closely. A simple
variant is to look for SetupSuite and TeardownSuite methods with the right signature and
call them around the discovered tests. If you need per-test hooks, you can do the same when
you invoke each test method. The point is that you only add what you need, and the boundary
is still plain Go.

Where I land after trying these shapes

I like the way a named hierarchy makes large suites readable. I like having setup and
teardown local to a group. I like seeing the exact places where parallelism begins. For
those goals I do not need a large framework. I sometimes use testify for a few assertions,
and I have no problem importing grpctest or go-testgroup in a codebase that already depends
on them. In most of my own projects I group by hand and, if I really want a small boost, I
add a tiny reflection runner that I can delete later.

If you want more background on why people choose to keep tests close to the standard
library, the Go FAQ explains the philosophy clearly and Matt Proud’s essay collects the
argument with references. I find that perspective accurate when a team needs to maintain
tests for years across Go versions. Go.dev +1

References for the helpers mentioned above:

gRPC-Go grpctest package: discovery by reflection and test hooks, including RunSubTests,
Interface, and Tester. Go Packages

Bloomberg go-testgroup: grouping by exported methods that accept \*testgroup.T, serial and
parallel runners, and guidance about not calling t.Parallel when you use the parallel mode.
GitHub +1

Testify suite documentation and note about parallel test support. Go Packages

Go FAQ section on testing frameworks and mini languages, and Matt Proud’s essay. Go.dev +1

If you want, I can trim or expand any section to match your tone even more closely.

<!-- References -->
<!-- prettier-ignore-start -->
[support for subtests]:
    https://go.dev/blog/subtests

[testify's test suite]:
    https://github.com/stretchr/testify?tab=readme-ov-file#suite-package

<!-- prettier-ignore-end-->

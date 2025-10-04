---
title: Lifecycle management in Go tests
date: 2025-08-30
slug: lifecycle-management-in-tests
aliases:
    - /go/lifecycle_management_in_tests/
tags:
    - Go
    - Testing
---

Unlike pytest or JUnit, Go's standard testing framework doesn't give you as many knobs for
tuning the lifecycle of your tests.

By lifecycle I mean the usual setup and teardown hooks or fixtures that are common in other
languages. I think this is a good thing because you don't need to pick up many different
framework-specific workflows for something so fundamental.

Go gives you enough hooks to handle this with less ceremony. But it can still be tricky to
figure out the right conventions for setup and teardown that don't look odd to other
Gophers, especially if you haven't written Go for a while. This text explores some common
ways to do lifecycle management in your Go tests.

Before we cover multiple testing scenarios, it's useful to understand how Go's test harness
actually runs your tests.

## How Go discovers and runs your tests

When you type `go test`, Go doesn't interpret test files directly. It collects all the
`_test.go` files in a package, compiles them together with the rest of the package, and
produces a temporary binary. That binary contains both your code and your tests, along with
a small harness that drives them. The harness then runs the binary and reports results.

From the ["go test" command doc]:

> _"go test" automates testing the packages named by the import paths. \[...] recompiles
> each package along with any files with names matching the file pattern "\*\_test.go"._

### Discovery

Inside each package, the harness looks for test functions. A function qualifies if it has
the form:

```go
func TestXxx(t *testing.T)
```

where `Xxx` starts with an uppercase letter. There are no annotations or decorators, just
naming convention. Functions that don't match this signature are ignored.

### Execution

By default, the harness runs tests sequentially. If you want concurrency, you can opt in at
the test level. Calling `t.Parallel()` inside a test signals that this test may run
alongside others in the same package that also call `t.Parallel()`. Tests that don't opt in
remain strictly ordered.

### Scope of binaries

Every package with tests produces its own binary, and those binaries are run independently.
There is no global suite that links packages together, so setup and teardown only exist
inside one package's process. If you have ten packages containing tests, you get ten
binaries, each with its own lifecycle.

For example:

```txt
project/
├── go.mod
├── db/
│   ├── db.go
│   └── db_test.go
└── api/
    ├── api.go
    └── api_test.go
```

Running `go test ./...` produces two binaries: one for `db` and one for `api`. Each binary
bundles the package code and its tests, and each binary runs on its own. The harness
aggregates the results and prints a combined report, but execution itself is confined to the
package.

It is important to note that there is no file-level scope. All `_test.go` files in a package
are merged into a single binary, so there is no way to run setup once per file. Similarly,
there is no cross-package scope. Go does not let you set up once for all tests in a module
or tear down after the last package finishes. If you need orchestration across packages, it
has to happen outside of `go test`, for example in a shell script or a CI pipeline step.

With this background, we can now look at the lifecycle hooks Go does provide. They apply at
three levels: per test function, per group of subtests, and per package.

## Three different scopes

Typically you need to perform setup and teardown before and after:

- each test function is executed (single test function scope)
- a group of tests is executed (multiple test function scope)
- the full test suite is executed (test package scope)

### Per-test setup and teardown

The smallest scope is the test function itself. You create resources at the start of the
test and clean them up when it ends. This pattern is common when you want each test to run
against a fresh state with no leakage from other tests. The idiomatic way in Go is to wrap
the setup in a helper and register the cleanup with `t.Cleanup`.

```go
type TestDB struct{}

// newTestDB sets up a fresh database for a single test
func newTestDB(t *testing.T) *TestDB {
    t.Helper()
    db := &TestDB{}

    // cleanup tied to the function scope
    t.Cleanup(func() {
        db.Close()
    })

    return db
}

func (db *TestDB) Close() {}
func (db *TestDB) Insert(k, v string) error       { return nil }
func (db *TestDB) Query(k string) (string, error) { return "value", nil }

func TestInsert(t *testing.T) {
    db := newTestDB(t) // new DB created for this test only

    if err := db.Insert("foo", "bar"); err != nil {
        t.Fatalf("insert failed: %v", err)
    }
}
```

In this example, `TestInsert` gets its own new database. The cleanup registered with
`t.Cleanup` makes sure the database is closed when the test finishes. The resource is never
shared with other tests, which gives you strong isolation. The downside is that if your
setup is expensive, it will run before and after every test function, which can slow things
down.

### Grouped setup and teardown with subtests

The next scope is a group of subtests. Instead of repeating setup for every test, you create
the resource once in the parent test and share it with the children. Teardown runs when the
parent finishes. This works well when you want to test a flow of operations against the same
shared state.

```go
func TestUserFlow(t *testing.T) {
    // new DB created once for this group
    // t.Cleanup() gets called after all the subtests finish and
    // the parent returns
    db := newTestDB(t)

    t.Run("insert user", func(t *testing.T) {
        if err := db.Insert("user:1", "alice"); err != nil {
            t.Fatal(err)
        }
    })

    t.Run("query user", func(t *testing.T) {
        val, err := db.Query("user:1")
        if err != nil {
            t.Fatal(err)
        }
        if val != "alice" {
            t.Fatalf("expected alice, got %s", val)
        }
    })
}
```

Here both subtests share the same database, and the cleanup runs once when `TestUserFlow`
ends. This is useful when your tests need to act on shared state, like inserting a record
and then querying it. The trade-off is that the tests are no longer fully independent, and
if one subtest leaves the database in a bad state, others may fail in unexpected ways.

### Package-wide setup and teardown with `TestMain`

The broadest scope is the package. If you define `TestMain`, the test harness calls it
instead of running the tests directly. You can perform setup, run all the tests, and then
perform teardown. This allows you to reuse an expensive resource across all tests in the
package.

```go
var globalDB *TestDB

func TestMain(m *testing.M) {
    globalDB = &TestDB{} // setup once for the entire package

    code := m.Run()

    globalDB.Close() // teardown after all tests

    os.Exit(code)
}

func TestGlobalInsert(t *testing.T) {
    if err := globalDB.Insert("k", "v"); err != nil {
        t.Fatal(err)
    }
}
```

Here the database is created once and reused by all tests in the package. The teardown runs
when everything is finished. This can make your tests run much faster if setup is expensive,
but you pay for it in global (package wide) state. If one test mutates the shared resource
in an unexpected way, other tests may start failing, and debugging those failures can be
difficult.

Also, remember your setup and teardown are still package bound, meaning each package can
have its own `TestMain`. Reasoning about their order can get out of hand quickly. Make sure
your tests never depends on the order of `TestMain` execution. Treat these like `init`
functions and use them sparingly.

### Combining the levels

These three scopes are not mutually exclusive. You can combine them when you need different
levels of control. A typical pattern is to have `TestMain` start a package-wide service,
create a shared schema or fixture in a parent test for a group of related subtests, and then
still use per-test setup inside individual subtests for fine-grained isolation. Each call to
`newTestDB` creates a fresh database, so using it at different levels produces different
resources with different lifetimes.

```go
func TestOrders(t *testing.T) {
    schema := newTestDB(t) // group-level DB shared across subtests

    t.Run("create order", func(t *testing.T) {
        db := newTestDB(t) // per-test DB, fresh for this subtest only
        db.Insert("order:1", "widget")
    })

    t.Run("query order", func(t *testing.T) {
        // uses the group-level DB, so the state persists across subtests
        schema.Insert("order:1", "widget")
        val, _ := schema.Query("order:1")
        if val != "widget" {
            t.Fatalf("expected widget, got %s", val)
        }
    })
}
```

In this example, `TestMain` could be running a package-wide database server. The parent test
`TestOrders` sets up a schema that is shared across its subtests. Inside, one subtest spins
up its own per-test database to work in isolation, while another uses the shared schema to
test how state persists across operations.

The combination of package, group, and function scopes gives you flexibility: reuse
expensive resources when you need to, and isolate state when correctness depends on it.
However, combining scopes can be hard to reason about when you have many different subtests
under a single parent that are also interacting with some global state. I tend to avoid this
whenever possible.

## Parting words

Most of your setup and teardown should happen at the function level. That gives you the
strongest isolation and keeps each test self-contained.

The next most useful pattern is at the subtest group level, where you create a resource once
in a parent test and let its children share it. Cleanup runs when the parent finishes, which
makes sense when you really do want that shared state.

Package-level setup through `TestMain` should be rare. It is tempting when setup is
expensive, but global state is the fastest way to end up with brittle tests. Mixing
different scopes is possible, but usually creates more confusion than clarity, so reach for
it only when you have no better option.

<!-- References -->

<!-- prettier-ignore-start -->
["go test" command doc]:
    https://pkg.go.dev/cmd/go#hdr-Test_packages

<!-- prettier-ignore-end -->

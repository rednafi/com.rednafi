---
title: "Modernizers & go fix"
slug: gofix
date: 2026-07-04
description: >-
    Go 1.26 rebuilt go fix on the analysis framework. It modernizes your code and respects
    the Go version your module declares. The post covers the modernizers, the bigger x/tools
    suite, and what //go:fix inline can and can't migrate.
tags:
    - Go
    - Tooling
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /go/gofix/
atUri: ""
---

Go 1.26 rebuilt `go fix` from scratch. If you haven't tried it yet, give it a spin: it
rewrites the code in your module to use modern language and library features.

It has quickly become one of my favorite features, and LLMs are a big part of why. Models
tend to use old APIs, and sometimes they deny that a new API exists even when you point them
to it. Coaxing a model is non-deterministic. `go fix` is a better way to keep code on the
latest features of the language. Run it locally or in CI and the dated idioms get rewritten
deterministically.

Alan Donovan's [GopherCon talk] argues that future models train on today's open-source Go,
so the corpus itself needs modernizing too.

## A proper revival

`go fix` is almost as old as Go. Before the [Go 1 compatibility promise], the language and
standard library changed incompatibly all the time. Early adopters ran gofix to mechanically
patch their code after each weekly snapshot. Then Go 1 froze the language, and there was
nothing left for gofix to patch. The command stayed in the toolchain for over a decade until
the last of its hardcoded rewrites was [finally removed].

Go 1.26 brought it back, [rebuilt on the analysis framework] that powers `go vet`. Currently
`go fix` ships with 22 analyzers. Most of them are _"modernizers"_: each one recognizes a
specific dated idiom and rewrites it with the feature that replaced it.

> [!Note]
>
> Analyzers are the programs `go fix` runs to transform your code. Analyzers that modernize
> your code are also called modernizers. But there's another tool, `modernize`, that has
> even more of these analyzers. A later section covers the difference between `go fix` and
> `modernize`.

[Using go fix to modernize Go code] already walks through the command. What I want to add is
how the pieces fit together: the analyzers, the two different ways to run them, and what it
takes to migrate your own APIs.

## Running it

Start from a clean git state, so the resulting change contains nothing but what the tool
did. Then preview:

```sh
go fix -diff ./...
```

Here's a file that uses `interface{}`, a three-clause loop, manual string splitting, and a
pointer helper:

```go
func ptr[T any](v T) *T { return &v }

type config struct {
    timeout *int
    name    *string
}

func parse(hosts string) map[string]string {
    result := make(map[string]string)
    for _, entry := range strings.Split(hosts, ",") {
        i := strings.Index(entry, ":")
        if i >= 0 {
            result[entry[:i]] = entry[i+1:]
        }
    }
    return result
}

func main() {
    cfg := config{timeout: ptr(30), name: ptr("api")}
    for i := 0; i < 3; i++ {
        fmt.Println(i, *cfg.timeout, *cfg.name)
    }
    var x interface{} = parse("db:5432,cache:6379")
    fmt.Println(x)
}
```

On a module whose go.mod says `go 1.26`, `go fix -diff` prints this:

```diff
-func ptr[T any](v T) *T { return &v }
+//go:fix inline
+func ptr[T any](v T) *T { return new(v) }

 func parse(hosts string) map[string]string {
     result := make(map[string]string)
-    for _, entry := range strings.Split(hosts, ",") {
+    for entry := range strings.SplitSeq(hosts, ",") {
-        i := strings.Index(entry, ":")
-        if i >= 0 {
-            result[entry[:i]] = entry[i+1:]
+        before, after, ok := strings.Cut(entry, ":")
+        if ok {
+            result[before] = after
         }
     }
     return result
 }

 func main() {
-    cfg := config{timeout: ptr(30), name: ptr("api")}
+    cfg := config{timeout: new(30), name: new("api")}
-    for i := 0; i < 3; i++ {
+    for i := range 3 {
         fmt.Println(i, *cfg.timeout, *cfg.name)
     }
-    var x interface{} = parse("db:5432,cache:6379")
+    var x any = parse("db:5432,cache:6379")
     fmt.Println(x)
 }
```

Five analyzers fired on one small file. In the order their changes appear in the diff:

- [newexpr] noticed `ptr` is a "new-like" helper and rewrote its body at the top of the file
  to use Go 1.26's [new(expr)], which takes a pointer to a value in one step. The `ptr(30)`
  and `ptr("api")` call sites in `main` were rewritten the same way
- [stringsseq] replaced ranging over `strings.Split` in `parse` with Go 1.24's
  `strings.SplitSeq`, which skips allocating the slice
- [stringscut] replaced the `strings.Index` call and the manual slicing under it with Go
  1.18's `strings.Cut`
- [rangeint] turned the three-clause loop in `main` into Go 1.22's `for i := range 3`
- [any] swapped `interface{}` for `any` on the last variable

That diff has one more change: the `//go:fix inline` directive `newexpr` wrote above the
helper. It marks `ptr` as a wrapper whose calls should be replaced by `new(...)`. Directives
like that are how `go fix` migrates whole APIs. In this diff the directive does nothing:
`newexpr` rewrote the call sites itself, and the separate analyzer that acts on these
directives [doesn't handle generic functions yet].

`go fix` also respects the Go version in your module. Flip that go.mod line to `go 1.21` and
rerun, and the `range 3`, `SplitSeq`, and `new(expr)` rewrites all disappear. Only the `any`
and `strings.Cut` fixes remain, because both arrived in Go 1.18.

Each modernizer knows which release introduced its target feature and skips files below that
version. The version can come from go.mod or from a `//go:build go1.N` constraint. New
toolchains ship new modernizers and unlock more rewrites, so the Go team suggests rerunning
`go fix ./...` after every upgrade.

When the diff looks right, apply it and then run the tool again:

```sh
go fix ./...
go fix ./...
```

Applying one fix can make another one applicable. The Go blog calls these _"synergistic
fixes"_, and twice is usually enough to catch them. Fixes can also overlap: when two
rewrites touch the same lines, `go fix` applies one, drops the other, and warns you to run
it again. A final pass removes imports the fixes left unused.

Generated files never get touched. If a generated file has dated idioms, fix the generator.

> [!NOTE]
>
> Two fixes can also clash without touching the same lines. Say a variable has exactly two
> uses and each fix removes one. Apply both and the variable ends up unused. In Go that's a
> compile error. These conflicts are rare and usually fail the build, which makes them hard
> to miss.

You don't have to run every analyzer at once. Each one gets a flag:

```sh
go fix -newexpr ./...
go fix -forvar ./...
```

Run one analyzer per PR and each diff does exactly one thing instead of one giant mixed
cleanup patch. Negation works too: `-any=false` runs everything minus the one analyzer
you're not ready for.

> [!NOTE]
>
> A `go fix` run only sees the files your current platform would compile. A file tagged
> `//go:build windows` doesn't get fixed when you run on Linux. Repeat the run with
> different `GOOS` and `GOARCH` values if you ship platform-specific code. Also, `go fix`
> only rewrites your own module. Dependencies and vendored packages get read for type
> information but are [never edited].

`-diff` has a second job: it exits non-zero when a fix is pending, so `go fix -diff ./...`
doubles as a CI check.

## What the modernizers cover

The first demo covered strings and loops. The modernizers also handle slices and
concurrency:

```go
func contains(xs []string, want string) bool {
    for _, x := range xs {
        if x == want {
            return true
        }
    }
    return false
}

func process(tasks []string) {
    var wg sync.WaitGroup
    for _, t := range tasks {
        wg.Add(1)
        go func() {
            defer wg.Done()
            fmt.Println(t)
        }()
    }
    wg.Wait()
}

func main() {
    xs := []string{"b", "a"}
    sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
    fmt.Println(contains(xs, "a"))
    process(xs)
}
```

Three more analyzers fire on it:

```diff
 func contains(xs []string, want string) bool {
-    for _, x := range xs {
-        if x == want {
-            return true
-        }
-    }
-    return false
+    return slices.Contains(xs, want)
 }

 func process(tasks []string) {
     var wg sync.WaitGroup
     for _, t := range tasks {
-        wg.Add(1)
-        go func() {
-            defer wg.Done()
-            fmt.Println(t)
-        }()
+        wg.Go(func() {
+            fmt.Println(t)
+        })
     }
     wg.Wait()
 }

 func main() {
     xs := []string{"b", "a"}
-    sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
+    slices.Sort(xs)
     fmt.Println(contains(xs, "a"))
     process(xs)
 }
```

Top to bottom:

- [slicescontains] replaced the membership loop in `contains` with `slices.Contains`
- [waitgroup] rewrote the `wg.Add(1)`, `go`, `defer wg.Done()` sequence in `process` into Go
  1.25's `wg.Go`. The manual version of that bookkeeping is easy to get wrong
- [slicessort] turned the `sort.Slice` closure in `main` into `slices.Sort`

The rewrites also swapped the `sort` import for `slices`.

Those two demos covered eight analyzers. `go tool fix help` prints the full roster:

```txt
Registered analyzers:

    any          replace interface{} with any
    buildtag     check //go:build and // +build directives
    fmtappendf   replace []byte(fmt.Sprintf) with fmt.Appendf
    forvar       remove redundant re-declaration of loop variables
    hostport     check format of addresses passed to net.Dial
    inline       apply fixes based on 'go:fix inline' comment directives
    mapsloop     replace explicit loops over maps with calls to maps package
    minmax       replace if/else statements with calls to min or max
    newexpr      simplify code by using go1.26's new(expr)
    omitzero     suggest replacing omitempty with omitzero for struct fields
    plusbuild    remove obsolete //+build comments
    rangeint     replace 3-clause for loops with for-range over integers
    reflecttypefor replace reflect.TypeOf(x) with TypeFor[T]()
    slicescontains replace loops with slices.Contains or slices.ContainsFunc
    slicessort   replace sort.Slice with slices.Sort for basic types
    stditerators use iterators instead of Len/At-style APIs
    stringsbuilder replace += with strings.Builder
    stringscut   replace strings.Index etc. with strings.Cut
    stringscutprefix replace HasPrefix/TrimPrefix with CutPrefix
    stringsseq   replace ranging over Split/Fields with SplitSeq/FieldsSeq
    testingcontext replace context.WithCancel with t.Context in tests
    waitgroup    replace wg.Add(1)/go/wg.Done() with wg.Go
```

`go tool fix help minmax` prints the full docs for one analyzer, examples included.

The Go team keeps adding analyzers. Candidates pile up on the [modernizer tracking issue],
and whenever a new feature gets approved, they consider shipping a modernizer with it.

## What powers it

None of the machinery behind the rebuild is new. In 2017 the Go team split `go vet` into two
layers:

- [analyzers]: the algorithms that inspect code and produce the findings and fixes
- [drivers]: the programs that load packages and run analyzers over them

The result was the [analysis framework]. Write an analyzer once and any driver can run it:
`go vet`, gopls as you type, Bazel's nogo, or a binary of your own.

The rebuild made `go fix` one more driver on this framework. That makes `go vet` and
`go fix` almost the same program. Per the Go blog:

> The only differences between them are the criteria for the suites of algorithms they use,
> and what they do with computed diagnostics. Go vet analyzers must detect likely mistakes
> with low false positives; their diagnostics are reported to the user. Go fix analyzers
> must generate fixes that are safe to apply without regression in correctness, performance,
> or style; their diagnostics may not be reported, but the fixes are directly applied.

The convergence shows up in the flags too. Go 1.26's `go vet` gained its own `-fix` for
applying the safe fixes attached to vet diagnostics. The help text of `go tool fix` says it
plainly: report-style analyzers belong behind `go vet -vettool`, and fix-style analyzers
behind `go fix -fixtool`. That `-fixtool` flag is also how you run a custom fixer.

## go fix or modernize?

The modernizers are developed as a suite in the [modernize package]. The suite updates
continuously, and each Go release freezes a copy into the toolchain. Go 1.26's copy is the
22-analyzer roster you saw from `go tool fix help`: eighteen modernizers, the `inline`
analyzer, and the three housekeeping fixers `buildtag`, `hostport`, and `plusbuild`.

You run the standalone `modernize` command directly with `go run`:

```sh
go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -fix ./...
```

Its help lists mostly the same analyzers plus five extras:

```diff
 Registered analyzers:

     any          replace interface{} with any
+    atomictypes  replace basic types in sync/atomic calls with atomic types
+    embedlit     simplify references to embedded fields in composite literals
+    errorsastype replace errors.As with errors.AsType[T]
     ...
+    slicesbackward replace backward loops over slices with slices.Backward
     ...
+    unsafefuncs  replace unsafe pointer arithmetic with function calls
     ...
```

None of the five are in Go 1.26's `go fix` yet. I find it a bit strange that `errorsastype`
didn't make the cut: it rewrites `errors.As` to the `errors.AsType[T]` API that Go 1.26
itself introduced. The newest Go's fixer can't apply a fix for the newest Go API. If you
want that rewrite today, you have to run `modernize`. My guess is it lands in Go 1.27's
toolchain.

> [!WARNING]
>
> `modernize` comes with fewer guarantees than `go fix`. Its docs call it "not an officially
> supported interface", and running it at `@latest` means the analyzers can change under you
> with every x/tools release. Read the diff before merging.

Three modernizers are missing from both `go fix` and the standalone `modernize` command.
Each got pulled because its fix could change behavior, the one thing a fix must never do:

- `appendclipped` rewrote `append([]T{}, s...)` to `slices.Clone(s)`, but Clone [returns nil
  for an empty slice] and the append form never does
- `slicesdelete` rewrote the append-based delete idiom to `slices.Delete`, which [zeroes the
  vacated tail] to prevent leaks
- `bloop` rewrote `for range b.N` benchmarks to Go 1.24's `b.Loop()`, which [can shift
  nanosecond-scale benchmark numbers]

All three still exist in the modernize package as exported analyzers, disabled by default.
If the behavior change doesn't bother you, wrap one in the x/tools [unitchecker] driver that
`go fix` itself is built on:

```go
package main

import (
    "golang.org/x/tools/go/analysis/passes/modernize"
    "golang.org/x/tools/go/analysis/unitchecker"
)

func main() { unitchecker.Main(modernize.BLoopAnalyzer) }
```

Build it and pass the binary to `go fix`:

```sh
go build -o bloopfix .
go fix -fixtool=$(pwd)/bloopfix ./...
```

`-fixtool` swaps the toolchain's `fix` tool for yours, which means this run applies `bloop`
and nothing else.

The modernizers themselves have bugs. The Go team ran them across the standard library
during development and produced a long [bug list]. `mapsloop` was [still getting caught]
last November, when it turned a valid loop into a `maps.Copy` call that doesn't compile.

Neither tool has a per-line ignore comment. Disable the offender with its flag and file a
bug.

Analyzers also arrive through gopls, which ships new modernizers first and sometimes
suggests fixes your toolchain's `go fix` won't apply yet. There's an open proposal to [fold
a subset of staticcheck's analyzers] into `go fix`. The Go blog expects it in Go 1.27. And
`fmtappendf` shipped in 1.26, but x/tools has [since dropped it from the suite] because its
fix didn't clearly improve the code.

Run `go fix` by default. The [release notes] say it outright: a fixer that changes your
program's behavior is a bug to report.

## Migrations with //go:fix inline

So far all the rewrites target the language and the standard library. The same machinery
works on your own APIs too.

The first tool for that is the `//go:fix inline` directive. The Go blog covers it in depth
in [//go:fix inline and the source-level inliner]. Say your library renamed `greet.Hello` to
`greet.Greet`. Keep the old name as a one-line wrapper, deprecate it, and annotate it:

```go
package greet

// Greet returns a greeting for name.
func Greet(name string) string {
    return fmt.Sprintf("Hello, %s!", name)
}

// Hello returns a greeting for name.
//
// Deprecated: use [Greet] instead.
//
//go:fix inline
func Hello(name string) string {
    return Greet(name)
}
```

Nothing breaks. When a user of your library runs `go fix ./...`:

```diff
 func main() {
-    fmt.Println(greet.Hello("Go"))
+    fmt.Println(greet.Greet("Go"))
 }
```

It beats deprecating something and hoping everyone reads the changelog. gopls honors the
directive too. Callers see "Call of greet.Hello should be inlined" right in the editor and
can apply it as a quick fix. gopls can also inline any call on demand, even without a
directive.

The deprecated [golang.org/x/net/context] package carries these annotations today. Run
`go fix` on anything that still imports it and the calls move to the standard `context`.

The directive handles more than renames. Say the new API added a parameter the old one
hardcoded, and a type got a better name in the same release:

```go
package client

// FetchTimeout fetches url, giving up after timeout.
func FetchTimeout(url string, timeout time.Duration) ([]byte, error)

// Fetch fetches url with a 30-second timeout.
//
// Deprecated: use [FetchTimeout] to pick the timeout.
//
//go:fix inline
func Fetch(url string) ([]byte, error) {
    return FetchTimeout(url, 30*time.Second)
}

// Options configures the client.
type Options struct{ Retries int }

// Deprecated: use [Options].
//
//go:fix inline
type Config = Options
```

One `go fix` run migrates a caller off both:

```diff
 func main() {
-    b, err := client.Fetch("https://example.com")
+    b, err := client.FetchTimeout("https://example.com", 30*time.Second)
     fmt.Println(string(b), err)
-    var c client.Config
+    var c client.Options
     fmt.Println(c)
 }
```

The wrapper's hidden default is spelled out at each call site, and the `time` import gets
added automatically. This is not textual substitution. A wrapper can reorder parameters or
forward to another package. That's how `ioutil.ReadFile` calls become `os.ReadFile`.

The directive works on functions, type aliases, and constants. Only exported, package-level
symbols migrate across packages: an unexported helper's calls get rewritten only inside its
own package. A function needs a body, a type has to be a true alias, and a constant has to
refer to another named constant. Tests in the wrapper's own package don't get rewritten.

Annotate a whole deprecated package like that and eventually nobody imports it, so you can
delete it. The x/tools [deadcode] command finds the wrappers nobody calls anymore.

The inliner refuses to inline a callee that contains `defer` rather than wrap the body in a
function literal. And it doesn't handle generics yet, as the `ptr` helper showed.

When a clean substitution isn't safe, the inliner settles for correct but clunky output.
When nothing safe exists, it skips the call silently. Directive mistakes are silent too. Run
`go fix -json` to see them.

## What inline can't do

The inliner has a hard boundary: it can only put at the call site what the wrapper's body
already contains. It has no way to use what's already in scope there.

The classic case is adding a `context.Context` parameter. Your library's `store.Get(key)`
needs to become `store.GetContext(ctx, key)`. The forwarding wrapper has no `ctx` to
forward, so the best it can write is:

```go
// Deprecated: use [GetContext].
//
//go:fix inline
func Get(key string) (string, error) {
    return GetContext(context.Background(), key)
}
```

When a caller runs `go fix`, the wrapper's body gets copied to every call site,
`context.Background()` included:

```diff
 func Handle(ctx context.Context, key string) {
-    v, err := store.Get(key)
+    v, err := store.GetContext(context.Background(), key)
     fmt.Println(v, err)
 }
```

There's a `ctx` in scope one line up, and the rewrite ignores it. Every call site now
carries `context.Background()`, and nothing marks the ones that need attention.
`context.TODO()` in the wrapper won't save you either: the wrapper is live code and needs a
real default.

The fix has to happen per call site: use the `ctx` in scope, or fall back to something
greppable. No annotation on the old function can express that, but a custom analyzer can.
`go fix` runs it through the `-fixtool` flag. I'll cover how to write one in a separate
post.

You can get a lot done with just the built-in analyzers. I ran the new `go fix` on a large
RPC service at work, and `newexpr` alone cleaned up a pile of pointer helper calls that had
been accumulating for years. Extremely satisfying.

<!-- references -->
<!-- prettier-ignore-start -->

[GopherCon talk]:
    https://www.youtube.com/watch?v=_VePjjjV9JU

[Go 1 compatibility promise]:
    https://go.dev/doc/go1compat

[finally removed]:
    https://github.com/golang/go/issues/73605

[rebuilt on the analysis framework]:
    https://github.com/golang/go/issues/71859

[Using go fix to modernize Go code]:
    https://go.dev/blog/gofix

[any]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize#hdr-Analyzer_any

[rangeint]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize#hdr-Analyzer_rangeint

[stringsseq]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize#hdr-Analyzer_stringsseq

[stringscut]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize#hdr-Analyzer_stringscut

[newexpr]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize#hdr-Analyzer_newexpr

[slicescontains]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize#hdr-Analyzer_slicescontains

[slicessort]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize#hdr-Analyzer_slicessort

[waitgroup]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize#hdr-Analyzer_waitgroupgo

[new(expr)]:
    https://github.com/golang/go/issues/45624

[doesn't handle generic functions yet]:
    https://github.com/golang/go/issues/68236

[never edited]:
    https://github.com/golang/go/issues/76479

[modernizer tracking issue]:
    https://github.com/golang/go/issues/70815

[bug list]:
    https://github.com/golang/go/issues/71847

[still getting caught]:
    https://github.com/golang/go/issues/76380

[release notes]:
    https://go.dev/doc/go1.26

[since dropped it from the suite]:
    https://github.com/golang/go/issues/77581

[golang.org/x/net/context]:
    https://pkg.go.dev/golang.org/x/net/context

[deadcode]:
    https://pkg.go.dev/golang.org/x/tools/cmd/deadcode

[analysis framework]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis

[analyzers]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis#hdr-Analyzer

[drivers]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/multichecker

[modernize package]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize

[unitchecker]:
    https://pkg.go.dev/golang.org/x/tools/go/analysis/unitchecker

[returns nil for an empty slice]:
    https://github.com/golang/go/issues/73557

[zeroes the vacated tail]:
    https://github.com/golang/go/issues/73686

[can shift nanosecond-scale benchmark numbers]:
    https://github.com/golang/go/issues/74967

[fold a subset of staticcheck's analyzers]:
    https://github.com/golang/go/issues/76918

[//go:fix inline and the source-level inliner]:
    https://go.dev/blog/inliner


<!-- prettier-ignore-end -->

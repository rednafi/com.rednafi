---
title: Touring txtar
date: 2026-05-10
slug: txtar
tags:
    - Go
    - Testing
    - Tooling
description: >-
    txtar is a tiny plain-text archive format Russ Cox introduced in 2018 for
    multi-file test fixtures. The Go Playground, cmd/go's script tests, gopls's
    marker tests, and rsc.io/rf all reach for it.
---

I ran into [txtar] today while poking around `cmd/go`'s [testdata] directory and got curious
about why every test file looked like it had a tiny diff embedded in it. Turns out it's a
trivial archive format [Russ Cox introduced in 2018], and once I noticed it I started seeing
it everywhere in Go tooling.

A txtar archive looks like this:

```txt
Lines up here are the comment.

-- hello.txt --
hello, world

-- nested/foo.go --
package nested

func Foo() string { return "foo" }
```

Two file markers, two files, a free-text comment up top. That's the whole format. The
package doc says it outright: "There are no possible syntax errors in a txtar archive."

## The format

The [package doc comment] is the spec. The rules:

- A marker line is exactly `-- FILENAME --` at the start of a line. Three bytes `--<space>`
  open the marker, three bytes `<space>--` close it.
- Anything before the first marker is the comment.
- File data runs from one marker to the next, or to EOF.
- Whitespace inside the marker is trimmed, so `--   foo.go   --` parses as `foo.go`.
- A missing trailing newline on the final file is treated as if it were there.

Binary data isn't supported. File modes and symlinks aren't represented. There's no escape
mechanism for the marker syntax either, which is the [explicit] trade-off for keeping the
format this small. If a file body contains a line beginning with `-- ` and ending with
` --`, the parser will treat it as a new marker, and you have no way to quote it. Reach for
`tar` or `zip` when you need any of that.

The reason it stays this small is that it was [purpose-built] around three goals listed in
the package doc: stay hand-editable, store trees of text files for go command test cases,
and diff cleanly in git history and code reviews. It first landed inside the early `vgo`
modules prototype in 2018.

## Parse it from a string

The Go API lives in [`golang.org/x/tools/txtar`]. Two types and two functions cover the
common case:

```go
type Archive struct {
    Comment []byte
    Files   []File
}

type File struct {
    Name string
    Data []byte
}

func Parse(data []byte) *Archive
func ParseFile(file string) (*Archive, error)
```

`Parse` doesn't return an error. The format can't fail to parse.

```go
package main

import (
    "fmt"

    "golang.org/x/tools/txtar"
)

const archive = `Lines up here are the comment.

-- hello.txt --
hello, world

-- nested/foo.go --
package nested

func Foo() string { return "foo" }
`

func main() {
    ar := txtar.Parse([]byte(archive))

    fmt.Printf("comment: %q\n", ar.Comment)
    for _, f := range ar.Files {
        fmt.Printf("%s (%d bytes)\n", f.Name, len(f.Data))
    }
}
```

```txt
comment: "Lines up here are the comment.\n\n"
hello.txt (14 bytes)
nested/foo.go (51 bytes)
```

`Parse` returns slices that alias the input. Mutating the bytes you passed in will corrupt
the archive on the next read.

## Read it from disk

`ParseFile` does what you'd expect:

```go
ar, err := txtar.ParseFile("fixture.txt")
if err != nil {
    log.Fatal(err)
}
```

Same `*Archive`, just fed by `os.ReadFile` instead of a string literal.

## Mount it as an fs.FS

`txtar.FS`, [added in July 2024], hands you a read-only `fs.FS` view over the archive
without ever touching disk:

```go
fsys, err := txtar.FS(ar)
if err != nil {
    log.Fatal(err)
}

fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
    if err != nil {
        return err
    }
    fmt.Printf("%s (dir=%v)\n", p, d.IsDir())
    return nil
})
```

```txt
. (dir=true)
hello.txt (dir=false)
nested (dir=true)
nested/foo.go (dir=false)
```

Anything that takes an `fs.FS` works against the archive directly. A parser, a template
engine, or a static-site generator reads its fixtures straight from the archive in memory.
You don't need a tempdir or an extraction step.

## Format it back to bytes

`txtar.Format(*Archive) []byte` is the inverse of `Parse`:

```go
ar := &txtar.Archive{
    Comment: []byte("generated\n"),
    Files: []txtar.File{
        {Name: "main.go", Data: []byte("package main\n")},
        {Name: "go.mod", Data: []byte("module example\n")},
    },
}

os.Stdout.Write(txtar.Format(ar))
```

```txt
generated
-- main.go --
package main
-- go.mod --
module example
```

The package doesn't ship a write-files-to-disk helper. The canonical pattern is to walk
`ar.Files`, validate each path stays inside your destination, and write yourself. Go's
[`cmd/internal/script`] package has an `ExtractFiles` method that does exactly that. The
[`golang.org/x/exp/cmd/txtar`] CLI is another option, with `txtar -x` for extracting and
`txtar <path>` for archiving a file or directory.

## A golden test in one file

Say you've got a function `Format(in []byte) []byte` that pretty-prints some text format you
care about. JSON, SQL, markdown, whatever. You want to feed it a stack of inputs without
scattering ten-line files all over `testdata`. One txtar archive per case covers it. The
comment up top says what's being tested, `-- in --` is the input, `-- want --` is the
expected output.

`testdata/empty_object.txt`:

```txt
the empty object collapses

-- in --
{
}

-- want --
{}
```

`testdata/nested.txt`:

```txt
nested objects re-indent

-- in --
{"a":{"b":1}}

-- want --
{
  "a": {
    "b": 1
  }
}
```

The test globs the directory, parses each archive, and compares `Format(in)` against `want`:

```go
func TestFormat(t *testing.T) {
    fixtures, _ := filepath.Glob("testdata/*.txt")
    for _, path := range fixtures {
        t.Run(filepath.Base(path), func(t *testing.T) {
            ar, err := txtar.ParseFile(path)
            if err != nil {
                t.Fatal(err)
            }
            files := map[string][]byte{}
            for _, f := range ar.Files {
                files[f.Name] = f.Data
            }
            got := Format(files["in"])
            if !bytes.Equal(got, files["want"]) {
                t.Errorf("got %q, want %q", got, files["want"])
            }
        })
    }
}
```

Adding a case is one new file in `testdata/`. The comment documents intent, and the input
and expected output sit side by side. There's no per-test setup boilerplate, and no separate
`golden/` directory drifts out of sync.

The same shape works for a multi-file fixture. Add a `-- go.mod --` and three `-- *.go --`
files to one archive and you've got a hermetic mini-module to feed your linter, refactorer,
or codegen tool. That's what `cmd/go`'s script tests and `gopls`'s marker tests do, with
hundreds of fixtures each.

## When the comment is a script

The previous example used the archive as static data. `cmd/go`'s script tests use it
differently: the comment is a sequence of commands to run, and the file entries are the
workspace those commands operate on. [`rogpeppe/go-internal/testscript`] is the same engine
packaged as a library you can call from any test.

Say you want a smoke test for `tree`. You hand it a small project shaped via the file
entries, and assert on the tree it prints. One archive does both jobs:

`testdata/tree.txt`:

```txt
# tree should walk the workspace it was extracted into
exec tree --noreport --charset=utf-8 -I want
cmp stdout want

-- README.md --
# project
-- src/main.go --
package main
-- src/util.go --
package main
-- want --
.
├── README.md
└── src
    ├── main.go
    └── util.go
```

`exec` and `cmp` are testscript commands, not shell. `exec` runs a process, `cmp` compares
its captured stdout against the file named `want`. testscript materializes every file entry
into a fresh temp directory before the script runs, so `tree` walks the four files above.
The `-I want` flag tells `tree` to skip the `want` file itself, since it'd otherwise show up
in the listing.

The Go side is one function:

```go
package tree_test

import (
    "testing"

    "github.com/rogpeppe/go-internal/testscript"
)

func TestTree(t *testing.T) {
    testscript.Run(t, testscript.Params{Dir: "testdata"})
}
```

`testscript.Run` globs `testdata/*.txt`, parses each archive, drops the file entries into a
fresh temp directory, runs the script line by line, and reports a diff if `cmp` fails.
Adding another case is one more `.txt` archive under `testdata/`.

## In the wild

The 900+ `.txt` files in [`src/cmd/go/testdata/script/`] are txtar archives. The comment up
top is the script the test runs, and the files below are the workspace it runs in. The
[README] in that directory says "Each script is a text archive."

Sharing a multi-file snippet on the Go Playground encodes a txtar archive into the share
URL. The code is in [playground/txtar.go], added by Brad Fitzpatrick in 2019. The pre-marker
comment is treated as `prog.go`, which keeps single-file shares backwards compatible.

`gopls`'s marker tests under [`gopls/internal/test/marker/testdata/`] are txtar files too.
One archive packs the Go source, the golden output, a `-- flags --` section, and the gopls
settings into one end-to-end LSP test case.

Russ Cox's refactoring tool [`rsc.io/rf`] keeps each test case as a txtar archive. The
comment is the refactor command, the files are the input, and `-- stdout --` plus
`-- stderr --` carry the expected output.

[`rsc.io/script`] and [`rogpeppe/go-internal/testscript`] both extract the script engine
from `cmd/go` so you can run script fixtures in your own packages. Russ covers them in [Go
Testing By Example] under "Use txtar for multi-file test cases".

The pattern repeats across most of them. Each archive holds one test case, with the script
or input on top, the workspace below, and a `-- name --` section for golden output when you
need one. `gopls`'s marker tests put everything in named files like `flags` and
`settings.json` instead of a top/bottom split.

## Why it caught on

A directory full of fixture files is hard to review and harder to paste into a bug report.
txtar collapses all of it into one plain-text file that diffs cleanly in Gerrit and GitHub
and drops into a chat or an issue. With `txtar.FS` you don't need to extract anything to run
a test against it.

The format is small enough that you'd implement it in an afternoon. The reason to use the
upstream package is that everyone else in Go tooling already does, so your fixtures are
portable to `testscript`, `rsc.io/script`, the Playground, and anything else that adds a
txtar reader later.

> [!Gist]
>
> - txtar is `-- filename --` markers separating file bodies, with free text on top as a
>   comment. No escaping, no binary, no metadata. The format has no possible syntax errors
>   and no spec beyond the package doc comment.
> - The package is `golang.org/x/tools/txtar`. `Parse` reads bytes, `ParseFile` reads a
>   path, `Format` writes back. `txtar.FS` mounts an archive as a read-only `fs.FS` so tests
>   can run without ever touching disk.
> - [`rogpeppe/go-internal/testscript`] runs an archive as a script: the comment becomes
>   shell-like commands, the file entries become the workspace those commands run against.
>   Use it to drive a CLI test from one fixture per case.
> - It's the file shape behind `cmd/go`'s 900+ script tests, the Go Playground's multi-file
>   shares, gopls's marker tests, and `rsc.io/rf`. Reach for it when you want one
>   PR-friendly file per test case.

<!-- references -->
<!-- prettier-ignore-start -->

[txtar]:
    https://pkg.go.dev/golang.org/x/tools/txtar

[testdata]:
    https://github.com/golang/go/tree/master/src/cmd/go/testdata

[Russ Cox introduced in 2018]:
    https://go-review.googlesource.com/123359

[package doc comment]:
    https://github.com/golang/tools/blob/master/txtar/archive.go

[explicit]:
    https://github.com/golang/tools/blob/master/txtar/archive.go

[purpose-built]:
    https://research.swtch.com/testing

[`golang.org/x/tools/txtar`]:
    https://pkg.go.dev/golang.org/x/tools/txtar

[added in July 2024]:
    https://go-review.googlesource.com/c/tools/+/598756

[`cmd/internal/script`]:
    https://github.com/golang/go/blob/master/src/cmd/internal/script/state.go

[`golang.org/x/exp/cmd/txtar`]:
    https://pkg.go.dev/golang.org/x/exp/cmd/txtar

[`src/cmd/go/testdata/script/`]:
    https://github.com/golang/go/tree/master/src/cmd/go/testdata/script

[README]:
    https://go.dev/src/cmd/go/testdata/script/README

[playground/txtar.go]:
    https://github.com/golang/playground/blob/master/txtar.go

[`gopls/internal/test/marker/testdata/`]:
    https://github.com/golang/tools/tree/master/gopls/internal/test/marker/testdata

[`rsc.io/rf`]:
    https://github.com/rsc/rf/tree/main/testdata

[`rsc.io/script`]:
    https://pkg.go.dev/rsc.io/script

[`rogpeppe/go-internal/testscript`]:
    https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript

[Go Testing By Example]:
    https://research.swtch.com/testing

<!-- prettier-ignore-end -->

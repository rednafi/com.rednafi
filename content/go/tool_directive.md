---
title: Go 1.24's "tool" directive
date: 2025-04-13
tags:
    - Go
    - TIL
---

Go 1.24 added a new `tool` directive that makes it easier to manage your project's tooling.

I used to rely on Make targets to install and run tools like `stringer`, `mockgen`, and
linters like `gofumpt`, `goimports`, `staticcheck`, and `errcheck`. Problem is, these
installations were global, and they’d often clash between projects.

Another big issue was frequent version mismatch. I ran into cases where people were
formatting the same codebase differently because they had different versions of the tools
installed. Then CI would yell at everyone because it was always installing the latest
version of the tools before running them. Chaos!

## The `tools.go` convention

To avoid this mess, the Go community came up with a convention where you'd pin your tool
versions in a `tools.go` file. I've written about this before[^1]. But the gist is, you'd
have a `tools.go` file in your root directory that imports the tooling and assigns them to
`_`:

```go
//go:build tools

// tools.go
package tools

import (
    _ "github.com/golangci/golangci-lint/cmd/golangci-lint"
    _ "mvdan.cc/gofumpt"
)
```

Since these dependencies aren't used directly in the codebase, the `//go:build tools`
directive ensures they're excluded from the main build.

Then running `go mod tidy` keeps things clean and includes these dev dependencies in the
`go.mod` and `go.sum` files.

This works, but it always felt a bit clunky. You end up polluting your main `go.mod` with
tooling-only dependencies. And sometimes, transitive dependencies of those tools clash with
your app's dependencies.

The new `tool` directive in Go 1.24 solves _some_ of these pain points[^2].

## Enter the `tool` directive

With Go 1.24, you can now add tooling with the `-tool` flag when using `go get`:

```sh
go get -tool github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

This adds the dependency to your `go.mod` like this:

```txt
module github.com/rednafi/foo

go 1.24.2

tool github.com/golangci/golangci-lint/cmd/golangci-lint

// ... other transitive dependencies
```

Notice the `tool` directive clearly separates these from regular module dependencies.

Then you can run the tool with:

```sh
go tool golangci-lint run ./...
```

One thing to keep in mind: the first time you run a tool this way, it might take a second—Go
needs to compile it before running. After that, it's cached, so subsequent runs are fast.

## What about `go generate`?

This also plays nicely with `go generate`. I've started replacing direct tool calls with
`go tool`, so contributors don't need to install tools globally. Just run `go generate` and
you're done:

```go
//go:generate go tool stringer -type=MyEnum
```

No further setup needed, no path issues, and it's always using the version you pinned.

## Still not perfect

That said, one thing still bugs me: `go get -tool` adds these dev tools to the main `go.mod`
file. That means your application and dev dependencies are still mixed together. Same
problem the `tools.go` hack had.

There's no built-in way to avoid this yet. So your options are:

- Accept that dev and app deps will live in the same `go.mod` file.
- Create a separate `tools` module to isolate your tooling. A bit clunky, but doable.

I went with the second option.

My layout looks like this:

```txt
.
├── go.mod
├── go.sum
└── tools
    └── go.mod
```

Then I install tools like this:

```sh
cd tools
go get -tool github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

And run them from the root directory as follows:

```sh
go tool -modfile tools/go.mod golangci-lint run ./...
```

The `go tool` command supports a `-modfile` flag that you can use to specify where to pull
the tool version from. I _really_ wish `go get` supported `-modfile` too—that way you
wouldn't need to manage the dependencies in such a wonky manner. This was close to being
perfect. Well, maybe in a future release.

## Only works with Go-native tools

Another limitation is that it only works with tools written in Go. So if you're using stuff
like `eslint`, `prettier`, or `jq`, you're on your own. But for most of my projects, the dev
tooling is written in Go anyway, so this setup has been working okay.

[^1]: [Omitting dev dependencies in Go binaries](/go/omit_dev_dependencies_in_binaries)

[^2]:
    [Go toolchain still sticks the dev dependencies into the main go.mod file](/go/tool_directive/#still-not-perfect)

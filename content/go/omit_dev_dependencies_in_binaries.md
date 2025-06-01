---
title: Omitting dev dependencies in Go binaries
date: 2024-01-21
tags:
    - Go
    - TIL
---

As of now, unlike Python or NodeJS, Go doesn't allow you to specify your development
dependencies separately from those of the application. However, I like to specify the dev
dependencies explicitly for better reproducibility.

While working on a new CLI tool[^1] for checking dead URLs in Markdown files, I came across
this neat convention: you can specify dev dependencies in a `tools.go` file and then exclude
them while building the binary using a build tag.

Here's how it works. Let's say our project `foo` currently has the following structure:

```txt
foo
├── go.mod
├── go.sum
└── main.go
```

The `main.go` file contains a simple hello-world function that uses a 3rd party dependency
just to make a point:

```go
package main

import (
    "fmt"

    // Cowsay is a 3rd party app dependency
    cowsay "github.com/Code-Hex/Neo-cowsay"
)

func main() {
    fmt.Println(cowsay.Say(cowsay.Phrase("Hello, World!")))
}
```

Here, `Neo-cowsay` is our app dependency. To initialize the project, we run the following
commands serially:

```sh
go mod init example.com/foo   # creates the go.mod and go.sum files
go mod tidy                   # installs the app dependencies
```

Now, let's say we want to add the following dev dependencies: golangci-lint[^2] to lint the
project in the CI and gofumpt[^3] as a stricter `gofmt`. Since we don't import these tools
directly anywhere, they aren't tracked by the build toolchain.

But we can leverage the following workflow:

- Place a `tools.go` file in the root directory.
- Import the dev dependencies in that file.
- Run `go mod tidy` to track both app and dev dependencies via `go.mod` and `go.sum`.
- Specify a build tag in `tools.go` to exclude the dev dependencies from the binary.

In this case, `tools.go` looks as follows:

```go
// go:build tools

package tools

import (
    // Dev dependencies
    _ "github.com/golangci/golangci-lint/cmd/golangci-lint"
    _ "mvdan.cc/gofumpt"
)
```

Above, we're importing the dev dependencies and assigning them to underscores since we won't
be using them directly. However, now if you run `go mod tidy`, Go toolchain will track the
dependencies via the `go.mod` and `go.sum` files. You can inspect the dependencies in
`go.mod`:

```txt
// go.mod
module example.com/foo

go 1.21.6

require (
    github.com/Code-Hex/Neo-cowsay v1.0.4       // app dependency
    github.com/golangci/golangci-lint v1.55.2   // dev dependency
    mvdan.cc/gofumpt v0.5.0                     // dev dependency
)

// ... transient dependencies
```

Although we're tracking the dev dependencies along with the app ones, the build tag
`// go:build tools` at the beginning of `tools.go` file will instruct the build toolchain to
ignore them while creating the binary.

From the root directory of `foo`, you can build the project by running:

```sh
go build main.go
```

This will create a binary called `main` in the root directory. To ensure that the binary
doesn't contain the dev dependencies, run:

```sh
go tool nm main | grep -Ei 'golangci-lint|gofumpt'
```

This won't return anything if the dev dependencies aren't packed into the binary.

But if you do that for the app dependency, it'll print the artifacts:

```sh
go tool nm main | grep -Ei 'cowsay'
```

This prints:

```txt
1000b6d40 T github.com/Code-Hex/Neo-cowsay.(*Cow).Aurora
1000b6fb0 T github.com/Code-Hex/Neo-cowsay.(*Cow).Aurora.func1
1000b5610 T github.com/Code-Hex/Neo-cowsay.(*Cow).Balloon
1000b6020 T github.com/Code-Hex/Neo-cowsay.(*Cow).Balloon.func1
...
```

For some weird reason, if you want to include the dev dependencies in your binary, you can
pass the `tools` tag while building the binary:

```sh
go build --tags tools main.go
```

However, this will most likely fail if any of your dev dependencies aren't importable.

Here's an example[^4] of this pattern in the wild from the Kubernetes repo.

While it works, I'd still prefer to have a proper solution instead of a hack. Fin!

[^1]: [link-patrol](https://github.com/rednafi/link-patrol)

[^2]: [golangci-lint](https://github.com/golangci/golangci-lint)

[^3]: [gofumpt](https://github.com/mvdan/gofumpt)

[^4]:
    [tools.go in the kubernetes repo](https://github.com/kubernetes/kubernetes/blob/master/hack/tools/tools.go)

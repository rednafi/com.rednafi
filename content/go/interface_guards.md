---
title: Interface guards in Go
date: 2023-08-18
tags:
    - Go
    - TIL
---

I love Go's implicit interfaces. While convenient, they can also introduce subtle bugs
unless you're careful. Types expected to conform to certain interfaces can fluidly add or
remove methods. The compiler will only complain if an identifier anticipates an interface,
but is passed a type that doesn't implement that interface. This can be problematic if you
need to export types that are required to implement specific interfaces as part of their API
contract.

However, there's a way you can statically check interface conformity at compile time with
zero runtime overhead. Turns out, this was always buried in Effective Go[^1]. Observe:

```go
import "io"

// Interface guard
var _ io.ReadWriter = (*T)(nil)

type T struct {
    //...
}

func (t *T) Read(p []byte) (n int, err error) {
    // ...
}

func (t *T) Write(p []byte) (n int, err error) {
    // ...
}
```

We're checking if struct `T` implements the `io.ReadWriter` interface. It needs to have both
`Read` and `Write` methods defined. The type conformity is explicitly checked via
`var _ io.ReadWriter = (*T)(nil)`. It verifies that a `nil` pointer to a value of type `T`
conforms to the `io.ReadWriter` interface. The code will fail to compile if the type ever
stops matching the interface.

This is only possible because `nil` values in Go can assume many different[^2] types. In
this case, `var _ io.ReadWriter = T{}` will also work, but then you'll have to fiddle with
different zero values if the type isn't a struct. One important thing to point out is that
we're using `_` because we don't want to accidentally refer to this `nil` pointer anywhere
in our code. Also, trying to access any method on it will cause runtime panic.

Here's another example borrowed from Uber's style guide[^3]:

No check:

```go
type Handler struct {
  //...
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  //...
}
```

Check:

```go
type Handler struct {
  // ...
}

// Interface guard
var _ http.Handler = (*Handler)(nil)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  //...
}
```

Neat, but don't abuse this. Effective Go warns[^6]:

> _Don't do this for every type that satisfies an interface, though. By convention, such
> declarations are only used when there are no static conversions already present in the
> code, which is a rare event._

[^1]: [Interface checks - Effective Go](https://go.dev/doc/effective_go#interfaces)

[^2]: [nils in Go](https://go101.org/article/nil.html)

[^3]:
    [Check interface compliance - Uber style guide](https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance)

[^4]:
    [Interface guards - Caddy docs](https://caddyserver.com/docs/extending-caddy#interface-guards)
    [^4]

[^5]:
    [Tweet by Matt Boyle](https://twitter.com/MattJamesBoyle/status/1692428212058403251?s=20)
    [^5]

[^6]:
    [Don't abuse interface checks](https://go.dev/doc/effective_go#interfaces:~:text=The%20appearance%20of,a%20rare%20event)

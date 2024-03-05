---
title: Dysfunctional options pattern in Go
date: 2024-03-05
tags:
    - Go
drafts: true
---

Ever since Rob Pike published the text on the functional options pattern[^1], there's been
no shortage of blogs, talks, or comments on how it improves or obfuscates configuration
ergonomics.

While the necessity of such a pattern is quite evident in a language that lacks default
arguments in functions, more often than not, it needlessly complicates things. The situation
gets worse if you have to maintain a public API where multiple configurations are controlled
in this manner.

However, the pattern solves a valid problem and it definitely has its place. Otherwise, it
wouldn't have been picked up by pretty much every other library[^2][^3].

If you have no idea what I'm talking about, you might want to give my previous write-up on
configuring options[^4] a read.

## Functional options pattern

As a recap, here's how the functional options pattern works. Let's say, you need to allow
the users of your API to configure something. You can expose a struct from your package
that'll be passed to some other function to tune its behavior. For example:

```go
package src

type Config struct {
    // Required
    foo, bar string

    // Optional
    fizz, bazz int
}

func Do(config *Config) {
    // Do something with the config values
}
```

Then the `Config` struct will be imported by your API users, initialized, and passed to the
`Do` function:

```go
package main

import ".../src"

func main() {
    // Initialize the config and pass it to the Do function
    config := &src.Config{
        foo: "hello",
        bar: "world",
        fizz: 0,
        bazz: 42,
    }

    // Call Do with the initialized Config struct
    Do(config)
}
```

This is one way of doing that, but it's generally discouraged since it requires you to
expose the internals of your API to the users. So instead, the library usually exposes a
factory function that'll do the struct initialization while keeping the struct itself
private. For instance:

```go
package src

type config struct { // Notice that the struct is now private
    // Required
    foo, bar string

    // Optional
    fizz, bazz int
}

// Public factory function
func NewConfig(foo, bar string, fizz, bazz int) config {
    return config{foo, bar, fizz, bazz}
}

func Do(c *config){}
```

The API consumers will now use `NewConfig` to produce a configuration and then pass the
struct instance to the `Do` function as follows:

```go
package main

import ".../src"

func main() {
    // Initialize the config with the NewConfig factory
    c := &src.NewConfig("hello", "world", 0, 42)

    // Call Do with the initialized Config struct
    Do(c)
}
```

This approach is better as it keeps the internal machinery hidden from users. However, it
doesn't allow for setting default values for some configuration attributes; all must be set
explicitly. What if your users want to override the value of many attributes? This leads
your configuration struct to be overloaded with options, making the `NewConfig` function
demand numerous positional arguments.

This setup isn't user-friendly, as it forces API users to explicitly pass all these options
to the `NewConfig` factory. Ideally, you'd initialize `config` with some default values,
offering users the chance to override them. But, Go doesn't support default values for
function arguments, which leads us to the functional options pattern.

Here's how you can build your API to leverage the pattern:

```go
package src

type config struct {
    // Required
    foo, bar string

    // Optional
    fizz, bazz int
}

type option func(*config)

// The value of each optional configuration attribute can be overridden with
// an associated function
func WithFizz(fizz int) option {
    return func(c *config) {
        c.fizz = fizz
    }
}

func WithBazz(bazz int) option {
    return func(c *config) {
        c.bazz = bazz
    }
}

func NewConfig(foo, bar string, opts ...option) config {
    // First fill in the options with default values
    c := config{foo, bar, 10, 100}

    // Now allow users to override the optional options
    for _, opt := range opts {
        opt(&c)
    }
    return c
}

func Do(c *config) {}
```

Then you'd use it as follows:

```go
package main

import ".../src"

func main() {
    c := &NewConfig("hello", "world", src.WithFizz(1), src.WithBazz(2))
    src.Do(c)
}
```

The functional options pattern relies on functions that modify the configuration struct's
state. These modifier functions, or option functions, are defined to accept a pointer to the
configuration struct `*config` and then directly alter its fields. This direct manipulation
is possible because the option functions are closures, which means they capture and modify
the variables from their enclosing scope, in this case, the `config` instance.

In the `NewConfig` factory, the variadic parameter `opts ...option` allows for an arbitrary
number of option functions to be passed. Here, `opts` represents the optional configurations
that the users can override if they want to.

The `NewConfig` function iterates over this slice of option functions, invoking each one
with the `&c` argument, which is a pointer to the newly created `config` instance. The
config instance is created with default values, and the users can use the `With*` functions
to override them.

## Curse of indirection

That's a fair bit of indirection just to allow API users to configure some options. I don't
know about you, but multi-layered higher order functions hurt my tiny brain. It's quite slow
as well.

All this complexity could've been avoided if Go allowed default arguments in functions. Your
configuration factory could just set the default values from the keyword arguments and allow
the users to override the desired options while calling the function.

Also, the multiple layers of indirection hurt API discoverability. While calling the
factory, your users need to know which package-scoped function they'll need to call to
override some configuration attribute. Hovering your IDE's cursor over the return value of
the factory function isn't much helpful since the the modifier functions live in the package
level.

So if you need to configure multiple structs in this manner, their respective package level
modifier makes it even harder for the user to know which function they'll need to use to
update certain configuration attribute.

Recently, I've spontaneously stumbled upon a fluent-style API to manage configurations that
doesn't require so many layers of indirection. Let's call it the dysfunctional options
pattern.

## Dysfunctional options pattern

```go
package src

type config struct {
    // Required
    foo, bar string

    // Optional
    fizz, bazz int
}

// Each optional option will have its own public method
func (c *config) WithFizz(fizz int) *config {
    c.fizz = fizz
    return c
}

func (c *config) WithBazz(bazz int) *config {
    c.bazz = bazz
    return c
}

// The only accept the required options as params
func NewConfig(foo, bar string) *config {
    // First fill in the options with default values
    return &config{foo, bar, 10, 100}
}

func Do(c *config) {}
```

```go
package main

import ".../src"

func main() {
    c := NewConfig("hello", "world").WithFizz(0).WithBazz(42)
    Do(c)
}
```

[^1]:
    [Self-referential functions and the design of options](https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html)

[^2]:
    [Functional options pattern in ngrok](https://github.com/ngrok/ngrok-api-go/blob/ec1a3e91cae94c70f0e5c31b95aed5a1d6dd65b7/client_config.go)

[^3]:
    [Function options pattern in elastic search agent](https://github.com/elastic/elastic-agent/blob/4aeba5b3fcf0d72924c70ff2127996a817b83a23/pkg/testing/fetcher_http.go)

[^4]: [Configuring options in Go](/go/configure_options)

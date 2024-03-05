---
title: Dysfunctional options pattern in Go
date: 2024-03-05
tags:
    - Go
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

As a recap, here's how the functional options pattern works. Let's say you need to allow the
users of your API to configure something. You can expose a struct from your package that'll
be passed to some other function to configure its behavior. For example:

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

The API consumers will now use `NewConfig` to configure the `Do` function as follows:

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

This is better and avoids exposing how things work internally. However, what happens if you
need to let your users customize a lot of settings? That means your configure struct will
end up with many options, leading the `NewConfig` function to require numerous arguments.

This isn't ideal for the users of your API, as they'll have to provide all these options as
arguments to the `NewConfig` factory. You might consider initializing `config` with some
default values, allowing users the option to override them. But, since Go doesn't support
default values for function arguments, we need something like the functional options
pattern.

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

// Each optional option will have its own public function
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
is possible because the option functions are closures, which means they capture and

modify the variables from their enclosing scope, in this case, the `config` instance.

In the `NewConfig` factory, the variadic parameter `opts ...option` allows for an arbitrary
number of option functions to be passed. Here, `opts` represents the optional configurations
that the users can override if they want to.

The `NewConfig` function iterates over this slice of option functions, invoking each one
with the `&c` argument, which is a pointer to the newly created `config` instance. The
config instance is created with default values, and the users can use the `With*` functions
to override them.

That's a lot of work to allow users to configure some options. It's quite slow as well.
Allowing default arguments in functions would've made this entirely redundant. Your config
factory could just set the default values from the keyword arguments and let the users to
override the desired options while calling the factory function.

Recently, I've been using a fluent-style API to manage configurations that doesn't require
so many layers of indirections. Let's call it dysfunctional options pattern.

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

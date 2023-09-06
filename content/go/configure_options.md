---
title: Configuring options in Go
date: 2023-09-05
tags:
    - Go
---

Suppose, you have a function that takes an option struct and a message as input. Then it
stylizes the message according to the option fields and prints it. What's the most sensible
API you can offer for users to configure your function? Observe:

```go
// app/src
package src

// Option struct
type Style struct {
    Fg string // ANSI escape codes for foreground color
    Bg string // Background color
}

// Display the message according to Style
func Display(s *Style, msg string) {}
```

In the `src` package, the function `Display` takes a pointer to a `Style` instance and a
`msg` string as parameters. Then it decorates the `msg` and prints it according to the style
specified in the option struct. In the wild, I've seen 3 main ways to write APIs that let
users configure options:

- Expose the option struct directly
- Use the option factory pattern
- Apply functional option factory pattern

Each comes with its own pros and cons.

## Expose the option struct

In this case, you'd export the `Style` struct with all its fields and let the user configure
them directly. The previous snipped already made the struct and fields public. From another
package, you could import the `src` package and instantiate `Style` like this:

```go
package main
import "app/src"

// Users instantiate the option struct
c := &src.Style{
    "\033[31m", // Maroon
    "\033[43m", // Yellow
}

// Then pass the struct to the function
Display(c, "Hello, World!")
```

To configure option fields, mutate the values in place:

```go
c.Fg = "\033[35m" // Magenta
c.Bg = "\033[40m" // Black
```

This works but will break users' code if new fields are added to the option struct. But your
users can instantiate the struct with named parameters to avoid breakage:

```go
c := &src.Style{
    Fg: "\033[31m", // Maroon
                   // Bg will be implicitly set to an empty string
}
```

In this case, the field that wasn't passed will assume the corresponding zero value. For
instance, `Bg` will be initialized as an empty string. However, this pattern puts the
responsibilty of retaining API compatibity on the users' shoulders. So if your code is meant
for external use, there are better ways to achieve option configurability.

## Option factory

Go standard library extensively uses this pattern. Instead of letting the users instantiate
`Style` directly, you expose a `NewStyle` factory function that constructs the struct
instance for them:

```go
package src
// same as before

// NewStyle option factory instantiates a Style instance
func NewStyle(fg, bg string) *Style {
    return &Style{fg, bg}
}
```

It'll be used as follows:

```go
package main
import "app/src"

// The users will now use NewStyle to instantiate Style
c := src.NewStyle(
    "\033[31m", // Maroon
    "\033[43m", // Yellow
)
Display(c, "Hello, World!")
```

If a new field is added to `Style`, update `NewStyle` to have a sensible default value for
it or initialize the struct with named parameters to set the optional fields to their
respective zero values. This avoids breaking users' code as long as the factory function's
signature doesn't change.

```go
package src

type Style struct {
    Fg string
    Bg string
    Und bool // Underline or not
}

// Function signature unchanged though new option field added
// Set sensible default in factory function
func NewStyle(fg, bg string) *Style{
    return &Style{
        Fg: fg, Bg: bg, // Und will be implicitly set to false
    }
}
```

In `NewStyle`, we implicitly set the value of `Und` to `false` but you can be explicit there
depending on your needs. The struct fields can be updated in the same manner as before:

```go
package main

c := src.NewStyle(
    "\033[31m", // Maroon
    "\033[43m", // Yellow
)
c.Und = true // Default is false, we're setting it to true
src.Display(c, "Hello, World!")
```

This should cover most use cases. However, if you don't want to export the underlying option
struct, or your struct has tons of optional fields requiring extensibility, you'll need an
extra layer of indirection to avoid the need to accept a zillion config parameters in your
option factory.

## Functional option factory

As mentioned at the tail of the last section, this approach works better when your struct
contains many optional fields and you need your users to be able to configure them if they
want. Go doesn't allow setting non-zero default values for struct fields. So an extra level
of indirection is necessary to let the users configure them. Let's say `Style` now has two
optional fields `Und` and `Zigzag` that allow users to decorate the message string with
underlines or zigzagged lines:

```go
package src

type style struct {
    fg string
    bg string
    und bool // Optional field
    zigzag bool // Optional field
}
```

Now, we'll define a new type called `styleoption` like this:

```go
// package src
type styleoption func(*style)
```

The `styleoption` function accepts a pointer to the option struct and updates a particular
field with a user-provided value. The implementation of this type would look as such:

```go
func (s *style) {s.fieldName = fieldValue}
```

Next, we'll need to define a higher order config function for each optional field in the
struct where the function will accept the field value and return another function with the
`styleoption` signature. The `WithUnd` and `WithZigzag` wrapper functions will be a part of
the public API that the users will use to configure `style`:

```go
// We only define config functions for the optional fields
func WithUnd(und bool) styleoption {
    return func(s *style) {
        s.und = true
    }
}

func WithZigzag(zigzag bool) styleoption {
    return func(s *style) {
        s.zigzag = true
    }
}
```

Finally, our option factory function needs to be updated to accept variadic options. Observe
how we're looping through the `options` slice and applying the field config functions to the
struct pointer:

```go
func NewStyle(fg, bg string, options ...styleoption) *style {
    s := &style{fg: fg, bg: bg} // und and zigzag are set to false

    // Apply all the styleoption functions returned from
    // field config functions.
    for _, opt := range options {
        opt(s)
    }
    return s
}
```

The users will use the code like this to instantiate `Style` and update the optional fields:

```go
c := src.NewStyle(
    "\033[31m",
    "\033[43m",
    src.WithUnd(true), // Default is false, but we're setting it to true
    src.WithZigzag(true), // Default is false
)
fmt.Printf("%+#v\n", c)
```

The required fields `fg` and `bg` must be passed while constructing the option struct. The
optional fields can be configured with the field config functions like `WithUnd` and
`WithZigzag`.

The complete snippet looks as follows:

```go
package src

// We can keep the option struct private
type style struct {
    fg string
    bg string
    und bool // Optional field
    zigzag bool // Optional field
}

// This can be priavate too since the users won't need it directly
type styleoption func(*style)

// We only define public config functions for the optional fields
func WithUnd(und bool) styleoption {
    return func(s *style) {
        s.und = true
    }
}

func WithZigzag(zigzag bool) styleoption {
    return func(s *style) {
        s.zigzag = true
    }
}

// Options are variadic but the required fiels must be passed
func NewStyle(fg, bg string, options ...styleoption) *style {
	// You can also intialize the optional values explicitly
    s := &style{fg: fg, bg: bg}
    for _, opt := range options {
        opt(s)
    }
    return s
}
```

I first came across this pattern in Rob Pike's [blog] on the same topic.

## Verdict

While the functional factory pattern is the most intriguing one among the three, I almost
never reach for it unless I need my users to be able to configure large option structs with
many optional fields. It's rare and the extra indirection makes the code inscrutable. Also,
it renders the IDE suggestions useless.

In most cases, you can get away with exporting the option struct `Stuff` and a companion
function `NewStuff` to instantiate it. For another canonical example, see `bufio.Read` and
`bufio.NewReader` in the standard library.

## References

* [Self-referential functions and the design of options - Rob Pike][blog]
* [Functional options pattern in Go - Matt Boyle][tweet]

[blog]: https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html
[tweet]: https://twitter.com/MattJamesBoyle/status/1698605808517288428

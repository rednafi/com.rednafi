---
title: Type assertion vs type switches in Go
date: 2024-01-31
tags:
    - Go
    - TIL
---

Despite moonlighting as a gopher for a while, the syntax for type assertion and type
switches still trips me up every time I need to go for one of them.

So, to avoid digging through the docs or crafting stodgy LLM prompts multiple times, I
decided to jot this down in a gobyexample[^1] style for the next run.

## Type assertion

Type assertion in Go allows you to access an interface variable's underlying concrete type.
After a successful assertion, the variable of interface type is converted to the concrete
type to which it is asserted.

The syntax is `i.(T)`, where `i` is a variable of interface type and `T` is the type you are
asserting.

### Basic usage

```go
var i interface{} = "Hello" // or use `any` as an alias for `interface{}`

s := i.(string)
fmt.Println(s)
```

Here, `s` gets the type `string`, and the program outputs `Hello`.

### Asserting primitive types and values

```go
var i interface{} = 42

if v, ok := i.(int); ok {
    fmt.Println("integer:", v)
}
```

This code checks if `i` is an `int` and prints its value if so. The value of `ok` will be
`false` if `i` isn't an integer and nothing will be printed to the console.

### Asserting composite types and values

```go
var i interface{} = []string{"apple", "banana", "cherry"}

if v, ok := i.([]string); ok {
    fmt.Println("slice of strings:", v)
}
```

This will print `slice of strings: [apple banana cherry]` to the console.

Similar to primitive types, you can also perform type assertions with composite types. In
the example above, we check whether the variable `i`, which is of an interface type, holds a
value of the type 'slice of strings'.

### Asserting other interfaces

```go
type fooer interface{ foo() }
type barer interface{ bar() }
type foobarer interface { fooer; barer }

type thing struct{}

func (t *thing) foo() {}
func (t *thing) bar() {}

var i foobarer = &thing{}

func main() {
    if v, ok := i.(fooer); ok {
        fmt.Println("i satiesfies fooer:", v)
    }
}
```

Type assertion can also be used to convert the type of an interface variable to another
interface type. Here struct `i` implements both `foo()` and `bar()` methods; satisfying the
`foobarer` interface.

Then in the `main` function, we check whether `i` satisfies `fooer` interface and print a
message if it does. Running this snippet will print `i satiesfies fooer: &{}`.

### Handling failures

```go
var i interface{} = "Hello"

f := i.(float64) // This triggers a panic
```

Wrong assertions, like attempting to convert a string to a float64, cause runtime panics.

## Type switches

Type switches let you compare an interface variable's type against several types. It's
similar to a regular switch statement, but focuses on types.

### Basic usage

```go
var i interface{} = 7

switch i.(type) {
case int:
    fmt.Println("i is an int")
case string:
    fmt.Println("i is a string")
default:
    fmt.Println("unknown type")
}
```

This outputs `i is an int`.

### Using a variable in a type switch case

```go
var i interface{} = []byte("hello")

switch v := i.(type) {
case []byte:
    fmt.Println(string(v))
case string:
    fmt.Println(v)
}
```

Notice how we're assinging variable `v` to `i.(type)` and then reusing the extracted value
in the case statements. The snippet converts `[]byte` to a string and prints `hello`.

### Handling multiple types

```go
var i interface{} = 2.5

switch i.(type) {
case int, float64:
    fmt.Println("i is a number")
case string:
    fmt.Println("i is a string")
}
```

The `case T1, T2` syntax works like an OR relationship, outputting `i is a number`.

### Addressing composite types

```go
var i interface{} = map[string]bool{"hello": true, "world": false}

switch i.(type) {
case map[string]bool:
    fmt.Println("i is a map")
case []string:
    fmt.Println("i is a slice")
default:
    fmt.Println("unknown type")
}
```

Similar to primitive types, you can check for composite types in the case statement of a
type switch. Here, we're checking whether `i` is a `map[string]bool` or not. Running this
will output `i is a map`.

### Comparing against interface types

```go
type fooer interface{ foo() }
type barer interface{ bar() }
type foobarer interface { fooer; barer }

type thing struct{}

func (t *thing) foo() {}
func (t *thing) bar() {}

var i foobarer = &thing{}

func main() {
    switch v := i.(type) {
    case fooer:
        fmt.Println("fooer:", v)
    case barer:
        fmt.Println("barer:", v)
    case foobarer:
        fmt.Println("foobarer:", v)
    default:
        panic("none of them")
    }
}
```

Type switches can be also used to compare an interface variable with another interface type.
This example is similar to the type assertion one where we're checking whether `i` satisfies
`fooer`, `barer` or `foobarer` interface. In this case, `i` satisfies all three of them but
the case statement will stop after the first successful check. So it prints `fooer: &{}` and
bails.

## Similarities and differences

### Similarities

-   Both handle interfaces and extract their concrete types.
-   They evaluate an interface's dynamic type.

### Differences

-   Type assertions check a single type, while type switches handle multiple types.
-   Type assertion uses `i.(T)`, type switch uses a switch statement with literal
    `i.(type)`.
-   Type assertions can panic or return a success boolean, type switches handle mismatches
    more gracefully.
-   Type assertions are good when you're sure of the type. Type switches are more versatile
    for handling various types.
-   Type assertion can get the value and success boolean. Type switches let you access the
    value in each case block.
-   Type switches can handle multiple types, including a default case, offering more
    flexibility for various types.

Fin!

[^1]: [Go by example](https://gobyexample.com/)

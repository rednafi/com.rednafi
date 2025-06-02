---
title: Nil comparisons and Go interface
date: 2025-03-12
tags:
    - Go
---

Comparing interface values in Go has caught me off guard a few times, especially with nils.
Often, I'd expect a comparison to evaluate to `true` but got `false` instead.

Many moons ago, Russ Cox wrote a fantastic [blog post] on interface internals that clarified
my confusion. This post is a distillation of my exploration of interfaces and nil
comparisons.

## Interface internals

Roughly speaking, an interface in Go has three components:

- A static type
- A dynamic type
- A dynamic value

For example:

```go
var n any  // The static type of n is any (interface{})
n = 1      // Upon assignment, the dynamic type becomes int
           // And the dynamic value becomes 1
```

Here, the static type of `n` is `any`, which tells the compiler what operations are allowed
on the variable. In the case of `any`, any operation is allowed. When we assign `1` to `n`,
it adopts the dynamic type `int` and the dynamic value `1`.

Internally, every interface value is implemented as a two [word] structure:

- One word holds a pointer to the dynamic type (i.e., a type descriptor).
- The other word holds the data associated with that type.

This data word might directly contain the value if it's small enough, or it might hold a
pointer to the actual data. Note that this internal representation is distinct from the
interface's declared or "static" type—the type you wrote in the code (`any` in the example
above). At runtime, what gets stored is only the pair of dynamic type and dynamic value.
Here's a crude diagram:

```txt
+-----------------------+
|   Interface           |
+-----------------------+
| Pointer to type info  |  ---> [Dynamic type descriptor]
+-----------------------+
| Data                  |  ---> [Dynamic value or pointer to the value]
+-----------------------+
```

## Comparing nils with interface variables

Nil comparisons can be tricky because an interface value is considered nil only when both
its dynamic type and dynamic value are nil. A few examples.

### Comparing a nil pointer directly

```go
var p *int  // p is a nil pointer of type *int
if p == nil {
    fmt.Println("p is nil")
}
// Output: p is nil
```

Here, `p` is a pointer to an int and is explicitly nil, so the comparison works as expected.
This doesn't have anything to do with explicit interfaces, but it's important to demo basic
nil comparison to understand how comparisons work with interfaces.

### An interface variable explicitly set to nil

```go
var r io.Reader  // The static type of r is io.Reader
r = nil          // The dynamic type is nil
                 // The dynamic value is nil

// Since both the dynamic type and value evaluate to nil, r == nil is true
if r == nil {
    fmt.Println("r is nil")
}
// Output: r is nil
```

In this case, `r` is directly set to nil. Since both the dynamic type and the dynamic value
are `nil`, the interface compares equal to nil.

### Assigning a nil pointer to an interface variable

```go
var b *bytes.Buffer    // b is a nil pointer of type *bytes.Buffer
var r io.Reader = b    // The static type of r is io.Reader.
                       // The dynamic type of r is *bytes.Buffer.
                       // The dynamic value of r is nil.

// Although b is nil, r != nil because r holds type information (*bytes.Buffer).
if r == nil {
    fmt.Println("r is nil")
} else {
    fmt.Println("r is not nil")
}
// Output: r is not nil
```

Even though `b` is nil, assigning it to the interface variable `r` gives `r` a non-nil
dynamic type (`*bytes.Buffer`) with a nil dynamic value. Since `r` still holds type
information, `r == nil` returns `false`, even though the underlying value is nil.

> _When comparing an interface variable, Go checks both the dynamic type and the value. The
> variable evaluates to nil only if both are nil._

### Using type assertions for reliable nil checks

In cases where an interface variable might hold a nil pointer, we've seen that comparing the
interface directly to nil may not yield the expected result.

A type assertion can help extract the underlying value so that you can perform a more
reliable nil check. This approach is especially useful when you know the expected underlying
type.

Below, we define a simple type `myReader` that implements the `Read` method to satisfy the
`io.Reader` interface.

```go
type myReader struct{}

func (mr *myReader) Read(p []byte) (int, error) {
    return 0, nil
}
```

Now, consider the following example:

```go
var mr *myReader        // mr is a nil pointer of type *myReader
var r io.Reader = mr    // The static type of r is io.Reader
                        // The dynamic type of r is *myReader
                        // The dynamic value of r is nil

// Use a type assertion to extract the underlying *myReader value.
if underlying, ok := r.(*myReader); ok && underlying == nil {
    fmt.Println("r holds a nil pointer")
} else {
    fmt.Println("r does not hold a nil pointer")
}
// Output: r holds a nil pointer
```

Here, we assert that `r` holds a value of type `*myReader`. If the assertion succeeds
(indicated by `ok` being `true`) and the `underlying` value is `nil`, we can conclude that
the interface variable holds a nil pointer—even though the interface itself is not nil due
to its dynamic type.

This type assertion trick only works when you know the underlying type of the interface
value. If the type might vary, consider using the reflect package to examine the underlying
value.

### Writing a generic nil checker with reflect

The following function introspects any variable and checks whether it's nil:

```go
func isNil(i any) bool {
    if i == nil {
        return true
    }
    // Note: Arrays are not nilable, so we don't check for reflect.Array.
    switch reflect.TypeOf(i).Kind() {
    case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Slice, reflect.Func:
        return reflect.ValueOf(i).IsNil()
    }
    return false
}
```

The switch on `.Kind()` is necessary because directly calling `reflect.ValueOf().IsNil()` on
a non-pointer value will cause a panic.

Calling this function on any value, including an interface, reliably checks whether it's
nil.

Fin!

<!-- References -->
<!-- prettier-ignore-start -->

<!-- go data structures: interfaces - russ cox -->
[blog post]:
    https://research.swtch.com/interfaces

[word]:
    https://en.wikipedia.org/wiki/Word_(computer_architecture)

<!-- prettier-ignore-end -->

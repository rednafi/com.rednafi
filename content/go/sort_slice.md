---
title: Three flavors of sorting slices in Go
date: 2025-03-22
tags:
    - Go
---

There are primarily three ways of sorting slices in Go. Early on, we had the verbose but
flexible method of implementing `sort.Interface` to sort the elements in a slice. Later,
Go 1.8 introduced `sort.Slice` to reduce boilerplate with inline comparison functions.
Most recently, Go 1.21 brought generic sorting via the `slices` package, which offers a
concise syntax and compile-time type safety.

These days, I mostly use the generic sorting syntax, but I wanted to document all three
approaches for posterity.

## Using sort.Interface

The oldest technique is based on `sort.Interface`. You create a custom type that wraps your
slice and implement three methods—`Len`, `Less`, and `Swap`—to satisfy the interface. Then
you pass this custom type to `sort.Sort()`.

### Sorting a slice of integers

The following example defines an `IntSlice` type. Passing an `IntSlice` to `sort.Sort`
arranges its integers in ascending order:

```go
import (
    "fmt"
    "sort"
)

// Define a custom IntSlice so that we can implement the sort.Interface
type IntSlice []int

// Len, Less, and Swap methods need to be implemented to conform to sort.Interface
func (s IntSlice) Len() int           { return len(s) }
func (s IntSlice) Less(i, j int) bool { return s[i] < s[j] }
func (s IntSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func main() {
    nums := IntSlice{4, 1, 3, 2}
    sort.Sort(nums)
    fmt.Println(nums) // [1 2 3 4]
}
```

To reverse the order, invert the comparison in the `Less` method and define a new type:

```go
import (
    "fmt"
    "sort"
)

// Define a custom IntSlice for descending order sorting.
type DescIntSlice []int

func (s DescIntSlice) Len() int           { return len(s) }
func (s DescIntSlice) Less(i, j int) bool { return s[i] > s[j] } // Inverted comp
func (s DescIntSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func main() {
    nums := DescIntSlice{4, 1, 3, 2}
    sort.Sort(nums)
    fmt.Println(nums) // [4 3 2 1]
}
```

Just reversing the order requires you to define a separate type and implement the three
methods again!

### Sorting a slice of structs by age

Here, we sort by the `Age` field in ascending order:

```go
import (
    "fmt"
    "sort"
)

type User struct {
    Name string
    Age  int
}

type ByAge []User

func (s ByAge) Len() int           { return len(s) }
func (s ByAge) Less(i, j int) bool { return s[i].Age < s[j].Age }
func (s ByAge) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func main() {
    users := ByAge{
        {"Alice", 32},
        {"Bob", 27},
        {"Carol", 40},
    }
    sort.Sort(users)
    fmt.Println(users) // [{Bob 27} {Alice 32} {Carol 40}]
}
```

Reversing the comparison sorts in descending order:

```go
import (
    "fmt"
    "sort"
)

type User struct {
    Name string
    Age  int
}

type ByAgeDesc []User

func (s ByAgeDesc) Len() int           { return len(s) }
func (s ByAgeDesc) Less(i, j int) bool { return s[i].Age > s[j].Age }
func (s ByAgeDesc) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func main() {
    users := ByAgeDesc{
        {"Alice", 32},
        {"Bob", 27},
        {"Carol", 40},
    }
    sort.Sort(users)
    fmt.Println(users) // [{Carol 40} {Alice 32} {Bob 27}]
}
```

Although `sort.Interface` can handle just about any sorting logic, you must create a new
custom type (or significantly modify an existing one) each time you want to sort a different
slice or the same slice in a different way. It's powerful but verbose, and can be cumbersome
to maintain if you have many different sorts in your code.

## Using sort.Slice

Go 1.8 introduced `sort.Slice` to minimize the amount of boilerplate needed for sorting.
Instead of creating a new type and implementing three methods, you provide an inline
comparison function that receives the two indices you're comparing.

### Sorting a slice of float64

Here's a simple example that sorts floats in ascending order:

```go
import (
    "fmt"
    "sort"
)

func main() {
    floats := []float64{2.5, 0.1, 3.9, 1.2}
    sort.Slice(floats, func(i, j int) bool {
        return floats[i] < floats[j]
    })
    fmt.Println(floats) // [0.1 1.2 2.5 3.9]
}
```

Inverting the comparison sorts them in descending order:

```go
import (
    "fmt"
    "sort"
)

func main() {
    floats := []float64{2.5, 0.1, 3.9, 1.2}
    sort.Slice(floats, func(i, j int) bool {
        return floats[i] > floats[j]  // Reverse the comp
    })
    fmt.Println(floats) // [3.9 2.5 1.2 0.1]
}
```

### Sorting a slice of structs by age

For structs, the inline comparator can access struct fields:

```go
import (
    "fmt"
    "sort"
)

type User struct {
    Name string
    Age  int
}

func main() {
    users := []User{
        {"Alice", 32},
        {"Bob", 27},
        {"Carol", 40},
    }
    sort.Slice(users, func(i, j int) bool {
        return users[i].Age < users[j].Age
    })
    fmt.Println(users) // [{Bob 27} {Alice 32} {Carol 40}]
}
```

Switching `>` for `<` will reverse the sort:

```go
import (
    "fmt"
    "sort"
)

type User struct {
    Name string
    Age  int
}

func main() {
    users := []User{
        {"Alice", 32},
        {"Bob", 27},
        {"Carol", 40},
    }
    sort.Slice(users, func(i, j int) bool {
        return users[i].Age > users[j].Age
    })
    fmt.Println(users) // [{Carol 40} {Alice 32} {Bob 27}]
}
```

While `sort.Slice` is much simpler than `sort.Interface`, it's still not strictly type-safe:
the `slice` parameter is defined as an `interface{}`, and you provide a comparator that uses
indices. Go won't necessarily stop you from doing something incorrect in the comparison at
compile time.

For example, this code compiles but will panic at runtime because `other` is referenced
inside the comparator of a different slice `ints`, and the indices `i` or `j` can go out of
bounds in `other`:

```go
import (
    "fmt"
    "sort"
)

func main() {
    ints := []int{3, 1, 2}
    other := []int{10, 20}
    sort.Slice(ints, func(i, j int) bool {
        // Using 'other' here compiles, but i or j might be out of range.
        return other[i] < other[j]
    })
    fmt.Println(ints)
}
```

You won't find out you've made a mistake until runtime, when a panic occurs. There is no
compiler-enforced guarantee that the `func(i, j int) bool` actually compares two values of
the intended slice.

**Note**: In `sort.Slice`, the comparison function parameters `i` and `j` are _indices_.
Inside the function, you must reference `slice[i]` and `slice[j]` to get the actual elements
being compared.

## Using generics with the slices package

Go 1.21 introduced the `slices` package, which provides generic sorting functions. These new
functions combine the convenience of `sort.Slice` with the ability to detect type errors at
compile time. For basic numeric or string slices that satisfy Go's “ordered” constraints,
you can just call `slices.Sort`. For more complex or custom sorting, `slices.SortFunc`
accepts a comparator function that returns an integer (negative if `a < b`, zero if they're
equal, and positive if `a > b`).

### Sorting primitive slices

When you're dealing with basic types like `int`, `float64`, or `string`, you can sort them
immediately using `slices.Sort`, which arranges them in ascending order:

```go
import (
    "fmt"
    "slices"
)

func main() {
    ints := []int{4, 1, 3, 2}
    floats := []float64{2.5, 0.1, 3.9, 1.2}

    slices.Sort(ints)
    slices.Sort(floats)

    fmt.Println(ints)   // [1 2 3 4]
    fmt.Println(floats) // [0.1 1.2 2.5 3.9]
}
```

For descending order, you can use `slices.SortFunc` and invert the usual comparison:

```go
import (
    "fmt"
    "slices"
)

func main() {
    ints := []int{4, 1, 3, 2}
    floats := []float64{2.5, 0.1, 3.9, 1.2}

    slices.SortFunc(ints, func(a, b int) int {
        switch {
        case a > b:
            return -1
        case a < b:
            return 1
        default:
            return 0
        }
    })

    slices.SortFunc(floats, func(a, b float64) int {
        switch {
        case a > b:
            return -1
        case a < b:
            return 1
        default:
            return 0
        }
    })

    fmt.Println(ints)   // [4 3 2 1]
    fmt.Println(floats) // [3.9 2.5 1.2 0.1]
}
```

### Sorting a slice of structs by age

When dealing with more complex structures, you can define precisely how two elements should
be compared:

```go
import (
    "fmt"
    "slices"
)

type User struct {
    Name string
    Age  int
}

func main() {
    users := []User{
        {"Alice", 32},
        {"Bob", 27},
        {"Carol", 40},
    }
    slices.SortFunc(users, func(a, b User) int {
        return a.Age - b.Age
    })
    fmt.Println(users) // [{Bob 27} {Alice 32} {Carol 40}]
}
```

To reverse the order, invert the numerical comparison:

```go
import (
    "fmt"
    "slices"
)

type User struct {
    Name string
    Age  int
}

func main() {
    users := []User{
        {"Alice", 32},
        {"Bob", 27},
        {"Carol", 40},
    }
    slices.SortFunc(users, func(a, b User) int {
        switch {
        case a.Age > b.Age:
            return -1
        case a.Age < b.Age:
            return 1
        default:
            return 0
        }
    })
    fmt.Println(users) // [{Carol 40} {Alice 32} {Bob 27}]
}
```

**Note**: Unlike `sort.Slice`, which passes **indices** to the comparison function,
`slices.SortFunc` passes the **actual elements** (`a` and `b`) to your comparator. Moreover,
the comparator must return an `int` (negative, zero, or positive), rather than a boolean.

### Compile-time safety

One of the major benefits of the `slices` package is compile-time type safety, which you
don't get with `sort.Sort` or `sort.Slice`. Those older APIs use `interface{}` parameters or
index-based comparators and don't strictly verify that your comparator operates on the right
types.

As shown previously, you can accidentally reference a different slice in the comparator and
your code will compile but crash at runtime. By contrast, `slices.Sort` and
`slices.SortFunc` are fully generic. The compiler enforces that you pass a slice of a valid
type (e.g., `[]int`, `[]string`, or a custom struct slice), and that your comparator's
signature matches the element type. This means you get errors at compile time instead of at
runtime.

For instance, if you attempt to pass an array instead of a slice:

```go
import "slices"

func main() {
    arr := [4]int{10, 20, 30, 40}
    // compile-time error: cannot use arr (value of type [4]int) as type []int
    slices.Sort(arr)
}
```

Go will refuse to compile this code because `arr` is not a slice. Similarly, if your
comparator for `slices.SortFunc` returns a type other than `int`, the compiler will produce
an error. This helps you detect mistakes immediately, rather than discovering them in
runtime.

For a practical illustration, consider sorting a slice by a case-insensitive string field:

```go
import (
    "fmt"
    "slices"
    "strings"
)

type Animal struct {
    Name    string
    Species string
}

func main() {
    animals := []Animal{
        {"Bob", "Giraffe"},
        {"alice", "Zebra"},
        {"Dave", "Elephant"},
    }

    // Sort by Name, ignoring case
    slices.SortFunc(animals, func(a, b Animal) int {
        aLower := strings.ToLower(a.Name)
        bLower := strings.ToLower(b.Name)
        switch {
        case aLower < bLower:
            return -1
        case aLower > bLower:
            return 1
        default:
            return 0
        }
    })

    fmt.Println(animals)
    // Output: [{alice Zebra} {Bob Giraffe} {Dave Elephant}]
}
```

Because your comparator expects an `Animal` for both `a` and `b`, you can't accidentally
compare two different types or reference the wrong fields without hitting a compile-time
error.

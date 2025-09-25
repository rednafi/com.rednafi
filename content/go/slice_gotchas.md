---
title: Go slice gotchas
date: 2025-02-06
slug: slice-gotchas
aliases:
    - /go/slice_gotchas/
tags:
    - Go
---

Just like any other dynamically growable container structure, Go slices come with a few
gotchas. I don't always remember all the rules I need to be aware of. So this is an attempt
to list some of the most common mistakes I've made at least once.

## Slices are views over arrays

In Go, a slice is a lightweight wrapper around an array. Instead of storing data itself, it
keeps track of three things: a pointer to an underlying array where the data is stored, the
number of elements it currently holds, and the total capacity before it needs more space.
The Go runtime defines it like this:

```go
// src/runtime/slice.go
type slice struct {
    array unsafe.Pointer // pointer to data array
    len   int            // slice length
    cap   int            // slice capacity
}
```

When you create a slice from an array or another slice, Go doesn't copy the data—it simply
points to a section of the existing array.

```
Slice Header              Underlying Array
+-------------+           +-------------------+
| array ------>|--------->| (data in memory)  |
| len         |           +-------------------+
| cap         |
+-------------+
```

This makes slices efficient. Passing a slice by value doesn't mean copying all its
elements—only the small slice struct gets copied, while the data stays where it is. But this
behavior is also the source of much confusion. The next sections cover some common pitfalls.

## Sliced slices share the underlying array

Reslicing a slice doesn't copy data. The newly created slices point to the same array. So
modifying one slice will affect others.

```go
// Define the original slice
original := []int{1, 2, 3, 4, 5}  // -> original: [1 2 3 4 5]

// Create slice1 from index 1 to 4
slice1 := original[1:4]          // -> slice1: [2 3 4]

// Create slice2 from index 2 to the end
slice2 := original[2:]           // -> slice2: [3 4 5]

// Modify the first element of slice1 (affects other slices)
slice1[0] = 100

// -> original: [1 100 3 4 5], slice1: [100 3 4], slice2: [3 4 5]
```

**Solution:** To get independent slices, you need to explicitly copy the data. Use `make` to
create a new slice and `copy` to transfer the elements.

```go
// Define the original slice
original := []int{1, 2, 3, 4, 5}  // -> [1 2 3 4 5]

// Create a new slice (slice1) from original[1:4]
slice1 := make([]int, len(original[1:4])) // -> [0 0 0]
copy(slice1, original[1:4])              // -> [2 3 4]

// Create a new slice (slice2) from original[2:]
slice2 := make([]int, len(original[2:])) // -> [0 0 0]
copy(slice2, original[2:])              // -> [3 4 5]

// Modify the first element of slice1 (doesn't affect others)
slice1[0] = 100 // -> original: [1 2 3 4 5], slice1: [100 3 4], slice2: [3 4 5]
```

## Append may reallocate

`append` reallocates the underlying array if capacity is insufficient, changing the backing
array pointer.

When passing slices to functions, reallocation inside the function won't update the original
slice header in the caller _unless_ the slice is returned and reassigned. Modifications
within the capacity _are_ visible.

If you create a slice with a predefined capacity and start appending elements, everything
looks fine until you exceed that capacity. Once that happens, Go reallocates memory and
moves the slice to a new backing array.

```go
// Create a slice with length=0 and capacity=3
slice := make([]int, 0, 3) // Let's say the array pointer is p1

// Append 3 elements (1,2,3) to fill up capacity
slice = append(slice, 1, 2, 3) // -> still pointer p1, slice: [1 2 3]

// Exceed capacity by appending 4
slice = append(slice, 4) // -> new pointer p2, slice: [1 2 3 4]
```

The same behavior applies when passing a slice to a function. If the function modifies
elements within the allocated capacity, those changes persist and are visible from outside
the function. But if `append` triggers a reallocation inside the function, the caller's
slice remains unchanged.

```go
// Demonstration function that modifies and appends
func modifySlice(s []int) {
    s[0] = 99 // modification within capacity is visible
    s = append(s, 100) // may trigger reallocation
    // s pointer might change here, but the caller won't see that
}

// Example usage
mySlice := make([]int, 1, 3) // -> [0], capacity=3
mySlice[0] = 1               // -> [1]

modifySlice(mySlice)
// -> mySlice[0] becomes 99 (within capacity)
// -> the append inside function might reallocate, but that reallocated
// version is lost

// mySlice is effectively [99], capacity still = 3
// (the "100" appended is not in mySlice)
```

**Solution:** If `append` inside a function reallocates memory, the caller won't see the
change. To make it explicit, return the modified slice and reassign it.

```go
// Correct approach: return the new slice
func modifySliceCorrected(s []int) []int {
    s = append(s, 100) // may reallocate
    return s           // return the updated slice
}

// Example usage
mySlice := make([]int, 1, 3) // -> [0], cap=3
mySlice[0] = 1               // -> [1]

mySlice = modifySliceCorrected(mySlice)
// -> now mySlice sees the appended element [1 100]
```

## Append returns new slice

`append` returns a _new_ slice. If you don't reassign the result back to the original slice
variable, the slice remains unchanged after the `append` operation. We already saw this in
last section but I think it deserves a section of its own.

```go
slice := []int{1, 2, 3}  // -> [1 2 3]

// Wrong usage (no reassign):
append(slice, 4)
// -> appended result is discarded, slice remains [1 2 3]

// Correct usage (assign back):
slice = append(slice, 4)
// -> slice is now [1 2 3 4]
```

**Solution:** Remember to always assign the return value of `append` back to the slice
variable you are working with.

```go
slice := []int{1, 2, 3}       // -> [1 2 3]
slice = append(slice, 4, 5, 6) // -> [1 2 3 4 5 6]
```

## Nil and empty slices differ

Nil slices have `nil` array pointers; empty slices have initialized, non-nil pointers and
zero length. While often interchangeable for emptiness checks, the distinction matters in
certain contexts like JSON encoding or API interactions.

```go
var nilSlice []int                 // -> nil
emptySliceMake := make([]int, 0)   // -> []
emptySliceLiteral := []int{}       // -> []

// nilSlice == nil -> true
// emptySliceMake == nil -> false
// emptySliceLiteral == nil -> false
```

**Solution:** When you need a truly empty slice (e.g., to represent an empty list in JSON),
initialize it as an empty slice (e.g., `[]int{}` or `make([]int, 0)`). For general emptiness
checks, `len(slice) == 0` works for both nil and empty slices.

```go
var nilSlice []int               // nil slice (pointer is nil)
emptySlice := []int{}            // empty slice (pointer is non-nil)

nilJSON, _ := json.Marshal(nilSlice)   // -> "null"
emptyJSON, _ := json.Marshal(emptySlice) // -> "[]"
```

## Slicing can leak memory

Small slices created from large arrays can keep the entire large array in memory.

```go
// Suppose we have a function returning a large slice
func getLargeSlice() []int {
    largeSlice := make([]int, 1_000_000) // large underlying array
    return largeSlice
}

// Usage example:
largeData := getLargeSlice()     // -> slice of 1,000,000 ints
smallSlice := largeData[10:20]   // -> slice with length=10, cap=999,990

// Setting largeData to nil does not free the large array,
// because smallSlice still references it.
largeData = nil
// The memory for the big array won't be garbage collected
// due to the reference from smallSlice.
```

**Solution:** To avoid memory leaks, copy the data of the small slice into a new,
independent slice. This allows the large underlying array to be garbage collected if no
longer referenced elsewhere.

```go
func getLargeSlice() []int {
    largeSlice := make([]int, 1_000_000)
    return largeSlice
}

// Usage example:
largeData := getLargeSlice()
subset := largeData[10:20]               // -> references big array
smallSlice := make([]int, len(subset))   // -> new small array
copy(smallSlice, subset)                // -> copies only 10 elements

largeData = nil
// Now only smallSlice references a small array (cap=10)
// The large array is eligible for GC.
```

## Range copies values

`for...range` on value types iterates over _copies_. Modifications to the loop variable
don't change the original slice.

```go
slice := []int{1, 2, 3} // -> [1 2 3]

// "val" is a copy of each element in the slice
for _, val := range slice {
    val *= 2 // modifies only "val," not slice
}
// slice remains [1 2 3]

// Using an index-based loop:
for i := range slice {
    slice[i] *= 2 // modifies the element in place
}
// slice is now [2 4 6]
```

**Solution:** If you need to modify slice elements during iteration, use an index-based
`for` loop. This provides direct access to each element via its index.

```go
slice := []int{1, 2, 3} // -> [1 2 3]

for i := range slice {
    slice[i] *= 2 // modifies the original slice
}
// slice is now [2 4 6]
```

## Make with length initializes

`make([]T, length, capacity)` initializes the first `length` elements with the zero value of
`T`. This can be a subtle point if you expect an uninitialized slice of a certain size.

```go
slice := make([]int, 3, 5) // -> [0 0 0], cap=5
// The first 3 elements are zero-initialized

slice[0] = 10 // -> [10 0 0]
slice = append(slice, 1, 2) // -> [10 0 0 1 2], len=5, cap=5

emptySliceCap := make([]int, 0, 5) // -> [], cap=5
// This one starts with length=0, so no initial elements
```

**Solution:** If you want an empty slice with a specific capacity but _without_ initial zero
values, use `make([]T, 0, capacity)`. Or use the slice literal `[]T{}` syntax if you don't
care about the capacity.

If you need a slice of a certain length initialized with zero values,
`make([]T, length, capacity)` is the correct approach.

```go
emptySliceWithCap := make([]int, 0, 5) // -> [], cap=5
initializedSlice := make([]int, 3, 5)  // -> [0 0 0], cap=5
```

## Overlapping copy is tricky

`copy(dst, src)` with overlapping slices can corrupt data when `dst` starts inside `src`.

```go
data := []int{1, 2, 3, 4, 5} // -> [1 2 3 4 5]
src := data[:]               // -> [1 2 3 4 5]
dst := data[2:]              // -> overlap (dst starts at index 2): [3 4 5]

// Copy from src to dst
copy(dst, src)

// Expected output: data -> [1 2 3 4 5] (if copied correctly)
// Actual output: data -> [1 2 1 2 3] (corrupted)
```

**Solution:** To avoid corruption, just don't do it. If you have to, then one way to fix it
is by using a temporary buffer. Even then it's messy.

```go
data := []int{1, 2, 3, 4, 5}
src := data[:]
dst := make([]int, len(src)-2) // Create dst as a NEW slice, shorter than src

// Use a temporary buffer
temp := make([]int, len(src))

// Copy from src to temp
copy(temp, src)

// Copy from temp to src
copy(dst, temp[2:])

// Expected output: data -> [1 2 3 4 5] (data remains unchanged)
// Actual output: data -> [1 2 3 4 5]
// dst -> [3 4 5] (dst is a copy of the last part of src)
```

## Copy truncates silently

`copy` also returns the number of elements copied, which is the smaller of `len(dst)` and
`len(src)`. If `dst` is shorter, data gets truncated.

```go
src := []int{1, 2, 3, 4, 5}  // -> [1 2 3 4 5]
dst := make([]int, 3)        // -> [0 0 0] (length 3)

copied := copy(dst, src)

// Expected output: dst -> [1 2 3 4 5], copied -> 5
// Real output:     dst -> [1 2 3], copied -> 3
```

**Solution:** On `dst`, always set the length from the `src` while copying.

```go
src := []int{1, 2, 3, 4, 5}  // -> [1 2 3 4 5]
dst := make([]int, len(src)) // -> [0 0 0 0 0] (length 5)

copied := copy(dst, src)

// Expected output: dst -> [1 2 3 4 5], copied -> 5
// Real output:     dst -> [1 2 3 4 5], copied -> 5
```

I may have missed, forgotten, or not yet encountered a few other gotchas. If you've run into
any that aren't listed here, I'd love to hear about them.

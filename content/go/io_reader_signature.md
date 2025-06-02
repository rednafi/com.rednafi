---
title: Why does Go's io.Reader have such a weird signature?
date: 2025-02-08
tags:
    - Go
    - TIL
---

I've always found the signature of `io.Reader` a bit odd:

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}
```

Why take a byte slice and write data into it? Wouldn't it be simpler to create the slice
inside `Read`, load the data, and return it instead?

```go
// Hypothetical; what I *thought* it should be
Read() (p []byte, err error)
```

This felt more intuitive to me—you call `Read`, and it gives you a slice filled with data,
no need to pass anything.

I found out why it's designed this way while watching this excellent [GopherCon Singapore
talk] on understanding allocations by Jacob Walker. It mainly boils down to two reasons.

## Reducing heap allocations

If `Read` created and returned a new slice every time, the memory would always end up on the
heap.

Heap allocations are slower because they require garbage collection, while stack allocations
are faster since they are freed automatically when a function returns. By taking a
caller-provided slice, `Read` lets the caller control memory and reuse buffers, keeping them
on the stack whenever possible.

This matters a lot when reading large amounts of data. If each `Read` call created a new
slice, you'd constantly be allocating memory, leading to more work for the garbage
collector. Instead, the caller can allocate a buffer once and reuse it across multiple
reads:

```go
buf := make([]byte, 4096) // Single allocation
n, err := reader.Read(buf) // Read into existing buffer
```

Go's escape analysis tool (`go build -gcflags=-m`) can confirm this. If `Read` returned a
new slice, the tool would likely show:

```txt
buf escapes to heap
```

meaning Go has to allocate it dynamically. But by reusing a preallocated slice, we avoid
unnecessary heap allocations—only if the buffer is small enough to fit in the stack. How
small? Only the compiler knows, and you shouldn't depend on it. Use the escape analysis tool
to see that. But most of the time, you don't need to worry about this at all.

## Reusing buffers in streaming

The second issue is correctness. When reading from a stream, you usually call `Read`
multiple times to get all the data. If `Read` returned a fresh slice every time, you'd have
no control over memory usage across calls. Worse, you couldn't efficiently handle partial
reads, making buffer management unpredictable.

With the hypothetical version of `Read`, every call would allocate a new slice. If you
needed to read a large stream of data, you'd have to manually piece everything together
using `append`, like this:

```go
var allData []byte
for {
    buf, err := reader.Read() // New allocation every call
    if err != nil {
        break
    }
    allData = append(allData, buf...) // Growing slice every time, more allocation
}
process(allData)
```

This is a mess. Every time `append` runs out of space, Go will have to allocate a larger
slice and copy the existing data over, piling on unnecessary GC pressure.

By contrast, `io.Reader`'s actual design avoids this problem:

```go
buf := make([]byte, 4096) // Allocate once
for {
    n, err := reader.Read(buf)
    if err != nil {
        break
    }
    process(buf[:n])
}
```

This avoids unnecessary allocations and produces less garbage for the GC to clean up.

<!-- Resources -->
<!-- prettier-ignore-start -->

<!-- understanding allocations: the stack and the heap - gophercon sg 2019 -->
[gophercon singapore talk]:
    https://www.youtube.com/watch?v=ZMZpH4yT7M0

<!-- prettier-ignore-end -->

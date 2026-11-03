---
title: "Accepted proposal: grow an existing map with maps.Grow"
slug: go-maps-grow
date: 2026-11-03
description: >-
    The maps package is getting a Grow function that presizes a map you already have, and
    maps.Copy will start doing it for you.
tags:
    - Go
    - Performance
    - API
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /shards/2026/11/go-maps-grow/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mzgbzzisrs2q"
---

Liz Fong-Jones's [proposal to add maps.Grow] was [accepted] on October 29. It gives you a
way to make room in a map that already exists, much like `make(M, n)` does for a new one.

A Go map grows on its own as you add entries. Each time it runs out of room, the runtime
allocates bigger storage, rehashes the entries into it, and leaves the old storage for the
garbage collector. One grow is cheap, but a loop that adds thousands of entries can trigger
many of them, and each one burns CPU and creates more garbage. If you know the final size
when you create the map, you pass it to `make` and skip all that extra work.

But that only works if you're the one creating the map. If someone else created it and
you're only adding entries, you're stuck with however much room it has.

`net/http/httputil.ReverseProxy` hits this on every request. It copies the upstream response
headers into the client's response:

```go
func copyHeader(dst, src http.Header) {
    for k, vv := range src {
        for _, v := range vv {
            dst.Add(k, v)
        }
    }
}
```

The proxy calls this as `copyHeader(rw.Header(), res.Header)`. `rw` is the
`http.ResponseWriter`, and `rw.Header()` returns the header map that the server will write
back to the client. The server created that map before the proxy ever saw it. The proxy
knows how many headers are coming but can't pass that number to the map, so the map may grow
several times inside this loop.

The usual workaround is to build a bigger map yourself and swap it in. [libhoney-go] does
this. It keeps an event's fields in a map and hands that map out through a `Fields()`
method. A simplified version looks like this:

```go {hl_lines=["13","15"]}
type fieldHolder struct {
    data map[string]any
}

// Fields returns a reference to the event's field map.
func (f *fieldHolder) Fields() map[string]any {
    return f.data
}

func (f *fieldHolder) AddFields(src map[string]any) {
    if len(src) > len(f.data) {
        m := make(map[string]any, len(f.data)+len(src))
        maps.Copy(m, f.data) // (1)
        maps.Copy(m, src)
        f.data = m // (2)
        return
    }
    maps.Copy(f.data, src)
}
```

Here:

- (1) every existing field gets copied again just to move it into the bigger map
- (2) `f.data` now points to a new map, so anyone who called `Fields()` earlier still holds
  the old one and won't see the new fields

Even with the extra copy, [that change] cut CPU use by about 5% in production. Copying
everything once was cheaper than letting the map grow step by step.

With `maps.Grow`, you don't need the swap:

```go
// Grow increases m's capacity, if necessary, so that at least n more
// entries can be added without further growth in the typical case, as if
// m had been created with make(M, len(m)+n). Grow never shrinks m.
func Grow[M ~map[K]V, K comparable, V any](m M, n int)
```

The proxy can make room before the loop starts:

```go {hl_lines=["2"]}
func copyHeader(dst, src http.Header) {
    maps.Grow(dst, len(src))
    for k, vv := range src {
        for _, v := range vv {
            dst.Add(k, v)
        }
    }
}
```

And libhoney can grow `f.data` in place. Callers who got the map from `Fields()` still hold
the same map, so they see the new fields:

```go {hl_lines=["2"]}
func (f *fieldHolder) AddFields(src map[string]any) {
    maps.Grow(f.data, len(src))
    maps.Copy(f.data, src)
}
```

`maps.Grow` returns nothing, while `slices.Grow` returns a new slice. The reason is what
each variable holds:

- A slice variable holds a small [slice struct] with a pointer to the backing array, a
  length, and a capacity. `slices.Grow` allocates a bigger array, copies the elements over,
  and returns a new slice that points to it. That's why you write `s = slices.Grow(s, n)`.
- A map variable holds a pointer to a [maps.Map] struct in the runtime. `maps.Grow` puts
  bigger storage behind that same pointer, and every variable holding the map sees it.
  There's nothing to return.

A nil map is a nil pointer with no storage behind it, so `Grow` panics if you ask it to add
room:

```go
var m map[string]int
maps.Grow(m, 0)  // fine, nothing to add
maps.Grow(m, 10) // panic: assignment to entry in nil map
maps.Grow(m, -1) // panics, n is negative
```

If you don't have a map yet, use `make`.

The `n` you pass is a hint, just like the size you give `make`. The doc comment calls `Grow`
best effort, so the map may still grow later. You also can't ask a map for its capacity. An
earlier proposal to allow [cap on maps] was declined.

> [!NOTE]
>
> With a slice, `cap(s)` tells you exactly how far you can append before a new array gets
> allocated. A map can't tell you that. Where an entry lands depends on its hash, and the
> runtime grows the map before every slot is full. So `cap(m)` could only be a rough guess.
> That's why `Grow` takes a hint but never reports a number back.

The proposal also changes `maps.Copy`.

> [!IMPORTANT]
>
> `maps.Copy(dst, src)` will make room for `len(src)` entries in `dst` before it starts
> copying. That never wastes space, because `dst` ends up with at least that many entries
> however the keys overlap. Code that already calls `maps.Copy` gets faster without any
> change. In the [benchmarks], copying into an empty or small map took about half the time
> on an AMD 5600X and allocated about half as much memory.

When you add entries without `maps.Copy`, you still call `Grow` yourself. The header loop
above uses `Header.Add`, so it's one example. Filling a map from a slice is another:

```go {hl_lines=["2"]}
func index(dst map[string]User, users []User) {
    maps.Grow(dst, len(users))
    for _, u := range users {
        dst[u.ID] = u
    }
}
```

If most of those users are already in `dst`, the extra room goes unused and you've spent
memory for nothing.

The [implementation], the [Copy change], and a [follow-up] that uses `maps.Grow` in
`httputil` and `cgi` are all still under review. The change is expected to land in Go 1.28.

<!-- references -->
<!-- prettier-ignore-start -->

[proposal to add maps.Grow]:
    https://github.com/golang/go/issues/81698

[accepted]:
    https://github.com/golang/go/issues/81698#issuecomment-6507741693

[libhoney-go]:
    https://github.com/honeycombio/libhoney-go/blob/02e9dbf361012fafbe8698f429b65630500dc10a/libhoney.go#L642-L658

[that change]:
    https://github.com/honeycombio/libhoney-go/pull/279

[slice struct]:
    https://github.com/golang/go/blob/go1.27.2/src/runtime/slice.go#L16-L20

[maps.Map]:
    https://github.com/golang/go/blob/go1.27.2/src/internal/runtime/maps/map.go#L214-L265

[cap on maps]:
    https://github.com/golang/go/issues/52157#issuecomment-1098330894

[benchmarks]:
    https://go-review.googlesource.com/c/go/+/839065

[implementation]:
    https://go-review.googlesource.com/c/go/+/819620

[Copy change]:
    https://go-review.googlesource.com/c/go/+/839065

[follow-up]:
    https://go-review.googlesource.com/c/go/+/856621

<!-- prettier-ignore-end -->

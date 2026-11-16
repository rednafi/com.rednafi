---
title: "Accepted proposal: omit the type in composite literals"
slug: go-untyped-composite-literals
date: 2026-11-16
description: >-
    Go 1.28 is set to let you drop the type from a composite literal when the compiler can
    infer it from the place where the value goes.
tags:
    - Go
    - Language
    - API
    - "Proposal Review"
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /shards/2026/11/go-untyped-composite-literals/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3n2hbb2vv362p"
---

Damien Neil's [proposal for type-inferred composite literals] was [accepted] back in April.
He filed it in October 2015, so it took more than ten years and about 260 comments to get
there.

It's now on track for Go 1.28, and it lets you drop the type of a composite literal when Go
already knows what that type must be.

You can already do this inside slice, array, and map literals. Go gets the type from the
outer literal, so you don't have to repeat it:

```go
// Slices and arrays
points := []Point{{1, 2}, {3, 4}}
corners := [2]Point{{0, 0}, {1, 1}}

// Map keys and values
byName := map[string]Point{"origin": {0, 0}}
byPoint := map[Point]string{{0, 0}: "origin"}

// Pointers: {1, 2} means &Point{1, 2}
ptrs := []*Point{{1, 2}}
```

Everywhere else, you have to write the type, and that gets noisy when the surrounding code
makes it obvious.

Take a function that returns a zero value with an error. The signature says the first result
is a `time.Time`, but the return statement has to say it again:

```go {hl_lines=["3"]}
func parseTime(s string) (time.Time, error) {
    if s == "" {
        return time.Time{}, errors.New("empty timestamp")
    }
    return time.Parse(time.RFC3339, s)
}
```

Options structs have the same problem. `slog.NewJSONHandler` only accepts a
`*slog.HandlerOptions`, so writing the type in the call adds nothing:

```go {hl_lines=["1"]}
h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
})
```

A struct field has a declared type just like a slice element does, but it doesn't get the
same shortcut. Go knows that `TLSConfig` is a `*tls.Config`, and you still have to say it:

```go {hl_lines=["3"]}
srv := &http.Server{
    Addr:      ":8080",
    TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13},
}
```

Anonymous structs are the worst case. Most people give up and name the struct, even when
they use the name only once. Otherwise, every send on a channel of that struct has to repeat
the whole definition:

```go {hl_lines=["6-9"]}
results := make(chan struct {
    val string
    err error
}, 1)

results <- struct {
    val string
    err error
}{val: "ok"}
```

With the proposal, all of these get shorter:

```go {hl_lines=["3","8","12","19"]}
func parseTime(s string) (time.Time, error) {
    if s == "" {
        return {}, errors.New("empty timestamp")
    }
    return time.Parse(time.RFC3339, s)
}

h := slog.NewJSONHandler(os.Stderr, {Level: slog.LevelDebug})

srv := &http.Server{
    Addr:      ":8080",
    TLSConfig: {MinVersion: tls.VersionTLS13},
}

results := make(chan struct {
    val string
    err error
}, 1)
results <- {val: "ok"}
```

The `slog` and `TLSConfig` lines also lost their `&`. Both places expect a pointer, so Go
reads `{...}` as `&T{...}`. Slice literals already work this way, which is why the inner
`{1, 2}` in `[]*Point{{1, 2}}` meant `&Point{1, 2}` earlier.

> [!IMPORTANT]
>
> You can drop the type from a composite literal whenever Go already knows which type the
> value must have. In `return {}, err`, the function signature tells Go the type. In
> `TLSConfig: {...}`, the field does.
>
> The core of the spec change is one new rule in [assignability]. It lets you use `{...}`
> where Go expects a `T` if `T{...}` is a valid literal. The composite literal section
> extends this to pointers to `T`.

Go knows the type in all of these places:

```go
var p Point = {1, 2}        // variable with a declared type
p = {3, 4}                  // assignment
draw({1, 2})                // function argument
ch <- {1, 2}                // channel send
line := Line{Start: {0, 0}} // struct field
return {}, err              // return value
```

You get a compile error when Go can't work out the type, or when you add the `&` yourself:

```go
q := {1, 2}                    // missing type in composite literal
var a any = {1, 2}             // invalid composite literal type any
json.Marshal({Name: "gopher"}) // invalid composite literal type any
var p *Point = &{1, 2}         // missing type in composite literal
```

The `:=` form creates a new variable, so there's no declared type to take from. A variable
or parameter of type `any` accepts a value of any type, and Go won't guess which one you
meant.

The last line fails even though Go knows the type, because `&{...}` isn't allowed. Leave the
`&` off, and Go adds it when it expects a pointer.

Passing a bare `{...}` for a type parameter doesn't work either:

```go
var pts []Point
// invalid composite literal type E (no common underlying type)
i := slices.Index(pts, {3, 4})
```

You might expect Go to learn from `pts` that the element type is `Point`, but it doesn't. It
checks `{3, 4}` against the type parameter `E`, so you still have to write `Point{3, 4}`. A
parameter that is a generic struct such as `Box[T]` fails too, with a different error:
`cannot use 1 (untyped int constant) as T value in struct literal`.

Fixing the `Box[T]` case needs [type inference on generic structs], which [won't land in
this release].

I like that this cuts a lot of boilerplate. But if you take it too far, the code gets hard
to read, because a reader now has to work out each missing type the way the compiler does.

Most of the pushback near the end of the proposal thread was about this. To see how bad it
could get, Alan Donovan made a [CL that removes every type it can] from the `x/tools` repo,
and he [didn't like the result]:

> It is definitely lighter on the page, but I find it a whole lot harder to read.
>
> -- Alan Donovan

The proposal review group accepted the rule as is anyway, without limiting it to the cases
that always read well. Austin Clements [explained why]:

> Types should not be elided everywhere they possibly can, and that's okay. Trying to
> somehow restrict it to just the cases that are deemed always acceptable would have
> dramatically complicated the change and led to potentially difficult to explain and
> understand inconsistency.
>
> -- Austin Clements

So it's up to you when to drop the type. Drop it when the reader can see it nearby, like in
a return statement or a field with a clear name, and keep it when they'd have to look it up.

When `ch` is declared in another file, `ch <- pkg.Event{ID: id}` is easier to read than
`ch <- {ID: id}`.

I was also worried that we'd get a `go fix` rule that strips types everywhere it can. That
doesn't seem to be happening.

Damien Neil [made it clear] that the team doesn't want any tool to remove more types than it
does today, because nobody knows how to tell when removing one helps.

So `gofmt -s` keeps its current behavior. A pending gopls change lets the
`simplifycompositelit` analyzer also drop a nested struct field's type, as in `S{X: X{}}`,
but it still [never removes the outermost type]. If you end up reading code where the types
are gone, turn on the `compositeLiteralTypes` gopls [inlay hint] to see them in your editor.

The compiler work and the [spec change] are merged, and the feature is in the draft [Go 1.28
release notes]. That release is due in February 2027, and the feature can still be pulled
before then.

You can try it today with `gotip`, as long as your `go.mod` says `go 1.28` or later.

<!-- references -->
<!-- prettier-ignore-start -->

[proposal for type-inferred composite literals]:
    https://github.com/golang/go/issues/12854

[accepted]:
    https://github.com/golang/go/issues/12854#issuecomment-4256916067

[assignability]:
    https://tip.golang.org/ref/spec#Assignability

[type inference on generic structs]:
    https://github.com/golang/go/issues/61731

[won't land in this release]:
    https://github.com/golang/go/issues/12854#issuecomment-5893704168

[CL that removes every type it can]:
    https://go.dev/cl/740720

[didn't like the result]:
    https://github.com/golang/go/issues/12854#issuecomment-3826072900

[explained why]:
    https://github.com/golang/go/issues/12854#issuecomment-4226485837

[made it clear]:
    https://github.com/golang/go/issues/12854#issuecomment-4226900297

[never removes the outermost type]:
    https://github.com/golang/go/issues/12854#issuecomment-5004591012

[inlay hint]:
    https://github.com/golang/tools/blob/master/gopls/doc/inlayHints.md

[spec change]:
    https://go.dev/cl/838545

[Go 1.28 release notes]:
    https://tip.golang.org/doc/go1.28#language

<!-- prettier-ignore-end -->

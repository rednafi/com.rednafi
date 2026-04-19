---
title: "Accepted proposal: UUID in the Go standard library"
date: 2026-04-19
slug: go-uuid
tags:
    - Go
    - API
description: >-
  Notes on Go's newly accepted uuid proposal and the tradeoffs behind the API.
---

It's good to see that Go is finally getting `uuid` in the standard library. The [proposal]
was accepted on April 8. I hadn't been following the conversation in the thread and only
found out about it from [Cup o' Go episode 154].

[google/uuid] is usually one of the first extra imports I add to a Go service that talks to
a database, so this one has felt overdue for years.

Python, Java, and C# have had native UUID support for years. I often use `python -m uuid`
whenever I need a one-off UUIDv4, and [Python 3.14+] added `python -m uuid -u uuid7` to get
UUIDv7s. So Go's not having it in the stdlib has always felt odd to me.

The accepted API is much smaller than `google/uuid`. The thread landed on a package that
covers the common cases and leaves the rest alone.

```go
package uuid

type UUID [16]byte

func Parse(string) (UUID, error)
func MustParse(string) UUID

func New() UUID
func NewV4() UUID
func NewV7() UUID

func Nil() UUID
func Max() UUID

func (UUID) String() string
func (UUID) Compare(UUID) int
func (UUID) MarshalText() ([]byte, error)
func (UUID) AppendText([]byte) ([]byte, error)
func (*UUID) UnmarshalText([]byte) error
```

The package is `uuid`, not `crypto/uuid`, which makes more sense to me. The stdlib's
`type UUID [16]byte` matches `google/uuid`, so conversion basically requires a single cast.
`Parse` accepts the same string forms as `google/uuid`: dashed strings, braces, `urn:uuid:`
prefixes, and bare 32-character hex. If someone changes the import path and the code still
compiles, it should still run.

The API keeps generation and parsing front and center. It adds `NewV4()` and `NewV7()`,
keeps a plain `New()` for the common case, and leaves most of the inspection surface from
`google/uuid` out.

I had expected `New()` to resolve to v7 because that is where most UUID discussion has moved
over the last two years. Go went with v4 instead. v7 is better for insertion locality in
B-tree indexes, which is great if you're planning to use UUIDs as primary keys in Postgres
or MySQL. But it also puts creation time in the ID and makes ordering visible.

```go
id := uuid.New()   // v4 for now
pk := uuid.NewV7() // choose this on purpose
```

If your system benefits from v7, you'll call `NewV7()` directly.

There's no v1, v3, v5, v6, or v8 constructor. There's no `Version()`, `Time()`, `NodeID()`,
or other API for pulling fields back out of the bytes. The package is trying to set a common
UUID type for Go and cover the common cases instead of absorbing every feature from the
third-party packages.

There's already a follow-up issue asking if `Nil()` should become `Zero()`. I would have
picked `Zero()`, because `nil` means something specific in Go and `[16]byte` is not that.
But the [RFC] calls it Nil, the existing Go packages call it Nil, and the [rename issue] is
leaning toward decline.

The proposal is accepted, but there is no target Go release attached to it yet. But I'm
hoping that it lands on 1.27.

<!-- references -->
<!-- prettier-ignore-start -->

[proposal]:
    https://github.com/golang/go/issues/62026

[google/uuid]:
    https://github.com/google/uuid

[Python 3.14+]:
    https://docs.python.org/3/library/uuid.html#command-line-usage

[Cup o' Go episode 154]:
    https://cupogo.dev/episodes/a-nil-by-any-other-name

[RFC]:
    https://www.rfc-editor.org/rfc/rfc9562.html

[rename issue]:
    https://github.com/golang/go/issues/78612

<!-- prettier-ignore-end -->

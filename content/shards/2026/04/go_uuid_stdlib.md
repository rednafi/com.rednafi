---
title: UUIDs are coming to the Go stdlib
date: 2026-04-18
slug: go-uuid-stdlib
tags:
    - Go
description: >-
  Go's proposal review group accepted a new uuid package with v4 and v7
  generators. Here are the decisions that shaped the final API.
---

Finally. The Go proposal review group accepted [issue #62026], landing a top-level `uuid`
package in the standard library with generators for v4 and v7. According to [this rollup],
`github.com/google/uuid` is the second most popular third-party Go dependency, behind
testify and above x/crypto. For anyone writing a service that talks to a database,
`go get github.com/google/uuid` is one of the first things you type.

Every other runtime I regularly use has had this forever. [Python has `uuid`] in its stdlib
(I've been running `python -m uuid` to generate a quick v4 from the shell for years). .NET
has `Guid.NewGuid`, Java has `java.util.UUID`, JavaScript has `crypto.randomUUID`, Ruby has
`SecureRandom.uuid`. Go has had to import a library.

The full thread is 342 comments and touches nearly every imaginable design question. Here's
the shape of what got accepted and the notable fights along the way.

## The accepted API

```go
package uuid

type UUID [16]byte

func New() UUID          // currently a v4
func NewV4() UUID
func NewV7() UUID
func Nil() UUID
func Max() UUID

func Parse(s string) (UUID, error)
func MustParse(s string) UUID

func (u UUID)  String() string
func (u UUID)  Compare(v UUID) int
func (u UUID)  MarshalText() ([]byte, error)
func (u UUID)  AppendText(b []byte) ([]byte, error)
func (u *UUID) UnmarshalText(b []byte) error
```

There's also a `database/sql` change so drivers that don't handle `UUID` explicitly will
round-trip it as the canonical hex-and-dash string, the same way `time.Time` is handled
today. The prototype is at [CL 725602].

## Why only v4 and v7

rolandshoemaker posted an ecosystem usage breakdown of `google/uuid` that pinned almost
every later decision. v4 is 94% of usage, v1 is 4% (mostly people calling `NewUUID` without
specifically wanting v1), v7 is 1%, and everything else is rounding error. v3 and v5 are
cryptographically broken since they use MD5 and SHA1. v6 has "very little observed use in
the wild", and v8 is vendor-specific and doesn't need a stdlib generator. v5 got some
lobbying for legacy migrations and lost. Existing v5 UUIDs can still be parsed, printed, and
compared; you just can't generate them from the stdlib.

## `[16]byte`, not string, not `uint128`

Short debate. it512 argued for a `struct { hi, lo uint64 }` internal layout for SIMD, and
neild shut it down:

> RFC 9562 introduces UUIDs as "16 octets (128 bits) in size." A UUID is not an integer. The
> conventional way to represent 16 octets of data in Go is as `[16]byte`.

Picking `[16]byte` also makes migration from `google/uuid.UUID` a free cast since that
library already uses the same layout.

## Package name

Started as `crypto/uuid`. dylan-bourque pointed out that "crypto" only appears four times in
RFC 9562, and the review group agreed to move it out to a top-level `uuid`.

## Monotonicity for v7

The biggest architectural debate. sergeyprokhorenko lobbied hard for the PostgreSQL 18
approach where the generator never rolls its timestamp backwards even if the wall clock
does. neild chose the other direction:

> Given that some users do rely on UUIDv7s containing the current generation time, I think
> the correct choice is to prefer using the current system time over strict monotonicity.

The final doc promises `NewV7` sorts in increasing order "except when the system clock moves
backwards."

## No custom RNG and no `Generator` type

This was the longest single thread on the issue. soypat spent pages advocating for a
`Generator` function type or interface for dependency injection in tests. aclements closed
it with "We're not going to take the Generator approach. Randomness is not state." The Go
team has publicly regretted supporting user-provided randomness in crypto ([#70942]), and
`testing/cryptotest.SetGlobalRandom` covers the test case.

There's no `NewRandomFromReader`, no `SetRand`, no generic `Must`, no `IsZero`, no
`ParseBytes`. Generation doesn't return an error because generating cryptographic randomness
doesn't fail on any supported platform, and if it ever did the process would crash rather
than silently hand you a bad UUID.

## No introspection

No `Version`, no `Time`, no `NodeID`, no `ClockSequence`. RFC 9562 recommends treating UUIDs
as opaque after generation, and the usage data backed that up. neild:

> UUIDs are widely used and a fundamental building block for many other technologies. Once
> you've generated one, it's just 16 opaque bytes.

If you actually need introspection, `github.com/google/uuid` is still there.

## What's next

The proposal is accepted but unimplemented. The milestone is "Backlog", which in Go-speak
means some release after the next one. A closely related proposal, [#76319], which wanted to
put just `UUIDv4` and `UUIDv7` in `crypto/rand`, was closed in favor of this one on the
reasoning that `crypto/rand` shouldn't accumulate structured-identifier functionality. I'll
be switching projects over as soon as the `uuid` package lands.

<!-- references -->
<!-- prettier-ignore-start -->

[issue #62026]:
    https://github.com/golang/go/issues/62026

[this rollup]:
    https://blog.thibaut-rousseau.com/blog/the-most-popular-go-dependency-is/

[Python has `uuid`]:
    https://docs.python.org/3/library/uuid.html

[CL 725602]:
    https://go.dev/cl/725602

[#70942]:
    https://github.com/golang/go/issues/70942

[#76319]:
    https://github.com/golang/go/issues/76319

<!-- prettier-ignore-end -->

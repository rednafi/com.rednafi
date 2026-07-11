---
title: "GC shape stenciling in Go generics"
slug: gc-shape-stenciling
date: 2026-07-11
description: >-
    How Go's compiler shares generic function bodies by GC shape and uses dictionaries for
    the concrete types.
tags:
    - Go
    - TIL
    - Codegen
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /go/gc-shape-stenciling/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mqflmo5cu62o"
---

While going through the [Go generics proposal], I got curious about how the compiler
implements it. Compilers usually handle generics in one of two ways:

- With [full monomorphization], the compiler turns generic code into concrete, type-specific
  code. It generates a separate version for every set of type arguments the program uses.
  Rust works this way, and so do C++ templates.

- With [type erasure], the compiler keeps one shared version of the generic code and
  replaces the type parameters with a common type. Java erases them to `Object` or to their
  declared bounds.

Full monomorphization gives the compiler exact types for every generated function. It can
optimize each one like ordinary code, and the generic abstraction adds no runtime overhead.
The drawback is that every distinct set of type arguments can add another function body,
which increases [compile time and binary size]. Erasure is at the opposite end of the
spectrum. There is only one body to compile, but the concrete types are gone at runtime. The
program needs casts and boxing instead.

A small Rust program shows how full monomorphization generates all the concrete functions.
It calls the generic `identity` function once with a `u32` and once with a `u64`:

```rs
#[inline(never)]
fn identity<T>(value: T) -> T {
    value
}

fn main() {
    println!("{} {}", identity(42_u32), identity(42_u64));
}
```

Save it as `mono.rs`. [`rustc`'s `--emit` option] writes the LLVM intermediate
representation to `mono.ll`. The [`-C` flags] select optimization level zero, a single
codegen unit, and v0 symbol mangling. Then `rg` keeps only the generated `identity`
functions:

```sh
rustc mono.rs --emit=llvm-ir=mono.ll \
    -C opt-level=0 \
    -C codegen-units=1 \
    -C symbol-mangling-version=v0

rg -A5 '; mono::identity' mono.ll | rg -v 'Function Attrs|^--$'
```

On Rust 1.96.1, the relevant output is:

```llvm {hl_lines=["2","7"]}
; mono::identity::<u32>
define internal i32 @_RINvCs15VVTAbh19D_4mono8identitymEB2_(i32 %value) unnamed_addr #0 {
start:
  ret i32 %value
}
; mono::identity::<u64>
define internal i64 @_RINvCs15VVTAbh19D_4mono8identityyEB2_(i64 %value) unnamed_addr #0 {
start:
  ret i64 %value
}
```

The highlighted signatures show what happened. One function takes and returns `i32`. The
other uses `i64`. One generic source function became two concrete function bodies.

C++ templates do this too. During [template instantiation], the compiler creates a
specialization for each concrete set of arguments. When the same specialization appears in
several object files, [GCC's implementation] emits every copy and leaves it to the linker to
collapse the duplicates.

Java keeps one implementation instead. `Box<String>` and `Box<Integer>` both run as the same
`Box` class. An unbounded `T` erases to `Object`, so a field declared as `T` is stored as an
`Object` in bytecode. When code reads that field from a `Box<String>`, `javac` [inserts a
cast] back to `String`. A bounded parameter like `T extends Number` erases to `Number`
instead.

The concrete type argument is [not available at runtime] to the erased class. Erasure also
means type parameters accept only reference types. Pass an `int` where a `T` is expected and
Java [boxes it as an `Integer`]. Boxing [entails heap allocation and indirection]. Those
casts and boxes are runtime work that a specialized version never does.

Go's generics proposal left the implementation strategy open. What shipped is a mix of the
two called [GC shape stenciling]. Here is the same `identity` function in Go:

```go
package main

import "fmt"

type User struct{}
type Order struct{}

//go:noinline
func identity[T any](value T) T { return value }

func main() {
    fmt.Println(identity(42))
    fmt.Println(identity(3.14))
    fmt.Println(identity(&User{}))
    fmt.Println(identity(&Order{}))
}
```

This program instantiates `identity` with `int`, `float64`, `*User`, and `*Order`. To decide
which calls can share compiled code, the compiler maps each type argument to its GC shape.

A _GC shape_ is how a type appears to the allocator and the garbage collector: its size, its
alignment, and which parts of it contain pointers. The actual rule is stricter than that.
Per the [Go 1.18 implementation notes], two types share a GC shape only when they have the
same underlying type, with one exception: all pointer types share a single shape named after
`*uint8`.

So `*User` and `*Order` end up in the same group. The exception covers pointer types only. A
`map[string]int` and a `chan int` are each one pointer at runtime, but neither is a pointer
type. They keep their own shapes.

The compiler substitutes each shape for `T` and compiles one version of the function per
shape. That substitution is the _stenciling_ part. The four calls above need only three
bodies: one for `int`, one for `float64`, and one shared by the two pointer types.

Sharing that third body loses information. The compiled code knows it received a pointer,
but not whether the call used `*User` or `*Order`. When the body needs the exact type, Go
supplies it through a hidden [dictionary] argument passed alongside the regular ones.

Despite the name, a _dictionary_ is a fixed table that the compiler generates for each
concrete instantiation and stores in the binary's read-only data. Inside are the runtime
type descriptors and whatever other type-specific entries the body might need. `identity`
does nothing type-dependent, so its body ignores the dictionary. The compiler emits one for
every instantiation anyway.

So this program should produce three function bodies and four dictionaries. The
[`//go:noinline` directive] stops the compiler from inlining the calls, so the `identity`
symbols stay in the binary. Save it as `main.go`, then build it and filter the symbol table.
`go tool nm` lists the symbols. `rg` keeps the `identity` bodies and dictionaries, and `awk`
drops the addresses:

```sh
go build -o /tmp/gcshape main.go
go tool nm /tmp/gcshape \
    | rg 'main\.(identity|\.dict\.identity)\[' \
    | awk '{print $2, $3}'
```

On Go 1.26.5 on `darwin/arm64`, I get:

```txt {hl_lines=["5"]}
R main..dict.identity[*main.Order]
R main..dict.identity[*main.User]
R main..dict.identity[float64]
R main..dict.identity[int]
T main.identity[go.shape.*uint8]
T main.identity[go.shape.float64]
T main.identity[go.shape.int]
```

The first column tells the two groups apart, and the [`go tool nm` docs] explain it:

- `R` marks read-only data. The first four lines are the dictionaries for `*Order`, `*User`,
  `float64`, and `int`.
- `T` marks the text segment, which holds code. The last three lines are the compiled
  bodies.

`int` and `float64` each got a body of their own. The highlighted `go.shape.*uint8` body is
shared by both pointer types, which is why four instantiations produced only three bodies.

> [!NOTE]
>
> The grouping is by underlying type, so a named type adds no new body. Add `type MyInt int`
> and a call `identity(MyInt(7))`. The body count stays at three: `MyInt` reuses
> `go.shape.int` and only adds a dictionary of its own.

That sharing has a runtime cost. Some operations in a shared body need the exact type:
method calls on values of a type parameter, conversions to an interface, and type assertions
and type switches. The compiler rewrites these operations in a [dictionary pass] so they
read what they need from the dictionary at runtime. A fully monomorphized body never does
that.

Apart from those dictionary reads, a shared body should mostly compile to the same assembly
as fully monomorphized code, per the [proposal's risk section]. The main exception is method
calls, which can't be fully resolved at compile time. They can also block inlining and make
escape analysis more conservative, which can mean extra heap allocations.

Generics did slow the compiler down at first. The [Go 1.18 release notes] said compile speed
could be roughly 15% slower than Go 1.17. [Go 1.20] improved build speeds by up to 10% and
brought them back in line with Go 1.17. Both figures cover the compiler as a whole rather
than stenciling alone, but the 1.20 notes attribute the earlier regression largely to
generics support.

Rust and C++ specialize generic functions down to concrete types. Java erases them to a
single implementation and drops the type information. Go stencils down to GC shapes instead.
Types with the same shape share machine code, and the dictionary provides the concrete type
when an operation needs it.

<!-- references -->
<!-- prettier-ignore-start -->

[Go generics proposal]:
    https://github.com/golang/proposal/blob/master/design/43651-type-parameters.md#implementation

[full monomorphization]:
    https://doc.rust-lang.org/book/ch10-01-syntax.html#performance-of-code-using-generics

[type erasure]:
    https://docs.oracle.com/en/java/javase/26/docs/specs/jls/jls-4.html#jls-4.6

[compile time and binary size]:
    https://github.com/golang/proposal/blob/master/design/generics-implementation-stenciling.md#risks

[`rustc`'s `--emit` option]:
    https://doc.rust-lang.org/rustc/command-line-arguments.html#--emit-specifies-the-types-of-output-files-to-generate

[`-C` flags]:
    https://doc.rust-lang.org/rustc/codegen-options/index.html

[template instantiation]:
    https://eel.is/c++draft/temp.inst#5

[GCC's implementation]:
    https://gcc.gnu.org/onlinedocs/gcc/Template-Instantiation.html

[inserts a cast]:
    https://docs.oracle.com/javase/tutorial/java/generics/erasure.html

[not available at runtime]:
    https://docs.oracle.com/en/java/javase/26/docs/specs/jls/jls-4.html#jls-4.7

[boxes it as an `Integer`]:
    https://docs.oracle.com/javase/tutorial/java/generics/restrictions.html#instantiate

[entails heap allocation and indirection]:
    https://openjdk.org/projects/valhalla/design-notes/state-of-valhalla/01-background#the-costs-of-boxing

[GC shape stenciling]:
    https://github.com/golang/proposal/blob/master/design/generics-implementation-gcshape.md

[Go 1.18 implementation notes]:
    https://github.com/golang/proposal/blob/master/design/generics-implementation-dictionaries-go1.18.md#gcshapes

[dictionary]:
    https://github.com/golang/proposal/blob/master/design/generics-implementation-dictionaries-go1.18.md#dictionary-format

[`//go:noinline` directive]:
    https://pkg.go.dev/cmd/compile#hdr-Function_Directives

[`go tool nm` docs]:
    https://pkg.go.dev/cmd/nm

[dictionary pass]:
    https://github.com/golang/proposal/blob/master/design/generics-implementation-dictionaries-go1.18.md#compiler-processing-for-calls-to-generic-functions-and-methods

[proposal's risk section]:
    https://github.com/golang/proposal/blob/master/design/generics-implementation-gcshape.md#risks

[Go 1.18 release notes]:
    https://go.dev/doc/go1.18#compiler

[Go 1.20]:
    https://go.dev/doc/go1.20#compiler

<!-- prettier-ignore-end -->

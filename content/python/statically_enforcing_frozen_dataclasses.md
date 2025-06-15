---
title: Statically enforcing frozen data classes in Python
date: 2024-01-04
tags:
    - Python
    - TIL
---

You can use `@dataclass(frozen=True)` to make instances of a data class immutable during
runtime. However, there's a small caveat—instantiating a frozen data class is slightly
slower than a non-frozen one. This is because, when you enable `frozen=True`, Python has to
generate `__setattr__` and `__delattr__` methods during class definition time and invoke
them for each instantiation.

Below is a quick benchmark comparing the instantiation times of a mutable dataclass and a
frozen one (in Python 3.12):

```py
from dataclasses import dataclass
import timeit


@dataclass
class NormalData:
    a: int
    b: int
    c: int


@dataclass(frozen=True)
class FrozenData:
    a: int
    b: int
    c: int


# Measure instantiation time for NormalData
normal_time = timeit.timeit(lambda: NormalData(1, 2, 3), number=1_000_000)

# Measure instantiation time for FrozenData
frozen_time = timeit.timeit(lambda: FrozenData(1, 2, 3), number=1_000_000)

print(f"Normal data class: {normal_time}")
print(f"Frozen data class: {frozen_time}")
print(f"Frozen data class is {frozen_time / normal_time}x slower")
```

Running this prints:

```txt
Normal data class: 0.13145725009962916
Frozen data class: 0.3248348340857774
Frozen data class is 2.4710301930064014x slower
```

So, frozen data classes are approximately 2.4 times slower to instantiate than their
non-frozen counterparts. This gap can widen further if you compare slotted data classes (via
`@dataclass(slots=True)`) with frozen ones. While the cost for immutability is small, it can
add up if you need to create many frozen instances.

I was reading Tin Tvrtković's article[^1] on making `attr`[^2] instances frozen at compile
time. He mentions how to leverage `mypy` to enforce instance immutability statically and use
mutable `attr` classes at runtime to avoid any instantiation cost. I wanted to see if I
could do the same with standard data classes.

Here's how to do it:

```py
from dataclasses import dataclass
from typing import TYPE_CHECKING, TypeVar, dataclass_transform

if TYPE_CHECKING:
    T = TypeVar("T")

    @dataclass_transform(frozen_default=True)
    def frozen(cls: type[T]) -> type[T]: ...

else:
    frozen = dataclass  # or dataclass(slots=True) for even faster performance


@frozen
class Foo:
    x: int
    y: int


# Instantiate the class
foo = Foo(1, 2)

# Mypy will raise an error here since foo is frozen during type checking
foo.x = 3

print(foo)
```

It involves:

- Using the type checker to ensure the data class instance is immutable.
- Replacing the immutable data class with a more performant mutable one at runtime.

The `if TYPE_CHECKING` condition only executes during type-checking. In that block, we use
`typing.dataclass_transform`, introduced in PEP-681[^3], to create a construct similar to
the `dataclass` function that type checkers recognize.

The `frozen_default` flag, added in Python 3.12, makes this work seamlessly, but the code
should also function in Python 3.11 without changes, as `dataclass_transform` accepts any
keyword arguments. In Python 3.10 and earlier, you can import `dataclass_transform` from
`typing_extensions` and leave the rest of the code as is.

The `else ...` block is what runs when you actually execute the code. There, we're just
aliasing the vanilla `dataclass` function as `frozen`.

Running this code snippet results in:

```txt
Foo(x=3, y=2)
```

However, `mypy` will flag an error since we're trying to mutate `foo.x`:

```txt
foo.py:24: error: Property "x" defined in "Foo" is read-only  [misc]
```

Voilà!

I struggled to figure this one out myself, and LLMs were of no help. So, I ended up posting
a question[^4] on Stack Overflow, where someone pointed out how to use `dataclass_transform`
to achieve this.

Fin!

[^1]:
    [Zero-overhead frozen attrs classes - Tin Tvrtković](https://threeofwands.com/attra-iv-zero-overhead-frozen-attrs-classes/)

[^2]: [attrs](https://www.attrs.org/en/stable/)

[^3]: [PEP 681 – Data Class Transforms](https://peps.python.org/pep-0681/)

[^4]:
    [How to statically enforce frozen data classes in Python?](https://stackoverflow.com/questions/77754655/how-to-statically-enforce-frozen-data-classes-in-python)

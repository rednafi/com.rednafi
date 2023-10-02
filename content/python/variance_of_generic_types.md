---
title: Variance of generic types in Python
date: 2022-01-31
tags:
    - Python
    - Typing
---

I've always had a hard time explaining **variance** of generic types while working with type
annotations in Python. This is an attempt to distill the things I've picked up on type
variance while going through PEP-483.

## A pinch of type theory

> A generic type is a class or interface that is parameterized over types. Variance refers
> to how subtyping between the generic types relates to subtyping between their parameters'
> types.
>
> Throughout this text, the notation `T2 <: T1` denotes `T2` is a subtype of `T1`. A subtype
> always lives in the pointy end.

If `T2 <: T1`, then a generic type constructor `GenType` will be:

* **Covariant**, if `GenType[T2] <: GenType[T1]` for all such `T1` and `T2`.
* **Contravariant**, if `GenType[T1] <: GenType[T2]` for all such `T1` and `T2`.
* **Invariant**, if neither of the above is true.

To better understand this definition, let's make an analogy with ordinary functions. Assume
that we have:

```python
# src.py
from __future__ import annotations


def cov(x: float) -> float:
    return 2 * x


def contra(x: float) -> float:
    return -x


def inv(x: float) -> float:
    return x * x
```

If `x1 < x2`, then always `cov(x1) < cov(x2)`, and `contra(x2) < contra(x1)`, while nothing
could be said about `inv`. Replacing `<` with `<:`, and **functions** with
**generic type constructors**, we get examples of **covariant**, **contravariant**, and
**invariant** behavior.

## A few practical examples

### Immutable generic types are usually type covariant

For example:

* `Union` behaves covariantly in all its arguments. That means: if `T2 <: T1`, then
`Union[T2] <: Union[T1]` for all such `T1` and `T2`.

* `FrozenSet[T]` is also covariant. Let's consider `int` and `float` in place of `T`. First,
`int <: float`. Second, a set of values of `FrozenSet[int]` is clearly a subset of values of
`FrozenSet[float]`. Therefore, `FrozenSet[int] <: FrozenSet[float]`.

### Mutable generic types are usually type invariant

For example:

* `list[T]` is invariant. Although a set of values of `list[int]` is a subset of values of
`list[float]`, only an `int` could be appended to a `list[int]`. Therefore, `list[int]` is
not a subtype of `list[float]`.

### The callable generic type is covariant in return type but contravariant in the arguments

* `Callable[[], int] <: Callable[[], float]` .
* If `Manager <: Employee` then `Callable[[], Manager] <: Callable[[], Employee]`.

However, for two callable types that differ only in the type of one argument, the subtype
relationship for the callable types goes in the opposite direction as for the argument
types. Examples:

* `Callable[[float], None] <: Callable[[int], None]`, where `int <: float`.

* `Callable[[Employee], None] <: Callable[[Manager], None]`, where
`Manager <: Employee`.

I found this odd at first. However, this actually makes sense. If a function can calculate
the salary for a `Manager`, it should also be able to calculate the salary of an `Employee`.

## Examples

### Covariance

```python
# src.py
from __future__ import annotations

# In <Python 3.9, import this from the 'typing' module.
from collections.abc import Sequence


class Animal:
    pass


class Dog(Animal):
    pass


def action(animals: Sequence[Animal]) -> None:
    pass


if __name__ == "__main__":
    action((Animal(),))  # ok
    action((Dog(),))  # ok
```

Here, `Dog <: Animal` and notice how Mypy doesn't raise an error when a tuple of `Dog`
instance is passed into the `action` function that expects a sequence of `Animal` instances.
However, if you make change the `action` function as follows:

```python
...


def action(animals: Sequence[Dog]) -> None:
    pass


if __name__ == "__main__":
    action((Animal(),))  # not ok
    action((Dog(),))  # ok
```

Mypy will complain about this snippet since now, `action` expects a sequence of `Dog`
instance or a subtype of it. A sequence of `Animal` is not a subtype of a sequence of `Dog`.
Hence, the error.

### Contravariance

The `Callable` generic type is **covariant** in return type. Here's how you can test it:

```python
from __future__ import annotations

# In <Python 3.9, import this from the 'typing' module.
from collections.abc import Callable


def factory(func: Callable[..., float]) -> Callable[..., float]:
    return func


def foo() -> int:
    return 42


def bar() -> float:
    return 42


if __name__ == "__main__":
    factory(foo)  # ok
    factory(bar)  # ok
```

Here, `int <: float` and the in the return type, you can see
`Callable[..., int] <: Callable[float]` as Mypy is satisfied when either `foo` or `bar` is
passed into the `factory` callable.

On the other hand, the `Callable` generic type is **contravariant** in the argument type.
Here's how you can test it:

```python
from __future__ import annotations

# In <Python 3.9, import this from the 'typing' module.
from collections.abc import Callable


def factory(func: Callable[[float], None]) -> Callable[[float], None]:
    return func


def foo(number: int) -> None:
    pass


def bar(number: float) -> None:
    pass


if __name__ == "__main__":
    factory(foo)  # not ok
    factory(bar)  # ok
```

Here, Mypy will complain in the case of `factory(foo)` as the factory function expects
`Callable[[float]], None]` or its subtype. However, in the above case,
`Callable[[float]], None] <: Callable[[int], None]` but not the other way around. That
causes the error.

### Invariance

In general, types defined with the `TypeVar` construct are invariant. You can mark them as
covariant or contravariant as well. However:

> Remember that variance is a property of the generic types; not their parameter types.

Here's how you can mark types as covariant, contravariant, or invariant:

```python
from __future__ import annotations

from typing import Generic, TypeVar


T = TypeVar("T")
T_co = TypeVar("T_co", covariant=True)
T_contra = TypeVar("T_contra", contravariant=True)


class HolderInv(Generic[T]):
    def __init__(self, *args: T) -> None:
        self.args = args


class HolderCov(Generic[T_co]):
    def __init__(self, *args: T_co) -> None:
        self.args = args


class HolderContra(Generic[T_contra]):
    def __init__(self, *args: T_contra) -> None:
        self.args = args


def process_holder_inv(holder: HolderInv[float]) -> None:
    pass


def process_holder_cov(holder: HolderCov[float]) -> None:
    pass


def process_holder_contra(holder: HolderContra[float]) -> None:
    pass


if __name__ == "__main__":
    holder_inv = HolderInv(1.0)  # ok
    holder_cov = HolderCov(1, 2)  # ok
    holder_contra = HolderContra(
        1, 2
    )  # raises error because T is contravariant

    process_holder_inv(holder_inv)
    process_holder_cov(holder_cov)
    process_holder_contra(holder_contra)
```

[^1]: [PEP 483 -- The theory of type hints](https://www.python.org/dev/peps/pep-0483/#generic-types) [^1]

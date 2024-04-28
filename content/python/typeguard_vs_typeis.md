---
title: TypeIs Does What I Thought TypeGuard Would Do in Python
date: 2024-04-27
tags:
    - Python
    - Typing
    - TIL
---

The handful of times I've reached for `typing.TypeGuard` in Python, I've always been
confused by its behavior and ended up ditching it with a `# type: ignore` comment.

For the uninitiated, `TypeGuard` allows you to apply custom type narrowing[^1]. For example,
let's say you have a function named `pretty_print` that accepts a few different types and
prints them differently onto the console:

```python
from typing import assert_never


def pretty_print(val: int | float | str) -> None:
    if isinstance(val, int):  # assert_type(val, int)
        print(f"Integer: {val}")

    elif isinstance(val, float):  # assert_type(val, float)
        print(f"Float: {val}")

    elif isinstance(val, str):  # assert_type(val, str)
        print(f"String: {val}")

    else:
        assert_never(val)
```

If you run it through `mypy`, in each branch, the type checker automatically narrows the
type and knows exactly what the type of `val` is. You can test the narrowed type in each
branch with the `typing.assert_type` function.

This works well for 99% of cases, but occasionally, you need to check an incoming value more
thoroughly to determine its type and want to take action based on the narrowed type. In
those cases, just using `isinstance` may not be sufficient. So, you need to factor out the
complex type checking logic into a separate function and return a boolean depending on
whether an inbound value satisfies all the criteria to be of the expected type. For example:

```python
from typing import TypedDict, TypeGuard


class Person(TypedDict):
    name: str
    age: int


def is_person(val: dict) -> TypeGuard[Person]:
    try:
        name, age = val["name"], val["age"]
    except KeyError:
        return False

    return len(name) > 1 and 0 < age < 150


def print_age(val: dict) -> None:
    if is_person(val):  # assert_type(val, Person)
        print(f"Age: {val['age']}")
    else:
        print("Not a person!")
```

Here, `is_person` first checks that the inbound dictionary conforms to the structure of the
`Person` typeddict and then verifies that the `name` is at least 1 character long and the
`age` is between 0 and 150.

This is a bit more involved than just checking the type with `isinstance` and the type
checker needs a little more help from the user. Although the return type of the `is_person`
function is `bool`, typing it with `TypeGuard[Person]` signals the type checker that if the
inbound value satisfies all the constraints and the checker function returns `True`, the
underlying type of `val` is `Person`.

You can see more examples of `TypeGuard` in PEP-647[^2].

All good, however, I find the behavior of `TypeGuard` a bit unintuitive whenever I need to
couple it with union types. For example:

```python
from typing import assert_never, assert_type, Any, TypeGuard


def is_non_zero_number(val: Any) -> TypeGuard[int | float]:
    return val != 0


def pretty_print(val: str | int | float) -> None:
    if is_non_zero_number(val):  # assert_type(val, int | float)
        print(f"Non zero number: {val}")

    else:  # assert_type(val, str | int | float); wat??
        print(f"String: {val}")
```

In the `if` branch, `TypeGuard` signals the type checker correctly that the narrowed type of
the inbound value is `int | float` but in the `else` branch, I was expecting it to be `str`
because the truthy `if` condition has already filtered out the `int | float`. But instead,
we get `str | int | float` as the narrowed type. While there might be a valid reason behind
this design choice, the resulting behavior with union types made `TypeGuard` fairly useless
for most of the cases I wanted to use it for.

`TypeIs` has been introduced via PEP-742[^3] to fix exactly that. The PEP agrees that people
might find the current behavior of `TypeGuard` a bit unexpected and introducing another
construct with a slightly different behavior doesn't make thing any less confusing.

> We acknowledge that this leads to an unfortunate situation where there are two constructs
> with a similar purpose and similar semantics. We believe that users are more likely to
> want the behavior of `TypeIs`, the new form proposed in this PEP, and therefore we
> recommend that documentation emphasize `TypeIs` over `TypeGuard` as a more commonly
> applicable tool.

`TypeGuard` and `TypeIs` have similar semantics, except, the latter can narrow the type in
both the `if` and `else` branches of a conditional. Here's another example with a union type
where `TypeIs` does what I expected `TypeGuard` to do:

```python
import sys

if sys.version_info > (3, 13):  # TypeIs is available in Python 3.13+
    from typing import TypeIs
else:
    from typing_extensions import TypeIs


def is_number(value: object) -> TypeIs[int | float | complex]:
    return isinstance(value, (int, float, complex))


def pretty_print(val: str | int | float | complex) -> None:
    if is_number(val):  # assert_type(val, int, float, complex)
        print(val)
    else:  # assert_type(val, str)
        print("Not a number!")
```

Notice that `TypeIs` has now correctly narrowed the type in the `else` branch as well. This
would've also worked had we returned early from the `pretty_print` function in the `if`
branch and skipped the `else` branch altogether. Exactly what I need!

Here are a few typeshed stubs[^4] for the stdlib functions in the `inspect` module that are
already taking advantage of the new `TypeIs` construct:

```python
# fmt: off

def isgenerator(obj: object) -> TypeIs[GeneratorType[Any, Any, Any]]: ...
def iscoroutine(obj: object) -> TypeIs[CoroutineType[Any, Any, Any]]: ...
def isawaitable(obj: object) -> TypeIs[Awaitable[Any]]: ...
def isasyncgen(object: object) -> TypeIs[AsyncGeneratorType[Any, Any]]: ...
def istraceback(object: object) -> TypeIs[TracebackType]: ...

# and so on and so forth
```

[^1]: [Type narrowing](https://mypy.readthedocs.io/en/latest/type_narrowing.html)
[^2]: [PEP 647 – User-Defined Type Guards](https://peps.python.org/pep-0647/)
[^3]: [PEP 742 – Narrowing types with TypeIs](https://peps.python.org/pep-0742/)
[^4]:
    [Typeshed stubs taking advantage of TypeIs](https://github.com/python/typeshed/blob/bdf75023dfe3930deac1c6b4e269770427b106c4/stdlib/inspect.pyi)

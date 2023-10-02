---
title: Narrowing types with TypeGuard in Python
date: 2022-02-23
tags:
    - Python
    - Typing
---

Static type checkers like Mypy follow your code flow and statically try to figure out the
types of the variables without you having to explicitly annotate inline expressions. For
example:

```python
# src.py
from __future__ import annotations


def check(x: int | float) -> str:
    if not isinstance(x, int):
        reveal_type(x)
        # Type is now 'float'.

    else:
        reveal_type(x)
        # Type is now 'int'.

    return str(x)
```

The `reveal_type` function is provided by Mypy and you don't need to import this. But
remember to remove the function before executing the snippet. Otherwise, Python will raise a
runtime error as the function is only understood by Mypy. If you run Mypy against this
snippet, it'll print the following lines:

```txt
src.py:6: note: Revealed type is "builtins.float"
src.py:10: note: Revealed type is "builtins.int"
```

Here, I didn't have to explicitly tell the type checker how the conditionals narrow the
types.

> Static type checkers commonly employ a technique called 'type narrowing' to determine a
> more precise type of an expression within a program's code flow. When type narrowing is
> applied within a block of code based on a conditional code flow statement (such as if and
> while statements), the conditional expression is sometimes referred to as a 'type guard'.
> — PEP-647

So, in the above snippet, Mypy performed **type narrowing** to determine the more precise
type of the variable `x`; and the `if ... else` conditionals, in this case, is known as
**type guards**.

However, when the type checker encounters a complex expression, often time, it can't figure
out the types statically. Mypy will complain when it faces one of these issues:

```python
from __future__ import annotations

# In <Python3.9, import this from the 'typing' module.
from collections.abc import Sequence


def check_sequence_str(container: Sequence[object]) -> bool:
    """Check all objects in the container is of type str."""

    return all(isinstance(elem, str) for elem in container)


def concat(
    container: Sequence[object],
    sep: str = "-",
) -> str | None:
    """Concat a sequence of string with the 'sep'."""

    if check_sequence_str(container):
        return f"{sep}".join(container)


if __name__ == "__main__":
    # Mypy complains here, as it can't figure out the
    # container type.
    concat(["hello", "world"])
```

Here, the `check_sequence_str` checks whether the input argument is a sequence of strings in
runtime. Then in the `concat` function, I used it to check whether the input conforms to the
expected type requirement; if it does, the function performs string concatenation on the
input and returns the value. Otherwise, it returns `None`. If you run mypy against this,
it'll complain:

```txt
src.py:22: error: Argument 1 to "join" of "str" has incompatible type
"Sequence[object]";

expected "Iterable[str]"
            return f"{sep}".join(container)
                                 ^
Found 1 error in 1 file (checked 1 source file)
```

The type checker can't figure out that the container type is `list[str]`.

Functions like `check_sequence_str` that—checks the type of an input object and returns a
boolean—are called **type guard functions**. PEP-647 proposed a `TypeGuard` class to help
the type checkers to narrow down types from more complex expressions. Python 3.10 added the
`TypeGuard` class to the `typing` module. You can use it like this:

```python
# src.py
...

# In <Python3.10, import this from 'typing_extensions' module.
from typing import TypeGuard


def check_sequence_str(
    container: Sequence[object],
) -> TypeGuard[Sequence[str]]:
    """Check all objects in the container is of type str."""

    return all(isinstance(elem, str) for elem in container)


...
```

Notice that the return type now has the expected type defined inside the `TypeGuard`
generic. Now, Mypy will be satisfied if you run it against the modified snippet.

## Properties of type guard functions

You've already seen how `check_sequence_str` narrows down the type of an object in runtime.
Functions like this can be loosely called user-defined type guard functions. However, to be
considered a proper type guard function by the type checker, the callable needs to pass
through a few more checks.

> When TypeGuard is used to annotate the return type of a function or method that accepts at
> least one parameter, that function or method is treated by type checkers as a user-defined
> type guard. The type argument provided for TypeGuard indicates the type that has been
> validated by the function. — PEP-647

-   Usually, a type guard function only takes a single parameter and returns a boolean value
    based on the conformity of the type of the incoming object with the expected type. The
    expected type needs to be wrapped in `TypeGuard` and added as the return type
    annotation.

-   Type checkers will only check if the first positional argument conforms to the expected
    return type annotation. It'll ignore other parameters if there is more than one.

-   If you define a type guard callable in a class, in that case, the type checker will
    ignore `self/cls` argument and check the second positional parameter for type
    conformity. Additional parameters won't be checked.

-   The input type is usually wider than the output type. In our example case, the input
    type `Sequence[object]` is less specific than that of the return type `Sequence[str]`.
    However, this is mostly a convention and not enforced by any means.

> The return type of a user-defined type guard function will normally refer to a type that
> is strictly "narrower" than the type of the first argument (that is, it's a more specific
> type that can be assigned to the more general type). However, it is not required that the
> return type be strictly narrower. — PEP-647

## Generic type guard functions

User-defined type guards can be generic functions, as shown in this example:

```python
from __future__ import annotations

# In <Python3.9, import these from the 'typing' module.
from collections.abc import Generator, Sequence

# In <Python3.10, import TypeGuard from 'typing_extensions'.
from typing import TypeGuard, TypeVar

T = TypeVar("T")


def list_of_t(
    container: Sequence[T],
    types: tuple = (int, str),  # Allowed types in the container.
) -> TypeGuard[list[T]]:
    return all(isinstance(elem, types) for elem in container)


def process(container: Sequence[T]) -> Generator[T, None, None]:
    if list_of_t(container):
        for elem in container:
            yield elem


if __name__ == "__main__":
    container = ["jupiter", "mars"]
    for elem in process(container):
        print(elem)
```

Here, type guard function `list_of_t` is a generic function since it accepts a generic
container `Sequence[T]`. The first parameter is the input type that the type checker will
focus on, and the second parameter denotes the default types that are allowed inside the
output list. Running the snippet will print `jupiter` and `mars` in the console and mypy
will also be happy with the types.

[^1]: [PEP 647 -- User-defined type guards](https://www.python.org/dev/peps/pep-0647/) [^1]
[^2]:
    [Python type hints - how to narrow types with TypeGuard — Adam Johnson](https://adamj.eu/tech/2021/06/09/python-type-hints-how-to-narrow-types-with-typeguard/)
    [^2]

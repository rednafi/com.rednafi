---
title: Go Rusty with exception handling in Python
date: 2022-02-02
tags:
    - Python
---

While grokking Black formatter's codebase, I came across this[^1] interesting way of
handling exceptions in Python. Exception handling in Python usually follows the EAFP
paradigm where it's easier to ask for forgiveness than permission.

However, Rust has this recoverable error[^2] handling workflow that leverages generic Enums.
I wanted to explore how Black emulates that in Python. This is how it works:

```py
# src.py
from __future__ import annotations

from typing import Generic, TypeVar, Union

T = TypeVar("T")
E = TypeVar("E", bound=Exception)


class Ok(Generic[T]):
    def __init__(self, value: T) -> None:
        self._value = value

    def ok(self) -> T:
        return self._value


class Err(Generic[E]):
    def __init__(self, e: E) -> None:
        self._e = e

    def err(self) -> E:
        return self._e


Result = Union[Ok[T], Err[E]]
```

In the above snippet, two generic types `Ok` and `Err` represent the return type and the
error types of a callable respectively. These two generics were then combined into one
`Result` generic type. You'd use the `Result` generic to handle exceptions as follows:

```py
# src.py
...


def div(dividend: int, divisor: int) -> Result[int, ZeroDivisionError]:
    if divisor == 0:
        return Err(ZeroDivisionError("Zero division error occurred!"))

    return Ok(dividend // divisor)


if __name__ == "__main__":
    result = div(10, 0)
    if isinstance(result, Ok):
        print(result.ok())
    else:
        print(result.err())
```

This will print:

```txt
Zero division error occurred!
```

If you run Mypy on the snippet, it'll succeed as well.

You can also apply constraints on the return or exception types as follows:

```py
# src.py
...
# Only int, float, and str types are allowed as input.
Convertible = TypeVar("Convertible", int, float, str)

# Create a more specialized generic type from Result.
IntResult = Result[int, TypeError]


def to_int(num: Convertible) -> IntResult:
    """Converts a convertible input to an integer."""

    if not isinstance(num, (int, float, str)):
        return Err(
            TypeError(
                "Input type is not convertible to an integer type.",
            )
        )

    return Ok(int(num))


if __name__ == "__main__":
    result = to_int(1 + 2j)

    if isinstance(result, Ok):
        print(result.ok())
    else:
        print(result.err())
```

Running the script will give you this:

```txt
Input type is not convertible to an integer type.
```

In this case, Mypy will catch the type inconsistency before runtime.

## Breadcrumbs

Black extensively uses this pattern[^3] in the transformation part of the codebase. This
showed me another way of thinking about handling recoverable exceptions while ensuring type
safety in a Python codebase.

However, I wouldn't go about and mindlessly refactor any exception handling logic that I
come across to follow this pattern. You might find it useful if you need to handle
exceptions in a recoverable fashion and need additional type safety around the logic.

[^1]:
    [An error-handling model influenced by the Rust programming language](https://github.com/psf/black/blob/main/src/black/rusty.py)

[^2]:
    [Recoverable errors with result](https://doc.rust-lang.org/book/ch09-02-recoverable-errors-with-result.html)

[^3]:
    [More rusty error handling in Black](https://github.com/psf/black/blob/6417c99bfdbdc057e4a10aeff9967a751f4f85e9/src/black/trans.py#L61)

[^4]:
    [Beginner's guide to error handling in Rust](https://www.sheshbabu.com/posts/rust-error-handling/)
    [^4]

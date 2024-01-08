---
title: Annotating args and kwargs in Python
date: 2024-01-08
tags:
    - TIL
---

While I tend to avoid `*args` and `**kwargs` in my function signatures, it's not always
possible to do so without hurting API ergonomics. Especially when you need to write
functions that call other helper functions with the same signature.

Typing `*args` and `**kwargs` has always been a pain since you couldn't annotate them
precisely before. For example, if all the positional and keyword arguments of a function had
the same type, you could do this:

```python
def foo(*args: int, **kwargs: bool) -> None:
    ...
```

This implies that `args` is a tuple where all the elements are integers, and `kwargs` is a
dictionary where the keys are strings and the values are booleans.

On the flip side, you couldn't annotate `*args` and `**kwargs` properly if the values of the
positional and keyword arguments had different types. In those cases, you'd have to fallback
to `Any`, which beats the purpose.

Consider this:

```python
def foo(*args: tuple[int, str], **kwargs: dict[str, bool | None]) -> None:
    ...
```

Here, the type checker sees each positional argument as a tuple of an integer and a string.
Plus, it considers each keyword argument as a dictionary where the keys are strings and the
values are either booleans or None.

With the previous annotation, `mypy` will reject this:

```python
foo(*(1, "hello"), **{"key1": 1, "key2": False})
```

```txt
error: Argument 1 to "foo" has incompatible type "*tuple[int, str]";
expected "tuple[int, str]"  [arg-type]

error: Argument 2 to "foo" has incompatible type "**dict[str, int]";
expected "dict[str, bool | None]"  [arg-type]
```

Instead, it'll accept the following:

```python
foo((1, "hello"), (2, "world"), kw1={"key1": 1, "key2": False})
```

You probably wanted to represent the former while the type checker wants the latter.

To annotate the second instance correctly, you'll need to leverage bits of PEP-589[^1],
PEP-646[^2], and PEP-692[^3]. We'll use `Unpack` and `TypedDict` from the `typing` module to
achieve this. Here's how:

```python
from typing import TypedDict, Unpack  # Python 3.12+

# from typing_extensions import TypedDict, Unpack # < Python 3.12


class Kw(TypedDict):
    key1: int
    key2: bool


def foo(*args: Unpack[tuple[int, str]], **kwargs: Unpack[Kw]) -> None:
    pass


args = (1, "hello")
kwargs: Kw = {"key1": 1, "key2": False}

foo(*args, **kwargs)  # Ok
```

`TypedDict` was added in Python 3.8 to allow you to annotate heterogenous dictionaries. If
all the values of a dictionary has the same type, you can simply use `dict[str, T]` to
annotate it. However, `TypedDict` covers the cases where all the keys of a dictionary are
strings but the type of the values vary.

The following dictionary

```python
movies = {"name": "Mad Max", "year": 2015}
```

can be typed as such:

```python
from typing import TypedDict


class Movie(TypedDict):
    name: str
    year: int


movies: Movie = {"name": "Mad Max", "year": 2015}
```

`Unpack` marks an object as having been unpacked.

Using `TypedDict` with `Unpack` allows us to communicate with the type checker so that each
positional and keyword aren't mistakenly assumed as a tuple and a dictionary respectively.

While the type checker is satisfied when you pass the `**args` and `**kwargs` as

```python
foo(*args, **kwargs)
```

But it'll complain if you don't pass all the keword arguments:

```python
foo(*args, key1=1)  # error: Missing named argument "key2" for "foo"
```

To make all the keywords optional, could turn off the `total` flag in the typed-dict
definition:

```python {hl_lines=3}
# ...


class Kw(TypedDict, total=False):
    key1: int
    key2: str


# ...
```

Or you could mark specific keywords as optional with `typing.NotRequired`:

```python {hl_lines=6}
# ...


class Kw(TypedDict):
    key1: int
    key2: NotRequired[str]


# ...
```

Fin!

[^1]: [](https://peps.python.org/pep-0589/)
[^2]: [](https://peps.python.org/pep-0646/)
[^3]: [](https://peps.python.org/pep-0692/)

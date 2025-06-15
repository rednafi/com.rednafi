---
title: Difference between constrained 'TypeVar' and 'Union' in Python
date: 2022-01-19
tags:
    - Python
    - Typing
    - TIL
---

If you want to define a variable that can accept values of multiple possible types, using
`typing.Union` is one way of doing that:

```py
from typing import Union

U = Union[int, str]
```

However, there's another way you can express a similar concept via constrained `TypeVar`.
You'd do so as follows:

```py
from typing import TypeVar

T = TypeVar("T", int, str)
```

So, what's the difference between these two and when to use which? The primary difference
is:

> T's type needs to be consistent across multiple uses within a given scope, while U's
> doesn't.

With a `Union` type used as function parameters, the arguments, as well as the return type,
can all be different:

```py
# src.py
from typing import Union

U = Union[int, str]


# Native generic tuple requires py3.10 or
# 'from __future__ import annotations' import.
def foo(a: U, b: U) -> tuple[U, ...]:
    return (a, b)


# Use the 'foo' function.
foo("apple", "bazooka")  # This is valid.
foo(1, "apple")  # Mypy won't complain here.
foo("apple", 1)  # Mypy won't complain here as well.
```

However, the above type definition will be too loose if you need to ensure that all of your
function parameters must be of the same type in a single scope. Here's where constrained
`TypeVar` can come in handy:

```py
# src.py
from typing import TypeVar

T = TypeVar("T", int, str)


def add(a: T, b: T) -> T:
    return a + b


add("hello", "world")  # This is allowed.
add(1, 2)  # This is fine as well.
add("hello", 1)  # Mypy will complain about this one and it'll fail in runtime.
```

If you run Mypy against the above snippet, you'll get this:

```txt
$ mypy src.py
src.py:12: error: Value of type variable "T" of "add" cannot be "object"
    add("hello", 1)  # Mypy will complain about this one and it'll fail in runtime.
    ^
Found 1 error in 1 file (checked 1 source file)
```

As the comment implies, this error is coming from the line where I called `add("hello", 1)`.
The function `add` can take parameters of either integer or string type. However, the type
of both the parameters needs to be the same. Also, the type of the input parameters will
define the type of the output value. So, the types of the input parameters must match,
otherwise, Mypy will complain and in this case, the snippet will also raise a `TypeError` in
runtime. Mypy is statically catching a bug that'd otherwise appear in runtime, how
convenient!

[^1]:
    [What's the difference between a constrained TypeVar and a Union?](https://stackoverflow.com/questions/58903906/whats-the-difference-between-a-constrained-typevar-and-a-union)
    [^1]

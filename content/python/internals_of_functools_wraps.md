---
title: Peeking into the internals of Python's 'functools.wraps' decorator
date: 2022-02-14
tags:
    - Python
---

The `functools.wraps` decorator allows you to keep your function's identity intact after
it's been wrapped by a decorator. Whenever a function is wrapped by a decorator, identity
properties like—function name, docstring, annotations of it get replaced by those of the
wrapper function. Consider this example:

```python
from __future__ import annotations

# In < Python 3.9, import this from the typing module.
from collections.abc import Callable
from typing import Any


def log(func: Callable) -> Callable:
    def wrapper(*args: Any, **kwargs: Any) -> Any:
        """Internal wrapper."""

        val = func(*args, **kwargs)
        return val

    return wrapper


@log
def add(x: int, y: int) -> int:
    """Add two numbers.

    Parameters
    ----------
    x : int
        First argument.
    y : int
        Second argument.

    Returns
    -------
    int
        Returns the summation of two integers.
    """
    return x + y


if __name__ == "__main__":
    print(add.__doc__)
    print(add.__name__)
```

Here, I've defined a simple logging decorator that wraps the `add` function. The function
`add` has its own type annotations and docstring. So, you'd expect the **docstring** and
**name** of the `add` function to be printed when the above snippet gets executed. However,
running the script prints the following instead:

```txt
Internal wrapper.
wrapper
```

This is surprising and probably not something you want. If you pay attention to the function
`wrapper` in the `log` decorator, you'll see that the identity properties of the `wrapper`
function replace the identity properties of the wrapped function `add`. This can easily be
avoided by decorating the `wrapper` function inside the `log` decorator with the
`functools.wraps` decorator:

```python
# src.py
from functools import wraps

...


def log(func: Callable) -> Callable:
    @wraps(func)  # Here's the decorator!
    def wrapper(*args: Any, **kwargs: Any) -> Any:
        """Internal wrapper."""
        val = func(*args, **kwargs)
        return val

    return wrapper


...
```

Now, running the script will return the expected output:

```txt
Add two numbers.

    Parameters
    ----------
    x : int
        First argument.
    y : int
        Second argument.

    Returns
    -------
    int
        Returns the summation of two integers.

add
```

I wanted to take a peek into how the `functools.wraps` decorator works internally. Turns out
that the implementation is quite straightforward. Here's the entire implementation from the
`functools.py` module. For brevity's sake, I've stripped out the comments and added type
annotations:

```python
# functools.py

from __future__ import annotations

# In < Python 3.9, import this from the typing module.
from collections.abc import Callable

WRAPPER_ASSIGNMENTS = (
    "__module__",
    "__name__",
    "__qualname__",
    "__doc__",
    "__annotations__",
)
WRAPPER_UPDATES = ("__dict__",)


def update_wrapper(
    wrapper: Callable,
    wrapped: Callable,
    assigned: tuple[str, ...] = WRAPPER_ASSIGNMENTS,
    updated: tuple[str, ...] = WRAPPER_UPDATES,
) -> Callable:
    for attr in assigned:
        try:
            value = getattr(wrapped, attr)
        except AttributeError:
            pass
        else:
            setattr(wrapper, attr, value)
    for attr in updated:
        getattr(wrapper, attr).update(getattr(wrapped, attr, {}))

    wrapper.__wrapped__ = wrapped
    return wrapper


def wraps(
    wrapped: Callable,
    assigned: tuple[str, ...] = WRAPPER_ASSIGNMENTS,
    updated: tuple[str, ...] = WRAPPER_UPDATES,
) -> Callable:
    return partial(
        update_wrapper,
        wrapped=wrapped,
        assigned=assigned,
        updated=updated,
    )
```

The bulk of the work is done in the `update_wrapper` function. It copies the identity
properties defined in `WRAPPER_ASSIGNMENTS` and `WRAPPER_UPDATES`—from the `wrapped`
function over to the `wrapper` function. Here, the `wrapped` function is the decorated one
(`add` function) and the `wrapper` function is the eponymous function inside the `log`
decorator.

Since you've already seen that whenever you try to introspect the identity properties of a
wrapped function, the wrapper function obfuscates them and returns its own properties.
However, if the identity properties are copied over from the wrapped to the wrapper
function, your inspection will return the expected result. The `update_wrapper` function is
doing exactly that.

The `wraps` function just binds the input arguments with the `update_wrapper` function using
the `partial` function defined in the same module. This allows us to use the `wraps`
function as a decorator.

You can also directly use the `update_wrapper` function to get the same result should you
choose to do so. Here's how to do it:


```python
# src.py
from functools import update_wrapper

...


def log(func: Callable) -> Callable:
    def wrapper(*args: Any, **kwargs: Any) -> Any:
        """Internal wrapper."""
        val = func(*args, **kwargs)
        return val

    # Only this line is different!
    return update_wrapper(func, wrapper)


...
```

[^1]: [functools.update_wrapper](https://github.com/python/cpython/blob/0ae40191793da1877a12d512f0116d99301b2c51/Lib/functools.py#L35) [^1]

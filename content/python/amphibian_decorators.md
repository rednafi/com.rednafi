---
title: Amphibian decorators in Python
date: 2022-02-06
tags:
    - Python
---

Whether you like it or not, the split world of sync and async functions in the Python
ecosystem is something we'll have to live with; at least for now. So, having to write things
that work with both sync and async code is an inevitable part of the journey. Projects like
Starlette[^1], HTTPx[^2] can give you some clever pointers on how to craft APIs that are
compatible with both sync and async code.

> Lately, I've been calling constructs that are compatible with both synchronous and
> asynchronous paradigms as Amphibian Constructs.

So, I wanted to write an amphibian decorator that'd work with both sync and async functions.
Let's consider writing a trivial decorator that'll tag the wrapped function. Here, by
tagging I mean, the decorator will attach a `_tags` attribute to the wrapped function where
the value of the tag can be passed as the function parameter.

This type of tagging can be helpful if you want to write code that'll classify functions
based on their tags and do interesting things with them. Locust[^3] uses this concept of
tagging to select and deselect load-testing routines in the CLI. Also, `@pytest.mark.*`
utilizes a similar concept.

Here's how you can do that:

```python
# src.py
from __future__ import annotations

import inspect

# In <Python 3.9, import these from the 'typing' module.
from collections.abc import Awaitable, Callable
from functools import wraps
from typing import Any


def tag(*names: str) -> Callable:
    def decorator(func: Callable) -> Callable:
        # Tagging has to happen in function definition time.
        # Othewise calling func._tags will raise AttributeError.
        func._tags = names  # type: ignore

        if inspect.iscoroutinefunction(func):
            @wraps(func)
            async def async_wrapped(*args: Any, **kwargs: Any) -> Awaitable:
                return await func(*args, **kwargs)
            return async_wrapped

        else:
            @wraps(func)
            def sync_wrapped(*args: Any, **kwargs: Any) -> Any:
                return func(*args, **kwargs)
            return sync_wrapped

    return decorator
```

In the above snippet—

-   The decorator `tag` is a variadic function that accepts the names of the tags.

-   I attached the tag to a function before dealing with the sync and async functions. The
    tag attachment is done via `func._tags = names` statement. Placing them outside of the
    wrapped function also makes sure that the attachment happens during the definition time
    of the wrapped function; not during runtime. Otherwise, it'll raise AttributeError if
    you try to access `func._tags` to inspect the tags.

-   Afterwards, I checked if the function is an async one via `iscoroutinefunction` function
    from the `inspect` module. If the wrapped function is an async function, then it's
    executed with the `await` statement. Otherwise, the function is a sync function and is
    executed as usual.

You can play around with the decorator as follows:

```python
In [1]: import asyncio

In [2]: @tag('tag_1', 'tag_2')
   ...: async def foo():
   ...:     await asyncio.sleep(1)
   ...:     return 42

In [3]: @tag('tag_3', 'tag_4')
   ...: def bar():
   ...:     return 24

In [4]: foo._tags
Out[4]: ('tag_1', 'tag_2')

In [5]: bar._tags
Out[5]: ('tag_3', 'tag_4')

In [6]: asyncio.run(foo())
Out[6]: 42

In [7]: bar()
Out[7]: 24
```

## Breadcrumbs

Astute readers might notice that the type annotations in this decorator are quite loose and
it doesn't take advantage of Python 3.10's `typing.ParamSpec` type. This is intentional as
it adds quite a bit of noise that might obfuscate the primary intent of the code snippet.
Also, typing a decorator that returns either a sync or async callable based on the control
flow is tricky.

[^1]: [Starlette](https://www.starlette.io/)
[^2]: [HTTPx](https://www.python-httpx.org/)
[^3]: [Locust](http://docs.locust.io/en/stable/api.html#locust.tag)
[^4]:
    [Amphibian decorator in Starlette's source code](https://github.com/encode/starlette/blob/424351cb231c67798a65c091b0b7d42790f5e444/starlette/authentication.py#L19)
    [^4]

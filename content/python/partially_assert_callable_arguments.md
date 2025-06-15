---
title: Partially assert callable arguments with 'unittest.mock.ANY'
date: 2022-07-17
tags:
    - Python
    - Testing
---

I just found out that you can use Python's `unittest.mock.ANY` to make assertions about
certain arguments in a mock call, without caring about the other arguments. This can be
handy if you want to test how a callable is called but only want to make assertions about
some arguments. Consider the following example:

```py
# test_src.py

import random
import time


def fetch() -> list[float]:
    # Simulate fetching data from a database.
    time.sleep(2)
    return [random.random() for _ in range(4)]


def add(w: float, x: float, y: float, z: float) -> float:
    return w + x + y + z


def procss() -> float:
    return add(*fetch())
```

Let's say we only want to test the `process` function. But `process` ultimately depends on
the `fetch` function, which has multiple side effects—it returns pseudo-random values and
waits for 2 seconds on a fictitious network call. Since we only care about `process`, we'll
mock the other two functions. Here's how `unittest.mock.ANY` can make life easier:

```py
# test_src.py

from unittest.mock import patch, ANY


@patch("test_src.fetch", return_value=[1, 2, 3, 4])
@patch("test_src.add", return_value=42)
def test_process(mock_add, mock_fetch):
    result = procss()

    assert result == 42
    mock_fetch.assert_called_once()

    # Assert that the 'add' function was called with the correct
    # arguments. Notice we only care about the first two arguments,
    # so we've set the remaining ones to ANY.
    mock_add.assert_called_once_with(1, 2, ANY, ANY)
```

While this is a simple example, I found `ANY` to be quite useful while making assertions
about callables that accept multiple complex objects as parameters. Being able to ignore
some aruments while calling `mock_callable.assert_called_with()` can make the tests more
tractable.

Under the hood, the implementation of `ANY` is quite simple. It's an instance of a class
that defines `__eq__` and `__ne__` in a way that comparing any value with `ANY` will return
`True`. Here's the full implementation:

```py
from __future__ import annotations
from typing import Any, Literal


class _ANY:
    "A helper object that compares equal to everything."

    def __eq__(self, other: Any) -> Literal[True]:
        return True

    def __ne__(self, other: Any) -> Literal[False]:
        return False

    def __repr__(self) -> str:
        return "<ANY>"


ANY = _ANY()
```

It always returns `True` whenever compared with some value:

```py
In [1]: from unittest.mock import ANY

In [2]: ANY == 1
Out[2]: True

In [3]: ANY == "anything"
Out[3]: True

In [4]: ANY == True
Out[4]: True

In [5]: ANY == False
Out[5]: True

In [6]: ANY == None
Out[6]: True
```

[^1]: [unittest.mock.ANY](https://docs.python.org/3/library/unittest.mock.html#any) [^1]

[^2]:
    [ANY in the wild](https://github.com/rednafi/example-rq-sentry/blob/9630e8ae31197fea6606a1972a108fa70de8b331/tests/test_tasks.py#L19)
    [^2]

---
title: Mocking chained methods of datetime objects in Python
date: 2022-03-16
tags:
    - Python
    - Testing
---

This is the 4th time in a row that I've wasted time figuring out how to mock out a function
during testing that calls the chained methods of a `datetime.datetime` object in the
function body. So I thought I'd document it here. Consider this function:

```python
# src.py
from __future__ import annotations

import datetime


def get_utcnow_isoformat() -> str:
    """Get UTCnow as an isoformat compliant string."""
    return datetime.datetime.utcnow().isoformat()
```

How'd you test it? Mocking out `datetime.datetime` is tricky because of its immutable
nature. Third-party libraries like freezegun[^1] make it easier to mock and test functions
like the one above. However, it's not too difficult to cover this simple case without any
additional dependencies. Here's one way to achieve the goal:

```python
# src.py
from __future__ import annotations

import datetime
from unittest.mock import patch
import pytest


def get_utcnow_isoformat() -> str:
    """Get UTCnow as an isoformat compliant string."""
    return datetime.datetime.utcnow().isoformat()


@pytest.fixture
def mock_datetime():
    with patch("datetime.datetime") as m:
        # This is where the magic happens!
        m.utcnow.return_value.isoformat.return_value = (
            "2022-03-15T23:11:12.432048"
        )
        yield m


def test_get_utcnow_isoformat(mock_datetime):
    frozen_date = "2022-03-15T23:11:12.432048"
    assert get_utcnow_isoformat() == frozen_date
```

Here, the `mock_datetime` fixture function makes the output of the chained calls on the
datetime object deterministic. Then I used it in the `test_get_utcnow_isoformat` function to
get a frozen output every time the function `get_utcnow_isoformat` gets called. If you run
the above snippet with Python, it'll pass.

```txt
======test session starts ======
platform linux -- Python 3.10.2, pytest-7.0.1, pluggy-1.0.0
rootdir: /home/rednafi/canvas/personal/reflections
plugins: anyio-3.5.0
collected 1 item

src.py .                                              [100%]

====== 1 passed in 0.01s ======
```

[^1]: [freezegun](https://github.com/spulec/freezegun)

[^2]:
    [Python test using mock with datetime.utcnow — Stackoverflow](https://stackoverflow.com/questions/57671585/python-test-using-mock-with-datetime-utcnow)
    [^2]

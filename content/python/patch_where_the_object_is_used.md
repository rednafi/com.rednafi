---
title: Patch where the object is used
date: 2022-07-18
slug: patch-where-the-object-is-used
aliases:
    - /python/patch_where_the_object_is_used/
tags:
    - Python
    - Testing
---

I was reading Ned Bachelder's blog "Why your mock doesn't work"[^1] and it triggered an
epiphany in me about a testing pattern that I've been using for a while without being aware
that there might be an aphorism on the practice.

> Patch where the object is used; not where it's defined.

To understand it, consider the example below. Here, you have a module containing a function
that fetches data from some fictitious database.

```py
# db.py
from __future__ import annotations
import random


def get_data() -> list[int]:
    # ...run some side effects and return data
    # from a fictitous database.
    return [random.randint(100, 200) for _ in range(4)]
```

Let's say another module named `service.py` imports the `get_data` function and calls that
inside of a function named `process_data`:

```py
# service.py
from __future__ import annotations

from db import get_data


def process_data() -> list[int]:
    data = get_data()
    # ... do some processing.
    return data
```

Now, let's say we want to write a test for the `service.process_data` function. Since the
function depends on `db.get_data`, we'll patch the `get_data` function and replace it with a
mock object that returns a canned response. This will make sure that calling `process`
doesn't invoke the real `get_data` which might have side effects that we don't want to
trigger during test runs. Also, in this case, instead of returning a list of pseudo-random
integers, the replaced `get_data` function will deterministically return a list of known
integers.

You could patch `get_data` in multiple ways. Here's the first attempt:

```py
# test_service.py
from unittest.mock import patch

from service import process


# Patching happens here!
@patch("db.get_data", return_value=[1, 2, 3, 4], autospec=True)
def test_process(mock_get_data):
    # Call the target function.
    result = process()

    # Check the result.
    assert result == [1, 2, 3, 4]

    # Check that get_data was called.
    mock_get_data.assert_called_once()
```

Since `get_data` is defined in the `db.py` module, we pass `db.get_data` to the `patch`
decorator. Unfortunately, if you run the above test with pytest[^2], you'll see that the
test fails with the following error:

```txt
test_service.py F                                      [100%]

========== FAILURES ==========
__________ test_process __________

mock_get_data = <function get_data at 0x7fc8b04d6440>

    @patch(
        "db.get_data",
        return_values=[1, 2, 3, 4], autospec=True
    )
    def test_process(mock_get_data):

        # Call the target function.
        result = process()

        # Check the result.
>       assert result == [1, 2, 3, 4]
E       assert [184, 112, 189, 135] == [1, 2, 3, 4]
E         At index 0 diff: 184 != 1
E         Use -v to get more diff

test_src.py:13: AssertionError
========== short test summary info ==========
FAILED test_src.py::test_process
    - assert [184, 112, 189, 135] == [1, 2, 3, 4]
========== 1 failed in 0.14s ==========
```

The original implementation of `get_data` returns a list of 4 pseudo-random integers where
the values lie between 100 and 200 whereas our patched version of `get_data` always returns
`[1, 2, 3, 4]`. So, the test is failing because the `get_data` function didn't get patched
properly and it's calling the original `get_data` function during the test run.

While the function `get_data` is defined in the `db.py` module, it's actally used in the
`service.py` module. So, we can avoid this missing target issue by patching `get_data` in
the location where it's used; not where it's defined. Here's how to do it:

```py
# test_service.py

# Notice how we're patching 'get_data' in the 'service.py' module.
@patch("service.get_data", return_value=[1, 2, 3, 4], autospec=True)
def test_process(mock_get_data):
    # ...rest of the test implementation is the same as before.
```

This time, when you run the tests, pytest doesn't complain.

[^1]:
    [Why your mock doesn't work](https://nedbatchelder.com/blog/201908/why_your_mock_doesnt_work.html)

[^2]: [pytest](https://docs.pytest.org/en/latest/)

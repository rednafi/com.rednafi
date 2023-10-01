---
title: Use daemon threads to test infinite while loops in Python
date: 2021-11-18
tags:
    - Python
---

Python's daemon threads are cool. A Python script will stop when the main thread is done and
only daemon threads are running. To test a simple `hello` function that runs indefinitely,
you can do the following:

```python
# test_hello.py
from __future__ import annotations

import asyncio
import threading
from functools import partial
from unittest.mock import patch


async def hello() -> None:
    while True:
        await asyncio.sleep(1)
        print("hello")


@patch("asyncio.sleep", autospec=True)
async def test_hello(mock_asyncio_sleep, capsys):
    run = partial(asyncio.run, hello())
    t = threading.Thread(target=run, daemon=True)
    t.start()
    t.join(timeout=0.1)

    out, err = capsys.readouterr()
    assert err == ""
    assert "hello" in out
    mock_asyncio_sleep.assert_awaited()
```

To execute the script, make sure you've your virtual env actiavated. Also you'll need to
install `pytest` and `pytest-asyncio`. Then run:

```sh
pytest -v -s --asyncio-mode=auto
```

The idea came from this quora answer[^1].

[^1]: [How do we test an infinite loop in Python using the unittest library?](https://qr.ae/pGDHVw)

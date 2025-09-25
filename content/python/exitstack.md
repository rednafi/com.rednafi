---
title: ExitStack in Python
date: 2022-08-27
tags:
    - Python
slug: exitstack
aliases:
    - /python/exitstack/
---

Over the years, I've used Python's `contextlib.ExitStack` in a few interesting ways. The
official documentation[^1] advertises it as a way to manage multiple context managers and
has a couple of examples of how to leverage it. However, neither in the docs nor in GitHub
code search[^2] I could find examples of some of the maybe unusual ways I've used it in the
past. So, I thought I'd document them here.

## Enforcing request level transaction

While consuming APIs, it's important to handle errors in a way that prevents database state
corruption. In the following example, I'm making two `POST` requests to an API and rolling
back to the original state if any one of them fails:

```py
# src.py
from __future__ import annotations

import logging
import uuid
from contextlib import ExitStack
from http import HTTPStatus

import httpx

logging.basicConfig(level=logging.INFO)


def group_create(uuid_a: str, uuid_b: str) -> tuple[httpx.Response, ...]:
    with httpx.Client() as client:
        url = "https://httpbin.org/post"
        response_a = client.post(
            url,
            json={"uuid": uuid_a, "foo": "bar"},
        )
        response_b = client.post(
            url,
            json={"uuid": uuid_b, "fizz": "bazz"},
        )
    return response_a, response_b


def maybe_rollback(
    uuid: str,
    incoming_status_code: int,
    expected_status_code: int = HTTPStatus.OK,
) -> None:
    if incoming_status_code != expected_status_code:
        logging.info(f"Rolling back request: {uuid}")
        url = f"https://httpbin.org/delete?uuid={uuid}"
        response = httpx.delete(url)
        assert response.status_code == HTTPStatus.OK
    else:
        logging.info(f"Request {uuid} completed successfully.")


def main() -> None:
    with ExitStack() as stack:
        uuid_a = str(uuid.uuid4())
        uuid_b = str(uuid.uuid4())

        response_a, response_b = group_create(uuid_a, uuid_b)
        stack.callback(
            maybe_rollback,
            uuid=uuid_a,
            incoming_status_code=response_a.status_code,
        )
        stack.callback(
            maybe_rollback,
            uuid=uuid_b,
            incoming_status_code=response_b.status_code,
        )


if __name__ == "__main__":
    main()
```

Running this will print the following output:

```txt
INFO:root:Request fec8fc9f-7762-4d53-b8f9-3dc7802108a4 completed successfully.
INFO:root:Request 4b6ed0ed-b7cf-46f0-9374-85627be4c26c completed successfully.
```

Here, the `group_create` function makes two calls to `POST httpbin.org/post` endpoint and
the `maybe_rollback` function deletes the created record if any one of the two requests
fails. In the `main` function, I've used the `ExitStack.callback` method to register the
`maybe_rollback` callback. If you change the `expected_status_code` in the `maybe_rollback`
function to something like `HTTPStatus.FORBIDDEN`, you'll be able to see the cleanup
callbacks in action:

```txt
INFO:root:Rolling back request: 50eb2734-f84c-4013-b5f6-0ccf1aa5d79a
INFO:root:Rolling back request: b326e567-a006-4648-bf04-202397f44e31
```

## Invoking conditional event hooks

The same strategy used in the previous section can be applied to invoke event hooks
conditionally. For example, let's say you want to run a callback function when some event
function executes. However, you want only a particular type of callback function to be
executed depending on the state of your conditionals or code path. I've found the following
pattern useful in this case:

```py
# src.py
from __future__ import annotations

from contextlib import ExitStack
from typing import Any


class EventHook:
    def __init__(self, event_name: str) -> None:
        self.event_name = event_name
        self.dispatch_config = {
            "success": self.on_success,
            "failure": self.on_failure,
        }

    def on_success(self) -> None:
        print(f"'{self.event_name}' hook called")

    def on_failure(self) -> None:
        print(f"'{self.event_name}' hook called")

    def __call__(self) -> Any:
        return self.dispatch_config[self.event_name]()


def successful_event() -> None:
    print("'successful_event' executed")


def failed_event() -> None:
    print("'failed_event' executed")
    1 / 0


def main() -> None:
    success_hook = EventHook("success")
    failure_hook = EventHook("failure")

    with ExitStack() as stack:
        try:
            # Run successful event and attach success hook.
            successful_event()
            stack.callback(success_hook)

            failed_event()
        except ZeroDivisionError:
            # When the failed even raises an error, attach failure hook.
            stack.callback(failure_hook)


if __name__ == "__main__":
    main()
```

```txt
'successful_event' executed
'failed_event' executed
'failure' hook called
'success' hook called
```

Here the `.on_failure` hook will only be called if there's an error in your execution path
raises an exception.

## Avoiding nested context structure

It can get ugly pretty quickly when you start using multiple nested context managers. For
example, if you need to open two files and copy content from one file to the other, you'd
typically start two nested context managers and transfer the content like this:

```py
# src.py
with open("file1.md") as f1:
    with open("file2.md") as f2:
        # Copy content from f1 to f2 and save it.
```

`ExitStack` can help you get away with only one level of nesting here. Here's a complete
example:

```py
# src.py
import io
import shutil
import tempfile
from contextlib import ExitStack


def copy_over(
    fsrc: io.IOBase,
    fdst: io.IOBase,
    skip_line: int = 0,
) -> None:
    if skip_line > 0:
        for _ in range(skip_line):
            fsrc.readline()
    shutil.copyfileobj(fsrc, fdst)


def main() -> None:
    with ExitStack() as stack:
        # Enter into the respective context managers without explicit
        # 'with' blocks.
        fsrc = stack.enter_context(
            tempfile.SpooledTemporaryFile(mode="rb"),
        )
        fdst = stack.enter_context(
            tempfile.SpooledTemporaryFile(mode="rb+"),
        )

        # Write some data to the source file.
        fsrc.write(b"hello world\nhello mars")

        # Rewind the source file and copy it to the destination file.
        fsrc.seek(0)
        copy_over(fsrc, fdst, skip_line=1)

        # Rewind the destination file and assert the data.
        fdst.seek(0)
        assert fdst.read() == b"hello mars"

        # Rewind the dst file and print out the shape of the fdst
        # content.
        fdst.seek(0)
        print(fdst.read())


if __name__ == "__main__":
    main()
```

This example creates two in-memory temporary file instances with
`tempfile.SpooledTemporaryFile`. The `SpooledTemporaryFile` can be used as a context
manager. However, instead of nesting the two instances, I'm using `ExitStack.enter_context`
to enter into the context manager without explicitly using the `with` statement. This
`.enter_context` method ensures that the `__exit__` method of the respective context
managers will be called properly at the end of the `main()` function run.

Then in the body of the `ExitStack`, we're writing some content to the first in-memory file
and then copying the content to the other in-memory file. If we had to open and manage even
more context managers, in this way, we'd be able to that without crating any additional
nestings.

## Applying multiple patches as context managers

Python's `unittest.mock.patch` can be used as both decorators and context managers. For
granular patching and unpatching during tests, the context manager approach gives you more
control than its decorator counterpart. In this case, `ExitStack` can help you avoid
multiple nestings just like in the previous section:

```py
# src.py
from __future__ import annotations

from contextlib import ExitStack
from http import HTTPStatus
from typing import Any
from unittest.mock import patch
import httpx


def get(url: str) -> dict[str, Any]:
    return httpx.get(url).json()


def post(url: str, data: dict[str, Any]) -> dict[str, Any]:
    return httpx.post(url, json=data).json()


def main() -> dict[str, Any]:
    res_get = get("https://httpbin.org/get")
    res_post = post("https://httpbin.org/post", {"foo": "bar"})

    return {"get": res_get, "post": res_post}


def test_main() -> None:
    with ExitStack() as stack:
        # Arrange
        mock_httpx_get = stack.enter_context(
            patch(
                "httpx.get",
                autospec=True,
                return_value=httpx.Response(
                    json={"fizz": "bazz"}, status_code=HTTPStatus.OK
                ),
            ),
        )
        mock_httpx_post = stack.enter_context(
            patch(
                "httpx.post",
                autospec=True,
                return_value=httpx.Response(
                    json={"foo": "bar"}, status_code=HTTPStatus.CREATED
                ),
            )
        )

        # Act
        res = main()

        # Assert
        assert res["get"]["fizz"] == "bazz"
        assert res["post"]["foo"] == "bar"
        assert mock_httpx_get.call_count == 1
        assert mock_httpx_post.call_count == 1
```

Running the above snippet with pytest will reveal that the test passes without any error:

```txt
src.py::test_main PASSED

======================= 1 passed in 0.11s =======================
```

Here, I'm making `GET` and `POST` requests with the `httpx` library and in the `test_main`
function, the `httpx.get` and `httpx.post` callable are patched with the `patch` context
manager. However, `ExitStack` allows me here to do it without creating additional nested
`with` blocks.

[^1]: [ExitStack](https://docs.python.org/3/library/contextlib.html#contextlib.ExitStack)

[^2]: [GitHub code search](https://github.com/search?l=Python&q=ExitStack&type=Code)

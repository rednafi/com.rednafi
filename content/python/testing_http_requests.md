---

title: Shades of testing HTTP requests in Python
date: 2024-09-02
tags:
    - API
    - Testing
    - TIL

---

Here’s a Python snippet that makes an HTTP POST request:

```python
# script.py

import httpx
from typing import Any


async def make_request(url: str) -> dict[str, Any]:
    headers = {"Content-Type": "application/json"}

    async with httpx.AsyncClient(headers=headers) as client:
        response = await client.post(
            url,
            json={"key_1": "value_1", "key_2": "value_2"},
        )
        return response.json()
```

The function `make_request` makes an async HTTP request with the httpx[^1] library. Running
this with `asyncio.run(make_request("https://httpbin.org/post"))` gives us the following
output:

```json
{
  "args": {},
  "data": "{\"key_1\": \"value_1\", \"key_2\": \"value_2\"}",
  "files": {},
  "form": {},
  "headers": {
    "Accept": "*/*",
    "Accept-Encoding": "gzip, deflate",
    "Content-Length": "40",
    "Content-Type": "application/json",
    "Host": "httpbin.org",
    "User-Agent": "python-httpx/0.27.2",
    "X-Amzn-Trace-Id": "Root=1-66d5f7b0-2ed0ddc57241f0960f28bc91"
  },
  "json": {
    "key_1": "value_1",
    "key_2": "value_2"
  },
  "origin": "95.90.238.240",
  "url": "https://httpbin.org/post"
}
```

We're only interested in the `json` field and want to assert in our test that making the
HTTP call returns the expected values.

## Testing the HTTP request

Now, how would you test it? One approach is by patching the `httpx.AsyncClient` instance to
return a canned response and asserting against that. The happy path might be tested as
follows:

```python
# test_script.py

from unittest.mock import AsyncMock, patch
import pytest
from script import make_request


@pytest.mark.asyncio
async def test_make_request_ok() -> None:
    url = "https://httpbin.org/post"
    expected_json = {"key_1": "value_1", "key_2": "value_2"}

    # Create a mock response object
    mock_response = AsyncMock()
    mock_response.json.return_value = expected_json
    mock_response.status_code = 200

    # Patch the httpx.AsyncClient.post method to return the mock_response
    with patch(
        "script.httpx.AsyncClient.post",  # Don't mock what you don't own
        return_value=mock_response,
    ) as mock_post:
        response = await make_request(url)

        # Await the coroutine that was returned
        response = await response

        # Assertions
        mock_post.assert_called_once_with(url, json=expected_json)
        assert response == expected_json
```

That’s quite a bit of work just to test a simple HTTP request. The mocking gets pretty hairy
as the complexity of your HTTP calls increases. One way to reduce this complexity is by
using a library like respx[^2] that handles the patching for you.

## Simplifying mocks with respx

For instance:

```python
# test_script.py

import pytest
import respx
from script import make_request, httpx


@pytest.mark.asyncio
async def test_make_request_ok() -> None:
    url = "https://httpbin.org/post"
    expected_json = {"key_1": "value_1", "key_2": "value_2"}

    # Mocking the HTTP POST request using respx
    with respx.mock:
        respx.post(url).mock(
            return_value=httpx.Response(200, json=expected_json)
        )

        # Calling the function
        response = await make_request(url)

        # Assertions
        assert response == expected_json
```

Much cleaner. Here, respx intercepts HTTP requests made by httpx during tests, allowing you
to mock responses easily. It provides a context manager to define how specific requests
should be handled, returning custom responses. This avoids the need to manually patch
methods like `post` in `httpx.AsyncClient`.

## Testing with a stub client

The previous strategy wouldn’t work if you want to change your HTTP client since respx is
coupled with httpx. As an alternative, you could rewrite `make_request` to parametrize the
HTTP client, pass a stub object during the test, and test against it. This eliminates the
need to write fragile mocking sludges or depend on an external mocking library.

Here’s how you’d change the code:

```python
# script.py

import httpx
import asyncio
from typing import Any


async def make_request(url: str, client: httpx.AsyncClient) -> dict[str, Any]:
    # We don't want to initiate the ctx manager in every request
    # AsyncClient.__enter__(...) will be called once and passed to this function
    response = await client.post(
        url,
        json={"key_1": "value_1", "key_2": "value_2"},
    )
    return response.json()


async def main() -> None:
    headers = {"Content-Type": "application/json"}
    url = "https://httpbin.org/post"

    # Enter into the context manager and pass the instance to make_request
    async with httpx.AsyncClient(headers=headers) as client:
        response = await make_request(url, client)
        print(response)
```

Now the tests would look as follows:

```python
import pytest
from typing import Any
from httpx import Response, Request, AsyncClient
from script import make_request


class StubAsyncClient(AsyncClient):
    async def post(
        self, url: str, json: Any = None, **kwargs: Any
    ) -> Response:
        request = Request(method="POST", url=url, json=json, **kwargs)
        # Simulate the original response that matches the request
        response = Response(
            status_code=200,
            json={"key_1": "value_1", "key_2": "value_2"},
            request=request,
        )
        return response


@pytest.mark.asyncio
async def test_make_request_ok() -> None:
    url = "https://httpbin.org/post"
    headers = {"Content-Type": "application/json"}
    client = StubAsyncClient(headers=headers)
    response_data = await make_request(url, client)

    assert response_data == {"key_1": "value_1", "key_2": "value_2"}
```

Much better!

## Integration testing with a test server

One thing I’ve picked up from writing Go is that it’s often just easier to perform
integration tests on these I/O-bound functions. That is, you can spin up a server that
returns a canned response and then test your code against it to assert if it's getting the
expected output.

The test could look as follows. This assumes `make_request` takes in an `AsyncClient`
instance as a parameter, as shown in the last example.

```python
import pytest
from starlette.applications import Starlette
from starlette.responses import JSONResponse
from starlette.routing import Route
from starlette.requests import Request
from httpx import AsyncClient
from script import make_request


async def test_endpoint(request: Request) -> JSONResponse:
    return JSONResponse({"key_1": "value_1", "key_2": "value_2"})


app = Starlette(routes=[Route("/post", test_endpoint, methods=["POST"])])


@pytest.mark.asyncio
async def test_make_request() -> None:
    # Manually create the AsyncClient
    async with AsyncClient(app=app, base_url="http://testserver") as client:
        url = "http://testserver/post"
        response = await make_request(url, client=client)
        assert response == {"key_1": "value_1", "key_2": "value_2"}
```

In the above test, we’re using starlette[^3] to define a simple ASGI server that returns our
expected response. Then we set up the `httpx.AsyncClient` so it makes the request against
the test server instead of making an external network call. Finally, we call the
`make_request` function and assert the expected payload.

Sure, you could set up the server with the standard library’s `http` module, but that code
doesn't look half as pretty.

[^1]: [httpx](https://www.python-httpx.org/)

[^2]: [respx](https://lundberg.github.io/respx/)

[^3]: [starlette](https://www.starlette.io/)

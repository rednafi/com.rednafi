---
title: Limit concurrency with semaphore in Python asyncio
date: 2022-02-10
tags:
    - Python
---

I was working with a rate-limited API endpoint where I continuously needed to send
short-polling GET requests without hitting HTTP 429 error. Perusing the API doc, I found out
that the API endpoint only allows a maximum of 100 requests per second. So, my goal was to
find out a way to send the maximum amount of requests without encountering the
too-many-requests error.

I picked up Python's [asyncio] and the amazing [HTTPx] library by Tom Christie to make the
requests. This is the naive version that I wrote in the beginning; it quickly hits the HTTP
429 error:

```python
# src.py
from __future__ import annotations

import asyncio
from http import HTTPStatus
from pprint import pprint

import httpx

# Reusing http client allows us to reuse a pool of TCP connections.
client = httpx.AsyncClient()


async def make_one_request(url: str, num: int) -> httpx.Response:
    headers = {"Content-Type": "application/json"}

    print(f"Making request {num}")
    r = await client.get(url, headers=headers)

    if r.status_code == HTTPStatus.OK:
        return r

    raise ValueError(
        f"Unexpected Status: Http status code is {r.status_code}.",
    )


async def make_many_requests(url: str, count: int) -> list[httpx.Response]:
    tasks = []
    for num in range(count):
        task = asyncio.create_task(make_one_request(url, num))
        tasks.append(task)

    results = await asyncio.gather(*tasks)

    # All the results will look the same, so we're just printing one.
    print("\n")
    print("Final result:")
    print("==============\n")
    pprint(results[0].json())

    return results


if __name__ == "__main__":
    asyncio.run(make_many_requests("https://httpbin.org/get", count=200))
```

Here, for this demonstration, I'm using the `https://httpbin.org/get` endpoint that's openly
accessible. This particular endpoint doesn't impose any limit on the number of requests per
second. However, in the above snippet, if you inspect the `for` loop in the
`make_many_requests` function, you'll see that it's sending 200 concurrent requests without
any restrictions.

Also, the snippet will raise a `ValueError` if it encounters an HTTP-429-too-many-requests
error. Running the script produces the following output:

```txt
Making request 0
Making request 1
Making request 2
Making request 3
Making request 4
Making request 5
Making request 6
...
Making request 199


Final result:
==============

{'args': {},
 'headers': {'Accept': '*/*',
             'Accept-Encoding': 'gzip, deflate',
             'Content-Type': 'application/json',
             'Host': 'httpbin.org',
             'User-Agent': 'python-httpx/0.22.0',
             'X-Amzn-Trace-Id': 'Root=1-62042fc6-007ccd7d6b2cf5c15c0963f6'},
 'origin': '103.84.246.3',
 'url': 'https://httpbin.org/get'}

```

From the output, it's pretty evident that the script is hammering the server without any
delay between the concurrent requests. While 200 requests per second may not be that high
but even if there weren't any restrictions, sending so many rogue requests like that isn't
desirable. It's easy to overwhelm any service if you're not being careful.

Luckily, Python exposes a `Semaphore` construct that allows you to synchronize the
concurrent workers (threads, processes, or coroutines) regarding how they should access a
shared resource. All concurrency primitives in Python have semaphores to help you control
resource access. This means if you're using any of the—`multiprocessing`, `threading`, or
`asyncio` module, you can take advantage of it. From the `asyncio` docs:

> _A semaphore manages an internal counter which is decremented by each `acquire()` call and
> incremented by each `release()` call. The counter can never go below zero; when
> `acquire()` finds that it is zero, it blocks, waiting until some task calls `release()`._

You can use the semaphores in the above script as follows:

```python
...
# Initialize a semaphore object with a limit of 3.
limit = asyncio.Semaphore(3)


async def make_one_request(url: str, num: int) -> httpx.Response:
    headers = {"Content-Type": "application/json"}

    # No more than 3 concurrent workers will be able to make
    # get request at the same time.
    async with limit:
        print(f"Making request {num}")
        r = await client.get(url, headers=headers)

        # When workers hit the limit, they'll wait for a second
        # before making more requests.
        if limit.locked():
            print("Concurrency limit reached, waiting ...")
            await asyncio.sleep(1)

        if r.status_code == HTTPStatus.OK:
            return r

    raise ValueError(
        f"Unexpected Status: Http status code is {r.status_code}.",
    )


...
```

Here, I only had to change the `make_one_request` function to take advantage of the
semaphore. First, I initialized an `asyncio.Semaphore` object with the limit `3`. This means
the semaphore won't allow more than three concurrent workers to make HTTP GET requests at
the same time. The semaphore instance is then used as a context manager. Inside the
`async with` block, the line starting with `if limit.locked()` makes the workers wait for a
second whenever the concurrency limit is reached. If you execute the script, it'll produce
the following output:

```txt
Making request 0
Making request 1
Making request 2
Concurrency limit reached, waiting ...
Concurrency limit reached, waiting ...
Concurrency limit reached, waiting ...
Making request 3
Making request 4
Making request 5
Concurrency limit reached, waiting ...
Concurrency limit reached, waiting ...
Concurrency limit reached, waiting ...
Making request 6
Making request 7
Making request 8
...
Making request 199
...
```

The output makes it clear that no more than 3 async functions are making concurrent requests
to the server at the same time. You can tune the number of concurrent workers by changing
the limit in the `asyncio.Semaphore` object.

## Complete script

```python
# src.py
from __future__ import annotations

import asyncio
from http import HTTPStatus
from pprint import pprint

import httpx

# Reusing http client allows us to reuse a pool of TCP connections.
client = httpx.AsyncClient()

# Initialize a semaphore object with a limit of 3.
limit = asyncio.Semaphore(3)


async def make_one_request(url: str, num: int) -> httpx.Response:
    headers = {"Content-Type": "application/json"}

    # No more than 3 concurrent workers will be able to make
    # get request at the same time.
    async with limit:
        print(f"Making request {num}")
        r = await client.get(url, headers=headers)

        # When workers hit the limit, they'll wait for a second
        # before making more requests.
        if limit.locked():
            print("Concurrency limit reached, waiting ...")
            await asyncio.sleep(1)

        if r.status_code == HTTPStatus.OK:
            return r

    raise ValueError(
        f"Unexpected Status: Http status code is {r.status_code}.",
    )


async def make_many_requests(url: str, count: int) -> list[httpx.Response]:
    tasks = []
    for num in range(count):
        task = asyncio.create_task(make_one_request(url, num))
        tasks.append(task)

    results = await asyncio.gather(*tasks)

    # All the results will look the same, so we're just printing one.
    print("\n")
    print("Final result:")
    print("==============\n")
    pprint(results[0].json())

    return results


if __name__ == "__main__":
    asyncio.run(make_many_requests("https://httpbin.org/get", count=200))
```

<!-- Resources -->
<!-- prettier-ignore-start -->

[asyncio]:
    https://docs.python.org/3/library/asyncio.html

[httpx]:
    https://www.python-httpx.org/

<!-- prettier-ignore-end -->

---
title: Using tqdm with concurrent.fututes in Python
date: 2023-01-06
tags:
    - Python
---

At my workplace, I was writing a script to download multiple files from different S3
buckets. The script relied on Django ORM, so I couldn't use Python's async paradigm to speed
up the process. Instead, I opted for `boto3` to download the files and
`concurrent.futures.ThreadPoolExecutor` to spin up multiple threads and make the requests
concurrently.

However, since the script was expected to be long-running, I needed to display progress bars
to show the state of execution. It's quite easy to do with `tqdm` when you're just looping
over a list of file paths and downloading the contents synchronously:

```python
from tqdm import tqdm

for file_path in tqdm(file_paths):
    download_file(file_path)
```

But you can't do this when multiple threads or processes are doing the work. Here's what
I've found that works quite well:

```python
from __future__ import annotations

import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import Generator

import httpx
from tqdm import tqdm


def make_request(url: str) -> dict:
    with httpx.Client() as client:
        response = client.get(url)
        # Additional delay to simulate a slow request.
        time.sleep(1)

        return response.json()


def make_requests(
    urls: list[str],
) -> Generator[list[dict], None, None]:
    with tqdm(total=len(urls)) as pbar:
        with ThreadPoolExecutor(max_workers=5) as executor:
            futures = [
                executor.submit(make_request, url) for url in urls
            ]
            for future in as_completed(futures):
                pbar.update(1)
                yield future.result()


def main() -> None:
    urls = [
        "https://httpbin.org/get",
        "https://httpbin.org/get?foo=bar",
        "https://httpbin.org/get?foo=baz",
        "https://httpbin.org/get?foo=qux",
        "https://httpbin.org/get?foo=quux",
    ]

    results = []
    for result in make_requests(urls):
        results.append(result)

    print(results)


if __name__ == "__main__":
    main()
```

Running this will print:

```txt
100%|█████████████████████████████████████████████████████| 5/5
                                        [00:01<00:00,  3.51it/s]
...
```

This script makes 5 concurrent requests by leveraging `ThreadPoolExecutor` from the
`concurrent.futures` module. The `make_request` function just sends one request to a URL and
sleeps for a second to simulate a long-running task. Then the `make_requests` function spins
up 5 threads and calls the `make_request` function in each one with a different URL.

Here, we're instantiating `tqdm` as a context manager and passing the total length of the
`urls`. This allows `tqdm` to calculate the progress bar. Then in a nested context manager,
we spin up the threads and pass the `make_request` to the `executor.submit` method. We
collect the future objects returned by the `executor.submit` methods in a list and update
the progress bar with `pbar.update(1)` while iterating through the futures. And that's it,
mission successful.

I usually use `contextlib.ExitStack` to avoid nested context managers like this:

```python
...

from contextlib import ExitStack


def make_requests(
    urls: list[str],
) -> Generator[list[dict], None, None]:
    with ExitStack() as stack:
        executor = stack.enter_context(
            ThreadPoolExecutor(max_workers=5)
        )
        pbar = stack.enter_context(tqdm(total=len(urls)))

        futures = [executor.submit(make_request, url) for url in urls]
        for future in as_completed(futures):
            pbar.update(1)
            yield future.result()


...
```

Running this script will yield the same result as before.

[^1]:
    [How to use tqdm with multithreading?](https://stackoverflow.com/questions/63826035/how-to-use-tqdm-with-multithreading)
    [^1]

---
title: Disallow large file download from URLs in Python
date: 2022-03-23
slug: disallow-large-file-download
aliases:
    - /python/disallow_large_file_download/
tags:
    - Python
---

I was working on a DRF POST API endpoint where the consumer is expected to add a URL
containing a PDF file and the system would then download the file and save it to an S3
bucket. While this sounds quite straightforward, there's one big issue. Before I started
working on it, the core logic looked like this:

```py
# src.py
from __future__ import annoatations

from urllib.request import urlopen
import tempfile
from shutil import copyfileobj


def save_to_s3(src_url: str, dest_url: str) -> None:
    with tempfile.NamedTemporaryFile() as file:
        with urlopen(src_url) as response:
            # This stdlib function saves the content of the file
            # in 'file'.
            copyfileobj(response, file)

        # Logic to save file in s3.
        _save_to_s3(des_url)


if __name__ == "__main__":
    save_to_s3(
        "https://citeseerx.ist.psu.edu/viewdoc/download?"
        "doi=10.1.1.92.4846&rep=rep1&type=pdf",
        "https://s3-url.com",
    )
```

In the above snippet, there's no guardrail against how large the target file can be. You
could bring the entire server down to its knees by posting a link to a ginormous file. The
server would be busy downloading the file and keep consuming resources.

I didn't want to use `urllib` at all for this purpose and went for HTTPx[^1]. It exposes a
neat API to perform streaming file download. Also, I didn't want to peek into the
`Content-Length` header to assess the file size since the file server can choose not to
include that header key. I was looking for something more dependable than that. Here's how I
solved it:

```py
# src
from __future__ import annotations

import httpx
import tempfile


def save_to_s3(
    src_url: str,
    dest_url: str,
    chunk_size: int = 1024 * 1024,  # 1 MB buffer.
    max_size: int = 10 * 1024 * 1024,  # 10 MB
) -> None:
    # Keep track of the already downloaded byte length.
    downloaded_content_length = 0  # bytes

    with tempfile.NamedTemporaryFile() as file:
        with httpx.stream("GET", src_url) as response:
            for chunk in response.iter_bytes(chunk_size):
                downloaded_content_length += len(chunk)
                if downloaded_content_length > max_size:
                    raise ValueError(
                        f"File size too large. Make sure your linked "
                        "file is not larger than 10 MB."
                    )
                file.write(chunk)

        # logic to save file in s3.
        _save_to_s3(dest_url)


if __name__ == "__main__":
    save_to_s3(
        "https://citeseerx.ist.psu.edu/viewdoc/download?"
        "doi=10.1.1.92.4846&rep=rep1&type=pdf",
        "",
    )
```

The `chunk_size` parameter explicitly dictates the buffer size of the file being downloaded.
This means the entire file won't be loaded into memory while being downloaded. The
`max_size` parameter defines the maximum file size that'll be allowed. In this example,
we're keeping track of the size of the already downloaded bytes in the
`downloaded_content_length` variable and raising an error if the size exceeds 10MB. Sweet!

[^1]: [HTTPx](https://www.python-httpx.org/)

[^2]:
    [Streaming download with HTTPx](https://www.python-httpx.org/advanced/#monitoring-download-progress)
    [^2]

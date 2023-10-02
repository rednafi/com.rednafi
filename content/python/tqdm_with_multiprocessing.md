---
title: Running tqdm with Python multiprocessing
date: 2021-11-18
tags:
    - Python
    - TIL
---

Making tqdm play nice with multiprocessing requires some additional work. It's not always
obvious and I don't want to add another third-party dependency just for this purpose.

The following example attempts to make tqdm work with `multiprocessing.imap_unordered`.
However, this should also work with similar mapping methods like—`multiprocessing.map`,
`multiprocessing.imap`, `multiprocessing.starmap`, etc.

```python
"""
Run `pip install tqdm` before running the script.

The function `foo` is going to be executed 100 times across
`MAX_WORKERS=5` processes. In a single pass, each process will
get an iterable of size `CHUNK_SIZE=5`. So 5 processes each consuming
5 elements of an iterable will require (100 / (5*5)) 4 passes to finish
consuming the entire iterable of 100 elements.

Tqdm progress bar will be updated after every `MAX_WORKERS*CHUNK_SIZE` iterations.
"""
# src.py


from __future__ import annotations

import multiprocessing as mp

from tqdm import tqdm
import time

import random
from dataclasses import dataclass

MAX_WORKERS = 5
CHUNK_SIZE = 5


@dataclass
class StartEnd:
    start: int
    end: int


def foo(start_end: StartEnd) -> int:
    time.sleep(0.2)
    return random.randint(start_end.start, start_end.end)


def main() -> None:
    inputs = [
        StartEnd(start, end)
        for start, end in zip(
            range(0, 100),
            range(100, 200),
        )
    ]

    with mp.Pool(processes=MAX_WORKERS) as pool:
        results = tqdm(
            pool.imap_unordered(foo, inputs, chunksize=CHUNK_SIZE),
            total=len(inputs),
        )  # 'total' is redundant here but can be useful
        # when the size of the iterable is unobvious

        for result in results:
            print(result)


if __name__ == "__main__":
    main()
```

This will print:

```txt
0%|                                                 | 0/100 [00:00<?, ?it/s]
14
1%|▌                                                | 1/100 [00:01<01:39,  1.00s/it]
6
9
70
...

26%|██████████████▎                                 | 26/100 [00:02<00:04, 15.10it/s]
70
42
41
...
51%|████████████████████████████                    | 51/100 [00:03<00:02, 19.61it/s]
114
135
59
...
76%|█████████████████████████████████████████▊      | 76/100 [00:04<00:01, 21.72it/s]
134
106
167
...
100%|██████████████████████████████████████████████████████| 100/100 [00:04<00:00]
```

[^1]: [Using tqdm with multiprocessing](https://stackoverflow.com/questions/58560686/using-tqdm-with-multiprocessing) [^1]

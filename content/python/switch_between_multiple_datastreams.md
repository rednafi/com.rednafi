---
title: Switching between multiple data streams in a single thread
date: 2023-02-19
tags:
    - Python
    - Database
    - TIL
---

I was working on a project where I needed to poll multiple data sources and consume the
incoming data points in a single thread. In this particular case, the two data streams were
coming from two different Redis lists. The correct way to consume them would be to write two
separate consumers and spin them up as different processes.

However, in this scenario, I needed a simple way to poll and consume data from one data
source, wait for a bit, then poll and consume from another data source, and keep doing this
indefinitely. That way I could get away with doing the whole workflow in a single thread
without the overhead of managing multiple processes.

Here's what I'm trying to do:

```python
# pseudocode.py

def stream_a():
    while True:
        # Poll the first redis list.

def stream_b():
    while True:
        # Poll the second redis list.

def consume():
    # How do I alternate between two infinite streams and consume them?
    while True:
        stream_a() # Somehow break out of the infinite while loop.
        stream_b() # Somehow run this infinite loop after one iteration of
                   # the first one.
```

One way is to poll the data sources in two generator functions and yield the result. Then in
the consumer, we'll have to alternate between the generators to fetch the next result like
this:

```python
# pseudocode.py
import redis


def stream_a():
    while True:
        # Fetch result from the first redis list.
        yield redis.rpop("stream_a")


def stream_b():
    while True:
        # Fetch result from the second redis list.
        yield redis.rpop("stream_b")


def consume():
    streams = (stream_a(), stream_b())

    while True:
        # Iterate through the stream generators.
        for stream in streams:
            # Wait for a second before polling each data source.
            time.sleep(1)

            # Get the result. If the result is None then go back to the
            # beginning of the loop
            if (result := next(stream, None)) is None:
                continue

            print(f"From {stream.__name__}:", result)
```

Let's make a concrete example out of the pesudocode:

```python
# src.py

from __future__ import annotations

import time
from itertools import count
from typing import Generator


def stream_even() -> Generator[int, None, None]:
    yield from count(start=0, step=2)


def stream_odd() -> Generator[int, None, None]:
    yield from count(start=1, step=2)


def consume() -> None:
    streams = (stream_even(), stream_odd())

    while True:
        for stream in streams:
            time.sleep(1)
            if (result := next(stream, None)) is None:
                continue
            print(f"From {stream.__name__}:", result)


if __name__ == "__main__":
    consume()
```

The code above defines two generator functions, `stream_even()` and `stream_odd()`, that use
the `count()` function from the `itertools` module to generate an infinite sequence of even
and odd integers respectively.

The `consume()` function creates a tuple containing the two generator objects, and enters an
infinite loop. On each iteration of the loop, it iterates over the tuple using a for loop;
effectively alternating between the two streams. In each iteration, it waits for 1 second
using the `time.sleep()` function and then uses the `next()` function to retrieve the next
item from the current stream. If the result is not `None`, it prints a message to the
console indicating which stream it came from and what the value was. Else, it loops back to
the beginning of the iteration.

Running the snippet will print the folling output to the console:

```txt
$ python src.py
From stream_even: 0
From stream_odd: 1
From stream_even: 2
From stream_odd: 3
From stream_even: 4
From stream_odd: 5
From stream_even: 6
From stream_odd: 7
From stream_even: 8
From stream_odd: 9
From stream_even: 10
^CTraceback (most recent call last):
  File "/Users/rednafi/Canvas/personal/reflections/src.py", line 29,
  in <module> consume()
  File "/Users/rednafi/Canvas/personal/reflections/src.py", line 22,
  in consume
  time.sleep(1)
KeyboardInterrupt
```

The consumer infinite loop can be written in a more concise manner with `itertools.cycle`.
Instead of using the `while` loop, we can use this function to indefinitely cycle between
the elements of an iterable.

```python
# src.py
...


from itertools import cycle


def consume() -> None:
    streams = (stream_even(), stream_odd())

    for stream in cycle(streams):  # Use itertools.cycle instead of while ...
        time.sleep(1)
        if (result := next(stream, None)) is None:
            break
        print(f"From {stream.__name__}:", result)


...
```

Here, the finalized executable script:

```python
# src.py

from __future__ import annotations

import time
from itertools import count, cycle
from typing import Generator


def stream_even() -> Generator[int, None, None]:
    yield from count(start=0, step=2)


def stream_odd() -> Generator[int, None, None]:
    yield from count(start=1, step=2)


def consume() -> None:
    streams = (stream_even(), stream_odd())

    for stream in cycle(streams):
        time.sleep(1)
        if (result := next(stream, None)) is None:
            continue
        print(f"From {stream.__name__}:", result)


if __name__ == "__main__":
    consume()
```

```txt
$ python src.py
From stream_even: 0
From stream_odd: 1
From stream_even: 2
From stream_odd: 3
From stream_even: 4
From stream_odd: 5
From stream_even: 6
^CTraceback (most recent call last):
  File "/Users/rednafi/Canvas/personal/reflections/src.py", line 28,
  in <module> consume()
  File "/Users/rednafi/Canvas/personal/reflections/src.py", line 21,
  in consume time.sleep(1)
KeyboardInterrupt
```

[^1]: [itertools-cycle](https://docs.python.org/3/library/itertools.html#itertools.cycle) [^1]

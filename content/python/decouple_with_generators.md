---
title: Decoupling producers and consumers of iterables with generators in Python
date: 2022-04-03
tags:
    - Python
---

> Find your redemption in lazy iterators.
>
> — Could be the untold 20th Zen of Python


Generators can help you decouple the production and consumption of iterables—making your
code more readable and maintainable. I learned this trick a few years back from David
Beazley's [slides](https://www.dabeaz.com/generators/Generators.pdf) on generators.
Consider this example:

```python
# src.py
from __future__ import annotations

import time
from typing import NoReturn


def infinite_counter(start: int, step: int) -> NoReturn:
    i = start
    while True:
        time.sleep(1)  # Not to flood stdout
        print(i)
        i += step


infinite_counter(1, 2)
# Prints
# 1
# 3
# 5
# ...
```

Now, how'd you decouple the print statement from the `infinite_counter`? Since the
function never returns, you can't collect the outputs in an iterable, return the
container, and print the elements of the iterable in another function. You might be
wondering why would you even need to do it. I can think of two reasons:

* The `infinite_counter` function is the producer of the numbers and the `print`
function is consuming them. These are two separate responsibilities tangled in the same
function which violates the
[Single Responsibility Principle (SRP)](https://en.wikipedia.org/wiki/Single-responsibility_principle).
* What'd you do if you needed a version of the infinite counter where the consumer had
different behavior?

One way the second point can be addressed is—by accepting the consumer function as a
parameter and applying that to the produced value.

```python
# src.py
from __future__ import annotations

import time

# In Python < 3.9, import this from the 'typing' module.
from collections.abc import Callable
from typing import NoReturn


def infinite_counter(
    start: int, step: int, consumer: Callable = print
) -> NoReturn:
    i = start
    while True:
        time.sleep(1)  # Not to flood stdout
        consumer(i)
        i += step


infinite_counter(1, 2)
# Prints
# 1
# 3
# 5
# ...
```
You can override the value of `consumer` with any callable and make the function more
flexible. However, applying multiple consumers will still be hairy. Doing this with
generators is cleaner. Here's how you'd transform the above script to take advantage of
generators:

```python
# src.py
from __future__ import annotations

import time

# In Python < 3.9, import this from the 'typing' module.
from collections.abc import Generator


# Producer.
def infinite_counter(start: int, step: int) -> Generator[int, None, None]:
    i = start
    while True:
        time.sleep(1)  # Not to flood stdout
        yield i
        i += step


# Consumer. This can be a callable doing anything.
def infinite_printer(gen: Generator[int, None, None]) -> None:
    for i in gen:
        print(i)


gen = infinite_counter(1, 2)
infinite_printer(gen)
# Prints
# 1
# 3
# 5
# ...
```

The `infinite_counter` returns a generator that can lazily be iterated to produce the
numbers and you can call any arbitrary consumer on the generated result without coupling
it with the producer.

## Writing a workflow that mimics 'tail -f'

In a UNIX system, you can call `tail -f <filename> | grep <pattern>` to print the lines
of a file in real-time where the lines match a specific pattern. Running the following
command on my terminal allows me to tail the `syslog` file and print out any line that
contains the word `xps`:

```sh
tail -f /var/logs/syslog | grep xps
```

```
Apr  3 04:42:21 xps slack.desktop[4613]: [04/03/22, 04:42:21:859]
...
```

If you look carefully, the above command has two parts. The `tail -f <filename>` returns
the new lines appended to the file and `grep <pattern>` consumes the new lines to look
for a particular pattern. This behavior can be mimicked via generators as follows:

```python
# src.py
from __future__ import annotations

import os
import time

# In Python < 3.9, import this from the 'typing' module.
from collections.abc import Generator


# Producer.
def tail_f(filepath: str) -> Generator[str, None, None]:
    file = open(filepath)
    file.seek(0, os.SEEK_END)  # End-of-file

    while True:
        line = file.readline()
        if not line:
            time.sleep(0.001)  # Sleep briefly
            continue

        yield line


# Consumer.
def grep(
    lines: Generator[str, None, None], pattern: str | None = None
) -> None:
    for line in gen:
        if not pattern:
            print(line)
        else:
            if not pattern in line:
                continue
            print(line, flush=True)


lines = tail_f(filepath="/var/log/syslog")
grep(lines, "xps")
# Prints
# Apr  3 04:42:21 xps slack.desktop[4613]: [04/03/22, 04:42:21:859]
# info: Store: SET_SYSTEM_IDLE idle
```

Here, the `tail_f` continuously yields the logs, and the `grep` function looks for the
pattern `xps` in the logs. Replacing `grep` with any other processing function is
trivial as long as it accepts a generator. The `tail_f` function doesn't know anything
about the existence of `grep` or any other consumer function.

## Continuously polling a database and consuming the results

This concept of polling a log file for new lines can be extended to databases and caches
as well. I was working on a microservice that polls a Redis queue at a steady interval
and processes the elements one by one. I took advantage of generators to decouple the
function that collects the data and the one that processes the data. Here's how it works:

```python
# src.py
from __future__ import annotations

# In Python < 3.9, import this from the 'typing' module.
from collections.abc import Generator

import redis  # Requires pip install

Data = Generator[tuple[bytes, bytes], None, None]


def collect(queue_name: str) -> Data:
    r = redis.Redis()
    while True:
        yield r.brpop(queue_name)


def process(data: Data) -> None:
    for datum in data:
        queue_name, content = datum[0].decode(), datum[1].decode()
        print(f"{queue_name=}, {content=}")


data = collect("default")
process(data)
```

You'll need to run an instance of [Redis](https://redis.io) server and
[Redis CLI](https://redis.io/docs/manual/cli/) to test this out. If you've got
[Docker](https://www.docker.com/) installed in your system, then you can run
`docker run -it redis` to quickly spin up a Redis instance. Afterward, run the above
script and start the CLI. Print the following command on the CLI prompt:

```sh
127.0.0.1:6379> lpush default hello world
```
The above script should print the following:

```
queue_name='default', content='hello'
queue_name='default', content='world'
```

This allows you to define multiple consumers and run them in separate threads/processes
without the producer ever knowing about their existence at all.

## References

* [Generator tricks for systems programmers — David Beazley](https://www.dabeaz.com/generators/Generators.pdf)

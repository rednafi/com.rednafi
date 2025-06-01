---
title: Effortless concurrency with Python's concurrent.futures
date: 2020-04-21
tags:
    - Python
---

Writing concurrent code in Python can be tricky. Before you even start, you have to worry
about all these icky stuff like whether the task at hand is I/O or CPU bound or whether
putting the extra effort to achieve concurrency is even going to give you the boost you
need. Also, the presence of Global Interpreter Lock, GIL[^1] foists further limitations on
writing truly concurrent code. But for the sake of sanity, you can oversimplify it like this
without being blatantly incorrect:

> In Python, if the task at hand is I/O bound, you can use use standard library's
> `threading` module or if the task is CPU bound then `multiprocessing` module can be your
> friend. These APIs give you a lot of control and flexibility but they come at the cost of
> having to write relatively low-level verbose code that adds extra layers of complexity on
> top of your core logic. Sometimes when the target task is complicated, it's often
> impossible to avoid complexity while adding concurrency. However, a lot of simpler tasks
> can be made concurrent without adding too much verbosity.

Python standard library also houses a module called the `concurrent.futures`. This module
was added in Python 3.2 for providing the developers a high-level interface to launch
asynchronous tasks. It's a generalized abstraction layer on top of `threading` and
`multiprocessing` modules for providing an interface to run tasks concurrently using pools
of threads or processes. It's the perfect tool when you just want to run a piece of eligible
code concurrently and don't need the added modularity that the `threading` and
`multiprocessing` APIs expose.

## Anatomy of concurrent.futures

From the official docs,

> The concurrent.futures module provides a high-level interface for asynchronously executing
> callables.

What it means is you can run your subroutines asynchronously using either threads or
processes through a common high-level interface. Basically, the module provides an abstract
class called `Executor`. You can't instantiate it directly, rather you need to use one of
two subclasses that it provides to run your tasks.

```txt
Executor (Abstract Base Class)
│
├── ThreadPoolExecutor
│
│   │A concrete subclass of the Executor class to
│   │manage I/O bound tasks with threading underneath
│
├── ProcessPoolExecutor
│
│   │A concrete subclass of the Executor class to
│   │manage CPU bound tasks with multiprocessing underneath
```

Internally, these two classes interact with the pools and manage the workers. `Future`s are
used for managing results computed by the workers. To use a pool of workers, an application
creates an instance of the appropriate executor class and then submits them for it to run.
When each task is started, a `Future` instance is returned. When the result of the task is
needed, an application can use the `Future` object to block until the result is available.
Various APIs are provided to make it convenient to wait for tasks to complete, so that the
`Future` objects do not need to be managed directly.

## Executor objects

Since both `ThreadPoolExecutor` and `ProcessPoolExecutor` have the same API interface, in
both cases I'll primarily talk about two methods that they provide. Their descriptions have
been collected from the official docs verbatim.

### submit(fn, \*args, \*\*kwargs)

Schedules the callable, `fn`, to be executed as `fn(*args **kwargs)` and returns a `Future`
object representing the execution of the callable.

```python
with ThreadPoolExecutor(max_workers=1) as executor:
    future = executor.submit(pow, 323, 1235)
    print(future.result())
```

### map(func, \*iterables, timeout=None, chunksize=1)

Similar to `map(func, *iterables)` except:

- the iterables are collected immediately rather than lazily;
- func is executed asynchronously and several calls to func may be made concurrently.

    The returned iterator raises a `concurrent.futures.TimeoutError` if `__next__()` is
    called and the result isn't available after timeout seconds from the original call to
    `Executor.map()`. Timeout can be an `int` or a `float`. If timeout is not specified or
    `None`, there is no limit to the wait time.

    If a func call raises an exception, then that exception will be raised when its value is
    retrieved from the iterator.

    When using `ProcessPoolExecutor`, this method chops iterables into a number of chunks
    which it submits to the pool as separate tasks. The (approximate) size of these chunks
    can be specified by setting `chunksize` to a positive integer. For very long iterables,
    using a large value for `chunksize` can significantly improve performance compared to
    the default size of 1. With ThreadPoolExecutor, `chunksize` has no effect.

## Generic workflows for running tasks concurrently

A lot of my scripts contains some variants of the following:

```python
for task in get_tasks():
    perform(task)
```

Here, `get_tasks` returns an iterable that contains the target tasks or arguments on which a
particular task function needs to applied. Tasks are usually blocking callables and they run
one after another, with only one task running at a time. The logic is simple to reason with
because of its sequential execution flow. This is fine when the number of tasks is small or
the execution time requirement and complexity of the individual tasks is low. However, this
can quickly get out of hands when the number of tasks is huge or the individual tasks are
time consuming.

A general rule of thumb is using `ThreadPoolExecutor` when the tasks are primarily I/O
bound, like—sending multiple http requests to many urls, saving a large number of files to
disk etc. `ProcessPoolExecutor` should be used in tasks that are primarily CPU bound,
like—running callables that are computation heavy, applying pre-process methods over a large
number of images, manipulating many text files at once etc.

### Running tasks with executor.submit

When you have a number of tasks, you can schedule them in one go and wait for them all to
complete and then you can collect the results.

```python
import concurrent.futures


with concurrent.futures.Executor() as executor:
    futures = {executor.submit(perform, task) for task in get_tasks()}

    for fut in concurrent.futures.as_completed(futures):
        print(f"The outcome is {fut.result()}")
```

Here you start by creating an Executor, which manages all the tasks that are running—either
in separate processes or threads. Using the with statement creates a context manager, which
ensures any stray threads or processes get cleaned up via calling the `executor.shutdown()`
method implicitly when you're done.

In real code, you'd would need to replace the `Executor` with `ThreadPoolExecutor` or a
`ProcessPoolExecutor` depending on the nature of the callables. Then a set comprehension has
been used here to start all the tasks. The `executor.submit()` method schedules each task.
This creates a Future object, which represents the task to be done. Once all the tasks have
been scheduled, the method `concurrent.futures_as_completed()` is called, which yields the
futures as they're done – that is, as each task completes. The `fut.result()` method gives
you the return value of `perform(task)`, or throws an exception in case of failure.

The `executor.submit()` method schedules the tasks asynchronously and doesn't hold any
contexts regarding the original tasks. So if you want to map the results with the original
tasks, you need to track those yourself.

```python
import concurrent.futures


with concurrent.futures.Executor() as executor:
    futures = {executor.submit(perform, task): task for task in get_tasks()}

    for fut in concurrent.futures.as_completed(futures):
        original_task = futures[fut]
        print(f"The result of {original_task} is {fut.result()}")
```

Notice the variable `futures` where the original tasks are mapped with their corresponding
futures using a dictionary.

### Running tasks with executor.map

Another way the results can be collected in the same order they're scheduled is via using
`executor.map()` method.

```python
import concurrent.futures


with concurrent.futures.Executor() as executor:
    for arg, res in zip(get_tasks(), executor.map(perform, get_tasks())):
        print(f"The result of {arg} is {res}")
```

Notice how the map function takes the entire iterable at once. It spits out the results
immediately rather than lazily and in the same order they're scheduled. If any unhandled
exception occurs during the operation, it'll also be raised immediately and the execution
won't go any further.

In Python 3.5+, `executor.map()` receives an optional argument: `chunksize`. While using
`ProcessPoolExecutor`, for very long iterables, using a large value for chunksize can
significantly improve performance compared to the default size of 1. With
`ThreadPoolExecutor`, chunksize has no effect.

## A few real world examples

Before proceeding with the examples, let's write a small decorator that'll be helpful to
measure and compare the execution time between concurrent and sequential code.

```python
import time
from functools import wraps


def timeit(method):
    @wraps(method)
    def wrapper(*args, **kwargs):
        start_time = time.time()
        result = method(*args, **kwargs)
        end_time = time.time()
        print(f"{method.__name__} => {(end_time-start_time)*1000} ms")

        return result

    return wrapper
```

The decorator can be used like this:

```python
@timeit
def func(n):
    return list(range(n))
```

This will print out the name of the method and how long it took to execute it.

### Download & save files from URLs with multi-threading

First, let's download some pdf files from a bunch of URLs and save them to the disk. This is
presumably an I/O bound task and we'll be using the `ThreadPoolExecutor` class to carry out
the operation. But before that, let's do this sequentially first.

```python
from pathlib import Path
import urllib.request


def download_one(url):
    """
    Downloads the specified URL and saves it to disk
    """

    req = urllib.request.urlopen(url)
    fullpath = Path(url)
    fname = fullpath.name
    ext = fullpath.suffix

    if not ext:
        raise RuntimeError("URL does not contain an extension")

    with open(fname, "wb") as handle:
        while True:
            chunk = req.read(1024)
            if not chunk:
                break
            handle.write(chunk)

    msg = f"Finished downloading {fname}"
    return msg


@timeit
def download_all(urls):
    return [download_one(url) for url in urls]


if __name__ == "__main__":
    urls = (
        "http://www.irs.gov/pub/irs-pdf/f1040.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040a.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040ez.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040es.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040sb.pdf",
    )

    results = download_all(urls)
    for result in results:
        print(result)
```

```txt
>>> download_all => 22850.6863117218 ms
... Finished downloading f1040.pdf
... Finished downloading f1040a.pdf
... Finished downloading f1040ez.pdf
... Finished downloading f1040es.pdf
... Finished downloading f1040sb.pdf
```

In the above code snippet, I have primary defined two functions. The `download_one` function
downloads a pdf file from a given URL and saves it to the disk. It checks whether the file
in URL has an extension and in the absence of an extension, it raises `RunTimeError`. If an
extension is found in the file name, it downloads the file chunk by chunk and saves to the
disk. The second function `download_all` just iterates through a sequence of URLs and
applies the `download_one` function on each of them. The sequential code takes about 22.8
seconds to run. Now let's see how our threaded version of the same code performs.

```python
from pathlib import Path
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed


def download_one(url):
    """
    Downloads the specified URL and saves it to disk
    """

    req = urllib.request.urlopen(url)
    fullpath = Path(url)
    fname = fullpath.name
    ext = fullpath.suffix

    if not ext:
        raise RuntimeError("URL does not contain an extension")

    with open(fname, "wb") as handle:
        while True:
            chunk = req.read(1024)
            if not chunk:
                break
            handle.write(chunk)

    msg = f"Finished downloading {fname}"
    return msg


@timeit
def download_all(urls):
    """
    Create a thread pool and download specified urls
    """

    with ThreadPoolExecutor(max_workers=13) as executor:
        return executor.map(download_one, urls, timeout=60)


if __name__ == "__main__":
    urls = (
        "http://www.irs.gov/pub/irs-pdf/f1040.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040a.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040ez.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040es.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040sb.pdf",
    )

    results = download_all(urls)
    for result in results:
        print(result)
```

```txt
>>> download_all => 5042.651653289795 ms
... Finished downloading f1040.pdf
... Finished downloading f1040a.pdf
... Finished downloading f1040ez.pdf
... Finished downloading f1040es.pdf
... Finished downloading f1040sb.pdf
```

The concurrent version of the code takes only about 1/4 th the time of it's sequential
counterpart. Notice in this concurrent version, the `download_one` function is the same as
before but in the `download_all` function, a `ThreadPoolExecutor` context manager wraps the
`executor.map()` method. The `download_one` function is passed into the `map` along with the
iterable containing the URLs. The `timeout` parameter determines how long a thread will
spend before giving up on a single task in the pipeline. The `max_workers` means how many
worker you want to deploy to spawn and manage the threads. A general rule of thumb is using
`2 * multiprocessing.cpu_count() + 1`. My machine has 6 physical cores with 12 threads. So
13 is the value I chose.

> Note: You can also try running the above functions with `ProcessPoolExecutor` via the same
> interface and notice that the threaded version performs slightly better than due to the
> nature of the task.

There is one small problem with the example above. The `executor.map()` method returns a
generator which allows to iterate through the results once ready. That means if any error
occurs inside `map`, it's not possible to handle that and resume the generator after the
exception occurs. From PEP-255[^2]:

> If an unhandled exception-- including, but not limited to, StopIteration --is raised by,
> or passes through, a generator function, then the exception is passed on to the caller in
> the usual way, and subsequent attempts to resume the generator function raise
> StopIteration. In other words, an unhandled exception terminates a generator's useful
> life.

To get around that, you can use the `executor.submit()` method to create futures,
accumulated the futures in a list, iterate through the futures and handle the exceptions
manually. See the following example:

```python
from pathlib import Path
import urllib.request
from concurrent.futures import ThreadPoolExecutor


def download_one(url):
    """
    Downloads the specified URL and saves it to disk
    """

    req = urllib.request.urlopen(url)
    fullpath = Path(url)
    fname = fullpath.name
    ext = fullpath.suffix

    if not ext:
        raise RuntimeError("URL does not contain an extension")

    with open(fname, "wb") as handle:
        while True:
            chunk = req.read(1024)
            if not chunk:
                break
            handle.write(chunk)

    msg = f"Finished downloading {fname}"
    return msg


@timeit
def download_all(urls):
    """
    Create a thread pool and download specified urls
    """

    futures_list = []
    results = []

    with ThreadPoolExecutor(max_workers=13) as executor:
        for url in urls:
            futures = executor.submit(download_one, url)
            futures_list.append(futures)

        for future in futures_list:
            try:
                result = future.result(timeout=60)
                results.append(result)
            except Exception:
                results.append(None)
    return results


if __name__ == "__main__":
    urls = (
        "http://www.irs.gov/pub/irs-pdf/f1040.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040a.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040ez.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040es.pdf",
        "http://www.irs.gov/pub/irs-pdf/f1040sb.pdf",
    )

    results = download_all(urls)
    for result in results:
        print(result)
```

The above snippet should print out similar messages as before.

### Running multiple CPU bound subroutines with multi-processing

The following example shows a CPU bound hashing function. The primary function will
sequentially run a compute intensive hash algorithm multiple times. Then another function
will again run the primary function multiple times. Let's run the function sequentially
first.

```python
import hashlib


def hash_one(n):
    """A somewhat CPU-intensive task."""

    for i in range(1, n):
        hashlib.pbkdf2_hmac("sha256", b"password", b"salt", i * 10000)

    return "done"


@timeit
def hash_all(n):
    """Function that does hashing in serial."""

    for i in range(n):
        hsh = hash_one(n)

    return "done"


if __name__ == "__main__":
    hash_all(20)
```

```txt
>>> hash_all => 18317.330598831177 ms
```

If you analyze the `hash_one` and `hash_all` functions, you can see that together, they are
actually running two compute intensive nested `for` loops. The above code takes roughly 18
seconds to run in sequential mode. Now let's run it parallelly using `ProcessPoolExecutor`.

```python
import hashlib
from concurrent.futures import ProcessPoolExecutor, ThreadPoolExecutor


def hash_one(n):
    """A somewhat CPU-intensive task."""

    for i in range(1, n):
        hashlib.pbkdf2_hmac("sha256", b"password", b"salt", i * 10000)

    return "done"


@timeit
def hash_all(n):
    """Function that does hashing in serial."""

    with ProcessPoolExecutor(max_workers=10) as executor:
        for arg, res in zip(
            range(n), executor.map(hash_one, range(n), chunksize=2)
        ):
            pass

    return "done"


if __name__ == "__main__":
    hash_all(20)
```

```txt
>>> hash_all => 1673.842430114746 ms
```

If you look closely, even in the concurrent version, the `for` loop in `hash_one` function
is running sequentially. However, the other `for` loop in the `hash_all` function is being
executed through multiple processes. Here, I have used 10 workers and a chunksize of 2. The
number of workers and chunksize were adjusted to achieve maximum performance. As you can see
the concurrent version of the above CPU intensive operation is about 11 times faster than
its sequential counterpart.

## Avoiding concurrency pitfalls

Since the `concurrent.futures` provides such a simple API, you might be tempted to apply
concurrency to every simple tasks at hand. However, that's not a good idea. First, the
simplicity has its fair share of constraints. In this way, you can apply concurrency only to
the simplest of the tasks, usually mapping a function to an iterable or running a few
subroutines simultaneously. If your task at hand requires queuing, spawning multiple threads
from multiple processes then you will still need to resort to the lower level `threading`
and `multiprocessing` modules.

Another pitfall of using concurrency is deadlock situations that might occur while using
`ThreadPoolExecutor`. When a callable associated with a `Future` waits on the results of
another `Future`, they might never release their control of the threads and cause deadlock.
Let's see a slightly modified example from the official docs.

```python
import time
from concurrent.futures import ThreadPoolExecutor


def wait_on_b():
    time.sleep(5)
    print(b.result())  # b will never complete because it is waiting on a.
    return 5


def wait_on_a():
    time.sleep(5)
    print(a.result())  # a will never complete because it is waiting on b.
    return 6


with ThreadPoolExecutor(max_workers=2) as executor:
    # here, the future from a depends on the future from b
    # and vice versa
    # so this is never going to be completed
    a = executor.submit(wait_on_b)
    b = executor.submit(wait_on_a)

    print("Result from wait_on_b", a.result())
    print("Result from wait_on_a", b.result())
```

In the above example, function `wait_on_b` depends on the result (result of the `Future`
object) of function `wait_on_a` and at the same time the later function's result depends on
that of the former function. So the code block in the context manager will never execute due
to having inter dependencies. This creates the deadlock situation. Let's explain another
deadlock situation from the official docs.

```python
from concurrent.futures import ThreadPoolExecutor


def wait_on_future():
    f = executor.submit(pow, 5, 2)
    # This will never complete because there is only one worker thread and
    # it is executing this function.
    print(f.result())


with ThreadPoolExecutor(max_workers=1) as executor:
    future = executor.submit(wait_on_future)
    print(future.result())
```

The above situation usually happens when a subroutine produces nested `Future` object and
runs on a single thread. In the function `wait_on_future`, the `executor.submit(pow, 5, 2)`
creates another `Future` object. Since I'm running the entire thing using a single thread,
the internal future object is blocking the thread and the external `executor.submit()`
method inside the context manager can not use any threads. This situation can be avoided
using multiple threads but in general, this is a bad design itself.

Then there're situations when you might be getting lower performance with concurrent code
than its sequential counterpart. This could happen for multiple reasons:

1. Threads were used to perform CPU bound tasks.
2. Multiprocessing were used to perform I/O bound tasks.
3. The tasks were too trivial to justify using either threads or multiple processes.

Spawning and squashing multiple threads or processes bring extra overheads. Usually threads
are much faster than processes to spawn and squash. However, using the wrong type of
concurrency can actually slow down your code rather than making it any performant. Below is
a trivial example where both `ThreadPoolExecutor` and `ProcessPoolExecutor` perform worse
than their sequential counterpart.

```python
import math

PRIMES = [num for num in range(19000, 20000)]


def is_prime(n):
    if n < 2:
        return False
    if n == 2:
        return True
    if n % 2 == 0:
        return False

    sqrt_n = int(math.floor(math.sqrt(n)))
    for i in range(3, sqrt_n + 1, 2):
        if n % i == 0:
            return False
    return True


@timeit
def main():
    for number in PRIMES:
        print(f"{number} is prime: {is_prime(number)}")


if __name__ == "__main__":
    main()
```

```txt
>>> 19088 is prime: False
... 19089 is prime: False
... 19090 is prime: False
... ...
... main => 67.65174865722656 ms
```

The above examples verifies whether a number in a list is prime or not. We ran the function
on 1000 numbers to determine if they're prime or not. The sequential version took roughly
67ms to do that. However, look below where the threaded version of the same code takes more
than double the time (140ms) to so the same task.

```python
from concurrent.futures import ProcessPoolExecutor, ThreadPoolExecutor
import math

num_list = [num for num in range(19000, 20000)]


def is_prime(n):
    if n < 2:
        return False
    if n == 2:
        return True
    if n % 2 == 0:
        return False

    sqrt_n = int(math.floor(math.sqrt(n)))
    for i in range(3, sqrt_n + 1, 2):
        if n % i == 0:
            return False
    return True


@timeit
def main():
    with ThreadPoolExecutor(max_workers=13) as executor:
        for number, prime in zip(PRIMES, executor.map(is_prime, num_list)):
            print(f"{number} is prime: {prime}")


if __name__ == "__main__":
    main()
```

```txt
>>> 19088 is prime: False
... 19089 is prime: False
... 19090 is prime: False
... ...
... main => 140.17250061035156 ms
```

The multiprocessing version of the same code is even slower. The tasks doesn't justify
opening so many processes.

```python
from concurrent.futures import ProcessPoolExecutor
import math

num_list = [num for num in range(19000, 20000)]


def is_prime(n):
    if n < 2:
        return False
    if n == 2:
        return True
    if n % 2 == 0:
        return False

    sqrt_n = int(math.floor(math.sqrt(n)))
    for i in range(3, sqrt_n + 1, 2):
        if n % i == 0:
            return False
    return True


@timeit
def main():
    with ProcessPoolExecutor(max_workers=13) as executor:
        for number, prime in zip(PRIMES, executor.map(is_prime, num_list)):
            print(f"{number} is prime: {prime}")


if __name__ == "__main__":
    main()
```

```txt
>>> 19088 is prime: False
... 19089 is prime: False
... 19090 is prime: False
... ...
... main => 311.3126754760742 ms
```

Although intuitively, it may seem like the task of checking prime numbers should be a CPU
bound operation, it's also important to determine if the task itself is computationally
heavy enough to justify spawning multiple threads or processes. Otherwise, you might end up
with complicated code that performs worse than the naive solution.

## References

- [concurrent.futures - the official documentation]
- [Easy concurrency in Python]
- [Adventures in Python with concurrent.futures]

[^1]: [GIL](https://wiki.python.org/moin/GlobalInterpreterLock)

[^2]:
    [PEP-255](https://www.python.org/dev/peps/pep-0255/#specification-generators-and-exception-propagation)

[^3]:
    [concurrent.futures - the official documentation](https://docs.python.org/3/library/concurrent.futures.html)
    [^3]

[^4]: [easy concurrency in python](http://pljung.de/posts/easy-concurrency-in-python/) [^4]

[^5]:
    [Adventures in python with concurrent.futures](https://alexwlchan.net/2019/10/adventures-with-concurrent-futures/)
    [^5]

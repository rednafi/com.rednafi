---
title: Don't wrap instance methods with 'functools.lru_cache' decorator in Python
date: 2022-01-15
tags:
    - Python
    - TIL
---

Recently, fell into this trap as I wanted to speed up a slow instance method by caching it.

> When you decorate an instance method with `functools.lru_cache` decorator, the instances
> of the class encapsulating that method never get garbage collected within the lifetime of
> the process holding them.

Let's consider this example:

```py
# src.py
import functools
import time
from typing import TypeVar

Number = TypeVar("Number", int, float, complex)


class SlowAdder:
    def __init__(self, delay: int = 1) -> None:
        self.delay = delay

    @functools.lru_cache
    def calculate(self, *args: Number) -> Number:
        time.sleep(self.delay)
        return sum(args)

    def __del__(self) -> None:
        print("Deleting instance ...")


# Create a SlowAdder instance.
slow_adder = SlowAdder(2)

# Measure performance.
start_time = time.perf_counter()
# ----------------------------------------------
result = slow_adder.calculate(1, 2)
# ----------------------------------------------
end_time = time.perf_counter()
print(f"Calculation took {end_time-start_time} seconds, result: {result}.")


start_time = time.perf_counter()
# ----------------------------------------------
result = slow_adder.calculate(1, 2)
# ----------------------------------------------
end_time = time.perf_counter()
print(f"Calculation took {end_time-start_time} seconds, result: {result}.")
```

Here, I've created a simple `SlowAdder` class that accepts a `delay` value; then it sleeps
for `delay` seconds and calculates the sum of the inputs in the `calculate` method. To avoid
this slow recalculation for the same arguments, the `calculate` method was wrapped in the
`lru_cache` decorator. The `__del__` method notifies us when the garbage collection has
successfully cleaned up instances of the class.

If you run this program, it'll print this:

```txt
Calculation took 2.0021052900010545 seconds, result: 3.
Calculation took 5.632002284983173e-06 seconds, result: 3.
Deleting instance ...
```

You can see that the `lru_cache` decorator is doing its job. The second call to the
`calculate` method with the same argument took noticeably less time compared to the first
one. In the second case, the `lru_cache` decorator is just doing a simple dictionary lookup.
This is all good but the instances of the `ShowAdder` class never get garbage collected in
the lifetime of the program. Let's prove that in the next section.

## Garbage collector can't clear up the affected instances

If you execute the above snippet with an `-i` flag, we can interactively prove that no
garbage collection takes place. Let's do it:

```txt
$ python -i src.py
Calculation took 2.002104839997628 seconds, result: 3.
Calculation took 5.566998879658058e-06 seconds, result: 3.
>>> import gc
>>>
>>> slow_adder.calculate(1,2)
3
>>> slow_adder = None
>>>
>>> gc.collect()
0
>>>
```

Here on the REPL, you can see that I've reassigned `slow_adder` to None and then explicitly
triggered the garbage collector. However, we don't see the message in the `__del__` method
printed here and the output of `gc.collect()` is 0. This implies that something is holding a
reference to the `slow_adder` instance and the garbage collector can't clear up the object.
Let's inspect who has that reference:

```txt
$ python -i src.py
Calculation took 2.00233274600032 seconds, result: 3.
Calculation took 5.453999619930983e-06 seconds, result: 3.
>>> slow_adder.calculate.cache_info()
CacheInfo(hits=1, misses=1, maxsize=128, currsize=1)
>>> slow_adder.calculate(1,2)
3
>>> slow_adder.calculate.cache_info()
CacheInfo(hits=2, misses=1, maxsize=128, currsize=1)
>>> slow_adder.calculate.cache_clear()
>>> slow_adder = None
Deleting instance ...
>>>
```

The `cache_info()` is showing that the cache container keeps a reference to the instance
until it gets cleared. When I manually cleared the cache and reassigned the variable
`slow_adder` to `None`, only then did the garbage collector remove the instance. By default,
the size of the `lru_cache` is 128 but if I had applied `lru_cache(maxsize=None)`, that
would've kept the cache forever and the garbage collector would wait for the reference count
to drop to zero but that'd never happen within the lifetime of the process.

This can be dangerous if you create millions of instances and they don't get garbage
collected naturally. It can overflow your working memory and cause the process to crash! I
accidentally did it where the infected class was being instantiated millions of times via
HTTP API requests.

## The solution

To solve this, we'll have to make the cache containers local to the instances so that the
reference from cache to the instance gets scraped off with the instance. Here's how you can
do that:

```py
# src_2.py
import functools
import time
from typing import TypeVar

Number = TypeVar("Number", int, float, complex)


class SlowAdder:
    def __init__(self, delay: int = 1) -> None:
        self.delay = delay
        self.calculate = functools.lru_cache()(self._calculate)

    def _calculate(self, *args: Number) -> Number:
        time.sleep(self.delay)
        return sum(args)

    def __del__(self) -> None:
        print("Deleting instance ...")
```

The only difference here is—instead of decorating the method directly, I called the
decorator function on the `_calculate` method just as a regular function and saved the
result as an instance variable named `calculate`. The instances of this class get garbage
collected as usual.

```txt
$ python -i src.py
>>> slow_adder = SlowAdder(2)
>>> slow_adder.calculate(1,2)
3
>>> slow_adder.calculate.cache_info()
CacheInfo(hits=0, misses=1, maxsize=128, currsize=1)
>>> import gc
>>> slow_adder = None
>>> gc.collect()
Deleting instance ...
11
```

Notice that this time, clearing out the cache wasn't necessary. I had to call `gc.collect()`
to invoke explicit garbage collection. That's because this shenanigan creates cyclical
references and the GC needs to do some special magic to clear the memory. In real code,
Python interpreter will clean this up for you in the background without you having to call
the GC.

## The self dilemma

Even after applying the solution above, a weird thing happens in the case of instance
methods. Let's run the `src_2.py` script interactively to demonstrate that:

```txt
$ python -i src_2.py
>>> slow_adder = SlowAdder(2)
>>> slow_adder.calculate(1,2)
>>> slow_adder
<__main__.SlowAdder object at 0x7f92595f9b40>
>>> slow_adder_2 = SlowAdder(2)
>>> slow_adder_2.calculate(1,2)
3
>>> slow_adder.calculate.cache_info()
CacheInfo(hits=1, misses=2, maxsize=128, currsize=2)
>>> slow_adder_2.calculate.cache_info()
CacheInfo(hits=1, misses=2, maxsize=128, currsize=2)
>>>
```

Here, I've created another instance of the `SlowAdder` class and called `calculate` with the
same arguments. But whenever I called the `calculate` method on the `slow_adder_2` instance
with the same parameters as before, the first time, it recalculated it instead of returning
the result from the cache. How come!

Underneath, the `lru_cache` decorator uses a dictionary to cache the calculated values. A
hash function is applied[^1] to all the parameters of the target function to build the key
of the dictionary, and the value is the return value of the function when those parameters
are the inputs. This means, the first argument `self` also gets included while building the
cache key. However, for different instances, this `self` object is going to be different and
that makes the hashed key of the cache different for every instance even if the other
parameters are the same.

## But what about class methods & static methods

Class methods and static methods don't suffer from the above issues as they don't have any
ties to their respective instances. In their case, the cache container is local to the
class, not the instances. Here, you can stack the `lru_cache` decorator as usual. Let's
demonstrate that for `classmethod` first:

```py
# src_3.py
import functools
import time


class Foo:
    @classmethod
    @functools.lru_cache
    def bar(cls, delay: int) -> int:
        # Do something with the cls.
        cls.delay = delay
        time.sleep(delay)
        return 42

    def __del__(self) -> None:
        print("Deleting instance ...")


foo_1 = Foo()
foo_2 = Foo()


start_time = time.perf_counter()
# ----------------------------------------------
result = foo_1.bar(2)
# ----------------------------------------------
end_time = time.perf_counter()
print(f"Calculation took {end_time - start_time} seconds, result: {result}.")


start_time = time.perf_counter()
# ----------------------------------------------
result = foo_2.bar(2)
# ----------------------------------------------
end_time = time.perf_counter()
print(f"Calculation took {end_time - start_time} seconds, result: {result}.")
```

You can inspect the garbage collection behavior here:

```txt
$ python src_3.py
Calculation took 2.0022965140015003 seconds, result: 42.
Calculation took 4.4819971662946045e-06 seconds, result: 42.
>>> foo_1 = None
Deleting instance ...
>>>
```

Static methods behave exactly the same. You can use the `lru_cache` decorator in similar
fashion as below:

```py
import functools
import time


class Foo:
    @staticmethod
    @functools.lru_cache
    def bar(delay: int) -> int:
        return 42

    def __del__(self) -> None:
        print("Deleting instance ...")
```

[^1]:
    [LRU cache key](https://github.com/python/cpython/blob/8882b30dab237c8b460cb8d18cecc8b8d031da25/Lib/functools.py#L448)

[^2]:
    [functools.lru_cache - Python Docs](https://docs.python.org/3/library/functools.html#functools.lru_cache)
    [^2]

[^3]:
    [Don't lru_cache methods! (intermediate) anthony explains #382](https://www.youtube.com/watch?v=sVjtp6tGo0g)
    [^3]

[^4]:
    [Python LRU cache in a class disregards maxsize limit when decorated with a staticmethod or classmethod decorator](https://stackoverflow.com/questions/70409673/python-lru-cache-in-a-class-disregards-maxsize-limit-when-decorated-with-a-stati)
    [^4]

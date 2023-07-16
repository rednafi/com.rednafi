---
title: Inspect docstrings with Pydoc
date: 2022-01-22
tags:
    - Python
---

How come I didn't know about the `python -m pydoc` command before today!

> *It lets you inspect the docstrings of any modules, classes, functions, or methods in
> Python.*

I'm running the commands from a Python 3.10 virtual environment but it'll work on any
Python version. Let's print out the docstrings of the `functools.lru_cache` function.
Run:

```sh
python -m pydoc functools.lru_cache
```

This will print the following on the console:

```txt
Help on function lru_cache in functools:

functools.lru_cache = lru_cache(maxsize=128, typed=False)
    Least-recently-used cache decorator.

    If *maxsize* is set to None, the LRU features are disabled and
    the cache can grow without bound.

    If *typed* is True, arguments of different types will be cached
    separately. For example, f(3.0) and f(3) will be treated as
    distinct calls with distinct results.

    Arguments to the cached function must be hashable.

    View the cache statistics named tuple (hits, misses, maxsize,
    currsize) with f.cache_info().  Clear the cache and statistics
    with f.cache_clear(). Access the underlying function with
    f.__wrapped__.

    See:
    https://en.wikipedia.org/wiki/Cache_replacement_policies#Least_recently_used_(LRU)
```

Works for third party tools as well:

```sh
python -m pydoc typing_extensions.ParamSpec
```

Also, works for any custom Python structure that is accessible from the current Python
path. Let's define a function with docstrings and put that in a module called `src.py`:

```python
# src.py
def greetings(name: str) -> None:
    """Prints Hello <name>! on the console.

    Parameters
    ----------
    name : str
        Name of the person you want to greet
    """

    print("Hello {name}!")
```

You can inspect the entire `src.py` module or the `greetings` function specifically as
follows:

To inspect the module, run:

```sh
python -m pydoc src
```

To inspect the `greetings` function only, run:

```sh
python -m pydoc src.greetings
```

It'll return:

```txt
Help on function greetings in src:

src.greetings = greetings(name: str) -> None
    Prints Hello <name>! on the console.

    Parameters
    ----------
    name : str
        Name of the person you want to greet
```

Instead of inspecting the docstrings one by one, you can also pull up all of them in the
current Python path and serve them as HTML pages. To do so, run:

```sh
python -m pydoc -b
```

This will render the docstrings as HTML web pages and automatically open the index page
with your default browser. From there you can use the built-in search to find and read
your ones you need.

## References

* [Tweet by Brandon Rhodes](https://twitter.com/brandon_rhodes/status/1354416534098214914)

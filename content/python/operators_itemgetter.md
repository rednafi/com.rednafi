---
title: Safer 'operator.itemgetter' in Python
date: 2022-06-16
slug: operators-itemgetter
aliases:
    - /python/operators_itemgetter/
tags:
    - Python
---

Python's `operator.itemgetter` is quite versatile. It works on pretty much any iterables and
map-like objects and allows you to fetch elements from them. The following snippet shows how
you can use it to sort a list of tuples by the first element of the tuple:

```py
In [2]: from operator import itemgetter
   ...:
   ...: l = [(10, 9), (1, 3), (4, 8), (0, 55), (6, 7)]
   ...: l_sorted = sorted(l, key=itemgetter(0))

In [3]: l_sorted
Out[3]: [(0, 55), (1, 3), (4, 8), (6, 7), (10, 9)]
```

Here, the `itemgetter` callable is doing the work of selecting the first element of every
tuple inside the list and then the `sorted` function is using those values to sort the
elements. Also, this is faster than using a lambda function and passing that to the `key`
parameter to do the sorting:

```py
In [6]: from operator import itemgetter

In [7]: l = [(10, 9), (1, 3), (4, 8), (0, 55), (6, 7)]

In [8]: %timeit sorted(l, key=itemgetter(0))
386 ns ± 4.2 ns per loop (mean ± std. dev. of 7 runs, 1,000,000 loops each)

In [9]: %timeit sorted(l, key=lambda x: x[0])
498 ns ± 0.444 ns per loop (mean ± std. dev. of 7 runs, 1,000,000 loops each)
```

You can also use `itemgetter` to extract multiple values from a dictionary in a single pass.
Consider this example:

```py
In [13]: from operator import itemgetter

In [14]: d = {'foo': 31, 'bar': 12, 'baz': 42, 'chez': 83, 'moi': 24}

In [15]: foo, bar, bazz = itemgetter('foo', 'bar', 'baz')(d)

In [16]: foo, bar, bazz
Out[16]: (31, 12, 42)

```

So, instead of extracting the key-value pairs with `d['foo'], d['bar'], ...`, `itemgetter`
allows us to make it DRY. The source code of the callable is freakishly simple. Here's the
entire thing:

```py
# operator.py


class itemgetter:
    """
    Return a callable object that fetches the given item(s) from
    its operand.

    After f = itemgetter(2), the call f(r) returns r[2].
    After g = itemgetter(2, 5, 3), the call g(r)
        returns (r[2], r[5], r[3])
    """

    __slots__ = ("_items", "_call")

    def __init__(self, item, *items):
        if not items:
            self._items = (item,)

            def func(obj):
                return obj[item]

            self._call = func
        else:
            self._items = items = (item,) + items

            def func(obj):
                return tuple(obj[i] for i in items)

            self._call = func

    def __call__(self, obj):
        return self._call(obj)

    def __repr__(self):
        return "%s.%s(%s)" % (
            self.__class__.__module__,
            self.__class__.__name__,
            ", ".join(map(repr, self._items)),
        )

    def __reduce__(self):
        return self.__class__, self._items
```

While this is all good and dandy, `itemgetter` will raise a `KeyError` if it can't find the
corresponding value against a key in a map or an `IndexError` if the provided index is
outside of the range of the sequence. This is how it looks in a dict:

```py

In [1]: from operator import itemgetter

In [2]: d = {'foo': 31, 'bar': 12, 'baz': 42, 'chez': 83, 'moi': 24}

In [3]: # These keys don't exist.

In [4]: fizz, up = itemgetter('fiz', 'up')(d)
---------------------------------------------------------------------------
KeyError                                  Traceback (most recent call last)
Input In [4], in <cell line: 1>()
----> 1 fizz, up = itemgetter('fiz', 'up')(d)

KeyError: 'fiz'
```

In the above snippet, `itemgetter` can't find the key `fiz` in the dict `d` and it complains
when we try to fetch the value against it. In a sequence, the error looks like this:

```py

In [5]: from operator import itemgetter

In [6]: l = [(10, 9), (1, 3), (4, 8), (0, 55), (6, 7)]

In [7]: # These indices don't exist.

In [8]: item_42, item_50 = itemgetter(42, 50)(l)
---------------------------------------------------------------------------
IndexError                                Traceback (most recent call last)
Input In [8], in <cell line: 1>()
----> 1 item_42, item_50 = itemgetter(42, 50)(l)

IndexError: list index out of range
```

## A more tolerant version of 'operator.itemgetter'

I wanted something that works similar to `itemgetter` but doesn't raise these exceptions
when it can't find the key in a dict or the index of a sequence is out of range. Instead,
it'd return a default value when these exceptions occur. So, to avoid `KeyError` in a map,
it'd use `d.get(key, default)` instead of `d[key]` to fetch the value. Similarly, in a
sequence, it'd first compare the length of the sequence with the index and return a default
value if the index is out of range.

Since `operator.itemgetter` is a class, we could inherit it and overwrite the `__init__`
method. However, your type-checker will complain if you do so. That's because, in the stub
file, the `itemgetter` class is decorated with the `typing.final` decorator and isn't meant
to be subclassed. So, our only option is to rewrite it. The good news is that this
implementation is quite terse just like the original. Here it goes:

```py
# src.py
from collections.abc import Mapping


class _Nothing:
    """Works as a sentinel value."""

    def __repr__(self):
        return "<NOTHING>"


_NOTHING = _Nothing()


class safe_itemgetter:
    """
    Return a callable object that fetches the given item(s)
    from its operand.
    """

    __slots__ = ("_items", "_call")

    def __init__(self, item, *items, default=_NOTHING):
        if not items:
            self._items = (item,)

            def func(obj):
                if isinstance(obj, Mapping):
                    return obj.get(item, default)
                if (item > 0 and len(obj) <= item) or (
                    item < 0 and len(obj) < abs(item)
                ):
                    return default
                return obj[item]

            self._call = func
        else:
            self._items = items = (item,) + items

            def func(obj):
                if isinstance(obj, Mapping):
                    get = obj.get  # Reduce attibute search call.
                    return tuple(get(i, default) for i in items)

                return tuple(
                    default
                    if (i > 0 and len(obj) <= i)
                    or (i < 0 and len(obj) < abs(i))
                    else obj[i]
                    for i in items
                )

            self._call = func

    # ----------------- same as operator.itemgetter --------------#

    def __call__(self, obj):
        return self._call(obj)

    def __repr__(self):
        return "%s.%s(%s)" % (
            self.__class__.__module__,
            self.__class__.__name__,
            ", ".join(map(repr, self._items)),
        )

    def __reduce__(self):
        return self.__class__, self._items
```

This class behaves almost the same way as the original `itemgetter` function. The only
difference is that you can pass a `default` value to return instead of raising
`KeyError/IndexError` depending on the type of the container. Let's try it out with a dict:

```py
In [12]: d = {'foo': 31, 'bar': 12, 'baz': 42, 'chez': 83, 'moi': 24}

In [13]: safe_itemgetter(-5, -3, -33, 'baz', 1)(d)
Out[13]: (<NOTHING>, <NOTHING>, <NOTHING>, 42, <NOTHING>)
```

Here, we're trying to access a bunch of keys that don't exist in the dict `d` and we want to
do this without raising any exceptions. You can see that instead of raising an exception,
`safe_itemgetter` returns a tuple containing the value(s) that it can find and the rest of
the positions are filled with the `default` value; in this case, the `<NOTHING>` sentinel.
We can pass any default value there:

```py
In[14]: safe_itemgetter(-5, -3, -33, "baz", 1, default="default")(d)
Out[14]: ("default", "default", "default", 42, "default")
```

This works similarly when a sequence is passed:

```py
In[18]: l = [(10, 9), (1, 3), (4, 8), (0, 55), (6, 7)]

In[19]: safe_itemgetter(-11, default=())(l)
Out[19]: ()
```

This returns an empty tuple when the sequence index is out of range. It works with multiple
indices as well:

```py
In [28]: l = [(10, 9), (1, 3), (4, 8), (0, 55), (6, 7)]

In [29]: safe_itemgetter(-1, -3, -7, 1)(l)
Out[29]: ((6, 7), (4, 8), <NOTHING>, (1, 3))
```

[^1]:
    [operator.itemgetter - Python docs](https://docs.python.org/3/library/operator.html#operator.itemgetter)
    [^1]

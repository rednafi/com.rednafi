---
title: String interning in Python
date: 2022-01-05
tags:
    - Python
---

I was reading the source code[^1] of the reference implementation of "PEP-661: Sentinel
Values"[^2] and discovered an optimization technique known as **String interning**. Modern
programming languages like Java, Python, PHP, Ruby, Julia, etc, performs _string interning_
to make their string operations more performant.

## String interning

> **String interning** makes common string processing operations time and space-efficient by
> caching them. Instead of creating a new copy of string every time, this optimization
> method dictates to keep just one copy of string for every appropriate immutable distinct
> value and use the pointer reference wherever referred.

Consider this example:

```py
# src.py

x = "This is a string"
y = "This is a string"

print(x is y)  # prints True
```

Running this will print `True` on the console. The `is` operator in Python is used to check
whether two objects refer to the same memory location or not. If it returns `True`, it
means, the two objects surrounding the operator are actually the same object.

This is kind of neat if you think about it. In the above snippet, instead of creating a new
copy when `y` is assigned to a string that has the same value as `x`, internally, Python
points to the same string that is assigned to `x`. This is only true for smaller strings;
larger strings will create individual copies as usual. The exact length that determines
whether a string will be interned or not depends on the implementation and you shouldn't
rely on this implicit behavior if your code needs interning. See this example:

```py
# src.py

x = "This is a string" * 300
y = "This is a string" * 300

print(x is y)  # prints False
```

This will print `False` on the console and the strings are not interned.

## Explicit string interning

Python's `sys` module in the standard library has a routine called `intern` that you can use
to intern even large strings. For example:

```py
# src.py

import sys

x = sys.intern("This is a string" * 300)
y = sys.intern("This is a string" * 300)

print(x is y)  # prints True
```

Here, the strings are interned and running the snippet will print `True` on the console.

## What strings are interned?

CPython performs string interning on constants such as Function Names, Variable Names,
String Literals, etc. This snippet[^3] from the CPython codebase suggests that when a new
Python object is created, the interpreter is interning all the compile-time constants,
names, and literals. Also, Dictionary Keys and Object Attributes are interned. Notice this:

```py
# src.py

# Dict key interning.
d = {"hello": "world"}
print(d.popitem()[0] is "hello")  # prints True


# Object attribute interning.
class Foo:
    def __init__(self, bar, baz):
        self.bar = bar
        self.baz = baz


foo = Foo("hello", "hello")
print(foo.bar is foo.baz)  # prints True
```

In both of these above cases, the print statement will print `True` on the
console—confirming the fact that dictionary keys and object attributes are interned. Having
interned attributes and keys means that the access operation is faster since the string
comparison operation is now just doing a pointer comparison.

## When explicit string interning might come in handy?

One use case that I've found is—interning large dictionary keys. Dictionary keys are in
general, interned automatically. However, if the key is large—something like a 4097 bytes
hash value—Python can choose not to perform interning. Here's an example:

```py
# src.py

# No dict key interning as the key is quite large.
d = {}
k = "#" * 4097
d["#" * 4097] = 1

print(d.popitem()[0] is k)  # prints False
```

This will print `False` as the key in this case, will not be interned. Dictionary value
access is slower if the key isn't interned. Let's test that out:

```py
# src.py
import time

# Interned.
t0 = time.perf_counter()

for _ in range(10000):
    d = {"#" * 4096: "Interned"}
    d["#" * 4096]

t1 = time.perf_counter()


# Non-interned.
t2 = time.perf_counter()

for _ in range(10000):
    d = {"#" * 4097: "Non-interned"}
    d["#" * 4097]

t3 = time.perf_counter()


print(f"Interned dict creation & access: {t1-t0} seconds")
print(f"Non-interned dict creation & access: {t3-t2} seconds")
print(f"Non-interned creation & access is {(t3-t2)/(t1-t0)} times slower")
```

This prints:

```txt
Interned dict creation & access: 0.0014631289996032137 seconds
Non-interned dict creation & access: 0.048660025000572205 seconds
Non-interned creation & access is 33.25750840409036 times slower
```

The above script creates and accesses a dictionary with interned and non-interned keys 10000
times. The time difference is quite huge. Non-interned dict creation and accession are in
fact, 33 times slower than its interned counterpart.

We can circumnavigate this limitation by using explicit string interning via the `sys`
module as follows:

```py
# src.py
import sys
import time

# Implicitly interned.
t0 = time.perf_counter()

for _ in range(10000):
    d = {"#" * 4096: "Implicitly-interned"}
    d["#" * 4096]

t1 = time.perf_counter()


# Explicitly interned.
t2 = time.perf_counter()

k1 = sys.intern("#" * 4097)
k2 = sys.intern("#" * 4097)
for _ in range(10000):
    d = {k1: "Explicitly-interned"}
    d[k2]

t3 = time.perf_counter()


print(t1 - t0)
print((t3 - t2) / (t1 - t0))
print(f"Implicitly interned dict creation & access: {t1-t0} seconds")
print(f"Explicitly interned dict creation & access: {t3-t2} seconds")
print(
    f"Explicitly interned creation & access is {(t3-t2)/(t1-t0)} times slower"
)
```

This prints:

```txt
Implicitly interned dict creation & access: 0.002887188999011414 seconds
Explicitly interned dict creation & access: 0.002545474999351427 seconds
Explicitly interned creation & access is 1.1264793204711423 times slower
```

Here, implicitly and explicitly interned dict creation and key access are almost equally
fast.

[^1]:
    [String interning in PEP-661's implementation](https://github.com/taleinat/python-stdlib-sentinels/blob/main/sentinels/sentinels.py)

[^2]: [PEP-661 – Sentinel Values](https://hugovk-peps.readthedocs.io/en/latest/pep-0661/#)

[^3]:
    [PyObject](https://github.com/python/cpython/blob/7d7817cf0f826e566d8370a0e974bbfed6611d91/Objects/codeobject.c#L537)

[^4]:
    [String interning in Python](https://arpitbhayani.me/blogs/string-interning-python/)
    [^4]

[^5]:
    [Python docs: `sys.intern`](https://docs.python.org/3/library/sys.html#sys.intern) [^5]

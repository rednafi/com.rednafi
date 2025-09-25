---
title: Pre-allocated lists in Python
date: 2022-03-27
slug: preallocated-list
aliases:
    - /python/preallocated_list/
tags:
    - Python
---

In CPython, elements of a list are stored as pointers to the elements rather than the values
of the elements themselves. This is evident from the struct[^1] that represents a list in C:

```c
// Fetched from CPython main branch. Removed comments for brevity.
typedef struct {

    PyObject_VAR_HEAD
    PyObject **ob_item; /* Pointer reference to the element. */
    Py_ssize_t allocated;

}PyListObject;
```

An empty list builds a `PyObject` and occupies some memory:

```py
from sys import getsizeof

l = []

print(getsizeof(l))
```

This returns:

```txt
56
```

The exact size of an empty list can vary across different Python versions and
implementations.

> A single pointer to an element requires 8 bytes of space in a list. Whenever additional
> elements are added to the list, Python dynamically allocates extra memory to accommodate
> future elements without resizing the container. This implies, adding a single element to
> an empty list will incite Python to allocate more memory than 8 bytes.

Let's put this to test and append some elements to the list:

```py
# src.py
from sys import getsizeof

l = []
l.append(0)

print(getsizeof(l))
```

This returns:

```txt
88
```

Wait, the size of `l` should have been 64 bytes (56+8) but instead, it increased to 88
bytes. This happens because in this case, Python over-allocated 32 extra bytes to
accommodate future incoming elements. Now, if you append 3 more elements to the list, you'll
see that it doesn't increase the size because no re-allocation is happening here:

```py
# src.py
from sys import getsizeof

l = []
l.append(0)
l.append(1)
l.append(2)
l.append(3)

print(getsizeof(l))
```

This prints:

```txt
88
```

Adding a fifth element to the above list will increase the size of the list by 32 bytes (can
be different in other implementations) again:

```py
# src.py
from sys import getsizeof

l = []
for i in range(6):
    l.append(l)

print(getsizeof(l))
```

```txt
120
```

This dynamic memory allocation makes lists so flexible, and since a list only holds
references to the elements, it can house heterogenous objects without any issue. But this
flexibility of being able to append any number of elements—without ever caring about memory
allocation—comes at the cost of slower execution time.

Although usually, you don't need to think about optimizing this at all, there's a way that
allows you to perform static pre-allocation of memory in a list instead of letting Python
perform dynamic allocation for you. This way, you can make sure that Python won't have to
perform dynamic memory allocation multiple times as your list grows.

Static pre-allocation will make your code go slightly faster. I had to do this once in a
tightly nested loop and the 10% performance improvement was significant for the service that
I was working on.

## Pre-allocating memory in a list

Let's measure the performance of appending elements to an empty list. I'm using IPython's
built-in `%%timeit` command to do it:

```py
In [1]: %%timeit
    ...:
    ...: l=[]
    ...: for i in range(10_000):
    ...:     l.append(i)
    ...:
499 µs ± 1.23 µs per loop (mean ± std. dev. of 7 runs, 1,000 loops each)
```

Now, if you know the final size of the list beforehand, then you don't need to create an
empty list and append elements to it via a loop. You can initialize the list with `None` and
then fill in the elements like this:

```py
# src.py
size = 10_000
l = [None] * size

for i in range(size):
    l[i] = i
```

This is quite a bit faster than the previous snippet:

```py
In [2]: %%timeit
    ...:
    ...: l=[None]*10_000
    ...: for i in range(10_000):
    ...:     l[i] = i
    ...:
321 µs ± 71.1 ns per loop (mean ± std. dev. of 7 runs, 1,000 loops each)
```

## Breadcrumbs

For simple cases demonstrated above, list comprehension is going to be quite a bit quicker
than the static pre-allocation technique. See for yourself:

```py
In [3]: %%timeit
    ...:
    ...: [i for i in range(10_000)]
225 µs ± 711 ns per loop (mean ± std. dev. of 7 runs, 1,000 loops each)
```

So, I don't recommend performing micro-optimization without instrumenting your code first.
However, list pre-allocation can still come in handy in more complex cases where you already
know the size of the final list, and shaving off a few micro-seconds makes a considerable
difference.

## References

[^1]:
    [List struct in CPython](https://github.com/python/cpython/blob/c19c3a09618ac400538ee412f84be4c1196c7bab/Include/cpython/listobject.h#L5)

[^2]:
    [Create a list with initial capacity in Python](https://stackoverflow.com/questions/311775/create-a-list-with-initial-capacity-in-python)
    [^2]

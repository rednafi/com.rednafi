---
title: Skipping the first part of an iterable in Python
date: 2023-02-12
tags:
    - Python
    - TIL
---

Consider this iterable:

```python
it = (1, 2, 3, 0, 4, 5, 6, 7)
```

Let's say you want to build another iterable that includes only the numbers that appear
starting from the element `0`. Usually, I'd do this:

```python
# This returns (0, 4, 5, 6, 7).
from_zero = tuple(
    elem for idx, elem in enumerate(it) if idx >= it.index(0)
)
```

While this is quite terse and does the job, it won't work with a generator. There's an even
more generic and terser way to do the same thing with `itertools.dropwhile` function. Here's
how to do it:

```python
from itertools import dropwhile

# This returns the same thing as before (0, 4, 5, 6, 7).
from_zero = tuple(dropwhile(lambda x: x != 0, it))
```

Here, `itertools.dropwhile` is a generator function that returns elements from an iterable
starting from the first element for which the predicate returns `False`. The predicate is a
function that takes one argument and returns a boolean value.

The `dropwhile` function takes two arguments:

-   A function (the predicate), which takes one argument and returns a boolean value.
-   An iterable, which can be any object that can be iterated over, such as a list, tuple,
    string, or even another generator.

The `dropwhile` function starts iterating over the elements of the iterable, and drops the
elements for which the predicate returns `True`. It then returns all the remaining elements
of the iterable, regardless of whether they satisfy the condition or not.

Apart from being concise, this implementation is more generic and can be used for other
purposes like skipping the header lines in a file. For example:

```python
from itertools import dropwhile

with open("/etc/passwd") as f:
    for line in dropwhile(lambda x: x.startswith("#"), f):
        print(line)
```

This will print all the lines from the `/etc/passwd` file after the header comments:

```txt
nobody:*:-2:-2:Unprivileged User:/var/empty:/usr/bin/false
root:*:0:0:System Administrator:/var/root:/bin/sh
daemon:*:1:1:System Services:/var/root:/usr/bin/false
...
```

Finally, let's see how you can skip straight to the data rows in a CSV file that contains
arbitrary comments and headers like this:

```txt
# persons.csv

This is a comment
These are some other comments
The fake header starts from the next line

id,name,age,height

The real header starts from here
ID,Name,Age,Height
1,John,20,1.8
2,Jane,21,1.7
3,Jack,22,1.6
```

```python
import csv
from itertools import dropwhile

with open("persons.csv", "r") as f:
    reader = csv.DictReader(
        f, fieldnames=("ID", "Name", "Age", "Height")
    )

    # Rows without comments.
    rows = dropwhile(lambda x: x["ID"] != "ID", reader)

    # Skip the header.
    next(rows)

    for row in rows:
        print(row)
```

Running this will give you the dicts containing the data rows only:

```txt
{'ID': '1', 'Name': 'John', 'Age': '20', 'Height': '1.8'}
{'ID': '2', 'Name': 'Jane', 'Age': '21', 'Height': '1.7'}
{'ID': '3', 'Name': 'Jack', 'Age': '22', 'Height': '1.6'}
```

[^1]:
    [Python Cookbook - David Beazley, Ch 4: Iterators and Generators](https://www.oreilly.com/library/view/python-cookbook-3rd/9781449357337/)
    [^1]

[^2]:
    [itertools.dropwhile](https://docs.python.org/3/library/itertools.html#itertools.dropwhile)
    [^2]

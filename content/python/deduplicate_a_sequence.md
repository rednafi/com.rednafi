---
title: Deduplicating an iterable while preserving order in Python
date: 2023-05-01
tags:
    - Python
---

Whenever I need to deduplicate the elements of an iterable in Python, my usual approach
is to create a set from the iterable and then convert it back into a list or tuple.
However, this approach does not preserve the original order of the elements, which can
be a problem if you need to keep the order unscathed. Here's a naive approach that
works:

```python
from __future__ import annotations
from collections.abc import Iterable  # Python >3.9


def dedup(it: Iterable) -> list:
    seen = set()
    result = []
    for item in it:
        if item not in seen:
            seen.add(item)
            result.append(item)
    return result


it = (2, 1, 3, 4, 66, 0, 1, 1, 1)
deduped_it = dedup(it)  # Gives you [2, 1, 3, 4, 66, 0]
```

This code snippet defines a function `dedup` that takes an iterable `it` as input and
returns a new list containing the unique elements of the input iterable in their
original order. The function uses a set `seen` to keep track of the elements that have
already been seen, and a list `result` to store the unique elements in their original
order.

Then it iterates over all elements of the input iterable using a for loop. For each
element, the function checks if it has already been seen (i.e., if it's in the `seen`
set) using the in operator. If the element hasn't been seen, it's added to both the
`seen` set and the `result` list. The final result list contains the unique elements of
`it` in their original order.

This can be made a little terser by using listcomp as follows:

```python
from __future__ import annotations

from collections.abc import Iterable  # Python >3.9


def dedup(it: Iterable) -> list:
    seen = set()

    # Binding seen.add to a variable reduces the cost of attribute
    # fetching within a tight loop
    seen_add = seen.add

    # Here, 'or' allows us to add the item to 'seen' when it doesn't
    # already exist there in a single line.
    return [item for item in it if not (item in seen or seen_add(item))]
```

## Dedup with collections.OrderedDict

```python
from collections import OrderedDict

dedup = lambda it: list(OrderedDict.fromkeys(seq))

it = (2, 1, 3, 4, 66, 0, 1, 1, 1)
deduped_it = dedup(it)  # Gives you [2, 1, 3, 4, 66, 0]
```

Similar to the first snippet, this also defines `dedup` that takes an iterable `it` as
input and returns a new list containing the unique elements of `it` in their original
order. The function uses the `OrderedDict.fromkeys()` method to create a new ordered
dict with the elements of `it` as keys and `None` as values.

Since an ordered dict maintains the insertion order of its keys, this effectively
removes any duplicate elements from the iterable while preserving the order of the
remaining elements. The iterable containing the keys of the resulting ordered dict is
then converted into a list using the `list()` function to obtain a list of the unique
elements in their original order.

While this is quite terse and does the job with `O(1)` complexity, neither this or the
previous solution would work for compound iterables as follows:

```python
# Here the dedup function will have to remove the duplicate elements by
# keys. So the desired output will be ((1,1), (2, 1), (3, 1))
it = ((1, 1), (2, 1), (3, 1), (1, 1), (1, 3))
```

The next solution works on one-level nested iterables.

## Dedup by any element of an item in a nested iterable

Consider this one-level nested iterable:

```python
# Here, (1,1), (2, 1) are items of the iterable 'it' and 1 is an element
# of the first item (1,1). We're referring items of item as elements.
it = ((1, 1), (2, 1), (3, 1), (1, 1), (1, 3))
```

We want to write a `dedup` function that'll allow us to deduplicate the iterable based
on a particular element of an item. Here, `(1,1)`, `(2, 1)` are items of the iterable
`it` and `1` is the second element of item `(2, 1)`. Here's how we can modify the first
`dedup` to allow deduplication by nested elements.

```python
from __future__ import annotations

from collections.abc import Iterable
from typing import Any, Generator


def dedup(
    it: Iterable[tuple[Any, ...]], index: int, lazy: bool = True
) -> list[Any] | Generator[Any, None, None]:
    seen = set()  # type: set[Any]
    seen_add = seen.add
    expr = (
        item for item in it if not ((elem := item[index]) in seen or seen_add(elem))
    )

    return expr if lazy else list(expr)


it = ((1, 1), (2, 1), (3, 1), (1, 1), (1, 3))

# We're deduplicating by the second element of the items.
dedup(it, 2, False)  # Returns [(1,1), (1,3)]
```

This time, the `dedup` function that takes in an iterable of tuples `it`, an element
index `index`, and a boolean `lazy` (defaulting to `True`) as arguments. The function
returns a list or generator of the unique elements in the input iterable based on the
specified index.

Just as before, the function first creates an empty set `seen` and assigns its `add`
method to a variable `seen_add`. It then creates a generator expression that iterates
over `it` and yields each item if its element at the specified index is not already in
the `seen` set. The element is added to the seen set using the seen_add method.

If `lazy` is `True`, the function returns the generator expression. Otherwise, it
returns a list created from the generator expression.

In the example provided, the function is called with arguments `it`, `2`, and `False`.
This means that it will deduplicate the input iterable based on the second element of
each tuple and return a list. The result is `[(1,1), (1,3)]`.

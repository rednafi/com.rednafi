---
title: Create a sub dictionary with O(K) complexity in Python
slug: create-sub-dict
aliases:
    - /python/create_sub_dict/
Date: 2022-01-30
tags:
    - Python
    - TIL
---

How'd you create a sub dictionary from a dictionary where the keys of the sub-dict are
provided as a list?

I was reading a tweet[^1] by Ned Bachelder on this today and that made me realize that I
usually solve it with `O(DK)` complexity, where `K` is the length of the sub-dict keys and
`D` is the length of the primary dict. Here's how I usually do that without giving it any
thoughts or whatsoever:

```py
# src.py
from __future__ import annotations

main_dict = {
    "this": 0,
    "is": 1,
    "an": 2,
    "example": 3,
    "of": 4,
    "speech": 5,
    "synthesis": 6,
    "in": 7,
    "english": 8,
}

sub_keys = ["this", "is", "an", "example"]

sub_dict = {k: v for k, v in main_dict.items() if k in sub_keys}

print(sub_dict)
```

This prints:

```txt
{'this': 0, 'is': 1, 'an': 2, 'example': 3}
```

While this works fine, if you look carefully you'll notice that in the above snippet, the
complexity of creating the sub-dict is O(DK). This means, in the worst-case scenario, it'll
have to traverse the entire length of the main-dict and all the keys of the sub-dict to
create the sub-dict. We can do better. Consider this:

```py
# src.py
...

# Only this line is different from the previous snippet.
sub_dict = {k: main_dict[k] for k in sub_keys}

...
```

It prints out the same thing as before:

```txt
{'this': 0, 'is': 1, 'an': 2, 'example': 3}
```

It's quite a bit faster because in the worst case scenario, it'll only have to traverse the
entire `sub_keys` list—O(K) complexity achieved. This is so simple and elegant. How did I
miss that! There's another functional but subjectively less readable way of achieving the
same thing. Here you go:

```py
# src.py
from operator import itemgetter

...

sub_dict = dict(zip(sub_keys, itemgetter(*sub_keys)(main_dict)))

...
```

## Benchmarks

I ran this naive benchmark in an ipython console:

```py
...

In [3]: %timeit {k: v for k, v in main_dict.items() if k in sub_keys}
886 ns ± 7.68 ns per loop (mean ± std. dev. of 7 runs, 1,000,000 loops each)

In [4]: %timeit {k:main_dict[k] for k in sub_keys}
340 ns ± 2.87 ns per loop (mean ± std. dev. of 7 runs, 1,000,000 loops each)

In [5]: %timeit dict(zip(sub_keys, itemgetter(*sub_keys)(main_dict)))
581 ns ± 2.73 ns per loop (mean ± std. dev. of 7 runs, 1,000,000 loops each)

...
```

It shows that the solution I was using does suffer from the effects of `O(DK)` complexity
even when the dict size is as small as 9 elements. The second solution is the fastest and
the least complex one to understand. While the third one is better than the first solution,
it's a gratuitously complex way of doing something so trivial.

[^1]: [Ned Bachelder's tweet](https://twitter.com/nedbat/status/1487084661163626506)

[^2]:
    [The second solution came out of a comment on the same tweet](https://twitter.com/__mharrison__/status/1487087733633781766/photo/1)
    [^2]

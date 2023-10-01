---
title: Python's 'functools.partial' flattens nestings Automatically
date: 2021-11-08
tags:
    - Python
---

The constructor for `functools.partial()` detects nesting and automatically flattens itself
to a more efficient form. For example:

```python
from functools import partial


def f(*, a: int, b: int, c: int) -> None:
    print(f"Args are {a}-{b}-{c}")


g = partial(partial(partial(f, a=1), b=2), c=3)

# Three function calls are flattened into one; free efficiency.
print(g)

# Bare function can be called as 3 arguments were bound previously.
g()
```

This returns:

```txt
functools.partial(<function f at 0x7f4fd16c11f0>, a=1, b=2, c=3)
Args are 1-2-3
```

[^1]: [Tweet by Raymond Hettinger](https://twitter.com/raymondh/status/1454865294120325124) [^1]

---
title: "Python's 'functools.partial' flattens nestings Automatically"
slug: functools-partial-flattens-nestings-automatically
date: 2021-11-08
description: >-
    Discover how Python's functools.partial automatically detects and flattens nested
    partial applications for optimal performance and cleaner code.
tags:
    - Python
aliases:
    - /python/functools_partial_flattens_nestings_automatically/
discussions: []
mermaid: false
type_label: ""
atprotoPath: /python/functools-partial-flattens-nestings-automatically/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mnl6jo47ie2b"
---

The constructor for `functools.partial()` detects nesting and automatically flattens itself
to a more efficient form. For example:

```py
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

## Further reading

- [Tweet by Raymond Hettinger]

<!-- references -->
<!-- prettier-ignore-start -->

[tweet by raymond hettinger]:
    https://twitter.com/raymondh/status/1454865294120325124

<!-- prettier-ignore-end -->

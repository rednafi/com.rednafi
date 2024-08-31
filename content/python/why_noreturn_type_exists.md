---
title: Why 'NoReturn' type exists in Python
date: 2022-02-21
tags:
    - Python
    - Typing
---

Technically, the type of `None` in Python is `NoneType`. However, you'll rarely see
`types.NoneType` being used in the wild as the community has pretty much adopted `None` to
denote the type of the `None` singleton. This usage is also documented[^1] in PEP-484.

Whenever a callable doesn't return anything, you usually annotate it as follows:

```python
# src.py
from __future__ import annotations


def abyss() -> None:
    return
```

But sometimes a callable raises an exception and never gets the chance to return anything.
Consider this example:

```python
# src.py
from __future__ import annotations

import logging


def raise_verbose_type_error(message: str) -> None:
    logging.error("Raising type error")
    raise TypeError(message)


if __name__ == "__main__":
    raise_verbose_type_error("type error occured")
```

This semantically makes sense and if you run Mypy against the snippet, it won't complain.
However, there's one difference between a callable that returns an implicit `None` vs one
that raises an exception. In the latter case, if you run any code after calling the
callable, that code won't be reachable. But Mypy doesn't statically catch that or warn you
about the potential dead code. This is apparently fine by the type checker:

```python
...

if __name__ == "__main__":
    raise_verbose_type_error("type error occured")
    print(
        "This part of the code is unreachable due to the exception"
        "above, but Mypy doesn't warn us."
    )
```

`NoReturn` type can be used in cases like this to warn us about potential dead code ahead.
To utilize it, you'd type the above snippet like this:

```python
# src.py
from __future__ import annotations

import logging
from typing import NoReturn


def raise_verbose_type_error(message: str) -> NoReturn:
    logging.error("Raising type error")
    raise TypeError(message)


if __name__ == "__main__":
    raise_verbose_type_error("type error occured")
    print(
        "This part of the code is unreachable due to the exception"
        "above, but this time, Mypy will warn us."
    )
```

Notice, that I changed the return type of the `raise_verbose_type_error` function to
`typing.NoReturn`. Now, if you run Mypy against the snippet with the `--warn-unreachable`
flag, it'll complain:

```sh
mypy --warn-unreachable src.py
```

```txt
src.py:14: error: Statement is unreachable
        print(
        ^
Found 1 error in 1 file (checked 1 source file)
```

## More practical examples

### Callables containing infinite loops

```python
# src.py
from __future__ import annotations

import itertools
from typing import NoReturn


def run_indefinitely() -> NoReturn:
    for i in itertools.cycle("abc"):
        print(i)


if __name__ == "__main__":
    run_indefinitely()
    print(" Dead code. Mypy will warn us.")
```

Mypy will warn us about the dead code.

```txt
src.py:14: error: Statement is unreachable
        print(
        ^
Found 1 error in 1 file (checked 1 source file)
```

Another case where `NoReturn` can be useful, is to type callables with `while True` loops.
This is common in webservers:

```python
# src.py
from __future__ import annotations

from typing import NoReturn


def loop_forever() -> NoReturn:
    while True:
        do_something()
```

### Callables that invoke 'sys.exit()', 'os.\_exit()', 'os.execvp()', etc

Both `sys.exit()` and `os._exit()` do similar things. The former function raises the
`SystemExit()` exception and exits the program without printing any stacktrace or
whatsoever. On the other hand, the latter function exits the process immediately without
letting the interpreter run any cleanup code. Prefer `sys.exit()` over `os._exit()`.

The `os.execvp()` function execute a new program, replacing the current process. It never
returns. Here's how you'd type the callables that call these functions:

```python
# src.py
from __future__ import annotations

import os
import sys
from typing import NoReturn


def call_sys_exit(code: int) -> NoReturn:
    sys.exit(code)


def call_os_exit(code: int) -> NoReturn:
    os._exit(code)


def call_os_execvp() -> NoReturn:
    os.execvp("echo", ("echo", "hi"))
```

[^1]: [Using None](https://www.python.org/dev/peps/pep-0484/#using-none)

[^2]:
    [Python return annotations: NoReturn vs None (intermediate) anthony explains #007](https://www.youtube.com/watch?v=-zH0qqDtd4w)
    [^2]

[^3]:
    [Python type hints - what’s the point of NoReturn? - Adam Johnson](https://adamj.eu/tech/2021/05/20/python-type-hints-whats-the-point-of-noreturn/)
    [^3]

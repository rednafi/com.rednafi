---
title: Gotchas of early-bound function argument defaults in Python
date: 2022-01-27
tags:
    - Python
---

I was reading a tweet about it yesterday and that didn't stop me from pushing a code change
in production with the same rookie mistake today. Consider this function:

```py
# src.py
from __future__ import annotations

import logging
import time
from datetime import datetime


def log(
    message: str,
    /,
    *,
    level: str,
    timestamp: str = datetime.utcnow().isoformat(),
) -> None:
    logger = getattr(logging, level)

    # Avoid f-string in logging as it's not lazy.
    logger("Timestamp: %s \nMessage: %s\n" % (timestamp, message))


if __name__ == "__main__":
    for _ in range(3):
        time.sleep(1)
        log("Reality can often be disappointing.", level="warning")
```

Here, the function `log` has a parameter `timestamp` that computes its default value using
the built-in `datetime.utcnow().isoformat()` method. I was under the impression that the
`timestamp` parameter would be computed each time when the `log` function was called.
However, that's not what happens when you try to run it. If you run the above snippet,
you'll get this instead:

```txt
WARNING:root:Timestamp: 2022-01-27T19:57:34.147403
Message: Reality can often be disappointing.

WARNING:root:Timestamp: 2022-01-27T19:57:34.147403
Message: Reality can often be disappointing.

WARNING:root:Timestamp: 2022-01-27T19:57:34.147403
Message: Reality can often be disappointing.
```

In the `__main__` block, I'm calling the `log` function 3 times with a 1-second delay
between each invocation. But if you take a look at the timestamp of each of the log entries
in the output, you'll notice that all 3 of them are exactly the same.

Default function arguments are early-bound in Python. That means:

> Python interpreter will bind the default parameters at function definition time and will
> use that static value at run time. It's also true for methods. This design choice was
> intentional.

We're getting the same value of the timestamp each time because Python is computing the
value of the default `timestamp` parameter once in the function definition time and then
reusing the same value across all the function calls. The `log` function was called 3 times
but the timestamp function was invoked only once; during the function definition time.

This is easy to fix. Remove the default value of the timestamp and explicitly pass the
parameter value while calling the function:

```py
# src.py
from __future__ import annotations

import logging
import time
from datetime import datetime


def log(
    message: str,
    /,
    *,
    level: str,
    timestamp: str,  # No default value here.
) -> None:
    logger = getattr(logging, level)

    # Avoid f-string in logging as it's not lazy.
    logger("Timestamp: %s \nMessage: %s\n" % (timestamp, message))


if __name__ == "__main__":
    for _ in range(3):
        time.sleep(1)
        log(
            "Reality can often be disappointing.",
            level="warning",
            # Pass this explicitly.
            timestamp=datetime.utcnow().isoformat(),
        )
```

Now if you run it, you'll get this:

```txt
WARNING:root:Timestamp: 2022-01-27T20:19:47.618326
Message: Reality can often be disappointing.

WARNING:root:Timestamp: 2022-01-27T20:19:48.618761
Message: Reality can often be disappointing.

WARNING:root:Timestamp: 2022-01-27T20:19:49.620116
Message: Reality can often be disappointing.
```

Notice, how the values of the seconds in the timestamps have roughly a 1-second delay
between them. Early-bound defaults can also produce surprising results if you try to use a
mutable data structure as the default value of a function/method. Here's an example:

```py
# src.py
from __future__ import annotations

# In <Python 3.9, import this from the 'typing' module.
from collections.abc import MutableSequence
from typing import Any


def append_to(value: Any, target: MutableSequence = []) -> MutableSequence:
    target.append(value)
    return target


if __name__ == "__main__":
    for i in range(3):
        ret = append_to(i)
        print(ret)
```

The function `append_to` takes any object and appends that to the `target` mutable sequence.
Here, the parameter `target` has a default value; an empty list. However, running the
function reveals something unexpected:

```txt
[0]
[0, 1]
[0, 1, 2]
```

Whereas, you might expect it to print out the following:

```txt
[0]
[1]
[2]
```

Python is reusing the same `MutableSequence` that was defined in the function definition
time; just like it was reusing the same return value of the `datetime.utcnow().isoformat()`
in the previous section. To fix this you can do the following:

```py
# src.py
from __future__ import annotations

from collections.abc import MutableSequence
from typing import Any


def append_to(value: Any) -> MutableSequence:
    target = []
    target.append(value)
    return target


if __name__ == "__main__":
    for i in range(3):
        ret = append_to(i)
        print(ret)
```

Running the snippet will produce the expected result this time:

```txt
[0]
[1]
[2]
```

Here, I just omitted the `target` parameter from the `append_to` function signature.
Defining the variable inside the function body can save you from being surprised at the most
unfortunate time.

## Breadcrumbs

Currently, there's an outstanding PEP ([PEP-671][^1]) that proposes late-bound function
argument defaults. It's still in a draft state and I'm quite fond of the syntax that it's
proposing. Here's how you'd make a default parameter late-bound:

```py
def foo(bar, baz => []):
    ...
```

The default parameter `baz` will be late-bound and will produce similar results that we've
seen in the last solution.

[^1]: [PEP-671](https://www.python.org/dev/peps/pep-0671)

[^2]:
    [Mutable default arguments - The hitchhiker's guide to Python!](https://docs.python-guide.org/writing/gotchas/#mutable-default-arguments)
    [^2]

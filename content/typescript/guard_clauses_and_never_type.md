---
title: Guard clause and exhaustiveness checking
date: 2022-05-22
tags:
    - TypeScript
    - Python
    - Typing
---

Nested conditionals suck. They're hard to write and even harder to read. I've rarely
regretted the time I've spent optimizing for the flattest conditional structure in my code.
The following piece mimics the actions of a traffic signal:

```ts
// src.ts

enum Signal {
  YELLOW = "Yellow",
  RED = "Red",
  GREEN = "Green",
}

function processSignal(signal: Signal) :void {
  if (signal === Signal.YELLOW) {
    console.log("Slow down!");
  } else {
    if (signal === Signal.RED) {
      console.log("Stop!");
    } else {
      if (signal === Signal.GREEN) {
        console.log("Go!");
      }
    }
  }
}

// Log
processSignal(Signal.YELLOW) // prints 'Slow down!'
processSignal(Signal.RED) // prints 'Stop!'
```

The snippet above suffers from two major issues:

- It contains three contiguous levels of nested conditionals.
- The conditionals don't cover the case where the return value is `undefined`.
- If you add a fourth member to the `Signal` enum, now the processing function doesn't
  exhaustively cover all the cases and it won't communicate that fact with you.

We can leverage _guard clauses_ to fix the first two issues.

> The guard (clause) provides an early exit from a subroutine, and is a commonly used
> deviation from structured programming, removing one level of nesting and resulting in
> flatter code: replacing `if guard { ... }` with `if not guard: return`; ...

We can rewrite the earlier snippet as follows:

```ts
// ...snip...

function processSignal(signal: Signal) {
  if (signal === Signal.YELLOW) {
    return "Slow down!";
  }
  if (signal === Signal.RED) {
    return "Stop!";
  }
  if (signal === Signal.GREEN) {
    return "Go!";
  } else {
    return "Not a valid input!";
  }
}
```

This model has a flatter structure and now it's gracefully handling the `undefined` return
path. However, the third issue still persists. In an alien world, if someone added a fourth
member to the `Signal` enum, that'd make the conditional flow in the `processSignal`
function incomplete since it wouldn't be covering that newly added fourth enum member. In
that case, the above snippet will execute the final catch-all conditional statement; not
something that we'd want.

TypeScript provides a `never` type to throw a compilation error if a new member isn't
covered by the conditional flow. Here's how you'd leverage it:

```ts
// src.ts

enum Signal {
  YELLOW = "Yellow",
  RED = "Red",
  GREEN = "Green",
  PURPLE = "Purple", // Newly added member.
}

function assertNever(value: never) {
  throw Error(`Invalid value: ${value}`);
}

function processSignal(signal: Signal) {
  if (signal === Signal.YELLOW) {
    return "Slow down!";
  }
  if (signal === Signal.RED) {
    return "Stop!";
  }
  if (signal === Signal.GREEN) {
    return "Go!";
  }
  // Try commenting out this line and typescript compiler
  // will throw an error.
  if (signal === Signal.PURPLE) {
    return "Go faster!";
  }
  assertNever(signal);
}

processSignal(Signal.PURPLE);
```

Ideally, the `assertNever` should never be called. Try removing a conditional and see how
TypeScript starts screaming at you regarding the unhandled case. The `assertNever` function
will also raise a runtime error if any case remains unhandled.

## Example in Python

The same idea can be demonstrated in Python using Python3.10's `match` statement and
`typing.NoReturn` type.

```py
# src.py (Python 3.10+)

from __future__ import annotations

from enum import Enum
from typing import NoReturn


class Signal(str, Enum):
    YELLOW = "Yellow"
    RED = "Red"
    GREEN = "Green"
    PURPLE = "Purple"


def assert_never(value: NoReturn) -> NoReturn:
    raise AssertionError(f"Invalid value: {value!r}")


def process_signal(signal: Signal) -> str:
    match signal:
        case Signal.YELLOW:
            return "Slow down!"

        case Signal.RED:
            return "Stop!"

        case Signal.GREEN:
            return "Go!"

        # Try commenting out this line and mypy will throw
        # an error.
        case Signal.PURPLE:
            return "Go faster!"

        case _:
            assert_never(signal)


if __name__ == "__main__":
    print(process_signal(Signal.PURPLE))
```

Similar to TypeScript, mypy will complain if you add a new member to the enum but forget to
handle that in the processor function. Python 3.11 added the `Never` type and `assert_never`
function to the `typing` module. Underneath, `Never` is an alias to the `NoReturn` type; so
you can use them interchangeably. However, in this case, `Never` seems to communicate the
intent better. You may also choose to use the backported versions of the type and function
from the `typing_extensions` module. Here's how:

```py
# src.py

from __future__ import annotations

import sys
from enum import Enum

if sys.version_info < (3, 11):
    from typing_extensions import assert_never
else:
    from typing import assert_never

...
```

[^1]:
    [Guard clause, guard code, or guard statement](<https://en.wikipedia.org/wiki/Guard_(computer_science)>)
    [^1]

[^2]: [Never type in TypeScript](https://www.zhenghao.io/posts/ts-never) [^2]

[^3]:
    [Unreachable Code and Exhaustiveness Checking in Python](https://typing.readthedocs.io/en/latest/source/unreachable.html)
    [^3]

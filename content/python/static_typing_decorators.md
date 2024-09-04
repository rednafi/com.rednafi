---
title: Static typing Python decorators
date: 2022-01-23
tags:
    - Python
    - Typing
---

Accurately static typing decorators in Python is an icky business. The **wrapper** function
obfuscates type information required to statically determine the types of the parameters and
the return values of the **wrapped** function.

Let's write a decorator that registers the decorated functions in a global dictionary during
function definition time. Here's how I used to annotate it:

```python
# src.py
# Import 'Callable' from 'typing' module in < Py3.9.
from collections.abc import Callable
from functools import wraps
from typing import Any, TypeVar

R = TypeVar("R")

funcs = {}


def register(func: Callable[..., R]) -> Callable[..., R]:
    """Register any function at definition time in
    the 'funcs' dict."""

    # Registers the function during function defition time.
    funcs[func.__name__] = func

    @wraps(func)
    def inner(*args: Any, **kwargs: Any) -> Any:
        return func(*args, **kwargs)

    return inner


@register
def hello(name: str) -> str:
    return f"Hello {name}!"
```

The `functools.wraps` decorator makes sure that the identity and the docstring of the
wrapped function don't get gobbled up by the decorator. This is syntactically correct and if
you run Mypy against the code snippet, it'll happily tell you that everything's alright.
However, this doesn't exactly do anything. If you call the `hello` function with the wrong
type of parameter, Mypy won't be able to detect the mistake statically. Notice this:

```python
...

hello(1)  # Mypy doesn't complain about it all
```

All this for nothing!

PEP-612[^1] proposed `ParamSpec` and `Concatenate` in the `typing` module to address this
issue. Later on, these were introduced in Python 3.10. The former is required to precisely
add type hints to any decorator while the latter is needed to type annotate decorators that
change wrapped functions' signatures.

> If you're not on Python 3.10+, you can import `ParamSpec` and `Concatenate` from the
> `typing_extensions` module. The package gets automatically installed with Mypy.

## Use `ParamSpec` to type decorators

I'll take advantage of both `ParamSpec` and `TypeVar` to annotate the `register` decorator
that we've seen earlier:

```python
# src.py

# Import 'Callable' from 'typing' module in < Py3.9.
from collections.abc import Callable
from functools import wraps
from typing import ParamSpec, TypeVar

P = ParamSpec("P")
R = TypeVar("R")

funcs = {}


def register(func: Callable[P, R]) -> Callable[P, R]:
    funcs[func.__name__] = func

    @wraps(func)
    def inner(*args: P.args, **kwargs: P.kwargs) -> R:
        return func(*args, **kwargs)

    return inner


@register
def hello(name: str) -> str:
    return f"Hello {name}!"


# Try calling the function with the wrong param type.
print(hello(1))  # Mypy will complain here!
```

Above, I've used `ParamSpec` to annotate the type of the wrapped function's input parameters
and `TypeVar` to annotate its return value. Underneath, `ParamSpec` is a type variable
similar to `TypeVar` but with a trick under its sleeve; it can relay type information to a
decorator's inner callable.

Notice the annotations of the `inner` function inside `register`. Here, `P.args` and
`P.kwargs` are transferring the type information from the wrapped `func` to the `inner`
function. This makes sure that static type checkers like Mypy can now precisely scream at
you whenever you call the decorated functions with the wrong type of parameters.

## Use `Concatenate` to type decorators that change the wrapped functions' signatures

There's another type of decorator that changes the signature of the wrapped function by
adding or removing parameters during runtime. Annotating these can be tricky; as the magic
happens mostly during runtime. The `Concatenate` type allows us to communicate this behavior
with the type checker.

Consider this `inject_logger` decorator, that adds a logger instance to the decorated
function. It sort of acts how Django injects the `request` instances into the **view**
functions. Here's the typed version of that:

```python
# src.py
import logging

# Import 'Callable' from 'typing' module in < Py3.9.
from collections.abc import Callable
from functools import wraps
from typing import Concatenate, ParamSpec, TypeVar

P = ParamSpec("P")
R = TypeVar("R")


def inject_logger(
    func: Callable[Concatenate[logging.Logger, P], R],
) -> Callable[P, R]:
    # Runs this during function definition time only.
    logger = logging.getLogger(func.__name__)

    @wraps(func)
    def inner(*args: P.args, **kwargs: P.kwargs) -> R:
        return func(logger, *args, *kwargs)

    return inner


@inject_logger
def hello(logger: logging.Logger, name: str) -> None:
    logger.warning("Spooky action in distance...")
    return f"Hello {name}!"


# Notice how you can call the hello function without
# inserting the first parameter. The decorator does
# that for you.
print(hello("world"))
```

This is a contrived example and a gratuitously complicated way to achieve a simple goal.
Also, it's not recommended to mutate function signatures like this in runtime. But it's
allowed and now Python gives you a way to statically type check the decorator and the
decorated function.

The only thing that's different from the previous section is the annotation of the `func`
parameter of the `inject_logger`. Notice how the `Callable` generic now contain
`Concatenate[logging.Logger, P]`. The first parameter of the `Concatenate` generic is the
injected parameter—`logging.Logger` in this case. Since the instance of `logging.Logger`
gets dynamically injected, an additional paradigm `Concatenate` is necessary to communicate
that with the type checker.

If you'd defined `hello` with the wrong types, the type checker would've complained.

```python
...


@inject_logger
def hello(logger: int, name: str) -> str:
    logger.warning("Spooky action in distance...")
    return f"Hello {name}!"
```

Above, I've changed the type of the `logger` parameter from `logging.Logger` to `int`. The
type checker will now dutifully chastise us for our transgressions.

Unfortunately, as of writing this post, Mypy doesn't understand `Concatenate` but
Microsoft's Pyright[^2] does. You can pip install Pyright and test out the above snippet as
follows:

```sh
pyright src.py
```

This will return:

```txt
...
Parameter 1: type "Logger" cannot be assigned to type "int"
"Logger" is incompatible with "int" (reportGeneralTypeIssues)
./src.py:83:12 - error: Cannot access member "warning" for type "int"
```

[^1]:
    [PEP 612 -- Parameter specification variables](https://www.python.org/dev/peps/pep-0612/)

[^2]: [pyright](https://github.com/microsoft/pyright)

[^3]:
    [Decorator typing (PEP 612) - Anthony explains #386](https://www.youtube.com/watch?v=fwZoxWyMGM8)
    [^3]

---
title: Structural subtyping in Python
date: 2021-12-04
tags:
    - Python
    - Typing
---

I love using Go's interface feature to declaratively define my public API structure.
Consider this example:

```go
package main

import (
    "fmt"
)

// Declare the interface.
type Geometry interface {
    area() float64
    perim() float64
}

// Struct that represents a rectangle.
type rect struct {
    width, height float64
}

// Method to calculate the area of a rectangle instance.
func (r *rect) area() float64 {
    return r.width * r.height
}

// Method to calculate the perimeter of a rectange instance.
func (r *rect) perim() float64 {
    return 2 * (r.width + r.height)
}

// Notice that we're calling the methods on the interface,
// not on the instance of the Rectangle struct directly.
func measure(g Geometry) {
    fmt.Println(g)
    fmt.Println(g.area())
    fmt.Println(g.perim())
}

func main() {
    r := &rect{width: 3, height: 4}

    measure(r)
}
```

You can play around with the example here[^1]. Running the example will print:

```txt
&{3 4}
12
14
```

Even if you don't speak Go, you can just take a look at the `Geometry` interface and
instantly know that the function `measure` expects a struct that implements the `Geometry`
interface where the `Geometry` interface is satisfied when the struct implements two
methods—`area` and `perim`. The function `measure` doesn't care whether the struct is a
rectangle, a circle, or a square. As long as it implements the interface `Geometry`,
`measure` can work on it and calculate the area and the perimeter.

This is extremely powerful as it allows you to achieve polymorphism like dynamic languages
without letting go of type safety. If you try to pass a struct that doesn't fully implement
the interface, the compiler will throw a type error.

In the world of Python, this polymorphism is achieved dynamically. Consider this example:

```python
def find(haystack, needle):
    return needle in haystack
```

Here, the type of the `haystack` can be anything that supports the `in` operation. It can be
a `list`, `tuple`, `set`, or `dict`; basically, any type that has the `__contains__` method.
Python's duck typing is more flexible than any static typing as you won't have to tell the
function anything about the type of the parameters and it'll work spectacularly; it's a
dynamically typed language, duh! The only problem is the lack of type safety. Since there's
no compilation step in Python, it won't stop you from accidentally putting a type that
`haystack` doesn't support and Python will only raise a `TypeError` when your try to run the
code.

In bigger codebases, this can often become tricky as it's difficult to tell the types of
these uber-dynamic function parameters without reading through all the methods that are
being called on them. In this situation, we want the best of the both world; we want the
flexibility of dynamic polymorphism and at the same time, we want some sort of type safety.
Moreover, the Go code is self-documented to some extent and I'd love this kind of
`polymorphic | type-safe | self-documented` trio in Python. Let's try to use nominal type
hinting to statically type the following example:

```python
# src.py
from __future__ import annotations

from typing import TypeVar

T = TypeVar("T")


def find(haystack: dict, needle: T) -> bool:
    return needle in haystack


if __name__ == "__main__":
    haystack = {1, 2, 3, 4}
    needle = 3

    print(contains(haystack, needle))
```

In this snippet, we're declaring `haystack` to be a `dict` and then passing a `set` to the
function parameter. If you try to run this function, it'll happily print `True`. However, if
you run mypy[^2] against this file, it'll complain as follows:

```txt
src.py:17: error: Argument 1 to "find" has incompatible type "Set[int]"; expected
"Dict[Any, Any]"
        print(contains(haystack, needle))
                       ^
Found 1 error in 1 file (checked 4 source files)
make: *** [makefile:61: mypy] Error 1
```

That's because mypy expects the type to be a dict but we're passing a `set` which is
incompatible. During runtime, Python doesn't raise any error because the `set` that we're
passing as the value of `haystack`, supports `in` operation. But we're not communicating
that with the type checker properly and mypy isn't happy about that. To fix this mypy error,
we can use Union type.

```python
...


def contains(haystack: dict | set, needle: T) -> bool:
    return needle in haystack


...
```

This will make mypy happy. However, it's still not bulletproof. If you try to pass a `list`
object as the value of `haystack`, mypy will complain again. So, nominal typing can get a
bit tedious in this kind of situation, as you'd have to explicitly tell the type checker
about every type that a variable can expect. There's a better way!

Enter structural subtyping[^3]. We know that the value of `haystack` can be anything that
has the `__contains__` method. So, instead of explicitly defining the name of all the
allowed types—we can create a class, add the `__contains__` method to it, and signal mypy
the fact that `haystack` can be anything that has the `__contains__` method. Python's
`typing.Protocol` class allows us to do that. Let's use that:

```python
# src.py
from __future__ import annotations

from typing import Protocol, TypeVar, runtime_checkable

T = TypeVar("T")


@runtime_checkable
class ProtoHaystack(Protocol):
    def __contains__(self, obj) -> bool:
        ...


def find(haystack: ProtoHaystack, needle: T) -> bool:
    return needle in haystack


if __name__ == "__main__":
    haystack = {1, 2, 3, 4}
    needle = 3

    print(find(haystack, needle))
    print(isinstance(ProtoHaystack, haystack))
```

Here, the `ProtoHaystack` class statically defines the structure of the type of objects that
are allowed to be passed as the value of `haystack`. The instance method `__contains__`
accepts an object (obj) as the second parameter and returns a boolean value based on the
fact whether that `obj` exists in the `self` instance or not. Now if you run mypy on this
snippet, it'll be satisfied.

The `runtime_checkable` decorator on the `ProtoHaystack` class allows you to check whether a
target object is an instance of the `ProtoHaystack` class in runtime via the `isinstance()`
function. Without the decorator, you'll only be able to test the conformity of an object to
`ProtoHaystack` statically but not in runtime.

This pattern of strurctural duck typing is so common, that the mixins in the
`collections.abc` module are now compatible with structural type checking. So, in this case,
instead of creating a `ProtoHaystack` class, you can directly use the
`collections.abc.Container` class from the standard library and it'll do the same job.

```python
...

from collections.abc import Container


def find(haystack: Container, needle: T) -> bool:
    return needle in haystack


...
```

## Avoid `abc` inheritance

Abstract base classes in Python let you validate the structure of subclasses in runtime.
Python's standard library APIs uses `abc.ABC` in many places. See this example:

```python
# src.py
from __future__ import annotations

from abc import (
    ABC,
    abstractmethod,
    abstractclassmethod,
    abstractproperty,
)


class FooInterface(ABC):
    @abstractmethod
    def bar(self) -> str:
        pass

    @abstractclassmethod
    def baz(cls) -> str:
        pass

    @abstractproperty
    def qux(self) -> str:
        pass


class Foo(FooInterface):
    """Foo implements FooInterface."""

    def bar(self) -> str:
        return "from instance method"

    @classmethod
    def baz(cls) -> str:
        return "from class method"

    @property
    def qux(self) -> str:
        return "from property method"
```

Here, the class `FooInterface` inherits from `abc.ABC` and then the methods are decorated
with `abstract*` decorators. The combination of `abc.ABC` class and these decorators make
sure that any class that inherits from `FooInterface` will have to implement the `bar`,
`baz`, and `qux` methods. Failing to do so will raise a `TypeError`. The `Foo` class
implements the `FooInterface`.

This works well in theory and practice but often time, people inadvertently start to use the
`*Interface` classes to share implementation methods with the subclasses. When you pollute
your interface with implementation methods, theoretically, it no longer stays and
**interface** class. Go doesn't even allow you to add implementation methods to interfaces.
Abstract base classes have their places and often time, you can't avoid them if you need a
dependable runtime interface conformity check.

However, more often than not, using `Protocol` classes with `@runtime_checkable` decorator
works really well. Here, the `Protocol` class implicitly (just like Go interfaces) makes
sure that your subclasses conform to the structure that you want, and the decorator, along
with `isinstance` check can guarantee the conformity in runtime. Let's replace the `abc.ABC`
and the shenanigans with the decorators with `typing.Protocol`:

```python
from __future__ import annotations

from typing import Protocol, runtime_checkable


@runtime_checkable
class ProtoFoo(Protocol):
    def bar(self) -> str:
        ...

    @classmethod
    def baz(cls) -> str:
        ...

    @property
    def qux(self) -> str:
        ...


class Foo:
    def bar(self) -> str:
        return "from instance method"

    @classmethod
    def baz(cls) -> str:
        return "from class method"

    @property
    def qux(self) -> str:
        return "from property method"


def run(foo: ProtoFoo) -> None:
    if not isinstance(foo, ProtoFoo):
        raise Exception("Foo do not conform to Protofoo interface")

    print(foo.bar())
    print(foo.baz())
    print(foo.qux)


if __name__ == "__main__":
    foo = Foo()
    run(foo)
```

Notice that `Foo` is not inheriting from `ProtoFoo` and when you run mypy against the
snippet, it'll statically check whether `Foo` conforms to the `ProtoFoo` interface or not.
Voila, we avoided inheritance. The `isinstance` in the `run` function later checks whether
`foo` is an instance of `ProtoFoo` or not.

## Complete example with tests

This example employs static duck-typing to check the type of `WebhookPayload` where the
class represents the structure of the payload that is going to be sent to an URL by the
`send_webhook` function.

```python
# Placeholder python file to test the snippets
from __future__ import annotations

import json
import unittest
from dataclasses import asdict, dataclass
from typing import Protocol, runtime_checkable


@runtime_checkable
class ProtoPayload(Protocol):
    url: str
    message: str

    @property
    def json(self):
        ...


@dataclass
class WebhookPayload:
    url: str = "https://dummy.com/post/"
    message: str = "Dummy message"

    @property
    def json(self) -> str:
        return json.dumps(asdict(self))


# Notice the type accepted by the function.
# Go-like static polymorphism right there!
def send_webhook(payload: ProtoPayload) -> None:
    """
    This function doesn't care what type of Payload it gets
    as long as the payload conforms to 'ProtoPayload' structure.
    """

    print(f"Webhook message: {payload.json}")
    print(f"Sending webhook to {payload.url}")


class TestWebHookPayload(unittest.TestCase):
    def setUp(self):
        self.payload = WebhookPayload()

    def test_payload(self):
        # We can do isinstance check because we decorated the
        # 'ProtoPayload' class with the 'runtime_checkable' decorator.
        implements_protocol = isinstance(self.payload, ProtoPayload)
        self.assertEqual(implements_protocol, True)


if __name__ == "__main__":
    unittest.main()
```

## Disclaimer

All the code snippets here are using Python 3.10's type annotation syntax. However, if
you're using `from __future__ import annotations`, you'll be able to run all of them in
earlier Python versions, going as far back as Python 3.7.

[^1]: [Interface - Go playground](https://go.dev/play/p/RG82v5Ubdlc)
[^2]: [mypy](https://mypy.readthedocs.io/en/stable/)
[^3]:
    [Nominal vs structural subtyping](https://www.python.org/dev/peps/pep-0544/#nominal-vs-structural-subtyping)

[^4]:
    [PEP 544 -- Protocols: Structural subtyping (static duck typing)](https://www.python.org/dev/peps/pep-0544/)
    [^4]

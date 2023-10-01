---
title: Self type in Python
date: 2022-02-28
tags:
    - Python
    - Typing
---

PEP-673[^1] introduces the `Self` type and it's coming to Python 3.11. However, you can
already use that now via the `typing_extenstions`[^2] module.

The `Self` type makes annotating methods that return the instances of the corresponding
classes trivial. Before this, you'd have to do some mental gymnastics to statically type
situations as follows:

```python
# src.py
from __future__ import annotations

from typing import Any


class Animal:
    def __init__(self, name: str, says: str) -> None:
        self.name = name
        self.says = says

    @classmethod
    def from_description(cls, description: str = "|") -> Animal:
        descr = description.split("|")
        return cls(descr[0], descr[1])


class Dog(Animal):
    def __init__(self, *args: Any, **kwargs: Any) -> None:
        super().__init__(*args, **kwargs)

    @property
    def legs(self) -> int:
        return 4


if __name__ == "__main__":
    dog = Dog.from_description("Matt | woof")
    print(dog.legs)  # Mypy complains here!
```

The class `Animal` has a `from_description` class method that acts as an additional
constructor. It takes a description string, and then builds and returns an instance of the
same class. The return type of the method is annotated as `Animal` here. However, doing this
makes the child class `Dog` conflate its identity with the `Animal` class. If you execute
the snippet, it won't raise any runtime error. Also, Mypy will complain about the type:

```txt
src.py:27: error: "Animal" has no attribute "legs"
        print(dog.legs)  # Mypy complains here!
              ^
Found 1 error in 1 file (checked 1 source file)
```

To fix this, we'll have to make sure that the return type of the `from_description` class
method doesn't confuse the type checker. This is one way to do this:

```python
from __future__ import annotations

from typing import TypeVar

T = TypeVar("T", bound="Animal")


class Animal:
    def __init__(self, name: str, says: str) -> None:
        self.name = name
        self.says = says

    @classmethod  # In <Python3.9, Use typing.Type[T].
    def from_description(cls: type[T], description: str = "|") -> T:
        descr = description.split("|")
        return cls(descr[0], descr[1])


...
```

In the above snippet, first I had to declare a `TypeVar` and bind that to the `Animal`
class. Then I had to explicitly type the `cls` variable in the `from_description` method.
This time, the type checker will be happy. While this isn't a lot of work, it surely goes
against the community convention. Usually, we don't explicitly type the `self`, `cls`
variables and instead, let the type checker figure out their types. Also, subjectively, this
sticks out like a sore thumb.

PEP-673 allows us to solve the issue elegantly:

```python
# src.py
from __future__ import annotations

import sys

if sys.version_info >= (3, 11):
    from typing import Self
else:
    from typing_extensions import Self


class Animal:
    def __init__(self, name: str, says: str) -> None:
        self.name = name
        self.says = says

    @classmethod
    def from_description(cls, description: str = "|") -> Self:
        descr = description.split("|")
        return cls(descr[0], descr[1])


...
```

If you run Mypy against the second snippet, it won't complain.

## Typing instance methods that return `self`

Take a look at this:

```python
# src.py
from __future__ import annotations

import sys

if sys.version_info >= (3, 11):
    from typing import Self
else:
    from typing_extensions import Self


class Counter:
    def __init__(self, start: int = 1) -> None:
        self.val = start

    def increment(self) -> Self:
        self.val += 1
        return self

    def decrement(self) -> Self:
        self.val -= 1
        return self
```

The `increment` and `decrement` method of the `Counter` class return the instance of the
same class after performing the operations on the `start` value. This is a perfect case
where the `Self` type can be useful.

## Typing `__new__` methods

You can also type the `__new__` method easily:

```python
from __future__ import annotations

import sys

if sys.version_info >= (3, 11):
    from typing import Self
else:
    from typing_extensions import Self

from typing import Any


class Config:
    def __new__(cls, var: int, *args: Any, **kwargs: Any) -> Self:
        """Validate the value before constructing the class."""

        if not 0 <= var < 10:
            raise TypeError(
                "'var' must be a positive integer between 0 and 9",
            )
        return super().__new__(cls)

    def __init__(self, var: int) -> None:
        self.var = var
```

The `__new__` method in the `Config` class validates the `var` before constructing an
instance of the class. The `Self` type makes it easy to annotate the method.

[^1]: [PEP 673 -- Self Type](https://www.python.org/dev/peps/pep-0673/)
[^2]: [typing_extensions](https://typing.readthedocs.io/)
[^3]: [Tweet by Raymond Hettinger](https://twitter.com/raymondh/status/1491187805636407298) [^3]

---
title: Explicit method overriding with @typing.override
date: 2024-11-06
tags:
    - Python
    - TIL
---

Although I've been using Python 3.12 in production for nearly a year, one neat feature in
the typing module that escaped me was the `@override` decorator. Proposed in PEP-698[^1],
it's been hanging out in `typing_extensions` for a while. This is one of those small
features you either don't care about or get totally psyched over. I'm definitely in the
latter camp.

In languages like C#, Java, and Kotlin, explicit overriding is required. For instance, in
Java, you use `@Override` to make it clear you're overriding a method in a sub class. If you
mess up the method name or if the method doesn't exist in the superclass, the compiler
throws an error. Now, with Python's `@override` decorator, we get similar benefits—though
only if you're using a static type checker.

Here's an example:

```py
from typing import override


class Animal:
    def sound(self) -> str:
        return "Unknown"


class Cat(Animal):
    @override
    def soud(self) -> str:  # Notice the typo: sound -> soud
        # Your implementation here
        return "Meow"
```

In this example, `Cat` inherits from `Animal`, and we intended to override the `sound`
method. But there's a typo in the subclass method name. Running `mypy` will flag it:

```txt
error: Method "soud" is marked as an override, but no base method was found
with this name  [misc]

Found 1 error in 1 file (checked 1 source file)
```

This decorator also works with class, property, or any other methods. Observe:

```py
from typing import override


class Animal:
    @property
    def species(self) -> str:
        return "Unknown"


class Cat(Animal):
    @override
    @property
    def species(self) -> str:
        return "Catus"
```

If the overriding method isn't marked with `@property`, `mypy` will raise an error:

```txt
error: Signature of "species" incompatible with supertype "Animal"  [override]
note:      Superclass:
note:          str
note:      Subclass:
note:          def species(self) -> str
Found 1 error in 1 file (checked 1 source file)
```

The error message could be clearer here, though. You can use `@override` with class methods
too:

```py
from typing import override


class Animal:
    @classmethod
    def category(cls) -> str:
        return "Unknown"


class Cat(Animal):
    @override
    @classmethod
    def category(cls) -> str:
        return "Mammal"
```

In these cases, the order of `@override` doesn't matter; you can put it before or after the
`property` decorator, and it'll still work. I personally prefer keeping it as the outermost
decorator.

I've been gradually adding the `@override` decorator to my code, as it not only prevents
typos but also alerts me if an upstream method name changes.

[^1]: [PEP 698 – Override decorator for static typing](https://peps.python.org/pep-0698/)

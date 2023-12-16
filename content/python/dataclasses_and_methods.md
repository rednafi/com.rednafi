---
title: Banish state-mutating methods from data classes
date: 2023-12-16
tags:
    - Python
---

Data classes are containers for your data—not behavior. The delineation is right there in
the name. Yet, I see state-mutating methods getting crammed into data classes and polluting
their semantics all the time. While this text will primarily talk about data classes in
Python, the message remains valid for any language that supports data classes and allows you
to add state-mutating methods to them, e.g., Kotlin, Swift, etc. By state-mutating method, I
mean methods that change attribute values during runtime. For instance:

```python
from dataclasses import dataclass


@dataclass
class Person:
    name: str
    age: int

    def make_older(by=1) -> None:
        self.age += by
```

In this case, calling the `make_older` method will change the value of `age` in-place.

Every time I spot a data class decked out with such methods, I feel like I'm looking at the
penguin with an elephant head[^1] from the Family Guy. Whenever I traverse down to see how
the instances of the class are being used, more often than not, I find them being treated
just like regular mutable class instances with fancy `repr`s. But if you only need a nice
`repr` for your large OO class, adding a `__repr__` to the class definition is not that
difficult. Why pay the price for building heavier data class instances only for that?

In Python, data classes are considerably slower[^2] to instantiate compared to vanilla
classes. However, they serve a different purpose than your typical run-of-the-mill classes.
When you decorate a class with the `@dataclass` decorator without changing any of the
default parameters, Python automatically generates `__init__`, `__eq__`, and `__repr__`
methods. If you set `@dataclass(order=True)`, it'll also generate `__lt__`, `__le__`,
`__gt__`, and `__ge__` special methods that enable you to compare and sort the data class
instances. All of this implicates that the construct was specifically designed to contain
rich data that provides the means for you to create nice abstractions around lower-level
primitives.

My gripe isn't against using data classes because of their heavier size. If it were, Python
probably wouldn't be one of my favorite languages. I use data classes all the time and love
how they often allow me to craft nicer APIs with little effort. My issue is when people add
state-mutating methods to data classes. The moment you're doing that, you're breaking the
semantics of the data structure. You probably wouldn't use hashmaps to represent sequential
data even though Python currently maintains[^3] the insertion order of the keys in dicts.

In Kotlin, I almost always define immutable data classes and pass them around in different
functions that perform transformations and calculations. In Python, however, instantiating
frozen data classes (`@dataclass(frozen=True)`) is almost twice as slow[^4] compared to
mutable data classes. So I just set `slots=True` to make the instantiation quicker and call
it a day. But in either case, if I need to add a method that mutates the attributes of the
class instance, I reconsider whether a data class is the right abstraction for the problem
at hand. The necessity to add a state-mutating method is an indicator that you need a
regular OO class. You'll signal incorrect intent to the reader if you keep using data
classes in this context.

Dataclasses are also great candidates for domain modeling with types. With the help of mypy,
you can leverage sum types[^5] to emulate ADTs[^6] as follows (using PEP-695[^7] generic
syntax):

```python
from dataclasses import dataclass


@dataclass(slots=True)
class Barcode[T: str | int]:
    code: T


@dataclass(slots=True)
class Sku[T: str | int]:  # Stock Keeping Unit
    code: T


type ProductId = Barcode | Sku | None
```

But it only works if your data containers don't exhibit any behavior. Here the data classes
are just labels for values in a set that can contain the instances of the classes. Adding
state-mutating methods to either Barcode or Sku would break the semantics of how these types
can be composed.

I still think it's okay if you need to validate the data class attributes in a
`__post_init__` method or override the `__eq__` or `__hash__` for some reason. Read-only
methods are also acceptable since they don't do in-place state modification. Comparing two
data class instances that have read-only methods is not as awkward as comparing data class
instances with methods that mutate attributes. So if you need to slap a method on a data
class, write a function and pass the instance as a parameter or write a normal class with a
repr and add the method there. This way, the reader won't have to wonder whether your data
containers have some hidden behavior attached to them or not.

[^1]: [Penguin with an elephant head – Family Guy](https://i.imgflip.com/3gb0nh.jpg?a472776)
[^2]:
    [Improving data classes startup performance](https://discuss.python.org/t/improving-dataclasses-startup-performance/15442/20)

[^3]:
    [Dicts are now ordered, get used to it](https://softwaremaniacs.org/blog/2020/02/05/dicts-ordered/en/)

[^4]:
    [Frozen data classes are slower](https://docs.python.org/3.12/library/dataclasses.html#frozen-instances)

[^5]: [Sum types](https://fsharpforfunandprofit.com/posts/discriminated-unions/)
[^6]:
    [Algebraic Data Types in (typed) Python](https://threeofwands.com/algebraic-data-types-in-python/)

[^7]: [PEP 695 – Type Parameter Syntax](https://peps.python.org/pep-0695/)

---
title: Reduce boilerplate code with Python's dataclasses
date: 2020-03-12
tags:
    - Python
---

Recently, my work needed me to create lots of custom data types and draw comparison among
them. So, my code was littered with many classes that somewhat looked like this:

```python
class CartesianPoint:
    def __init__(self, x, y, z):
        self.x = x
        self.y = y
        self.z = z

    def __repr__(self):
        return f"CartesianPoint(x = {self.x}, y = {self.y}, z = {self.z})"


print(CartesianPoint(1, 2, 3))
```

```txt
>>> CartesianPoint(x = 1, y = 2, z = 3)
```

This class only creates a `CartesianPoint` type and shows a pretty output of the instances
created from it. However, it already has two methods inside, `__init__` and `__repr__` that
do not do much.

## Dataclasses

Let's see how data classes can help to improve this situation. Data classes were introduced
to python in version 3.7. Basically they can be regarded as code generators that reduce the
amount of boilerplate you need to write while generating generic classes. Rewriting the
above class using `dataclass` will look like this:

```python
from dataclasses import dataclass


@dataclass
class CartesianPoint:
    x: float
    y: float
    z: float


# using the class
point = CartesianPoint(1, 2, 3)
print(point)
```

```txt
>>> CartesianPoint(x=1, y=2, z=3)
```

In the above code, the magic is done by the `dataclass` decorator. Data classes require you
to use explicit type annotations[^1] and it automatically implements methods like
`__init__`, `__repr__`, `__eq__` etc beforehand. You can inspect the methods that
`dataclass` auto defines via Python's help.

```python
help(CartesianPoint)
```

```txt
Help on class CartesianPoint in module __main__:

class CartesianPoint(builtins.object)
 |  CartesianPoint(x:float, y:float, z:float)
 |
 |  Methods defined here:
 |
 |  __eq__(self, other)
 |
 |  __init__(self, x:float, y:float, z:float) -> None
 |
 |  __repr__(self)
 |
 |  ----------------------------------------------------------------------
 |  Data descriptors defined here:
 |
 |  __dict__
 |      dictionary for instance variables (if defined)
 |
 |  __weakref__
 |      list of weak references to the object (if defined)
 |
 |  ----------------------------------------------------------------------
 |  Data and other attributes defined here:
 |
 |  __annotations__ = {'x': <class 'float'>, 'y': <class 'float'>, 'z': <c...
 |
 |  __dataclass_fields__ = {'x': Field(name='x',type=<class 'float'>,defau...
 |
 |  __dataclass_params__ = _DataclassParams(init=True,repr=True,eq=True,or...
 |
 |  __hash__ = None
```

## Using default values

You can provide default values to the fields in the following way:

```python
from dataclasses import dataclass


@dataclass
class CartesianPoint:
    x: float = 0
    y: float = 0
    z: float = 0
```

## Using arbitrary field type

If you don't want to specify your field type during type hinting, you can use `Any` type
from python's `typing` module.

```python
from dataclasses import dataclass
from typing import Any


@dataclass
class CartesianPoint:
    x: Any
    y: Any
    z: Any
```

## Instance ordering

You can check if two instances are equal without making any modification to the class.

```python
from dataclasses import dataclass


@dataclass
class CartesianPoint:
    x: float
    y: float
    z: float


point_1 = CartesianPoint(1, 2, 3)
point_2 = CartesianPoint(1, 2, 5)

print(point_1 == point_2)
```

```txt
>>> False
```

However, if you want to compare multiple instances of dataclasses, aka add `__gt__` or
`__lt__` methods to your instances, you have to turn on the `order` flag manually.

```python
from dataclasses import dataclass


@dataclass(order=True)
class CartesianPoint:
    x: float
    y: float
    z: float


# comparing two instances
point_1 = CartesianPoint(10, 12, 13)
point_2 = CartesianPoint(1, 2, 5)

print(point_1 > point_2)
```

```txt
>>> True
```

By default, while comparing instances, all of the fields are used. In our above case, all
the fields `x`, `y`, `z`of `point_1` instance are compared with all the fields of `point_2`
instance. You can customize this using the `field` function.

Suppose you want to acknowledge two instances as equal only when attribute `x` of both of
them are equal. You can emulate this in the following way:

```python
from dataclasses import dataclass, field


@dataclass(order=True)
class CartesianPoint:
    x: float
    y: float = field(compare=False)
    z: float = field(compare=False)


# create intance where only the x attributes are equal
point_1 = CartesianPoint(1, 3, 5)
point_2 = CartesianPoint(1, 4, 6)

# compare the instances
print(point_1 == point_2)
print(point_1 < point_2)
```

```txt
>>> True
>>> False
```

You can see the above code prints out `True` despite the instances have different `y` and
`z` attributes.

## Adding methods

Methods can be added to dataclasses just like normal classes. Let's add another method
called `dist` to our `CartesianPoint` class. This method calculates the distance of a point
from origin.

```python
from dataclasses import dataclass
import math


@dataclass
class CartesianPoint:
    x: float
    y: float
    z: float

    def dist(self):
        return math.sqrt(self.x**2 + self.y**2 + self.z**2)


# create a new instance and use method `abs_val`
point = CartesianPoint(5, 6, 7)
norm = point.abs_val()

print(norm)
```

```txt
>>> 10.488088481701515
```

## Making instances immutable

By default, instances of dataclasses are immutable. If you want to prevent mutating your
instance attributes, you can set `frozen=True` while defining your dataclass.

```python
from dataclasses import dataclass


@dataclass(frozen=True)
class CartesianPoint:
    x: float
    y: float
    z: float
```

If you try to mutate the any of the attributes of the above class, it will raise
`FrozenInstanceError`.

```python
point = CartesianPoint(2, 4, 6)
point.x = 23
```

```txt
---------------------------------------------------------------------------

FrozenInstanceError                       Traceback (most recent call last)

<ipython-input-34-b712968bd0eb> in <module>
        1 point = CartesianPoint(2, 4, 6)
----> 2 point.x = 23


<string> in __setattr__(self, name, value)


FrozenInstanceError: cannot assign to field 'x'
```

## Making instances hashable

You can turn on the `unsafe_hash` parameter of the `dataclass` decorator to make the class
instances hashable. This may come in handy when you want to use your instances as dictionary
keys or want to perform set operation on them. However, if you are using `unsafe_hash` make
sure that your dataclasses do not contain any mutable data structure in it.

```python
from dataclasses import dataclass


@dataclass(unsafe_hash=True)
class CartesianPoint:
    x: float
    y: float
    z: float


# creating instance
point = CartesianPoint(0, 0, 0)

# use the class instances as dictionary keys
print({f"{point}": "origin"})
```

```txt
>>> {'CartesianPoint(x=0, y=0, z=0)': 'origin'}
```

## Converting instances to dicts

The `asdict()` function converts a dataclass instance to a dict of its fields.

```python
from dataclasses import dataclass, asdict

point = CartesianPoint(1, 5, 6)
print(asdict(point))
```

```txt
>>> {'x': 1, 'y': 5, 'z': 6}
```

## Post-init processing

When dataclass generates the `__init__` method, internally it'll call `_post_init__` method.
You can add additional processing in the `__post_init__` method. Here, I've added another
attribute `tup` that returns the cartesian point as a tuple.

```python
from dataclasses import dataclass


@dataclass
class CartesianPoint:
    x: float
    y: float
    z: float

    def __post_init__(self):
        self.tup = (self.x, self.y, self.z)


# checking the tuple
point = CartesianPoint(4, 5, 6)
print(point.tup)
```

```txt
>>> (4, 5, 6)
```

## Refactoring the CartesianPoint class

The feature rich original `CartesianPoint` looks something like this:

```python
import math


class CartesianPoint:
    """Immutable Cartesian point class.
    Although mathematically incorrect,
    for demonstration purpose, all the
    comparisons are done based on
    the first field only."""

    def __init__(self, x, y, z):
        self.x = x
        self.y = y
        self.z = z

    def __repr__(self):
        """Print the instance neatly."""

        return f"CartesianPoint(x = {self.x}, y = {self.y}, z = {self.z})"

    def __eq__(self, other):
        "Checks if equal."

        return self.x == other.x

    def __nq__(self, other):
        """Checks non equality."""

        return self.x != other.x

    def __gt__(self, other):
        """Checks if greater than."""

        return self.x > other.x

    def __ge__(self, other):
        """Checks if greater than or equal."""

        return self.x >= other.x

    def __lt__(self, other):
        """Checks if less than."""

        return self.x < other.x

    def __le__(self, other):
        """Checks if less than or equal."""

        return self.x <= other.x

    def __hash__(self):
        """Make the instances hashable."""
        return hash(self)

    def dist(self):
        """Finds distance of point from origin."""

        return math.sqrt(self.x**2 + self.y**2 + self.z**2)
```

Let's see the class in action:

```python
# create multiple instances of the class
a = CartesianPoint(1, 2, 3)
b = CartesianPoint(1, 3, 3)
c = CartesianPoint(0, 3, 5)
d = CartesianPoint(5, 6, 7)

# checking the __repr__ method
print(a)

# checking the __eq__ method
print(a == b)

# checking the __nq__ method
print(a != c)

# checking the __ge__ method
print(b >= d)

# checking the __lt__ method
print(c < a)

# checking __hash__ and __dist__ method
print({f"{a}": a.dist()})
```

```txt
CartesianPoint(x = 1, y = 2, z = 3)
True
True
False
True
{'CartesianPoint(x = 1, y = 2, z = 3)': 3.7416573867739413}
```

Below is the same class refactored using dataclass.

```python
from dataclasses import dataclass, field


@dataclass(unsafe_hash=True, order=True)
class CartesianPoint:
    """Immutable Cartesian point class.
    Although mathematically incorrect,
    for demonstration purpose, all the
    comparisons are done based on
    the first field only."""

    x: float
    y: float = field(compare=False)
    z: float = field(compare=False)

    def dist(self):
        """Finds distance of point from origin."""

        return math.sqrt(self.x**2 + self.y**2 + self.z**2)
```

Use this class like before.

```python
# create multiple instances of the class
a = CartesianPoint(1, 2, 3)
b = CartesianPoint(1, 3, 3)
c = CartesianPoint(0, 3, 5)
d = CartesianPoint(5, 6, 7)

# checking the __repr__ method
print(a)

# checking the __eq__ method
print(a == b)

# checking the __nq__ method
print(a != c)

# checking the __ge__ method
print(b >= d)

# checking the __lt__ method
print(c < a)

# checking __hash__ and __dist__ method
print({f"{a}": a.dist()})
```

```txt
CartesianPoint(x=1, y=2, z=3)
True
True
False
True
{'CartesianPoint(x=1, y=2, z=3)': 3.7416573867739413}
```

[^1]: [Type annotations](https://docs.python.org/3/library/typing.html)

[^2]:
    [Python dataclasses - official docs](https://docs.python.org/3/library/dataclasses.html)
    [^2]

[^3]:
    [The ultimate guide to dataclasses in python 3.7](https://realpython.com/python-data-classes/)
    [^3]

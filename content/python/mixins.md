---
title: Interfaces, mixins and building powerful custom data structures in Python
date: 2020-07-03
tags:
    - Python
---

Imagine a custom _set-like_ data structure that doesn't perform hashing and trades
performance for tighter memory footprint. Or imagine a _dict-like_ data structure that
automatically stores data in a PostgreSQL or Redis database the moment you initialize it;
also it lets you to _get-set-delete_ key-value pairs using the usual
_retrieval-assignment-deletion_ syntax associated with built-in dictionaries. Custom data
structures can give you the power of choice and writing them will make you understand how
the built-in data structures in Python are constructed.

One way to understand how built-in objects like dictionary, list, set etc work is to build
custom data structures based on them. Python provides several mixin classes in the
`collection.abc` module to design custom data structures that look and act like built-in
structures with additional functionalities baked in.

## Concepts

To understand how all these work, you'll need a fair bit of knowledge about Interfaces,
Abstract Base Classes, Mixin Classes etc. I'll build the concept edifice layer by layer
where you'll learn about interfaces first and how they can be created and used via the
`abc.ABC` class. Then you'll learn how abstract base classes differ from interfaces. After
that I'll introduce mixins and explain how all these concepts can be knitted together to
architect custom data structures with amazing capabilities. Let's dive in.

## Interfaces

Python interfaces can help you write classes based on common structures. They ensure that
classes that provide similar functionalities will also have similar footprints. Interfaces
are not as popular in Python as they are in other statically typed language. The dynamic
nature and duck-typing capabilities of Python often make them redundant.

However, in larger applications, interfaces can make you avoid writing code that is poorly
encapsulated or build classes that look awfully similar but provide completely unrelated
functionalities. Moreover, interfaces implicitly spawn other powerful techniques like mixin
classes which can help you achieve DRY nirvana.

### Overview

At a high level, an interface acts as a blueprint for designing classes. In Python, an
interface is a specialized abstract class that defines one or more abstract methods.
Abstract classes differs from concrete classes in the sense that they aren't intended to
stand on their own and the methods they define shouldn't have any implementation.

Usually, you inherit from an interface and implement the methods defined in the abstract
class in a concrete subclass. Interfaces provide the skeletons and concrete classes provide
the implementation of the methods based on those skeletons. Depending on the ways you can
architect interfaces, they can be segmented into two primary categories.

-   Informal Interfaces
-   Formal Interfaces

## Informal interfaces

Informal interfaces are classes which define methods that can be overridden, but there’s no
strict enforcement.

Let's write an informal interface for a simple calculator class:

```python
class ICalc:
    """Informal Interface: Abstract calculator class."""

    def add(self, a, b):
        raise NotImplementedError

    def sub(self, a, b):
        raise NotImplementedError

    def mul(self, a, b):
        raise NotImplementedError

    def div(self, a, b):
        raise NotImplementedError
```

Notice that the `ICalc` class has four different methods that don't give you any
implementation. It's an informal interface because you can still instantiate the class but
the methods will raise `NotImplementedError` if you try to apply them. You've to subclass
the interface to use it. Let's do it:

```python
class Calc(ICalc):
    """Concrete Class: Calculator"""

    def add(self, a, b):
        return a + b

    def sub(self, a, b):
        return a - b

    def mul(self, a, b):
        return a * b

    def div(self, a, b):
        return a / b


# Using the class
c = Calc()
print(c.add(1, 2))
print(c.sub(2, 3))
print(c.mul(4, 5))
print(c.div(5, 6))
```

```txt
3
-1
20
0.8333333333333334
```

Now, you might be wondering why you even need all of these boilerplate code and inheritance
when you can directly define the concrete `Calc` class and call it a day.

Consider the following scenario where you want to add additional functionalities to each of
the method of the `Calc` class. Here, you've two options. Either you can mutate the original
class and add those extra functionalities to the methods or you can create another class
with similar footprint and implement all the methods with the added functionalities.

The first option isn't always viable and can cause regression in real life scenario. The
second approach ensures modularity and is generally quicker to implement since you won't
have to worry about messing up the original concrete class. However, figuring out which
methods you'll need to implement in the extended class can be hard because the concrete
class might have additional methods that you don't want in the extended class.

In this case, instead of figuring out the methods from the concrete `Calc` class, it's
easier to do so from an established structure defined in the `ICalc` interface. Interfaces
make the process of extending class functionalities more tractable. Let's make another class
that will add logging to all of the methods of the `Calc` class:

```python
import logging

logging.basicConfig(level=logging.INFO)


class CalcLog(ICalc):
    """Concrete Class: Calculator with logging"""

    def add(self, a, b):
        logging.info(f"Operation: Addition, Arguments: {(a, b)}")
        return a + b

    def sub(self, a, b):
        logging.info(f"Operation: Subtraction, Arguments: {(a, b)}")
        return a - b

    def mul(self, a, b):
        logging.info(f"Operation: Multiplication, Arguments: {(a, b)}")
        return a * b

    def div(self, a, b):
        logging.info(f"Operation: Division, Arguments: {(a, b)}")
        return a / b


# Using the class
clog = CalcLog()
print(clog.add(1, 2))
print(clog.sub(2, 3))
print(clog.mul(4, 5))
print(clog.div(5, 6))
```

```txt
INFO:root:Operation: Addition, Arguments: (1, 2)
INFO:root:Operation: Subtraction, Arguments: (2, 3)
INFO:root:Operation: Multiplication, Arguments: (4, 5)
INFO:root:Operation: Division, Arguments: (5, 6)


3
-1
20
0.8333333333333334
```

In the above class, I've defined another class called `CalcLog` that basically extends the
functionalities of the previously defined `Calc` class. Here, I've inherited from the
informal interface `ICalc` and implemented all the methods with additional info logging
capability.

Although writing informal interfaces is trivial, there are multiple issues that plagues
them. The user of the interface class can still instantiate it like a normal class and won't
be able to tell the difference between a it and a concrete class until she tries to use any
of the methods define inside the interface. Only then the methods will throw exceptions.
This can have unintended side effects.

Moreover, informal interfaces won't compel you to implement all the methods in the
subclasses. You can easily get away without implementing a particular method defined in the
interface. It won't complain about the unimplemented methods in the subclasses. However, if
you try to use a method that hasn't been implemented in the subclass, you'll get an error.
This means even if `issubclass(ConcreteSubClass, Interface)` shows `True`, you can't rely on
it since it doesn't give you the guarantee that the `ConcreteSubClass` has implemented all
the methods defined in the `Interface`.

Let's create another class `FakeCalc` an only implement one method defined in the `ICalc`
abstract class:

```python
class FakeCalc(ICalc):
    """Concrete Class: Fake calculator that doesn't implement all the methods
    defined in the interface."""

    def add(self, a, b):
        return a + b


# Using the class
cfake = FakeCalc()
print(cfake.add(1, 2))
print(cfake.sub(2, 3))
```

```txt
3

---------------------------------------------------------------------------

NotImplementedError                       Traceback (most recent call last)

<ipython-input-48-035c519cee55> in <module>
        10 cfake = FakeCalc()
        11 print(cfake.add(1,2))
---> 12 print(cfake.sub(2,3))


<ipython-input-45-255c6a2093b0> in sub(self, a, b)
        6
        7     def sub(self, a, b):
----> 8         raise NotImplementedError
        9
        10     def mul(self, a, b):


NotImplementedError:
```

Despite not implementing all the methods defined in the `ICalc` class, I was still able to
instantiate the `FakeCalc` concrete class. However, when I tried to apply a method `sub`
that wasn't implemented in the concrete class, it gave me an error. Also,
`issubclass(FakeCalc, ICalc)` returns `True` which can mislead you into thinking that all
the methods of the subclass `FakeCalc` are usable. It can cause subtle bugs can be difficult
to detect. Formal interfaces try to overcome these issues.

## Formal interfaces

Formal interfaces do not suffer from the problems that plague informal interfaces. So if you
want to implement an interface that the users can't initiate independently and that forces
them to implement all the methods in the concrete sub classes, formal interface is the way
to go. In Python, the idiomatic way to define formal interfaces is via the `abc` module.
Let's transform the previously mentioned `ICalc` interface into a formal one:

```python
from abc import ABC, abstractmethod


class ICalc(ABC):
    """Formal interface: Abstract calculator class."""

    @abstractmethod
    def add(self, a, b):
        pass

    @abstractmethod
    def sub(self, a, b):
        pass

    @abstractmethod
    def mul(self, a, b):
        pass

    @abstractmethod
    def div(self, a, b):
        pass
```

Here, I've imported `ABC` class and `abstractmethod` decorator from the `abc` module of
Python's standard library. The name `ABC` stands for _Abstract Base Class_. The interface
class needs to inherit from this `ABC`class and all the abstract methods need to be
decorated using the `abstractmethod` decorator. If your knowledge on decorators are fuzzy,
checkout this in-depth article on python decorators[^1].

Although, it seems like `ICalc` has merely inherited from the `ABC` class, under the hood, a
metaclass[^2] `ABCMeta` gets attached to the interface which essentially makes sure that you
can't instantiate this class independently. Let's try to do so and see what happens:

```python
i = ICalc()
```

```txt
---------------------------------------------------------------------------
TypeError                                 Traceback (most recent call last)
<ipython-input-118-a3cb2945d943> in <module>
----> 1 i = ICalc()

TypeError: Can't instantiate abstract class ICalc with abstract methods
add, div, mul, sub
```

The error message clearly states that you can't instantiate the class `ICalc` directly at
all. You've make a subclass of `ICalc` and implement all the abstract methods and only then
you'll be able to make an instance of the subclass. The subclassing and implementation part
is same as before.

```python
class Calc(ICalc):
    """Concrete calculator class"""

    def add(self, a, b):
        return a + b

    def sub(self, a, b):
        return a - b

    def mul(self, a, b):
        return a * b

    def div(self, a, b):
        return a / b


# Using the class
c = Calc()
print(c.add(1, 2))
print(c.sub(2, 3))
print(c.mul(4, 5))
print(c.div(5, 6))
```

In the case of formal interface, failing to implement even one abstract method in the
subclass will raise `TypeError`. So you can never write something the like the `FakeCalc`
with a formal interface. This approach is more explicit and if there is an issue, it fails
early.

### Interfaces vs abstract base classes

You've probably seen the term _Interface_ and _Abstract Base Class_ being used
interchangeably. However, conceptually they're different. Interfaces can be thought of as a
special case of Abstract Base Classes.

It's imperative that all the methods of an interface are abstract methods and the classes
don't store any state (instance variables). However, in case of abstract base classes, the
methods are generally abstract but there can also be methods that provide implementation
(concrete methods) and also, these classes can have instance variables. This generic
abstract base classes can get very interesting and they can be used as _mixins_ but more on
that in the later sections.

Both interfaces and abstract base classes are similar in the sense that they can't stand on
their own, that means these classes aren't meant to be instantiated independently. Pay
attention to the following snippet to understand how interfaces and abstract base classes
differ.

**Interface**

```python
from abc import ABC, abstractmethod


class InterfaceExample(ABC):
    @abstractmethod
    def method_a(self):
        pass

    @abstractmethod
    def method_b(self):
        pass
```

Here, all the methods must have to be abstract.

**Abstract Base Class**

```python
from abc import ABC, abstractmethod


class AbstractBaseClassExample(ABC):
    @abstractmethod
    def method_a(self):
        pass

    @abstractmethod
    def method_b(self):
        pass

    def method_c(self):
        # implement something
        pass
```

Notice how `method_c` in the above class is a concrete method and can have implementation.

The two examples above establish the fact that

> All interfaces are abstract base classes but not all abstract base classes are interfaces.

### A complete example

Before moving on to the next section, let's see another contrived example to get the idea
about the cases where interfaces can come handy. I'll define an interface called
`AutoMobile` and create three concrete classes called `Car`, `Truck` and `Bus` from it. The
interface defines three abstract methods `start`, `accelerate` and `stop` that the concrete
classes will need to implement later.

![mixins][image_1]

```python
from abc import ABC, abstractmethod


class Automobile(ABC):
    """Formal interface: Abstract automobile class."""

    @abstractmethod
    def start(self):
        pass

    @abstractmethod
    def accelerate(self):
        pass

    @abstractmethod
    def stop(self):
        pass


class Car(Automobile):
    """Concrete Class: Car"""

    def start(self):
        return "The car is starting"

    def accelerate(self):
        return "The car is accelerating"

    def stop(self):
        return "The car is stopping"


class Truck(Automobile):
    """Concrete Class: Truck"""

    def start(self):
        return "The truck is starting"

    def accelerate(self):
        return "The truck is accelerating"

    def stop(self):
        return "The truck is stopping"


class Bus(Automobile):
    """Concrete Class: Bus"""

    def start(self):
        return "The bus is starting"

    def accelerate(self):
        return "The bus is accelerating"

    def stop(self):
        return "The bus is stopping"


car = Car()
truck = Truck()
bus = Bus()

print(car.start())
print(car.accelerate())
print(car.stop())
print(truck.start())
print(truck.accelerate())
print(truck.stop())
print(bus.start())
print(bus.accelerate())
print(bus.stop())
```

```txt
The car is starting
The car is accelerating
The car is stopping
The truck is starting
The truck is accelerating
The truck is stopping
The bus is starting
The bus is accelerating
The bus is stopping
```

The above example delineates the use cases for interfaces. When you need to create multiple
similar classes, interfaces can provide a basic foundation for the subclasses to build upon.
In the next section, I'll be using formal interfaces to create Mixin classes. So, before
understanding mixin classes and how they can be used to inject additional plugins to your
classes, it's important that you understand interfaces and abstract base classes properly.

## Mixins

Imagine you're baking chocolate brownies. Now, you can have them without any extra fluff
which is fine or you can top them with cream cheese, caramel sauce, chocolate chips etc.
Usually you don't make the extra toppings yourself, rather you prepare the brownies and use
off the shelf toppings. This also gives you the ability to mix and match different
combinations of toppings to spruce up the flavors quickly. However, making the the toppings
from scratch would be a lengthy process and doing it over an over again can ruin the fun of
baking.

While creating software, there’s sometimes a limit to the depth we should go. When pieces of
what we’d like to achieve have already been executed well by others, it makes a lot of sense
to reuse them. One way to achieve modularity and reusability in object-oriented programming
is through a concept called a mixin. Different languages implement the concept of mixin in
different ways. In Python, mixins are supported via multiple inheritance.

### Overview

In the context of Python especially, a mixin is a parent class that provides functionality
to subclasses but is not intended to be instantiated itself. This should already incite deja
vu in you since **classes that aren't intended to be instantiated** and can have both
concrete and abstract methods are basically **abstract base classes**. Mixins can be
regarded as a specific strain of abstract base classes where they can house both concrete
and abstract methods but don't keep any internal states.

These can help you when -

-   You want to provide a lot of optional features for a class.
-   You want to provide a lot of not-optional features for a class, but you want the
    features in separate classes so that each of them is about one feature (behavior).
-   You want to use one particular feature in many different classes.

Let's see a contrived example. Consider Werkzeug's[^3] request and response system. Werkzeug
is a small library that Flask[^4] depends on. I can make a plain old request object by
saying:

```python
from werkzeug import BaseRequest


class Request(BaseRequest):
    pass
```

If I want to add accept header support, I would make that:

```python
from werkzeug import BaseRequest, AcceptMixin


class Request(AcceptMixin, BaseRequest):
    pass
```

If I wanted to make a request object that supports accept headers, etags, user agent and
authentication support, I could do this:

```python
from werkzeug import (
    BaseRequest,
    AcceptMixin,
    ETagRequestMixin,
    UserAgentMixin,
    AuthenticationMixin,
)


class Request(
    AcceptMixin,
    ETagRequestMixin,
    UserAgentMixin,
    AuthenticationMixin,
    BaseRequest,
):
    pass
```

The above example might cause you to say, "that's just multiple inheritance, not really a
mixin", which is can be true in a special case. Indeed, the differences between plain old
multiple inheritance and mixin based inheritance collapse when the parent class can be
instantiated. Understanding the subtlety in the differences between a mixin class, an
abstract base class, an interface and the scope of multiple inheritance is important, so
I'll explore them in a dedicated section.

### Differences between interfaces, abstract classes and mixins

In order to better understand mixins, it's be useful to compare mixins with abstract classes
and interfaces from a code/implementation perspective:

**Interfaces**

Interfaces can contain abstract methods only, no concrete methods and no internal states
(instance variables).

**Abstract Classes**

Abstract classes can contain abstract methods, concrete methods and internal state.

**Mixins**

Like interfaces, mixins do not contain any internal state. But like abstract classes, they
can contain one or more concrete methods. **_So mixins are basically abstract classes
without any internal states._**

In Python, these are just conventions because all of the above are defined as classes.
However, one trait that is common among _interfaces_, _abstract classes_ and _mixins_ is
that they shouldn't exist on their own, i.e. shouldn't be instantiated independently.

### A complete example

Before diving into the real-life examples and how mixins can be used to construct custom
data structures, let's have a look at a self-contained example of a mixin class at work:

```python
import inspect
from abc import ABC, abstractmethod
from pprint import pprint


class DisplayFactorMult(ABC):
    """Mixin class that reveals factor calculation details."""

    @abstractmethod
    def multiply(self, x):
        pass

    def multiply_show(self, x):
        result = self.multiply(x)
        print(f"Factor: {self.factor}, Argument: {x},  Result: {result}")
        return result


class FactorMult(DisplayFactorMult):
    """Concrete class that uses the DisplayFactorMult mixin."""

    def __init__(self, factor):
        self.factor = factor

    def multiply(self, x):
        return x * self.factor


# Putting the FactorMult class to use
f = FactorMult(10)
f.multiply_show(20)

# Use the inspect.getmembers method to inspect the methods
pprint(inspect.getmembers(f, predicate=inspect.ismethod))
```

```txt
Factor: 10, Argument: 20,  Result: 200
[('__init__',
    <bound method FactorMult.__init__ of
    <__main__.FactorMult object at 0x7f0f0546bf40>>),
    ('multiply', <bound method FactorMult.multiply of
    <__main__.FactorMult object at 0x7f0f0546bf40>>),
    ('multiply_show', <bound method DisplayFactorMult.multiply_show of
    <__main__.FactorMult object at 0x7f0f0546bf40>>)]
```

The `FactorMult` class takes in a number as a factor and the `multiply` method simply
multiplies an argument with the factor. The mixin class `DisplayFactorMult` provides an
additional method `multiply_show` that enhances the `multiply` method of the concrete class.
Method `multiply_show` prints the value of the factor, arguments an the result before
returning the result. Here, `DisplayFactoryMult` is a mixin since it houses an abstract
method `multiply`, a concrete method `multiply_show` and doesn't store any instance
variable.

If you really want to dive deeper into mixins and their real-life use cases, checkout the
codebase of the requests[^5] library. It defines and employs many powerful mixin classes to
bestow superpowers upon different concrete classes.

## Building powerful custom data structures with mixins

You've reached the hall of fame where I'll be building custom data structures using the
mixin classes from the `collections.abc` module.

### Verbose tuple

This is a tuple-like data structure that acts exactly like the built-in tuple but with one
exception. It'll print out the special methods underneath when you perform any operation
with it.

```python
from collections.abc import Sequence


class VerboseTuple(Sequence):
    """Custom class that is exactly like a tuple but does some
    extra magic.

    Sequence:
    -------------------
    Inherits From: Reversible, Collection
    Abstract Methods: __getitem__, __len__
    Mixin Methods: __contains__, __iter__, __reversed__, index,
            and count
    """

    def __init__(self, *args):
        self.args = args

    @classmethod
    def _classname(cls):
        # This method just returns the name of the class
        return cls.__name__

    def __getitem__(self, index):
        print(f"Method: __getitem__, Index: {index}")
        return self.args[index]

    def __len__(self):
        print(f"Method: __len__")
        return len(self.args)

    def __repr__(self):
        return f"{self._classname()}{tuple(self.args)}"


vt = VerboseTuple(1, 3, 4)

print(vt)
print(f"Abstract Methods: {set(Sequence.__abstractmethods__)}")
print(
    f"Mixin Methods: { {k for k, v in Sequence.__dict__.items() if callable(v)} }"
)
```

```txt
VerboseTuple(1, 3, 4)
Abstract Methods: {'__len__', '__getitem__'}
Mixin Methods: {'__iter__', '__contains__', 'index', 'count', '__getitem__',
'__reversed__'}
```

To build the `VerboseTuple` data structure, first, I've inherited the `Sequence` mixin class
from the `collections.abc` module. The docstring mentions all the abstract and mixin methods
provided by the `Sequence` class. To build the new data structure, you'll have to implement
all the abstract methods defined in the `Sequence` class and you'll get all the mixin
methods implemented automatically. Notice that the print statement above also reveals the
abstract and the mixin methods.

In the following snippet I've used some of the functionalities offered by tuple and printed
them in a way that will reveal the special methods when they perform any action.

```python
# check __getitem__
print("\n ==== Checking __getitem__ ====")
print(vt[2])

# check __len__
print("\n ==== Checking __len__ ====")
print(len(vt))

# check __contains__
print("\n ==== Checking __contains__ ====")
print(3 in vt)

# check __len__
print("\n ==== Checking __iter__ ====")
for elem in vt:
    print(elem)

# check reverse
print(f"\n ==== Checking reverse ====")
print(list(reversed(vt)))

# check count
print("\n ==== Checking count ====")
print(vt.count(1))
```

```txt
    ==== Checking __getitem__ ====
Method: __getitem__, Index: 2
4

    ==== Checking __len__ ====
Method: __len__
3

    ==== Checking __contains__ ====
Method: __getitem__, Index: 0
Method: __getitem__, Index: 1
True

    ==== Checking __iter__ ====
Method: __getitem__, Index: 0
1
Method: __getitem__, Index: 1
3
Method: __getitem__, Index: 2
4
Method: __getitem__, Index: 3

    ==== Checking reverse ====
Method: __len__
Method: __getitem__, Index: 2
Method: __getitem__, Index: 1
Method: __getitem__, Index: 0
[4, 3, 1]

    ==== Checking count ====
Method: __getitem__, Index: 0
Method: __getitem__, Index: 1
Method: __getitem__, Index: 2
Method: __getitem__, Index: 3
1
```

The printed statements reveal the corresponding special methods used internally when a
particular tuple operation occurs.

### Verbose list

This is a list-like data structure that acts exactly like the built-in list but with one
exception. Like `VerboseTuple`, it'll also print out the special methods underneath when you
perform any operation on or with it.`

```python
from collections.abc import MutableSequence


class VerboseList(MutableSequence):
    """Custom class that is exactly like a list but does some
    extra magic.

    MutableSequence:
    -----------------
    Inherits From: Sequence
    Abstract Methods: __getitem__, __setitem__, __delitem__,
            __len__, insert
    Mixin Methods: Inherited Sequence methods and append, reverse,
            extend, pop, remove, and __iadd__
    """

    def __init__(self, *args):
        self.args = list(args)

    @classmethod
    def _classname(cls):
        # This method just returns the name of the class
        return cls.__name__

    def __getitem__(self, index):
        print(f"Method: __getitem__, Index: {index}")
        return self.args[index]

    def __setitem__(self, index, value):
        print(f"Method: __setitem__, Index: {index}, Value: {value}")
        self.args[index] = value

    def __delitem__(self, index):
        print(f"Method: __delitem__, Index: {index}")
        del self.args[index]

    def __len__(self):
        print(f"Method: __len__")
        return len(self.args)

    def __repr__(self):
        return f"{self._classname()}{tuple(self.args)}"

    def insert(self, index, value):
        self.args.insert(index, value)


vl = VerboseList(4, 5, 6)
vl2 = VerboseList(7, 8, 9)

print(vl)
print(f"Abstract Methods: {set(MutableSequence.__abstractmethods__)}")
print(
    f"Mixin Methods: {
        {k for k, v in MutableSequence.__dict__.items() if callable(v)}
    }"
)
```

```txt
VerboseList(4, 5, 6)
Abstract Methods: {'__delitem__', '__len__', '__getitem__', 'insert', '__setitem__'}
Mixin Methods: {'__iadd__', '__setitem__', 'pop', 'append', 'extend', '__delitem__',
'reverse', 'insert', 'clear', 'remove'}
```

In the above segment, I've inherited the `MutableSequence` mixin class from the
`collections.abc` module. This ensures that the `VerboseList` object will be mutable. All
the abstract methods mentioned in the docstring have been implemented and the output print
statements reveal the structure of the custom data structure as well as all the abstract and
mixin methods.

In the following snippet, I've used some of the functionalities offered by list and printed
them in a way that will reveal the special methods when they perform any action.

```python
# check __setitem__
print("\n ==== Checking __setitem__ ====")
vl[1] = 44
print(vl)

# check remove (__delitem__)
print("\n ==== Checking remove ====")
vl.remove(6)
print(vl)

# check extend
print("\n ==== Checking extend ====")
vl.extend([0, 0])
print(vl)

# check pop
print("\n ==== Checking pop ====")
vl.pop(-1)
print(vl)

# check __iadd__
print("\n ==== Checking __iadd__")
vl += vl2
print(vl)
```

```txt
    ==== Checking __setitem__ ====
Method: __setitem__, Index: 1, Value: 44
VerboseList(4, 44, 6)

    ==== Checking remove ====
Method: __getitem__, Index: 0
Method: __getitem__, Index: 1
Method: __getitem__, Index: 2
Method: __delitem__, Index: 2
VerboseList(4, 44)

    ==== Checking extend ====
Method: __len__
Method: __len__
VerboseList(4, 44, 0, 0)

    ==== Checking pop ====
Method: __getitem__, Index: -1
Method: __delitem__, Index: -1
VerboseList(4, 44, 0)

    ==== Checking __iadd__
Method: __getitem__, Index: 0
Method: __len__
Method: __getitem__, Index: 1
Method: __len__
Method: __getitem__, Index: 2
Method: __len__
Method: __getitem__, Index: 3
VerboseList(4, 44, 0, 7, 8, 9)
```

### Verbose frozen dict

Here, `VerboseFrozenDict` is an immutable data structure that is similar to the built-in
dictionaries. Like the previous structures, this also reveals the internal special methods
while performing different operations.

```python
from collections.abc import Mapping


class VerboseFrozenDict(Mapping):
    """Custom class that is exactly like an immutable dict but does
    some extra magic.

    Mapping:
    -----------------
    Inherits From: Collection
    Abstract Methods: __getitem__, __iter__, __len__
    Mixin Methods: __contains__, keys, items, values, get, __eq__,
            and __ne__
    """

    def __init__(self, **kwargs):
        self.kwargs = kwargs

    @classmethod
    def _classname(cls):
        # This method just returns the name of the class
        return cls.__name__

    def __getitem__(self, key):
        print(f"Method: __getitem__, Key: {key}")
        return self.kwargs[key]

    def __iter__(self):
        print(f"Method: __iter__")
        return iter(self.kwargs)

    def __len__(self):
        print(f"Method: __len__")
        return len(self.kwargs)

    def __repr__(self):
        return f"{self._classname()}({self.kwargs})"


vf = VerboseFrozenDict(**{"a": "apple"})
vf2 = VerboseFrozenDict(**{"b": "orange", "c": "mango"})

print(vf)
print(f"Abstract Methods: {set(Mapping.__abstractmethods__)}")
print(
    f"Mixin Methods: {
        {k for k, v in Mapping.__dict__.items() if callable(v)}
    }"
)
```

```txt
VerboseFrozenDict({'a': 'apple'})
Abstract Methods: {'__len__', '__getitem__', '__iter__'}
Mixin Methods: {'items', '__contains__', 'values', '__eq__', 'keys', 'get',
'__getitem__'}
```

In the above segment, I've inherited the `Mapping` mixin class from the `collections.abc`
module. This ensures that the output sequence will be immutable. Just like before, all the
abstract methods mentioned in the docstring have been implemented and the output print
statements reveal the structure of the custom data structure, all the abstract and mixin
methods.

Below the printed output will reveal the special methods used internally when the
`VerboseFrozenDict` objects perform any operation.

```python
# check __getitem__
print("\n ==== Checking __getitem__ ====")
print(vf["a"])

# check __iter__
print("\n ==== Checking __iter__ ====")
for elem in vf:
    print(elem)

# check __len__
print("\n ==== Checking __len__ ====")
print(len(vf))

# check __contains__
print("\n ==== Checking __iter__ ====")
print("a" in vf)

# check keys, values
print(f"\n ==== Checking items, keys, values ====")
print(vf.items())
print(vf.keys())
print(vf.values())

# check get
print("\n ==== Checking get ====")
print(vf.get("b", None))

# check eq & nq
print("\n ==== Checking __eq__, __nq__ ====")
print(vf == vf2)
print(vf != vf2)
```

```txt
    ==== Checking __getitem__ ====
Method: __getitem__, Key: a
apple

    ==== Checking __iter__ ====
Method: __iter__
a

    ==== Checking __len__ ====
Method: __len__
1

    ==== Checking __iter__ ====
Method: __getitem__, Key: a
True

    ==== Checking items, keys, values ====
ItemsView(VerboseFrozenDict({'a': 'apple'}))
KeysView(VerboseFrozenDict({'a': 'apple'}))
ValuesView(VerboseFrozenDict({'a': 'apple'}))

    ==== Checking get ====
Method: __getitem__, Key: b
None

    ==== Checking __eq__, __nq__ ====
Method: __iter__
Method: __getitem__, Key: a
Method: __iter__
Method: __getitem__, Key: b
Method: __getitem__, Key: c
False
Method: __iter__
Method: __getitem__, Key: a
Method: __iter__
Method: __getitem__, Key: b
Method: __getitem__, Key: c
True
```

### Verbose dict

The `VerboseDict` data structure is the mutable version of `VerboseFrozedDict`. It supports
all the operations of `VerboseFrozenDict` with some additional features like adding and
deleting key-value pairs, updating values corresponding to different keys etc.

```python
from collections.abc import MutableMapping


class VerboseDict(MutableMapping):
    """Custom class that is exactly like a dict but does some
    extra magic.

    MutableMapping:
    -----------------
    Inherits From: Mapping
    Abstract Methods: __getitem__, __setitem__, __delitem__, __iter__,
                __len__
    Mixin Methods: Inherited Mapping methods and pop, popitem, clear,
                update, and setdefault
    """

    def __init__(self, **kwargs):
        self.kwargs = kwargs

    @classmethod
    def _classname(cls):
        # This method just returns the name of the class
        return cls.__name__

    def __getitem__(self, key):
        print(f"Method: __getitem__, Key: {key}")
        return self.kwargs[key]

    def __setitem__(self, key, value):
        print(f"Method: __setitem__, Key: {key}")
        self.kwargs[key] = value

    def __delitem__(self, key):
        print(f"Method: __delitem__, Key: {key}")
        del self.kwargs[key]

    def __iter__(self):
        print(f"Method: __iter__")
        return iter(self.kwargs)

    def __len__(self):
        print(f"Method: __len__")
        return len(self.kwargs)

    def __repr__(self):
        return f"{self._classname()}({self.kwargs})"


vd = VerboseDict(**{"a": "apple", "b": "ball", "c": "cat"})

print(vd)
print(f"Abstract Methods: {set(MutableMapping.__abstractmethods__)}")
print(
    f"Mixin Methods: {
        {k for k, v in MutableMapping.__dict__.items() if callable(v)}
    }"
)
```

```txt
VerboseDict({'a': 'apple', 'b': 'ball', 'c': 'cat'})
Abstract Methods: {
    '__delitem__', '__len__', '__iter__',
    '__getitem__', '__setitem__'
}
Mixin Methods: {
    '__setitem__', 'pop', 'popitem',
    '__delitem__', 'setdefault', 'update',
    'clear'
}
```

The output statements reveal the structure of the `VeboseDict` class and the abstract and
mixin methods associated with it. The following snippet will print the special methods used
internally by the custom data structure (also in the built-in one) while performing
different operations.

```python
# check __getitem__
print("\n ==== Checking __setitem__ ====")
vd["a"] = "orange"
print(vd)

# check popitem
print("\n ==== Checking popitem ====")
vd.popitem()
print(vd)

# check update
print("\n ==== Checking update ====")
vd.update({"d": "dog"})
print(vd)

# check clear
print("\n ==== Checking clear ====")
vd.clear()
print(vd)

# check setdefault
print(f"\n ==== Checking setdefault ====")
x = vd.setdefault("a", "pepsi")
print(x)
print(vd)
```

```txt
    ==== Checking __setitem__ ====
Method: __setitem__, Key: a
VerboseDict({'a': 'orange', 'b': 'ball', 'c': 'cat'})

    ==== Checking popitem ====
Method: __iter__
Method: __getitem__, Key: a
Method: __delitem__, Key: a
VerboseDict({'b': 'ball', 'c': 'cat'})

    ==== Checking update ====
Method: __setitem__, Key: d
VerboseDict({'b': 'ball', 'c': 'cat', 'd': 'dog'})

    ==== Checking clear ====
Method: __iter__
Method: __getitem__, Key: b
Method: __delitem__, Key: b
Method: __iter__
Method: __getitem__, Key: c
Method: __delitem__, Key: c
Method: __iter__
Method: __getitem__, Key: d
Method: __delitem__, Key: d
Method: __iter__
VerboseDict({})

    ==== Checking setdefault ====
Method: __getitem__, Key: a
Method: __setitem__, Key: a
pepsi
VerboseDict({'a': 'pepsi'})
```

## Going ballistic with custom data structures

This section discusses two advanced data structures that I mentioned at the beginning of the
post.

-   BitSet : Mutable set-like data structure that doesn't perform hashing.
-   SQLAlchemyDict: Mutable dict-like data structure that can store key-value pairs in any
    SQLAlchemy supported relational database.

### BitSet

This mutable set-like data structure doesn't perform hashing to store data. It can store
integers in a fixed range. While storing integers, `BitSet` objects use less memory compared
to built-in sets.

However, since no hashing happens, it's slower to perform addition and retrieval compared to
built-in sets. The following code snippet was taken directly from Raymond Hettinger's 2019
PyCon Russia talk[^6] on advanced data structures.

```python
from collections.abc import MutableSet


class BitSet(MutableSet):
    "Ordered set with compact storage for integers in a fixed range"

    def __init__(self, limit, iterable=()):
        self.limit = limit
        num_bytes = (limit + 7) // 8
        self.data = bytearray(num_bytes)
        self |= iterable

    def _get_location(self, elem):
        if elem < 0 or elem >= self.limit:
            raise ValueError(
                f"{elem!r} must be in range 0 <= elem < {self.limit}"
            )
        return divmod(elem, 8)

    def __contains__(self, elem):
        bytenum, bitnum = self._get_location(elem)
        return bool((self.data[bytenum] >> bitnum) & 1)

    def add(self, elem):
        bytenum, bitnum = self._get_location(elem)
        self.data[bytenum] |= 1 << bitnum

    def discard(self, elem):
        bytenum, bitnum = self._get_location(elem)
        self.data[bytenum] &= ~(1 << bitnum)

    def __iter__(self):
        for elem in range(self.limit):
            if elem in self:
                yield elem

    def __len__(self):
        return sum(1 for elem in self)

    def __repr__(self):
        return (
            f"{type(self).__name__}(limit={self.limit}, iterable={list(self)})"
        )

    def _from_iterable(self, iterable):
        return type(self)(self.limit, iterable)
```

Let's inspect the above data structure to understand exactly how much memory we can save.
I'll digress a little here. Normally, you'd use `sys.getsizeof` to measure the memory
footprint of an object where the function reveals the size in bytes.

But there's a problem. The function `sys.getsizeof` only reveals the size of the target
object, excluding the objects the target objects might be referring to. To understand what I
mean, consider the following situation:

Suppose, you have a nested list that looks like this:

```python
lst = [[1], [2, 3], [[4, 5], 6, 7], 8, 9]
```

When you apply `sys.getsizeof` function on the list, it shows 96 bytes. This means only the
outermost list consumes 96 bytes of memory. Here, `sys.getsizeof` doesn't include the size
of the nested lists.

The same is true for other data structures. In case of nested dictionaries, `sys.getsizeof`
will not include the size of nested data structures. I'll only reveal the size of the
outermost dictionary object. The following snippet will traverse through the reference tree
of a nested object and reveal the _true_ size of it.

```python
from collections.abc import Mapping, Container
from sys import getsizeof


def deep_getsizeof(o: object, ids: None = None) -> int:
    """Find the memory footprint of a Python object.

    This is a recursive function that drills down a Python object graph
    like a dictionary holding nested dictionaries with lists of lists
    and tuples and sets.

    The sys.getsizeof function does a shallow size of only. It counts each
    object inside a container as pointer only regardless of how big it
    really is.

    Params
    ------
     o: object
        The object
     ids: None
        Later an iterable is assigned to store the object ids

     Returns
     --------
     int
        Returns the size of object in bytes
    """

    if ids is None:
        ids = set()

    d = deep_getsizeof
    if id(o) in ids:
        return 0

    r = getsizeof(o)
    ids.add(id(o))

    if isinstance(o, str):
        return r

    if isinstance(o, Mapping):
        return r + sum(d(k, ids) + d(v, ids) for k, v in o.iteritems())

    if isinstance(o, Container):
        return r + sum(d(x, ids) for x in o)

    return r
```

Let's use the `deep_getsizeof` to inspect the size differences between built-in set and
`BitSet` objects.

```python
bs = BitSet(limit=5, iterable=[0, 4])
s = {0, 4}
print(f"Normal Set object: {s}")
print(f"BitSet object: {bs}")
print(f"Size of a normal Set object: {deep_getsizeof(s)} bytes")
print(f"Size of a BitSet object: {deep_getsizeof(bs)} bytes")
```

```txt
Normal Set object: {0, 4}
BitSet object: BitSet(limit=5, iterable=[0, 4])
Size of a normal Set object: 268 bytes
Size of a BitSet object: 100 bytes
```

The output of the print statements reveal that the `BitSet` object uses less than half the
memory compared to its built-in counterpart!

### SQLAlchemyDict

Here goes the second type of custom data structure that I mentioned in the introduction.
It's also a mutable dict-like structure that can automatically store key-value pairs to any
SQLAlchemy supported relational database when initialized.

I was inspired to write this one from the same Raymond Hettinger talk that I mentioned
before. For demonstration purposes, I've chosen `SQLite` databse to store the key value
pairs.

This structure gives you immense power since you can abstract away the entire process of
database communication inside the custom object. You'll perform `get-set-delete` operations
on the object just like you'd do so with built-in dictionary objects and the custom object
will take care of storing and updating the data to the target database.

Before running the code snippet below, you'll need to install SQLAlchemy as an external
dependency.

```python
# sqla_dict.py

from collections.abc import MutableMapping
from contextlib import contextmanager
from operator import itemgetter

from sqlalchemy import create_engine, text
from sqlalchemy.exc import OperationalError
from sqlalchemy.orm import sessionmaker


def create_transaction_session(dburl):
    # an engine, which the Session will use for connection resources
    some_engine = create_engine(dburl)

    # create a configured "Session" class
    Session = sessionmaker(bind=some_engine)

    @contextmanager
    def session_scope():
        """Provide a transactional scope around a series of operations."""

        session = Session()
        try:
            yield session
            session.commit()

        except OperationalError:
            pass

        except Exception:
            session.rollback()
            raise
        finally:
            session.close()

    return session_scope


session_scope = create_transaction_session("sqlite:///foo.db")


class SQLAlechemyDict(MutableMapping):
    def __init__(self, dbname, session_scope, items=None, **kwargs):
        self.dbname = dbname
        self.session_scope = session_scope

        if items is None:
            items = []

        with self.session_scope() as session:
            session.execute("CREATE TABLE Dict (key text, value text)")
            session.execute("CREATE UNIQUE INDEX KIndx ON Dict (key)")

        self.update(items, **kwargs)

    def __setitem__(self, key, value):
        if key in self:
            del self[key]

        with self.session_scope() as session:
            session.execute(
                text("INSERT INTO  Dict VALUES (:key, :value)"),
                {"key": key, "value": value},
            )

    def __getitem__(self, key):
        with self.session_scope() as session:
            r = session.execute(
                text("SELECT value FROM Dict WHERE key=:key"),
                {"key": key},
            )
            row = r.fetchone()

            if row is None:
                raise KeyError(key)
            return row[0]

    def __delitem__(self, key):
        if key not in self:
            raise KeyError(key)

        with self.session_scope() as session:
            session.execute(
                text("DELETE FROM Dict WHERE key=:key"), {"key": key}
            )

    def __len__(self):
        with self.session_scope() as session:
            r = session.execute("SELECT COUNT(*) FROM Dict")
            return next(r)[0]

    def __iter__(self):
        with self.session_scope() as session:
            r = session.execute("SELECT key FROM Dict")
            return map(itemgetter(0), r.fetchall())

    def __repr__(self):
        return f"{type(self).__name__}(dbname={self.dbname!r}, items={list(self.items())})"

    def vacuum(self):
        with self.session_scope() as session:
            session.execute("VACUUM;")


if __name__ == "__main__":
    # test the struct
    sqladict = SQLAlechemyDict(
        dbname="foo.db",
        session_scope=session_scope,
        items={"hello": "world"},
    )
    print(sqladict)
    sqladict["key"] = "val"

    for key in sqladict:
        print(key)

    # >>> SQLAlechemyDict(dbname='foo.db', items=[('hello', 'world'), ('key', 'val')])
    # >>> hello
    # >>> key
```

```txt
SQLAlechemyDict(dbname='foo.db', items=[('hello', 'world'), ('key', 'val')])
hello
key
```

Running the above code snippet will create a SQLite database named `foo.db` in your current
working directory. You can inspect the database with any database viewer and find your
key-value pairs there. Everything else is the same as a built-in dictionary object.

[^1]: [decorators](/python/decorators)
[^2]: [metaclass](/python/metaclasses)
[^3]: [werkzeug](https://werkzeug.palletsprojects.com/en/latest/)
[^4]: [flask](https://flask.palletsprojects.com/)
[^5]:
    [Mixins in the requests library](https://github.com/psf/requests/blob/8149e9fe54c36951290f198e90d83c8a0498289c/requests/models.py#L60)

[^6]:
    [Build powerful, new data structures with Python's abstract base classes - Raymond Hettinger](https://www.youtube.com/watch?v=S_ipdVNSFlo)

[^7]:
    [Implementing an interface in Python - Real Python](https://realpython.com/python-interface/)
    [^7]

[^8]:
    [What is a mixin, and why are they useful? - Stackoverflow](https://stackoverflow.com/questions/533631/what-is-a-mixin-and-why-are-they-useful)
    [^8]

[^9]:
    [Mixins for fun and profit - Dan Hillard](https://easyaspython.com/mixins-for-fun-and-profit-cb9962760556)
    [^9]

[image_1]:
    https://user-images.githubusercontent.com/30027932/86243108-96bbd680-bbc7-11ea-9ddb-9fe46b4a17a1.png

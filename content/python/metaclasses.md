---
title: Deciphering Python's metaclasses
date: 2020-06-26
tags:
    - Python
---

**_Updated on 2023-09-11_**: _Fix broken URLs._

In Python, metaclass is one of the few tools that enables you to inject metaprogramming
capabilities into your code. The term metaprogramming refers to the potential for a program
to manipulate itself in a self referential manner. However, messing with metaclasses is
often considered an arcane art that's beyond the grasp of the plebeians. Heck, even Tim
Peters[^1] advices you to tread carefully while dealing with these.

> Metaclasses are deeper magic than 99% of users should ever worry about. If you wonder
> whether you need them, you don't (the people who actually need them know with certainty
> that they need them, and don't need an explanation about why).

Metaclasses are an esoteric OOP concept, lurking behind virtually all Python code. Every
Python class that you create is attached to a default metaclass and Python cleverly
abstracts away all the meta-magics. So, you're indirectly using them all the time whether
you are aware of it or not. For the most part, you don't need to be aware of it. Most Python
programmers rarely, if ever, have to think about metaclasses. This makes metaclasses
exciting for me and I want to explore them in this post to formulate my own judgement.

## Metaclasses

A metaclass is a class whose instances are classes. Like an "ordinary" class defines the
behavior of the instances of the class, a metaclass defines the behavior of classes and
their instances.

![metaclass][image_1]

Metaclasses aren't supported by every object oriented programming language. Those
programming language, which support metaclasses, considerably vary in way they implement
them. Python provides you a way to get under the hood and define custom metaclasses.

## Understanding type and class

In Python, everything is an object. Classes are objects as well. As a result, all classes
must have corresponding types. You deal with built in types like `int`, `float`, `list` etc
all the time. Consider this example:

```python
a = 5

print(type(a))
print(type(int))
```

```txt
<class 'int'>
<class 'type'>
```

In the above example, variable `a` is an instance of the built in class `int`. Type of `a`
is `int` and the type of `int` is `type`. User defined classes also show similar behavior.
For example:

```python
class Foo:
    pass


a = Foo()

print(type(a))
print(type(Foo))
```

```txt
<class '__main__.Foo'>
<class 'type'>
```

Here, I've defined another class named `Foo` and created an instance `a` of the class.
Applying `type` on instance `a` reveals its type as `__main__.Foo` and applying `type` on
class `Foo` reveals the type as `type`. So here, we can use the term `class` and `type`
interchangeably. This brings up the question:

> What on earth this `type` (function? class?) thing actually is and what is the type of
> `type`?

Let's apply `type` on `type`:

```python
print(type(type))
```

```txt
<class 'type'>
```

Weird. The type of any class (not instance of a class) in Python is `type` and the type of
`type` is also `type`. By now, you've probably guessed that `type` is a very special class
in Python that can reveal the type of itself and of any other class or object. In fact,
`type` is a metaclass and all the classes in Python are instances of it. You can inspect
that easily:

```python
class Foo:
    pass


for klass in [int, float, list, dict, Foo, type]:
    print(type(klass))

print(isinstance(Foo, type))
print(isinstance(type, type))
```

```txt
<class 'type'>
<class 'type'>
<class 'type'>
<class 'type'>
<class 'type'>
<class 'type'>
True
True
```

The the last line of the above code snippet demonstrates that `type` is also an instance of
metaclass `type`. Normally, you can't write self referential classes like that in pure
Python. However, you can circumvent this limitation by subclassing from `type`. This enables
you to write custom metaclasses that you can use to dictate and mutate the way classes are
created and instantiated. From now on, I'll be referring to the instance class of a
metaclass as _target class_. Let's create a custom metaclass that just prints the name of
the target class while creating it:

```python
class PrintMeta(type):
    def __new__(metacls, cls, bases, classdict):
        """__new__ gets executed before the target is created.

        Parameters
        ----------
        metacls : PrintMeta
            Instance of the the PrintMeta class itself
        cls : str
            Name of the class being defined (Point in this example)
        bases : tuple
            Base classes of the constructed class, empty tuple in this case
        classdict : dict
            Dict containing methods and fields defined in the class

        Returns
        -------
            instance class of this metaclass
        """

        print(f"Name of this class is: {cls}")
        return super().__new__(metacls, cls, bases, classdict)


class A(metaclass=PrintMeta):
    pass
```

```txt
Name of this class is A
```

Despite the fact that we haven't called class `A` or created an instance of it, the
`__new__` method of metaclass `PrintMeta` was executed and printed the name of the target
class. In the return statement of `__new__` method, `super()` was used to call the `__new__`
method of the base class (`type`) of the metaclass `PrintMeta`.

## Special methods used by metaclasses

Type `type`, as the default metaclass in Python, defines a few special methods that new
metaclasses can override to implement unique code generation behavior. Here is a brief
overview of these "magic" methods that exist on a metaclass:

-   `__new__`: This method is called on the Metaclass before an instance of a class based on
    the metaclass is created
-   `__init__`: This method is called to set up values after the instance/object is created
-   `__prepare__`: Defines the class namespace in a mapping that stores the attributes
-   `__call__`: This method is called when the constructor of the new class is to be used to
    create an object

These are the methods to override in your custom metaclass to give your classes behaviors
different from that of `type`. The following example shows the default behaviors of these
special methods and their execution order.

> Some people immediately think of `__init__`, and I've occasionally called it “the
> constructor” myself; but in actuality, as its name indicates, it's an initializer and by
> the time it's invoked, the object has already been created, seeing as it's passed in as
> self. The real constructor is a far less famous function: `__new__`. The reason you might
> never hear about it or use it—is that allocation doesn't mean that much in Python, which
> manages memory for you. So if you do override `__new__`, it'd be just like your
> `__init__`—except you'll have to call into Python to actually create the object, and then
> return that object afterward.

```python
class ExampleMeta(type):
    """Simple metaclass showing the execution flow of the
    special methods."""

    @classmethod
    def __prepare__(metacls, cls, bases):
        """Defines the class namespace in a mapping that stores
        the attributes

        Parameters
        ----------
        cls : str
            Name of the class being defined (Point in this example)
        bases : tuple
            Base classes for constructed class, empty tuple in this case
        """

        print(f"Calling __prepare__ method of {super()}!")
        return super().__prepare__(cls, bases)

    def __new__(metacls, cls, bases, classdict):
        """__new__ is a classmethod, even without @classmethod decorator

        Parameters
        ----------
        cls : str
            Name of the class being defined (Point in this example)
        bases : tuple
            Base classes for constructed class, empty tuple in this case
        classdict : dict
            Dict containing methods and fields defined in the class
        """

        print(f"Calling __new__ method of {super()}!")
        return super().__new__(metacls, cls, bases, classdict)

    def __init__(self, cls, bases, classdict):
        """This method is called to set up values after the
        instance/object is created."""

        print(f"Calling __init__ method of {super()}!")
        super().__init__(cls, bases, classdict)

    def __call__(self, *args, **kwargs):
        """This method is called when the constructor of the new class
        is to be used to create an object

        Parameters
        ----------
        args : tuple
            Position only arguments of the new class
        kwargs : dict
            Keyward only arguments of the new class

        """

        print(f"Calling __call__ method of {super()}!")
        print(f"Printing {self} args:", args)
        print(f"Printing {self} kwargs", kwargs)
        return super().__call__(*args, **kwargs)


class A(metaclass=ExampleMeta):
    def __init__(self, x, y):
        self.x = x
        self.y = y
        print(f"Calling __init__ method of {self}")


a = A(1, 3)
```

```txt
Calling __prepare__ method of <super: <class 'ExampleMeta'>, <ExampleMeta object>>!
Calling __new__ method of <super: <class 'ExampleMeta'>, <ExampleMeta object>>!
Calling __init__ method of <super: <class 'ExampleMeta'>, <ExampleMeta object>>!
Calling __call__ method of <super: <class 'ExampleMeta'>, <ExampleMeta object>>!
Printing <class '__main__.A'> args: (1, 3)
Printing <class '__main__.A'> kwargs {}
Calling __init__ method of <__main__.A object at 0x7febe710a130>
```

Pay attention to the execution order of the special methods of the custom metaclass
`ExampleMeta`. The `__prepare__` method is called first and is followed by `__new__`,
`__init__` and `__call__` respectively. Only after that the first method `__init__` of the
target class `A` is called. This is important since it'll help you to decide how you'll
mutate and change the behavior of your target class.

## Metaclass conflicts

Note that the metaclass argument is singular – you can't attach more than one metaclass to a
class. However, through multiple inheritance you can accidentally end up with more than one
metaclass, and this produces a conflict which must be resolved.

```python
class FirstMeta(type):
    pass


class SecondMeta(type):
    pass


class First(metaclass=FirstMeta):
    pass


class Second(metaclass=SecondMeta):
    pass


class Third(First, Second):
    pass


third = Third()
```

```txt
---------------------------------------------------------------------------

TypeError                                 Traceback (most recent call last)

<ipython-input-340-6afe6bc8f8bc> in <module>
        11     pass
        12
---> 13 class Third(First, Second):
        14     pass
        15


TypeError: metaclass conflict: the metaclass of a derived class must be a
(non-strict) subclass of the metaclasses of all its bases
```

Class `First` and `Second` are attached to different metaclasses and class `Third` inherits
from both of them. Since metaclasses trickle down to subclasses, class `Third` is now
effective attached to two metaclasses (`FirstMeta` and `SecondMeta`). This will raise
`TypeError`. Attachment with only one metaclass is allowed here.

## Examples in the wild

In this section, I'll go through a few real life examples where metaclasses can provide
viable solutions to several tricky problems that you might encounter during software
development. The solutions might appear over-engineered in some cases and almost all of them
can be solved without using metaclasses. However, the purpose is to peek into the inner
wirings of metaclasses and see how they can offer alternative solutions.

### Simple logging with metaclasses

The goal here is to log a few basic information about a class without directly adding any
logging statements to it. Instead, you can whip up a custom metaclass to perform some
metaprogramming and add those statements to the target class without mutating it explicitly.

```python
import logging

logging.basicConfig(level=logging.INFO)


class LittleMeta(type):
    def __new__(metacls, cls, bases, classdict):
        logging.info(f"classname: {cls}")
        logging.info(f"baseclasses: {bases}")
        logging.info(f"classdict: {classdict}")

        return super().__new__(metacls, cls, bases, classdict)


class Point(metaclass=LittleMeta):
    def __init__(self, x: float, y: float) -> None:
        self.x = x
        self.y = y

    def __repr__(self) -> str:
        return f"Point({self.x}, {self.y})"


p = Point(5, 10)
print(p)
```

```txt
INFO:root:classname: Point
INFO:root:baseclasses: ()
INFO:root:attrs: {'__module__': '__main__', '__qualname__': 'Point', '__init__':
<function Point.__init__ at 0x7f436c2db790>, '__repr__': <function Point.__repr__ at
0x7f436c2db4c0>}


Point(5, 10)
```

In the above example, I've created a metaclass called `LittleMeta` and added the necessary
logging statements to record the information about the target class. Since the logging
statements are residing in the `__new__` method of the metaclass, these informations will be
logged before the creation of the target class. In the target class `Point`, `LittleMeta`
replaces the default `type` metaclass and produces the desired result by mutating the class.

### Returning class attributes in a custom list

In this case, I want to dynamically attach a new attribute to the target class called
`__attrs_ordered__`. Accessing this attribute from the target class (or instance) will give
out an alphabetically sorted list of the attribute names. Here, the `__prepare__` method
inside the metaclass `AttrsListMeta` returns an `OrderDict` instead of a simple `dict`—so
all attributes gathered before the `__new__` method call will be ordered. Just like the
previous example, here, the `__new__` method inside the metaclass implements the logic
required to get the sorted list of the attribute names.

```python
from collections import OrderedDict


class AttrsListMeta(type):
    @classmethod
    def __prepare__(metacls, cls, bases):
        return OrderedDict()

    def __new__(metacls, cls, bases, classdict, **kwargs):
        attrs_names = [k for k in classdict.keys()]
        attrs_names_ordered = sorted(attrs_names)
        classdict["__attrs_ordered__"] = attrs_names_ordered

        return super().__new__(metacls, cls, bases, classdict, **kwargs)


class A(metaclass=AttrsListMeta):
    def __init__(self, x, y):
        self.y = y
        self.x = x


a = A(1, 2)
print(a.__attrs_ordered__)
```

```txt
['__init__', '__module__', '__qualname__']
```

You can access the `__attrs_ordered__` attribute from both class `A` and an instance of
class `A`. Try removing the `sorted()` function inside the `__new__` method of the metaclass
and see what happens!

### Creating a singleton class

In OOP term, a singleton class is a class that can have only one object (an instance of the
class) at a time.

After the first time, if you try to instantiate a Singleton class, it will basically return
the same instance of the class that was created before. So any modifications done to this
apparently new instance will mutate the original instance since they're basically the same
instance.

```python
class Singleton(type):
    _instance = {}

    def __call__(cls, *args, **kwargs):
        if cls not in cls._instance:
            cls._instance[cls] = super().__call__(*args, **kwargs)
        return cls._instance[cls]


class A(metaclass=Singleton):
    pass


a = A()
b = A()

a is b
```

```txt
True
```

In the above example, at first, I've created a singleton class `A` by attaching the
`Singleton` metaclass to it. Secondly, I've instantiated class `A` and assigned the instance
of the class to a variable `a`. Thirdly, I've instantiated the class again and assigned
variable a `b` to this seemingly new instance. Checking the identity of the two variables
`a` and `b` reveals that both of them are actually the same object.

### Implementing a class that can't be subclassed

Suppose you want to create a base class where the users of your class won't be able to
create any subclasses from the base class. In that case, you can write a metaclass and
attach that your base class. The base class will raise `RuntimeError` if someone tries to
create a subclass from it.

```python
class TerminateMeta(type):
    def __new__(metacls, cls, bases, classdict):
        type_list = [type(base) for base in bases]

        for typ in type_list:
            if typ is metacls:
                raise RuntimeError(
                    f"Subclassing a class that has "
                    + f"{metacls.__name__} metaclass is prohibited"
                )
        return super().__new__(metacls, cls, bases, classdict)


class A(metaclass=TerminateMeta):
    pass


class B(A):
    pass


a = A()
```

```txt
---------------------------------------------------------------------------

RuntimeError                              Traceback (most recent call last)

<ipython-input-438-ccba42f1f95b> in <module>
        20
        21
---> 22 class B(A):
        23     pass
        24

...

RuntimeError: Subclassing a class that has TerminateMeta metaclass is prohibited
```

### Disallowing multiple inheritance

Multiple inheritance can be fragile and error prone. So, if you don't want to allow the
users to use a base class with any other base classes to form multiple inheritance, you can
do so by attaching a metaclass to that target base class.

```python
class NoMultiMeta(type):
    def __new__(metacls, cls, bases, classdict):
        if len(bases) > 1:
            raise TypeError("Inherited multiple base classes!")
        return super().__new__(metacls, cls, bases, classdict)


class Base(metaclass=NoMultiMeta):
    pass


# no error is raised
class A(Base):
    pass


# no error is raised
class B(Base):
    pass


# This will raise an error!
class C(A, B):
    pass
```

```txt
---------------------------------------------------------------------------

TypeError                                 Traceback (most recent call last)

<ipython-input-404-36c323db1ea0> in <module>
        18
        19 # This will raise an error!
---> 20 class C(A, B):
        21     pass

...

TypeError: Inherited multiple base classes!
```

### Timing classes with metaclasses

Suppose you want to measure the execution time of different methods of a class. One way of
doing that is to define a timer decorator and decorating all the methods to measure and show
the execution time. However, by using a metaclass, you can avoid decorating the methods in
the class individually and the metaclass will dynamically apply the timer decorator to all
of the methods of your target class. This can reduce code repetition and improve code
readability.

```python
import inspect
import time
from functools import wraps


def timefunc(func):
    @wraps(func)
    def wrapper(*args, **kwargs):
        start_time = time.time()
        ret = func(*args, **kwargs)
        end_time = time.time()
        run_time = end_time - start_time
        print(f"Executing {func.__qualname__} took {run_time} seconds.")
        return ret

    return wrapper


class TimerMeta(type):
    def __new__(metacls, cls, bases, classdict):
        new_cls = super().__new__(metacls, cls, bases, classdict)

        # key is attribute name and val is attribute value in attribute dict
        for key, val in classdict.items():
            if inspect.isfunction(val) or inspect.ismethoddescriptor(val):
                setattr(new_cls, key, timefunc(val))
        return new_cls


class Shouter(metaclass=TimerMeta):
    def __init__(self):
        pass

    def intro(self):
        time.sleep(1)
        print("I shout!")


s = Shouter()
s.intro()
```

```txt
Executing Shouter.__init__ took 9.5367431640625e-07 seconds.
I shout!
Executing Shouter.intro took 1.0011515617370605 seconds.
```

### Registering plugins with metaclasses

Suppose a specific single class represents a plugin in your code. You can write a metaclass
to keep track of all of the plugins so than you don't have to count them manually.

```python
registry = {}


class RegisterMeta(type):
    def __new__(metacls, cls, bases, classdict):
        new_cls = super().__new__(metacls, cls, bases, classdict)
        registry[new_cls.__name__] = new_cls
        return new_cls


class A(metaclass=RegisterMeta):
    pass


class B(A):
    pass


class C(A):
    pass


class D(B):
    pass


b = B()
print(registry)
```

```txt
{'A': __main__.A, 'B': __main__.B, 'C': __main__.C, 'D': __main__.D}
```

### Debugging methods with metaclasses

Debugging a class often involves inspecting the individual methods and adding extra
debugging logic to those. However, this can get tedious if you've do this over an over
again. Instead, you can write an inspection decorator and use a metaclass to dynamically
apply the decorator to all of the methods of your target class. Later on, you can simply
detach the metaclass once you're done with debugging and don't want the extra logic in your
target class.

```python
import inspect
from functools import wraps


def debug(func):
    """Decorator for debugging passed function."""

    @wraps(func)
    def wrapper(*args, **kwargs):
        print("Full name of this method:", func.__qualname__)
        return func(*args, **kwargs)

    return wrapper


class DebugMeta(type):
    def __new__(metacls, cls, bases, classdict):
        new_cls = super().__new__(metacls, cls, bases, classdict)

        # key is attribute name and val is attribute value in the attrs dict
        for key, val in classdict.items():
            if inspect.isfunction(val) or inspect.ismethoddescriptor(val):
                setattr(new_cls, key, debug(val))
        return new_cls


class Base(metaclass=DebugMeta):
    pass


class Calc(Base):
    def add(self, x, y):
        return x + y


class CalcAdv(Calc):
    def mul(self, x, y):
        return x * y


mycal = CalcAdv()
print(mycal.mul(2, 3))
```

```txt
Full name of this method: CalcAdv.mul
6
```

### Exception handling with metaclasses

Sometimes you need to handle exceptions in multiple methods of a class in a generic manner.
That means all the methods of the class have the same exception handling, logging logic etc.
Metaclasses can help you avoid adding repetitive exception handling and logging logics to
your methods.

```python
import inspect
from functools import wraps


def exc_handler(func):
    """Decorator for custom exception handling."""

    @wraps(func)
    def wrapper(*args, **kwargs):
        try:
            ret = func(*args, **kwargs)
        except Exception:
            print("Exception Occured!")
            print(f"Method name: {func.__qualname__}")
            print(f"Args: {args}, Kwargs: {kwargs}")
            raise
        return ret

    return wrapper


class ExceptionMeta(type):
    def __new__(metacls, cls, bases, classdict):
        new_cls = super().__new__(metacls, cls, bases, classdict)

        # key is attribute name and val is attribute value in attribute dict
        for key, val in classdict.items():
            if inspect.isfunction(val) or inspect.ismethoddescriptor(val):
                setattr(new_cls, key, exc_handler(val))
        return new_cls


class Base(metaclass=ExceptionMeta):
    pass


class Calc(Base):
    def add(self, x, y):
        return x + y


class CalcAdv(Calc):
    def div(self, x, y):
        return x / y


mycal = CalcAdv()
print(mycal.div(2, 0))
```

```txt
Exception Occured!
Method name: CalcAdv.div
Args: (<__main__.CalcAdv object at 0x7febe692d1c0>, 2, 0), Kwargs: {}


---------------------------------------------------------------------------

ZeroDivisionError                         Traceback (most recent call last)

<ipython-input-467-accaebe919a8> in <module>
        43
        44 mycal = CalcAdv()
---> 45 print(mycal.div(2, 0))

...

ZeroDivisionError: division by zero
```

### Abstract base classes

An abstract class can be regarded as a blueprint for other classes. It allows you to provide
a set of methods that must be implemented within any child classes built from the abstract
class. Abstract classes usually house multiple abstract methods. An abstract method is a
method that has a declaration but does not have an implementation.

When you want to provide a common interface for different implementations of a component,
abstract classes are the way to go. You can't directly initialize or use an abstract class.
Rather, you've to subclass the abstract base class and provide concrete implementations of
all the abstract methods. Python has a dedicated `abc` module to help you create abstract
classes. Let's see how you can define a simple abstract class that provides four abstract
methods:

```python
from abc import ABC, abstractmethod


class ICalc(ABC):
    """Interface for a simple calculator."""

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


intrf = ICalc()
```

```txt
---------------------------------------------------------------------------

TypeError                                 Traceback (most recent call last)

<ipython-input-21-7be58e3a2a92> in <module>
        21
        22
---> 23 intrf = ICalc()


TypeError: Can't instantiate abstract class ICalc with abstract methods
add, div, mul, sub
```

Although it seems like interface `ICalc` is simply inheriting from the class `ABC`, in fact,
`ABC` is attaching a metaclass `ABCMeta` to `ICalc`. This metaclass transforms the `ICalc`
class into an abstract class. You can see that the class `ICalc` gives `TypeError` when you
take an attempt to initialize it. The only way to use this interface is via creating
subclasses from `ICalc` base class and implementing all the abstract methods there. The
snippet below shows that:

```python
class Calc(ICalc):
    """Concrete class that uses Icalc interface."""

    def add(self, a, b):
        return a + b

    def sub(self, a, b):
        return a - b

    def mul(self, a, b):
        return a * b

    def div(self, a, b):
        return a / b


calc = Calc()

print(calc.add(1, 2))
print(calc.sub(2, 3))
print(calc.mul(3, 4))
print(calc.div(4, 5))
```

```txt
3
-1
12
0.8
```

### Metaclasses & dataclasses

Data classes were introduced to python in version 3.7. Basically they can be regarded as
code generators that reduce the amount of boilerplate you need to write while generating
generic classes. Dataclasses automatically create `__init__`, `__repr__`, `__eq__`,
`__gt__`, `__lt__` etc methods without you having to add them explicitly. This can be very
handy when you need to create custom collections for your data. You can create dataclasses
in the following manner:

#### Creating multiple dataclasses

```python
from dataclasses import dataclass
from datetime import datetime


@dataclass(unsafe_hash=True, frozen=True)
class Event:
    created_at: datetime


@dataclass(unsafe_hash=True, frozen=True)
class InvoiceIssued(Event):
    invoice_uuid: int
    customer_uuid: int
    total_amount: float
    due_date: datetime


@dataclass(unsafe_hash=True, frozen=True)
class InvoiceOverdue(Event):
    invoice_uuid: int
    customer_uuid: int


inv = InvoiceIssued(
    **{
        "invoice_uuid": 22,
        "customer_uuid": 34,
        "total_amount": 100.0,
        "due_date": datetime(2020, 6, 19),
        "created_at": datetime.now(),
    }
)


print(inv)
```

```txt
InvoiceIssued(
    created_at=datetime.datetime(2020, 6, 20, 1, 3, 24, 967633),
    invoice_uuid=22,
    customer_uuid=34,
    total_amount=100.0,
    due_date=datetime.datetime(2020, 6, 19, 0, 0)
)
```

#### Avoiding dataclass decorator with metaclasses

Now, one thing that I find cumbersome while creating multiple dataclasses is having to
attach the `@dataclasses.dataclass` decorator to each of the dataclasses. Also, the
decorator takes multiple arguments to customize the dataclass behavior and it can quickly
get cumbersome when you've to create multiple dataclasses with custom behavior. Moreover,
this goes against the DRY (Don't Repeat Yourself) principle in software engineering.

To avoid this, you can write a metaclass that will automatically apply the customized
dataclass decorator to all of the target classes implicitly. All you have to do is to attach
the metaclass to a base dataclass and inherit from it in the later dataclasses that need to
be created.

```python
from dataclasses import dataclass
from datetime import datetime


class EventMeta(type):
    def __new__(metacls, cls, bases, classdict):
        """__new__ is a classmethod, even without @classmethod decorator

        Parameters
        ----------
        cls : str
            Name of the class being defined (Event in this example)
        bases : tuple
            Base classes of the constructed class, empty tuple in this case
        attrs : dict
            Dict containing methods and fields defined in the class
        """
        new_cls = super().__new__(metacls, cls, bases, classdict)

        return dataclass(unsafe_hash=True, frozen=True)(new_cls)


class Event(metaclass=EventMeta):
    created_at: datetime


class InvoiceIssued(Event):
    invoice_uuid: int
    customer_uuid: int
    total_amount: float
    due_date: datetime


class InvoiceOverdue(Event):
    invoice_uuid: int
    customer_uuid: int


inv = InvoiceIssued(
    **{
        "invoice_uuid": 22,
        "customer_uuid": 34,
        "total_amount": 100.0,
        "due_date": datetime(2020, 6, 19),
        "created_at": datetime.now(),
    }
)

print(inv)
```

```txt
InvoiceIssued(
    created_at=datetime.datetime(2020, 6, 24, 12, 57, 22, 543328),
    invoice_uuid=22,
    customer_uuid=34,
    total_amount=100.0,
    due_date=datetime.datetime(2020, 6, 19, 0, 0)
)
```

## Should you use it?

Almost all of the problems you've encountered above can be solved without using metaclasses.
Decorators can also be exclusively used to perform metaprogramming in a more manageable and
subjectively cleaner way. One case where you absolutely have to use metaclasses is to avoid
applying decorators to multiple classes or methods repetitively.

Also, metaclasses can easily veer into the realm of being a _"solution in search of a
problem"._ If the problem at hand can be solved in a simpler way, it probably should be.
However, I still think that you should at least try to understand how metaclasses work to
have a better grasp on how Python classes work in general and can recognize when a metaclass
really is the appropriate tool to use.

## Remarks

Wrapping your mind around metaclasses can be tricky. So, to avoid any unnecessary confusion,
I've entirely evaded any discussion regarding the behavioral difference between _old style
classes_ and _new style classes_ in Python. Also, I've intentionally excluded mentioning the
differences between `type` in Python 2 and `type` in Python 3 entirely. Python 2.x has
reached its EOL. Save yourself some trouble and switch to Python 3.x if you already haven't
done so.

This article assumes familiarity with decorators, dataclasses etc. If your knowledge on them
is rusty, checkout these posts on decorators[^2] and dataclasses[^3].

[^1]: [Tim Peters](<https://en.wikipedia.org/wiki/Tim_Peters_(software_engineer)>)

[^2]: [decorators](/python/decorators)

[^3]: [dataclasses](/python/dataclasses)

[^4]:
    [Understanding Python's metaclasses](https://blog.ionelmc.ro/2015/02/09/understanding-python-metaclasses/)
    [^4]

[^5]:
    [What are metaclasses in Python - Stackoverflow](https://stackoverflow.com/questions/100003/what-are-metaclasses-in-python)
    [^5]

[^6]: [Python metaclasses - Real Python](https://realpython.com/python-metaclasses/) [^6]

[^7]:
    [Metaclasses - Python course EU](https://www.python-course.eu/python3_metaclasses.php)
    [^7]

[^8]:
    [When to use metaclasses in Python](https://breadcrumbscollector.tech/when-to-use-metaclasses-in-python-5-interesting-use-cases/)
    [^8]

[^9]:
    [A primer on Python metaclasses](https://jakevdp.github.io/blog/2012/12/01/a-primer-on-python-metaclasses/)
    [^9]

[image_1]: https://blob.rednafi.com/static/images/metaclasses/img_1.jpeg

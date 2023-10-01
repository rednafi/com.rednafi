---
title: Generic functions with Python's singledispatch
date: 2020-04-05
tags:
    - Python
---

***Updated on 2022-02-13***: *Change import style of `functools.singledispatch`.*

Recently, I was refactoring a portion of a Python function that somewhat looked like this:

```python
def process(data):
    if cond0 and cond1:
        # apply func01 on data that satisfies the cond0 & cond1
        return func01(data)
    elif cond2 or cond3:
        # apply func23 on data that satisfies the cond2 & cond3
        return func23(data)
    elif cond4 and cond5:
        # apply func45 on data that satisfies cond4 & cond5
        return func45(data)


def func01(data):
    ...


def func23(data):
    ...


def func45(data):
    ...
```

This pattern gets tedious when the number of conditions and actionable functions start to
grow. I was looking for a functional approach to avoid defining and calling three different
functions that do very similar things. Situations like this is where parametric
polymorphism[^1] comes into play. The idea is, you have to define a single function that'll
be dynamically overloaded with alternative implementations based on the type of the function
arguments.

## Function overloading & generic functions

**Function overloading** is a specific type of polymorphism where multiple functions can
have the same name with different implementations. Calling an overloaded function will run a
specific implementation of that function based on some prior conditions or appropriate
context of the call.

When function overloading happens based on its argument types, the resulting function is
known as **generic function**. Let's see how Python's `singledispatch` decorator can help to
design generic functions and refactor the icky code above.

## Singledispatch

Python fairly recently added partial support for function overloading in *Python 3.4*. They
did this by adding a neat little decorator to the *functools* module called
`singledispatch`.  In *Python 3.8*, there is another decorator for methods called
`singledispatchmethod`. This decorator will transform your regular function into a single
dispatch generic function.

> A generic function is composed of multiple functions implementing the same operation for
> different types. Which implementation should be used during a call is determined by the
> dispatch algorithm. When the implementation is chosen based on the type of a single
> argument, this is known as single dispatch.

As PEP-443[^2] said, singledispatch only happens based on the first argument’s type. Let’s
take a look at an example to see how this works!

### Example-1: Singledispatch with built-in argument type

Let's consider the following code:

```python
# procedural.py


def process(num):
    if isinstance(num, int):
        return process_int(num)
    elif isinstance(num, float):
        return process_float(num)


def process_int(num):
    # processing interger
    return f"Integer {num} has been processed successfully!"


def process_float(num):
    # processing float
    return f"Float {num} has been processed successfully!"


# use the function
print(process(12.0))
print(process(1))
```

Running this code will return:

```python
>>> Float 12.0 has been processed successfully!
>>> Integer 1 has been processed successfully!
```

The above code snippet applies `process_int` or `process_float` functions on the incoming
number based on its type. Now let's see how the same thing can be achieved with
`singledispatch`:

```python
# single_dispatch.py
from functools import singledispatch


@singledispatch
def process(num=None):
    raise NotImplementedError("Implement process function.")


@process.register(int)
def sub_process(num):
    # processing interger
    return f"Integer {num} has been processed successfully!"


@process.register(float)
def sub_process(num):
    # processing float
    return f"Float {num} has been processed successfully!"


# use the function
print(process(12.0))
print(process(1))
```

Running this will return the same result as before.

```txt
>>> Float 12.0 has been processed successfully!
>>> Integer 1 has been processed successfully!
```

### Example-2: Singledispatch with custom argument type

Suppose, you want to dispatch your function based on custom argument type where the type
will be deduced from data. Consider this example:

```python
def process(data: dict):
    if data["genus"] == "Felis" and data["bucket"] == "cat":
        return process_cat(data)
    elif data["genus"] == "Canis" and data["bucket"] == "dog":
        return process_dog(data)


def process_cat(data: dict):
    # processing cat
    return "Cat data has been processed successfully!"


def process_dog(data: dict):
    # processing dog
    return "Dog data has been processed successfully!"


if __name__ == "__main__":
    cat_data = {"genus": "Felis", "species": "catus", "bucket": "cat"}
    dog_data = {"genus": "Canis", "species": "familiaris", "bucket": "dog"}

    # using process
    print(process(cat_data))
    print(process(dog_data))
```

Running this snippet will print out:

```txt
>>> Cat data has been processed successfully!
>>> Dog data has been processed successfully!
```

To refactor this with `singledispatch`, you can create two data types `Cat` and `Dog`. When
you make `Cat` and `Dog` objects from the classes and pass them through the `process`
function, singledispatch will take care of dispatching the appropriate implementation of
that function.

```python
from dataclasses import dataclass
from functools import singledispatch


@dataclass
class Cat:
    genus: str
    species: str


@dataclass
class Dog:
    genus: str
    species: str


@singledispatch
def process(obj=None):
    raise NotImplementedError("Implement process for bucket")


@process.register(Cat)
def sub_process(obj):
    # processing cat
    return "Cat data has been processed successfully!"


@process.register(Dog)
def sub_process(obj):
    # processing dog
    return "Dog data has been processed successfully!"


if __name__ == "__main__":
    cat_obj = Cat(genus="Felis", species="catus")
    dog_obj = Dog(genus="Canis", species="familiaris")

    print(process(cat_obj))
    print(process(dog_obj))
```

Running this will print out the same output as before:

```txt
>>> Cat data has been processed successfully!
>>> Dog data has been processed successfully!
```

## References

1. [Transform a function into a single dispatch generic function]
2. [Function overloading]
3. [Parametric polymorphism]


[^1]: [Parametric polymorphism](https://en.wikipedia.org/wiki/Parametric_polymorphism)
[^2]: [PEP-443](https://peps.python.org/pep-0443/)
[^3]: [Transform a function into a single dispatch generic function](https://docs.python.org/3/library/functools.html#functools.singledispatch) [^3]
[^4]: [Function overloading](https://en.wikipedia.org/wiki/Function_overloading) [^4]

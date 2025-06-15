---
title: Memory leakage in Python descriptors
date: 2023-07-16
tags:
    - Python
    - TIL
---

Unless I'm hand rolling my own ORM-like feature or validation logic, I rarely need to write
custom descriptors in Python. The built-in descriptor magics like `@classmethod`,
`@property`, `@staticmethod`, and vanilla instance methods usually get the job done.
However, every time I need to dig my teeth into descriptors, I reach for this fantastic how
to[^1] guide by Raymond Hettinger. You should definitely set aside the time to read it if
you haven't already. It has helped me immensely to deepen my understanding of how many of
the fundamental language constructs are wired together underneath.

Descriptors are considered fairly advanced Python features and can easily turn into footguns
if used carelessly. Recently, while working on an app with a descriptor-based data
validator, I discovered a subtle but obvious bug that was hemorrhaging memory all across the
app. The app was using a descriptor to validate class variables while simultaneously
tracking instances where validation occurred. This validator was being used all over the
codebase, so it slowly started blowing up memory usage in the background. The problem is
that it was keeping hard references to everything it validated, so none of those objects
could get garbage collected. But the really sneaky thing was how slowly and secretly the
problem happened—the leakage built up bit by bit over time even when people used the
validator in totally innocuous ways.

Here's a simpler example of a validation descriptor that tracks the instances it's applied
to:

```py
class Within:
    # The instances are tracked here
    _seen = {}

    def __init__(self, min, max):
        self.min = min
        self.max = max

    def __set_name__(self, instance, name):
        self.name = name

    def __get__(self, instance, instance_type):
        return instance.__dict__[self.name]

    def __set__(self, instance, value):
        if not self.min <= value <= self.max:
            raise ValueError(
                f"{value} is not within {self.min} and {self.max}"
            )
        instance.__dict__[self.name] = value

        # Track the instances that have been seen.
        # This is the memory leak.
        self._seen[instance] = value
```

The `Within` descriptor validates that the values assigned to instance attributes are within
a specified min and max range. It does this by implementing the `__set__` and `__get__`
dunder methods. When the descriptor is accessed via `instance.attrname`, the `__get__`
method is called which returns the value from the instance's dict. When a value is assigned
via `instance.attrname = value`, the `__set__` method is called which validates the value is
within the min/max bounds before setting it on the instance. A memory leak occurs because
the `_seen` dict keeps a reference to every instance the descriptor has been accessed on.
This prevents the instances from being garbage collected even if there are no other
references to them. You can use the descriptor and observe the memory leakage like this:

```py
import gc


class Exam:
    math = Within(0, 100)
    physics = Within(0, 100)
    chemistry = Within(0, 100)

    def __init__(self, math, physics, chemistry):
        self.math = math
        self.physics = physics
        self.chemistry = chemistry


if __name__ == "__main__":
    exam = Exam(30, 50, 40)
    exam.math = 60

    # Delete the exam instance
    del exam

    # Force garbage collection
    gc.collect()

    # Check the strong reference to the deleted instance
    print(tuple(Within._seen.items()))
```

Here, we're defining an `Exam` class that uses the `Within` descriptor to apply constraints
on the values of the `math`, `physics`, and `chemistry` class variables. Then we initialize
the class instance and mutate the `math` attribute to demonstrate that the validator is
working as expected. The instance of the `Exam` class is saved to the `_seen` dictionary of
the descriptor when the `__set__` method is called. Next, we delete the `Exam` instance and
force garbage collection. However, when you run the snippet, you'll see that it prints the
following:

```txt
((<__main__.Exam object at 0x10466ca10>, 60),)
```

This indicates that although we've deleted the `Exam` instance, it can't be fully garbage
collected since the `Within` descriptor's `_seen` dictionary holds a strong reference to it.

## Dispel the malady

Once I spotted the bug, the solution was fairly simple. Don't keep strong references to the
class instances if you don't need to. Also, use a more robust tool like Pydantic[^2] to
perform validation but I digress here! Using a `weakref.WeakKeyDictionary` instead of a
regular dict for `_seen` would prevent the memory leakage by avoiding strong references to
the deleted instances. Since `WeakKeyDictionary` holds weak references to the keys, if all
other strong references to an instance are deleted, the garbage collector can reclaim it.
The weak reference in `WeakKeyDictionary` won't keep the instance alive. Here's how you'd
modify `Within` to fix the issue:

```py
from weakref import WeakKeyDictionary


class Within:
    _seen = WeakKeyDictionary()  # Drop in dict replacement

    def __init__(self, min, max): ...

    def __set_name__(self, instance, name): ...

    def __get__(self, instance, instance_type): ...

    def __set__(self, instance, value): ...
```

The modified descriptor is a drop-in replacement for the previous one—minus the memory
leakage issue. So in the last snippet, when `exam` is deleted and the gc is called, weakref
allows the instance to be garbage collected correctly instead of remaining in memory due to
the strong reference in `_seen`. The weak reference doesn't interfere with gc freeing up the
memory as desired. If you run the demonstration snippet again, this time you'll see that
once we force the gc to collect the garbage, the `_seen` container gets emptied out.

```py
exam = Exam(30, 50, 40)
exam.math = 60

# Delete the exam instance
del exam

# Force garbage collection
gc.collect()

# Check the strong reference to the deleted instance
print(tuple(Within._seen.items()))
```

This will print an empty tuple:

```txt
()
```

This also means that now `Within` will only keep track of instances that are alive in
memory.

[^1]:
    [Descriptor how to - Raymond Hettinger](https://docs.python.org/3/howto/descriptor.html)

[^2]: [Pydantic](https://docs.pydantic.dev/latest/)

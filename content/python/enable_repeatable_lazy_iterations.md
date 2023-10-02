---
title: Enabling repeatable lazy iterations in Python
date: 2023-07-13
tags:
    - TIL
    - Python
---

The current title of this post is probably incorrect and may even be misleading. I had a
hard time coming up with a suitable name for it. But the idea goes like this: sometimes you
might find yourself in a situation where you need to iterate through a generator more than
once. Sure, you can use an iterable like a tuple or list to allow multiple iterations, but
if the number of elements is large, that'll cause an OOM error. On the other hand, once
you've already consumed a generator, you'll need to restart it if you want to go through it
again. This behavior is common in pretty much every programming language that supports the
generator construct.

So, in the case where a function returns a generator and you've already consumed its values,
you'll need to call the function again to generate a new instance of the generator that you
can use. Observe:

```python
from __future__ import annotations
from collections.abc import Generator


def get_numbers(
    start: int, end: int, step: int
) -> Generator[int, None, None]:
    yield from range(start, end, step)
```

This can be used like this:

```python
numbers = get_numbers(1, 10, 2)

for number in numbers:
    print(number)
```

It'll return:

```txt
1
3
5
7
9
```

Now, if you try to consume the iterable again, you'll get empty value. Run this again:

```python
for number in numbers:
    print(number)
```

It won't print anything since the previous loop has exhausted the generator. This is
expected and if you want to loop through the same elements again, you'll have to call the
function again to produce another generator that you can consume. So, the following will
always work:

```python
for number in get_numbers():
    print(number)
```

If you run this snippet multiple times, on each pass, the `get_numbers()` function will be
called again and that'll return a new generator for you to iterate through. Calling the
generator function like this works but here's another thing that I learned today while
reading Effective Python[^1] by Brett Slatkin. You can create a class with the `__iter__`
method and yield numbers from it just like the function. Then when you initiate the class,
the instance of the class will allow you to loop through it multiple times; each time
creating a new generator.

> _I knew that you could create an iterable class by adding `__iter__` to a class and
> yielding values from it. But I wasn't aware that the you could also iterate through the
> instance of the class multiple times and the class will run `__iter__` on each pass and
> produce a new generator for you to consume._

For example:

```python
from __future__ import annotations
from collections.abc import Generator


class NumberGen:
    def __init__(self, start: int, end: int, step: int) -> None:
        self.start = start
        self.end = end
        self.step = step

    def __iter__(self) -> Generator[int, None, None]:
        yield from range(self.start, self.end, self.step)
```

Now use the class as such:

```python
numbers = NumberGen()
for number in numbers:
    print(number)
```

This prints:

```txt
1
3
5
7
9
```

If you run the for-loop again on the number instance, you'll see that the snippet will print
the same numbers again. Here, instantiating the `NumberGen` class creates a `NumberGen`
instance that is not a generator per se, but can return a generator if you call the `iter()`
function on the instance. When you run the for loop on the instance, it runs the underlying
`__iter__` method to produce a new generator that the loop can iterate through. This allows
you to run the for-loop multiple times on the instance, since each run creates a new
generator that the loop can consume.

> _A generator can still only be consumed once but each time you're running a new for-loop
> on the above instance, the `__iter__` method on it gets called and the method returns a
> new generator for you to iterate through._

This is more convenient than having to repeatedly call a generator function if your API
needs to consume a generator multiple times.

[^1]: [Effective Python - Brett Slatkin](https://effectivepython.com/)

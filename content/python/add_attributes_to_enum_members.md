---
title: Add extra attributes to enum members in Python
date: 2022-02-17
tags:
    - Python
---

While grokking the source code of `http.HTTPStatus` module, I came across this technique to
add extra attributes to the values of enum members. Now, to understand what do I mean by
adding attributes, let's consider the following example:

```py
# src.py
from __future__ import annotations

from enum import Enum


class Color(str, Enum):
    RED = "Red"
    GREEN = "Green"
    BLUE = "Blue"
```

Here, I've inherited from `str` to ensure that the values of the enum members are strings.
This class can be used as follows:

```py
# src.py
...

# Print individual members.
print(f"{Color.RED=}")

# Print name as a string.
print(f"{Color.GREEN.name=}")

# Print value.
print(f"{Color.BLUE.value=}")
```

Running the script will print:

```txt
Color.RED=<Color.RED: 'Red'>
Color.GREEN.name='GREEN'
Color.BLUE.value='Blue'
```

While this works but it's evident that you can only assign a single value to an enum member.
How'd you rewrite this if you needed multiple values attached to a single enum member?

Suppose, in the above case, along with the color title, you also need to save the hex codes
and short descriptions of the colors. One way you can achieve this is via the assignment of
an immutable container as the value of an enum member:

```py
# src.py
from __future__ import annotations

from enum import Enum


class Color(Enum):
    RED = ("Red", "#ff0000", "Ruby Red")
    GREEN = ("Green", "#00ff00", "Guava Green")
    BLUE = ("Blue", "#0000ff", "Baby Blue")
```

Here, I'm using a tuple to contain the title, hex code, and description of the `Color`
members. This gets awkward whenever you'll need to access the individual elements inside the
tuple. You'll have to use hardcoded indexes to access the elements of the tuple. This is how
you'll probably use it:

```py
...

for c in Color:
    print(
        f"title={c.value[0]}, hex_code={c.value[1]}, description={c.value[2]}"
    )
```

It prints:

```txt
title=Red, hex_code=#ff0000, description=Ruby Red
title=Green, hex_code=#00ff00, description=Guava Green
title=Blue, hex_code=#0000ff, description=Baby Blue
```

Hardcoding indexes in such a manner is fragile and will break if you drop a new value in the
middle of the tuple assigned to an enum member. Also, it's hard to reason through logic when
you've to keep the semantic meanings of the index positions in your working memory. A better
thing to do is to rewrite the enum in a way that'll allow you to access different elements
of the member values by their attribute names. Let's do it:

```py
from __future__ import annotations

from enum import Enum


class Color(str, Enum):
    # Declaring the additional attributes here keeps mypy happy.
    hex_code: str
    description: str

    def __new__(
        cls, title: str, hex_code: str = "", description: str = ""
    ) -> Color:
        obj = str.__new__(cls, title)
        obj._value_ = title

        obj.hex_code = hex_code
        obj.description = description
        return obj

    RED = ("Red", "#ff0000", "Ruby Red")
    GREEN = ("Green", "#00ff00", "Guava Green")
    BLUE = ("Blue", "#0000ff", "Baby Blue")
```

Here, I overrode the `__new__` method of the class `Color`. Method `__new__` is a special
class method that you don't need to decorate with the `@classmethod` decorator. It gets
executed during the creation of the `Color` object; before the `__init__` method. Other than
the first argument `cls`, you can define the `__new__` method with any number of arbitrarily
named arguments.

In this case, the value of each member of `Color` will have three elements—`title`,
`hex_code`, and `description`. So, I defined the `__new__` method to accept those arguments.
In the following line, the `str` class was initialized via `obj = str.__new__(cls, title)`
and then `title` was assigned to the newly created string object via `obj._value_=title`.
This line is crucial; without it, the enum won't operate at all. This assignment makes sure
that the `Enum.member.value` will return a string value.

In the next two lines, `hex_code` and `description` were attached to the member values via
the `obj.hex_code=hexcode` and `obj.description=description` statements respectively.

Now, you'll be able to use this enum without any hardcoded shenanigans:

```py
...

# Access the elements of the values of the members by names.
print(f"{Color.RED.value=}")
print(f"{Color.BLUE.hex_code=}")
print(f"{Color.GREEN.description=}")

# Iterate through all the memebers.
for c in Color:
    print(
        f"title={c.value}, hex_code={c.hex_code}, description={c.description}"
    )
```

This will print:

```txt
Color.RED.value='Red'
Color.BLUE.hex_code='#0000ff'
Color.GREEN.description='Guava Green'
title=Red, hex_code=#ff0000, description=Ruby Red
title=Green, hex_code=#00ff00, description=Guava Green
title=Blue, hex_code=#0000ff, description=Baby Blue
```

[^1]:
    [http.HTTPStatus](https://github.com/python/cpython/blob/6f1efd19a70839d480e4b1fcd9fecd3a8725824b/Lib/http/__init__.py#L6)
    [^1]

---
title: Access 'classmethod's like 'property' methods in Python
date: 2021-11-26
tags:
    - Python
---

I wanted to add a helper method to an Enum class. However, I didn't want to make it a
`classmethod` as `property` method made more sense in this particular case. Problem is, you
aren't supposed to initialize an enum class, and `property` methods can only be accessed
from the instances of a class; not from the class itself.

While sifting through Django 3.2's codebase, I found this neat trick to make a `classmethod`
that acts like a `property` method and can be accessed directly from the class without
initializing it.

```python
# src.py
# This requires Python 3.4+.
from enum import Enum, EnumMeta


class PlanetsMeta(EnumMeta):
    @property
    def choices(cls):
        return [(v.name, v.value) for v in cls]


class Planets(Enum, metaclass=PlanetsMeta):
    EARTH = "earth"
    MARS = "mars"


# This can be accessed as follows.
print(Planets.choices)
```

If you run the script, you'll see the following output:

```txt
$ python3.8 src.py
[('EARTH', 'earth'), ('MARS', 'mars')]
```

While the previous example is quite impressive, I still don't like the solution as it
requires creating a metaclass and doing a bunch of magic to achieve something so simple.
Luckily, Python3.9+ makes it possible without any additional magic. Notice the example
below:

```python
# src.py
# Requires Python 3.9+
class ModernPlanets(Enum):
    EARTH = "earth"
    MARS = "mars"

    @classmethod
    @property
    def choices(cls):
        return [(v.name, v.value) for v in cls]


# This can be accessed as follows.
print(ModernPlanets.choices)
```

The only thing that matters here is the order of the `property` and `classmethod` decorator.
Python applies them from bottom to top. Changing the order will make it behave unexpectedly.

## Complete example with tests

```python
# src.py
# Requires Python 3.4+
import sys
import unittest
from enum import Enum, EnumMeta


class PlanetsMeta(EnumMeta):
    @property
    def choices(cls):
        return [(v.name, v.value) for v in cls]


class Planets(Enum, metaclass=PlanetsMeta):
    EARTH = "earth"
    MARS = "mars"


# Requires Python 3.9+
class ModernPlanets(Enum):
    EARTH = "earth"
    MARS = "mars"

    @classmethod
    @property
    def choices(cls):
        return [(v.name, v.value) for v in cls]


class TestPlanets(unittest.TestCase):
    python_version = (sys.version_info.major, sys.version_info.minor)

    def setUp(self):
        self.expected_result = [("EARTH", "earth"), ("MARS", "mars")]

    def test_planets(self):
        self.assertEqual(Planets.choices, self.expected_result)

    @unittest.skipIf(
        python_version < (3, 9),
        "Not supported under Python 3.9",
    )
    def test_modern_planets(self):
        """This test method will fail if we try to run it on a
        version earlier than Python 3.9. So we skip it accordingly."""

        self.assertEqual(ModernPlanets.choices, self.expected_result)


if __name__ == "__main__":
    unittest.main()
```

Running this will print out the following:

```txt
$ python src.py
..
----------------------------------------------------------------------
Ran 2 tests in 0.000s

OK
```

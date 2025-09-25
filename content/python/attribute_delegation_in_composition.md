---
title: Automatic attribute delegation in Python composition
date: 2021-11-28
slug: attribute-delegation-in-composition
aliases:
    - /python/attribute_delegation_in_composition/
tags:
    - Python
    - TIL
---

While trying to avoid inheritance in an API that I was working on, I came across this neat
trick to perform attribute delegation on composed classes. Let's say there's a class called
`Engine` and you want to put an engine instance in a `Car`. In this case, the car has a
classic 'has a' (inheritance usually refers to 'is a' relationships) relationship with the
engine. So, composition makes more sense than inheritance here. Consider this example:

```py
# src.py
from typing import Any


class Engine:
    def __init__(self, name: str, sound: str) -> None:
        self.name = name
        self.sound = sound

    def noise(self) -> str:
        return f"Engine {self.name} goes {self.sound}!"


class Car:
    def __init__(self, engine: Engine, tier: str, price: int) -> None:
        self.engine = engine
        self.tier = tier
        self.price = price

    def info(self) -> dict[str, Any]:
        return {"tier": self.tier, "price": self.price}
```

Ideally, you'd to use the classes as a good citizen as follows:

```py
engine = Engine("w16", "vroom")
car = Car(engine, "supercar", 3_000_000)

# If you want to access an attribute on the 'engine' instance from the 'car'
# instance, you'll do it like this:

print(car.engine.name)
print(car.engine.sound)
```

This will print the following:

```txt
$ python src.py
w16
vroom
```

However, I wanted free attribute access, just like we get in inheritance. We should be able
to do `car.name`, not `car.engine.name`, and get the name of the engine instance. With a
little bit of `__getattr__` magic, it's easy to do so:

```py
# src.py
from typing import Any


class Engine:
    def __init__(self, name: str, sound: str) -> None:
        self.name = name
        self.sound = sound

    def noise(self) -> str:
        return f"Engine {self.name} goes {self.sound}!"


class Car:
    def __init__(self, engine: Engine, tier: str, price: int) -> None:
        self.engine = engine
        self.tier = tier
        self.price = price

    def info(self) -> dict[str, Any]:
        return {"tier": self.tier, "price": self.price}

    # NOTE: This is new!!
    def __getattr__(self, attr: str) -> Any:
        return getattr(self.engine, attr)
```

This snippet is exactly the same as before and the only thing that was added here is the
`__getattr__` method in the `Car` class. Whenever you'll try to access an attribute or a
method on an instance of the `Car` class, the `__getattr__` will intervene. It'll first look
for the attribute in the instance of the `Car` class and if it can't find it there, then
it'll look for the attribute in the instance of the `Engine` class; just like type
inheritance. This will work in case of method access as well. So now you can use the classes
as below:

```py
engine = Engine("w16", "vroom")
car = Car(engine, "supercar", 3_000_000)

print(car.name)  # Actually prints the 'name' of the engine
print(car.sound)  # Prints the 'sound' of the engine
print(car.info())  # Method 'info' is in the 'Car' instance
print(car.noise())  # Method 'noise' is in the 'Engine' instance
```

This will print:

```txt
$ python src.py
w16
vroom
{'tier': 'supercar', 'price': 3000000}
Engine w16 goes vroom!
```

> While this was all fun and dandy, I don't recommend putting it in any serious code as it
> can obfuscate the program's intent and can make obvious things not-so-obvious. Also, in
> case of attributes and methods with the same names in different classes, this can get
> hairy. I just found this gymnastics intellectually stimulating.

## Complete example with tests

```py
# src.py
import unittest
from typing import Any


class Engine:
    def __init__(self, name: str, sound: str) -> None:
        self.name = name
        self.sound = sound

    def noise(self) -> str:
        return f"Engine {self.name} goes {self.sound}!"


class Car:
    def __init__(self, engine: Engine, tier: str, price: int) -> None:
        self.engine = engine
        self.tier = tier
        self.price = price

    def info(self) -> dict[str, Any]:
        return {"tier": self.tier, "price": self.price}

    def __getattr__(self, attr: str) -> Any:
        return getattr(self.engine, attr)


class Test(unittest.TestCase):
    def setUp(self):
        self.engine = Engine("w16", "vroom")
        self.car = Car(self.engine, "supercar", 3_000_000)

    def test_auto_delegation(self):
        expected_info = {"tier": "supercar", "price": 3000000}
        expected_noise = "Engine w16 goes vroom!"

        self.assertEqual(self.car.info(), expected_info)
        self.assertEqual(self.car.noise(), expected_noise)


if __name__ == "__main__":
    unittest.main(Test())
```

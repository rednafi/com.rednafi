---
title: Quicker startup with module-level \_\_getattr\_\_
date: 2024-11-03
tags:
    - Python
    - TIL
---

This morning, someone on Twitter pointed me to PEP 562[^1], which introduces `__getattr__`
and `__dir__` at the module level. While `__dir__` helps control which attributes are
printed when calling `dir(module)`, `__getattr__` is the more interesting addition.

The `__getattr__` method in a module works similarly to how it does in a Python class. For
example:

```python
class Cat:
    def __getattr__(self, name: str) -> str:
        if name == "voice":
            return "meow!!"
        raise AttributeError(f"Attribute {name} does not exist")


# Try to access 'voice' on Cat
cat = Cat()
cat.voice  # Prints "meow!!"

# Raises AttributeError: Attribute something_else does not exist
cat.something_else
```

In this class, `__getattr__` defines what happens when specific attributes are accessed,
allowing you to manage how missing attributes behave. Since Python 3.7, you can also define
`__getattr__` at the module level to handle attribute access on the module itself.

For instance, if you have a module `my_module.py`:

```python
# my_module.py


def existing_function() -> str:
    return "I exist!"


def __getattr__(name: str) -> str:
    if name == "dynamic_attribute":
        return "I was generated dynamically!"
    raise AttributeError(f"Module {__name__} has no attribute {name}")
```

Using this module:

```python
# another_module.py

import my_module

print(my_module.existing_function())  # Prints "I exist!"
print(my_module.dynamic_attribute)  # Prints "I was generated dynamically!"
print(my_module.non_existent)  # Raises AttributeError
```

If an attribute isn't found through the regular lookup (using `object.__getattribute__`),
Python will look for `__getattr__` in the module's `__dict__`. If found, it calls
`__getattr__` with the attribute name and returns the result. But if you're looking up a
name directly as a module global, it bypasses `__getattr__`. This prevents performance
issues that would arise from repeatedly invoking `__getattr__` for built-in or common
attributes.

One practical use for module-level `__getattr__` is lazy-loading heavy dependencies to
improve startup performance. Imagine you have a module that relies on a large library but
don't need it immediately at import.

```python
# heavy_module.py

from typing import Any


def __getattr__(name: str) -> Any:
    if name == "np":
        import numpy as np

        globals()["np"] = np  # Cache it in the module's namespace
        return np
    raise AttributeError(f"Module {__name__} has no attribute {name}")
```

With this setup, importing `heavy_module` doesn't immediately import NumPy. Only when you
access `heavy_module.np` does it trigger the import:

```python
# main.py

import heavy_module

# NumPy hasn't been imported yet.

# Code that doesn't need NumPy...

# Now we need NumPy
arr = heavy_module.np.array([1, 2, 3])
print(arr)  # NumPy is now imported and used
```

The first access to `heavy_module.np` imports NumPy (adding ~150ns), but since we cache `np`
with `globals()['np'] = np`, subsequent accesses are fast, as the module now holds the
reference to NumPy.

This approach is handy in scenarios like CLIs where you want to keep startup quick. For
example, if you need to initialize a database connection but only for specific commands, you
can defer the setup until needed.

Here's an example with SQLite (though SQLite connections are quick, imagine a slower
connection here):

```python
# db_module.py

import sqlite3

# Caching initialized connection in the global namespace
_connection: sqlite3.Connection | None = None


def __getattr__(name: str) -> sqlite3.Connection:
    if name == "connection":
        global _connection
        if _connection is None:
            print("Initializing database connection...")
            _connection = sqlite3.connect("my_database.db")
        return _connection
    raise AttributeError(f"Module {__name__} has no attribute {name}")
```

In this setup, nothing is instantiated when you import `db_module`. The connection is only
initialized on the first access of `db_module.connection`. Later calls use the cached
`_connection`, making subsequent access fast.

Here's how you might use it in a CLI:

```python
# cli.py

import click
import db_module


@click.group()
def cli() -> None:
    pass


@cli.command()
def greet() -> None:
    click.echo("Hello!")


@cli.command()
def show_data() -> None:
    conn = (
        db_module.connection
    )  # Initializes the database connection if needed
    cursor = conn.cursor()
    cursor.execute("SELECT * FROM my_table")
    results = cursor.fetchall()
    click.echo(f"Data: {results}")


if __name__ == "__main__":
    cli()
```

When you run `python cli.py greet`, the CLI starts quickly since it doesn't initialize the
database connection. But running `python cli.py show_data` accesses `db_module.connection`,
which triggers the connection setup.

This could also be achieved by defining a function that initializes the database connection
and caches it for subsequent calls. However, using module-level `__getattr__` can be more
convenient if you have multiple global variables that require expensive calculations or
initializations. Instead of writing separate functions for each variable, you can handle
them all within the `__getattr__` method.

Here's one example of using it for a non-trivial case in the wild[^2].

[^1]: [PEP 562 – Module \_\_getattr\_\_ and \_\_dir\_\_](https://peps.python.org/pep-0562/)
[^2]: [Prefect - \_\_getattr\_\_](https://github.com/PrefectHQ/prefect/blob/f196fb3da6ae747f7362be2f21e85b01f32e539c/src/prefect/__init__.py#L102)

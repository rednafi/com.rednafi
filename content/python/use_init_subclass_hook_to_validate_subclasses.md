---
title: Use '__init_subclass__' hook to validate subclasses in Python
date: 2021-11-20
tags:
    - Python
    - TIL
---

At my workplace, we have a fairly large Celery config file where you're expected to subclass
from a base class and extend that if there's a new domain. However, the subclass expects the
configuration in a specific schema. So, having a way to enforce that schema in the
subclasses and raising appropriate runtime exceptions is nice.

Wrote a fancy Python 3.6+ `__init_subclasshook__` to validate the subclasses as below. This
is neater than writing a metaclass.

```python
# main.py
from collections.abc import Mapping
from typing import Any


class Base:
    def __init_subclass__(
        cls,
        validate_config: bool = False,
        **kwargs: Any,
    ) -> None:
        if validate_config:
            cls._raise_error_for_invalid_config(cls)

    @staticmethod
    def _raise_error_for_invalid_config(cls) -> None:
        if not "config" in cls.__dict__:
            raise Exception(
                f"'{cls.__name__}' should define a class attribute named 'config'",
            )

        if not isinstance(cls.config, Mapping):
            raise Exception(
                "attribute 'config' should be of 'Mapping' type",
            )

        config = cls.config
        config_keys = config.keys()
        expected_keys = ("foo", "bar", "bazz")

        if not tuple(config_keys) == expected_keys:
            raise Exception(
                f"'config' map should have only '{', '.join(expected_keys)}' keys",
            )

    def __repr__(self) -> str:
        return f"{self.config}"


class Sub(Base, validate_config=True):
    config = {"foo": 1, "bar": 2, "bazz": 3}


s = Sub()

print(s)
```

Running the script will print:

```txt
{'foo': 1, 'bar': 2, 'bazz': 3}
```

However, if we initialize the `Sub` class like this:

```python
class Sub(Base):
    config = {"not" : 1, "allowed": 2}
```

This will raise an error:

```txt
Traceback (most recent call last):
  File "main.py", line 29, in <module>
    class Sub(Base, validate_config=True):
  File "main.py", line 8, in __init_subclass__
    cls._raise_error_for_invalid_config(cls)
  File "main.py", line 23, in _raise_error_for_invalid_config
    raise Exception(f"'config' map should have only '{', '.join(expected_keys)}' keys")
Exception: 'config' map should have only 'foo, bar, bazz' keys
```

## Test

```python
# test_base.py
# Install pytest before running the script.

import pytest
from main import Base


def test_base():
    # Don't raise any exception if validate_config is False.
    class A(Base, validate_config=False):
        hello = "world"

    # Raise error when there's no attribute called config.
    with pytest.raises(
        Exception,
        match="'B' should define a class attribute named 'config'",
    ):

        class B(Base, validate_config=True):
            hello = "world"

    # Raise error when config isn't a Mapping.
    with pytest.raises(
        Exception,
        match="attribute 'config' should be of 'Mapping' type",
    ):

        class C(Base, validate_config=True):
            config = [1, 2, 3]

    # Raise error when config is empty.
    with pytest.raises(
        Exception,
        match="'config' map should have only 'foo, bar, bazz' keys",
    ):

        class D(Base, validate_config=True):
            config = {}

    # Raise error when config doesn't have `foo, bar, bazz` keys.
    with pytest.raises(
        Exception,
        match="'config' map should have only 'foo, bar, bazz' keys",
    ):

        class E(Base, validate_config=True):
            config = {"foo": 1, "bar": 2, "wrong_attribute": 3}

    # Should pass successfully.
    class F(Base):
        config = {"foo": 1, "bar": 2, "bazz": 3}

    # Assert
    f = F()

    # Check the repr.
    assert str(f) == f"{{'foo': 1, 'bar': 2, 'bazz': 3}}"
```

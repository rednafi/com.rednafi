---
title: Patching pydantic settings in pytest
date: 2024-01-27
tags:
    - Python
    - TIL
---

I've been a happy user of pydantic[^1] settings to manage all my app configurations since
the 1.0 era. When pydantic 2.0 was released, the settings portion became a separate package
called `pydantic_settings`[^2].

It does two things that I love: it automatically reads the environment variables from the
`.env` file and allows you to declaratively convert the string values to their desired types
like integers, booleans, etc.

Plus, it lets you override the variables defined in `.env` by exporting them in your shell.

So if you have a variable called `FOO` in your `.env` file like this:

```txt
FOO="some_value"
```

Then you can override it via:

```sh
export FOO="other_value"
```

And pydantic settings will automatically pick up the overridden values without much fuss.

This is neat but can make writing deterministic unit tests tricky. If the settings instance
implicitly pulls config values from both the environment file and shell, testing functions
using those values can easily become flaky. Also, it's usually frowned upon if your unit
tests depend on environment variables in general.

Consider this common instantiation workflow of the settings class. Here, we have the
following app structure:

```txt
.
├── src
│   ├── __init__.py
│   ├── config.py
│   └── main.py
├── tests
│   ├── __init__.py
│   └── test_main.py
└── .env
```

In the `src/config.py` file, we define our settings class as follows:

```python
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    # Override defaults with .env file values.
    model_config = SettingsConfigDict(env_file=".env")

    env_var_1: str = "default_value"
    env_var_2: int = 123
    env_var_3: bool = False
```

Then the corresponding values of the environment variables are defined in the `.env` file.
Pydantic will automatically convert the upper-cased definitions to lower-case.

```txt
ENV_VAR_1="overridden_value"
ENV_VAR_2="42"
ENV_VAR_3="true"
```

Next, we instantiate the `Settings` class in the `src/__init__.py` file:

```python
from src.config import Settings

settings = Settings()
```

Finally, we use the config values in `src/main.py`:

```python
from src import settings


def read_env() -> tuple[str, int, bool]:
    return settings.env_var_1, settings.env_var_2, settings.env_var_3


if __name__ == "__main__":
    env_var_1, env_var_2, env_var_3 = read_env()
    print(f"{env_var_1=}")
    print(f"{env_var_2=}")
    print(f"{env_var_3=}")
```

From the root directory, run the `main.py` file with this command:

```sh
python -m src.main
```

This reveals that pydantic settings is doing its magic--reading the `.env` file and
overriding the default config values:

```txt
env_var_1='overridden_value'
env_var_2=42
env_var_3=True
```

Fantastic! But now, testing the `read_env` function becomes tricky. Normally, you'd try to
patch the environment variables in a pytest fixture and then test the values like this:

```python
# tests/test_main.py

import os
from collections.abc import Iterator
from unittest.mock import patch

import pytest
from src.main import read_env


@pytest.fixture
def patch_env_vars() -> Iterator[None]:
    with patch.dict(
        os.environ,
        {
            "ENV_VAR_1": "test_env_var_1",
            "ENV_VAR_2": "456",
            "ENV_VAR_3": "True",
        },
    ):
        yield


def test_read_env(patch_env_vars: None) -> None:
    env_var_1, env_var_2, env_var_3 = read_env()
    assert env_var_1 == "test_env_var_1"
    assert env_var_2 == 456
    assert env_var_3 is True
```

But the test will fail because we're initializing the `Settings` class in the
`src/__init__.py` file and pydantic processes the environment file and variables before
pytest can intervene.

We want our unit tests to have no dependencies on the environment variables.

You might say initializing a class in the `__init__.py` file like that is an anti-pattern
and all this can be avoided through dependency injection. You'd be right but you'd also be
surprised at how many apps with 7+ figure ARR initialize their config classes like that.

So patching the environment variables doesn't work, what does?

The idea is to let pydantic do its magic and then reset the attributes of the `Settings`
instance to their default values in a fixture. We also want the user of the fixture to be
able to override the values of some or all of the environment variables if necessary.

Here's what has worked well for me:

```python
import pytest
from src.main import read_env
from src import settings, Settings

import pytest
from collections.abc import Iterator
from pytest import FixtureRequest


@pytest.fixture
def patch_settings(request: FixtureRequest) -> Iterator[Settings]:
    # Make a copy of the original settings
    original_settings = settings.model_copy()

    # Collect the env vars to patch
    env_vars_to_patch = getattr(request, "param", {})

    # Patch the settings to use the default values
    for k, v in settings.model_fields.items():
        setattr(settings, k, v.default)

    # Patch the settings with the parametrized env vars
    for key, val in env_vars_to_patch.items():
        # Raise an error if the env var is not defined in the settings
        if not hasattr(settings, key):
            raise ValueError(f"Unknown setting: {key}")

        # Raise an error if the env var has an invalid type
        expected_type = getattr(settings, key).__class__
        if not isinstance(val, expected_type):
            raise ValueError(
                f"Invalid type for {key}: {val.__class__} instead "
                "of {expected_type}"
            )
        setattr(settings, key, val)

    yield settings

    # Restore the original settings
    settings.__dict__.update(original_settings.__dict__)
```

Here, `patch_settings` is a parametrizable fixture where you can optionally pass values via
`pytest.mark.parametrize` to override certain config attributes. If you don't override
anything, the fixture sets the attributes of the `Setting` instance to their default values
defined in the class.

Above, first we make a copy of the original settings instance. Then we reset the attributes
of the `Setting` instance to their default values. Next, we move on to override any values
passed via the `@parametrize` decorator. While doing this, we also check for the correct
type of the incoming values and raise an error accordingly.

Finally, we yield the patched instance and reset everything back to their original values
after a test ends.

You can use the fixture like this:

```python
def test_read_env(patch_settings: Settings) -> None:
    env_var_1, env_var_2, env_var_3 = read_env()
    assert env_var_1 == "default_value"
    assert env_var_2 == 123
    assert env_var_3 is False


@pytest.mark.parametrize(
    "patch_settings",
    [
        {"env_var_1": "patched_value", "env_var_2": 456},
        {"env_var_2": 459},
    ],
    indirect=True,
)
def test_read_env_override(patch_settings: Settings) -> None:
    env_var_1, env_var_2, env_var_3 = read_env()
    assert env_var_1 == patch_settings.env_var_1
    assert env_var_2 == patch_settings.env_var_2
    assert env_var_3 is patch_settings.env_var_3
```

In the first case, we're not overriding anything. So the tests will use the `Settings`
instance with all the default values. In the second test, we're overriding a few values and
the `read_env` function will use the overridden values.

Either way, the tests don't directly depend on the environment variables and it reduces the
probability of spooky actions at a distance.

Fin!

[^1]: [pydantic](https://docs.pydantic.dev/latest/)
[^2]: [pydantic_settings](https://docs.pydantic.dev/latest/concepts/pydantic_settings/)

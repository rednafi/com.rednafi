---
title: Injecting Pytest fixtures without cluttering test signatures
date: 2024-12-02
slug: inject-pytest-fixture
aliases:
    - /python/inject_pytest_fixture/
tags:
  - Python
  - TIL
---

Sometimes, when writing tests in Pytest, I find myself using fixtures that the test
function/method doesn't directly reference. Instead, Pytest runs the fixture, and the test
function implicitly leverages its side effects. For example:

```py
import os
from collections.abc import Iterator
from unittest.mock import Mock, patch
import pytest


# Define an implicit environment mock fixture that patches os.environ
@pytest.fixture
def mock_env() -> Iterator[None]:
    with patch.dict("os.environ", {"IMPLICIT_KEY": "IMPLICIT_VALUE"}):
        yield


# Define an explicit service mock fixture
@pytest.fixture
def mock_svc() -> Mock:
    service = Mock()
    service.process.return_value = "Explicit Mocked Response"
    return service


# IDEs tend to dim out unused parameters like mock_env
def test_stuff(mock_svc: Mock, mock_env: Mock) -> None:
    # Use the explicit mock
    response = mock_svc.process()
    assert response == "Explicit Mocked Response"
    mock_svc.process.assert_called_once()

    # Assert the environment variable patched by mock_env
    assert os.environ["IMPLICIT_KEY"] == "IMPLICIT_VALUE"
```

In the `test_stuff` function above, we directly use the `mock_svc` fixture but not
`mock_env`. Instead, we expect Pytest to run `mock_env`, which modifies the environment
variables. This works, but IDEs often mark `mock_env` as an unused parameter and dims it
out.

One way to avoid this is by marking the `mock_env` fixture with
`@pytest.fixture(autouse=True)` and omitting it from the test function's parameters.
However, I prefer not to use `autouse=True` because it can make reasoning about tests
harder.

TIL that you can use `@pytest.mark.usefixtures`[^1] to inject these implicit fixtures
without cluttering the test function signature or using `autouse`. Here's the same test
marked with `usefixtures`:

```py
# ... same as above


@pytest.mark.usefixtures("mock_env")
def test_stuff(mock_svc: Mock) -> None:
    # Use the explicit mock
    response = mock_svc.process()
    assert response == "Explicit Mocked Response"
    mock_svc.process.assert_called_once()

    # Assert the environment variable patched by mock_env
    assert os.environ["IMPLICIT_KEY"] == "IMPLICIT_VALUE"
```

Now, the `mock_env` fixture is applied without cluttering the test function's signature, and
no more greyed-out unused parameter warnings! The `usefixtures` marker also accepts multiple
fixtures as variadic arguments: `@pytest.mark.usefixtures("fixture_a", "fixture_b")`.

One thing to keep in mind is that it won't work if you try to mark another fixture with the
`usefixtures` decorator. The pytest documentation includes a warning[^2] about this.

Fin!

[^1]:
    [Use fixtures in classes and modules with usefixtures - Pytest docs](https://docs.pytest.org/en/7.1.x/how-to/fixtures.html#usefixtures)

[^2]:
    [The usefixutre mark has no effect on fixtures - Pytest docs](https://docs.pytest.org/en/7.1.x/how-to/fixtures.html#usefixtures:~:text=usefixtures%20%3D%20cleandir-,Warning,-Note%20this%20mark)

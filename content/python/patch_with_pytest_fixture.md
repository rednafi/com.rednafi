---
title: Patching test dependencies via pytest fixture & unittest mock
date: 2022-02-27
tags:
    - Python
    - Testing
---

In Python, even though I adore writing tests in a functional manner via pytest, I still have
a soft corner for the tools provided in the `unittest.mock` module. I like the fact it's
baked into the standard library and is quite flexible. Moreover, I'm yet to see another
`mock` library in any other language or in the Python ecosystem that allows you to mock your
targets in such a terse, flexible, and maintainable fashion.

So, in almost all the tests that I write for both my OSS projects and at my workplace, I use
`unittest.mock.patch` exclusively for performing mock-patch. Consider this example:

```python
# src.py
from __future__ import annotations

import random

# In <Python3.9, import this from the 'typing' module.
from collections.abc import Sequence


def prepend_random(choices: Sequence[str], target: str) -> str:
    """Prepend a random prefix from the choices squence to
    the target string.
    """
    return f"{random.choice(choices)}_{target}"


if __name__ == "__main__":
    print(prepend_random(["hello", "world", "mars"], target="greet"))
```

Here, the `prepend_random` function prepends a random prefix from the `choices` sequence to
a `target` string. To accomplish this randomness, I used the `random.choice` function from
the standard library. Now, the question is, how'd you test this. The function
`prepend_random` has one global dependency; it's the `random.choice` function and you'll
need to mock it out. Otherwise, you won't be able to test the enclosing `prepend_random`
function in a determinstic way. Here's how you might test it with pytest:

```python
# src.py
...
from unittest.mock import patch


@patch("src.random.choice", return_value="test_choice", autospec=True)
def test_prepend_random(mock_random_choice):
    choices = ("some", "choice")
    target = "test_target"
    expected_value = "test_choice_test_target"

    assert prepend_random(choices, target) == expected_value

    mock_random_choice.assert_called_once_with(choices)


...
```

If you run `pytest -v -s src.py`, the test will pass. The `unittest.mock.patch` function is
used here to mock the `random.choice` function and guarantee that it returns a controlled
value instead of a random one. Shaving off this randomness of the `random.choice` function
helps us test the behaviors of the `prepend_random` function in a more reproducible fashion.
The `autospec=True` makes sure that the behavior of the mocked object—in this case, the
function signature—is the same as the original object.

Another thing is, you can also use the `patch` function as a context manager like this:

```python
# src.py
...


def test_prepend_random():
    choices = ("some", "choice")
    target = "test_target"
    expected_value = "test_choice_test_target"

    with patch(
        "src.random.choice", return_value="test_choice", autospec=True
    ) as mock_random_choice:
        assert prepend_random(choices, target) == expected_value

        mock_random_choice.assert_called_once_with(choices)


...
```

While this works, the situation quickly spirals out of control whenever you need to test out
multiple behaviors of a function and you want to achieve loose coupling between the tests by
disentangling the behaviors in separate test functions. In that case, you'll need to mock
out the dependencies in each test function.

The situation worsens when each of your target functions has multiple dependencies. To be
specific, if your target function has `m` behaviors and `n` dependencies that need to be
mocked out, then the number of the `patch` decorators that practically do the same thing
will be `m x n`. Just for a single function, it'll create a monstrosity similar to this:

```python
# src.py
from unittest.mock import patch

...


def func() -> int:
    """Target function that'll be tested."""

    dep_1()
    dep_2()
    return 42


@patch("src.dep_1", autospec=True)
@patch("src.dep_2", autospec=True)
def test_func_error(mock_dep_1, mock_dep_2):
    """Behavior one."""
    ...


@patch("src.dep_1", autospec=True)
@patch("src.dep_2", autospec=True)
def test_func_ok(mock_dep_1, mock_dep_2):
    """Behavior two."""
    ...
```

Now imagine the situation for multiple target functions with multiple behaviors where
testing each behavior requires multiple dependencies. The DRY gods will be furious!

The situation can be improved by wrapping the tests in a `unittest` style class and mocking
out the common dependencies in the class scope as follows:

```python
# src.py
from unittest.mock import patch

...


@patch("src.dep_1", return_value=42, autospec=True)
@patch("src.dep_2", return_value=42, autospec=True)
class TestFunc:
    def test_func_error(self): ...

    def test_func_ok(self): ...
```

The above solution forces us to write `unittest` style OOP-driven tests and I'd like to
avoid that in my test suite. Also, this approach will mock all the dependencies in the class
scope and you can't opt-out of mocked dependencies if some of your tests don't need that. We
can do better. Let's rewrite a slightly modified version of the above case with
`pytest.fixture`. Here's how to do it:

```python
# src.py
from __future__ import annotations

import pytest


def dep_1():
    pass


def dep_2():
    pass


def func(error: bool = False) -> int:
    """Target function that we're going to test."""

    dep_1()
    dep_2()
    if error:
        raise TypeError("Dummy type error.")
    return 42


@pytest.fixture
def mock_dep_1():
    with patch("src.dep_1", return_value=0, autospec=True) as m:
        yield m


@pytest.fixture
def mock_dep_2():
    with patch("src.dep_2", return_value=0, autospec=True) as m:
        yield m


def test_func_error(mock_dep_1, mock_dep_2):
    """Test one behavior."""

    with pytest.raises(TypeError, match="Dummy type error."):
        func(error=True)
    mock_dep_1.assert_called_once()
    mock_dep_2.assert_called_once()


def test_func_ok(mock_dep_1, mock_dep_2):
    """Test another behavior."""

    assert func() == 42
    mock_dep_1.assert_called_once()
    mock_dep_2.assert_called_once()
```

The target function `func` has two dependencies that need to be mocked-up—`dep_1` and
`dep_2`. I mocked out the dependencies using the `unittest.mock.patch` interjector as
context managers while wrapping them in separate functions decorated with the
`@pytest.fixture` decorator.

Pay attention to the `yield` statement in the fixture functions. Pytest also allows you to
use `return` statement in fixtures. However, in this case, making the fixture functions
return generator objects was necessary. This way, the fixture function makes sure that the
teardown logic in the `with patch()...` block gets executed. Had you used `return` here, the
logic in the `__exit__` block of the `patch` context manager wouldn't have the chance to be
executed. If you replace the `yield` statement with `return` and try to run the above
snippet, pytest will throw an error.

You can also write your custom teardown logic after the `yield` statement and pytest will
execute the logic each time the fixture gets executed. It's similar to how teardown works in
`unittest` but in a functional and decoupled manner.

This approach has the following advantages:

- It appeases the DRY gods.

- You won't have to wrap your tests in a class to avoid patching the same objects multiple
  times.

- This makes the _mocked dependency_ usage more composable. In a test, you can pick and
  choose which dependencies you need in their mocked form and which dependencies you don't
  want to be mocked. If you don't want a particular dependency to be mocked in a test, then
  don't pass the corresponding fixture as an argument of the test function.

- If some of your mocked dependencies don't vary much during their test lifetime, you can
  change the `scope` of the fixture to speed up the overall execution. By default, fixtures
  run in `function` scope; that means, the fixture (mocking) will be executed once per test
  function. This behavior can be changed via using the `scope` parameter of the
  `@pytest.fixture(scope=...)` decorator. Other allowed scopes are `module` and `session`.
  **Module** scope means, the fixture will be executed once per test module and **session**
  scope means, the fixture will run once per pytest session.

## Another practical example

The following snippet defines the `get` and `post` functions that make `GET` and `POST`
requests to a URL respectively. I used the HTTPx[^1] library to make the requests. Here, the
functions make external network calls to the `https://httpbin.org` URL:

```python
# src.py
from __future__ import annotations

from typing import Any

# You'll have to pip install this.
import httpx

client = httpx.Client(headers={"Content-Type": "application/json"})


def get(url: str) -> httpx.Response:
    return client.get(url)


def post(url: str, data: dict[str, Any]) -> httpx.Response:
    return client.post(url, json=data)


if __name__ == "__main__":
    r_get = get("https://httpbin.org/get")
    print(r_get.status_code)

    r_post = post("https://httpbin.org/post", data={"hello": "world"})
    print(r_post.status_code)
```

Running the snippet will print the functions' respective HTTP response codes. If you look
closely, you'll notice that I'm instantiating the `client` object in the global scope. This
is because, both the `GET` and `POST` API calls share the same header. Let's see how you can
test these two functions:

```python
# test_src.py
from http import HTTPStatus
from typing import Any
from unittest.mock import patch

import httpx
import pytest

import src


@pytest.fixture(scope="module")
def mock_get():
    with patch.object(src, "client", autospec=True) as m:
        m.get.return_value = httpx.Response(
            status_code=200,
            json={"a": "b"},
        )
        yield m


@pytest.fixture(scope="module")
def mock_post():
    with patch.object(src, "client", autospec=True) as m:
        m.post.return_value = httpx.Response(
            status_code=201,
            json={"a": "b"},
        )
        yield m


def test_get(mock_get):
    assert get("test_url").status_code == HTTPStatus.OK
    mock_get.get.assert_called()


def test_get(mock_post):
    assert post("test_url", {}).status_code == HTTPStatus.CREATED
    mock_post.post.assert_called()
```

The `get` and `post` function share a global dependency, the `client`. Also, these functions
have side effects—since they make external network calls. We'll have to mock the functions
in a way so that our tests are completely isolated and they don't make any network calls. I
mocked out the functions in the `mock_get` and `mock_post` fixtures respectively. The
functions are mocked in a way that whenever the original functions are called in the mock
scope, they'll return consistent values without making any network call.

Since `client` was instantiated in the global scope, it had to be mocked using the
`patch.object(...)` interjector. Also, notice how the mocked-up `get` and `post` functions'
return-values mimic their orginal return-values. In the above case, the fixture runs in the
module scope, which implies, they'll only run once in the entire test module. This makes the
test session quicker. However, keep in mind that making fixtures run in the module scope has
also its demerits. Since the target functions get mocked and stay mocked through the entire
module, it can subtly create coupling between your test functions if you aren't careful.

[^1]: [HTTPx](https://www.python-httpx.org/)

[^2]:
    [Test async code with pytest-asyncio](https://github.com/rednafi/reflections/issues/73)
    [^2]

[^3]:
    [Unittest mock — mock object library](https://docs.python.org/3/library/unittest.mock.html)
    [^3]

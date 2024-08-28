---
title: Taming parametrize with pytest.param
date: 2024-08-28
tags:
    - Python
    - Testing
    - TIL
---

I love `@pytest.mark.parametrize`—so much so that I sometimes shoehorn my tests to fit into
it. But the default style of writing tests with `parametrize` can quickly turn into an
unreadable mess when the test complexity grows. For example:

```python
import pytest
from math import atan2


def polarify(x: float, y: float) -> tuple[float, float]:
    r = (x**2 + y**2) ** 0.5
    theta = atan2(y, x)
    return r, theta


@pytest.mark.parametrize(
    "x, y, expected",
    [
        (0, 0, (0, 0)),
        (1, 0, (1, 0)),
        (0, 1, (1, 1.5707963267948966)),
        (1, 1, (2**0.5, 0.7853981633974483)),
        (-1, -1, (2**0.5, -2.356194490192345)),
    ],
)
def test_polarify(x: float, y: float, expected: tuple[float, float]) -> None:
    # pytest.approx helps us ignore floating point discrepancies
    assert polarify(x, y) == pytest.approx(expected)
```

The `polarify` function converts Cartesian coordinates to polar coordinates. We're using
`@pytest.mark.parametrize` in its standard form to test different conditions.

Here, the list of nested tuples with inputs and expected values becomes hard to read as the
test suite grows larger. When the function under test has a more complex signature, I find
myself needing to do more mental gymnastics to parse the positional input and expected
values inside `parametrize`.

Also, how do you run a specific test case within the suite? For instance, what if you want
to run only the third case where `x, y, expected = (0, 1, (1, 1.5707963267948966))`?

I used to set custom test IDs like below to be able to run individual test cases within
`parametrize`:

```python
# ... polarify implementation hasn't changed.


@pytest.mark.parametrize(
    "x, y, expected",
    [
        (0, 0, (0, 0)),
        (1, 0, (1, 0)),
        (0, 1, (1, 1.5707963267948966)),
        (1, 1, (2**0.5, 0.7853981633974483)),
        (-1, -1, (2**0.5, -2.356194490192345)),
    ],
    ids=[
        "origin",
        "positive_x_axis",
        "positive_y_axis",
        "first_quadrant",
        "third_quadrant",
    ],
)
def test_polarify(x: float, y: float, expected: tuple[float, float]) -> None:
    # pytest.approx helps us ignore floating point discrepancies
    assert polarify(x, y) == pytest.approx(expected)
```

This works, but mentally associating the IDs with the examples is cumbersome, and it doesn't
make things any easier to read.

TIL, `pytest.param` gives you a better syntax and more control to achieve the same. Observe:

```python
# ... polarify implementation hasn't changed.


@pytest.mark.parametrize(
    "x, y, expected",
    [
        pytest.param(0, 0, (0, 0), id="origin"),
        pytest.param(1, 0, (1, 0), id="positive_x_axis"),
        pytest.param(0, 1, (1, 1.5707963267948966), id="positive_y_axis"),
        pytest.param(
            1, 1, (2**0.5, 0.7853981633974483), id="first_quadrant"
        ),
        pytest.param(
            -1, -1, (2**0.5, -2.356194490192345), id="third_quadrant"
        ),
    ],
)
def test_polarify(x: float, y: float, expected: tuple[float, float]) -> None:
    # pytest.approx helps us ignore floating point discrepancies
    assert polarify(x, y) == pytest.approx(expected)
```

We're setting the unique IDs inside `pytest.param`. Now, any test can be targeted with
pytest's `-k` flag like this:

```sh
pytest -k positive_x_axis
```

This will only run the second test case on the list.

Or,

```sh
pytest -k 'first or third'
```

This will run the last two tests.

But the test is still somewhat hard to read. I usually refactor mine to take a `kwargs`
argument so that I can neatly tuck all the input and expected values associated with a test
case in a single dictionary. Notice:

```python
# ... polarify implementation hasn't changed.


@pytest.mark.parametrize(
    "kwargs",
    [
        pytest.param({"x": 0, "y": 0, "expected": (0, 0)}, id="origin"),
        pytest.param(
            {"x": 1, "y": 0, "expected": (1, 0)}, id="positive_x_axis"
        ),
        pytest.param(
            {"x": 0, "y": 1, "expected": (1, 1.5707963267948966)},
            id="positive_y_axis",
        ),
        pytest.param(
            {"x": 1, "y": 1, "expected": (2**0.5, 0.7853981633974483)},
            id="first_quadrant",
        ),
        pytest.param(
            {"x": -1, "y": -1, "expected": (2**0.5, -2.356194490192345)},
            id="third_quadrant",
        ),
    ],
)
def test_polarify(kwargs: dict[str, float]) -> None:
    # Extract expected from kwargs
    expected = kwargs.pop("expected")
    # Unpack the remaining kwargs to the polarify function
    assert polarify(**kwargs) == pytest.approx(expected)
```

Everything associated with a single test case is passed to `pytest.param` in a single
dictionary, eliminating the need to guess any positional arguments.

Using `pytest.param` also allows you to set custom test execution conditionals, which I've
started to take advantage of recently:

```python
# ... polarify implementation hasn't changed.


@pytest.mark.parametrize(
    "kwargs",
    [
        pytest.param(
            {"x": 0, "y": 1, "expected": (1, 1.5707963267948966)},
            id="positive_y_axis",
            marks=pytest.mark.xfail(
                reason="Known issue with atan2 in this quadrant"
            ),
        ),
        pytest.param(
            {"x": 1, "y": 1, "expected": (2**0.5, 0.7853981633974483)},
            id="first_quadrant",
        ),
        pytest.param(
            {
                "x": 1e10,
                "y": 1e10,
                "expected": (2**0.5 * 1e10, 0.7853981633974483),
            },
            id="too_large",
            marks=pytest.mark.skipif(
                lambda kwargs: kwargs["x"] > 1e6 or kwargs["y"] > 1e6,
                reason="Input values are too large",
            ),
        ),
    ],
)
def test_polarify(kwargs: dict[str, float]) -> None:
    # Extract expected from kwargs
    expected = kwargs.pop("expected")
    # Unpack the remaining kwargs to the polarify function
    assert polarify(**kwargs) == pytest.approx(expected)
```

In the last block, `pytest.param` bundles test data with execution conditions. We're using
`xfail` to mark a test as expected to fail, while `skipif` skips tests based on conditions.
This keeps all the logic for handling test cases, including failures and skips, directly
alongside the test data.

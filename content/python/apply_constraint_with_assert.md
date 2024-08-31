---
title: Apply constraints with 'assert' in Python
date: 2022-07-10
tags:
    - Python
---

Whenever I need to apply some runtime constraints on a value while building an API, I
usually compare the value to an expected range and raise a `ValueError` if it's not within
the range. For example, let's define a function that throttles some fictitious operation.
The `throttle` function limits the number of times an operation can be performed by
specifying the `throttle_after` parameter. This parameter defines the number of iterations
after which the operation will be halted. The `current_iter` parameter tracks the current
number of times the operation has been performed. Here's the implementation:

```python
# src.py
def throttle(current_iter: int, throttle_after: int = -1) -> None:
    """
    The value of 'throttle_after' must be -1 or an integer
    greater than 0. Here, -1 means no throttling, and 'n'
    means that the function will throttle some operation
    after 'n' iterations.

    The `current_iter` parameter denotes the current iteration
    of some operation. When 'current_iter > throttle_after' this
    function will throttle the operation.
    """

    # Return early if 'throttle_after=-1'.
    if throttle_after == -1:
        print("No throttling.")
        return

    # Ensure 'current_iter' is a positive integer.
    if not (isinstance(current_iter, int) and current_iter >= 0):
        raise ValueError(
            "Value of 'current_iter' must be a" " positive integer."
        )

    # Ensure 'throttle_after' is a non-zero positive integer.
    if not (isinstance(throttle_after, int) and throttle_after > 0):
        raise ValueError(
            "Value of 'throttle_after' must be either -1 or an"
            " integer greater than 0."
        )

    # Do the throttling.
    if current_iter > throttle_after:
        print(f"Thottling after {throttle_after} iteration(s).")
        return


if __name__ == "__main__":
    # Prints 'Throttling after 1 iteration(s).'
    throttle(current_iter=2, throttle_after=1)
```

We return early if the value of `throttle_after` is -1. Otherwise, we check to see if
`current_iter` is a positive integer and `throttle_after` is a non-zero positive integer. If
not, we raise a `ValueError`. When the parameters pass these checks then we compare
`current_iter` with `throttle_after`. If the value of `current_iter` exceeds that of the
`throttle_after` parameter, we throttle the operation.

While this works fine, recently, I've started to use `assert` to replace the _conditionals
with `ValueError`_ pattern. It works as follows:

```python
# src.py
def throttle(current_iter: int, throttle_after: int = -1) -> None:
    # Return early if 'throttle_after=-1'.
    if throttle_after == -1:
        print("No throttling.")
        return

    # Ensure 'current_iter' is a positive integer.
    assert (
        isinstance(current_iter, int) and current_iter >= 0
    ), "Value of 'current_iter' must be a positive integer."

    # Ensure 'throttle_after' is a non-zero positive integer.
    assert isinstance(throttle_after, int) and throttle_after > 0, (
        "Value of 'throttle_after' must be either -1 or an "
        " integer greater than 0."
    )

    # Do the throttling.
    if current_iter > throttle_after:
        print(f"Thottling after {throttle_after} iterations.")
        return


if __name__ == "__main__":
    # AssertionError: Value of 'current_iter' must be a positive
    # integer.
    throttle(current_iter=-2, throttle_after=1)
```

So, instead of using the `if not expression ... raise ValueError` pattern, we can leverage
`assert expression, "Error message"` pattern. In the latter case, `assert` will raise
`AssertionError` with the "Error message" if the expression evaluates to a falsy value.
Otherwise, the statement will remain silent and allow the execution to move forward.

This is more succinct and makes the code flatter. I've no idea why I haven't started using
it earlier and this[^1] piece of code in the Starlette[^2] repository jolted my brain. Eh
bien, better late than never, I guess.

## Breadcrumbs

After this blog was published, several people mentioned[^3] on Twitter that the second
approach has a small caveat. Python has a flag that allows you to disable `assert`
statements in a script. You can disable the assertions in the snippet above by running the
script with the `-OO` flag:

```sh
python -00 src.py
```

Removing assert statements will disable the constraints needed for the second `throttle`
function to work, which could lead to unexpected behavior or even subtle bugs. However, I
see this being used frequently[^4] in frameworks like Starlette and FastAPI. Also, from my
experience, using assertions is much more common than running production code with the
optimization flag.

[^1]:
    [Usage of assert in the Starlette codebase](https://github.com/encode/starlette/blob/14ef6bbbd6c5f03f0e1222a0a1b33ccc3a5f04cf/starlette/applications.py#L63)

[^2]: [Starlette](https://github.com/encode/starlette)

[^3]:
    [Caveats of using asserts to enforce constraints](https://twitter.com/rednafi/status/1546010546297659392)

[^4]:
    [Usage of assert in the FastAPI codebase](https://github.com/tiangolo/fastapi/blob/bcabbf8b37db3fbc020560e94ad2f90e64d1510a/fastapi/applications)

---
title: Check whether an integer is a power of two in Python
date: 2022-01-21
tags:
    - Python
    - TIL
---

To check whether an integer is a power of two, I've deployed hacks like this:

```python
def is_power_of_two(x: int) -> bool:
    return x > 0 and hex(x)[-1] in ("0", "2", "4", "8")
```

While this works[^1], I've never liked explaining the pattern matching hack that's going on
here.

Today, I came across this tweet[^2] by Raymond Hettinger where he proposed an elegant
solution to the problem. Here's how it goes:

```python
def is_power_of_two(x: int) -> bool:
    return x > 0 and x.bit_count() == 1
```

This is neat as there's no hack and it uses a mathematical invariant to check whether an
integer is a power of `2` or not. Also, it's a tad bit faster.

## Explanation

> Any integer that's a power of `2`, will only contain a single `1` in its binary
> representation.

For example:

```txt
>>> bin(2)
'0b10'
>>> bin(4)
'0b100'
>>> bin(8)
'0b1000'
>>> bin(16)
'0b10000'
>>>
```

The `.bit_count()` function checks how many on-bits (`1`) are there in the binary
representation of an integer.

## Complete example with tests

```python
import unittest


def is_power_of_two(number: int) -> bool:
    return number > 0 and number.bit_count() == 1


class IsPowerofTwoTest(unittest.TestCase):
    def setUp(self):
        self.power_of_twos = [2**x for x in range(2, 25_000)]
        self.not_power_of_twos = [3**x for x in range(2, 25_000)]

    def test_is_power_of_two(self):
        for x, y in zip(self.power_of_twos, self.not_power_of_twos):
            self.assertIs(is_power_of_two(x), True)
            self.assertIs(is_power_of_two(y), False)


if __name__ == "__main__":
    unittest.main()
```

[^1]: [My tweet on the hack](https://twitter.com/rednafi/status/1484326191687696391/photo/1)
[^2]:
    [A better solution by Raymond Hettinger](https://twitter.com/raymondh/status/1483948152906522625)
